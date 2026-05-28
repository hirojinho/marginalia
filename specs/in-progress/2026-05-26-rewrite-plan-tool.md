---
id: 2026-05-26-rewrite-plan-tool
title: Add rewrite_plan agent tool preserving task UUIDs across wholesale plan rewrites
max_wall_clock_minutes: 45
max_diff_lines: 300
max_retries: 1
max_tokens: 150000
requires_visual_approval: false
allow_web_search: false
created_at: 2026-05-26
created_by: laptop-cc + eduardo
---

## Goal

The chat agent's only structured plan-edit tool is `update_plan` with verbs {`toggle`, `set_done`, `set_undone`, `add_task`}. None of those handle the agent's most common rewrite need: **wholesale restructuring** (rephasing, regrouping, deleting old tasks). When the agent created `data/plans/guitar.json` on 2026-05-25 it used the `save_note` escape hatch — writing a raw JSON file with no awareness of plan invariants.

This is a real load-bearing problem now that `2026-05-25-stable-task-ids` shipped UUIDs on every task: a `save_note`-style monthly rewrite **wipes the UUIDs**, breaking the foreign key from `confidence_log.kc_id` (R4, 2026-05-26-r4-confidence-log) that Bayesian Knowledge Tracing depends on. Stable per-KC identity is the core BKT assumption (Corbett & Anderson 1995); we'd be invalidating it monthly.

This ticket adds a single structured tool `rewrite_plan(plan_id, plan_json)` that:

1. Reads the **old** plan from disk
2. Builds a `title → uuid` map over all tasks in the old plan
3. Iterates the **new** plan; for each task, inherits the old UUID if the title matches (exact, case-insensitive after trim), generates a fresh UUIDv4 otherwise
4. Writes the new plan via `SavePlan` (which already exists)

The agent's prompt is updated to prefer this tool over `save_note` for plan rewrites. `save_note` stays available (deletion is out of scope), but Rule X (new) in the system prompt names the preference.

**Out of scope for this ticket:**
- Schema-level path allowlist on `save_note` (separate concern; tracked verbally, not queued)
- Fuzzy/semantic title matching across rewrites (exact-match is the cheap win)
- Plan version history / undo
- Phase/cluster-level structured verbs (`rename_phase`, `move_task_between_phases`, etc.) — wholesale rewrite covers these via JSON

## References

- `agent/types.go:22-45` — `JSONPlan`, `Phase`, `Cluster`, `Task` structs (Task has the `ID string \`json:"id"\`` field shipped 2026-05-25)
- `agent/types.go:115-135` — `LoadPlan` / `SavePlan` (the file-persistence layer; auto-migrates missing IDs on load)
- `agent/uuid.go` (shipped 2026-05-25) — `newTaskID()` generator
- `agent/tools.go:93-104` — `update_plan` tool registration pattern
- `agent/tools.go:182` — tool dispatch switch
- `agent/tools_plan.go` — file containing `ToolUpdatePlan`. Add `ToolRewritePlan` alongside it.
- `agent/sandbox.go` — system prompt with the agent's behavioral rules. Look for where `update_plan` is referenced and add a parallel mention of `rewrite_plan` next to it.

## Implementation plan

1. **Add the `rewrite_plan` tool registration** in `agent/tools.go` (after `update_plan`'s block at line ~104):
   ```go
   {Type: "function", Function: ToolFunc{
       Name:        "rewrite_plan",
       Description: "Replace the entire plan JSON for a course with new content (phases, clusters, tasks). Preserves task UUIDs across rewrites when titles match exactly — this is required so confidence/retrieval data stays anchored. Use this instead of save_note for plan rewrites. The plan_json field must be a valid JSON object matching the JSONPlan schema (id, name, phases[]).",
       Parameters: map[string]interface{}{
           "type": "object",
           "properties": map[string]interface{}{
               "plan_id":   map[string]interface{}{"type": "string", "description": "Plan ID, e.g. 'ce297', 'guitar'. Must already exist in the courses table."},
               "plan_json": map[string]interface{}{"type": "string", "description": "Full JSONPlan as a JSON string. Tasks without an id field will be assigned UUIDs (existing ones inherited via title match)."},
           },
           "required": []string{"plan_id", "plan_json"},
       },
   }},
   ```
   Add `case "rewrite_plan": return a.ToolRewritePlan(args)` to the dispatch switch.

2. **Implement `ToolRewritePlan`** in `agent/tools_plan.go` (append at end of file):
   ```go
   func (a *App) ToolRewritePlan(args json.RawMessage) string {
       var p struct {
           PlanID   string `json:"plan_id"`
           PlanJSON string `json:"plan_json"`
       }
       if err := json.Unmarshal(args, &p); err != nil { return "error: " + err.Error() }
       if p.PlanID == "" { return "error: plan_id is required" }
       if p.PlanJSON == "" { return "error: plan_json is required" }

       var newPlan JSONPlan
       if err := json.Unmarshal([]byte(p.PlanJSON), &newPlan); err != nil {
           return "error: plan_json failed to parse as JSONPlan: " + err.Error()
       }
       if newPlan.ID != p.PlanID {
           return fmt.Sprintf("error: plan_json.id (%q) does not match plan_id arg (%q)", newPlan.ID, p.PlanID)
       }

       oldPlan := a.LoadPlan(p.PlanID)  // may be nil if first time
       titleToID := buildTitleToIDMap(oldPlan)
       inheritOrGenerateIDs(&newPlan, titleToID)

       if err := a.SavePlan(&newPlan); err != nil {
           return "error saving plan: " + err.Error()
       }
       preserved, generated := countIDOrigins(&newPlan, titleToID)
       return fmt.Sprintf("rewrote plan %q: %d tasks, %d inherited UUIDs, %d new UUIDs",
           p.PlanID, preserved+generated, preserved, generated)
   }
   ```

3. **Helpers** (also in `agent/tools_plan.go`):
   ```go
   func normalizeTitle(t string) string {
       return strings.ToLower(strings.TrimSpace(t))
   }

   func buildTitleToIDMap(p *JSONPlan) map[string]string {
       m := make(map[string]string)
       if p == nil { return m }
       walk := func(t Task) {
           if t.ID != "" { m[normalizeTitle(t.Title)] = t.ID }
       }
       for _, ph := range p.Phases {
           for _, t := range ph.Tasks { walk(t) }
           for _, cl := range ph.Clusters {
               for _, t := range cl.Tasks { walk(t) }
           }
       }
       return m
   }

   func inheritOrGenerateIDs(p *JSONPlan, titleToID map[string]string) {
       walk := func(t *Task) {
           if t.ID != "" { return } // explicitly provided
           if id, ok := titleToID[normalizeTitle(t.Title)]; ok {
               t.ID = id
           } else {
               t.ID = newTaskID()
           }
       }
       for i := range p.Phases {
           for j := range p.Phases[i].Tasks { walk(&p.Phases[i].Tasks[j]) }
           for k := range p.Phases[i].Clusters {
               for j := range p.Phases[i].Clusters[k].Tasks { walk(&p.Phases[i].Clusters[k].Tasks[j]) }
           }
       }
   }

   func countIDOrigins(p *JSONPlan, titleToID map[string]string) (preserved, generated int) {
       count := func(t Task) {
           if titleToID[normalizeTitle(t.Title)] == t.ID { preserved++ } else { generated++ }
       }
       for _, ph := range p.Phases {
           for _, t := range ph.Tasks { count(t) }
           for _, cl := range ph.Clusters {
               for _, t := range cl.Tasks { count(t) }
           }
       }
       return
   }
   ```
   Add `"strings"` to imports if not already.

4. **Update the system prompt** in `agent/sandbox.go` — find the section where `update_plan` is referenced (grep for "update_plan" in sandbox.go) and add a sibling sentence:
   > For full plan rewrites (restructuring phases, regrouping tasks, monthly review), prefer `rewrite_plan` over `save_note`. `rewrite_plan` preserves task UUIDs across rewrites so confidence/retrieval data stays anchored — `save_note` would wipe them and break that data.

5. **Tests** in `agent/tools_plan_test.go` (append, using existing `samplePlan()` and `writePlan` helpers):
   - `TestRewritePlan_PreservesUUIDsForMatchingTitles` — write a plan with known UUIDs, rewrite with same titles (no id field in new), assert all UUIDs preserved
   - `TestRewritePlan_AssignsNewUUIDsForNewTitles` — rewrite includes 2 new task titles, assert their IDs are fresh UUIDv4 (not in old set)
   - `TestRewritePlan_DropsTasksMissingFromNew` — old plan has 4 tasks, new plan has 2, assert SavePlan persisted exactly 2 (and the 2 missing don't reappear)
   - `TestRewritePlan_RejectsMismatchedID` — plan_id arg "ce297", plan_json.id="guitar" → error
   - `TestRewritePlan_FirstTimeCreate` — plan_id "newcourse" doesn't exist on disk, rewrite creates it (all UUIDs generated)

6. **Run `go test ./...` locally before declaring done.**

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# The new tool isn't exposed via HTTP — its surface is the agent dispatch.
# Verify by checking GetTools() includes "rewrite_plan" through the
# /api/runtime endpoint which lists tool definitions. On current main
# this returns 6-ish tools without rewrite_plan; after impl it includes it.
tools_json=$(curl -sf -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/api/runtime")
has_rewrite=$(echo "$tools_json" | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
tools = data.get('tools', [])
names = [t.get('name') or t.get('function', {}).get('name') for t in tools]
print('yes' if 'rewrite_plan' in names else 'no')
" 2>/dev/null || echo "no")

if [ "$has_rewrite" = "yes" ]; then
  echo "OK: rewrite_plan tool registered"
  exit 0
else
  echo "FAIL: rewrite_plan not in /api/runtime tool list"
  exit 1
fi
```

**If `/api/runtime` doesn't expose tool names** (Pi will discover this during pre-baseline), Pi must add a minimal extension to it — the spec contract still requires the verifier to pass post-impl. Pi's pre-baseline failure will be: "FAIL: rewrite_plan not in /api/runtime" (correct — feature missing). Pi must then implement step 4 plus a small adjustment to `handler/runtime.go` so the verifier can find the tool. If `/api/runtime` already lists tools, no handler change is needed.

### Post-acceptance (must PASS after Pi's implementation)

**Same script as above.** Pre-baseline expects exit 1 (rewrite_plan absent); post-acceptance expects exit 0 (tool registered, visible in `/api/runtime`).

### Human-eyeball notes (NOT part of the gate)

- After deploy, ask the chat agent something like: "Rephase the guitar plan — combine the first two phases." It should call `rewrite_plan` not `save_note`. Check `data/plans/guitar.json` afterwards: every task that survived from the old plan should have the same `id` as before.
- Spot-check by `jq '.phases[].tasks[].id' data/plans/guitar.json` before and after — UUIDs for matched-title tasks should be identical strings.

## Done criteria

- [ ] `rewrite_plan` tool registered in `GetTools()` and dispatched in `ExecuteTool`
- [ ] `ToolRewritePlan` reads old plan, builds title→UUID map, assigns IDs to new plan, persists
- [ ] Title-matching is exact case-insensitive after trim (no fuzzy)
- [ ] Rejects `plan_id` ≠ `plan_json.id`
- [ ] First-time create works (old plan absent)
- [ ] System prompt updated to prefer this over `save_note` for plan rewrites
- [ ] All 5 unit tests pass
- [ ] `go test ./...` all green
- [ ] Verifier exits 0 against new binary (tool visible in `/api/runtime`)
- [ ] Diff stays under 300 lines

## Rollback notes

`git revert` removes the tool registration and the helpers. Plans rewritten via `rewrite_plan` between deploy and rollback keep their saved JSON on disk — the UUIDs Pi assigned remain valid (they're just regular UUIDs at that point, not gated by the tool). No data loss.
