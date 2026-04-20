package casemgr

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nlink-jp/data-agent/internal/dbengine"
)

// CaseInfo represents a data analysis case.
// Named CaseInfo (not Case) to avoid JavaScript reserved word conflict in Wails bindings.
type CaseInfo struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    Status    `json:"status"`
}

// Status represents case lifecycle state.
type Status string

const (
	StatusOpen   Status = "open"
	StatusClosed Status = "closed"
)

// openCase tracks a currently open case with its DB engine.
type openCase struct {
	meta   CaseInfo
	engine *dbengine.Engine
	refCnt int32 // atomic: background job reference count
}

// Manager manages case lifecycle and DB instances.
type Manager struct {
	baseDir string
	open    map[string]*openCase
	mu      sync.RWMutex
}

// NewManager creates a Manager rooted at the given directory.
func NewManager(baseDir string) (*Manager, error) {
	casesDir := filepath.Join(baseDir, "cases")
	if err := os.MkdirAll(casesDir, 0o700); err != nil {
		return nil, fmt.Errorf("create cases dir: %w", err)
	}
	return &Manager{
		baseDir: baseDir,
		open:    make(map[string]*openCase),
	}, nil
}

// ResetGhostStatus resets cases that were left "open" from an unclean shutdown.
// On startup, no cases should be in the in-memory open map, so any "open" status
// in meta.json is stale.
// ResetGhostStatus resets cases left "open" from unclean shutdown. Returns count reset.
func (m *Manager) ResetGhostStatus() int {
	cases, err := m.List()
	if err != nil {
		return 0
	}
	count := 0
	for _, c := range cases {
		if c.Status == StatusOpen {
			c.Status = StatusClosed
			c.UpdatedAt = time.Now()
			m.saveMeta(&c)
			count++
		}
	}
	return count
}

// Create creates a new case with the given name.
func (m *Manager) Create(name string) (*CaseInfo, error) {
	c := &CaseInfo{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Status:    StatusClosed,
	}

	caseDir := m.caseDir(c.ID)
	for _, sub := range []string{"sessions", "reports"} {
		if err := os.MkdirAll(filepath.Join(caseDir, sub), 0o700); err != nil {
			return nil, fmt.Errorf("create case dir: %w", err)
		}
	}

	if err := m.saveMeta(c); err != nil {
		return nil, err
	}
	return c, nil
}

// List returns all cases (open and closed).
func (m *Manager) List() ([]CaseInfo, error) {
	casesDir := filepath.Join(m.baseDir, "cases")
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var cases []CaseInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := m.loadMeta(e.Name())
		if err != nil {
			continue
		}
		cases = append(cases, *c)
	}
	return cases, nil
}

// Open opens a case, creating its DB engine instance.
func (m *Manager) Open(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.open[id]; exists {
		return nil // already open
	}

	c, err := m.loadMeta(id)
	if err != nil {
		return fmt.Errorf("load case %s: %w", id, err)
	}

	dbPath := filepath.Join(m.caseDir(id), "data.duckdb")
	engine, err := dbengine.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db for case %s: %w", id, err)
	}

	c.Status = StatusOpen
	c.UpdatedAt = time.Now()
	if err := m.saveMeta(c); err != nil {
		engine.Close()
		return err
	}

	m.open[id] = &openCase{
		meta:   *c,
		engine: engine,
	}
	return nil
}

// Close closes a case, releasing its DB engine.
// Returns an error if background jobs are still running.
func (m *Manager) Close(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oc, exists := m.open[id]
	if !exists {
		return nil // already closed
	}

	if atomic.LoadInt32(&oc.refCnt) > 0 {
		return fmt.Errorf("case %s has running background jobs", id)
	}

	if err := oc.engine.Close(); err != nil {
		return fmt.Errorf("close db for case %s: %w", id, err)
	}

	oc.meta.Status = StatusClosed
	oc.meta.UpdatedAt = time.Now()
	m.saveMeta(&oc.meta)

	delete(m.open, id)
	return nil
}

// Delete removes a case entirely. The case must be closed first.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.open[id]; exists {
		return fmt.Errorf("cannot delete open case %s", id)
	}

	return os.RemoveAll(m.caseDir(id))
}

// Engine returns the DB engine for an open case.
func (m *Manager) Engine(id string) (*dbengine.Engine, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	oc, exists := m.open[id]
	if !exists {
		return nil, fmt.Errorf("case %s is not open", id)
	}
	return oc.engine, nil
}

// AcquireRef increments the reference count for background jobs.
func (m *Manager) AcquireRef(id string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	oc, exists := m.open[id]
	if !exists {
		return fmt.Errorf("case %s is not open", id)
	}
	atomic.AddInt32(&oc.refCnt, 1)
	return nil
}

// ReleaseRef decrements the reference count for background jobs.
func (m *Manager) ReleaseRef(id string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if oc, exists := m.open[id]; exists {
		atomic.AddInt32(&oc.refCnt, -1)
	}
}

// GetMeta returns the metadata for a case.
func (m *Manager) GetMeta(id string) (*CaseInfo, error) {
	m.mu.RLock()
	if oc, exists := m.open[id]; exists {
		m.mu.RUnlock()
		c := oc.meta
		return &c, nil
	}
	m.mu.RUnlock()
	return m.loadMeta(id)
}

// CaseDir returns the directory path for a case.
func (m *Manager) CaseDir(id string) string {
	return m.caseDir(id)
}

func (m *Manager) caseDir(id string) string {
	return filepath.Join(m.baseDir, "cases", id)
}

func (m *Manager) saveMeta(c *CaseInfo) error {
	path := filepath.Join(m.caseDir(c.ID), "meta.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal case meta: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func (m *Manager) loadMeta(id string) (*CaseInfo, error) {
	path := filepath.Join(m.caseDir(id), "meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c CaseInfo
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse case meta: %w", err)
	}
	return &c, nil
}
