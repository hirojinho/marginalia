# Agent PDF / Slide Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the Pi agent read the slide/PDF the user is viewing instead of reconstructing it from memory, by adding `claw-cli pdf list` + `claw-cli pdf current` and instructing the agent to use them in `AGENTS.md`.

**Architecture:** Two new read-only `claw-cli` subcommands wrap existing `*App` DB methods (`ListPDFs`, `GetSessionLastPDFID`, `GetLastOpenedPDFID`, `GetPDF`) — no new DB code. A new "Slides / PDFs" section in the generated `AGENTS.md` tells the agent how/when to call them, chaining into the pre-existing `pdf extract`. Reading is on-demand; no PDF text is force-injected into turns.

**Tech Stack:** Go 1.26 (build with `/opt/homebrew/bin/go`), SQLite, the `flag` stdlib package, standard table/`bytes.Buffer` tests. Deploy: cross-compile linux/amd64, scp, `systemctl --user restart` on `nanoclaw`.

---

## File Structure

| File | Change | Responsibility |
|------|--------|----------------|
| `claw-cli/main.go` | Modify `runPDF` dispatch; add `pdfList`, `pdfCurrent`; add `"sort"` import | CLI surface for PDF discovery |
| `claw-cli/main_test.go` | Add `seedPDF`/`openApp` helpers + 4 tests; add `fmt`,`strconv` imports | Test the new subcommands |
| `agent/sandbox.go` | Add a "Slides / PDFs" section in `writeAgentsMD` (before pedagogy) | Instruct the agent to read PDFs |
| `agent/sandbox_test.go` | Add 1 test; add `strings` import | Verify the instruction is emitted |

No changes to `pdf extract`, `ToolPDFExtract`, or any DB method.

---

## Task 1: `claw-cli pdf list` subcommand

**Files:**
- Modify: `claw-cli/main.go` (the `runPDF` switch ~line 634; add `pdfList` func; add `"sort"` to imports)
- Test: `claw-cli/main_test.go`

- [ ] **Step 1: Add test helpers and the failing test**

Add to the top of `claw-cli/main_test.go` (after `newTempDB`). First ensure the import block includes `"fmt"` and `"strconv"`:

```go
func seedPDF(t *testing.T, dbPath string, id int, name, lastReadAt string) {
	t.Helper()
	db, err := agent.OpenDB(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `/opt/homebrew/bin/go test ./claw-cli/ -run TestPDFListEmptyAndOrdered -v`
Expected: FAIL — `run` returns exit 2 with "unknown pdf subcommand: \"list\"" (and/or compile error once helpers reference nothing yet). The assertion `exit 0` fails.

- [ ] **Step 3: Implement `pdf list`**

In `claw-cli/main.go`, add `"sort"` to the import block. In `runPDF`, add a case:

```go
	case "list":
		return pdfList(args[1:], stdout, stderr, dbPath)
```

Add the function (next to `pdfExtract`):

```go
func pdfList(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("pdf list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id filter (optional)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()
	pdfs, err := app.ListPDFs(*course)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "pdf list: %v\n", err)
		return 1
	}
	if pdfs == nil {
		pdfs = []agent.PDFEntry{}
	}
	// Most-recently-read first; nil/empty last_read_at sorts last.
	sort.SliceStable(pdfs, func(i, j int) bool {
		ri, rj := "", ""
		if pdfs[i].LastReadAt != nil {
			ri = *pdfs[i].LastReadAt
		}
		if pdfs[j].LastReadAt != nil {
			rj = *pdfs[j].LastReadAt
		}
		if ri == rj {
			return false
		}
		if ri == "" {
			return false
		}
		if rj == "" {
			return true
		}
		return ri > rj
	})
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(map[string]any{"pdfs": pdfs})
	return 0
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `/opt/homebrew/bin/go test ./claw-cli/ -run TestPDFListEmptyAndOrdered -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add claw-cli/main.go claw-cli/main_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(claw-cli): add pdf list subcommand

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: `claw-cli pdf current` subcommand

**Files:**
- Modify: `claw-cli/main.go` (`runPDF` switch; add `pdfCurrent` func)
- Test: `claw-cli/main_test.go`

- [ ] **Step 1: Add the failing tests**

Append to `claw-cli/main_test.go`:

```go
func TestPDFCurrentSessionHit(t *testing.T) {
	dbPath := newTempDB(t)
	seedPDF(t, dbPath, 2, "ch8.pdf", "2026-05-20T10:00:00Z")

	var sessID int64
	func() {
		app := openApp(t, dbPath)
		defer app.Close()
		sess, err := app.CreateSession("ce297", "topic")
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		sessID = sess.ID
		if _, err := app.DB.Exec("UPDATE sessions SET last_pdf_id = ? WHERE id = ?", 2, sessID); err != nil {
			t.Fatalf("set last_pdf_id: %v", err)
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
		defer app.Close()
		if err := app.SetLastOpenedPDF(5); err != nil {
			t.Fatalf("set last opened: %v", err)
		}
	}()

	var out, errb bytes.Buffer
	// No --session → falls back to last-opened PDF.
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `/opt/homebrew/bin/go test ./claw-cli/ -run TestPDFCurrent -v`
Expected: FAIL — `pdf current` is an unknown subcommand (exit 2), so the `exit 0`/`exit 1` assertions fail.

- [ ] **Step 3: Implement `pdf current`**

In `claw-cli/main.go` `runPDF`, add a case:

```go
	case "current":
		return pdfCurrent(args[1:], stdout, stderr, dbPath)
```

Add the function:

```go
func pdfCurrent(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("pdf current", flag.ContinueOnError)
	fs.SetOutput(stderr)
	session := fs.Int64("session", 0, "session id (optional; uses its last-viewed PDF)")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()

	var pdfID int64
	if *session > 0 {
		if id, err := app.GetSessionLastPDFID(*session); err == nil && id > 0 {
			pdfID = id
		}
	}
	if pdfID == 0 {
		if id, err := app.GetLastOpenedPDFID(); err == nil && id > 0 {
			pdfID = id
		}
	}
	if pdfID == 0 {
		_, _ = fmt.Fprintln(stderr, "pdf current: no PDF is currently open")
		return 1
	}
	entry, err := app.GetPDF(pdfID)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "pdf current: %v\n", err)
		return 1
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(entry)
	return 0
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `/opt/homebrew/bin/go test ./claw-cli/ -run TestPDFCurrent -v`
Expected: PASS (all three)

- [ ] **Step 5: Commit**

```bash
git add claw-cli/main.go claw-cli/main_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(claw-cli): add pdf current subcommand

Resolves the user's current PDF via session last_pdf_id, falling back
to the global last-opened PDF.

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: `AGENTS.md` "Slides / PDFs" instruction

**Files:**
- Modify: `agent/sandbox.go` (`writeAgentsMD`, insert before `pedagogySection` is appended ~line 174)
- Test: `agent/sandbox_test.go`

- [ ] **Step 1: Add the failing test**

In `agent/sandbox_test.go`, add `"strings"` to the import block, then append:

```go
func TestWriteAgentsMDIncludesPDFSection(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, err := sm.Create(42, "", "", "")
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestWriteAgentsMDIncludesPDFSection -v`
Expected: FAIL — AGENTS.md does not yet contain "## Slides / PDFs".

- [ ] **Step 3: Implement the section**

In `agent/sandbox.go`, immediately **before** the `pedagogySection` block (so pedagogy stays last/closest to the user message), insert:

```go
	pdfSection := fmt.Sprintf(
		"\n## Slides / PDFs\n\n"+
			"The user studies from PDF documents shown in a viewer beside this chat. "+
			"**Never reconstruct slide or document content from your own memory — read the actual pages.** "+
			"To see what the user is currently reading:\n```\nclaw-cli pdf current --session %d\n```\n"+
			"This returns the open PDF's `id` and `last_page` (the page they are on). Then read the relevant pages:\n"+
			"```\nclaw-cli pdf extract --id <id> --pages <range around last_page, e.g. 40-50>\n```\n"+
			"Use `claw-cli pdf list` to see every uploaded PDF (most-recently-read first).\n",
		sessionID,
	)
	content = append(content, []byte(pdfSection)...)
```

(`fmt` is already imported in `sandbox.go`.)

- [ ] **Step 4: Run the test to verify it passes**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestWriteAgentsMDIncludesPDFSection -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add agent/sandbox.go agent/sandbox_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(agent): instruct Pi to read PDFs via claw-cli in AGENTS.md

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: Full test run, build, deploy, acceptance

**Files:** none (verification + deploy only)

- [ ] **Step 1: Run the full affected test suites**

Run: `/opt/homebrew/bin/go test ./agent/... ./claw-cli/...`
Expected: `ok` for both packages, no failures.

- [ ] **Step 2: Cross-compile server and claw-cli for the VPS**

```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
ls -la /tmp/study-app-linux /tmp/claw-cli-linux
```
Expected: two ELF binaries produced (study-app ~18 MB).

- [ ] **Step 3: Deploy both binaries with backups, restart**

```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
scp /tmp/claw-cli-linux  nanoclaw:/home/eduardo/stack/study-app/bin/claw-cli.new
ssh nanoclaw 'cd ~/stack/study-app/bin && \
  cp study-app study-app.bak && mv study-app.new study-app && \
  cp claw-cli claw-cli.bak && mv claw-cli.new claw-cli && \
  chmod +x study-app claw-cli && \
  export XDG_RUNTIME_DIR=/run/user/$(id -u) && \
  systemctl --user restart study-app.service && \
  systemctl --user is-active study-app.service'
```
Expected: final line `active`.

- [ ] **Step 4: Smoke-test the new subcommands on the VPS**

```bash
ssh nanoclaw 'CLAW_STUDY_ROOT=/home/eduardo/stack/study-app /home/eduardo/stack/study-app/bin/claw-cli pdf current'
ssh nanoclaw 'CLAW_STUDY_ROOT=/home/eduardo/stack/study-app /home/eduardo/stack/study-app/bin/claw-cli pdf list'
```
Expected: `pdf current` returns JSON for PDF id 4 ("Chapter 8 - PHI ETA Risk Assessment.pdf"); `pdf list` returns all 4 PDFs, the most-recently-read first.

- [ ] **Step 5: Manual acceptance through the app**

Open `https://study.claw-study.xyz`, start a **fresh** Pi `/chat-v2` session on the safety course while viewing the Ch.8 PDF, and ask about ALARP / secondary risk. Confirm in the tool stream that the agent calls `pdf current` then `pdf extract`, and that it quotes actual slide wording rather than saying it must reconstruct from memory.

If acceptance fails, capture `journalctl --user -u study-app.service` for the session and return to systematic-debugging — do not patch blindly.

---

## Self-Review

**Spec coverage:**
- `pdf list` (sorted last_read_at desc) → Task 1. ✓
- `pdf current` (session → fallback → error) → Task 2. ✓
- AGENTS.md "Slides / PDFs" section with session id baked in → Task 3. ✓
- `pdf extract` unchanged → no task (correct, it already works). ✓
- Rebuild + deploy **both** server and claw-cli → Task 4 Steps 2–3. ✓
- Data caveat (rely on `last_page`) → reflected in the AGENTS.md wording ("`last_page` (the page they are on)") and the extract page-range hint. ✓

**Placeholder scan:** No TBD/TODO; every code step shows complete code; `<id>`/`<range...>` inside the AGENTS.md string are intentional literal template tokens shown to the agent, not plan gaps.

**Type consistency:** `pdfList`/`pdfCurrent` signatures match the existing `pdfExtract` shape `(args []string, stdout, stderr io.Writer, dbPath string) int`. `GetPDF`/`GetSessionLastPDFID`/`GetLastOpenedPDFID` take/return `int64`; `*session` is `*int64`; `sess.ID` is `int64` (matches `GetSessionLastPDFID(id int64)`). `agent.PDFEntry` is the type returned by `ListPDFs`/`GetPDF`. Test imports to add: `fmt`, `strconv` (main_test.go), `strings` (sandbox_test.go).
