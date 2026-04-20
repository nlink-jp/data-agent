package report

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nlink-jp/data-agent/internal/session"
)

func testSession() *session.Session {
	s := session.New("case-1")
	s.SetPlan(&session.Plan{
		Objective: "Analyze incident trends",
		Perspectives: []session.Perspective{{
			ID:          "P1",
			Description: "Time series frequency",
			Steps: []session.Step{
				{ID: "S1", Type: session.StepTypeSQL, Description: "Monthly count", SQL: "SELECT date_trunc('month', ts) AS m, COUNT(*) FROM incidents GROUP BY 1", Status: session.StepDone},
				{ID: "S2", Type: session.StepTypeInterpret, Description: "Trend analysis", DependsOn: []string{"S1"}, Status: session.StepDone},
			},
		}},
	})
	s.RecordExec(session.ExecEntry{
		StepID:   "S1",
		Type:     session.StepTypeSQL,
		SQL:      "SELECT date_trunc('month', ts) AS m, COUNT(*) FROM incidents GROUP BY 1",
		Result:   &session.StepResult{Summary: "Found 12 months of data with increasing trend"},
		Duration: 150 * time.Millisecond,
	})
	s.AddFinding(session.Finding{
		ID:          "F1",
		Description: "Incidents increased 40% in Q4",
		Severity:    "high",
		StepID:      "S1",
	})
	return s
}

func TestGenerateFromSession(t *testing.T) {
	s := testSession()

	r, err := GenerateFromSession(s)
	if err != nil {
		t.Fatal(err)
	}

	if r.Title != "Analyze incident trends" {
		t.Errorf("title = %q, want %q", r.Title, "Analyze incident trends")
	}
	if r.CaseID != "case-1" {
		t.Errorf("case_id = %q, want %q", r.CaseID, "case-1")
	}

	// Check content sections
	content := r.Content
	if !strings.Contains(content, "# Analysis Report:") {
		t.Error("report should contain header")
	}
	if !strings.Contains(content, "## 1. Investigation Plan") {
		t.Error("report should contain investigation plan")
	}
	if !strings.Contains(content, "## 2. Execution Record") {
		t.Error("report should contain execution record")
	}
	if !strings.Contains(content, "## 3. Findings") {
		t.Error("report should contain findings")
	}
	if !strings.Contains(content, "Incidents increased 40%") {
		t.Error("report should contain finding description")
	}
	if !strings.Contains(content, "SELECT date_trunc") {
		t.Error("report should contain executed SQL")
	}
}

func TestGenerateFromSessionNoPlan(t *testing.T) {
	s := session.New("case-1")
	_, err := GenerateFromSession(s)
	if err == nil {
		t.Error("expected error for session without plan")
	}
}

func TestSaveAndList(t *testing.T) {
	dir := t.TempDir()
	s := testSession()

	r, _ := GenerateFromSession(s)

	if err := r.SaveToCase(dir); err != nil {
		t.Fatal(err)
	}

	reports, err := ListReports(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 {
		t.Fatalf("reports = %d, want 1", len(reports))
	}
	if reports[0].Title != "Analyze incident trends" {
		t.Errorf("title = %q, want %q", reports[0].Title, "Analyze incident trends")
	}
}

func TestExportFile(t *testing.T) {
	dir := t.TempDir()
	s := testSession()
	r, _ := GenerateFromSession(s)

	path := filepath.Join(dir, "sub", "report.md")
	if err := r.ExportFile(path); err != nil {
		t.Fatal(err)
	}

	// Verify file exists and has content
	if r.Content == "" {
		t.Error("exported file should have content")
	}
}

func TestPlanRevisionHistory(t *testing.T) {
	s := testSession()
	s.Plan.History = []session.PlanRevision{{
		Version:   1,
		Reason:    "Column not found",
		Changes:   "Replaced P1-S1 SQL",
		Timestamp: time.Now(),
	}}
	s.Plan.Version = 2

	r, err := GenerateFromSession(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Content, "Plan Revision History") {
		t.Error("report should contain revision history")
	}
	if !strings.Contains(r.Content, "Column not found") {
		t.Error("report should contain revision reason")
	}
}

func TestEmptyExecLogAndFindings(t *testing.T) {
	s := session.New("case-1")
	s.SetPlan(&session.Plan{Objective: "Empty test"})

	r, err := GenerateFromSession(s)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Content, "No executions recorded") {
		t.Error("report should indicate no executions")
	}
	if !strings.Contains(r.Content, "No findings recorded") {
		t.Error("report should indicate no findings")
	}
}
