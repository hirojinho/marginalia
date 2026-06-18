# Session Sidebar — Course-First Launcher Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rebuild the session sidebar as a single-open, course-first accordion with one-click per-course session creation and async LLM-generated titles.

**Architecture:** The drawer becomes a launcher + archive (see [ADR 0008](../../adr/0008-sidebar-course-first-launcher.md)). All courses are always listed; one expands at a time (state in `localStorage`); each course header has a `+` that creates a session instantly via the existing `POST /api/sessions` with an empty topic (defaults to `"General"` in `CreateSession`). On the first chat turn, `chat_v2.go` already auto-titles and emits a `session_topic` SSE event that the frontend (`chat.js:173`) reacts to by reloading — we upgrade that titling from a deterministic truncation to an async LLM call, keeping the truncation as fallback.

**Tech Stack:** Go 1.26 (backend, `/opt/homebrew/bin/go`), vanilla JS + CSS (frontend, no build step per [ADR 0004](../../adr/0004-vanilla-js-frontend.md)). Go tests via `go test`. No JS test runner — frontend is gated on a manual verification checklist (Task 6).

**Deploy:** build linux/amd64 → scp → `systemctl --user restart study-app.service` (see `claw_study_service.md` ops cheat sheet).

---

## File Structure

| File | Change | Responsibility |
|---|---|---|
| `agent/llm.go` | Modify | Add `titlePrompt`, `cleanTitle()` helper, `GenerateTitle()` method |
| `agent/llm_test.go` | Create or modify | Unit tests for `cleanTitle()` |
| `handler/chat_v2.go` | Modify `:115-154`, `:334-349` | Replace sync deterministic titling with async LLM titling + fallback |
| `static/sessions.js` | Rewrite `renderSessionList`, add accordion + per-course create, remove modal wiring | Sidebar render + create/expand logic |
| `static/style.css` | Modify (session sidebar block ~`:891-1009`) | Accordion + course-header + count + `+` + empty-hint styles |
| `static/index.html` | Modify `:64-66`, remove `:133-156` | Header without "+ New"; remove obsolete create modal |

Note on the modal: per-course `+` becomes the **only** create path. The global `+ New` button and the create modal are removed as dead UX (Task 5). If during review you'd rather keep a manual course+topic create, skip Task 5 and leave the modal wired.

---

## Task 1: `GenerateTitle` + `cleanTitle` helper (backend, TDD)

**Files:**
- Modify: `agent/llm.go` (add near `GenerateSummary`, ~`:282`)
- Test: `agent/llm_test.go`

- [ ] **Step 1: Write the failing test**

Add to `agent/llm_test.go` (create the file with `package agent` + `import "testing"` if it doesn't exist):

```go
func TestCleanTitle(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain", "Explain Raft consensus", "Explain Raft consensus"},
		{"strips wrapping double quotes", "\"Raft Consensus Basics\"", "Raft Consensus Basics"},
		{"strips wrapping single quotes", "'Replication Lag'", "Replication Lag"},
		{"trims whitespace", "  Title with spaces  ", "Title with spaces"},
		{"strips trailing period", "Quorum reads and writes.", "Quorum reads and writes"},
		{"collapses inner whitespace/newlines", "Two\n\nWord", "Two Word"},
		{"empty stays empty", "   ", ""},
		{"truncates over 60 runes", "aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff gggg", "aaaaaaaaaa bbbbbbbbbb cccccccccc dddddddddd eeeeeeeeee ffffffffff…"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cleanTitle(c.in); got != c.want {
				t.Errorf("cleanTitle(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestCleanTitle -v`
Expected: FAIL — `undefined: cleanTitle`.

- [ ] **Step 3: Write minimal implementation**

Add to `agent/llm.go`:

```go
const titlePrompt = `You generate a concise title for a study chat session, given the user's opening message. Reply with ONLY the title: 3 to 7 words, no surrounding quotes, no trailing punctuation, no preamble.`

// cleanTitle normalizes a raw model title: trims, strips wrapping quotes,
// collapses internal whitespace, drops a trailing period, and caps length.
func cleanTitle(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ") // collapse whitespace/newlines
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	s = strings.TrimRight(s, ".")
	s = strings.TrimSpace(s)
	const maxRunes = 60
	runes := []rune(s)
	if len(runes) > maxRunes {
		s = string(runes[:maxRunes]) + "…"
	}
	return s
}

// GenerateTitle produces a short session title from the opening user message.
func (c *LLMClient) GenerateTitle(ctx context.Context, firstMessage string) (string, error) {
	msgs := []Message{
		{Role: "system", Content: titlePrompt},
		{Role: "user", Content: firstMessage},
	}
	raw, err := c.CallLLMNonStreaming(ctx, msgs)
	if err != nil {
		return "", err
	}
	return cleanTitle(raw), nil
}
```

Confirm `strings` is already imported in `agent/llm.go` (it is — used elsewhere). If not, add it.

- [ ] **Step 4: Run test to verify it passes**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestCleanTitle -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
cd ~/Documents/ITA/claw-study
git add agent/llm.go agent/llm_test.go
git -c user.email=you@example.com -c user.name=your-name commit -m "feat: add GenerateTitle + cleanTitle for async session titling"
```

---

## Task 2: Async LLM titling in chat_v2 (backend)

**Files:**
- Modify: `handler/chat_v2.go` (the first-turn block `:115-154` and `autoTopic` at `:334-349`)

The current code (lines 115–124) synchronously calls `autoTopic(req.Message)` (truncation) and stores it, then emits `session_topic` at lines 151–154 before the Pi turn. We replace this with: kick off an async LLM title (falling back to `autoTopic` on failure), let the multi-second Pi turn run, then after streaming completes emit `session_topic` if the title arrived.

- [ ] **Step 1: Replace the synchronous titling block**

Replace lines 115–124 (the `var autoSetTopic string … }` block) with:

```go
	// Kick off async title generation on the first turn. The LLM call runs
	// concurrently with the (multi-second) Pi turn; result is emitted after
	// streaming completes. Falls back to a deterministic truncation.
	titleCh := make(chan string, 1)
	if isFirstTurn && sess.Topic == "General" {
		sid := req.SessionID
		firstMsg := req.Message
		go func() {
			title, err := h.LLM.GenerateTitle(context.Background(), firstMsg)
			if err != nil || title == "" {
				slog.Warn("generate session title", "session_id", sid, "err", err)
				title = autoTopic(firstMsg) // deterministic fallback
			}
			if title == "" {
				titleCh <- ""
				return
			}
			if err := h.App.UpdateSessionTopic(sid, title); err != nil {
				slog.Warn("update session topic", "session_id", sid, "err", err)
				titleCh <- ""
				return
			}
			titleCh <- title
		}()
	} else {
		close(titleCh) // no title this turn; recv yields ""
	}
```

- [ ] **Step 2: Remove the now-premature SSE emit**

Delete lines 151–154 (the `if autoSetTopic != "" { … writeSSEEvent(..., "session_topic", ...) }` block) — the title isn't ready yet at this point.

- [ ] **Step 3: Emit `session_topic` after the Pi turn completes**

Find where the turn finishes and `done` is emitted (`writeSSEEvent(w, flusher, "done", …)` around `:287`). Immediately **before** that `done` emit, add:

```go
	// Title generation almost always finishes before the Pi turn does; if so,
	// tell the client to refresh the sidebar. If not ready, the frontend picks
	// it up on its next sessions reload.
	select {
	case title := <-titleCh:
		if title != "" {
			data, _ := json.Marshal(map[string]string{"topic": title})
			writeSSEEvent(w, flusher, "session_topic", string(data))
		}
	default:
	}
```

Confirm `encoding/json` is imported in `chat_v2.go` (it is — `json.Marshal` already used at `:152`).

- [ ] **Step 4: Verify the package builds and existing tests pass**

Run: `/opt/homebrew/bin/go build . && /opt/homebrew/bin/go test ./handler/ ./agent/`
Expected: build OK; tests PASS. If `chat_v2_test.go` asserted on the old synchronous `session_topic`-before-stream ordering, update that assertion to expect the event after the stream (or its absence in a mocked-LLM test). Read the failing assertion and adjust to match the new ordering — do not delete coverage.

- [ ] **Step 5: Commit**

```bash
git add handler/chat_v2.go
git -c user.email=you@example.com -c user.name=your-name commit -m "feat: async LLM session titling on first turn, deterministic fallback"
```

---

## Task 3: Accordion styles (frontend CSS)

**Files:**
- Modify: `static/style.css` (session sidebar block, ~`:891-1009`)

- [ ] **Step 1: Add accordion CSS**

Append after the existing `.session-item .session-delete:hover { … }` rule (~`:1009`):

```css
/* === COURSE ACCORDION === */
.course-group {
  border-bottom: 1px solid var(--border);
}
.course-header {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 10px 12px;
  cursor: pointer;
  user-select: none;
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.6px;
  color: var(--text-secondary);
}
.course-header:hover {
  background: var(--bg-sunken);
}
.course-header .course-chevron {
  flex-shrink: 0;
  font-size: 10px;
  width: 10px;
  transition: transform 0.15s;
  color: var(--text-tertiary);
}
.course-group.expanded .course-header .course-chevron {
  transform: rotate(90deg);
}
.course-header .course-dot {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.course-header .course-name {
  flex: 1;
  min-width: 0;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.course-header .course-count {
  flex-shrink: 0;
  font-size: 11px;
  font-weight: 500;
  color: var(--text-tertiary);
}
.course-header .course-add {
  flex-shrink: 0;
  font-size: 15px;
  line-height: 1;
  color: var(--text-tertiary);
  padding: 0 4px;
  border-radius: var(--radius-sm);
}
.course-header .course-add:hover {
  color: var(--accent);
  background: var(--accent-subtle);
}
.course-sessions {
  display: none;
  padding: 2px 8px 8px;
}
.course-group.expanded .course-sessions {
  display: block;
}
.course-empty-hint {
  font-size: 12px;
  color: var(--text-tertiary);
  padding: 6px 12px 10px;
  font-style: italic;
}
```

- [ ] **Step 2: Commit**

```bash
git add static/style.css
git -c user.email=you@example.com -c user.name=your-name commit -m "style: course accordion styles for sidebar"
```

(Visual verification happens in Task 6 after deploy — no JS test runner exists.)

---

## Task 4: Accordion render + per-course create (frontend JS)

**Files:**
- Modify: `static/sessions.js`

- [ ] **Step 1: Add expanded-course persistence helpers**

Add near the top of `static/sessions.js`, after the `let allSessions = [];` line (~`:18`):

```js
const EXPANDED_KEY = 'claw-study:expandedCourse';

function getExpandedCourse() {
  return localStorage.getItem(EXPANDED_KEY); // string courseId, '' for General, or null
}
function setExpandedCourse(courseId) {
  if (courseId === null) localStorage.removeItem(EXPANDED_KEY);
  else localStorage.setItem(EXPANDED_KEY, courseId);
}
```

- [ ] **Step 2: Rewrite `renderSessionList`**

Replace the entire `renderSessionList()` function (`:55-94`) with:

```js
function renderSessionList() {
  const container = document.getElementById('session-list');

  // Group sessions by course, newest-first within each course.
  const byCourse = {};
  for (const s of allSessions) {
    const key = s.course_id || '';
    (byCourse[key] = byCourse[key] || []).push(s);
  }
  for (const key of Object.keys(byCourse)) {
    byCourse[key].sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));
  }

  // Which course is expanded: stored choice, else the active session's course,
  // else the first course with sessions.
  let expanded = getExpandedCourse();
  if (expanded === null) {
    const active = allSessions.find((s) => s.id === activeSessionId);
    if (active) expanded = active.course_id || '';
  }

  // Always show every defined course (so '+' can launch into any).
  const order = Object.keys(courseMeta);

  let html = '';
  for (const key of order) {
    const meta = courseMeta[key];
    const sessions = byCourse[key] || [];
    const isExpanded = expanded === key;
    html += `<div class="course-group${isExpanded ? ' expanded' : ''}" data-course="${key}">
      <div class="course-header" data-action="toggle-course" data-course="${key}">
        <span class="course-chevron">&#x25B6;</span>
        <span class="course-dot" style="background:${meta.color}"></span>
        <span class="course-name">${escapeHtml(meta.name)}</span>
        <span class="course-count">${sessions.length || ''}</span>
        <span class="course-add" data-action="create-in-course" data-course="${key}" title="New session">&#x2b;</span>
      </div>
      <div class="course-sessions">`;
    if (sessions.length === 0) {
      html += `<div class="course-empty-hint">No sessions yet — + to start</div>`;
    } else {
      for (const s of sessions) {
        const isActive = s.id === activeSessionId;
        html += `<div class="session-item${isActive ? ' active' : ''}" data-session-id="${s.id}" data-action="switch-session">
          <div style="display:flex;justify-content:space-between;align-items:center;">
            <div class="session-topic-wrap">
              <span class="session-topic" data-action="start-rename" data-session-id="${s.id}">${escapeHtml(s.topic || 'Untitled')}</span>
              <span class="session-rename-btn" data-action="start-rename" data-session-id="${s.id}" title="Rename">&#x270E;</span>
            </div>
            <span class="session-delete" data-action="delete-session" data-session-id="${s.id}" title="Delete">&#x2715;</span>
          </div>
        </div>`;
      }
    }
    html += `</div></div>`;
  }
  container.innerHTML = html;
}
```

(Note: the `formatTimeAgo` helper at `:96` is now unused by the renderer. Leave it — it may be used elsewhere; removing is out of scope. Verify with `grep -n formatTimeAgo static/*.js` and only delete if it has zero other references.)

- [ ] **Step 3: Add the per-course create function**

Add after `createSession()` (~`:225`):

```js
export async function createSessionInCourse(courseId) {
  try {
    const resp = await fetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ course_id: courseId, topic: '' }),
    });
    if (!resp.ok) throw new Error('HTTP ' + resp.status);
    const session = await resp.json();
    setExpandedCourse(courseId); // keep the course we just created in open
    activeSessionId = session.id;
    await loadActiveSession();
    await loadSessions();
    document.getElementById('messages').innerHTML = '';
  } catch (err) {
    console.error('Failed to create session', err);
    showErrorBanner('Failed to create session: ' + err.message);
  }
}
```

- [ ] **Step 4: Wire the new click actions in `initSessionsUI`**

In `initSessionsUI()` (`:290`), the existing `session-list` click listener only handles `start-rename`. Replace that listener body with a dispatch covering the new actions (place this as the first listener in the function, replacing the current `start-rename`-only one at `:291-296`):

```js
  document.getElementById('session-list').addEventListener('click', function (e) {
    const renameEl = e.target.closest('[data-action="start-rename"]');
    if (renameEl) {
      e.stopPropagation();
      startRenameSession(parseInt(renameEl.dataset.sessionId, 10));
      return;
    }
    const addEl = e.target.closest('[data-action="create-in-course"]');
    if (addEl) {
      e.stopPropagation();
      createSessionInCourse(addEl.dataset.course);
      return;
    }
    const delEl = e.target.closest('[data-action="delete-session"]');
    if (delEl) {
      e.stopPropagation();
      deleteSession(parseInt(delEl.dataset.sessionId, 10));
      return;
    }
    const headerEl = e.target.closest('[data-action="toggle-course"]');
    if (headerEl) {
      const course = headerEl.dataset.course;
      setExpandedCourse(getExpandedCourse() === course ? null : course);
      renderSessionList();
      return;
    }
    const switchEl = e.target.closest('[data-action="switch-session"]');
    if (switchEl) {
      switchSession(parseInt(switchEl.dataset.sessionId, 10));
    }
  });
```

> Verify how session-switch and delete are currently wired before this task: `grep -n "switch-session\|delete-session\|addEventListener" static/sessions.js static/app.js`. If a delegated listener in `app.js` already handles `switch-session`/`delete-session`, do NOT duplicate it here — drop those two branches and keep only `start-rename`, `create-in-course`, and `toggle-course`. The goal is exactly one handler per action.

- [ ] **Step 5: Verify the empty-state path still renders**

The old `renderSessionList` had a "No sessions yet" early return for a totally empty `allSessions`. The new version shows all courses (each empty with a hint), which is the desired behavior — no early return needed. Confirm by reading the new function: with `allSessions = []`, every course renders collapsed with count `''` and, when expanded, the empty hint. Good.

- [ ] **Step 6: Build check (syntax) + commit**

There is no JS test runner. Sanity-check syntax by serving locally is done in Task 6. For now, eyeball that braces/exports balance, then commit:

```bash
git add static/sessions.js
git -c user.email=you@example.com -c user.name=your-name commit -m "feat: course-first accordion sidebar with per-course instant create"
```

---

## Task 5: Remove obsolete create modal + global "+ New" (frontend HTML/JS)

**Files:**
- Modify: `static/index.html` (`:64-66`, `:133-156`)
- Modify: `static/sessions.js` (modal functions + their wiring)

> If at review you want to keep a manual course+topic create path, SKIP this task entirely. Per-course `+` already works without it.

- [ ] **Step 1: Simplify the sidebar header**

In `static/index.html`, replace lines 64–67:

```html
        <div id="session-sidebar-header">
          <h2>Sessions</h2>
          <button id="new-session-btn">+ New</button>
        </div>
```

with:

```html
        <div id="session-sidebar-header">
          <h2>Sessions</h2>
        </div>
```

- [ ] **Step 2: Remove the modal markup**

In `static/index.html`, delete the entire `#session-modal-overlay` block (lines ~133–156, from `<div id="session-modal-overlay">` through its closing `</div>`). Read the block first to get exact boundaries.

- [ ] **Step 3: Remove dead modal JS**

In `static/sessions.js`, remove the now-unreferenced functions and listeners: `openSessionModal`, `closeSessionModal`, `createSession`, and in `initSessionsUI` the listeners referencing `session-modal-cancel`, `session-modal-create`, `new-session-btn`, `session-course`, `session-modal-overlay`, `session-topic`. Also remove any `import`/usage of `openSessionModal`/`createSession` elsewhere — run `grep -rn "openSessionModal\|createSession\b\|new-session-btn\|session-modal" static/` and clear every hit. Keep `createSessionInCourse`.

- [ ] **Step 4: Verify no dangling references**

Run: `grep -rn "openSessionModal\|session-modal\|new-session-btn" static/`
Expected: no matches (or only inside comments you then remove).

- [ ] **Step 5: Commit**

```bash
git add static/index.html static/sessions.js
git -c user.email=you@example.com -c user.name=your-name commit -m "refactor: remove obsolete create modal; per-course + is the only create path"
```

---

## Task 6: Build, deploy, manual verification

**Files:** none (build + deploy + verify)

- [ ] **Step 1: Full build + test**

```bash
cd ~/Documents/ITA/claw-study
/opt/homebrew/bin/go vet ./... && /opt/homebrew/bin/go test ./...
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
ls -la /tmp/study-app-linux   # expect ~20 MB ELF
```
Expected: vet clean, tests PASS, binary built.

- [ ] **Step 2: Deploy**

```bash
scp /tmp/study-app-linux nanoclaw:$VAULT_ROOT/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && mv study-app.new study-app && chmod +x study-app && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service'
ssh nanoclaw 'export XDG_RUNTIME_DIR=/run/user/$(id -u); systemctl --user is-active study-app.service study-app-tunnel.service'
```
Expected: two `active` lines.

- [ ] **Step 3: Manual verification checklist** (hard-refresh `https://your-host.example`, Cmd+Shift+R)

  - [ ] All courses listed, collapsed, with correct counts; long course names ellipsize.
  - [ ] Clicking a course header expands it; opening another collapses the first (single-open).
  - [ ] Reload the page → the same course stays expanded (localStorage persists).
  - [ ] Expanding a course with no sessions shows "No sessions yet — + to start".
  - [ ] Clicking a course's `+` creates a session in that course, makes it active, clears the chat, and keeps that course expanded. No modal appears.
  - [ ] The new row shows "Untitled"; after sending the first message and the turn finishing, the row updates to an LLM-generated title (verify via `journalctl` if needed; on LLM failure it falls back to the truncated message).
  - [ ] Hover a session row → pencil + ✕ appear; rename inline works; delete works.
  - [ ] Long session titles ellipsize and don't overflow the drawer (the earlier fix still holds).

- [ ] **Step 4: Confirm titling in logs (optional)**

```bash
ssh nanoclaw 'journalctl --user -u study-app.service --since "5 min ago" --no-pager | grep -i "title\|topic"'
```
Expected: no `generate session title` errors on a healthy turn (or, if present, the fallback still set a topic).

- [ ] **Step 5: Push**

```bash
cd ~/Documents/ITA/claw-study && git push
```

---

## Self-Review

**Spec coverage** (against ADR 0008 + the grilling decisions):
- Single-open accordion, all courses always, active course open on load, persisted → Task 4 (`getExpandedCourse`, single-key localStorage, default to active session's course). ✅
- Per-course `+` instant create, no modal → Task 4 (`createSessionInCourse`) + Task 5 (modal removal). ✅
- Async LLM title from first message, deterministic fallback → Task 1 + Task 2. ✅
- Newest-first within course → Task 4 (`.sort` on `updated_at`). ✅
- Title-only rows, hover rename/delete → Task 4 render (no meta line). ✅
- Empty-course hint → Task 3 (`.course-empty-hint`) + Task 4 render. ✅
- Keep active pill → untouched (no task modifies `updateSessionPill`). ✅
- No recents/continue row → never rendered. ✅

**Placeholder scan:** No TBD/"handle errors"/"similar to" — every code step is concrete. ✅

**Type/name consistency:** `createSessionInCourse`, `getExpandedCourse`/`setExpandedCourse`, `EXPANDED_KEY`, `GenerateTitle`, `cleanTitle`, `titleCh`, `session_topic` event name — all used consistently across tasks. The `data-action` values (`toggle-course`, `create-in-course`, `switch-session`, `delete-session`, `start-rename`) match between the render HTML (Task 4 Step 2) and the click dispatch (Task 4 Step 4). ✅
