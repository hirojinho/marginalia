package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
	code := run([]string{"clawcli", "course", "interests", "--course", "ce297"}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Formal methods angle on safety") {
		t.Fatalf("expected file contents in stdout, got: %s", stdout.String())
	}
}

func TestRunCourseInterestsMissingFileExits1(t *testing.T) {
	t.Setenv("VAULT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "course", "interests", "--course", "no-such"}, &stdout, &stderr, "")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
}

func TestRunCourseInterestsMissingCourseExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "course", "interests"}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (missing --course)", code)
	}
	if !strings.Contains(stderr.String(), "--course") {
		t.Fatalf("expected --course error in stderr, got: %s", stderr.String())
	}
}

func TestRunNoteSaveWritesFile(t *testing.T) {
	dbPath := newTempDB(t)
	vault := t.TempDir()
	t.Setenv("VAULT_ROOT", vault)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "note", "save", "--course", "ce297",
		"--kind", "fleeting", "--content", "test note from CLI",
		"--db", dbPath,
	}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	// Confirm a file was written somewhere under vault/memory/courses/ce297/fleeting/
	matches, _ := filepath.Glob(filepath.Join(vault, "memory", "courses", "ce297", "fleeting", "*.md"))
	if len(matches) == 0 {
		t.Fatalf("expected fleeting note written under vault, found none")
	}
}

func TestRunPDFExtractInvalidIDReturnsErrorString(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "pdf", "extract", "--id", "999", "--db", dbPath}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	// stdout will contain whatever the tool returns for missing PDFs — likely an "error" string,
	// but the CLI plumbing exits 0 because it's the tool's text output, not a CLI failure.
	if stdout.Len() == 0 {
		t.Fatalf("expected some stdout output")
	}
}

func TestRunWebFetchOK(t *testing.T) {
	if testing.Short() {
		t.Skip("network test")
	}
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "web", "fetch", "--db", dbPath, "https://example.com"}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Example Domain") {
		t.Fatalf("expected example.com content, got: %s", stdout.String())
	}
}

func TestRunWebFetchMissingURLExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "web", "fetch", "--db", dbPath}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (missing URL)", code)
	}
}

func TestRunNoteSaveMissingFlagsExits2(t *testing.T) {
	dbPath := newTempDB(t)
	// missing --content
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "note", "save", "--course", "ce297",
		"--db", dbPath,
	}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2 (missing --content)", code)
	}
	if !strings.Contains(stderr.String(), "--content") {
		t.Fatalf("expected --content in stderr, got: %s", stderr.String())
	}
}

func TestRunSkillDispatchReturnsPrompt(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "skill", "dispatch",
		"--skill", "orientation", "--topic", "STAMP", "--course", "ce297",
		"--db", dbPath,
	}, &stdout, &stderr, "")
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if len(out) == 0 {
		t.Fatalf("expected non-empty prompt output")
	}
	if !strings.Contains(out, "STAMP") {
		t.Fatalf("expected STAMP in output, got: %s", out)
	}
}

func TestRunSkillDispatchMissingFlagsExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "skill", "dispatch", "--db", dbPath}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func seedPDF(t *testing.T, dbPath string, id int, name, lastReadAt string) {
	t.Helper()
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	var lra any
	if lastReadAt != "" {
		lra = lastReadAt
	}
	_, err = db.Exec(
		"INSERT INTO pdfs (id, filename, original_name, course_id, pages, last_page, last_read_at, uploaded_at) VALUES (?,?,?,?,?,?,?,?)",
		id, fmt.Sprintf("%d.pdf", id), name, nil, 10, 1, lra, "2026-05-01T00:00:00Z",
	)
	if err != nil {
		t.Fatalf("seed pdf: %v", err)
	}
}

func openApp(t *testing.T, dbPath string) *agent.App {
	t.Helper()
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	return agent.NewApp(agent.Config{VaultRoot: t.TempDir()}, db)
}

func TestPDFListEmptyAndOrdered(t *testing.T) {
	dbPath := newTempDB(t)

	// Empty DB → {"pdfs": []}
	var out, errb bytes.Buffer
	if code := run([]string{"clawcli", "pdf", "list"}, &out, &errb, dbPath); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var empty struct {
		PDFs []map[string]any `json:"pdfs"`
	}
	if err := json.Unmarshal(out.Bytes(), &empty); err != nil {
		t.Fatalf("parse empty: %v\n%s", err, out.String())
	}
	if len(empty.PDFs) != 0 {
		t.Fatalf("want 0 pdfs, got %d", len(empty.PDFs))
	}

	// id1 read earlier, id2 read later, id3 never read.
	seedPDF(t, dbPath, 1, "older.pdf", "2026-05-10T10:00:00Z")
	seedPDF(t, dbPath, 2, "newer.pdf", "2026-05-20T10:00:00Z")
	seedPDF(t, dbPath, 3, "unread.pdf", "")

	out.Reset()
	errb.Reset()
	if code := run([]string{"clawcli", "pdf", "list"}, &out, &errb, dbPath); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var got struct {
		PDFs []struct {
			ID           int    `json:"id"`
			OriginalName string `json:"original_name"`
		} `json:"pdfs"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if len(got.PDFs) != 3 {
		t.Fatalf("want 3 pdfs, got %d", len(got.PDFs))
	}
	if got.PDFs[0].ID != 2 {
		t.Fatalf("want most-recently-read (id 2) first, got id %d", got.PDFs[0].ID)
	}
	if got.PDFs[2].ID != 3 {
		t.Fatalf("want unread (id 3) last, got id %d", got.PDFs[2].ID)
	}
}

func TestPDFCurrentSessionHit(t *testing.T) {
	dbPath := newTempDB(t)
	seedPDF(t, dbPath, 2, "ch8.pdf", "2026-05-20T10:00:00Z")

	var sessID int64
	func() {
		app := openApp(t, dbPath)
		defer func() { _ = app.Close() }()
		sess, err := app.CreateSession("ce297", "topic", "scratch")
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		sessID = sess.ID
		if _, err := app.DB.Exec("UPDATE sessions SET last_pdf_id = ? WHERE id = ?", 2, sessID); err != nil {
			t.Fatalf("set last_pdf_id: %v", err)
		}
		if err := app.SetLastOpenedPDF(99); err != nil { // competing global; session must win
			t.Fatalf("set last opened: %v", err)
		}
	}()

	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "pdf", "current", "--session", strconv.FormatInt(sessID, 10)}, &out, &errb, dbPath)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var got struct {
		ID           int    `json:"id"`
		OriginalName string `json:"original_name"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if got.ID != 2 {
		t.Fatalf("want session's pdf id 2, got %d", got.ID)
	}
}

func TestPDFCurrentFallbackToLastOpened(t *testing.T) {
	dbPath := newTempDB(t)
	seedPDF(t, dbPath, 5, "last.pdf", "2026-05-21T10:00:00Z")
	func() {
		app := openApp(t, dbPath)
		defer func() { _ = app.Close() }()
		if err := app.SetLastOpenedPDF(5); err != nil {
			t.Fatalf("set last opened: %v", err)
		}
	}()

	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "pdf", "current"}, &out, &errb, dbPath)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	var got struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, out.String())
	}
	if got.ID != 5 {
		t.Fatalf("want fallback id 5, got %d", got.ID)
	}
}

func TestPDFCurrentNoneOpen(t *testing.T) {
	dbPath := newTempDB(t)
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "pdf", "current"}, &out, &errb, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1, got %d (stderr: %s)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "no PDF is currently open") {
		t.Fatalf("stderr: %s", errb.String())
	}
}

func TestCourseSettingsSetAndGet(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "course", "settings", "set",
		"--course", "ce297", "--key", "chunk_pages", "--value", "6",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("set exit %d, stderr: %s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"clawcli", "course", "settings", "get", "--course", "ce297",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("get exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "chunk_pages: 6") {
		t.Fatalf("expected chunk_pages: 6 in output:\n%s", out)
	}
	if !strings.Contains(out, "stop_after_task: true") {
		t.Fatalf("expected default stop_after_task: true:\n%s", out)
	}
}

func TestCourseSettingsSetRejectsBadKey(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "course", "settings", "set",
		"--course", "ce297", "--key", "nope", "--value", "x",
	}, &stdout, &stderr, dbPath)
	if code == 0 {
		t.Fatalf("expected non-zero exit for bad key; stderr: %s", stderr.String())
	}
}

func TestCourseCreateInsertsRow(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "course", "create",
		"--id", "new-course", "--name", "Brand New Course",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("create exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Created course") {
		t.Fatalf("expected confirmation, got: %s", stdout.String())
	}
	app := openApp(t, dbPath)
	defer func() { _ = app.Close() }()
	c, err := app.GetCourse("new-course")
	if err != nil {
		t.Fatalf("GetCourse: %v", err)
	}
	if c.ID != "new-course" || c.Name != "Brand New Course" {
		t.Fatalf("course not persisted, got %+v", c)
	}
}

func TestCourseCreateWithSettings(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "course", "create",
		"--id", "framed-course", "--name", "Framed",
		"--framing", "exam-prep lens", "--exam-style", "short essays",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("create exit %d, stderr: %s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run([]string{
		"clawcli", "course", "settings", "get", "--course", "framed-course",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("get exit %d, stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "framing: exam-prep lens") {
		t.Fatalf("framing not persisted:\n%s", out)
	}
	if !strings.Contains(out, "exam_style: short essays") {
		t.Fatalf("exam_style not persisted:\n%s", out)
	}
}

func TestCourseCreateDuplicateExits1(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	run([]string{"clawcli", "course", "create", "--id", "dup-course", "--name", "First"}, &stdout, &stderr, dbPath)
	stdout.Reset()
	stderr.Reset()
	code := run([]string{"clawcli", "course", "create", "--id", "dup-course", "--name", "Second"}, &stdout, &stderr, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1 on duplicate, got %d", code)
	}
	if !strings.Contains(stderr.String(), "course already exists") {
		t.Fatalf("stderr: %s", stderr.String())
	}
}

func TestCourseCreateInvalidIDExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "course", "create", "--id", "Bad ID", "--name", "X"}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("want exit 2 on invalid id, got %d (stderr: %s)", code, stderr.String())
	}
}

func TestPlanRewriteValidFileCreatesAndPreservesIDs(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir()) // plans live under VAULT_ROOT/data/plans/
	planJSON := `{"id":"rw-course","name":"RW","phases":[{"title":"P1","tasks":[` +
		`{"id":"keep-123","title":"Existing","done":false},` +
		`{"title":"Fresh","done":false}]}]}`
	planFile := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(planFile, []byte(planJSON), 0o644); err != nil {
		t.Fatalf("write plan file: %v", err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course", "--plan-file", planFile}, &out, &errb, dbPath)
	if code != 0 {
		t.Fatalf("rewrite exit %d, stderr: %s", code, errb.String())
	}
	out.Reset()
	errb.Reset()
	if code := run([]string{"clawcli", "plan", "show", "--course", "rw-course"}, &out, &errb, dbPath); code != 0 {
		t.Fatalf("show exit %d, stderr: %s", code, errb.String())
	}
	var shown struct {
		Phases []struct {
			Tasks []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"tasks"`
		} `json:"phases"`
	}
	if err := json.Unmarshal(out.Bytes(), &shown); err != nil {
		t.Fatalf("parse show output: %v\n%s", err, out.String())
	}
	if len(shown.Phases) != 1 || len(shown.Phases[0].Tasks) != 2 {
		t.Fatalf("unexpected plan shape: %+v", shown)
	}
	var existingID, freshID string
	for _, tk := range shown.Phases[0].Tasks {
		switch tk.Title {
		case "Existing":
			existingID = tk.ID
		case "Fresh":
			freshID = tk.ID
		}
	}
	if existingID != "keep-123" {
		t.Fatalf("explicit id not preserved, got %q", existingID)
	}
	if freshID == "" || freshID == "keep-123" {
		t.Fatalf("new task did not get a fresh uuid, got %q", freshID)
	}
}

func TestPlanRewriteBadJSONExits1(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	planFile := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(planFile, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course", "--plan-file", planFile}, &out, &errb, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1 on bad JSON, got %d", code)
	}
	if !strings.Contains(errb.String(), "failed to parse") {
		t.Fatalf("stderr: %s", errb.String())
	}
}

func TestPlanRewriteIDMismatchExits1(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	planFile := filepath.Join(t.TempDir(), "mismatch.json")
	if err := os.WriteFile(planFile, []byte(`{"id":"other","name":"X","phases":[]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course", "--plan-file", planFile}, &out, &errb, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1 on id mismatch, got %d (stderr: %s)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "does not match") {
		t.Fatalf("expected id-mismatch message, stderr: %s", errb.String())
	}
}

func TestPlanRewriteMissingFlagsExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course"}, &out, &errb, dbPath)
	if code != 2 {
		t.Fatalf("want exit 2 when --plan-file missing, got %d (stderr: %s)", code, errb.String())
	}
}

func TestPlanRewriteMissingFileExits1(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VAULT_ROOT", t.TempDir())
	var out, errb bytes.Buffer
	code := run([]string{"clawcli", "plan", "rewrite", "--course", "rw-course", "--plan-file", "/nonexistent/plan.json"}, &out, &errb, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1 on missing file, got %d (stderr: %s)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "reading") {
		t.Fatalf("expected a read error, stderr: %s", errb.String())
	}
}

func TestCourseCreateWithMissingSessionExits1(t *testing.T) {
	dbPath := newTempDB(t)
	var out, errb bytes.Buffer
	code := run([]string{
		"clawcli", "course", "create", "--id", "orphan-course", "--name", "Orphan",
		"--session", "99999",
	}, &out, &errb, dbPath)
	if code != 1 {
		t.Fatalf("want exit 1 for missing session, got %d (stderr: %s)", code, errb.String())
	}
	if !strings.Contains(errb.String(), "not found") {
		t.Fatalf("expected 'not found' in stderr, got: %s", errb.String())
	}
}

func TestCourseCreateWithSessionRetags(t *testing.T) {
	dbPath := newTempDB(t)
	var sid int64
	func() {
		app := openApp(t, dbPath)
		defer func() { _ = app.Close() }()
		s, err := app.CreateSession("", "Design a new course", "authoring")
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		sid = s.ID
	}()
	var out, errb bytes.Buffer
	code := run([]string{
		"clawcli", "course", "create", "--id", "retag-course", "--name", "Retag",
		"--session", strconv.FormatInt(sid, 10),
	}, &out, &errb, dbPath)
	if code != 0 {
		t.Fatalf("create exit %d, stderr: %s", code, errb.String())
	}
	app := openApp(t, dbPath)
	defer func() { _ = app.Close() }()
	s, err := app.GetSession(sid)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if s.CourseID != "retag-course" {
		t.Fatalf("session not re-tagged, course_id = %q", s.CourseID)
	}
}

func TestConfidenceLogWritesRowAndQueue(t *testing.T) {
	dbPath := newTempDB(t)
	var sessID int64
	var atomID string
	func() {
		app := openApp(t, dbPath)
		defer func() { _ = app.Close() }()
		s, err := app.CreateSession("", "confidence log test", "study")
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		sessID = s.ID
		// Confidence now keys on a real atom (ADR 0019), not a task id.
		atomID, err = app.CreateKnowledgeComponent("an atom", "body", "", 0)
		if err != nil {
			t.Fatalf("create atom: %v", err)
		}
	}()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "confidence", "log",
		"--session", strconv.FormatInt(sessID, 10),
		"--kc", atomID,
		"--value", "0.8",
		"--raw", "pretty solid",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}

	db, err := agent.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	var n int
	var sess int64
	var kc, source string
	var val float64
	if err := db.QueryRow(`SELECT count(*) FROM confidence_log`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("confidence_log rows = %d (err %v), want 1", n, err)
	}
	if err := db.QueryRow(
		`SELECT session_id, knowledge_component_id, value, source FROM confidence_log`,
	).Scan(&sess, &kc, &val, &source); err != nil {
		t.Fatalf("scan row: %v", err)
	}
	if sess != sessID || kc != atomID || val != 0.8 || source != "tool_call" {
		t.Fatalf("row = (%d,%q,%v,%q)", sess, kc, val, source)
	}

	var qn int
	if err := db.QueryRow(
		`SELECT count(*) FROM retrieval_queue WHERE knowledge_component_id = ?`, atomID,
	).Scan(&qn); err != nil || qn != 1 {
		t.Fatalf("retrieval_queue rows = %d (err %v), want 1", qn, err)
	}
}

func TestConfidenceLogRejectsOutOfRange(t *testing.T) {
	dbPath := newTempDB(t)
	var sessID int64
	func() {
		app := openApp(t, dbPath)
		defer func() { _ = app.Close() }()
		s, err := app.CreateSession("", "out of range test", "study")
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		sessID = s.ID
	}()
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "confidence", "log",
		"--session", strconv.FormatInt(sessID, 10), "--kc", "task-x", "--value", "1.5",
	}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("exit code: %d, want 2 (out-of-range is a usage error)", code)
	}
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	var n int
	_ = db.QueryRow(`SELECT count(*) FROM confidence_log`).Scan(&n)
	if n != 0 {
		t.Fatalf("confidence_log rows = %d, want 0 (no write on rejection)", n)
	}
}

func TestConfidenceLogMissingKCExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "confidence", "log",
		"--session", "1", "--value", "0.5",
	}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("exit code: %d, want 2", code)
	}
}

func TestPlanToggleForceBypassesGate(t *testing.T) {
	dbPath := newTempDB(t)
	// The CLI resolves plan files from VAULT_ROOT; pin it to the temp dir so
	// LoadPlan finds our plan deterministically.
	vault := filepath.Dir(dbPath)
	t.Setenv("VAULT_ROOT", vault)
	plansDir := filepath.Join(vault, "data", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	planJSON := `{"id":"gate-course","name":"Gate","phases":[{"title":"P1","tasks":[{"id":"t-0","title":"Read: Task zero","done":false}]}]}`
	if err := os.WriteFile(filepath.Join(plansDir, "gate-course.json"), []byte(planJSON), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "plan", "toggle",
		"--course", "gate-course", "--task", "0", "--force",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "atomicity gate") {
		t.Fatalf("force should bypass gate, stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "done") {
		t.Fatalf("expected task marked done, stdout: %s", stdout.String())
	}
}

func TestPlanToggleWithoutForceHitsGate(t *testing.T) {
	dbPath := newTempDB(t)
	vault := filepath.Dir(dbPath)
	t.Setenv("VAULT_ROOT", vault)
	plansDir := filepath.Join(vault, "data", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	planJSON := `{"id":"gate-course2","name":"Gate","phases":[{"title":"P1","tasks":[{"id":"t-0","title":"Read: Task zero","done":false}]}]}`
	if err := os.WriteFile(filepath.Join(plansDir, "gate-course2.json"), []byte(planJSON), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "plan", "toggle",
		"--course", "gate-course2", "--task", "0",
	}, &stdout, &stderr, dbPath)
	// Gate refusal is a normal tool result (exit 0), not a CLI error.
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "atomicity gate") {
		t.Fatalf("expected gate refusal, stdout: %s", stdout.String())
	}
}
