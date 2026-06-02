# S1 — Confidence Persistence on the Pi Path — Design

**Date:** 2026-06-02
**Status:** Approved, pending implementation plan
**Part of:** a 4-spec series consolidating the pedagogy loop onto Pi `/chat-v2`
(S1 confidence persistence → S2 hard mastery gate → S3 legacy `/chat` removal →
S4 two-step reveal). This is S1.

## Problem

The R4 (confidence) and R1 (retrieval-queue) machinery is fully built and
correct, but **nothing writes to it**. On the live DB, `confidence_log`,
`retrieval_queue`, and `knowledge_components` are all empty.

Root cause is a wiring gap, not a missing feature:

- `App.LogConfidence(...)` (`agent/db.go:1110`) inserts into `confidence_log`
  **and** cascades into `retrieval_queue` via `UpsertRetrievalItem`
  (`agent/db.go:1075`). It works.
- It is reachable only as a **Go LLM tool** (`log_confidence`, registered in
  `agent/tools.go`, dispatched by `ExecuteTool`) — i.e. only on the legacy
  `/chat` path.
- The default and only path the user actually runs is **Pi `/chat-v2`**, which
  is **bash-only**: it can call `claw-cli` and skills, never Go tools.
- `claw-cli confidence` exposes only **read** subcommands
  (`trajectory|recent|schema`, `claw-cli/main.go:1165`). There is **no write
  subcommand.**
- Pedagogy Rule 3 (`agent/sandbox.go:243`) instructs the agent to *"call the
  log_confidence tool"* — a tool Pi physically cannot invoke.

So the agent asks "how confident are you?", the user answers, and the value
evaporates. Observed live in DDIA session #56 (2026-06-02): Rule 3 fired in
prose, no number was elicited, nothing persisted.

## Goal

Give the Pi agent a `claw-cli` write path to persist a confidence value, and
rewrite Rule 3 to use it. One subcommand fixes **both** R4 (confidence
trajectory) and R1 (retrieval queue), because the upsert already lives inside
`LogConfidence`.

## Non-goals

- **No mastery gate.** Refusing task completion below a threshold is S2.
- **No legacy `/chat` deletion.** That is S3. The legacy `log_confidence` Go
  tool and `ToolLogConfidence` keep working untouched in this spec.
- **No `knowledge_components` rows.** The `knowledge_component_id` is the plan
  task's `id` string (matching Rule 3's existing wording). `confidence_log`
  and `retrieval_queue` key on a free `TEXT` id with no FK to
  `knowledge_components`, so no component row is required for the loop to work.
  Richer KC entities remain a separate future concern.
- **No SM-2 scheduling changes.** The queued SM-2 spec
  (`specs/queue/2026-06-01-sm2-spaced-review.md`) layers on top of
  `LogConfidence` later; S1 only calls the existing method.

## Design

### 1. New `claw-cli confidence log` subcommand (CLI wiring only — no new DB code)

Reuses the existing `App.LogConfidence`. Added under the existing
`runConfidence` dispatch (`claw-cli/main.go:1170`) by adding `case "log"`.
Follows the exact pattern of `retrieveDue` / `confidenceTrajectory`:
`flag.NewFlagSet` → `resolveDBPath` → `newAppFromEnv(resolvedDB, false)` →
call the `*App` method → errors to stderr with non-zero exit.

**`claw-cli confidence log --session <id> --kc <taskId> --value <0.0–1.0> --raw "<verbatim reply>" [--db <path>]`**

- `--session` (int64, required, ≥1): the session id, for `confidence_log.session_id`.
- `--kc` (string, required): the active plan task's `id` field → `knowledge_component_id`.
- `--value` (float64, required): parsed confidence in `[0.0, 1.0]`.
- `--raw` (string, optional): the user's verbatim reply → `raw_text`.
- `--db` (string, optional): standard override.
- Calls `app.LogConfidence(session, kc, value, "tool_call", raw)`.
  `LogConfidence` already validates `value ∈ [0.0,1.0]` and `source` membership,
  and upserts `retrieval_queue` — no extra validation needed in the CLI beyond
  required-flag checks.
- On success: print `logged confidence <value> for <kc> (row <id>)` to stdout,
  exit 0. On error (out-of-range value, missing flags, DB error): message to
  stderr, exit 1 (or 2 for usage/flag errors, matching siblings).

`source` is hard-coded to `"tool_call"` (one of the four allowed values),
matching what the legacy `ToolLogConfidence` passes — keeps the two write paths
indistinguishable in the data.

### 2. Rewrite Rule 3 in `agent/sandbox.go`

Current (`agent/sandbox.go:243`):

> 3. **ALWAYS ask "How confident are you with this?"** before moving to a new
> topic. After the user replies, parse a value in [0.0, 1.0] from their answer
> and **call the log_confidence tool** with knowledge_component_id = the active
> task's id field from the plan, value = your parsed value, and raw = their
> verbatim reply. If no active task is in context, skip the tool call
> (prompt-only behavior). Low confidence → return to the previous topic; do not
> advance.

New (intent):

> 3. **ALWAYS ask "How confident are you with this?"** before moving to a new
> topic. **You must elicit an actual number** — if the reply is vague ("I think
> I'm ok"), ask again for a 0–1 value before advancing. Then persist it by
> running:
> ```
> claw-cli confidence log --session <SESSION_ID> --kc <active task id from the plan> --value <0.0–1.0> --raw "<their verbatim reply>"
> ```
> where the session id is the one in the Session section above and the task id
> is the `id` field of the active task from `claw-cli plan status`. If no active
> task is in context, skip the command (prompt-only). Low confidence → return to
> the previous topic; do not advance.

The session id is already baked literally into `AGENTS.md` (the Session section
written by `writeAgentsMD`), so the agent has it without an extra lookup. This
edit is in `studyTuningSections`, the Pi-facing prompt source.

### 3. Sync note (no edits this spec)

The same Rule-3 wording exists in two legacy-path mirrors —
`agent/agent.go` (`toolsAndRulesPrompt`) and `CLAUDE.local.md`. Those drive the
`/chat` path only and are slated for **deletion in S3**, so S1 deliberately
leaves them as-is rather than maintaining a path about to be removed. The
divergence is intentional and temporary; S3 closes it by deleting the mirrors.

## Components & boundaries

| Unit | Responsibility | Depends on |
|------|----------------|------------|
| `confidence log` (CLI) | parse flags, call writer, report | `App.LogConfidence` |
| `App.LogConfidence` | insert row + upsert queue (unchanged) | `confidence_log`, `retrieval_queue` |
| `writeAgentsMD` Rule 3 | instruct Pi to run the command | (string only) |

## Testing

TDD. Unit tests in `claw-cli/main_test.go` (mirroring existing subcommand
tests), using an in-test SQLite DB seeded via `agent.OpenDB`+`InitSchema`:

- `confidence log` happy path — valid flags → exit 0, **one** `confidence_log`
  row with the given session/kc/value/source=`tool_call`/raw, **and** one
  matching `retrieval_queue` row (proves the R1 cascade fires from the CLI).
- `confidence log` rejects out-of-range — `--value 1.5` → non-zero exit, no row
  written (surfaces the `LogConfidence` validation error).
- `confidence log` missing required flag — no `--kc` → exit 2, usage on stderr.

`go test ./...` and `go test ./claw-cli/...` must be green.

Manual acceptance: build + deploy both binaries to `nanoclaw`, run a fresh Pi
turn on a course session that asks the confidence question, answer with a
number, and confirm a real `confidence_log` row lands:

```
ssh nanoclaw 'cd ~/stack/study-app && sqlite3 -header -column data/study.db \
  "SELECT session_id, knowledge_component_id, value, source FROM confidence_log ORDER BY id DESC LIMIT 3;"'
```

Expect ≥1 row — the table that was empty before this change.

## Deploy

Standard claw-study flow: `GOOS=linux GOARCH=amd64 go build` the server **and**
rebuild the `claw-cli` binary (the agent invokes the deployed `claw-cli`, so it
must be rebuilt and copied alongside `study-app`), then
`systemctl --user restart study-app.service`. `AGENTS.md` is re-written on the
next session turn, so the new Rule 3 takes effect automatically.
