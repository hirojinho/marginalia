---
id: 2026-06-17-plan-interleaving
title: Plan-spine interleaving — insert revisit tasks into the plan at a cadence
max_wall_clock_minutes: 60
max_diff_lines: 360
max_retries: 1
max_tokens: 200000
requires_visual_approval: false
allow_web_search: false
model: deepseek-v4-pro
thinking: low
created_at: 2026-06-17
created_by: laptop-cc + eduardo
---

## Goal

Add **structural interleaving** to the study plan (ROADMAP R6). After every `cadence`
new-content tasks, insert a `revisit` task into the plan spine that points back at an
earlier task, so spaced retrieval is woven into the navigation the learner already
follows ("next task from the last checked one"). **Why:** interleaved practice aids
retention and category discrimination (Rohrer & Taylor 2007; Kornell & Bjork 2008),
and the existing interleaving is only at the *session opener* (Rule 15 + the
`interleaving` course setting). Eduardo's decision (2026-06-17): interleaving should
also exist **as plan structure**, because that's where the learner navigates — revisit
tasks are first-class navigable items, not a separate gate.

This ships the deterministic core: a pure transform that produces the interleaved plan,
exposed over HTTP (stateless transform **and** a persist-to-course mode) and over
`claw-cli` (the surface the tutor triggers it from). It does **not** auto-apply
interleaving inside plan authoring/rewrite — that wiring is a later, separate change.

## References

No web research needed. All required code context is quoted inline in the plan below.

## Implementation plan

Work in package `agent` (Go module `study-app`). Match existing style; the pre-commit
hook runs `gofmt -s` + `golangci-lint` (errcheck, gochecknoglobals, forbidigo — no
`any`/`interface{}` outside protocol boundaries). Keep the package flat; do **not** add
a service/repository layer (ADR 0002).

### Step 1 — Add a `Kind` field to `Task` (`agent/types.go`)

In the `Task` struct (currently lines 43–50), add one field after `BloomLevel`:

```go
type Task struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	Done       bool   `json:"done"`
	Priority   string `json:"priority,omitempty"`
	Notes      string `json:"notes,omitempty"`
	BloomLevel string `json:"bloom_level,omitempty"`
	Kind       string `json:"kind,omitempty"` // "" = normal new-content task; "revisit" = interleaved spaced-retrieval task
}
```

`omitempty` keeps existing plan JSON byte-identical for normal tasks (no migration).

### Step 2 — New file `agent/interleave.go` with the pure transform

Create `agent/interleave.go`:

```go
package agent

const revisitKind = "revisit"

// InterleaveRevisitTasks weaves spaced-retrieval "revisit" tasks into the plan's
// phase task lists. Counting only new-content tasks (Kind != revisitKind) across the
// whole plan, after every `cadence`-th new-content task it inserts one revisit task
// pointing back at the earliest task of the block just completed.
//
// Idempotent under a fixed cadence: if a revisit task already immediately follows the
// cadence boundary, no new one is inserted (re-running returns 0). Existing revisit
// tasks reset the running counter and are never counted as new content.
//
// Only Phase.Tasks are processed; tasks nested in Phase.Clusters are left untouched
// in v1. cadence < 1 is a no-op. Returns the number of revisit tasks inserted.
func InterleaveRevisitTasks(plan *JSONPlan, cadence int) int {
	if plan == nil || cadence < 1 {
		return 0
	}
	inserted := 0
	sinceLast := 0
	var newContent []Task // new-content tasks seen so far, in plan order

	for pi := range plan.Phases {
		tasks := plan.Phases[pi].Tasks
		rebuilt := make([]Task, 0, len(tasks)+1)
		for i := 0; i < len(tasks); i++ {
			t := tasks[i]
			if t.Kind == revisitKind {
				rebuilt = append(rebuilt, t)
				sinceLast = 0
				continue
			}
			rebuilt = append(rebuilt, t)
			newContent = append(newContent, t)
			sinceLast++
			if sinceLast == cadence {
				// Peek: if a revisit already follows in the original list, leave it
				// to the existing revisit (idempotency); otherwise insert one now.
				if i+1 < len(tasks) && tasks[i+1].Kind == revisitKind {
					// existing revisit will reset the counter next iteration
				} else {
					target := newContent[len(newContent)-cadence]
					rebuilt = append(rebuilt, Task{
						ID:    newTaskID(),
						Title: "Revisit: " + target.Title,
						Kind:  revisitKind,
						Notes: "Interleaved spaced retrieval of an earlier task.",
					})
					inserted++
					sinceLast = 0
				}
			}
		}
		plan.Phases[pi].Tasks = rebuilt
	}
	return inserted
}
```

`newTaskID()` already exists in `agent/uuid.go` (same package).

### Step 3 — App method that persists (`agent/interleave.go`, same file)

Add a method that loads a course's plan, interleaves it, and saves it back to the spine:

```go
// InterleavePlan loads a course's plan, inserts revisit tasks at the given cadence,
// persists the result, and returns the updated plan plus the count inserted.
func (a *App) InterleavePlan(courseID string, cadence int) (*JSONPlan, int, error) {
	plan := a.LoadPlan(courseID)
	if plan == nil {
		return nil, 0, fmt.Errorf("plan not found: %s", courseID)
	}
	n := InterleaveRevisitTasks(plan, cadence)
	if err := a.SavePlan(plan); err != nil {
		return nil, 0, err
	}
	return plan, n, nil
}
```

Add `"fmt"` to the file's imports.

### Step 4 — Unit tests `agent/interleave_test.go`

Cover, with a 6-new-content-task single-phase plan and `cadence = 3` (expected
`inserted == 2`; resulting `Phase.Tasks` length `8`; exactly `2` tasks with
`Kind == "revisit"`; the two revisit titles are `"Revisit: A"` and `"Revisit: D"`
when the new-content titles in order are A,B,C,D,E,F):

1. **Cadence insertion** — build the 6-task plan, call `InterleaveRevisitTasks(p, 3)`,
   assert returned count `== 2`, `len(p.Phases[0].Tasks) == 8`, revisit count `== 2`,
   and the first revisit title is `"Revisit: A"`, the second `"Revisit: D"`.
2. **Idempotency** — call it a second time on the already-interleaved plan with the same
   cadence; assert it returns `0` and `len(p.Phases[0].Tasks)` is still `8`.
3. **cadence < 1 no-op** — assert `InterleaveRevisitTasks(p, 0) == 0` and the plan is
   unchanged.
4. **Empty / nil plan** — `InterleaveRevisitTasks(nil, 3) == 0`; a plan with zero phases
   returns `0`.

**Edge cases to handle (bake into the code, not discovery):**
- Empty input: nil plan or zero phases → return 0, no panic.
- Boundary: a phase whose task count is an exact multiple of `cadence` inserts a revisit
  after its last task (the `i+1 < len(tasks)` peek is false at the end → insert).
- Idempotency: existing `revisitKind` tasks reset `sinceLast` and are not recounted, so
  re-running with the same cadence inserts 0.
- The running counter `sinceLast`/`newContent` are **plan-global** (they span phases) —
  do not reset them per phase.

### Step 5 — HTTP endpoint `handler/plan.go`

Add a handler. It supports two request shapes (JSON body, `POST` only):

- `{"plan": {<JSONPlan>}, "cadence": N}` → **stateless**: interleave the supplied plan,
  return it. No persistence. (This is what the verifier uses.)
- `{"course_id": "<id>", "cadence": N}` → **persist**: load that course's plan,
  interleave, save to the spine, return it.

If both `plan` and `course_id` are absent → 400. If `cadence` is absent or `< 1`,
default it to `4`. Match the existing helpers (`methodNotAllowed`, `writeError`,
`writeServerError`, `writeJSON`) already used in this file.

```go
func (h *Handler) handlePlanInterleave(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Plan     *agent.JSONPlan `json:"plan"`
		CourseID string          `json:"course_id"`
		Cadence  int             `json:"cadence"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cadence := body.Cadence
	if cadence < 1 {
		cadence = 4
	}
	switch {
	case body.Plan != nil:
		inserted := agent.InterleaveRevisitTasks(body.Plan, cadence)
		writeJSON(w, http.StatusOK, map[string]any{"inserted": inserted, "plan": body.Plan})
	case body.CourseID != "":
		plan, inserted, err := h.App.InterleavePlan(body.CourseID, cadence)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"inserted": inserted, "plan": plan})
	default:
		writeError(w, http.StatusBadRequest, "one of plan or course_id is required")
	}
}
```

Add `"encoding/json"` to `handler/plan.go` imports (the file currently imports
`log/slog`, `net/http`, `strconv`, `time`, `study-app/agent`). `map[string]any` in
`writeJSON` calls is an allowed protocol boundary (`.golangci.yml` excludes
`handler/.*\.go` from `forbidigo`).

### Step 6 — Register the route (`handler/handler.go`)

Immediately after the `/api/plan/toggle` registration (currently line 53):

```go
	mux.HandleFunc("/api/plan/interleave", h.handlePlanInterleave)
```

### Step 7 — HTTP handler test (`handler/plan_http_test.go`)

Add a test that POSTs `{"plan": <6-task single-phase plan>, "cadence": 3}` to
`/api/plan/interleave` through the handler and asserts the JSON response has
`inserted == 2` and the returned `plan` contains exactly `2` tasks with
`kind == "revisit"`. Re-use whatever auth/test harness the existing tests in this file
use (the endpoint is behind `AuthMiddleware` in prod; the unit test calls the handler
method directly like the other tests here).

### Step 8 — claw-cli subcommand (`claw-cli/main.go`)

In `runPlan` (line 393), update the usage string and add a case:

```go
	_, _ = fmt.Fprintln(stderr, "usage: claw-cli plan <show|status|active|toggle|rewrite|interleave> [args]")
	...
	case "interleave":
		return planInterleave(args[1:], stdout, stderr, dbPath)
```

Add `planInterleave`, modeled on `planToggle` (lines 583–616):

```go
func planInterleave(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("plan interleave", flag.ContinueOnError)
	fs.SetOutput(stderr)
	course := fs.String("course", "", "course id / plan id (required)")
	cadence := fs.Int("cadence", 4, "insert a revisit task after every N new-content tasks")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *course == "" {
		_, _ = fmt.Fprintln(stderr, "plan interleave: --course is required")
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
	_, inserted, err := app.InterleavePlan(*course, *cadence)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "interleaved plan %q: inserted %d revisit task(s) at cadence %d\n", *course, inserted, *cadence)
	return 0
}
```

## Verification recipe

The verifier hits the HTTP surface (`POST /api/plan/interleave`) in **stateless inline
mode** — it sends a fixed 6-task plan in the request body and asserts the structure of
the returned plan. This needs no fixture in the DB / data dir and writes nothing to
prod (inline mode never persists). Requires `jq`.

### Pre-baseline (must FAIL on current main)

On current main the endpoint does not exist (404), so `curl -sf` fails and the script
exits non-zero — the desired "feature missing" state.

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

PLAN='{"plan":{"id":"ileave-fixture","name":"fix","phases":[{"title":"P1","tasks":[
{"id":"t1","title":"A","done":false},
{"id":"t2","title":"B","done":false},
{"id":"t3","title":"C","done":false},
{"id":"t4","title":"D","done":false},
{"id":"t5","title":"E","done":false},
{"id":"t6","title":"F","done":false}
]}]},"cadence":3}'

# First call: 6 new-content tasks, cadence 3 -> exactly 2 revisit tasks inserted.
resp=$(curl -sf -X POST \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$PLAN" \
  "$STAGING_URL/api/plan/interleave")

inserted=$(echo "$resp" | jq -r '.inserted')
[ "$inserted" = "2" ] || { echo "FAIL: expected inserted=2, got $inserted"; exit 1; }

revisits=$(echo "$resp" | jq '[.plan.phases[].tasks[] | select(.kind=="revisit")] | length')
[ "$revisits" = "2" ] || { echo "FAIL: expected 2 revisit tasks, got $revisits"; exit 1; }

total=$(echo "$resp" | jq '[.plan.phases[].tasks[]] | length')
[ "$total" = "8" ] || { echo "FAIL: expected 8 total tasks, got $total"; exit 1; }

# Idempotency: feed the returned (already-interleaved) plan back at the same cadence.
returned_plan=$(echo "$resp" | jq -c '.plan')
resp2=$(curl -sf -X POST \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"plan\":$returned_plan,\"cadence\":3}" \
  "$STAGING_URL/api/plan/interleave")
inserted2=$(echo "$resp2" | jq -r '.inserted')
[ "$inserted2" = "0" ] || { echo "FAIL: idempotency — expected inserted=0 on re-run, got $inserted2"; exit 1; }

echo "OK: interleaving inserts 2 revisit tasks at cadence 3 and is idempotent"
```

### Post-acceptance (must PASS after Pi's implementation)

**Same script as above.** One canonical verifier, two contexts: pre-baseline runs it
against current-main staging (fails because the endpoint 404s under `curl -sf`),
post-acceptance runs it against the new-binary staging (passes, exit 0).

### Human-eyeball notes (NOT part of the gate)

- After deploy, optionally `curl` the prod endpoint with the same inline `PLAN` body and
  eyeball that the two inserted tasks are titled `"Revisit: A"` and `"Revisit: D"` — the
  earliest task of each completed 3-task block.
- The `course_id` (persisting) mode and the `claw-cli plan interleave` subcommand are
  covered by `go test ./...`, not by this HTTP verifier. Spot-check the CLI once on a
  scratch course if you want belt-and-braces.
- This ships the mechanism only; nothing auto-applies interleaving yet. Deciding when the
  tutor invokes it (and at what cadence per course) is intentional follow-up.

## Done criteria

- [ ] `agent/types.go` `Task` has a `Kind string \`json:"kind,omitempty"\`` field
- [ ] `agent/interleave.go` defines `InterleaveRevisitTasks` + `(*App).InterleavePlan`
- [ ] `agent/interleave_test.go` covers cadence insertion, idempotency, cadence<1, nil/empty
- [ ] `POST /api/plan/interleave` handles inline-plan and course_id modes; registered in `handler/handler.go`
- [ ] `handler/plan_http_test.go` asserts inline mode returns `inserted=2`, 2 revisit tasks
- [ ] `claw-cli plan interleave --course --cadence` works; usage string updated
- [ ] `go build ./...` + `go test ./...` green; pre-commit lint clean
- [ ] Diff under 360 lines
- [ ] Pre-baseline verifier FAILS on current main; post-acceptance PASSES on staging

## Rollback notes

No data migration. The `Kind` field is additive with `omitempty`, so plans written
before this change parse unchanged and plans written after (containing revisit tasks)
still parse on an older binary — `Kind` is simply dropped. Binary swap + `git revert`
fully undoes it. If revisit tasks were persisted into a real course plan via the
`course_id` mode and need removing after a revert, delete the `"kind":"revisit"` tasks
from that course's `data/plans/<id>.json` by hand (or re-run a plan rewrite).
