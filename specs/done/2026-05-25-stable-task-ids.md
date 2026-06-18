---
id: 2026-05-25-stable-task-ids
title: Add stable UUIDs to plan tasks with auto-migration on first load
max_wall_clock_minutes: 45
max_diff_lines: 250
max_retries: 1
max_tokens: 150000
requires_visual_approval: false
allow_web_search: false
created_at: 2026-05-25
created_by: laptop-cc + eduardo
---

## Goal

Give every `agent.Task` a stable, immutable string `ID` field (UUIDv4). Plans currently identify tasks positionally (sequential index across phases and clusters), so a user reordering a plan changes which row a tool call targets. Stable IDs are the prerequisite for tracking anything *about* a task across sessions — the next ticket (R4 — `confidence_log`) needs a durable foreign key, and Bayesian Knowledge Tracing (the literature-canonical student model — see Corbett & Anderson 1995, Wikipedia "Bayesian knowledge tracing") explicitly requires one stable identity per Knowledge Component. In claw-study the plan-task is the KC; this ticket makes that identity persistent.

**Why ship this in isolation (not bundled with R4):** plan persistence is shared state — a bug in the migration corrupts user plan JSON files. Splitting it out gives the gate a focused surface: "every task has an id after migration" is a single assertion that can be verified deterministically.

## References

- `agent/types.go:40` — `Task` struct definition.
- `agent/types.go:115` — `LoadPlan` reads `data/plans/<id>.json`; this is the *only* call path that produces a `*JSONPlan` from disk. All migration logic must live here.
- `agent/types.go:128` — `SavePlan` writes the file back. Migration in `LoadPlan` calls `SavePlan` to persist generated IDs.
- `agent/tools_plan.go:90` — `Task{}` literal in `applyAddTask`. Must populate `ID` on construction.
- `agent/tools_plan_test.go:33,40` — `Task{}` literals in test fixtures (no ID set). Tests should still pass — `LoadPlan` migration will fill IDs in.
- `handler/plan_test.go:16,21,26` and `handler/plan_http_test.go:109,143` — more `Task{}` literals in tests; same migration coverage applies.
- `handler/plan.go:12-45` — `GET /api/plan?view=full&id=<plan_id>` returns full plan JSON; this is the verifier's read path.
- UUID generation: use `github.com/google/uuid` if already in `go.mod`; otherwise prefer a minimal hand-rolled v4 via `crypto/rand` (~10 lines) to avoid pulling a new dep. Check `go.mod` first.

## Implementation plan

1. **Add `ID string` field to `Task` struct** in `agent/types.go` (line 40), with json tag `"id"`. Place it as the first field so JSON output reads naturally:
   ```go
   type Task struct {
       ID       string `json:"id"`
       Title    string `json:"title"`
       Done     bool   `json:"done"`
       Priority string `json:"priority,omitempty"`
       Notes    string `json:"notes,omitempty"`
   }
   ```

2. **Add a UUID generator** in a new file `agent/uuid.go`. If `go.mod` already has `github.com/google/uuid`, use `uuid.NewString()`. Otherwise implement a minimal v4 using `crypto/rand`:
   ```go
   package agent

   import (
       "crypto/rand"
       "fmt"
   )

   func newTaskID() string {
       var b [16]byte
       _, _ = rand.Read(b[:])
       b[6] = (b[6] & 0x0f) | 0x40 // v4
       b[8] = (b[8] & 0x3f) | 0x80 // RFC 4122 variant
       return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
   }
   ```

3. **Migrate-on-load inside `LoadPlan`** (`agent/types.go:115`). After successful `json.Unmarshal`, walk the plan; if any task has an empty `ID`, assign one via `newTaskID()` and set a `dirty` flag. If `dirty` is true, call `a.SavePlan(&p)` before returning. The save failure should not crash the read — log it via `slog.Warn` and return the plan anyway (the in-memory version has IDs even if persistence failed; next save attempt may succeed). Add `log/slog` import if not already there.

   Walk order: `for _, phase := range plan.Phases { for tasks; for clusters { for tasks } }`. A small helper `assignMissingTaskIDs(p *JSONPlan) bool` keeps `LoadPlan` short.

4. **Populate `ID` in `applyAddTask`** (`agent/tools_plan.go:90`). The `Task{}` literal must set `ID: newTaskID()`.

5. **Do NOT modify test fixtures** in `agent/tools_plan_test.go`, `handler/plan_test.go`, `handler/plan_http_test.go`. Existing tests build `Task{}` without IDs and write the plan through `writePlan` (which bypasses `LoadPlan`'s migration). When those tests *load* the plan via `LoadPlan` (e.g. `agent/tools_plan_test.go:129`), the migration runs and IDs get filled in. Tests pass unchanged. The exception: any test that asserts the loaded plan equals the saved plan byte-for-byte will break — search for `reflect.DeepEqual(loaded, plan)` style assertions in the three test files above and adjust to ignore the `ID` field if found.

6. **Run `go test ./...` locally before declaring done.** All ~46 tests must pass.

7. **Do NOT change tool signatures.** `update_plan` still uses `task_index`. Tools consuming IDs come in the next ticket; this ticket is purely the substrate.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# Fetch list of plans
plans=$(curl -sf -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/api/plan")

# Find at least one plan that has tasks (HasPlan=true and Total>0)
plan_id=$(echo "$plans" | jq -r '[.[] | select(.hasPlan == true and .total > 0)] | .[0].id // empty')

if [ -z "$plan_id" ]; then
  echo "FAIL: no plan with tasks found on $STAGING_URL — cannot verify task-ID coverage"
  exit 2
fi

# Fetch the full plan
plan=$(curl -sf -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/api/plan?view=full&id=$plan_id")

# Count tasks total and tasks-with-id
total=$(echo "$plan" | jq '[.phases[].tasks[]?, .phases[].clusters[]?.tasks[]?] | length')
with_id=$(echo "$plan" | jq '[.phases[].tasks[]?, .phases[].clusters[]?.tasks[]?] | map(select(.id != null and .id != "")) | length')

echo "plan=$plan_id total_tasks=$total with_id=$with_id"

if [ "$total" -eq 0 ]; then
  echo "FAIL: plan $plan_id has zero tasks — pick a different plan or seed test data"
  exit 2
fi

if [ "$with_id" -eq "$total" ]; then
  echo "OK: all $total tasks have stable IDs"
  exit 0
else
  echo "FAIL: $((total - with_id)) tasks missing IDs"
  exit 1
fi
```

### Post-acceptance (must PASS after Pi's implementation)

**Same script as above.** Pi's gate-runner runs it twice: pre-baseline against current main (expects exit 1 — no `id` field exists), post-acceptance against the new binary on staging (expects exit 0 — every task has an id).

### Human-eyeball notes (NOT part of the gate)

- After deploy, manually `curl https://your-host.example/api/plan?view=full&id=ce297 | jq '.phases[0].tasks[0]'` and confirm the `id` looks like a UUIDv4 (`xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx`).
- On the VPS, inspect one persisted plan file: `ssh nanoclaw 'jq ".phases[0].tasks[0]" ~/stack/study-app/vault/data/plans/ce297.json'`. The `id` should match what the HTTP response returned (proves migration persisted, not just runtime-generated each load).

## Done criteria

- [ ] `agent.Task` struct has `ID string` json:"id" as first field
- [ ] `agent.newTaskID` (or import of `google/uuid`) generates RFC 4122 v4 UUIDs
- [ ] `LoadPlan` assigns IDs to tasks missing them and persists via `SavePlan`
- [ ] `applyAddTask` populates `ID` on the new task
- [ ] `go build .` succeeds for linux/amd64
- [ ] `go test ./...` all green
- [ ] Verifier passes against staging (post-acceptance exit 0)
- [ ] After deploy, prod `/api/plan?view=full&id=ce297` shows UUIDs on every task
- [ ] Persisted plan JSON on disk has IDs (migration is durable, not just in-memory)
- [ ] Diff stays under 250 lines

## Rollback notes

Plan JSON files **will be modified in place** on first read after deploy. `git revert` removes the code that *generates* IDs but **does not strip them from already-migrated files** — the `id` keys will linger as unused JSON fields. This is harmless (Go's `json.Unmarshal` ignores unknown fields by default with the current struct shape, and the field is purely additive). No data corruption, no schema change in SQLite.

If a rollback drill happens before any user has loaded a plan, files are untouched. If it happens after, the migrated files are still readable by the reverted code — the `id` field is just ignored. **One-way migration, but reversal-safe.**
