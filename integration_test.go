package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nlink-jp/data-agent/internal/casemgr"
	"github.com/nlink-jp/data-agent/internal/config"
	"github.com/nlink-jp/data-agent/internal/llm"
	"github.com/nlink-jp/data-agent/internal/report"
	"github.com/nlink-jp/data-agent/internal/session"
)

// TestIntegration_CaseLifecycle tests the full case lifecycle:
// create → open → import data → query → close → reopen → verify persistence
func TestIntegration_CaseLifecycle(t *testing.T) {
	baseDir := t.TempDir()

	mgr, err := casemgr.NewManager(baseDir)
	if err != nil {
		t.Fatal(err)
	}

	// Create case
	c, err := mgr.Create("Integration Test Case")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Created case: %s", c.ID)

	// Open case
	if err := mgr.Open(c.ID); err != nil {
		t.Fatal(err)
	}

	// Get engine
	engine, err := mgr.Engine(c.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Create test data files
	dataDir := t.TempDir()

	// CSV data
	csvPath := filepath.Join(dataDir, "incidents.csv")
	csvData := `timestamp,category,severity,source
2024-01-15,malware,high,endpoint
2024-01-20,phishing,medium,email
2024-02-01,malware,critical,server
2024-02-10,unauthorized_access,high,firewall
2024-03-01,phishing,low,email
2024-03-15,malware,high,endpoint
2024-04-01,data_leak,critical,database
`
	os.WriteFile(csvPath, []byte(csvData), 0o600)

	// JSON data
	jsonPath := filepath.Join(dataDir, "users.json")
	jsonData := `[
		{"id":1,"name":"Alice","department":"Security","active":true},
		{"id":2,"name":"Bob","department":"IT","active":true},
		{"id":3,"name":"Charlie","department":"Security","active":false}
	]`
	os.WriteFile(jsonPath, []byte(jsonData), 0o600)

	// JSONL data
	jsonlPath := filepath.Join(dataDir, "logs.jsonl")
	jsonlData := `{"ts":"2024-01-15T10:00:00Z","action":"login","user":"alice","status":"success"}
{"ts":"2024-01-15T10:05:00Z","action":"query","user":"alice","status":"success"}
{"ts":"2024-01-15T11:00:00Z","action":"login","user":"bob","status":"failed"}
{"ts":"2024-01-15T11:01:00Z","action":"login","user":"bob","status":"success"}
`
	os.WriteFile(jsonlPath, []byte(jsonlData), 0o600)

	// Import all data
	for _, tc := range []struct {
		path  string
		table string
	}{
		{csvPath, "incidents"},
		{jsonPath, "users"},
		{jsonlPath, "logs"},
	} {
		if err := engine.Import(tc.path, tc.table); err != nil {
			t.Fatalf("import %s: %v", tc.table, err)
		}
		t.Logf("Imported: %s", tc.table)
	}

	// Verify tables
	tables := engine.Tables()
	if len(tables) != 3 {
		t.Fatalf("tables = %d, want 3", len(tables))
	}
	t.Logf("Tables: %d", len(tables))

	// Verify schema context
	schemaCtx := engine.SchemaContext()
	if !strings.Contains(schemaCtx, "incidents") {
		t.Error("schema context should contain 'incidents'")
	}
	if !strings.Contains(schemaCtx, "users") {
		t.Error("schema context should contain 'users'")
	}
	t.Logf("Schema context: %d chars", len(schemaCtx))

	// Execute queries
	queries := []struct {
		sql      string
		wantRows int
	}{
		{"SELECT COUNT(*) AS total FROM incidents", 1},
		{"SELECT category, COUNT(*) AS cnt FROM incidents GROUP BY category ORDER BY cnt DESC", 4},
		{"SELECT severity, COUNT(*) AS cnt FROM incidents GROUP BY severity ORDER BY cnt DESC", 4},
		{"SELECT * FROM users WHERE active = true", 2},
		{"SELECT action, COUNT(*) AS cnt FROM logs GROUP BY action", 2},
		{"SELECT i.category, u.department FROM incidents i, users u WHERE i.source = 'endpoint' LIMIT 5", 5},
	}

	for _, q := range queries {
		result, err := engine.Execute(q.sql)
		if err != nil {
			t.Errorf("query failed: %s: %v", q.sql, err)
			continue
		}
		if result.RowCount != q.wantRows {
			t.Errorf("%s: rows = %d, want %d", q.sql, result.RowCount, q.wantRows)
		}
		t.Logf("Query OK: %s → %d rows (%s)", q.sql[:min(50, len(q.sql))], result.RowCount, result.Duration)
	}

	// Verify write queries are blocked
	writeQueries := []string{
		"INSERT INTO incidents VALUES ('2024-05-01','test','low','test')",
		"DROP TABLE incidents",
		"CREATE TABLE evil (id INT)",
		"DELETE FROM users WHERE id = 1",
	}
	for _, q := range writeQueries {
		_, err := engine.Execute(q)
		if err == nil {
			t.Errorf("write query should be blocked: %s", q)
		}
	}

	// Close and reopen to verify persistence
	if err := mgr.Close(c.ID); err != nil {
		t.Fatal(err)
	}
	t.Log("Case closed")

	if err := mgr.Open(c.ID); err != nil {
		t.Fatal(err)
	}
	t.Log("Case reopened")

	engine2, _ := mgr.Engine(c.ID)
	tables2 := engine2.Tables()
	if len(tables2) != 3 {
		t.Fatalf("tables after reopen = %d, want 3", len(tables2))
	}

	result, err := engine2.Execute("SELECT COUNT(*) AS total FROM incidents")
	if err != nil {
		t.Fatal(err)
	}
	if result.RowCount != 1 {
		t.Errorf("count after reopen: rows = %d, want 1", result.RowCount)
	}
	t.Log("Persistence verified after reopen")

	mgr.Close(c.ID)
}

// TestIntegration_SessionWorkflow tests the session phase transitions:
// create session → set plan → approve → record executions → review → finalize → report
func TestIntegration_SessionWorkflow(t *testing.T) {
	baseDir := t.TempDir()
	sessionsDir := filepath.Join(baseDir, "sessions")

	// Create session
	sess := session.New("case-integration")
	t.Logf("Session created: %s (phase: %s)", sess.ID, sess.Phase)

	// Planning phase: add messages
	sess.AddMessage("user", "I want to analyze incident frequency by category and severity")
	sess.AddMessage("assistant", "I'll create a plan with two perspectives: category distribution and severity trends.")

	// Set plan
	plan := &session.Plan{
		Objective: "Analyze incident patterns by category and severity",
		Perspectives: []session.Perspective{
			{
				ID:          "P1",
				Description: "Category distribution analysis",
				Steps: []session.Step{
					{ID: "P1-S1", Type: session.StepTypeSQL, Description: "Count by category", SQL: "SELECT category, COUNT(*) AS cnt FROM incidents GROUP BY category ORDER BY cnt DESC"},
					{ID: "P1-S2", Type: session.StepTypeInterpret, Description: "Interpret category distribution", DependsOn: []string{"P1-S1"}},
				},
			},
			{
				ID:          "P2",
				Description: "Severity trend analysis",
				Steps: []session.Step{
					{ID: "P2-S1", Type: session.StepTypeSQL, Description: "Count by severity", SQL: "SELECT severity, COUNT(*) AS cnt FROM incidents GROUP BY severity"},
					{ID: "P2-S2", Type: session.StepTypeSQL, Description: "Monthly severity breakdown", SQL: "SELECT substr(timestamp,1,7) AS month, severity, COUNT(*) FROM incidents GROUP BY 1,2"},
					{ID: "P2-S3", Type: session.StepTypeAggregate, Description: "Synthesize severity findings", DependsOn: []string{"P2-S1", "P2-S2"}},
				},
			},
		},
	}
	sess.SetPlan(plan)
	t.Logf("Plan set: %s (version %d, %d perspectives, %d total steps)",
		plan.Objective, sess.Plan.Version,
		len(sess.Plan.Perspectives),
		countSteps(sess.Plan))

	// Approve plan → Execution
	if err := sess.ApprovePlan(); err != nil {
		t.Fatal(err)
	}
	t.Logf("Phase: %s", sess.Phase)

	// Simulate execution
	steps := []struct {
		stepID  string
		sql     string
		summary string
	}{
		{"P1-S1", "SELECT category, COUNT(*) AS cnt FROM incidents GROUP BY category ORDER BY cnt DESC", "malware:3, phishing:2, unauthorized_access:1, data_leak:1"},
		{"P1-S2", "", "Malware is the dominant category (43%), followed by phishing (29%)"},
		{"P2-S1", "SELECT severity, COUNT(*) AS cnt FROM incidents GROUP BY severity", "critical:2, high:3, medium:1, low:1"},
		{"P2-S2", "SELECT substr(timestamp,1,7) AS month, severity, COUNT(*) FROM incidents GROUP BY 1,2", "Severity escalated from Q1 to Q2"},
		{"P2-S3", "", "Overall severity trend is increasing, with critical incidents in Feb and Apr"},
	}

	for _, s := range steps {
		sess.RecordExec(session.ExecEntry{
			StepID: s.stepID,
			Type:   session.StepTypeSQL,
			SQL:    s.sql,
			Result: &session.StepResult{Summary: s.summary},
		})
		step, _ := sess.FindStep(s.stepID)
		if step != nil {
			step.Status = session.StepDone
		}
		t.Logf("Executed: %s → %s", s.stepID, s.summary[:min(60, len(s.summary))])
	}

	// Add findings
	sess.AddFinding(session.Finding{ID: "F1", Description: "Malware is the dominant threat (43%)", Severity: "high", StepID: "P1-S2"})
	sess.AddFinding(session.Finding{ID: "F2", Description: "Severity trend is escalating", Severity: "critical", StepID: "P2-S3"})
	t.Logf("Findings: %d", len(sess.Findings))

	// Transition to review
	if err := sess.TransitionToReview(); err != nil {
		t.Fatal(err)
	}
	t.Logf("Phase: %s", sess.Phase)

	// Generate report
	rpt, err := report.GenerateFromSession(sess, "Integration test: analysis shows patterns.")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Report generated: %s (%d chars)", rpt.Title, len(rpt.Content))

	// Verify report content
	content := rpt.Content
	checks := []struct {
		name string
		want string
	}{
		{"header", "# Analysis Report:"},
		{"objective", "Analyze incident patterns"},
		{"executive summary", "## 1. Executive Summary"},
		{"plan section", "## 2. Investigation Plan"},
		{"exec section", "## 4. Execution Details"},
		{"finding F1", "Malware is the dominant threat"},
		{"finding F2", "Severity trend is escalating"},
		{"SQL in report", "SELECT category"},
		{"step result", "malware:3"},
	}
	for _, c := range checks {
		if !strings.Contains(content, c.want) {
			t.Errorf("report missing %s: %q", c.name, c.want)
		}
	}

	// Save report
	reportsDir := filepath.Join(baseDir, "reports")
	if err := rpt.SaveToCase(reportsDir); err != nil {
		t.Fatal(err)
	}

	// Verify report can be listed
	reports, err := report.ListReports(reportsDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %d, want 1", len(reports))
	}
	t.Logf("Report saved and listed OK")

	// Finalize
	if err := sess.Finalize(); err != nil {
		t.Fatal(err)
	}
	t.Logf("Phase: %s", sess.Phase)

	// Save and reload session
	if err := sess.Save(sessionsDir); err != nil {
		t.Fatal(err)
	}

	loaded, err := session.Load(sessionsDir, sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Phase != session.PhaseDone {
		t.Errorf("loaded phase = %q, want %q", loaded.Phase, session.PhaseDone)
	}
	if loaded.Plan.Objective != "Analyze incident patterns by category and severity" {
		t.Errorf("loaded objective mismatch")
	}
	if len(loaded.ExecLog) != 5 {
		t.Errorf("loaded exec_log = %d, want 5", len(loaded.ExecLog))
	}
	if len(loaded.Findings) != 2 {
		t.Errorf("loaded findings = %d, want 2", len(loaded.Findings))
	}
	t.Log("Session persistence verified")
}

// TestIntegration_ErrorRecoveryFlow tests the replan flow on critical error.
func TestIntegration_ErrorRecoveryFlow(t *testing.T) {
	sess := session.New("case-error")
	sess.SetPlan(&session.Plan{
		Objective: "Test error recovery",
		Perspectives: []session.Perspective{{
			ID:          "P1",
			Description: "Analysis requiring specific columns",
			Steps: []session.Step{
				{ID: "S1", Type: session.StepTypeSQL, Description: "Query timestamp column", SQL: "SELECT timestamp FROM events"},
				{ID: "S2", Type: session.StepTypeInterpret, Description: "Interpret timeline", DependsOn: []string{"S1"}},
				{ID: "S3", Type: session.StepTypeAggregate, Description: "Final summary", DependsOn: []string{"S2"}},
				{ID: "S4", Type: session.StepTypeSQL, Description: "Independent query", SQL: "SELECT COUNT(*) FROM events"},
			},
		}},
	})
	sess.ApprovePlan()
	t.Logf("Phase: %s", sess.Phase)

	// S1 fails critically (column doesn't exist)
	s1, p := sess.FindStep("S1")
	s1.Status = session.StepFailed
	s1.Error = &session.StepError{
		Message:  "column 'timestamp' does not exist",
		Severity: session.ErrorCritical,
	}

	// Find dependent steps
	affected := sess.FindDependentSteps("S1", p)
	affectedIDs := make([]string, len(affected))
	for i, s := range affected {
		s.Status = session.StepSkipped
		affectedIDs[i] = s.ID
	}
	t.Logf("S1 failed critically, affected steps: %v", affectedIDs)

	if len(affected) != 2 {
		t.Errorf("affected = %d, want 2 (S2, S3)", len(affected))
	}

	// S4 should NOT be affected (independent)
	s4, _ := sess.FindStep("S4")
	if s4.Status != session.StepPlanned {
		t.Errorf("S4 status = %q, want %q (should be unaffected)", s4.Status, session.StepPlanned)
	}

	// Force replan
	sess.ForceReplan("column 'timestamp' does not exist in events table")
	t.Logf("Phase after replan: %s (plan version: %d)", sess.Phase, sess.Plan.Version)

	if sess.Phase != session.PhasePlanning {
		t.Errorf("phase = %q, want %q", sess.Phase, session.PhasePlanning)
	}
	if sess.Plan.Version != 2 {
		t.Errorf("plan version = %d, want 2", sess.Plan.Version)
	}
	if len(sess.Plan.History) != 1 {
		t.Errorf("history = %d, want 1", len(sess.Plan.History))
	}
	t.Logf("Replan history: %s", sess.Plan.History[0].Reason)
}

// TestIntegration_TokenEstimation verifies token estimation across different data types.
func TestIntegration_TokenEstimation(t *testing.T) {
	tests := []struct {
		name string
		text string
		min  int
		max  int
	}{
		{"empty", "", 0, 0},
		{"short english", "Hello world", 2, 10},
		{"japanese", "データ分析を開始します", 10, 30},
		{"json object", `{"category":"malware","count":42,"severity":"high"}`, 10, 30},
		{"large json array", generateLargeJSON(100), 500, 5000},
		{"sql query", "SELECT category, COUNT(*) AS cnt FROM incidents WHERE severity = 'high' GROUP BY category ORDER BY cnt DESC", 15, 50},
		{"mixed ja+en", "インシデント分析: malware incidents increased by 40% in Q4", 15, 60},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := llm.EstimateTokenCount(tt.text)
			if tokens < tt.min || tokens > tt.max {
				t.Errorf("EstimateTokenCount = %d, want [%d, %d]", tokens, tt.min, tt.max)
			}
			t.Logf("%s: %d tokens (%d chars)", tt.name, tokens, len(tt.text))
		})
	}
}

// TestIntegration_ConfigRoundTrip tests config save → load → verify.
func TestIntegration_ConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")

	cfg := config.DefaultConfig()
	cfg.LLM.Backend = "vertex_ai"
	cfg.VertexAI.Project = "test-project"
	cfg.VertexAI.Region = "asia-northeast1"
	cfg.LocalLLM.APIKey = "secret-key"
	cfg.Analysis.ContextLimit = 65536
	cfg.Container.Runtime = "docker"

	if err := config.Save(cfg, path); err != nil {
		t.Fatal(err)
	}
	t.Logf("Config saved to %s", path)

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.LLM.Backend != "vertex_ai" {
		t.Errorf("backend = %q", loaded.LLM.Backend)
	}
	if loaded.VertexAI.Project != "test-project" {
		t.Errorf("project = %q", loaded.VertexAI.Project)
	}
	if loaded.VertexAI.Region != "asia-northeast1" {
		t.Errorf("region = %q", loaded.VertexAI.Region)
	}
	if loaded.LocalLLM.APIKey != "secret-key" {
		t.Errorf("api_key = %q", loaded.LocalLLM.APIKey)
	}
	if loaded.Analysis.ContextLimit != 65536 {
		t.Errorf("context_limit = %d", loaded.Analysis.ContextLimit)
	}
	if loaded.Container.Runtime != "docker" {
		t.Errorf("runtime = %q", loaded.Container.Runtime)
	}
	t.Log("Config round-trip verified")
}

func countSteps(plan *session.Plan) int {
	n := 0
	for _, p := range plan.Perspectives {
		n += len(p.Steps)
	}
	return n
}

func generateLargeJSON(n int) string {
	items := make([]map[string]any, n)
	for i := range items {
		items[i] = map[string]any{
			"id":       i,
			"category": "test",
			"value":    i * 100,
			"active":   i%2 == 0,
		}
	}
	data, _ := json.Marshal(items)
	return string(data)
}
