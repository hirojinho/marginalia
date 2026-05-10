# Code Style — claw-study

This is the source of truth for how code is written in this repo. The locked decisions below come
from a 2026-05-10 grill-me session and are not up for re-litigation without an explicit follow-up
session.

> **For agents:** read this file before writing any code. Lint enforces the mechanically-checkable
> subset (see `.golangci.yml`, `.eslintrc.cjs`); the prose rules below carry the rest.
> Pre-commit blocks on lint failures — do not bypass without permission.

---

## Philosophy

Code is read more than written. The rules below privilege the reader.

Two non-negotiable principles:

1. **One operation per line.** No chained calls. Named intermediates carry meaning and are
   self-documenting.
2. **The smallest unit names a concept.** Methods name subject-verb-object sentences. Files have a
   one-paragraph responsibility statement. If a function needs an "and" in its name, it needs to be
   split.

---

## The 11 rules

### Rule 1 — Scope

The style guide covers Go and JavaScript. The root `STYLE.md` carries the philosophy and the 11
locked rules. Language-specific examples and details live in `docs/style/go.md` and
`docs/style/js.md`. `CLAUDE.md` at the repo root points agents at this file.

### Rule 2 — Philosophy: idiomatic Go + hard type safety

Write idiomatic Go. Enforce hard type safety. One operation per line; no chained calls. Use
typed identifiers for domain primitives (`type CourseID string`). Avoid `any` and `interface{}`
outside protocol boundaries (JSON unmarshaling, SQL `Scan` targets).

```go
// Compliant
type CourseID string
func Save(course CourseID, content string) error { ... }

// Non-compliant
func Save(courseID string, content string) error { ... }
```

See `docs/style/go.md` for the full set of Go examples.

### Rule 3 — Naming (hybrid)

Domain values use descriptive names (`store`, `memory`, `recentSessions`, `courseID`). Plumbing
uses short, conventional names (`db`, `ctx`, `r` for `*http.Request`, `t *testing.T`). Receivers
are 1–3 characters. Loop indices use `i` only in tight numeric loops; otherwise use a domain-
meaningful name.

```go
// Compliant
for i, memory := range memories { ... }

// Non-compliant
for memoryIndex := 0; memoryIndex < len(memories); memoryIndex++ { ... }
```

### Rule 4 — Comments (layered)

Every non-trivial file opens with a header paragraph stating its responsibility, its key invariant,
and any non-obvious dependency. Exported types and functions carry standard Go doc comments.
Inline comments answer "why", never "what" — the code states what it does.

```go
// Compliant inline comment
// SQLite does not enforce FK constraints by default; PRAGMA foreign_keys is set on every conn.
db.Exec("PRAGMA foreign_keys = ON")

// Non-compliant inline comment
// Set foreign keys pragma
db.Exec("PRAGMA foreign_keys = ON")
```

### Rule 5 — Errors

Wrap every fallible operation with `fmt.Errorf("verb noun: %w", err)`. One log site per request
(HTTP handler or CLI `main`); intermediate layers never both log and return. Sentinel errors only
when callers branch on them (YAGNI otherwise).

```go
// Compliant
if err != nil {
    return fmt.Errorf("save memory: %w", err)
}

// Non-compliant
if err != nil {
    return err
}
```

### Rule 6 — Decomposition

Use the smallest unit that names a concept. Create a struct only when two or more methods share
non-trivial state. Use a method when the receiver is a meaningful subject (`store.Save(memory)`).
Use a free function when there is no natural subject (`AssembleAgentsMD(...)`). Split a file when
its header sentence needs an "and". Split a function when its body has more than ~3 distinct phases.

### Rule 7 — Tests

Tests are behavior-first and use real dependencies (`:memory:` SQLite, `t.TempDir()`). Test names
are English sentences (`TestMemoryStoreSavesAndRetrieves`). Table tests only when all cases share
the same assertions. Helper constructors go at the top of the test file (`newMemoryDB(t)`).
Use stdlib `testing` only — no assert libraries. No `t.Run` unless subtest isolation or
parallelism is genuinely needed.

```go
// Compliant test name
func TestMemoryStoreSavesAndRetrieves(t *testing.T) { ... }

// Non-compliant test name
func TestSave(t *testing.T) { ... }
```

### Rule 8 — JS structure

JavaScript uses ES modules (`<script type="module">`). Files are split by concern (`api.js`,
`sessions.js`, `chat.js`, `plan.js`, `pdf.js`). One responsibility per file.

See `docs/style/js.md` for the full module structure and file-split target.

### Rule 9 — JS objects

Stateful units are closure factories, not classes. Module-scope `let` is acceptable for genuine
singletons. Async code always uses `async/await`, never `.then()` chains. Variables are `const`
by default; use `let` only when reassignment is genuinely needed. `var` is banned.

```js
// Compliant
export function createChatStream({ sessionId, onToken }) {
  let abortController = null;
  async function start(prompt) { /* ... */ }
  function stop() { abortController?.abort(); }
  return { start, stop };
}

// Non-compliant
class ChatStream {
  constructor({ sessionId, onToken }) { ... }
  async start(prompt) { ... }
  stop() { ... }
}
```

### Rule 10 — Enforcement

Hard lint blocking pre-commit. Every rule that can be lint-enforced will be. Conceptual rules
without lint coverage (file-header invariants, verb-noun error wraps, behavior-test naming) live
in prose only and are enforced by reviewer attention.

### Rule 11 — Embrace / ban list

See the **Embrace / ban** section below.

---

## Embrace / ban

### Embrace

| Pattern | Rationale | Rule |
|---|---|---|
| Typed identifiers (`type CourseID string`) | Makes domain primitives distinct; prevents string aliasing bugs | #2 |
| Behavior-named small interfaces (`Saver`, `Loader`) | Names the capability; enables real test substitution without mocks | #6 |
| Constructor functions (`NewMemoryStore`) | Explicit initialization path; no zero-value traps | #6 |
| Pure free functions for transformations | No hidden state; composable; easy to test | #6 |
| Total functions (return `error` instead of panicking) | Callers can handle failures; no surprise crashes | #5 |
| Named intermediates over chains | One operation per line; named step carries meaning | #2 |
| File-header docstrings (responsibility + invariant + dep) | Every file is self-documenting; new readers orient quickly | #4 |
| Closure factories in JS (`createChatStream`) | Explicit state; no `this` binding; easy to reason about lifetime | #9 |

### Ban

| Pattern | Rationale | Rule |
|---|---|---|
| `any` / `interface{}` outside protocol boundaries | Erases type information; hides bugs at compile time | #2 |
| `panic` outside `main` + unrecoverable init | Uncatchable in library code; crashes the whole program | #5 |
| `init()` functions | Invisible execution order; hard to test; surprising side effects | #6 |
| Package-level mutable state in Go | Concurrency hazards; test pollution between runs | #6 |
| `var` in JS | Hoisting + function scope lead to subtle bugs; `const`/`let` cover all real uses | #9 |
| Classes in JS | `this` binding is a perennial source of bugs; closure factories are simpler | #9 |
| Prototype mutation | Modifies global object behavior; breaks library consumers | #9 |
| Mocks for things we own (DB, FS) | Mocks drift; use `:memory:` SQLite and `t.TempDir()` instead | #7 |
| Comments that restate code | Noise; double-maintenance cost; code self-documents | #4 |
| "Smart" one-liners with chaining | Hard to read; hard to debug; violates one-operation-per-line | #2 |
| Bare `return err` without wrap | Loses call-site context; makes error traces unnavigable | #5 |
| `console.log` in committed JS | Debug artifact; clutters production logs | #10 |

---

## Where rules live

| Concern | File |
|---|---|
| Philosophy + 11 rules | `STYLE.md` (this file) |
| Go specifics + examples | `docs/style/go.md` |
| JS specifics + examples | `docs/style/js.md` |
| Go lint config | `.golangci.yml` |
| JS lint config | `.eslintrc.cjs` |
| Format config (JS) | `.prettierrc.json` |
| Pre-commit hook | `scripts/git-hooks/pre-commit` |

---

## Enforcement

- `gofmt -s` and `prettier --check` run pre-commit. Format failures block the commit.
- `golangci-lint run` and `npx eslint` run pre-commit. Lint failures block the commit.
- Conceptual rules (file-header invariants, verb-noun error wraps, behavior-test naming,
  decomposition judgment) are not lint-enforceable — reviewer attention catches drift.

Do not bypass the pre-commit hook (`--no-verify`) without explicit permission from the project
owner.

---

## Updating this file

Style changes require a fresh grill-me session. Single-rule edits land via PR with a rationale
comment. Sweeping changes need a sibling document explaining the new philosophy and what is
superseded.
