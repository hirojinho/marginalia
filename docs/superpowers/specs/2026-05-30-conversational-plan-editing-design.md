# Conversational Plan Editing — design

**Date:** 2026-05-30
**Status:** Approved (grill-with-docs complete; ADR 0017 + CONTEXT.md amended; ready for implementation plan)

## Problem

The live Pi tutor (`/chat-v2`, acts only through `claw-cli` + bash) cannot restructure a
study Plan. `claw-cli plan` exposes only `show | status | toggle`. In session 46
(critical-theory, 2026-05-30) the learner asked to split a monolithic 51-page reading
task into the semantic chunks they had just designed together; the tutor correctly
reported it could not — "the plan tool only supports show, status, and toggle."

The Go layer already has a full, tested write path — `ToolRewritePlan` (validates,
preserves task UUIDs via `inheritOrGenerateIDs`, `SavePlan`) — but it was never wired
to the CLI. Same shape of gap as the course-creation and pre-ADR-0016 settings cases.

## Goal

Let the tutor reshape the Plan in-flow (split / add / rename / reorder / remove tasks)
through a single deterministic write, preserving session anchors and JSON validity,
with the rail refreshing live.

## Decisions (locked via grill-with-docs)

- **Classification = Steering.** A learner-directed declarative restructure is Steering,
  persisted by a deterministic one-shot write, confirm-and-resume, no accretion — the
  ADR 0016 pattern carried up from settings knobs to plan structure. Recorded in
  **ADR 0017** (amends 0016); `CONTEXT.md` Steering/Authoring section amended. The
  generative design that may precede an edit (deriving a chunk map from a PDF) stays
  Studying/Authoring; only the *write* is the Steering one-shot.
- **Surface = one generic `rewrite`**, not bespoke split/add/rename primitives. Minimal
  surface that edits anything; conformant to the Pi philosophy of general tools. The
  learner's earlier lean toward split+add+rename was superseded by this.
- **Raw file editing rejected.** The agent does not hand-edit `data/plans/<id>.json`;
  the write goes through validating `ToolRewritePlan` (ADR 0016 invariant: structured
  source-of-truth is never narrated raw into a file).
- **Anchors preserved by construction.** `inheritOrGenerateIDs` honors any `id` the
  agent carries forward; AGENTS.md instructs keeping `id` on continuing tasks and
  omitting it for new ones. Orphaning is recoverable (Detached bucket); no hard guard
  in v1.

## Scope

In scope: one `claw-cli plan rewrite` subcommand; one AGENTS.md guidance edit; ADR 0017
+ CONTEXT.md (already written).

Out of scope: bespoke split/add/rename/delete/reorder subcommands; a frontend plan
editor; an orphan-refusal guard; changes to `show/status/toggle` or the legacy `/chat`
plan tools.

## Design

### 1. CLI command (`claw-cli/main.go`)

Add a `rewrite` case to `runPlan` (line 392) and update the usage string to
`plan <show|status|toggle|rewrite>`. New `planRewrite` function, modeled on
`planToggle` (line 512) / `planStatus`:

```
claw-cli plan rewrite --course <id> --plan-file <path.json> [--db <path>]
```

1. Parse flags: `--course` (required), `--plan-file` (required), `--db` (override).
   Missing either → stderr message, exit 2.
2. `os.ReadFile(*planFile)`; read error → stderr, exit 1.
3. `resolveDBPath` → `newAppFromEnv(resolvedDB, false)` → `defer app.Close()`.
4. Marshal `{"plan_id": *course, "plan_json": string(fileBytes)}` and call
   `app.ToolRewritePlan(argsJSON)`.
5. **Inspect the result string** (unlike `planToggle`, which always exits 0): if it
   starts with `"error"` → print to stderr, exit 1; otherwise print to stdout, exit 0.
   This lets the agent detect bad-JSON / id-mismatch failures by exit code.

`ToolRewritePlan` already validates: `plan_json` parses as `JSONPlan`, `plan_json.id`
== `plan_id`, `MkdirAll`s the plans dir (so a non-existent plan is created), respects
provided task `id`s and fills blanks via title-match-or-new.

### 2. Agent guidance (`agent/sandbox.go` `writeAgentsMD`)

Extend the existing `planSection` (line 158, the "Study plan — JSON is the only source
of truth" block). Today it ends at "...never edit a markdown plan file." Append an
"Editing the plan" paragraph:

- The plan is a **live document** — when Eduardo asks to restructure it (split a task,
  add/rename/reorder/remove tasks), edit it directly.
- Procedure: `claw-cli plan show --course <id>` (full JSON) → edit the JSON → write a
  temp file → `claw-cli plan rewrite --course <id> --plan-file <tmp>`.
- **Preserve anchors:** keep each task's existing `id` on tasks that continue existing
  work (a renamed/split-from task keeps its `id` so its Session stays attached); leave
  `id` empty only for genuinely new tasks (they get fresh UUIDs).
- **One-shot discipline (ADR 0016/0017):** make the change, confirm in one line, resume
  the study work. Don't let a Studying Session accrete into an open-ended plan-editing
  conversation, and don't restructure unasked.

### 3. Live refresh

No work needed. `handler/chat_v2.go:178-196` fingerprints the plan before the turn and
emits `plan_changed` after if it changed; a CLI rewrite changes the file → fingerprint
differs → rail refreshes, exactly as a toggle does.

## Testing

CLI tests in `claw-cli/main_test.go` (helpers `run(...)`, `newTempDB(t)`, `openApp`):

- **rewrite valid file** — write a small plan JSON (id == course) with one task bearing
  an explicit `id` and one with blank `id`; run rewrite; assert exit 0, then
  `openApp`/`LoadPlan` shows the explicit `id` preserved and the blank one filled with a
  non-empty UUID.
- **bad JSON** — `--plan-file` pointing at malformed JSON → exit 1, stderr mentions
  parse/error.
- **id mismatch** — plan JSON `id` != `--course` → exit 1.
- **missing flags** — no `--plan-file` → exit 2.
- **creates when absent** — rewrite for a course with no existing plan file → exit 0,
  plan now loads.

No new Go write logic: `ToolRewritePlan` / `inheritOrGenerateIDs` are already covered;
we wrap them.

## Deploy notes

- Touches `claw-cli` (new subcommand) AND `study-app` (`sandbox.go` AGENTS.md block) →
  **rebuild + deploy BOTH binaries.**
- Bare-SSH `claw-cli plan ...` needs `--db $VAULT_ROOT/data/study.db`
  AND a reachable `VAULT_ROOT` for plan files (`data/plans/`); the Pi sandbox inherits
  the service env so it resolves both without flags.
- Live smoke: on the VPS, `plan show` a test course → edit JSON (rename a task keeping
  its `id`, add one with blank `id`) → `plan rewrite --plan-file` → `plan show` again
  confirms rename + preserved id + new uuid; clean up.
- Concurrency: `git fetch` + re-check `origin/main` before merge/push/deploy.

## References

- ADR 0017 — `docs/adr/0017-agent-may-restructure-plan-via-deterministic-rewrite.md`.
- ADR 0016 — deterministic Steering write (amended by 0017).
- ADR 0011 — Plan is the navigation spine. ADR 0014 — task-anchored sessions (UUID anchors).
- Write path: `agent/tools_plan.go` `ToolRewritePlan` + `inheritOrGenerateIDs`.
- Pattern template: `planToggle` / `planStatus` at `claw-cli/main.go:512` / `:408`.
- AGENTS.md target: `agent/sandbox.go:158` `planSection`.
- Refresh: `handler/chat_v2.go:178-196`.
