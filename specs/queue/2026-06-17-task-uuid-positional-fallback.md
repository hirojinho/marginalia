---
id: 2026-06-17-task-uuid-positional-fallback
title: Preserve task UUIDs across rewrites via positional/type fallback
max_wall_clock_minutes: 60
max_diff_lines: 320
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

Reduce Session orphaning when the tutor rewrites a plan and **rephrases a task in
place**. Today `ToolRewritePlan` → `inheritOrGenerateIDs` matches old→new tasks only by
**normalized title** (`agent/tools_plan.go`). A reworded title finds no match, so a new
UUID is generated and the Session anchored to the old `sessions.task_id` is orphaned
(falls into the *Detached* bucket). **Why:** rephrasing a task while keeping its slot is
the common rewrite; losing the Session's anchor on every such edit is the main avoidable
orphaning cause (ROADMAP "Harden task-UUID preservation"). Real plan titles carry **no
stable slug** (format is `<emoji> **<Type>** — <prose>`, e.g.
`🔴 **Read** — [L] Ch. 2 "Questioning the Foundations…"`), so slug-matching is impossible;
the structural lever that *does* survive a rephrase is **position + task type**.

This adds a **positional/type fallback**: when exact-title match fails, inherit the old
task's UUID if a task of the **same type** sits at the **same position in the same phase**.
It deliberately does **not** try to handle reorder-and-rename together (the accepted
Phase-3 trade-off: orphans survivable + recoverable, just rarer). Exact-title match still
wins; no UUID is ever assigned to two tasks.

## References

No web research needed; all required code context is quoted inline below.

## Implementation plan

Work in package `agent` (module `study-app`) + one handler. Keep the package flat (no
service/repository layer — ADR 0002). `gofmt -s` + `golangci-lint` run in the pre-commit
hook; `map[string]any` is allowed in `handler/*.go` (forbidigo excludes it there).

### Step 1 — Task-type extraction + positional index (`agent/tools_plan.go`)

The current ID-inheritance helpers are `buildTitleToIDMap` (lines ~290–311),
`inheritOrGenerateIDs` (~313–334), `normalizeTitle` (~286–288), and `countIDOrigins`
(~336–355), all called from `ToolRewritePlan` (~246–284). Add:

```go
import "regexp" // add to the existing import block if not already present

// taskTypeRe captures the first **bold** token in a title — the task type
// (Read / Reflect / Watch / …). Real titles look like: "🔴 **Read** — …".
var taskTypeRe = regexp.MustCompile(`\*\*\s*([^*]+?)\s*\*\*`)

// taskType returns the lowercased first bold token, or "" if none.
func taskType(title string) string {
	m := taskTypeRe.FindStringSubmatch(title)
	if len(m) < 2 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(m[1]))
}

// buildPositionalIDMap indexes the OLD plan's direct phase tasks by
// "<phaseIdx>:<taskIdx>:<type>" -> id. Clusters are intentionally excluded
// (positional fallback applies to Phase.Tasks only in v1). Tasks with an empty
// type or empty id are skipped (nothing stable to match on).
func buildPositionalIDMap(p *JSONPlan) map[string]string {
	m := make(map[string]string)
	if p == nil {
		return m
	}
	for i := range p.Phases {
		for j, t := range p.Phases[i].Tasks {
			ty := taskType(t.Title)
			if t.ID == "" || ty == "" {
				continue
			}
			m[fmt.Sprintf("%d:%d:%s", i, j, ty)] = t.ID
		}
	}
	return m
}
```

`strings` and `fmt` are already imported in this file.

### Step 2 — Two-pass inheritance with used-ID tracking (`agent/tools_plan.go`)

Replace `inheritOrGenerateIDs` with a version that (1) takes the positional map, (2)
tracks which old IDs have been consumed so an ID is never assigned twice, and (3) applies
the positional fallback only to direct phase tasks:

```go
// inheritOrGenerateIDs assigns IDs to tasks in p that don't already have one.
// Order of precedence per task: explicit id (kept) > exact normalized-title match >
// positional+type match (direct phase tasks only) > newly generated. An old id is
// consumed at most once (exact matches claim theirs first, so a positional fallback
// never steals an id an exact match will use).
func inheritOrGenerateIDs(p *JSONPlan, titleToID, posToID map[string]string) {
	used := make(map[string]bool)

	// Pass 1 — exact normalized-title match (all tasks, incl. clusters).
	exact := func(t *Task) {
		if t.ID != "" {
			used[t.ID] = true
			return
		}
		if id, ok := titleToID[normalizeTitle(t.Title)]; ok && !used[id] {
			t.ID = id
			used[id] = true
		}
	}
	for i := range p.Phases {
		for j := range p.Phases[i].Tasks {
			exact(&p.Phases[i].Tasks[j])
		}
		for k := range p.Phases[i].Clusters {
			for j := range p.Phases[i].Clusters[k].Tasks {
				exact(&p.Phases[i].Clusters[k].Tasks[j])
			}
		}
	}

	// Pass 2 — positional+type fallback (direct phase tasks only).
	for i := range p.Phases {
		for j := range p.Phases[i].Tasks {
			t := &p.Phases[i].Tasks[j]
			if t.ID != "" {
				continue
			}
			ty := taskType(t.Title)
			if ty == "" {
				continue
			}
			if id, ok := posToID[fmt.Sprintf("%d:%d:%s", i, j, ty)]; ok && !used[id] {
				t.ID = id
				used[id] = true
			}
		}
	}

	// Pass 3 — generate for anything still unassigned.
	gen := func(t *Task) {
		if t.ID == "" {
			t.ID = newTaskID()
		}
	}
	for i := range p.Phases {
		for j := range p.Phases[i].Tasks {
			gen(&p.Phases[i].Tasks[j])
		}
		for k := range p.Phases[i].Clusters {
			for j := range p.Phases[i].Clusters[k].Tasks {
				gen(&p.Phases[i].Clusters[k].Tasks[j])
			}
		}
	}
}
```

**Edge cases to handle (in the code above — verify, don't rediscover):**
- New plan has more phases/tasks than old: `posToID` lookup misses → generate. No panic.
- Type mismatch at the same slot (e.g. old `Read`, new `Reflect`): no positional inherit → generate.
- Old id already consumed by an exact match: positional pass skips it (`!used[id]`) → generate.
- Empty type (`taskType=""`): skipped in both the index and the positional pass.
- Explicitly-provided ids in the new plan are kept and marked used (so nothing else can take them).

### Step 3 — Wire the positional map into `ToolRewritePlan` (`agent/tools_plan.go`)

In `ToolRewritePlan`, where it currently calls `buildTitleToIDMap` + `inheritOrGenerateIDs`
(~270–271), build the positional map from the old plan and pass it through:

```go
	oldPlan := a.LoadPlan(p.PlanID)
	titleToID := buildTitleToIDMap(oldPlan)
	posToID := buildPositionalIDMap(oldPlan)
	inheritOrGenerateIDs(&newPlan, titleToID, posToID)
```

`countIDOrigins` (the `preserved`/`generated` reporting) is unchanged — it compares final
ids against `titleToID`; positionally-inherited ids will be counted as `generated` in the
summary string, which is fine (the summary is informational only).

### Step 4 — Unit tests (`agent/tools_plan_test.go`)

Add tests (match the existing test style/helpers in this file):

1. **Positional inherit on rephrase-in-place** — old plan: phase[0] tasks
   `[{id:"keepme", title:"🔴 **Read** — Original"}]`; new plan: phase[0] tasks
   `[{title:"🔴 **Read** — Reworded"}]` (no id). After
   `inheritOrGenerateIDs(new, buildTitleToIDMap(old), buildPositionalIDMap(old))`,
   assert `new.Phases[0].Tasks[0].ID == "keepme"`.
2. **Exact match still wins + no double-assign** — old `[{id:"a","🔴 **Read** — X"},{id:"b","🔴 **Read** — Y"}]`;
   new `[{"🔴 **Read** — Y"},{"🔴 **Read** — Z"}]`. Assert task 0 → `"b"` (exact title Y),
   task 1 → a fresh id (NOT `"b"`; `"a"` is positionally at slot 0 not slot 1, and slot-1
   type-Read old id is `"b"` which is already used → generate). No id appears twice.
3. **Type mismatch → new id** — old `[{id:"a","🔴 **Read** — X"}]`; new
   `[{"🔴 **Reflect** — X"}]`. Assert the new task's id is freshly generated, not `"a"`.
4. **Explicit id preserved** — a new task carrying `id:"explicit"` keeps it.

### Step 5 — Gateable HTTP surface (`handler/plan.go` + `handler/handler.go`)

The deployed `claw-cli` binary is not rebuilt by the gate, so the bash verifier cannot use
`claw-cli plan rewrite`. Expose the same logic over HTTP (stateless; the real
`inheritOrGenerateIDs` runs). In `handler/plan.go` add (the file already imports
`net/http`, `study-app/agent`; add `encoding/json`):

```go
func (h *Handler) handlePlanReconcileIDs(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}
	var body struct {
		Old *agent.JSONPlan `json:"old"`
		New *agent.JSONPlan `json:"new"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.New == nil {
		writeError(w, http.StatusBadRequest, "both old and new plans are required")
		return
	}
	plan, inherited, generated := agent.ReconcilePlanIDs(body.Old, body.New)
	writeJSON(w, http.StatusOK, map[string]any{
		"plan": plan, "inherited": inherited, "generated": generated,
	})
}
```

Register it in `handler/handler.go` right after the `/api/plan/interleave` line:

```go
	mux.HandleFunc("/api/plan/reconcile-ids", h.handlePlanReconcileIDs)
```

### Step 6 — Exported wrapper (`agent/tools_plan.go`)

Add a small exported function so the handler does not duplicate logic and so the
`inherited`/`generated` counts are computed against the **pre-assignment** id set:

```go
// ReconcilePlanIDs assigns ids to newPlan inheriting from oldPlan (exact title, then
// positional+type), mutating newPlan in place. Returns newPlan plus counts of how many
// task ids were inherited from oldPlan vs newly generated. Used by the rewrite path and
// the /api/plan/reconcile-ids endpoint.
func ReconcilePlanIDs(oldPlan, newPlan *JSONPlan) (*JSONPlan, int, int) {
	if newPlan == nil {
		return newPlan, 0, 0
	}
	oldIDs := collectTaskIDs(oldPlan) // set of ids present in oldPlan
	inheritOrGenerateIDs(newPlan, buildTitleToIDMap(oldPlan), buildPositionalIDMap(oldPlan))
	inherited, generated := 0, 0
	for _, id := range collectTaskIDsSlice(newPlan) {
		if oldIDs[id] {
			inherited++
		} else {
			generated++
		}
	}
	return newPlan, inherited, generated
}
```

Add two tiny helpers (`collectTaskIDs(p) map[string]bool` over all tasks incl. clusters;
`collectTaskIDsSlice(p) []string` likewise) near `countIDOrigins`. Keep them simple
walkers mirroring the existing walk pattern. (`ToolRewritePlan` may optionally call
`ReconcilePlanIDs` instead of the inline calls in Step 3 — either is fine as long as the
positional map is wired in.)

## Verification recipe

The verifier POSTs known old+new plans to `POST /api/plan/reconcile-ids` and asserts the
returned ids. Hermetic (plans in the request body), hits the HTTP surface the gate
rebuilds, writes nothing to prod. Requires `jq`.

### Pre-baseline (must FAIL on current main)

On current main the endpoint does not exist (404) → `curl -sf` fails → non-zero exit.

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# Old: one Read task with id "keepme" at phase 0, slot 0.
# New: same slot, same type (Read), reworded prose, NO id -> must inherit "keepme".
BODY='{"old":{"id":"f","name":"f","phases":[{"title":"P1","tasks":[
{"id":"keepme","title":"🔴 **Read** — Original prose","done":false}]}]},
"new":{"id":"f","name":"f","phases":[{"title":"P1","tasks":[
{"title":"🔴 **Read** — Reworded prose","done":false}]}]}}'

resp=$(curl -sf -X POST \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$BODY" "$STAGING_URL/api/plan/reconcile-ids")

got=$(echo "$resp" | jq -r '.plan.phases[0].tasks[0].id')
[ "$got" = "keepme" ] || { echo "FAIL: rephrase-in-place did not inherit id (got '$got', want 'keepme')"; exit 1; }

inherited=$(echo "$resp" | jq -r '.inherited')
[ "$inherited" = "1" ] || { echo "FAIL: expected inherited=1, got $inherited"; exit 1; }

# Control: different type at the same slot must NOT inherit (gets a fresh id).
BODY2='{"old":{"id":"f","name":"f","phases":[{"title":"P1","tasks":[
{"id":"keepme","title":"🔴 **Read** — Original prose","done":false}]}]},
"new":{"id":"f","name":"f","phases":[{"title":"P1","tasks":[
{"title":"🔴 **Reflect** — Different type","done":false}]}]}}'
resp2=$(curl -sf -X POST -H "Authorization: Bearer $STAGING_TOKEN" \
  -H "Content-Type: application/json" -d "$BODY2" "$STAGING_URL/api/plan/reconcile-ids")
got2=$(echo "$resp2" | jq -r '.plan.phases[0].tasks[0].id')
[ "$got2" != "keepme" ] && [ -n "$got2" ] || { echo "FAIL: type-mismatch wrongly inherited (got '$got2')"; exit 1; }

echo "OK: positional/type fallback inherits on rephrase-in-place, not on type change"
```

### Post-acceptance (must PASS after Pi's implementation)

**Same script as above.** One canonical verifier, two contexts: pre-baseline runs it
against current-main staging (fails because the endpoint 404s under `curl -sf`),
post-acceptance runs it against the new-binary staging (passes, exit 0).

### Human-eyeball notes (NOT part of the gate)

- The fix targets *rephrase-in-place*; *reorder + rename together* still orphans (accepted
  trade-off). After deploy, optionally rewrite a real course's plan rewording one task and
  confirm its Session stays anchored (not in the Detached group).
- `countIDOrigins`' summary string still counts positionally-inherited ids as "generated"
  (cosmetic only); the authoritative inherited/generated counts come from `ReconcilePlanIDs`.
- The `claw-cli plan rewrite` path now benefits automatically (same `inheritOrGenerateIDs`);
  spot-check once on a scratch course if you want belt-and-braces.

## Done criteria

- [ ] `agent/tools_plan.go`: `taskType`, `buildPositionalIDMap`, two-pass
      `inheritOrGenerateIDs(.., posToID)`, `ReconcilePlanIDs` + id-collect helpers
- [ ] `ToolRewritePlan` builds + passes the positional map
- [ ] `agent/tools_plan_test.go`: positional inherit, exact-wins/no-double-assign, type-mismatch, explicit-id
- [ ] `POST /api/plan/reconcile-ids` added + registered in `handler/handler.go`
- [ ] `go build ./...` + `go test ./...` green; pre-commit lint clean
- [ ] Diff under 320 lines
- [ ] Pre-baseline verifier FAILS on current main; post-acceptance PASSES on staging

## Rollback notes

No data migration, no schema change. Pure logic + one additive endpoint. Plans written
after this change are byte-compatible with the old binary. Binary swap + `git revert`
fully undoes it. Already-rewritten plans keep whatever ids they were assigned (inheriting
*more* ids is strictly safer than the old behavior — it never orphans a task that the old
code would have preserved, because exact-title match runs first and unchanged).
