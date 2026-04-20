package casemgr

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	dir := t.TempDir()
	mgr, err := NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	return mgr
}

func TestCreateAndList(t *testing.T) {
	mgr := newTestManager(t)

	c1, err := mgr.Create("Case Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if c1.Name != "Case Alpha" {
		t.Errorf("name = %q, want %q", c1.Name, "Case Alpha")
	}
	if c1.Status != StatusClosed {
		t.Errorf("status = %q, want %q", c1.Status, StatusClosed)
	}

	mgr.Create("Case Beta")

	cases, err := mgr.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) != 2 {
		t.Errorf("cases = %d, want 2", len(cases))
	}
}

func TestOpenAndClose(t *testing.T) {
	mgr := newTestManager(t)

	c, _ := mgr.Create("Test")

	if err := mgr.Open(c.ID); err != nil {
		t.Fatal(err)
	}

	// Verify status is open
	meta, _ := mgr.GetMeta(c.ID)
	if meta.Status != StatusOpen {
		t.Errorf("status = %q, want %q", meta.Status, StatusOpen)
	}

	// Engine should be available
	engine, err := mgr.Engine(c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if engine == nil {
		t.Error("engine should not be nil for open case")
	}

	if err := mgr.Close(c.ID); err != nil {
		t.Fatal(err)
	}

	// Engine should not be available after close
	_, err = mgr.Engine(c.ID)
	if err == nil {
		t.Error("expected error for engine of closed case")
	}
}

func TestOpenIdempotent(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.Create("Test")
	mgr.Open(c.ID)

	// Opening again should not error
	if err := mgr.Open(c.ID); err != nil {
		t.Fatal(err)
	}
	mgr.Close(c.ID)
}

func TestCloseBlockedByRefCount(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.Create("Test")
	mgr.Open(c.ID)

	mgr.AcquireRef(c.ID)

	err := mgr.Close(c.ID)
	if err == nil {
		t.Error("expected error when closing case with running jobs")
	}

	mgr.ReleaseRef(c.ID)

	if err := mgr.Close(c.ID); err != nil {
		t.Fatal(err)
	}
}

func TestDelete(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.Create("ToDelete")

	// Cannot delete open case
	mgr.Open(c.ID)
	if err := mgr.Delete(c.ID); err == nil {
		t.Error("expected error when deleting open case")
	}
	mgr.Close(c.ID)

	// Can delete closed case
	if err := mgr.Delete(c.ID); err != nil {
		t.Fatal(err)
	}

	cases, _ := mgr.List()
	if len(cases) != 0 {
		t.Errorf("cases = %d, want 0 after delete", len(cases))
	}
}

func TestCaseDirStructure(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.Create("Structure")

	dir := mgr.CaseDir(c.ID)

	// Check expected subdirectories
	for _, sub := range []string{"sessions", "reports"} {
		path := filepath.Join(dir, sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected dir %s: %v", sub, err)
		} else if !info.IsDir() {
			t.Errorf("%s should be a directory", sub)
		}
	}

	// Check meta.json exists
	if _, err := os.Stat(filepath.Join(dir, "meta.json")); err != nil {
		t.Errorf("meta.json should exist: %v", err)
	}
}

func TestImportDataViaEngine(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.Create("WithData")
	mgr.Open(c.ID)
	defer mgr.Close(c.ID)

	// Create test CSV
	csvPath := filepath.Join(t.TempDir(), "test.csv")
	os.WriteFile(csvPath, []byte("x,y\n1,2\n3,4\n"), 0o600)

	engine, _ := mgr.Engine(c.ID)
	if err := engine.Import(csvPath, "points"); err != nil {
		t.Fatal(err)
	}

	tables := engine.Tables()
	if len(tables) != 1 {
		t.Errorf("tables = %d, want 1", len(tables))
	}
}

func TestRefCountMultiple(t *testing.T) {
	mgr := newTestManager(t)
	c, _ := mgr.Create("RefTest")
	mgr.Open(c.ID)

	mgr.AcquireRef(c.ID)
	mgr.AcquireRef(c.ID)

	// Should still fail with 2 refs
	if err := mgr.Close(c.ID); err == nil {
		t.Error("expected error with 2 refs")
	}

	mgr.ReleaseRef(c.ID)
	if err := mgr.Close(c.ID); err == nil {
		t.Error("expected error with 1 ref")
	}

	mgr.ReleaseRef(c.ID)
	if err := mgr.Close(c.ID); err != nil {
		t.Fatal(err)
	}
}
