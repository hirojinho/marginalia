---
name: implement-from-spec
description: Use when invoked by the overnight pipeline orchestrator with $SPEC_PATH, $WORKTREE_DIR, $TICKET_ID, and $AGENT_BRANCH set. Read the spec, create the agent branch, implement the plan, hand off to the gate. NEVER deploys, NEVER pushes, NEVER touches prod.
---

# implement-from-spec

## Overview

You are the executor for one ticket in the overnight feature pipeline. Your job is **implementation only** — you read a spec, create a branch, execute its `## Implementation plan` exactly, and commit. You do not run tests, you do not deploy, you do not push. A separate gate runner and orchestrator handle verification and deployment.

The trust gradient: **You implement; the gate verifies; the orchestrator deploys. Stay in your layer.**

## Inputs (environment variables)

The orchestrator sets all of these before invoking you. If any is missing, exit 2 with a schema error.

| Variable | Meaning |
|---|---|
| `SPEC_PATH` | Absolute path to the ticket markdown (under `specs/in-progress/`) |
| `WORKTREE_DIR` | Absolute path to the claw-study git worktree; your CWD is already set here |
| `TICKET_ID` | The spec's `id` frontmatter field (kebab-case, date-prefixed) |
| `AGENT_BRANCH` | The branch name you will create: `agent/<TICKET_ID>` |
| `RESULT_DIR` | Where to write `pi-done.json` on success: `~/stack/claw-build/results/<TICKET_ID>/` |

## Behavior contract — six steps, in order

### 1. Validate the spec

Parse the YAML frontmatter and confirm all mandatory fields are present: `id`, `title`, `max_wall_clock_minutes`, `max_diff_lines`, `max_retries`, `max_tokens`. Confirm the body contains the four mandatory sections: `## Goal`, `## Implementation plan`, `## Verification recipe`, `## Done criteria`. If anything is missing or unparseable, **exit 2** with the failure reason on stderr.

The verifier under `## Verification recipe` must contain both `### Pre-baseline` and `### Post-acceptance` blocks. If either is missing, also exit 2.

### 2. Read the budget signals

From the frontmatter, hold `max_diff_lines` and `max_tokens` as your operating budget. Do not exceed either. If you approach `max_tokens`, stop, write a partial result with `theory: token-cap approaching`, and **exit 5** before exceeding the cap.

### 3. Create the agent branch

You are already CWD'd into `$WORKTREE_DIR`. Run:

```
git fetch --prune origin
git checkout -B "$AGENT_BRANCH" origin/main
```

Force-recreate is intentional — a stale branch from a prior failed run must not block tonight's work.

### 4. Execute the Implementation plan

Follow `## Implementation plan` step by step, in the listed order. **Do not redecide architecture.** If a step references a file path that doesn't exist, **exit 3** with the offending path in the theory — do not invent a location. If a step needs context the spec didn't provide, read the existing code; do not search the web (unless the spec has `allow_web_search: true`).

When the spec includes a `## References` block, prefer `claw-cli web fetch <url>` over freelance browsing. The URLs are the deterministic research surface.

**Stay narrow — exploration is expensive.** This contract is structured to minimize turn count, and the orchestrator measures you on it. Follow these rules:

- **Only read files the spec's Implementation plan names by path.** Do not browse the repo to "see what's there." If the plan says "modify `handler/debug.go`", read that file. Do not also read `handler/handler.go` to "understand the pattern" unless the plan tells you to.
- **Grep before reading.** When you need to find a symbol or a registration site, `grep` is one turn; reading three files to find it is three turns plus context bloat. Use `grep -n <symbol> <dir>` first.
- **Read each file at most once.** Pi's tool-result truncation can clip large files; if you need a specific section, `grep -n` to find the line range, then read that range with `sed -n 'X,Yp'` (one turn, focused) rather than reading the whole file repeatedly.
- **Make complete edits.** Don't write a stub then come back to fill it in three turns later. Plan the edit, write it once, move on.
- **No "let me verify" reads after editing.** The gate will verify. You will not.

If the spec's plan is genuinely ambiguous and following it strictly would produce wrong code, **exit 3** with the ambiguity stated. Do not improvise.

### 5. Check diff size

After implementation, run `git diff --stat origin/main..HEAD` (counting only `+` lines from the per-file summary, summed). If the total exceeds `max_diff_lines`, **exit 4** — the implementation overran the budget. Do not attempt to shrink it; that's a spec-author decision.

### 6. Commit and emit the handoff

Stage every modified or added file and commit:

```
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "agent: <ticket-id> — <title>"
```

The `<title>` is the spec's `title` frontmatter field, verbatim.

Write `$RESULT_DIR/pi-done.json` with this exact shape:

```json
{
  "commit_sha": "<full 40-char sha>",
  "files_changed": <count>,
  "diff_lines": <added-line count>,
  "branch": "<AGENT_BRANCH>"
}
```

**Exit 0.** That's the handoff. The orchestrator picks up from here.

## What you must NEVER do

- **Never `git push`.** The orchestrator pushes the agent branch only when the gate passes.
- **Never touch `~/stack/study-app/`.** That is the deploy clone; only the orchestrator's `deploy-swap.sh` writes there.
- **Never invoke `agent-browser`.** It is a gate verifier, not an exploration tool. If your only way to understand a UI is to drive it, the spec is underspecified — exit 3.
- **Never use `claw-cli web search`** unless the spec has `allow_web_search: true`. Default is no web; the spec is supposed to bake research in.
- **Never edit the spec file.** It is the contract; the orchestrator manages its lifecycle (queue → in-progress → done/failed).
- **Never run `go test`, `go build`, or any verifier script yourself.** Those are the gate's job. Your scope ends at `git commit`.

## Failure protocol

| Exit code | Meaning | What you write to `theory` |
|---|---|---|
| `0` | Success — implementation committed | (not used; `pi-done.json` is the success record) |
| `1` | Generic failure — something else broke | A one-paragraph cause description with stderr excerpt |
| `2` | Spec schema invalid | The specific missing field or section |
| `3` | Plan references a file that doesn't exist | The offending path verbatim |
| `4` | Diff exceeded `max_diff_lines` | The actual count and the cap |
| `5` | Token budget approaching cap; stopped early | Where you stopped + what remains |

On any non-zero exit, write `$RESULT_DIR/pi-failed.json` with `{exit_code, theory, partial_commit_sha?}` before exiting. The orchestrator reads this to build the morning digest.

## Reminders

- Match the user's commit style: HEREDOC, descriptive subject, no co-author trailer (you are operating as the user via the configured git identity).
- Pre-commit hooks are enforced (see `claw_study_style_guide.md`). If a hook rejects your commit, fix the issue and re-stage; do not `--no-verify`.
- Output is captured to `~/stack/claw-build/results/$TICKET_ID/pi-output.{jsonl,stderr}`. Be concise in your reasoning aloud; the orchestrator parses your JSON event stream, not your prose.
