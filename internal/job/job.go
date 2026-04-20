package job

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Status represents job lifecycle state.
type Status string

const (
	StatusPending   Status = "pending"
	StatusRunning   Status = "running"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
	StatusCancelled Status = "cancelled"
)

// Type classifies the kind of job.
type Type string

const (
	TypeSession      Type = "session"       // Full session execution
	TypeSlidingWindow Type = "sliding_window"
	TypeContainer    Type = "container"
)

// Job represents a background analysis task.
type Job struct {
	ID        string    `json:"id"`
	CaseID    string    `json:"case_id"`
	Type      Type      `json:"type"`
	Status    Status    `json:"status"`
	Progress  float64   `json:"progress"` // 0.0 - 1.0
	Error     string    `json:"error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	ctx    context.Context
	cancel context.CancelFunc
}

// ProgressCallback is called to report job progress.
type ProgressCallback func(jobID string, progress float64)

// CompletionCallback is called when a job finishes.
type CompletionCallback func(jobID string, err error)

// Manager manages background jobs.
type Manager struct {
	jobs       map[string]*Job
	mu         sync.RWMutex
	onProgress ProgressCallback
	onComplete CompletionCallback
}

// NewManager creates a job Manager with callbacks.
func NewManager(onProgress ProgressCallback, onComplete CompletionCallback) *Manager {
	return &Manager{
		jobs:       make(map[string]*Job),
		onProgress: onProgress,
		onComplete: onComplete,
	}
}

// Submit creates and starts a new background job.
// The work function receives a context and a progress reporter.
func (m *Manager) Submit(caseID string, jobType Type, work func(ctx context.Context, setProgress func(float64)) error) string {
	ctx, cancel := context.WithCancel(context.Background())

	job := &Job{
		ID:        uuid.New().String(),
		CaseID:    caseID,
		Type:      jobType,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()

	go m.run(job, work)

	return job.ID
}

// Cancel requests cancellation of a running job.
func (m *Manager) Cancel(id string) error {
	m.mu.RLock()
	job, exists := m.jobs[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job %s not found", id)
	}
	if job.Status != StatusRunning && job.Status != StatusPending {
		return fmt.Errorf("job %s is %s, cannot cancel", id, job.Status)
	}

	job.cancel()
	return nil
}

// Get returns a copy of the job state.
func (m *Manager) Get(id string) (*Job, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[id]
	if !exists {
		return nil, fmt.Errorf("job %s not found", id)
	}
	// Return a copy
	copy := *job
	copy.ctx = nil
	copy.cancel = nil
	return &copy, nil
}

// List returns all jobs for a case.
func (m *Manager) List(caseID string) []Job {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Job
	for _, j := range m.jobs {
		if j.CaseID == caseID {
			copy := *j
			copy.ctx = nil
			copy.cancel = nil
			result = append(result, copy)
		}
	}
	return result
}

func (m *Manager) run(job *Job, work func(ctx context.Context, setProgress func(float64)) error) {
	m.mu.Lock()
	job.Status = StatusRunning
	job.UpdatedAt = time.Now()
	m.mu.Unlock()

	setProgress := func(p float64) {
		m.mu.Lock()
		job.Progress = p
		job.UpdatedAt = time.Now()
		m.mu.Unlock()
		if m.onProgress != nil {
			m.onProgress(job.ID, p)
		}
	}

	err := work(job.ctx, setProgress)

	m.mu.Lock()
	if err != nil {
		if job.ctx.Err() != nil {
			job.Status = StatusCancelled
		} else {
			job.Status = StatusFailed
			job.Error = err.Error()
		}
	} else {
		job.Status = StatusCompleted
		job.Progress = 1.0
	}
	job.UpdatedAt = time.Now()
	m.mu.Unlock()

	if m.onComplete != nil {
		m.onComplete(job.ID, err)
	}
}
