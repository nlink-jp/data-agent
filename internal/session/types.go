package session

import "time"

// Phase represents the current phase of an analysis session.
type Phase string

const (
	PhasePlanning  Phase = "planning"
	PhaseExecution Phase = "execution"
	PhaseReview    Phase = "review"
	PhaseDone      Phase = "done"
)

// Session represents an analysis session within a case.
type Session struct {
	ID        string        `json:"id"`
	CaseID    string        `json:"case_id"`
	Phase     Phase         `json:"phase"`
	Plan      *Plan         `json:"plan,omitempty"`
	Chat      []ChatMessage `json:"chat"`
	ExecLog   []ExecEntry   `json:"exec_log"`
	Findings  []Finding     `json:"findings"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// ChatMessage is a single message in the session conversation.
type ChatMessage struct {
	Role      string    `json:"role"` // "user", "assistant", "system"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// Plan represents a structured investigation plan.
type Plan struct {
	Objective    string         `json:"objective"`
	Perspectives []Perspective  `json:"perspectives"`
	Version      int            `json:"version"`
	History      []PlanRevision `json:"history,omitempty"`
}

// Perspective represents an analysis angle within a plan.
type Perspective struct {
	ID          string            `json:"id"`
	Description string            `json:"description"`
	Steps       []Step            `json:"steps"`
	Status      PerspectiveStatus `json:"status"`
}

// PerspectiveStatus represents the status of a perspective.
type PerspectiveStatus string

const (
	PerspectiveActive      PerspectiveStatus = "active"
	PerspectiveCompleted   PerspectiveStatus = "completed"
	PerspectiveInvalidated PerspectiveStatus = "invalidated"
)

// Step represents a single analysis step within a perspective.
type Step struct {
	ID          string     `json:"id"`
	Type        StepType   `json:"type"`
	Description string     `json:"description"`
	SQL         string     `json:"sql,omitempty"`
	DependsOn   []string   `json:"depends_on,omitempty"`
	Status      StepStatus `json:"status"`
	Result      *StepResult `json:"result,omitempty"`
	Error       *StepError  `json:"error,omitempty"`
	RetryCount  int        `json:"retry_count"`
}

// StepType defines what executes a step.
type StepType string

const (
	StepTypeSQL       StepType = "sql"
	StepTypeInterpret StepType = "interpret"
	StepTypeAggregate StepType = "aggregate"
	StepTypeContainer StepType = "container"
)

// StepStatus represents step lifecycle state.
type StepStatus string

const (
	StepPlanned StepStatus = "planned"
	StepRunning StepStatus = "running"
	StepDone    StepStatus = "done"
	StepFailed  StepStatus = "failed"
	StepSkipped StepStatus = "skipped"
	StepRevised StepStatus = "revised"
)

// StepResult holds the output of a completed step.
type StepResult struct {
	Summary string `json:"summary"`
	Data    string `json:"data,omitempty"` // JSON-encoded result data
}

// StepError holds error information for a failed step.
type StepError struct {
	Message  string `json:"message"`
	Severity ErrorSeverity `json:"severity"`
}

// ErrorSeverity classifies error impact.
type ErrorSeverity string

const (
	ErrorMinor    ErrorSeverity = "minor"    // SQL syntax, type mismatch
	ErrorModerate ErrorSeverity = "moderate" // Missing column, empty data
	ErrorCritical ErrorSeverity = "critical" // Perspective premise collapsed
)

// PlanRevision records a change to the plan.
type PlanRevision struct {
	Version   int       `json:"version"`
	Reason    string    `json:"reason"`
	Changes   string    `json:"changes"`
	Timestamp time.Time `json:"timestamp"`
}

// ExecEntry records a single execution in the audit trail.
type ExecEntry struct {
	StepID      string        `json:"step_id"`
	Type        StepType      `json:"type"`
	SQL         string        `json:"sql,omitempty"`
	Result      *StepResult   `json:"result,omitempty"`
	Error       string        `json:"error,omitempty"`
	Decision    string        `json:"decision,omitempty"`
	Duration    time.Duration `json:"duration"`
	Timestamp   time.Time     `json:"timestamp"`
	PlanVersion int           `json:"plan_version"`
}

// Finding represents a discovery made during analysis.
type Finding struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Severity    string `json:"severity"` // high, medium, low, info
	StepID      string `json:"step_id"`
	Data        string `json:"data,omitempty"`
}
