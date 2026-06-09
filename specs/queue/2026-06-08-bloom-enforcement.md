---
id: 2026-06-08-bloom-enforcement
title: Bloom-level enforcement — structural gate at phase boundaries (R8)
max_wall_clock_minutes: 30
max_diff_lines: 120
max_retries: 1
max_tokens: 30000
requires_visual_approval: false
allow_web_search: false
---

## Goal

Add a `bloom_level` field to Task and enforce that a phase cannot be completed
unless it covers every Bloom level from Analyze through Create. Currently Rule 5
in the sandbox is purely advisory ("Progress through Bloom's levels: explain →
apply → analyze → evaluate → create. Do not skip levels") — Pi may or may not
follow it, and the learner can mark any phase done regardless. This spec makes
it a structural gate: toggling the last task in a phase DONE triggers a
coverage check, and if the phase lacks Analyze, Evaluate, or Create, the toggle
is refused with a message naming the missing levels.

This is R8 from the pedagogy backlog. It does NOT add any session-level
interactions — it fires only at toggle time (same lifecycle as the existing
mastery gate), not during study.

## Bloom's levels

| Level | Tag value | What the learner does |
|---|---|---|
| Remember | `remember` | Recall facts, definitions, terms |
| Understand | `understand` | Explain in own words, summarize |
| Apply | `apply` | Use the concept on a new problem |
| Analyze | `analyze` | Compare, contrast, find weaknesses |
| Evaluate | `evaluate` | Judge, critique, justify |
| Create | `create` | Synthesize, design, extend |

Enforcement set: **analyze, evaluate, create** — the top three. `remember`,
`understand`, and `apply` are assumed to be covered by the reading itself and
are not enforced. (A phase of pure reading with zero Apply tasks would be
caught by this, but the sandbox already guides the agent toward including Apply;
the gate focuses on the levels most commonly skipped.)

## Implementation plan

### Step 1 — Add `bloom_level` to Task struct (`agent/types.go`)

Add the field:

```go
type Task struct {
    ID         string `json:"id"`
    Title      string `json:"title"`
    Done       bool   `json:"done"`
    Priority   string `json:"priority,omitempty"`
    Notes      string `json:"notes,omitempty"`
    BloomLevel string `json:"bloom_level,omitempty"`
}
```

- Optional (`omitempty`) — existing plans with no `bloom_level` are unaffected.
- Valid values: `""` (unspecified), `"remember"`, `"understand"`, `"apply"`, `"analyze"`, `"evaluate"`, `"create"`.
- The field is set by the agent when building/rewriting plans (Step 4).

### Step 2 — Add phase-completion Bloom check (`agent/tools_plan.go`)

In `applyToggle`, after the mastery gate check and before `applyAction`, add a
**phase-completion Bloom check** that fires only when toggling the LAST undone
task in a phase DONE:

```go
// phaseCompletionBloomCheck returns a refusal message if completing
// this task would finish the phase without covering all required Bloom
// levels. Returns "" if the check passes or is skipped.
func phaseCompletionBloomCheck(tasks []Task, bloomOk func(t Task) bool) string {
    // Collect bloom levels from all tasks in the phase.
    levels := map[string]bool{}
    allTagged := true
    for _, t := range tasks {
        if t.BloomLevel == "" {
            allTagged = false
        } else {
            levels[t.BloomLevel] = true
        }
    }
    // Skip enforcement if ANY task lacks a bloom_level (backward compat).
    if !allTagged {
        return ""
    }
    required := []string{"analyze", "evaluate", "create"}
    var missing []string
    for _, r := range required {
        if !levels[r] {
            missing = append(missing, r)
        }
    }
    if len(missing) > 0 {
        return fmt.Sprintf("Phase %q cannot be completed: missing Bloom levels — %s. Add a task at each missing level then try again, or pass --force to override.",
            phaseTitle, strings.Join(missing, ", "))
    }
    return ""
}
```

The function needs the phase's tasks (flat list — combine `phase.Tasks` + all
cluster tasks) and the phase title. It is called inside `applyToggle` when:
1. The action is `"done"` (not `"undo"`)
2. The task being toggled is the LAST undone task in its phase
3. `force` is `false`

The check collects bloom levels from ALL tasks in the phase (both done and
undone — the gate looks at the phase's task *design*, not just completed tasks).

**Natural integration point**: inside `applyToggle`, right after the
mastery-gate check and before `applyAction`. The function already has access to
the full plan. Add a helper `isLastUndoneInPhase` that counts undone tasks in
the phase — if `count == 1` and this task is undone, it's the last one.

### Step 3 — Write tests (`agent/tools_plan_test.go`)

Add test cases:

**`TestBloomEnforcementRefusesIncompletePhase`**: create a plan with a phase
containing tasks tagged `understand`, `apply`, `analyze` (missing `evaluate`
and `create`). Toggle the last undone task done. Expect refusal message
containing "cannot be completed" and "evaluate, create".

**`TestBloomEnforcementAllowsCompletePhase`**: create a plan with a phase
containing tasks tagged `understand`, `apply`, `analyze`, `evaluate`, `create`.
Toggle the last task done. Expect success (no refusal).

**`TestBloomEnforcementSkippedWhenUntagged`**: create a plan with a phase where
one task has `bloom_level` set and another does not. Toggle the last task done.
Expect success (enforcement skipped for backward compat).

**`TestBloomEnforcementForceBypasses`**: same incomplete phase, toggle with
`force=true`. Expect success.

**`TestBloomEnforcementUndoAlwaysAllowed`**: toggling a task UNDO never triggers
the Bloom check (only DONE toggles do).

### Step 4 — Update sandbox template (`agent/sandbox.go`)

In the plan-building guidance (the `planSection` string in `writeAgentsMD`),
after the paragraph about keeping task `id`s stable, add:

```
When writing the plan JSON, set `bloom_level` on every task to one of:
remember, understand, apply, analyze, evaluate, create. Each phase MUST
include at least one task at each of analyze, evaluate, and create.
Tasks that are primarily reading/comprehension are `understand`; tasks
that ask the learner to compare/critique are `analyze`; tasks that ask
them to judge or justify are `evaluate`; tasks that ask them to design
or synthesize are `create`. The app enforces this at phase completion —
a phase missing analyze, evaluate, or create will refuse to complete.
```

### Step 5 — Build + vet + test

No new imports beyond `strings` (already imported in `tools_plan.go`). No
frontend changes, no DB changes.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
# 1. Task struct has no bloom_level field.
grep -q 'BloomLevel' agent/types.go && echo "PRE-FAIL: BloomLevel already exists" || true
# Invert for pre-baseline: BloomLevel should NOT exist on main.
! grep -q 'BloomLevel' agent/types.go

# 2. No phaseCompletionBloomCheck function exists.
! grep -q 'phaseCompletionBloomCheck\|bloom.*enforcement\|Bloom.*enforcement' agent/tools_plan.go

# 3. No Bloom guidance in sandbox template.
! grep -q 'bloom_level.*remember.*understand.*apply.*analyze.*evaluate.*create\|analyze.*evaluate.*create.*MUST' agent/sandbox.go
```

### Post-acceptance (must PASS after implementation)

```bash
# 1. BloomLevel field exists on Task struct.
grep -q 'BloomLevel.*string.*json.*bloom_level' agent/types.go && echo "PASS: BloomLevel field"

# 2. phaseCompletionBloomCheck function exists.
grep -q 'phaseCompletionBloomCheck\|bloomCompletionCheck' agent/tools_plan.go && echo "PASS: check function"

# 3. Bloom guidance in sandbox template.
grep -q 'bloom_level.*remember\|analyze.*evaluate.*create.*MUST' agent/sandbox.go && echo "PASS: sandbox guidance"

# 4. Build + vet + all tests pass.
go build ./... && echo "PASS: build"
go vet ./... && echo "PASS: vet"
go test ./... -count=1 && echo "PASS: tests"

# 5. Specific Bloom enforcement tests exist and pass.
go test ./agent/ -run 'Bloom' -count=1 -v 2>&1 | grep -E 'PASS|FAIL'
```

### Human-eyeball notes

- The enforcement fires only on DONE toggles, not UNDO.
- The `--force` flag bypasses the check (same pattern as mastery gate).
- The enforcement skips entirely when any task in the phase lacks a `bloom_level` (backward compat for existing plans).
- The refusal message names the specific missing levels so the learner knows exactly what to add.

## Done criteria

- [ ] `BloomLevel` field added to `Task` struct (optional, `omitempty`).
- [ ] `phaseCompletionBloomCheck` function in `agent/tools_plan.go`.
- [ ] Check fires on DONE toggle of last undone task in a phase (skip on UNDO, skip on force).
- [ ] Enforcement skipped when any task lacks `bloom_level` (backward compat).
- [ ] Required set: analyze, evaluate, create.
- [ ] Sandbox template updated with Bloom-level guidance for plan building.
- [ ] Unit tests for: refusal on incomplete phase, success on complete phase, skip on untagged, force bypass, undo always allowed.
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass.
- [ ] Pre-baseline fails on current main; post-acceptance passes on the branch.

## Rollback notes

Pure additive — a new optional field that nothing depends on. Revert the commit.
Existing plans with no `bloom_level` are unaffected (enforcement skips them).
If a plan already has `bloom_level` populated and a phase is stuck refusing to
complete, use `--force` or add a task at the missing level.
