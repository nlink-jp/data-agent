package session

import (
	"testing"
)

func TestNewSession(t *testing.T) {
	s := New("case-123")
	if s.CaseID != "case-123" {
		t.Errorf("case_id = %q, want %q", s.CaseID, "case-123")
	}
	if s.Phase != PhasePlanning {
		t.Errorf("phase = %q, want %q", s.Phase, PhasePlanning)
	}
	if s.ID == "" {
		t.Error("session ID should not be empty")
	}
}

func TestPhaseTransitions(t *testing.T) {
	s := New("case-1")

	// Cannot approve without plan
	if err := s.ApprovePlan(); err == nil {
		t.Error("expected error when approving without plan")
	}

	// Set plan and approve
	s.SetPlan(&Plan{
		Objective: "Test",
		Perspectives: []Perspective{{
			ID:          "P1",
			Description: "Test perspective",
			Steps: []Step{{
				ID:          "S1",
				Type:        StepTypeSQL,
				Description: "Count rows",
				SQL:         "SELECT COUNT(*) FROM t",
			}},
		}},
	})

	if err := s.ApprovePlan(); err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhaseExecution {
		t.Errorf("phase = %q, want %q", s.Phase, PhaseExecution)
	}

	// Cannot approve again
	if err := s.ApprovePlan(); err == nil {
		t.Error("expected error when approving in execution phase")
	}

	// Transition to review
	if err := s.TransitionToReview(); err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhaseReview {
		t.Errorf("phase = %q, want %q", s.Phase, PhaseReview)
	}

	// Request additional analysis
	if err := s.RequestAdditionalAnalysis(); err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhasePlanning {
		t.Errorf("phase = %q, want %q", s.Phase, PhasePlanning)
	}
}

func TestFinalize(t *testing.T) {
	s := New("case-1")
	s.SetPlan(&Plan{Objective: "Test"})
	s.ApprovePlan()
	s.TransitionToReview()

	if err := s.Finalize(); err != nil {
		t.Fatal(err)
	}
	if s.Phase != PhaseDone {
		t.Errorf("phase = %q, want %q", s.Phase, PhaseDone)
	}
}

func TestForceReplan(t *testing.T) {
	s := New("case-1")
	s.SetPlan(&Plan{Objective: "Test"})
	s.ApprovePlan()

	s.ForceReplan("column not found")
	if s.Phase != PhasePlanning {
		t.Errorf("phase = %q, want %q", s.Phase, PhasePlanning)
	}
	if s.Plan.Version != 2 {
		t.Errorf("plan version = %d, want 2", s.Plan.Version)
	}
	if len(s.Plan.History) != 1 {
		t.Errorf("history = %d, want 1", len(s.Plan.History))
	}
}

func TestSetPlanInitializesStatus(t *testing.T) {
	s := New("case-1")
	s.SetPlan(&Plan{
		Objective: "Test",
		Perspectives: []Perspective{{
			ID: "P1",
			Steps: []Step{
				{ID: "S1", Type: StepTypeSQL},
				{ID: "S2", Type: StepTypeInterpret},
			},
		}},
	})

	p := s.Plan.Perspectives[0]
	if p.Status != PerspectiveActive {
		t.Errorf("perspective status = %q, want %q", p.Status, PerspectiveActive)
	}
	for _, step := range p.Steps {
		if step.Status != StepPlanned {
			t.Errorf("step %s status = %q, want %q", step.ID, step.Status, StepPlanned)
		}
	}
}

func TestFindStep(t *testing.T) {
	s := New("case-1")
	s.SetPlan(&Plan{
		Perspectives: []Perspective{
			{ID: "P1", Steps: []Step{{ID: "S1"}, {ID: "S2"}}},
			{ID: "P2", Steps: []Step{{ID: "S3"}}},
		},
	})

	step, p := s.FindStep("S2")
	if step == nil {
		t.Fatal("S2 should be found")
	}
	if p.ID != "P1" {
		t.Errorf("perspective = %q, want %q", p.ID, "P1")
	}

	step, _ = s.FindStep("S99")
	if step != nil {
		t.Error("S99 should not be found")
	}
}

func TestFindDependentSteps(t *testing.T) {
	s := New("case-1")
	p := Perspective{
		ID: "P1",
		Steps: []Step{
			{ID: "S1", Type: StepTypeSQL},
			{ID: "S2", Type: StepTypeInterpret, DependsOn: []string{"S1"}},
			{ID: "S3", Type: StepTypeAggregate, DependsOn: []string{"S2"}},
			{ID: "S4", Type: StepTypeSQL}, // independent
		},
	}

	affected := s.FindDependentSteps("S1", &p)
	ids := map[string]bool{}
	for _, s := range affected {
		ids[s.ID] = true
	}
	if !ids["S2"] {
		t.Error("S2 depends on S1 and should be affected")
	}
	if !ids["S3"] {
		t.Error("S3 transitively depends on S1 and should be affected")
	}
	if ids["S4"] {
		t.Error("S4 is independent and should not be affected")
	}
}

func TestRecordExec(t *testing.T) {
	s := New("case-1")
	s.SetPlan(&Plan{Objective: "Test"})

	s.RecordExec(ExecEntry{
		StepID: "S1",
		Type:   StepTypeSQL,
		SQL:    "SELECT 1",
	})

	if len(s.ExecLog) != 1 {
		t.Fatalf("exec_log = %d, want 1", len(s.ExecLog))
	}
	if s.ExecLog[0].PlanVersion != 1 {
		t.Errorf("plan_version = %d, want 1", s.ExecLog[0].PlanVersion)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	s := New("case-1")
	s.AddMessage("user", "Hello")
	s.SetPlan(&Plan{Objective: "Test investigation"})

	if err := s.Save(dir); err != nil {
		t.Fatal(err)
	}

	loaded, err := Load(dir, s.ID)
	if err != nil {
		t.Fatal(err)
	}

	if loaded.CaseID != "case-1" {
		t.Errorf("case_id = %q, want %q", loaded.CaseID, "case-1")
	}
	if loaded.Plan.Objective != "Test investigation" {
		t.Errorf("objective = %q, want %q", loaded.Plan.Objective, "Test investigation")
	}
	if len(loaded.Chat) != 1 {
		t.Errorf("chat = %d, want 1", len(loaded.Chat))
	}
}

func TestListSessions(t *testing.T) {
	dir := t.TempDir()

	s1 := New("case-1")
	s2 := New("case-1")
	s1.Save(dir)
	s2.Save(dir)

	sessions, err := ListSessions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 {
		t.Errorf("sessions = %d, want 2", len(sessions))
	}
}

func TestAddFinding(t *testing.T) {
	s := New("case-1")
	s.AddFinding(Finding{
		ID:          "F1",
		Description: "Anomaly detected",
		Severity:    "high",
		StepID:      "S1",
	})

	if len(s.Findings) != 1 {
		t.Fatalf("findings = %d, want 1", len(s.Findings))
	}
	if s.Findings[0].Severity != "high" {
		t.Errorf("severity = %q, want %q", s.Findings[0].Severity, "high")
	}
}
