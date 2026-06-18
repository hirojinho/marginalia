# specs/ — Overnight pipeline ticket queue

This directory is the input queue for the overnight feature pipeline. One file = one ticket. State is the directory the file lives in.

Authoritative design memory: `~/.claude/projects/<project-slug>/claw_overnight_pipeline.md`.
Implementation plan: `docs/superpowers/plans/2026-05-21-overnight-pipeline-v1.md`.

## Directory state machine

```
queue/        →  in-progress/  →  done/        (shipped)
                                  failed/      (gate or impl failure)
```

State transitions are made by the orchestrator (`overnight-run.sh` on the VPS), committed directly to `main` with `agent: ...` commit messages. Hand-edits between dirs are allowed for reprioritization or cancellation; `git mv` and `git rm` are the operator's tools.

FIFO ordering: filenames are date-prefixed kebab-case (`YYYY-MM-DD-short-title.md`); the orchestrator picks `ls queue/*.md | sort | head -1`.

## Spec schema

### Mandatory frontmatter

```yaml
---
id: 2026-05-21-short-kebab-title         # also the filename stem and branch name
title: Human-readable one-liner
max_wall_clock_minutes: 60               # Pi killed by timeout(1) if exceeded
max_diff_lines: 300                      # Pi aborts implementation if exceeded
max_retries: 1                           # gate-failure retries only; impl failure never retries
max_tokens: 200000                       # Pi exits if exceeded
requires_visual_approval: false          # if true, pipeline pauses at staging; user replies "merge <id>" / "discard <id>"
allow_web_search: false                  # if true, Pi may use claw-cli web search; default off
---
```

### Optional frontmatter (cost / capability tuning)

```yaml
model: mimo-v2.5            # OpenCode-go model slug; default: mimo-v2.5 (medium-tier)
thinking: minimal           # off | minimal | low | medium | high | xhigh; default: minimal
```

**When to override `model:`**
- Trivial mechanical work (endpoint addition, doc fix, schema cleanup) → `deepseek-v4-flash` (lowest cost)
- Default ticket → leave absent (`mimo-v2.5`)
- Tricky cross-cutting refactor → `glm-5.1` or `mimo-v2.5-pro`

**When to override `thinking:`**
- Default `minimal` is enough for "follow the plan" execution
- Bump to `low` if the plan involves subtle Go type-system juggling or algorithmic logic
- Never `xhigh` on the overnight pipeline — that's for interactive exploration, not unattended runs

Both fields are optional. Absent = orchestrator falls back to `PI_MODEL` / `PI_THINKING` env vars (set in `overnight-run.sh`). The orchestrator does not validate the model slug; an invalid slug means Pi errors at startup and the run records `failed-impl`.

### Mandatory body sections

- `## Goal` — one paragraph: what + why.
- `## References` (optional but recommended) — URLs Pi can `claw-cli web fetch` deterministically. Bake research into the spec; don't make Pi freelance.
- `## Implementation plan` — numbered steps, explicit file paths, function names. Specific enough that Pi executes without re-deciding architecture.
- `## Verification recipe` — bash-only verifier. Exit codes are the only judgment that crosses the model/deterministic boundary.
  - `### Pre-baseline (must FAIL on current main)` — Pi runs against current main *before* implementing. Exit non-zero = feature missing as expected. Exit zero = spec already satisfied or verifier broken → Pi aborts to `failed/`.
  - `### Post-acceptance (must PASS after implementation)` — usually identical script. Run against staging with the new binary. Exit zero = ship.
  - `### Human-eyeball notes` — operator context for the morning digest. **Not part of the gate.**
- `## Done criteria` — checklist. For author + reviewer; not enforced by Pi.
- `## Rollback notes` (optional) — data migrations or anything `git revert` alone cannot undo.

## Authoring workflow

On laptop:

1. Draft spec markdown with all mandatory blocks.
2. **Run the pre-baseline verifier locally** against the current prod URL (`STAGING_URL=https://your-host.example STAGING_TOKEN=<prod-token> bash <(awk ...)`). Confirm it fails for the right reason. This is the spec author's TDD-red step — it catches structurally-broken verifiers before Pi ever sees them.
3. `git add specs/queue/<id>.md && git commit && git push`.

The verifier IS the contract. Author quality dominates pipeline quality.

## Example ticket

See `queue/2026-05-21-debug-version-endpoint.md` for a complete, validated reference.

## What lives in each subdirectory

- `queue/` — waiting to be picked. Operator can hand-reorder by renaming.
- `in-progress/` — currently being worked on by tonight's Pi run. The orchestrator moves the file here before invoking Pi and pushes immediately so the state is visible.
- `done/` — gate passed, deployed to prod. Linked from the morning digest.
- `failed/` — pre-baseline rejected the spec, implementation failed, or post-acceptance verifier failed. Triage by reading the matching `~/stack/claw-build/results/<id>/result.json` on the VPS.

## Operator notes

- A `.gitkeep` in each empty directory keeps the structure committable. Do not remove.
- Full Pi transcripts, gate timing, and result JSON live at `~/stack/claw-build/results/<id>/` on the VPS. Telegram digest is triage; the results dir is the audit log.
- Rollback is two-step: `ssh nanoclaw 'bash ~/stack/claw-build/bin/rollback.sh --ticket-id <id>'` (binary swap + `git revert` + push).
