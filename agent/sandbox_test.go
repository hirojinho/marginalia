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
	for _, want := range []string{"~8 pages", "Chunk by meaning", "Stop-after-task is ON", "retrieve due"} {
		if !strings.Contains(md, want) {
			t.Errorf("AGENTS.md missing default %q", want)
		}
	}
	if strings.Contains(md, "How to teach this course") {
		t.Errorf("framing section should be absent when framing/exam_style empty")
	}
}

func TestWriteAgentsMDIncludesToolHonesty(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	dir, err := sm.Create(7, "", "ddia", "eduardo", "study")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	md := readAgentsMD(t, dir)
	for _, want := range []string{
		"Tool use — report results honestly (MANDATORY)",
		"A failed command is not a benign result",
		"is **not** \"nothing due\"",
		"Never narrate state you have not verified",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("AGENTS.md missing tool-honesty clause %q", want)
		}
	}
}

// TestWriteAgentsMDConceptLevelRules guards the ADR 0019 prompt edits: scoring
// keys on an atom (not a task), atom capture is search-first, completion is an
// atomicity gate, and the tutor must not read the next task before stopping.
func TestWriteAgentsMDConceptLevelRules(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	dir, err := sm.Create(9, "", "ddia", "eduardo", "study")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	md := readAgentsMD(t, dir)
	for _, want := range []string{
		"(NEVER a task id)",                     // Rule 3 keys on the atom id
		"knowledge search",                      // Rule 9 + knowledge section: search-before-create
		"the atomicity gate",                    // Rule 9: completion model
		"before the current one is marked done", // Rule 10: no next-task preview/extract
		"Atomicity gate:",                       // steering note retired the mastery threshold
	} {
		if !strings.Contains(md, want) {
			t.Errorf("AGENTS.md missing concept-level clause %q", want)
		}
	}
	if strings.Contains(md, "Mastery gate:** `claw-cli plan toggle` will refuse with a \"mastery gate\"") {
		t.Errorf("retired mastery-gate steering note still present")
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

func TestRule3UsesClawCLIConfidenceLog(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	if !strings.Contains(out, "claw-cli confidence log") {
		t.Fatalf("Rule 3 must instruct running 'claw-cli confidence log'; got:\n%s", out)
	}
	if strings.Contains(out, "call the log_confidence tool") {
		t.Fatalf("Rule 3 still references the unreachable 'log_confidence tool'")
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
	if !strings.Contains(string(body), "course create --session 101") {
		t.Fatalf("authoring frame missing the create+retag instruction with session id:\n%s", body)
	}
	if strings.Contains(string(body), "Pedagogical Rules (MANDATORY") {
		t.Fatalf("course-less authoring must NOT carry pedagogy rules:\n%s", body)
	}
	if strings.Contains(string(body), "Course settings (Steering)") {
		t.Fatalf("course-less authoring must NOT carry steering tool section:\n%s", body)
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
	path3, err := sm.Create(103, "", "", "", "scratch")
	if err != nil {
		t.Fatalf("create scratch: %v", err)
	}
	body3, err := os.ReadFile(filepath.Join(path3, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read3: %v", err)
	}
	if strings.Contains(string(body3), "course create --session") {
		t.Fatalf("scratch session should not have the authoring frame:\n%s", body3)
	}
}

func TestWriteAgentsMDExistingCourseAuthoringFrame(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, err := sm.Create(201, "", "ce297", "", "authoring")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(path, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s := string(body)
	if !strings.Contains(s, "plan rewrite --course ce297") {
		t.Fatalf("existing-course authoring frame should reference plan rewrite for the course:\n%s", s)
	}
	if strings.Contains(s, "course create --session") {
		t.Fatalf("existing-course authoring must NOT tell the agent to create a course:\n%s", s)
	}
	if strings.Contains(s, "Pedagogical Rules (MANDATORY") {
		t.Fatalf("authoring session must NOT carry the study pedagogy rules:\n%s", s)
	}
	if strings.Contains(s, "Course settings (Steering)") {
		t.Fatalf("authoring session must NOT carry the steering tool section:\n%s", s)
	}
}

func TestWriteAgentsMDStudyKeepsPedagogy(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, err := sm.Create(202, "", "ce297", "", "study")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(path, "AGENTS.md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "Pedagogical Rules (MANDATORY") {
		t.Fatalf("study session must still carry the pedagogy rules (gating over-reached):\n%s", body)
	}
}

func TestAgentsMDMentionsAtomicityGate(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	// ADR 0019: completion is gated on distilling an atom, not on a confidence
	// threshold. The mastery_threshold setting still exists (harmless) but no
	// longer gates completion, so the agent is told about the atomicity gate.
	if !strings.Contains(out, "Atomicity gate:") {
		t.Fatalf("must explain the plan-toggle atomicity gate to the agent")
	}
	if strings.Contains(out, "refuse with a \"mastery gate\"") {
		t.Fatalf("retired mastery-gate wording must be gone")
	}
}

func TestPedagogyHasTwoStepReveal(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	if !strings.Contains(out, "cue — don't complete") {
		t.Fatalf("pedagogy rules must include the two-step-reveal rule ('cue — don't complete')")
	}
}

func TestRule3ScoresRetrievalNotSelfRating(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	if strings.Contains(out, "How confident are you") {
		t.Fatalf("Rule 3 must NOT ask for a self-rating")
	}
	if !strings.Contains(out, "key idea-units") {
		t.Fatalf("Rule 3 must instruct scoring the recall by key idea-units")
	}
	if !strings.Contains(out, "claw-cli confidence log") {
		t.Fatalf("Rule 3 must still log via claw-cli confidence log")
	}
}

// TestRule6OneLightOpener guards ADR 0020: the opener is ONE understanding-first
// move; an empty queue routes to the Rule 7 prediction (no stacked recall round).
func TestRule6OneLightOpener(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	for _, want := range []string{
		"ONE light move",                // one situational opener, not three stacked phases
		"understanding-first",           // why/how/when, not enumeration
		"give me your own example",      // generate, don't recite the source's example
		"the prediction IS your opener", // empty queue → Rule 7, no separate recall
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("Rule 6 (ADR 0020) missing %q", want)
		}
	}
	// The retired "empty queue still forces a recall" wording must be gone.
	if strings.Contains(out, "does NOT license skipping") {
		t.Fatalf("retired forced-recall-on-empty-queue wording still present")
	}
}

// TestBoundaryRecallIsUnderstandingFirst guards the ADR-0020-era polish: the
// per-chunk boundary recall is ONE understanding-first question, not a
// "full recall scored per Rule 3" list-everything dump.
func TestBoundaryRecallIsUnderstandingFirst(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	// The retired "list everything" boundary-recall wording must be gone.
	if strings.Contains(out, "ending with a full recall scored per Rule 3") {
		t.Fatalf("boundary recall must no longer be a full list-everything recall")
	}
	// The boundary recall must now be one understanding-first question.
	if !strings.Contains(out, "the boundary recall is ONE understanding-first question") {
		t.Fatalf("boundary recall must be ONE understanding-first question")
	}
	// The one-cue cap must cover boundary recalls, not just the opener.
	if !strings.Contains(out, "At the session opener AND at each boundary recall, cap this at ONE cue") {
		t.Fatalf("one-cue cap must extend to boundary recalls (Rule 11)")
	}
	// The capture rule must tell the agent to fetch the task id first.
	if !strings.Contains(out, "claw-cli plan active --course") {
		t.Fatalf("capture rule must instruct getting the active task id via `plan active`")
	}
}
