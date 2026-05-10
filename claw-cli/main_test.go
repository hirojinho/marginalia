package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-app/agent"
)

func newTempDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "study.db")
	db, err := agent.OpenDB(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	db.Close()
	return path
}

func TestRunUnknownSubcommandExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "wat"}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit code: %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown") {
		t.Fatalf("stderr: %s", stderr.String())
	}
}

func TestRunMemorySaveJSONOutput(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
		"--course", "ce297",
		"--title", "no abbreviations",
		"--body", "spell out Software Control Category not SCC",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout.String())
	}
	if got["kind"] != "feedback" || got["title"] != "no abbreviations" {
		t.Fatalf("got %+v", got)
	}
	if id, ok := got["id"].(float64); !ok || id == 0 {
		t.Fatalf("expected non-zero id, got %v", got["id"])
	}
}

func TestRunMemorySaveBodyFromStdin(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("body from stdin\nsecond line")
	code := runWithStdin([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
		"--title", "stdin-test",
		"--body", "-",
	}, stdin, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
}

func TestRunMissingRequiredFlagExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
	}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("exit code: %d", code)
	}
}

var _ = os.Stdout

func TestRunMemorySearchReturnsResults(t *testing.T) {
	dbPath := newTempDB(t)
	var sb, eb bytes.Buffer
	for _, body := range []string{"density rule", "abbreviations rule", "unrelated text"} {
		sb.Reset()
		eb.Reset()
		code := run([]string{
			"clawcli", "memory", "save",
			"--kind", "feedback", "--title", body, "--body", body,
		}, &sb, &eb, dbPath)
		if code != 0 {
			t.Fatalf("seed: %s", eb.String())
		}
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "search", "--query", "rule",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	var got struct {
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout.String())
	}
	if len(got.Results) != 2 {
		t.Fatalf("expected 2 hits, got %d:\n%s", len(got.Results), stdout.String())
	}
}

func TestRunMemorySearchMissingQueryExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "memory", "search"}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("exit: %d", code)
	}
}

func TestRunMemoryLoadProducesAgentsMD(t *testing.T) {
	dbPath := newTempDB(t)
	for _, args := range [][]string{
		{"--kind", "profile", "--title", "user", "--body", "Eduardo studies safety at ITA"},
		{"--kind", "project", "--course", "ce297", "--title", "course-arc", "--body", "STAMP vs Avizienis"},
		{"--kind", "feedback", "--course", "ce297", "--title", "no-abbrev", "--body", "spell out Software Control Category"},
		{"--kind", "feedback", "--title", "density", "--body", "match existing density"},
	} {
		var sb, eb bytes.Buffer
		full := append([]string{"clawcli", "memory", "save"}, args...)
		if code := run(full, &sb, &eb, dbPath); code != 0 {
			t.Fatalf("seed: %s", eb.String())
		}
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "load", "--course", "ce297", "--user", "eduardo",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"# AGENTS.md", "## User profile", "Eduardo studies safety",
		"## Course context: ce297", "STAMP",
		"## Active feedback rules", "no-abbrev", "density",
		"## Available skills", "_(none yet)_",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
	if len(out) > 3072 {
		t.Fatalf("over cap: %d", len(out))
	}
}

func TestRunMemoryLoadEmptyDBStillProducesShell(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "memory", "load", "--course", "ce297"}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# AGENTS.md") {
		t.Fatalf("expected AGENTS.md header even on empty db")
	}
}

func TestRunMemoryLoadResolvesDBFromEnvRoot(t *testing.T) {
	root := t.TempDir()
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	dbPath := filepath.Join(dataDir, "study.db")
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("seed open: %v", err)
	}
	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("seed init: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	t.Setenv("CLAW_STUDY_ROOT", root)
	t.Setenv("CLAW_STUDY_DB", "")
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "memory", "load", "--course", "ce297"}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "# AGENTS.md") {
		t.Fatalf("expected AGENTS.md output, got: %s", stdout.String())
	}
}

func TestRunMemoryLoadErrorsOnExplicitMissingDB(t *testing.T) {
	t.Setenv("CLAW_STUDY_ROOT", "")
	t.Setenv("CLAW_STUDY_DB", "")
	missing := filepath.Join(t.TempDir(), "missing.db")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "load", "--course", "ce297", "--db", missing,
	}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "database not found") || !strings.Contains(stderr.String(), missing) {
		t.Fatalf("expected database-not-found error mentioning %q, got: %s", missing, stderr.String())
	}
}

func TestRunMemoryLoadErrorsOnExplicitMissingSkillsDir(t *testing.T) {
	dbPath := newTempDB(t)
	missing := filepath.Join(t.TempDir(), "no-such-skills")
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "load", "--course", "ce297",
		"--db", dbPath, "--skills-dir", missing,
	}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "skills directory not found") || !strings.Contains(stderr.String(), missing) {
		t.Fatalf("expected skills-not-found error, got: %s", stderr.String())
	}
}

func TestRunMemoryLoadDefaultSkillsDirMissingIsOK(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("CLAW_STUDY_ROOT", t.TempDir()) // root has no skills/ subdir
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "load", "--course", "ce297", "--db", dbPath,
	}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "_(none yet)_") {
		t.Fatalf("expected skill section fallback, got: %s", stdout.String())
	}
}

func TestRunRagSearchRequiresAPIKey(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("OPENCODE_API_KEY", "")
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "rag", "search", "--query", "x", "--db", dbPath}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "LLM_API_KEY") {
		t.Fatalf("expected LLM_API_KEY error, got: %s", stderr.String())
	}
}

func TestRunRagSearchMissingQueryExits2(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("LLM_API_KEY", "stub")
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "rag", "search", "--db", dbPath}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestRunPlanShowEmptyPlanReturnsError(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir()) // empty vault → no plan files
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "plan", "show", "--course", "ce297", "--db", dbPath}, &stdout, &stderr, "")
	// Plan not found is a soft error: exit 1 with a clear message.
	if code != 1 {
		t.Fatalf("exit %d, want 1; stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "plan not found") {
		t.Fatalf("expected plan-not-found error, got: %s", stderr.String())
	}
}

func TestRunPlanToggleMissingTaskExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "plan", "toggle", "--course", "ce297", "--db", dbPath}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (missing --task)", code)
	}
	if !strings.Contains(stderr.String(), "--task") {
		t.Fatalf("expected missing-task error, got: %s", stderr.String())
	}
}

func TestRunCourseInterestsReturnsFile(t *testing.T) {
	dbPath := newTempDB(t)
	vault := t.TempDir()
	dir := filepath.Join(vault, "memory", "courses", "ce297")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	body := "# CE-297 interests\n\nFormal methods angle on safety.\n"
	if err := os.WriteFile(filepath.Join(dir, "interests.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VAULT_ROOT", vault)

	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "course", "interests", "--course", "ce297", "--db", dbPath}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Formal methods angle on safety") {
		t.Fatalf("expected file contents in stdout, got: %s", stdout.String())
	}
}

func TestRunCourseInterestsMissingFileExits1(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "course", "interests", "--course", "no-such", "--db", dbPath}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}
