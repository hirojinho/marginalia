package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSandboxManagerCreateBuildsStructure(t *testing.T) {
	vaultRoot := t.TempDir()
	sm := NewSandboxManager(vaultRoot)

	path, err := sm.Create(42, "", "", "", "study")
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

	path1, err := sm.Create(7, "", "", "", "study")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	path2, err := sm.Create(7, "", "", "", "study")
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if path1 != path2 {
		t.Errorf("idempotent: got %q and %q", path1, path2)
	}
}

func TestSandboxManagerPathMatchesCreate(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, err := sm.Create(99, "", "", "", "study")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if sm.Path(99) != path {
		t.Errorf("Path(99) = %q, Create returned %q", sm.Path(99), path)
	}
}

func TestSandboxManagerDeleteRemovesDir(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, _ := sm.Create(5, "", "", "", "study")

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
	_, _ = sm.Create(1, "", "", "", "study")
	_, _ = sm.Create(2, "", "", "", "study")

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
	_, _ = sm.Create(3, "", "", "", "study")

	removed, err := sm.Sweep(7)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if removed != 0 {
		t.Errorf("Sweep removed %d fresh sandboxes, want 0", removed)
	}
}

func readAgentsMD(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	return string(b)
}

func TestWriteAgentsMDParameterizesSteering(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	sm.Settings = func(string) CourseSettings {
		return CourseSettings{
			CourseID: "ce297", Framing: "exam-prep first", ExamStyle: "conceptual oral",
			ChunkPages: 6, StopAfterTask: false, Interleaving: false,
		}
	}
	dir, err := sm.Create(1, "", "ce297", "eduardo", "study")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	md := readAgentsMD(t, dir)
	for _, want := range []string{"~6 pages", "Chunk by meaning", "Stop-after-task is OFF", "exam-prep first", "conceptual oral", "How to teach this course"} {
		if !strings.Contains(md, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
	if strings.Contains(md, "interleaved spaced retrieval") {
		t.Errorf("interleaving clause should be absent when Interleaving=false")
	}
}

func TestWriteAgentsMDUsesDefaultsWhenNoProvider(t *testing.T) {
	sm := NewSandboxManager(t.TempDir()) // Settings nil → defaults
	dir, err := sm.Create(2, "", "ce297", "eduardo", "study")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	md := readAgentsMD(t, dir)
	for _, want := range []string{"~8 pages", "Chunk by meaning", "Stop-after-task is ON", "interleaved spaced retrieval"} {
		if !strings.Contains(md, want) {
			t.Errorf("AGENTS.md missing default %q", want)
		}
	}
	if strings.Contains(md, "How to teach this course") {
		t.Errorf("framing section should be absent when framing/exam_style empty")
	}
}

func TestWriteAgentsMDIncludesPDFSection(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, err := sm.Create(42, "", "", "", "study")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(path, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read AGENTS.md: %v", err)
	}
	body := string(data)
	for _, want := range []string{
		"## Slides / PDFs",
		"claw-cli pdf current --session 42",
		"Never reconstruct",
		"claw-cli pdf extract",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("AGENTS.md missing %q", want)
		}
	}
}

func TestWriteAgentsMDAuthoringFrame(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, err := sm.Create(101, "", "", "", "authoring")
	if err != nil {
		t.Fatalf("create authoring: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(path, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "course create --session") {
		t.Fatalf("authoring frame missing the create+retag instruction:\n%s", body)
	}
	path2, err := sm.Create(102, "", "ce297", "", "study")
	if err != nil {
		t.Fatalf("create study: %v", err)
	}
	body2, err := os.ReadFile(filepath.Join(path2, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read2: %v", err)
	}
	if strings.Contains(string(body2), "course create --session") {
		t.Fatalf("study session should not have the authoring frame:\n%s", body2)
	}
}
