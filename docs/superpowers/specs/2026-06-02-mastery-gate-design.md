# S2 — Hard Mastery Gate on Task Completion — Design

**Date:** 2026-06-02
**Status:** Approved, pending implementation plan
**Part of:** the 4-spec pedagogy-consolidation series (S1 confidence persistence ✅ shipped → **S2 hard mastery gate** → S3 legacy `/chat` removal → S4 two-step reveal). Depends on S1.

## Problem

DDIA session #56 marked task #19 "done" on a vague "I think I'm ok" — no
number, no qualifying evidence. Rule 3 (now S1-persisted) asks for confidence,
but nothing *enforces* it: the agent can flip a task to done regardless. The
soft prompt rule was ignored once already, so S2 makes completion a **hard
gate** the agent must clear or explicitly override.

## Goal

Refuse to mark a plan task **done** unless a `confidence_log` row at or above a
per-course `mastery_threshold` (default 0.7) exists for that task's `id` — with
an explicit `--force` escape hatch for "mark it done anyway."

## Design decisions (locked)

- **Hard gate at the CLI/tool layer**, not another prompt rule (prompt rules get
  ignored — that's the whole point).
- **`--force` override.** Pi passes it only when Eduardo explicitly says to
  complete anyway.
- **Threshold is a steerable course setting** `mastery_threshold` (default 0.7),
  following the existing `chunk_pages`/`interleaving` pattern.
- **The UI checkbox stays UNGATED.** A human clicking the sidebar checkbox
  (`/api/plan/toggle` → `toggleTaskAt`) is the human's own deliberate call; the
  gate targets *agent* auto-completion only. `handler/plan.go` is untouched.
- **Gate keys on the task's `id`** (the same kc id S1 logs confidence against).
  Tasks with an empty `id` are **ungateable → allowed** (can't look up evidence
  for a task that has no id; never block on missing-id, to avoid breaking older
  plans). This is logged behavior, not silent: the refusal path only triggers
  for tasks that *have* an id and lack qualifying confidence.

## Design

### 1. `mastery_threshold` course setting (`agent/course_settings.go`, `agent/db.go`)

- **Struct:** add `MasteryThreshold float64 \`json:"mastery_threshold"\`` to
  `CourseSettings` (after `Interleaving`).
- **Default:** `DefaultCourseSettings` returns `MasteryThreshold: 0.7`.
- **Schema:** add `mastery_threshold REAL NOT NULL DEFAULT 0.7` to the
  `course_settings` CREATE TABLE (`agent/db.go:178`), AND add an idempotent
  migration to the `InitSchema` migrations slice:
  `ALTER TABLE course_settings ADD COLUMN mastery_threshold REAL NOT NULL DEFAULT 0.7`.
- **Read:** `GetCourseSettings` SELECT + Scan extended to include
  `mastery_threshold`.
- **Write:** `UpsertCourseSettings` INSERT/UPDATE extended with the column.
- **Keyed set:** `SetCourseSetting` gets a `case "mastery_threshold"` using
  `strconv.ParseFloat`, and the unknown-key error message lists it.
- **Validate:** `ValidateCourseSettings` rejects `MasteryThreshold` outside
  `[0.0, 1.0]`.

### 2. Confidence-evidence helper (`agent/db.go`)

New small method, reusing the existing read path:

```go
// HasConfidenceAtLeast reports whether the most recent logged confidence for
// knowledgeComponentID is ≥ threshold. False if none logged.
func (a *App) HasConfidenceAtLeast(knowledgeComponentID string, threshold float64) (bool, error) {
	pts, err := a.GetConfidenceTrajectory(knowledgeComponentID, 1)
	if err != nil {
		return false, err
	}
	return len(pts) > 0 && pts[0].Value >= threshold, nil
}
```

(`GetConfidenceTrajectory` already orders `created_at DESC`, so `pts[0]` is the
latest value.)

### 3. The gate in the toggle path (`agent/tools_plan.go`)

The gate lives where the task is already located, so it covers both phase-level
and cluster-level tasks without duplicating traversal.

- Extend the `ToolUpdatePlan` arg struct with `Force bool \`json:"force"\``.
- Thread `force` into `applyToggle` / `applyToggleCluster`.
- Before `applyAction` flips a task **to done**, run the gate. The transition is
  "to done" when `action == "set_done"`, or `action == "toggle"` while the task
  is currently `!Done`. A shared helper:

```go
// masteryGateRefusal returns a non-empty refusal message if completing `task`
// must be blocked, or "" if allowed. plan.ID is the course id for settings.
func (a *App) masteryGateRefusal(planID string, task *Task, action string, force bool) string {
	if force {
		return ""
	}
	completing := action == "set_done" || (action == "toggle" && !task.Done)
	if !completing {
		return ""
	}
	if task.ID == "" {
		return "" // ungateable: no id to check evidence against
	}
	s, _ := a.GetCourseSettings(planID)
	ok, err := a.HasConfidenceAtLeast(task.ID, s.MasteryThreshold)
	if err != nil {
		return "" // never block on a read error
	}
	if !ok {
		return fmt.Sprintf(
			"refused: mastery gate — task %q has no logged confidence ≥ %.2f. Ask the learner to rate confidence and run `claw-cli confidence log`, or pass --force to override.",
			task.Title, s.MasteryThreshold)
	}
	return ""
}
```

When the helper returns non-empty, `applyToggle`/`applyToggleCluster` returns
that message **without** calling `applyAction` or `SavePlan` (the task stays
undone). `set_undone` and any non-completing toggle are never gated.

### 4. `--force` flag on `claw-cli plan toggle` (`claw-cli/main.go`)

- Add `force := fs.Bool("force", false, "bypass the mastery gate")`.
- Include `"force": *force` in the JSON marshaled to `ToolUpdatePlan`.
- No `--session` flag needed: the gate looks up confidence by task id across all
  sessions, not per-session.

### 5. Tell the agent the gate exists (`agent/sandbox.go`)

Two prompt touch-ups so a refusal isn't mysterious to Pi:

- **Steering key list** (`sandbox.go:210`): add `mastery_threshold` to
  `<framing|exam_style|chunk_pages|stop_after_task|interleaving>`.
- **A line in the study-plan / completion guidance**: note that
  `claw-cli plan toggle` may refuse with "mastery gate" if no confidence ≥
  threshold is logged for the task, and that the fix is to elicit + log
  confidence (Rule 3) first; pass `--force` **only** if Eduardo explicitly says
  to complete it anyway.

(Legacy mirrors `agent/agent.go` / `CLAUDE.local.md` remain untouched — S3
deletes them.)

## Components & boundaries

| Unit | Responsibility | Depends on |
|------|----------------|------------|
| `CourseSettings.MasteryThreshold` + settings CRUD | store/validate the threshold | `course_settings` table |
| `HasConfidenceAtLeast` | "is there qualifying evidence?" | `GetConfidenceTrajectory` |
| `masteryGateRefusal` + gated `applyToggle` | block completion w/o evidence | settings + helper |
| `plan toggle --force` (CLI) | human/agent override | `ToolUpdatePlan` |
| `writeAgentsMD` touch-ups | make the gate legible to Pi | (string only) |
| `handlePlanToggle` (UI) | **unchanged — ungated** | — |

## Testing

TDD throughout.

`agent/course_settings_test.go`:
- default `MasteryThreshold == 0.7`; round-trip via `SetCourseSetting("mastery_threshold","0.85")`; `ValidateCourseSettings` rejects `1.5` and `-0.1`.

`agent/tools_plan_test.go`:
- **gate blocks:** `set_done` on a task with id and no confidence → returns a "mastery gate" message, task stays `Done==false`, plan unchanged.
- **gate allows with evidence:** after `LogConfidence(sess, taskID, 0.8, "manual", "")`, `set_done` succeeds and `Done==true`.
- **`--force`/force=true bypasses:** `set_done` with `force:true` and no confidence → succeeds.
- **toggle to done is gated; toggle to undone is not:** toggling an undone task with no confidence is blocked; `set_undone` always succeeds.
- **empty-id task is allowed** (ungateable).

`claw-cli/main_test.go`:
- `plan toggle --force` on a no-confidence task → exit 0 and the task is done (end-to-end through `ToolUpdatePlan`).
- `course settings set --key mastery_threshold --value 0.6` round-trips.

`go test ./...` green.

Manual acceptance: deploy both binaries; on the VPS, take a real task with no
logged confidence, run `claw-cli plan toggle --course ddia --task <n>` and
confirm it's refused; log confidence ≥ 0.7 for that task id and confirm the
toggle then succeeds; confirm `--force` bypasses. Confirm the UI checkbox still
toggles freely.

## Deploy

Standard flow: cross-compile `study-app` **and** `claw-cli`, scp both (back up
`.bak`), `systemctl --user restart study-app.service`. The
`mastery_threshold` migration runs on startup (idempotent ALTER). `AGENTS.md`
regenerates next turn with the new steering key + gate note.
