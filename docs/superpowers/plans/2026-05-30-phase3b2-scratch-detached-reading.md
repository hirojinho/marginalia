# Phase 3b-2 — Scratch / Detached buckets + Reading auto-open Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for INLINE execution with browser-verified checkpoints. The frontend has **no JS test runner** (ADR 0004) — JS tasks are verified by driving a locally-run binary via Chrome DevTools Protocol (`Runtime.evaluate`, reusable driver at `/tmp/cdp.mjs`). The one backend task IS unit-testable (Go). Steps use checkbox (`- [ ]`) syntax.

**Goal:** Finish the IA rebuild's visible half — split the flat "Other chats" stub into **Scratch**, **Detached**, and a collapsible **"Before the redesign"** archive; and auto-open a task's learned PDF when its session opens.

**Architecture:** All client-side in `static/rail.js`: `loadRailData` already fetches plan + sessions; extend it to classify task-less sessions into **Scratch** (`archived=false`) vs **archived** (`archived=true`, the pre-redesign chats), and to detect **Detached** sessions (a `task_id` set but absent from the *current course's* plan, course-scoped). `renderOther` becomes three labelled sections (archive collapsible). Separately, `openTask`'s existing-session branch auto-opens the session's `last_pdf_id` via `pdf.js` (`setCurrentPdfId`/`openPdf`/`showView`). One small backend hardening: `GetSessionByTask` excludes `archived` rows (defends the "anchored sessions are never archived" invariant the 3b-1 review flagged), TDD'd in Go.

**Tech Stack:** Vanilla ES modules, `[data-action]` delegation (`static/app.js`), embedded static (`//go:embed static/*` — JS ships in the Go binary). Backend: Go (`modernc.org/sqlite`). Live endpoints unchanged: `/api/plan?view=full&id=`, `/api/sessions` (returns `task_id`, `archived`, `last_pdf_id`), `/api/sessions/for-task`.

**Spec:** [ADR 0014](../../adr/0014-phase3-task-anchored-sessions-data-model.md) (Detached bucket = orphaned `task_id`; archived = clean-break migration), [ADR 0011](../../adr/0011-plan-is-navigation-spine.md) (Scratch), [ADR 0012](../../adr/0012-segmented-active-reading.md) (reading tied to the task). Glossary: [CONTEXT.md](../../CONTEXT.md).

**Builds on:** 3b-1 (`static/rail.js`, head `525940a` on `main`). The `Session` JSON already carries `task_id`, `archived`, `last_pdf_id` (Phase 3a).

**Sync/deploy:** `static/*` + `agent/*.go` — all in the `study-app` binary. Deploy = `go build` + scp + swap + restart (Task 4). No `claw-cli`, no disk files.

---

## Local run (every JS verification step)

```bash
mkdir -p /tmp/claw-rail-vault/data/plans   # reuse 3b-1's scratch vault if present
VAULT_ROOT=/tmp/claw-rail-vault LISTEN_ADDR=127.0.0.1:8099 AUTH_TOKEN= LLM_API_KEY=dummy AGENT_RUNTIME=pi /opt/homebrew/bin/go run .
# Chrome CDP driver (already written for 3b-1): node /tmp/cdp.mjs <url> <screenshot.png> <evalFile.js>
# launch chrome once: "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" --headless=new --disable-gpu --remote-debugging-port=9222 --user-data-dir=/tmp/chrome-rail-profile about:blank &
```

If `/tmp/cdp.mjs` is missing, recreate it (CDP client: `fetch PUT /json/new`, connect `webSocketDebuggerUrl`, `Page.enable`+`Runtime.enable`, navigate, settle ~1.8s, `Runtime.evaluate` the eval file with `awaitPromise`, `Page.captureScreenshot`). Seed plans `ce297.json`/`ddia.json` as in 3b-1 Task 1.

---

## File Structure

- `static/rail.js` — **Modify.** `loadRailData` classifies task-less + detached sessions into module-level `scratchSessions`, `archivedSessions`, `detachedSessions`; `renderOther` renders three sections; `openTask` auto-opens the session's PDF.
- `static/style.css` — **Modify.** Styles for the three bucket sections + the collapsible archive.
- `agent/db.go` — **Modify.** `GetSessionByTask` adds `AND archived = 0`.
- `agent/db_test.go` — **Modify.** Test that an archived anchored session is not returned.

---

## Task 1: Classify sessions into Scratch / Detached / Archived

**Files:** Modify `static/rail.js`.

- [ ] **Step 1: Replace the session-indexing block in `loadRailData`.** Replace the `for (const s of sessions || []) { ... }` loop (and the two lines initializing `sessionsByTaskId`/`otherSessions`) with course-scoped classification. Also replace the module-state declaration `let otherSessions = [];` (line ~12) with the three new arrays.

Change the declaration line:
```js
let otherSessions = []; // task_id-less sessions (Scratch stub)
```
to:
```js
let scratchSessions = []; // task_id-less, not archived (live Scratch)
let archivedSessions = []; // task_id-less, archived (pre-redesign — "Before the redesign")
let detachedSessions = []; // task_id set but absent from the current course's plan (orphaned)
```

Replace the classification loop inside `loadRailData`:
```js
    sessionsByTaskId = {};
    otherSessions = [];
    for (const s of sessions || []) {
      if (s.task_id) sessionsByTaskId[s.task_id] = s;
      else otherSessions.push(s);
    }
```
with:
```js
    // Task ids present in the currently-selected course's plan.
    const planTaskIds = new Set();
    if (currentPlan) {
      walkTasks(currentPlan, (n) => {
        if (n.kind === 'task') planTaskIds.add(n.task.id);
      });
    }
    sessionsByTaskId = {};
    scratchSessions = [];
    archivedSessions = [];
    detachedSessions = [];
    for (const s of sessions || []) {
      if (s.task_id) {
        if (planTaskIds.has(s.task_id)) {
          sessionsByTaskId[s.task_id] = s; // has-work for a current plan task
        } else if (s.course_id === selectedCourse) {
          detachedSessions.push(s); // anchored to this course, but the task is gone
        }
        // task-anchored sessions of OTHER courses are not shown on this rail
      } else {
        // task-less = Scratch family; show global (no course) + this course's
        const inScope = !s.course_id || s.course_id === selectedCourse;
        if (!inScope) continue;
        if (s.archived) archivedSessions.push(s);
        else scratchSessions.push(s);
      }
    }
```

- [ ] **Step 2: Syntax-check.** `node --check static/rail.js` → no output (note: `renderOther` still references the now-removed `otherSessions` — that's fixed in Task 2; this step only confirms no parse error from the loop edit, so EXPECT a runtime reference later, not a parse error. `node --check` checks syntax only and will pass).

- [ ] **Step 3: Commit.** (Deferred — `renderOther` is updated in Task 2; commit Tasks 1+2 together to keep the file runnable and lint-clean. Skip committing here.)

---

## Task 2: Render the three buckets (Scratch, Detached, Before the redesign)

**Files:** Modify `static/rail.js`, `static/style.css`.

- [ ] **Step 1: Replace `renderOther`.** Replace the whole `renderOther` function with:

```js
function renderSessionLine(s) {
  return `<div class="rail-other-item" data-action="switch-session" data-session-id="${s.id}">${escapeHtml(s.topic || 'Untitled')}</div>`;
}

function renderOther() {
  let html = '';
  if (scratchSessions.length) {
    html += '<div class="rail-bucket"><div class="rail-other-label">Scratch</div>';
    for (const s of scratchSessions) html += renderSessionLine(s);
    html += '</div>';
  }
  if (detachedSessions.length) {
    html +=
      '<div class="rail-bucket"><div class="rail-other-label">Detached' +
      ` <span class="rail-bucket-hint">task removed from plan</span></div>`;
    for (const s of detachedSessions) html += renderSessionLine(s);
    html += '</div>';
  }
  if (archivedSessions.length) {
    html +=
      '<details class="rail-bucket rail-archive"><summary class="rail-other-label">' +
      `Before the redesign <span class="rail-bucket-hint">${archivedSessions.length}</span></summary>`;
    for (const s of archivedSessions) html += renderSessionLine(s);
    html += '</details>';
  }
  return html;
}
```

- [ ] **Step 2: Add bucket CSS.** Append to `static/style.css`:

```css
.rail-bucket {
  margin-top: 18px;
  border-top: 1px solid var(--border);
  padding-top: 10px;
}
.rail-bucket-hint {
  font-weight: 400;
  color: var(--text-tertiary);
  text-transform: none;
  letter-spacing: 0;
}
.rail-archive > summary {
  cursor: pointer;
  list-style: revert;
}
.rail-archive[open] > summary {
  margin-bottom: 4px;
}
```

(The existing `.rail-other-label` / `.rail-other-item` styles from 3b-1 are reused.)

- [ ] **Step 3: Build + run + seed.** Reuse the 3b-1 scratch vault. Start the local server. Seed one session of each kind for `ce297`:
```bash
# Scratch (task-less, not archived)
curl -s -XPOST 127.0.0.1:8099/api/sessions -H 'Content-Type: application/json' -d '{"course_id":"ce297","topic":"scratch aside"}' >/dev/null
# Detached: anchor to a task id NOT in ce297.json, same course
curl -s -XPOST 127.0.0.1:8099/api/sessions/for-task -H 'Content-Type: application/json' -d '{"course_id":"ce297","task_id":"GONE-task","topic":"detached chat"}' >/dev/null
# Archived: create task-less then flip archived=1 directly in the scratch DB
SID=$(curl -s -XPOST 127.0.0.1:8099/api/sessions -H 'Content-Type: application/json' -d '{"course_id":"ce297","topic":"old pre-redesign chat"}' | sed -n 's/.*"id":\([0-9]*\).*/\1/p')
sqlite3 /tmp/claw-rail-vault/data/study.db "UPDATE sessions SET archived=1 WHERE id=$SID;"
```

- [ ] **Step 4: Verify with CDP.** Eval file `/tmp/eval-3b2-buckets.js`:
```js
(async () => {
  const sel = document.getElementById('rail-course-select');
  sel.value = 'ce297';
  sel.dispatchEvent(new Event('change', { bubbles: true }));
  await new Promise((r) => setTimeout(r, 1000));
  const labels = [...document.querySelectorAll('.rail-other-label')].map((e) =>
    e.textContent.replace(/\s+/g, ' ').trim(),
  );
  const archiveItemsHiddenUntilOpen = document.querySelectorAll(
    '.rail-archive:not([open]) .rail-other-item',
  ).length; // present in DOM even when collapsed
  return {
    labels,
    scratchItems: [...document.querySelectorAll('.rail-bucket:not(.rail-archive) .rail-other-item')].map(
      (e) => e.textContent,
    ),
  };
})()
```
Run: `node /tmp/cdp.mjs "http://127.0.0.1:8099/" /tmp/3b2-buckets.png /tmp/eval-3b2-buckets.js`
Expected: `labels` contains "Scratch", "Detached task removed from plan", and "Before the redesign 1"; `scratchItems` contains "scratch aside" and "detached chat" (both are non-archive buckets) but NOT "old pre-redesign chat" (it's inside the collapsed `<details>`). Read `/tmp/3b2-buckets.png` to confirm the three sections render and the archive is collapsed.

- [ ] **Step 5: Commit Tasks 1+2.**
```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
npm run format:write >/dev/null 2>&1
git add static/rail.js static/style.css
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(rail): bucket task-less sessions into Scratch / Detached / Before-the-redesign (ADR 0014/0011)"
```

---

## Task 3: Reading auto-open on task entry

**Files:** Modify `static/rail.js`.

When a task's session opens and it has a learned `last_pdf_id`, open that PDF on the right (ADR 0012: reading tied to the task). Pending/empty tasks have no resource yet — no-op.

- [ ] **Step 1: Import the PDF helpers.** In `static/rail.js`, add after the existing imports:

```js
import { openPdf, showView, setCurrentPdfId } from './pdf.js';
```

- [ ] **Step 2: Auto-open in the existing-session branch of `openTask`.** In `openTask`, inside `if (data && data.id) { ... }`, after `setBanner(title);`, add:

```js
      if (data.last_pdf_id) {
        setCurrentPdfId(data.last_pdf_id);
        openPdf(data.last_pdf_id);
        showView('split');
      }
```

(`openPdf` is async/fire-and-forget here — it loads the PDF panel; mirrors `plan.js`'s `openPdfFromDrawer`. `last_pdf_id` is null for sessions that never opened a PDF, so the block is skipped.)

- [ ] **Step 3: Build + verify.** Rebuild + restart. Seed a session with a PDF link: upload is heavy locally, so instead set `last_pdf_id` directly on the t-aaa session and insert a fake pdf row, then confirm `openTask` calls `openPdf`. Simpler deterministic check — assert the wiring fires without a real PDF by stubbing `window.pdfjsLib` is overkill; instead verify the call path via CDP by spying:
```js
// /tmp/eval-3b2-reading.js — confirm openTask triggers a PDF open when last_pdf_id is set
(async () => {
  // Point t-aaa's session at pdf id 1 in the scratch DB beforehand (see Step 3 shell).
  let opened = null;
  const origFetch = window.fetch;
  // We can't import modules here; instead detect the side effect: showView('split')
  // adds the 'split' layout. Click t-aaa and read the layout class.
  const sel = document.getElementById('rail-course-select');
  sel.value = 'ce297';
  sel.dispatchEvent(new Event('change', { bubbles: true }));
  await new Promise((r) => setTimeout(r, 900));
  document.querySelector('[data-task-id="t-aaa"]').click();
  await new Promise((r) => setTimeout(r, 1200));
  return {
    mainContentView: document.getElementById('main-content')?.className || '(none)',
    pdfPanelVisible: getComputedStyle(document.getElementById('pdf-panel')).display,
  };
})()
```
Shell before the eval (link the session to a pdf row):
```bash
sqlite3 /tmp/claw-rail-vault/data/study.db "INSERT INTO pdfs (id, filename, original_name, course_id, pages, last_page, uploaded_at) VALUES (1,'1.pdf','Chapter 8.pdf','ce297',104,40,'2026-05-30T00:00:00Z'); UPDATE sessions SET last_pdf_id=1, last_page=40 WHERE course_id='ce297' AND task_id='t-aaa';"
```
Run the eval. Expected: after clicking t-aaa, the layout switches toward split (`mainContentView` reflects the split view set by `showView('split')`, or `pdfPanelVisible` is not `none`). The PDF file itself 404s locally (no real file), but the **view switches and `openPdf` is invoked** — that's the wiring under test. Confirm via the screenshot that the right panel opened. **Read `showView` in `static/pdf.js` first** to learn the exact class/DOM it toggles, and assert on that exact signal.

- [ ] **Step 4: Commit.**
```bash
git add static/rail.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(rail): auto-open the task's learned PDF on session open (ADR 0012)"
```

---

## Task 4: Backend — `GetSessionByTask` excludes archived (TDD)

**Files:** Modify `agent/db.go`, `agent/db_test.go`.

3b-1 review flagged that `GetSessionByTask` returning an archived anchored session would resurface pre-redesign work as if it were the task's live session. Anchored sessions are never archived today (the migration only archives `task_id IS NULL`), but hardening the query removes the latent footgun.

- [ ] **Step 1: Write the failing test.** Add to `agent/db_test.go`:

```go
func TestGetSessionByTaskExcludesArchived(t *testing.T) {
	a := newMemoryApp(t)
	s, err := a.CreateSessionForTask("ddia", "task-arch", "anchored then archived")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Visible while live.
	if _, ok, err := a.GetSessionByTask("ddia", "task-arch"); err != nil || !ok {
		t.Fatalf("expected found before archive, ok=%v err=%v", ok, err)
	}
	if _, err := a.DB.Exec("UPDATE sessions SET archived = 1 WHERE id = ?", s.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// Once archived, the task should resolve as having no live session.
	if _, ok, err := a.GetSessionByTask("ddia", "task-arch"); err != nil || ok {
		t.Errorf("expected not-found after archive, ok=%v err=%v", ok, err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails.** `/opt/homebrew/bin/go test ./agent/ -run TestGetSessionByTaskExcludesArchived -v` → FAIL (the row is still returned; `ok=true` after archive).

- [ ] **Step 3: Add the guard.** In `agent/db.go`, in `GetSessionByTask`, change the query:
```go
		"SELECT id FROM sessions WHERE course_id = ? AND task_id = ? AND hidden = 0 ORDER BY id LIMIT 1",
```
to:
```go
		"SELECT id FROM sessions WHERE course_id = ? AND task_id = ? AND hidden = 0 AND archived = 0 ORDER BY id LIMIT 1",
```

- [ ] **Step 4: Run test to verify it passes + full suite.** `/opt/homebrew/bin/go test ./agent/ -run TestGetSessionByTaskExcludesArchived -v` → PASS. Then `/opt/homebrew/bin/go build ./... && /opt/homebrew/bin/go test ./agent/ ./handler/` → build OK, both `ok`.

- [ ] **Step 5: Commit.**
```bash
git add agent/db.go agent/db_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "fix(sessions): GetSessionByTask excludes archived rows (ADR 0014 hardening)"
```

---

## Task 5: Deploy — REQUIRES EXPLICIT DEPLOY APPROVAL

**Files:** none (rebuild + ship `study-app`; static embedded).

> Do not run without the user's explicit "deploy" go-ahead (prod boundary). No DB migration.

- [ ] **Step 1: Clean + green.** `git status -sb && /opt/homebrew/bin/go build ./... && /opt/homebrew/bin/go vet ./... && /opt/homebrew/bin/go test ./...` → clean, all `ok`.
- [ ] **Step 2: Build linux binary.** `GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .` → exit 0.
- [ ] **Step 3: Ship + swap + restart.**
```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak.2026-05-30-phase3b2 && mv study-app.new study-app && chmod +x study-app && systemctl --user restart study-app.service && sleep 3 && systemctl --user is-active study-app.service && systemctl --user is-active study-app-tunnel.service'
```
Expected: both `active`.
- [ ] **Step 4: Verify live.** `curl -s -o /dev/null -w "%{http_code}\n" https://study.claw-study.xyz/` → 401. In the browser (logged in): the "Other chats" stub is gone, replaced by Scratch / Detached (if any) / collapsible "Before the redesign" holding the ~32 archived chats; opening a task with a prior PDF auto-opens reading.
- [ ] **Step 5: Push.** `git push origin main`.

---

## Self-Review

**1. Spec coverage:**
- ADR 0014 archived = "Before the redesign" group → Task 1 (`archivedSessions`) + Task 2 (collapsible `<details>`). ✓
- ADR 0014 Detached bucket (orphaned `task_id`) → Task 1 (course-scoped: `task_id` set, not in this plan, same course) + Task 2. ✓
- ADR 0011 Scratch (task-less, global + course-tagged) → Task 1 (`scratchSessions`, `inScope`) + Task 2. ✓
- ADR 0012 reading tied to the task → Task 3 (auto-open `last_pdf_id`). ✓
- 3b-1 carry-over (GetSessionByTask vs archived) → Task 4 (now a real fix + test, not just a comment). ✓
- Removes the live "Other chats" clutter → Tasks 1+2 replace `renderOther`. ✓

**2. Placeholder scan:** No TBD/TODO. Each code step shows complete code; CDP evals + shell seeds are concrete. Task 3 Step 3 instructs reading `showView` first to assert on the exact DOM signal — a deliberate "confirm against the real file" step (the precise class `showView` toggles isn't reproduced here because it's read at execution time), not a placeholder.

**3. Type/name consistency:** `scratchSessions`/`archivedSessions`/`detachedSessions` declared in Task 1, rendered in Task 2's `renderOther`, replacing the 3b-1 `otherSessions` (which is fully removed — no dangling reference after Task 2). `planTaskIds` built via the existing `walkTasks`. `renderSessionLine` defined and used in Task 2. `setCurrentPdfId`/`openPdf`/`showView` imported in Task 3 (exist in `pdf.js`, confirmed). `GetSessionByTask` signature unchanged in Task 4 (only the SQL string changes). The Session fields `task_id`/`archived`/`last_pdf_id`/`course_id` all exist on the Phase-3a `Session` JSON.

**Known interim (acceptable):** Scratch course-scoping shows global + current-course task-less sessions; a session tagged to a *different* course's Scratch won't appear while viewing this course (consistent with the one-course rail). Detached is course-scoped to avoid mislabeling other courses' tasks.
