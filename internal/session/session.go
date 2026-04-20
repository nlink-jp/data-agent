package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/google/uuid"
)

// New creates a new analysis session for a case.
func New(caseID string) *Session {
	return &Session{
		ID:        uuid.New().String(),
		CaseID:    caseID,
		Phase:     PhasePlanning,
		Chat:      []ChatMessage{},
		ExecLog:   []ExecEntry{},
		Findings:  []Finding{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// AddMessage appends a chat message to the session.
func (s *Session) AddMessage(role, content string) {
	s.Chat = append(s.Chat, ChatMessage{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	})
	s.UpdatedAt = time.Now()
}

// SetPlan sets the investigation plan and transitions to execution readiness.
func (s *Session) SetPlan(plan *Plan) {
	plan.Version = 1
	for i := range plan.Perspectives {
		plan.Perspectives[i].Status = PerspectiveActive
		for j := range plan.Perspectives[i].Steps {
			plan.Perspectives[i].Steps[j].Status = StepPlanned
		}
	}
	s.Plan = plan
	s.UpdatedAt = time.Now()
}

// ApprovePlan transitions the session from Planning to Execution.
func (s *Session) ApprovePlan() error {
	if s.Phase != PhasePlanning {
		return fmt.Errorf("cannot approve plan in %s phase", s.Phase)
	}
	if s.Plan == nil {
		return fmt.Errorf("no plan to approve")
	}
	s.Phase = PhaseExecution
	s.UpdatedAt = time.Now()
	return nil
}

// TransitionToReview moves the session to the Review phase.
func (s *Session) TransitionToReview() error {
	if s.Phase != PhaseExecution {
		return fmt.Errorf("cannot transition to review from %s phase", s.Phase)
	}
	s.Phase = PhaseReview
	s.UpdatedAt = time.Now()
	return nil
}

// RequestAdditionalAnalysis returns to the Planning phase for more analysis.
func (s *Session) RequestAdditionalAnalysis() error {
	if s.Phase != PhaseReview {
		return fmt.Errorf("cannot request additional analysis from %s phase", s.Phase)
	}
	s.Phase = PhasePlanning
	s.UpdatedAt = time.Now()
	return nil
}

// Finalize marks the session as done. Can be called from Execution or Review phase.
func (s *Session) Finalize() error {
	if s.Phase != PhaseExecution && s.Phase != PhaseReview {
		return fmt.Errorf("cannot finalize from %s phase", s.Phase)
	}
	s.Phase = PhaseDone
	s.UpdatedAt = time.Now()
	return nil
}

// Reopen transitions a Done session back to Planning for additional analysis.
// Clears the plan so a new one must be proposed by the LLM.
func (s *Session) Reopen() error {
	if s.Phase != PhaseDone {
		return fmt.Errorf("cannot reopen from %s phase", s.Phase)
	}
	s.Phase = PhasePlanning
	s.Plan = nil // Clear plan — LLM must propose a new one
	s.UpdatedAt = time.Now()
	return nil
}

// ForceReplan transitions back to Planning due to critical error.
func (s *Session) ForceReplan(reason string) {
	if s.Plan == nil {
		s.Phase = PhasePlanning
		s.UpdatedAt = time.Now()
		return
	}
	if s.Plan != nil {
		s.Plan.History = append(s.Plan.History, PlanRevision{
			Version:   s.Plan.Version,
			Reason:    reason,
			Changes:   "Forced replan due to critical error",
			Timestamp: time.Now(),
		})
		s.Plan.Version++
	}
	s.Phase = PhasePlanning
	s.UpdatedAt = time.Now()
}

// RecordExec appends an execution entry to the audit log.
func (s *Session) RecordExec(entry ExecEntry) {
	if s.Plan != nil {
		entry.PlanVersion = s.Plan.Version
	}
	entry.Timestamp = time.Now()
	s.ExecLog = append(s.ExecLog, entry)
	s.UpdatedAt = time.Now()
}

// AddFinding appends a finding to the session.
func (s *Session) AddFinding(f Finding) {
	s.Findings = append(s.Findings, f)
	s.UpdatedAt = time.Now()
}

// FindStep finds a step by ID across all perspectives.
func (s *Session) FindStep(stepID string) (*Step, *Perspective) {
	if s.Plan == nil {
		return nil, nil
	}
	for i := range s.Plan.Perspectives {
		p := &s.Plan.Perspectives[i]
		for j := range p.Steps {
			if p.Steps[j].ID == stepID {
				return &p.Steps[j], p
			}
		}
	}
	return nil, nil
}

// FindDependentSteps returns all steps that depend on the given step ID.
func (s *Session) FindDependentSteps(failedID string, perspective *Perspective) []*Step {
	if perspective == nil {
		return nil
	}
	affected := map[string]bool{failedID: true}
	var result []*Step

	// Iteratively find all transitive dependents
	changed := true
	for changed {
		changed = false
		for i := range perspective.Steps {
			step := &perspective.Steps[i]
			if affected[step.ID] {
				continue
			}
			for _, dep := range step.DependsOn {
				if affected[dep] {
					affected[step.ID] = true
					result = append(result, step)
					changed = true
					break
				}
			}
		}
	}
	return result
}

// Save persists the session to disk.
func (s *Session) Save(sessionsDir string) error {
	dir := filepath.Join(sessionsDir, s.ID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}
	return writeJSON(filepath.Join(dir, "session.json"), s)
}

// Load reads a session from disk.
func Load(sessionsDir, sessionID string) (*Session, error) {
	path := filepath.Join(sessionsDir, sessionID, "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s Session
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse session: %w", err)
	}
	return &s, nil
}

// ListSessions returns metadata for all sessions in a case.
func ListSessions(sessionsDir string) ([]Session, error) {
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var sessions []Session
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		s, err := Load(sessionsDir, e.Name())
		if err != nil {
			continue
		}
		sessions = append(sessions, *s)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].CreatedAt.After(sessions[j].CreatedAt)
	})
	return sessions, nil
}

// DeleteSession removes a session from disk.
func DeleteSession(sessionsDir, sessionID string) error {
	dir := filepath.Join(sessionsDir, sessionID)
	return os.RemoveAll(dir)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
