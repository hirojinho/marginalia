# Go style — claw-study

This file carries Go-specific concrete examples for the rules in `STYLE.md`. Every rule from
`STYLE.md` has at least one good and one bad snippet here, drawn from the live repo wherever
possible. Read `STYLE.md` first for the philosophy and locked decisions; this file is the
practitioner's reference.

---

## Naming

Domain values use descriptive names. Plumbing stays conventional. Receivers are 1–3 characters.

**Bad** — single-letter receiver for domain struct, single-letter argument:

```go
func (s *MemoryStore) Save(m Memory) (Memory, error) { ... }
```

**Good** — the existing code is already compliant here; the receiver `s` is acceptable (1 char,
conventional for a store). But if you find yourself calling it at the use site, prefer the full
domain word:

```go
// Bad — both sides look like abbreviations
s.Save(m)

// Good — readable at the call site
store.Save(memory)
```

**Bad** — C-style index loop:

```go
for memoryIndex := 0; memoryIndex < len(memories); memoryIndex++ {
    process(memories[memoryIndex])
}
```

**Good** — range with domain name for the value, `i` only when the index is used:

```go
for i, memory := range memories {
    process(i, memory)
}
```

**Receiver convention** — 1–3 chars, abbreviation of the type name:

```go
func (s *MemoryStore) Save(...) { ... }   // good
func (a *App) CreateSession(...) { ... }  // good
func (h *Handler) ServeHTTP(...) { ... }  // good
```

**Plumbing names** — stay conventional throughout the codebase:

```go
db  *sql.DB        // not database, not dbConn
ctx context.Context
r   *http.Request
t   *testing.T
```

---

## Type safety

Use typed identifiers for domain primitives. Do not use bare `string` or `int` when a domain
concept is distinguishable.

**Bad** — two `string` parameters; the compiler cannot catch transposed arguments:

```go
func Save(courseID string, userID string, body string) error { ... }
```

**Good** — each domain primitive has its own type:

```go
type CourseID string
type UserID  string

func Save(course CourseID, user UserID, body string) error { ... }
```

**When NOT to type-wrap** — boundary-layer and ephemeral values stay raw. SQL `Scan` targets must
match what the driver returns; wrapping them causes a runtime type mismatch:

```go
// Good — Scan targets stay raw; the typed value is constructed after
var rawCourse string
row.Scan(&rawCourse)
course := CourseID(rawCourse)
```

`title string` inside a struct used only as a local accumulator is also fine — ephemeral values
that never cross an API boundary do not need a named type.

---

## One operation per line

No chained calls. Name the intermediate — the name is documentation.

**Bad** — two transformations hidden in one `return`:

```go
return strings.ToLower(strings.TrimSpace(input))
```

**Good** — each step is visible and named:

```go
trimmed := strings.TrimSpace(input)
return strings.ToLower(trimmed)
```

**Exception** — a chain that reads as a single concept is fine. `fmt.Errorf` with `%w` is one
operation conceptually; splitting it adds noise, not clarity:

```go
// Fine — one conceptual unit
return Memory{}, fmt.Errorf("memory save: %w", err)

// Overkill — splitting this adds nothing
wrapped := fmt.Errorf("memory save: %w", err)
return Memory{}, wrapped
```

The rule targets nested transformation chains, not every method call in a statement.

---

## Errors

Wrap every fallible operation with `fmt.Errorf("verb noun: %w", err)`. One log site per request
path. Inner layers return; only the outermost handler (HTTP handler or CLI `main`) logs.

**Good** — live in `agent/memory.go`:

```go
if err != nil {
    return Memory{}, fmt.Errorf("memory save: %w", err)
}
```

```go
if err != nil {
    return Memory{}, fmt.Errorf("memory save: last id: %w", err)
}
```

**Bad** — bare return loses the call site:

```go
if err != nil {
    return Memory{}, err
}
```

**Bad** — wrapping with the function name instead of a verb-noun phrase:

```go
if err != nil {
    return Memory{}, fmt.Errorf("Save: %w", err)
}
```

**One log site per request** — `MemoryStore.Save` only returns the error; it never logs. The CLI
`main` (or HTTP handler) is the single log site:

```go
// claw-cli/main.go — the outermost layer logs and exits
if err != nil {
    fmt.Fprintf(stderr, "save: %v\n", err)
    return 1
}
```

```go
// agent/handler — HTTP handler logs with slog, then responds
if err != nil {
    slog.Error("memory save failed", "err", err)
    http.Error(w, "internal error", http.StatusInternalServerError)
    return
}
```

Inner functions that both log and return an error are banned — the caller gets the error twice.

---

## Decomposition

Use the smallest unit that names a concept. Earn structs; default to free functions.

**`MemoryStore` earns its struct** — three methods share `*sql.DB`, and Phase 2 will add more.
The state is meaningful; the methods form a coherent subject:

```go
type MemoryStore struct{ db *sql.DB }

func (s *MemoryStore) Save(memory Memory) (Memory, error)       { ... }
func (s *MemoryStore) Search(userID, query, courseID string, limit int) ([]Memory, error) { ... }
func (s *MemoryStore) LoadByScope(userID, courseID string) (Scope, error) { ... }
```

**`AssembleAgentsMD` is a free function** — no natural subject, pure transformation, no shared
state:

```go
func AssembleAgentsMD(scope Scope, recent []SessionDigest, skills []SkillMeta, courseID string) string
```

**Behavior interface** — narrow the capability at the call site. A caller that only writes memory should not depend on the full store:

```go
// MemorySaver narrows MemoryStore to the Save capability for callers that
// only need to write. Tests can supply a fake without dragging in the full store.
type MemorySaver interface {
    Save(Memory) (Memory, error)
}
```

Declare behavior interfaces in the package that consumes them, not the package that implements them. The concrete `MemoryStore` satisfies `MemorySaver` automatically; no registration required.

**Bad pattern** — a struct with one method is just a free function wearing a disguise:

```go
type Truncator struct{}
func (t *Truncator) Truncate(s string, n int) string { ... }

// Good — just be a free function
func truncate(s string, n int) string { ... }
```

**File-split rule** — if the responsibility statement needs an "and", split the file. Reference:
`agent/memory.go` is ~390 lines covering three concerns (Memory CRUD, skill parsing, document
assembly). It is cohesive today because all three serve the same pipeline. Watch the threshold;
if a fourth concern lands, split.

**Function-split rule** — if a function body has more than ~3 distinct phases, each phase becomes
its own function.

---

## Comments

Every non-trivial file opens with a header paragraph: responsibility, key invariant, non-obvious
dependency. Exported symbols carry standard Go doc comments. Inline comments answer "why", never
"what".

**Good file header** — from `claw-cli/main.go`:

```go
// claw-cli is the agent's command-line surface into claw-study state.
// It is invoked by Pi via the bash tool. All subcommands write JSON
// (or markdown for `memory load`) to stdout. Errors go to stderr with
// non-zero exit codes.
package main
```

**Good file header** — from `seed-memory/main.go`:

```go
// seed-memory imports Eduardo's existing memory store at
// ~/.claude/projects/<project-slug>/
// into the agent_memory SQLite table. Idempotent: deletes all rows for
// the user before reseeding.
package main
```

**Good file header** — memory subsystem: store + AGENTS.md assembly + helpers (`handler/handler.go`):

```go
// Package handler holds the HTTP layer for the study app.
//
// All handlers hang off Handler, which carries the App (database +
// config) and an LLM client. Construct one with New, then call
// Register to wire the routes onto an http.ServeMux.
package handler
```

**Good inline comment** — explains a non-obvious constraint (`agent/db.go`):

```go
// Modernc sqlite is single-threaded per conn; let the driver pool
// handle concurrency, but keep the upper bound modest.
db.SetMaxOpenConns(8)
```

**Good exported doc comment** — from `agent/db.go`:

```go
// OpenDB opens the SQLite database at path and applies pragmas
// required for safe concurrent operation (WAL mode, busy timeout,
// foreign keys, balanced sync). Returns the *sql.DB ready to use.
func OpenDB(path string) (*sql.DB, error) { ... }
```

**Bad** — comment restates the code; adds noise, forces double maintenance:

```go
// increment counter
counter++

// Set foreign keys pragma
db.Exec("PRAGMA foreign_keys = ON")
```

The rule for `revive`: exported types and functions must have a doc comment. Non-exported code
comments are discretionary but must answer "why" if present.

---

## Tests

Behavior-first. Real dependencies (`:memory:` SQLite, `t.TempDir()`). English sentence names.
Stdlib `testing` only.

**Good test name** — names subject, verb, and condition:

```go
func TestMemoryStoreSearchCourseFilter(t *testing.T) { ... }
func TestMemoryStoreSaveAssignsID(t *testing.T) { ... }
func TestAssembleAgentsMDIncludesAllSections(t *testing.T) { ... }
```

**Bad test name** — opaque or indexed:

```go
func TestSearch1(t *testing.T) { ... }
func TestSearchEdgeCase(t *testing.T) { ... }
```

**Helper constructor** at top of test file — real DB, cleanup registered:

```go
func newMemoryDB(t *testing.T) *MemoryStore {
    t.Helper()
    db, err := OpenDB(":memory:")
    if err != nil {
        t.Fatalf("open: %v", err)
    }
    t.Cleanup(func() { _ = db.Close() })
    if err := InitSchema(db); err != nil {
        t.Fatalf("init: %v", err)
    }
    return NewMemoryStore(db)
}
```

**Table test** — use only when every case shares the same assertion shape. The canonical form
from `seed-memory/main_test.go`:

```go
func TestDeriveCourseIDFromPath(t *testing.T) {
    cases := []struct{ in, root, want string }{
        {"/m/courses/ce297/safety.md", "/m", "ce297"},
        {"/m/feedback_dsa_names.md", "/m", ""},
    }
    for _, c := range cases {
        got := deriveCourseID(c.in, c.root)
        if got != c.want {
            t.Fatalf("deriveCourseID(%q, %q) = %q, want %q", c.in, c.root, got, c.want)
        }
    }
}
```

Do not use `t.Run` unless subtest isolation or parallelism is genuinely needed. No assert
libraries — `t.Fatalf` + `t.Errorf` cover everything.

---

## Patterns embraced

| Pattern | Rationale | Rule |
|---|---|---|
| Typed identifiers (`type CourseID string`) | Prevents string-aliasing bugs at compile time | #2, #11 |
| Behavior-named small interfaces (`Saver`, `Loader`) | Names the capability; enables real substitution without mocks | #11 |
| Constructor functions (`NewMemoryStore`) | Explicit init path; no zero-value traps | #11 |
| Pure free functions for transformations (`AssembleAgentsMD`, `truncate`) | No hidden state; composable; trivially testable | #6, #11 |
| Total functions (return `error`; no panic on user data) | Callers can handle failures; no surprise crashes | #11 |
| Named intermediates over chains | One operation per line; each step carries a name | #2, #11 |
| File-header docstrings (responsibility + invariant + dep) | Every file is self-documenting; new readers orient quickly | #4, #11 |

---

## Patterns banned

| Pattern | Rationale | Rule |
|---|---|---|
| `any` / `interface{}` outside protocol boundaries | Erases type info; hides bugs at compile time | #2, #11 |
| `panic` outside `main` and unrecoverable init | Uncatchable in library code; crashes the whole program | #11 |
| `init()` functions | Invisible execution order; hard to test; surprising side effects | #11 |
| Package-level mutable state | Concurrency hazards; test pollution between runs | #11 |
| Mocks for things we own (DB, FS) | Mocks drift from reality; use `:memory:` SQLite and `t.TempDir()` | #7, #11 |
| Comments that restate code | Noise; double-maintenance cost; code self-documents | #4, #11 |
| "Smart" one-liners with chaining | Hard to read, hard to debug; violates one-operation-per-line | #2, #11 |
| Bare `return err` without wrap | Loses call-site context; makes error traces unnavigable | #5, #11 |
