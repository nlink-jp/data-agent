package job

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestSubmitAndComplete(t *testing.T) {
	var completedID string
	var completedErr error
	var wg sync.WaitGroup
	wg.Add(1)

	mgr := NewManager(nil, func(id string, err error) {
		completedID = id
		completedErr = err
		wg.Done()
	})

	id := mgr.Submit("case-1", TypeSession, func(ctx context.Context, setProgress func(float64)) error {
		setProgress(0.5)
		setProgress(1.0)
		return nil
	})

	wg.Wait()

	if completedID != id {
		t.Errorf("completed ID = %q, want %q", completedID, id)
	}
	if completedErr != nil {
		t.Errorf("completed error = %v, want nil", completedErr)
	}

	job, err := mgr.Get(id)
	if err != nil {
		t.Fatal(err)
	}
	if job.Status != StatusCompleted {
		t.Errorf("status = %q, want %q", job.Status, StatusCompleted)
	}
	if job.Progress != 1.0 {
		t.Errorf("progress = %f, want 1.0", job.Progress)
	}
}

func TestSubmitAndFail(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	mgr := NewManager(nil, func(id string, err error) {
		wg.Done()
	})

	id := mgr.Submit("case-1", TypeSession, func(ctx context.Context, setProgress func(float64)) error {
		return fmt.Errorf("analysis failed")
	})

	wg.Wait()

	job, _ := mgr.Get(id)
	if job.Status != StatusFailed {
		t.Errorf("status = %q, want %q", job.Status, StatusFailed)
	}
	if job.Error != "analysis failed" {
		t.Errorf("error = %q, want %q", job.Error, "analysis failed")
	}
}

func TestCancel(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	mgr := NewManager(nil, func(id string, err error) {
		wg.Done()
	})

	id := mgr.Submit("case-1", TypeSession, func(ctx context.Context, setProgress func(float64)) error {
		<-ctx.Done()
		return ctx.Err()
	})

	// Give the goroutine time to start
	time.Sleep(10 * time.Millisecond)

	if err := mgr.Cancel(id); err != nil {
		t.Fatal(err)
	}

	wg.Wait()

	job, _ := mgr.Get(id)
	if job.Status != StatusCancelled {
		t.Errorf("status = %q, want %q", job.Status, StatusCancelled)
	}
}

func TestProgressCallback(t *testing.T) {
	var progressValues []float64
	var mu sync.Mutex
	var wg sync.WaitGroup
	wg.Add(1)

	mgr := NewManager(
		func(id string, p float64) {
			mu.Lock()
			progressValues = append(progressValues, p)
			mu.Unlock()
		},
		func(id string, err error) {
			wg.Done()
		},
	)

	mgr.Submit("case-1", TypeSession, func(ctx context.Context, setProgress func(float64)) error {
		setProgress(0.25)
		setProgress(0.50)
		setProgress(0.75)
		return nil
	})

	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(progressValues) != 3 {
		t.Errorf("progress callbacks = %d, want 3", len(progressValues))
	}
}

func TestList(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(3)

	mgr := NewManager(nil, func(id string, err error) {
		wg.Done()
	})

	mgr.Submit("case-1", TypeSession, func(ctx context.Context, setProgress func(float64)) error {
		return nil
	})
	mgr.Submit("case-1", TypeSlidingWindow, func(ctx context.Context, setProgress func(float64)) error {
		return nil
	})
	mgr.Submit("case-2", TypeSession, func(ctx context.Context, setProgress func(float64)) error {
		return nil
	})

	wg.Wait()

	jobs := mgr.List("case-1")
	if len(jobs) != 2 {
		t.Errorf("case-1 jobs = %d, want 2", len(jobs))
	}
}

func TestGetNotFound(t *testing.T) {
	mgr := NewManager(nil, nil)
	_, err := mgr.Get("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent job")
	}
}

func TestCancelNotFound(t *testing.T) {
	mgr := NewManager(nil, nil)
	err := mgr.Cancel("nonexistent")
	if err == nil {
		t.Error("expected error for cancelling nonexistent job")
	}
}
