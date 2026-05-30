# Phase 3b-1 — Plan-Spine Rail + Task Workspace (Frontend) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for INLINE execution with browser-verified checkpoints. This frontend has **no JS test runner** (vanilla JS, no build step, ADR 0004), so every task's "test" is a **manual browser verification** (use the `verify`/`run` skills against a locally-run binary), not a unit test. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Replace the flat session sidebar with the **Plan as the navigation spine** — a left rail of course-switcher → phases → tasks, where clicking a task opens its (lazily-created) Session as the center workspace; nothing becomes unreachable.

**Architecture:** A new `static/rail.js` module owns the left rail: it fetches the selected course's full plan (`/api/plan?view=full&id=`) plus the sessions list (now carrying `task_id`), builds a `task_id → session` map, and renders phases→tasks with a done-checkbox and a "has-work" dot. Clicking a task resolves its Session via `/api/sessions/for-task` (GET = lookup; if absent, hold a *pending-task* workspace and POST-create on the first message — the lazy hook from 3a). `sessions.js` keeps session state/message-loading/pill but loses its course-accordion renderer. A collapsed **"Other"** list at the rail bottom keeps task-less/archived chats reachable until 3b-2 builds the proper Scratch/archive/Detached buckets.

**Tech Stack:** Vanilla ES modules (no framework, no build), document-level `[data-action]` click delegation (see `static/app.js`), `apiFetch` wrapper, embedded static (`//go:embed static/*` in `main.go` — JS changes ship in the Go binary). Backend endpoints from Phase 3a are live: `GET/POST /api/sessions/for-task`, `task_id` on the `Session` JSON, `/api/plan?view=full&id=`, `/api/plan/toggle`.

**Spec:** [ADR 0011](../../adr/0011-plan-is-navigation-spine.md), [ADR 0014](../../adr/0014-phase3-task-anchored-sessions-data-model.md). Glossary: [CONTEXT.md](../../CONTEXT.md) (*Plan*, *Session*, *Scratch*).

**Out of scope (→ Plan 3b-2):** the polished Scratch area, the "Before the redesign" archived group, the Detached bucket (orphaned `task_id`), and reading auto-open to the task's learned PDF. 3b-1 keeps those reachable via a minimal "Other" stub but does not bucket them.

**Sync/deploy:** All changes are `static/*` + (optionally) one rename in `index.html` — everything is embedded in the `study-app` binary. Deploy = `go build` + scp + swap + restart (Task 9). No `claw-cli`, no disk-file sync.

---

## Local run (used by every verification step)

The app reads config from env and serves on `cfg.ListenAddr`. For UI verification, run a local instance against a **scratch vault** so you never touch prod data:

```bash
# from repo root; scratch vault under /tmp so prod data is untouched
mkdir -p /tmp/claw-rail-vault/data
# seed a plan + a couple of sessions to exercise the rail (see Task 1 Step 2)
VAULT_ROOT=/tmp/claw-rail-vault LISTEN_ADDR=127.0.0.1:8099 AUTH_TOKEN= /opt/homebrew/bin/go run .
# then open http://127.0.0.1:8099/  (AUTH_TOKEN empty disables the bearer gate for local dev)
```

> Confirm the exact env var names first: `grep -n "os.Getenv\|LISTEN\|VAULT\|AUTH" loadConfig` in the repo (the config loader is in a `loadConfig()` near `main.go`). Use whatever names it reads. If an empty `AUTH_TOKEN` does NOT disable auth, read the auth middleware (`handler/auth.go`) and pass the token it expects as a `Authorization: Bearer` header in the browser via a `?token=` shim if one exists, or set the token env and use it. Chat-send (Task 6) additionally needs an LLM/Pi config; rail rendering + task-click + session-load (Tasks 2–5, 7, 8) do NOT need an LLM and work against the scratch vault alone.

Rebuild between tasks by restarting `go run .` (static is recompiled each run; `Cache-Control: no-store` means a plain browser refresh shows changes).

---

## File Structure

- `static/rail.js` — **NEW.** The plan-spine rail: data load, render, course switch, task open, done toggle, the "Other" stub. One clear responsibility (left-rail navigation).
- `static/sessions.js` — **Modify.** Keep `courseMeta`, session state (`activeSessionId` get/set), `loadSessionMessages`, `loadActiveSession`, `deleteSession`, `updateSessionPill`. **Remove** `renderSessionList` + the course-accordion `initSessionsUI` click handler (the rail replaces them). Export a small `clearWorkspace()` helper.
- `static/chat.js` — **Modify.** Before sending, if the rail has a pending task with no Session, create it via `/api/sessions/for-task` and set it active, then send.
- `static/app.js` — **Modify.** Import + init the rail; add `select-course` / `open-task` / `toggle-task` data-actions; drop the retired session-accordion actions.
- `static/index.html` — **Modify.** Rename the sidebar header "Sessions" → "Plan"; the `#session-sidebar`/`#session-list` containers are reused as the rail mount.
- `static/style.css` — **Modify.** Add rail-specific classes (course switcher, task row, has-work dot, Other stub). Reuse existing `.topic-row`/`.topic-checkbox` look where possible.

---

## Task 1: Rail data layer (`rail.js` skeleton + data load)

**Files:** Create `static/rail.js`.

- [ ] **Step 1: Create `static/rail.js` with the data layer.**

```js
// Plan-spine rail: the selected course's plan (phases → tasks) is the left-rail
// navigator. Tasks resolve to Sessions via /api/sessions/for-task (lazy).
import { apiFetch } from './apiFetch.js';
import { courseMeta } from './sessions.js';

const SELECTED_COURSE_KEY = 'claw-study:railCourse';

let selectedCourse = localStorage.getItem(SELECTED_COURSE_KEY) || 'ce297';
let currentPlan = null;            // JSONPlan for selectedCourse, or null
let sessionsByTaskId = {};         // task_id -> session (has-work lookup)
let otherSessions = [];            // task_id-less sessions (Scratch stub)
let pendingTask = null;            // {courseId, taskId, title} when a task is open with no session yet

export function getSelectedCourse() {
  return selectedCourse;
}
export function getPendingTask() {
  return pendingTask;
}
export function clearPendingTask() {
  pendingTask = null;
}

// loadRailData fetches the selected course's full plan and the sessions list,
// then indexes sessions by task_id (for has-work dots and task→session resolution).
export async function loadRailData() {
  try {
    const [planResp, sessResp] = await Promise.all([
      apiFetch('/api/plan?view=full&id=' + encodeURIComponent(selectedCourse)),
      apiFetch('/api/sessions'),
    ]);
    currentPlan = planResp.ok ? await planResp.json() : null;
    const sessions = await sessResp.json();
    sessionsByTaskId = {};
    otherSessions = [];
    for (const s of sessions || []) {
      if (s.task_id) sessionsByTaskId[s.task_id] = s;
      else otherSessions.push(s);
    }
  } catch (err) {
    console.error('Failed to load rail data', err);
    currentPlan = null;
  }
}
```

- [ ] **Step 2: Seed the scratch vault for verification.** Create a minimal plan + sessions so the rail has something to show. With the local server NOT yet running:

```bash
mkdir -p /tmp/claw-rail-vault/data/plans
cat > /tmp/claw-rail-vault/data/plans/ce297.json <<'JSON'
{"id":"ce297","name":"CE-297 Safety","phases":[
  {"title":"Phase 1 — Foundations","tasks":[
    {"id":"t-aaa","title":"Read Laprie taxonomy","done":true},
    {"id":"t-bbb","title":"STAMP vs chain causality","done":false}
  ]},
  {"title":"Phase 2 — Analysis","tasks":[
    {"id":"t-ccc","title":"Event Tree Analysis","done":false}
  ]}
]}
JSON
```

- [ ] **Step 3: Verify the module loads (no render yet).** Temporarily add `window.__rail = { loadRailData, getSelectedCourse };` at the end of `rail.js`, start the local server (see "Local run"), open the page, and in the browser console run `await __rail.loadRailData()` then inspect — no errors, and a follow-up `console.log` you add prints the plan. Expected: the plan object with 2 phases. **Remove the `window.__rail` line after confirming.**

- [ ] **Step 4: Commit.**

```bash
git add static/rail.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(rail): plan-spine rail data layer — load plan + index sessions by task_id (ADR 0011)"
```

---

## Task 2: Render the rail (course switcher + phases → tasks + has-work dots)

**Files:** Modify `static/rail.js`, `static/index.html`, `static/app.js`, `static/style.css`.

- [ ] **Step 1: Rename the sidebar header in `index.html`.** Change:

```html
        <div id="session-sidebar-header">
          <h2>Sessions</h2>
        </div>
```

to:

```html
        <div id="session-sidebar-header">
          <h2>Plan</h2>
        </div>
```

- [ ] **Step 2: Add `renderRail` + `initRail` to `rail.js`.** Append:

```js
import { escapeHtml } from './dom.js';

// Linear task index across phases+clusters — matches the backend's toggle index
// (handler/plan.go toggleTaskAt walks phases' tasks then each cluster's tasks).
function walkTasks(plan, fn) {
  let idx = 0;
  for (const phase of plan.phases || []) {
    fn({ kind: 'phase', title: phase.title });
    for (const task of phase.tasks || []) fn({ kind: 'task', task, idx: idx++ });
    for (const cluster of phase.clusters || []) {
      fn({ kind: 'cluster', title: cluster.title });
      for (const task of cluster.tasks || []) fn({ kind: 'task', task, idx: idx++ });
    }
  }
}

function firstUndoneTaskId(plan) {
  let found = null;
  walkTasks(plan, (n) => {
    if (found) return;
    if (n.kind === 'task' && !n.task.done) found = n.task.id;
  });
  return found;
}

export function renderRail() {
  const container = document.getElementById('session-list');
  const switcher = renderCourseSwitcher();
  if (!currentPlan || !currentPlan.phases || currentPlan.phases.length === 0) {
    container.innerHTML = switcher + '<div class="rail-empty">No plan for this course yet.</div>' + renderOther();
    return;
  }
  const nextId = firstUndoneTaskId(currentPlan);
  let html = switcher + '<div class="rail-plan">';
  walkTasks(currentPlan, (n) => {
    if (n.kind === 'phase') {
      html += `<div class="rail-phase">${escapeHtml(n.title)}</div>`;
    } else if (n.kind === 'cluster') {
      html += `<div class="rail-cluster">${escapeHtml(n.title)}</div>`;
    } else {
      const t = n.task;
      const hasWork = !!sessionsByTaskId[t.id];
      const isNext = t.id === nextId;
      html += `<div class="rail-task${t.done ? ' done' : ''}${isNext ? ' next' : ''}" data-action="open-task" data-task-id="${escapeHtml(t.id)}" data-idx="${n.idx}">
        <span class="rail-check${t.done ? ' done' : ''}" data-action="toggle-task" data-idx="${n.idx}">${t.done ? '&#x2713;' : ''}</span>
        <span class="rail-task-title">${escapeHtml(t.title)}</span>
        <span class="rail-work-dot" style="visibility:${hasWork ? 'visible' : 'hidden'}" title="Has work"></span>
      </div>`;
    }
  });
  html += '</div>' + renderOther();
  container.innerHTML = html;
}

function renderCourseSwitcher() {
  let opts = '';
  for (const id of Object.keys(courseMeta)) {
    if (id === '') continue; // skip the "General" pseudo-course in the switcher
    const sel = id === selectedCourse ? ' selected' : '';
    opts += `<option value="${escapeHtml(id)}"${sel}>${escapeHtml(courseMeta[id].name)}</option>`;
  }
  return `<select id="rail-course-select" class="rail-course-select" data-action="noop">${opts}</select>`;
}

function renderOther() {
  if (!otherSessions.length) return '';
  let html = '<div class="rail-other"><div class="rail-other-label">Other chats</div>';
  for (const s of otherSessions) {
    html += `<div class="rail-other-item" data-action="switch-session" data-session-id="${s.id}">${escapeHtml(s.topic || 'Untitled')}</div>`;
  }
  return html + '</div>';
}

export function initRail() {
  // Course switcher: change event (not a click data-action) swaps the rail.
  document.getElementById('session-list').addEventListener('change', (e) => {
    const sel = e.target.closest('#rail-course-select');
    if (!sel) return;
    selectCourse(sel.value);
  });
}

export async function selectCourse(courseId) {
  selectedCourse = courseId;
  localStorage.setItem(SELECTED_COURSE_KEY, courseId);
  await loadRailData();
  renderRail();
}

// loadRail is the public entry: load data for the selected course, then render.
export async function loadRail() {
  await loadRailData();
  renderRail();
}
```

- [ ] **Step 3: Add rail CSS to `static/style.css`.** Append:

```css
.rail-course-select { width: 100%; margin: 8px 0 12px; padding: 6px 8px; font: inherit; font-size: 13px; border: 1px solid var(--border); border-radius: var(--radius-sm); background: var(--bg-surface); color: var(--text-primary); }
.rail-phase { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.3px; color: var(--text-secondary); margin: 14px 0 6px; }
.rail-cluster { font-size: 11px; font-weight: 600; color: var(--text-secondary); margin: 8px 0 4px 6px; }
.rail-task { display: flex; align-items: center; gap: 6px; padding: 5px 6px; border-radius: var(--radius-sm); cursor: pointer; }
.rail-task:hover { background: var(--bg-sunken); }
.rail-task.active { background: var(--bg-sunken); }
.rail-task.next { box-shadow: inset 2px 0 0 var(--accent, #2563EB); }
.rail-task-title { flex: 1; font-size: 13px; line-height: 1.3; overflow-wrap: anywhere; }
.rail-task.done .rail-task-title { color: var(--text-tertiary); text-decoration: line-through; }
.rail-check { width: 15px; height: 15px; flex: 0 0 15px; border: 1.5px solid var(--border); border-radius: 3px; display: inline-flex; align-items: center; justify-content: center; font-size: 11px; cursor: pointer; }
.rail-check.done { background: var(--accent, #2563EB); color: #fff; border-color: var(--accent, #2563EB); }
.rail-work-dot { width: 6px; height: 6px; flex: 0 0 6px; border-radius: 50%; background: var(--accent, #2563EB); }
.rail-empty { color: var(--text-tertiary); font-size: 13px; padding: 16px 4px; }
.rail-other { margin-top: 20px; border-top: 1px solid var(--border); padding-top: 10px; }
.rail-other-label { font-size: 11px; font-weight: 700; text-transform: uppercase; letter-spacing: 0.3px; color: var(--text-tertiary); margin-bottom: 6px; }
.rail-other-item { font-size: 12px; color: var(--text-secondary); padding: 4px 6px; border-radius: var(--radius-sm); cursor: pointer; overflow-wrap: anywhere; }
.rail-other-item:hover { background: var(--bg-sunken); }
```

- [ ] **Step 4: Wire init + replace the old session-list bootstrap in `app.js`.** In `static/app.js`:
  - Add to the imports: `import { initRail, loadRail } from './rail.js';`
  - In the `[data-action]` switch, add a `noop` case so the course `<select>` doesn't fall through: `case 'noop': break;`
  - Replace the `initSessionsUI()` call with `initRail()`.
  - In the `initApp` IIFE, replace `await loadSessions();` with `await loadRail();` (keep `loadCourses()` before it — the rail switcher reads `courseMeta`).

- [ ] **Step 5: Verify in browser.** Restart the local server, refresh. Expected: the left rail shows a course `<select>` (defaulting to CE-297), the two phases as headers, three tasks, the first task checked/struck-through, "STAMP vs chain causality" marked as **next** (left accent bar). No has-work dots yet (no sessions seeded). No console errors.

- [ ] **Step 6: Commit.**

```bash
git add static/rail.js static/index.html static/app.js static/style.css
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(rail): render plan-spine rail (course switcher, phases, tasks, next/done) (ADR 0011)"
```

---

## Task 3: Course switcher swaps the rail

**Files:** none new — `initRail`/`selectCourse` from Task 2 already implement it. This task is **verification + a second seeded plan**.

- [ ] **Step 1: Seed a second course plan.**

```bash
cat > /tmp/claw-rail-vault/data/plans/ddia.json <<'JSON'
{"id":"ddia","name":"DDIA","phases":[{"title":"Phase 3 — Transactions","tasks":[
  {"id":"d-1","title":"Weak isolation: Read Committed","done":false}
]}]}
JSON
```

- [ ] **Step 2: Verify.** Restart server, refresh. Change the course `<select>` from CE-297 to DDIA. Expected: the rail instantly swaps to DDIA's single phase/task; reloading the page keeps DDIA selected (persisted in `localStorage` under `claw-study:railCourse`). Switch back to CE-297 → its plan returns.

- [ ] **Step 3: Commit (if any tweak was needed; otherwise skip).** If Step 2 required a fix, commit it:

```bash
git add static/rail.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "fix(rail): course switcher persistence/swap"
```

---

## Task 4: Task click → load existing Session

**Files:** Modify `static/rail.js`, `static/app.js`, `static/sessions.js`.

- [ ] **Step 1: Export a `clearWorkspace` helper from `sessions.js`.** Add near `loadSessionMessages`:

```js
// clearWorkspace empties the chat pane (used when opening an as-yet-empty task).
export function clearWorkspace() {
  document.getElementById('messages').innerHTML = '';
}
```

- [ ] **Step 2: Add `openTask` to `rail.js`.** Append (imports at top of file: add `setActiveSessionId, loadSessionMessages, updateSessionPill, clearWorkspace` to the existing `./sessions.js` import — note `updateSessionPill` must be exported from sessions.js; if it is not yet exported, add `export` to its declaration there):

```js
import {
  setActiveSessionId,
  loadSessionMessages,
  clearWorkspace,
} from './sessions.js';

// openTask resolves the Session for a task. If one exists, it is activated and
// its chat loaded. If not, a pending-task workspace is shown (lazy: the row is
// created on the first message — see chat.js).
export async function openTask(taskId) {
  const title = taskTitleById(taskId);
  markActiveTask(taskId);
  try {
    const resp = await apiFetch(
      '/api/sessions/for-task?course_id=' + encodeURIComponent(selectedCourse) +
      '&task_id=' + encodeURIComponent(taskId),
    );
    const data = await resp.json();
    if (data && data.id) {
      pendingTask = null;
      await fetch('/api/sessions/active', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: data.id }),
      });
      setActiveSessionId(data.id);
      await loadSessionMessages();
    } else {
      // No session yet — hold a pending task; the workspace is empty until the
      // first message creates the row.
      pendingTask = { courseId: selectedCourse, taskId, title };
      setActiveSessionId(null);
      clearWorkspace();
    }
  } catch (err) {
    console.error('Failed to open task', err);
  }
}

function taskTitleById(taskId) {
  let title = '';
  if (currentPlan) {
    walkTasks(currentPlan, (n) => {
      if (n.kind === 'task' && n.task.id === taskId) title = n.task.title;
    });
  }
  return title;
}

function markActiveTask(taskId) {
  document.querySelectorAll('.rail-task').forEach((el) => {
    el.classList.toggle('active', el.dataset.taskId === taskId);
  });
}
```

- [ ] **Step 3: Wire the `open-task` action in `app.js`.** Add to the imports: `import { openTask } from './rail.js';` and in the `[data-action]` switch add:

```js
    case 'open-task':
      openTask(el.dataset.taskId);
      break;
```

(The existing document-level delegator already runs for rail elements. Ensure `toggle-task` — Task 7 — is handled before `open-task` so a checkbox click doesn't also open the task; see Task 7.)

- [ ] **Step 4: Seed a session anchored to a task** (so "load existing" has something to load). With the local server running, in the browser console (or via curl against `127.0.0.1:8099`):

```bash
curl -s -XPOST 127.0.0.1:8099/api/sessions/for-task -H 'Content-Type: application/json' \
  -d '{"course_id":"ce297","task_id":"t-aaa","topic":"Laprie taxonomy"}'
# then add a message so the chat pane shows something
curl -s "127.0.0.1:8099/api/sessions/for-task?course_id=ce297&task_id=t-aaa"
```

(If chat-send isn't wired locally, this confirms the session exists; the message pane may be empty, which is fine for this task.)

- [ ] **Step 5: Verify.** Refresh. The "Read Laprie taxonomy" task should now show a **has-work dot**. Click it → it becomes the active task (left accent / `.active`), and the center pane loads that session's messages (empty if none). Click "STAMP vs chain causality" (no session) → center pane clears, no error, and `GET /api/sessions` count does NOT increase (verify: `curl 127.0.0.1:8099/api/sessions | json length unchanged`). No row created yet — that is the lazy behavior.

- [ ] **Step 6: Commit.**

```bash
git add static/rail.js static/app.js static/sessions.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(rail): task click resolves/loads its Session; pending-task for empties (ADR 0014 lazy)"
```

---

## Task 5: Empty-task workspace affordance (pending-task header)

**Files:** Modify `static/rail.js` (`openTask` pending branch), `static/index.html` (a workspace header), `static/style.css`.

Make the empty (pending) task state legible: show the task title above the chat input so the learner knows what they're about to start.

- [ ] **Step 1: Add a workspace banner element to `index.html`.** Inside `#chat-panel`, immediately before `<div id="messages">`, add:

```html
        <div id="workspace-banner" style="display:none"></div>
```

- [ ] **Step 2: Render the banner in `rail.js`.** Add a helper and call it from both branches of `openTask`:

```js
function setBanner(text) {
  const el = document.getElementById('workspace-banner');
  if (!text) { el.style.display = 'none'; el.textContent = ''; return; }
  el.style.display = 'block';
  el.textContent = text;
}
```

In `openTask`: in the existing-session branch call `setBanner(title)`; in the pending branch call `setBanner(title + '  ·  new — your first message starts it')`.

- [ ] **Step 3: CSS for the banner.** Append to `style.css`:

```css
#workspace-banner { padding: 8px 16px; font-size: 12px; font-weight: 600; color: var(--text-secondary); border-bottom: 1px solid var(--border); background: var(--bg-sunken); }
```

- [ ] **Step 4: Verify.** Click an empty task → banner shows "<title> · new — your first message starts it"; click a task with work → banner shows just the title. No console errors.

- [ ] **Step 5: Commit.**

```bash
git add static/rail.js static/index.html static/style.css
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(rail): workspace banner shows the open task (pending vs existing)"
```

---

## Task 6: Lazy create on first message (chat-send through `/api/sessions/for-task`)

**Files:** Modify `static/chat.js`, `static/rail.js`.

The pending task has no Session until the learner sends. On send, create it, set active, then proceed — and refresh the rail so the has-work dot appears.

- [ ] **Step 1: Read the current send path.** `grep -n "function\|sendMessage\|activeSession\|getActiveSessionId\|fetch(\|EventSource\|/chat" static/chat.js` — find where a message is sent and how the active session id is obtained. The send handler currently requires an active session.

- [ ] **Step 2: Add a pre-send hook in `chat.js`.** Import the rail's pending-task accessors at the top: `import { getPendingTask, clearPendingTask, loadRail } from './rail.js';` and `import { setActiveSessionId } from './sessions.js';`. At the very start of the send handler (before it reads the active session id / posts the message), insert:

```js
  // Lazy task-session creation (ADR 0014): if a task is open but has no Session
  // yet, create it now, on the first message.
  const pending = getPendingTask();
  if (pending) {
    try {
      const resp = await fetch('/api/sessions/for-task', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ course_id: pending.courseId, task_id: pending.taskId, topic: pending.title }),
      });
      if (!resp.ok) throw new Error('HTTP ' + resp.status);
      const session = await resp.json();
      setActiveSessionId(session.id);
      await fetch('/api/sessions/active', {
        method: 'PUT', headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: session.id }),
      });
      clearPendingTask();
    } catch (err) {
      console.error('Failed to create task session', err);
      return; // do not send into a void
    }
  }
```

Then ensure the existing code uses the now-active session id (`getActiveSessionId()` should return `session.id`). After the send completes (where the handler already reloads messages / on the `done` event), add `await loadRail();` so the new has-work dot renders.

> If `chat.js` caches the session id in a local variable at module load, adjust so it re-reads `getActiveSessionId()` at send time. Confirm by reading the file in Step 1.

- [ ] **Step 3: Verify (needs an LLM/Pi-capable local config, OR stub).** With a chat-capable local config: open an empty task, type a message, send. Expected: a session is created (`GET /api/sessions` count +1, the new row has `task_id` set), the message sends, and after the turn the rail shows a **has-work dot** on that task. Without an LLM locally: verify the two network calls fire (POST `/api/sessions/for-task` then the chat POST) via the browser Network tab, and that `GET /api/sessions/for-task?...` now returns the row.

- [ ] **Step 4: Commit.**

```bash
git add static/chat.js static/rail.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(chat): first message lazily creates the task's Session (ADR 0014)"
```

---

## Task 7: Done-toggle from the rail checkbox

**Files:** Modify `static/app.js`, `static/rail.js`.

Clicking the checkbox toggles the task done WITHOUT opening it. Reuses `/api/plan/toggle` (by linear index — the same index `walkTasks` assigns, matching `handler/plan.go`).

- [ ] **Step 1: Add `toggleTask` to `rail.js`.** Append:

```js
// toggleTask flips a task's done flag via the linear index (matches the backend
// walk order in handler/plan.go) and re-renders the rail.
export async function toggleTask(idx) {
  const fd = new FormData();
  fd.append('course', selectedCourse);
  fd.append('index', String(idx));
  try {
    await fetch('/api/plan/toggle', { method: 'POST', body: fd });
    await loadRail();
  } catch (err) {
    console.error('toggle failed', err);
  }
}
```

- [ ] **Step 2: Wire `toggle-task` in `app.js` BEFORE `open-task`.** In the `[data-action]` switch add (and because the checkbox is nested inside the task row, the checkbox carries its own `data-action="toggle-task"`, so `e.target.closest('[data-action]')` resolves to the checkbox first — add a `stopPropagation` so the row's open-task does not also fire):

```js
    case 'toggle-task':
      e.stopPropagation();
      toggleTask(parseInt(el.dataset.idx, 10));
      break;
```

Add the import: `import { openTask, toggleTask } from './rail.js';` (merge with the Task 4 import line).

- [ ] **Step 3: Verify.** Click the checkbox on "STAMP vs chain causality" → it becomes done (struck through, checkmark) and the **next** accent moves to the following undone task; the center workspace does NOT change (the task did not open). Click again → undone. Confirm the plan file on disk reflects the toggle (`cat /tmp/claw-rail-vault/data/plans/ce297.json`).

- [ ] **Step 4: Commit.**

```bash
git add static/app.js static/rail.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(rail): toggle task done from the rail checkbox"
```

---

## Task 8: Retire the flat session list; keep "Other" reachable

**Files:** Modify `static/sessions.js`, `static/app.js`.

The rail is now the navigator. Remove the dead course-accordion code so it can't double-render or confuse maintenance, while the "Other chats" stub (Task 2) preserves access to task-less sessions.

- [ ] **Step 1: Remove dead code in `sessions.js`.** Delete `renderSessionList` and the `initSessionsUI` body's accordion/rename/create-in-course click wiring (the rail owns the list now). Keep: `courseMeta`, `loadCourses`, `getActiveSessionId`/`setActiveSessionId`, `loadActiveSession`, `loadSessionMessages`, `clearWorkspace`, `switchSession`, `deleteSession`, `updateSessionPill`. If `loadSessions`/`createSessionInCourse`/`startRenameSession` are now unused, delete them. **Grep for each name across `static/` before deleting** to confirm no remaining importer:

```bash
for fn in renderSessionList initSessionsUI loadSessions createSessionInCourse startRenameSession; do echo "== $fn =="; grep -rn "$fn" static/; done
```

Delete only those with no remaining references outside their own definition/export. `switchSession` is still used by the "Other" stub (`data-action="switch-session"`), so keep it.

- [ ] **Step 2: Clean `app.js`.** Remove imports that are now unused (`loadSessions`, `initSessionsUI`, `createSessionInCourse` if deleted) and any retired data-action cases (`create-in-course`, `toggle-course`, `start-rename` — these were handled in `sessions.js`'s own listener which is now gone; if any are referenced in the document-level switch, remove those cases). Keep `switch-session` and `delete-session` cases (the "Other" stub uses them). Run `grep -n "loadSessions\|initSessionsUI\|create-in-course\|toggle-course" static/app.js` and remove stragglers.

- [ ] **Step 3: Verify.** Restart, refresh. Expected: only the rail renders in the left panel (no leftover accordion). The "Other chats" stub shows any task-less sessions and clicking one loads it (via `switchSession`). No console errors, no double-rendered lists. `grep -rn "renderSessionList" static/` returns nothing.

- [ ] **Step 4: Commit.**

```bash
git add static/sessions.js static/app.js
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "refactor(rail): retire the flat session accordion; rail is the navigator (ADR 0011)"
```

---

## Task 9: Deploy — REQUIRES EXPLICIT DEPLOY APPROVAL

**Files:** none (rebuild + ship the `study-app` binary; static is embedded).

> Do not run without the user's explicit "deploy" go-ahead (prod boundary). No DB migration here; this is purely the embedded frontend.

- [ ] **Step 1: Full backend check still green (no Go changed, but confirm).** `git status -sb && /opt/homebrew/bin/go build ./... && /opt/homebrew/bin/go vet ./... && /opt/homebrew/bin/go test ./...` → clean, all `ok`.

- [ ] **Step 2: Build the linux binary.** `GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .` → exit 0.

- [ ] **Step 3: Ship + swap + restart.**
```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak.2026-05-30-phase3b1 && mv study-app.new study-app && chmod +x study-app && systemctl --user restart study-app.service && sleep 3 && systemctl --user is-active study-app.service && systemctl --user is-active study-app-tunnel.service'
```
Expected: both `active`.

- [ ] **Step 4: Verify live.** `curl -s -o /dev/null -w "%{http_code}\n" https://study.claw-study.xyz/` → 401 (auth healthy). Then in a browser at the live URL: the left rail shows the plan spine, switching courses works, clicking a task loads/opens its workspace, the checkbox toggles done, "Other chats" lists task-less sessions. Spot-check the browser console for errors.

- [ ] **Step 5: Push.** `git push origin main` (after merging the branch per finishing-a-development-branch).

---

## Self-Review

**1. Spec coverage:**
- ADR 0011 "left rail = course switcher → plan (phases→tasks)" → Tasks 2, 3. ✓
- ADR 0011 "center = task workspace; opening a Task opens its Session" → Tasks 4, 5, 6. ✓
- ADR 0014 lazy creation (row on first message) → Task 6 (pending-task → POST on first send). ✓
- ADR 0014 has-work per task (session exists for task_id) → Task 1 (`sessionsByTaskId`) + Task 2 (dot). ✓
- ADR 0011 "no flat session list" → Task 8 (retire accordion), with "Other" stub so nothing is unreachable. ✓
- Per-task done from the rail → Task 7 (reuses `/api/plan/toggle` linear index, matching `handler/plan.go`). ✓
- Out-of-scope (Scratch buckets, archived "Before the redesign", Detached, reading auto-open) → explicitly deferred to 3b-2; "Other" stub is the interim. ✓

**2. Placeholder scan:** No TBD/TODO. Each code step shows complete code; verification steps state exact expected observations. Three steps contain a `grep`/`curl` to confirm a local name or seed data before acting (Local-run env vars, Task 6 Step 1 send-path, Task 8 dead-code refs) — these are deliberate "confirm against the real file" steps, not placeholders.

**3. Type/name consistency:** `selectedCourse`, `currentPlan`, `sessionsByTaskId`, `otherSessions`, `pendingTask` are defined in Task 1 and used consistently in 2/4/6/7. `loadRailData`/`loadRail`/`renderRail`/`selectCourse`/`openTask`/`toggleTask`/`getPendingTask`/`clearPendingTask`/`getSelectedCourse` exported from `rail.js` and imported where used (`app.js`, `chat.js`). `walkTasks` linear index matches `handler/plan.go` `toggleTaskAt` order (phases' tasks, then each cluster's tasks). `clearWorkspace`/`setActiveSessionId`/`loadSessionMessages`/`updateSessionPill` are sessions.js exports (Task 4 Step 2 notes `updateSessionPill` may need an `export` added). The `noop` action (course `<select>`) is handled in `app.js` (Task 2 Step 4).

**Risk note for execution:** the rail reuses `#session-list`/`#session-sidebar` as its mount, so existing CSS for those IDs may need pruning if it conflicts; adjust in Task 2 Step 3 if the browser shows stale styling. Exact `chat.js` send-path wording (Task 6) and `loadConfig` env names (Local run) must be confirmed against the real files at execution time — both steps say so.
