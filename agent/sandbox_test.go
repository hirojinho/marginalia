package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSandboxManagerCreateBuildsStructure(t *testing.T) {
	vaultRoot := t.TempDir()
	sm := NewSandboxManager(vaultRoot)

	path, err := sm.Create(42, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, rel := range []string{"notes", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(path, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}

	// out symlink should resolve to data/agent-out
	outLink := filepath.Join(path, "out")
	target, err := os.Readlink(outLink)
	if err != nil {
		t.Fatalf("readlink out: %v", err)
	}
	expected := filepath.Join(vaultRoot, "data", "agent-out")
	if target != expected {
		t.Errorf("out symlink = %q, want %q", target, expected)
	}
}

func TestSandboxManagerCreateIsIdempotent(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())

	path1, err := sm.Create(7, "", "", "")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	path2, err := sm.Create(7, "", "", "")
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if path1 != path2 {
		t.Errorf("idempotent: got %q and %q", path1, path2)
	}
}

func TestSandboxManagerPathMatchesCreate(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, _ := sm.Create(99, "", "", "")
	if sm.Path(99) != path {
		t.Errorf("Path(99) = %q, Create returned %q", sm.Path(99), path)
	}
}

func TestSandboxManagerDeleteRemovesDir(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, _ := sm.Create(5, "", "", "")

	if err := sm.Delete(5); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("sandbox dir still exists after Delete")
	}
}

func TestSandboxManagerDeleteMissingIsNoop(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	if err := sm.Delete(999); err != nil {
		t.Errorf("Delete of non-existent sandbox: %v", err)
	}
}

func TestSandboxManagerSweepRemovesStale(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())

	// Create two sandboxes.
	_, _ = sm.Create(1, "", "", "")
	_, _ = sm.Create(2, "", "", "")

	// Back-date AGENTS.md for sandbox 1 to simulate staleness.
	staleTime := time.Now().Add(-8 * 24 * time.Hour)
	agentsMD := filepath.Join(sm.Path(1), "AGENTS.md")
	if err := os.Chtimes(agentsMD, staleTime, staleTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	removed, err := sm.Sweep(7)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if removed != 1 {
		t.Errorf("Sweep removed %d, want 1", removed)
	}

	// Sandbox 2 must still exist.
	if _, err := os.Stat(sm.Path(2)); err != nil {
		t.Errorf("sandbox 2 missing after sweep: %v", err)
	}
	// Sandbox 1 must be gone.
	if _, err := os.Stat(sm.Path(1)); !os.IsNotExist(err) {
		t.Errorf("stale sandbox 1 still exists after sweep")
	}
}

func TestSandboxManagerSweepKeepsFresh(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	_, _ = sm.Create(3, "", "", "")

	removed, err := sm.Sweep(7)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if removed != 0 {
		t.Errorf("Sweep removed %d fresh sandboxes, want 0", removed)
	}
}
