# JS style — marginalia

This file carries JS-specific concrete examples for the rules in `STYLE.md`. The project uses
vanilla JS with no framework and no bundler — ES modules loaded directly by the browser. Every
rule from `STYLE.md` has at least one good and one bad snippet here, drawn from the live code in
`static/` wherever possible. Read `STYLE.md` first for the philosophy and locked decisions; this
file is the practitioner's reference.

---

## Module structure

JavaScript is split by concern. The target layout under `static/` is:

```
static/
  js/
    api.js        — apiFetch + shared fetch helpers
    sessions.js   — session list, active session, CRUD
    chat.js       — message stream, input handling
    plan.js       — study plan panel, topic toggles
    pdf.js        — PDF viewer integration
    main.js       — document-level wiring (click delegator + boot sequence)
```

`index.html` loads one entry point:

```html
<!-- Good — single ES module entry; all other imports are static imports inside main.js -->
<script type="module" src="/static/js/main.js"></script>

<!-- Bad — multiple script tags with implicit load order -->
<script src="/static/sessions.js"></script>
<script src="/static/chat.js"></script>
<script src="/static/main.js"></script>
```

The actual file split is a separate refactor task. This section describes the target shape.

Every module file opens with the same header rule as Go: one paragraph stating the file's
responsibility, its key invariant, and any non-obvious dependency.

**Good file header:**

```js
// sessions.js — loads and renders the session list, tracks the active
// session ID, and exposes CRUD actions called by the main click delegator.
// Invariant: activeSessionId is always in sync with the rendered .active row.
// Depends on: apiFetch (api.js), showErrorBanner (errorBanner.js).
```

**Bad file header** — restates the filename, adds nothing:

```js
// This file handles sessions.
```

---

## Stateful units (closure factories)

Stateful units are closure factories, not classes. The factory owns its private state as closed-over
variables; callers get a plain object of functions with no `this` involved.

**Good:**

```js
export function createChatStream({ sessionId, onToken }) {
  let abortController = null;

  async function start(prompt) {
    abortController = new AbortController();
    const resp = await apiFetch(`/api/sessions/${sessionId}/stream`, {
      method: 'POST',
      signal: abortController.signal,
      body: JSON.stringify({ prompt }),
    });
    /* ...read SSE tokens, call onToken for each... */
  }

  function stop() {
    abortController?.abort();
  }

  return { start, stop };
}
```

**Bad** — class with `this`-binding traps:

```js
export class ChatStream {
  constructor({ sessionId, onToken }) {
    this.sessionId = sessionId;
    this.onToken = onToken;
    this.abortController = null;
  }
  async start(prompt) {
    this.abortController = new AbortController();
    /* ... */
  }
  stop() { this.abortController?.abort(); }
}

// Caller trouble:
const stream = new ChatStream({ sessionId, onToken });
button.addEventListener('click', stream.start); // `this` is undefined inside start
// Forces: button.addEventListener('click', stream.start.bind(stream))
// or:     button.addEventListener('click', (e) => stream.start(e))
// Both are accidental complexity from the class choice, not the problem.
```

The `this`-binding problem: when `stream.start` is passed as a callback, the receiver is lost.
Every call site must manually rebind or wrap. Closure factories have no `this` — `abortController`
is a closed-over variable; `start` closes over it directly, independent of how it is called.

---

## Module-scope singletons

Module-scope `let` is acceptable for genuine singletons — values where there is exactly one
instance per page lifetime and all writes flow through exported setters.

**Good** — single source of truth for the active session:

```js
// sessions.js
let activeSessionId = null;

export function setActiveSession(id) {
  activeSessionId = id;
}

export function getActiveSessionId() {
  return activeSessionId;
}
```

**Bad** — the variable leaks into multiple modules as a de-facto global:

```js
// utils.js
export let activeSessionId = null; // exported mutable state

// chat.js
import { activeSessionId } from './utils.js';
activeSessionId = newId; // direct write from a different module — readers can't find all writers
```

Module-scope state is a legitimate tool for singletons. The rule is: all writes go through named
setters; reads go through a named getter. Scattered direct writes to exported `let` values are the
banned pattern.

---

## Async

`async/await` always. `.then()` chains are banned.

**Good:**

```js
const resp = await apiFetch('/api/sessions');
const sessions = await resp.json();
renderSessionList(sessions);
```

**Bad** — chained `.then()` is a "smart one-liner" that hides two async steps:

```js
apiFetch('/api/sessions').then(r => r.json()).then(renderSessionList);
```

`apiFetch` integration (from `static/apiFetch.js`): pass all idempotent GETs through `apiFetch`
to get the built-in retry + exponential backoff. Use raw `fetch` for POSTs and other non-idempotent
methods — `apiFetch` passes non-GET calls through with a single attempt, but callers that use
`fetch` directly for writes are also fine and make the intent explicit.

```js
// Good — GET through apiFetch (retried on 5xx)
const resp = await apiFetch(`/api/sessions/${sessionId}/messages`);

// Good — POST via raw fetch (write; no retry wanted)
const resp = await fetch('/api/sessions', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ title }),
});
```

---

## Variables

`const` by default. `let` only when the binding is genuinely reassigned. `var` is banned — lint
enforces `no-var` and `prefer-const`.

**Good:**

```js
const trimmed = input.trim();
const lower = trimmed.toLowerCase();
```

**Bad** — `var` plus a chained transformation disguised as one operation:

```js
var normalized = input.trim().toLowerCase();
```

Reassignment is a smell. When you feel the urge to reassign, consider whether a new name carries
more meaning — the same logic as the Go "named intermediates" rule:

```js
// Bad — same name reused for a different value
let label = session.title;
label = label || 'Untitled';
label = label.slice(0, 40);

// Good — each step is a new concept
const rawLabel = session.title || 'Untitled';
const label = rawLabel.slice(0, 40);
```

---

## Naming

Hybrid: descriptive for domain, short for plumbing.

**DOM element references** carry the `El` suffix so the variable's role is obvious at the call
site:

```js
// Good
const sessionListEl = document.getElementById('session-list');
const chatInputEl   = document.getElementById('chat-input');
const errorBannerEl = document.getElementById('error-banner');

// Bad — undifferentiated, could be anything
const el = document.getElementById('session-list');
```

**Event handlers** use the `on`-event format:

```js
// Good
function onSessionClick(e) { /* ... */ }
function onChatSubmit(e)    { /* ... */ }

// Bad — opaque
function handler(e) { /* ... */ }
function cb(e)      { /* ... */ }
```

**Short names are fine for true plumbing.** `e` as the event argument in a one-line handler is
acceptable; it is not acceptable for a handler with a real body:

```js
// Good — e is fine when the handler is one line
button.addEventListener('click', (e) => e.preventDefault());

// Bad — e in a multi-line handler; the type is not obvious
document.addEventListener('click', function (e) {
  const sessionId = parseInt(e.target.dataset.sessionId, 10);
  loadSessionMessages(sessionId);
});

// Good — name the event where the body is non-trivial
document.addEventListener('click', function (clickEvent) {
  const sessionId = parseInt(clickEvent.target.dataset.sessionId, 10);
  loadSessionMessages(sessionId);
});
```

---

## Errors

Try/catch only where you can do something useful with the failure. Surface failures to the user
via the existing `showErrorBanner` (from `static/errorBanner.js`). `console.log` in committed
code is banned; `console.error` and `console.warn` are allowed.

**Good** — catch, surface to user, log with context:

```js
try {
  const resp = await fetch('/api/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title }),
  });
  const session = await resp.json();
  setActiveSession(session.id);
} catch (err) {
  showErrorBanner('Failed to create session. Try again.');
  console.error('createSession failed:', err);
}
```

**Bad** — no recovery, no user feedback, debug log left in:

```js
const resp = await fetch('/api/sessions', { method: 'POST', body: ... });
const session = await resp.json();
console.log('created session', session); // debug log, not allowed in committed code
```

The error banner pattern is already installed globally by `installErrorBanner()` in
`static/errorBanner.js`, which catches `window.error` and `unhandledrejection`. Explicit
`try/catch` blocks are for cases where you want a specific message or recovery path, not blanket
suppression.

---

## DOM patterns

Event delegation via `data-action` is the established pattern. A single document-level dispatcher
in `main.js` routes all clicks; feature modules export the handler functions.

**Good** — HTML uses data attributes:

```html
<button data-action="delete-session" data-session-id="42">Delete</button>
<button data-action="switch-session" data-session-id="42">Session name</button>
```

**Good** — single dispatcher in `main.js` (from `static/app.js`):

```js
document.addEventListener('click', function (e) {
  const el = e.target.closest('[data-action]');
  if (!el) return;
  const action = el.dataset.action;
  switch (action) {
    case 'delete-session':  deleteSession(parseInt(el.dataset.sessionId, 10)); break;
    case 'switch-session':  switchSession(parseInt(el.dataset.sessionId, 10)); break;
  }
});
```

**Bad** — inline `onclick` breaks CSP and scatters event logic:

```html
<!-- Bad — inline handler, global function required, CSP-hostile -->
<button onclick="deleteSession(42)">Delete</button>
```

No `onclick`, `onsubmit`, or any other inline event attribute. All events go through the central
dispatcher or through explicit `addEventListener` calls in module init functions.

---

## Patterns embraced

| Pattern | Rationale | Rule |
|---|---|---|
| Closure factories (`createChatStream`) | Explicit state; no `this` binding; lifetime is obvious | #9, #11 |
| Destructuring at function boundaries | Flat, named parameters; no positional argument guessing | #11 |
| Template literals for string interpolation | Readable; no concatenation noise | #11 |
| Optional chaining `?.` | Safe property traversal without nested guards | #11 |
| `apiFetch` for all idempotent GETs | Built-in retry + backoff; single policy point | #11 |
| ES modules `import`/`export` | Explicit dependency graph; no globals | #8, #11 |
| `data-action` event delegation | Central dispatch; CSP-friendly; no per-element listeners | #11 |
| Named handler functions (`onSessionClick`) | Stack traces are readable; handlers are testable | #11 |

---

## Patterns banned

| Pattern | Rationale | Rule |
|---|---|---|
| Classes | `this`-binding is a perennial source of bugs; closure factories are simpler | #9, #11 |
| Prototypal inheritance | Modifies global behavior; breaks in subtle ways across call sites | #9, #11 |
| `var` | Hoisting + function scope lead to subtle bugs; `const`/`let` cover all real uses | #9, #11 |
| `this`-binding tricks / `.bind()` workarounds | Symptom of choosing a class; fix the root cause | #9, #11 |
| `.then()` chains | Hard to read; hard to debug; violates one-operation-per-line | #9, #11 |
| `console.log` in committed code | Debug artifact; clutters production logs | #10, #11 |
| Inline `onclick` / `onsubmit` | CSP-hostile; scatters event logic; no central dispatch | #11 |
| Anonymous functions where named would help | Stack traces become useless; handlers can't be referenced | #11 |
| Deep destructuring as cleverness | Obscures data shape; violates one-operation-per-line | #11 |
