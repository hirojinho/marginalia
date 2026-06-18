# First overnight pipeline run — log

**Date:** 2026-05-21
**Ticket:** `2026-05-21-debug-version-endpoint` (Add `/debug/version`)
**Plan executed:** `docs/superpowers/plans/2026-05-21-overnight-pipeline-v1.md` (Phases 0–7)
**Pipeline runs in this session:** 3 (one failed, two shipped)

## Outcome summary

| Run | Trigger | Outcome | Duration | Commit SHA |
|-----|---------|---------|----------|------------|
| 1 | Manual via SSH `... &` | **Aborted mid-gate** | Pi 203s, then orch vanished | `fbf83ea` (Pi committed locally; never pushed) |
| 2 | `systemd-run --user --collect` | **🟢 Shipped** | 358s (Pi 346s, gate 4s, deploy 1s) | `30c035f` |
| 3 | After rollback + re-queue | **🟢 Shipped** (idempotency check) | 215s (Pi 205s, gate 2s, deploy 1s) | `8a05617` |

End state: `https://your-host.example/debug/version` returns `{"commit":"8a05617e...","built_at":"2026-05-21T15:06:28Z"}` matching `origin/main`.

## What worked first try

- **Pi `--mode json` invocation.** No hangs, no Issue #161 zombies. Exits cleanly at `turn_end` event. The `--no-skills + --skill <path>` isolation keeps Pi loading only `implement-from-spec`.
- **The spec-author red step (Phase 7.2).** Pre-baseline verifier returned exit 22 (`curl -sf` against 404) against current prod — confirmed the verifier is structurally correct *before* Pi ever ran. This is the spec author's TDD discipline doing its job.
- **Staging isolation (Phase 2).** Round-trip on port 8082 with copied `study.db` + symlinked read-only sibling dirs (corpus, courses, memory, pdf-*). Prod on 8081 served at 200 throughout all three runs.
- **Gate's awk block extraction.** Pulled the LAST `\`\`\`bash` block from each `### Pre-baseline` / `### Post-acceptance` section. For ticket 1's `### Post-acceptance` which says "Same script as above", the gate-runner's fallback to the pre-baseline block worked unchanged.
- **Trust-gradient classification.** Run 2's `result.json` shows three discrete gate steps (`pi_run` / `gate` / `deploy`) with exit codes — exactly the artifact the morning digest needs.
- **Two-step rollback.** Step A (binary swap) took ~1s with no prod blip beyond the systemd restart window. Step B (`git revert` + push) pushed the revert commit cleanly to `origin/main`.

## What broke (and was fixed in this session)

### 1. Run 1 silently died after Pi finished — `lib/result.sh` set `-e` in the orchestrator

Sourcing `lib/result.sh` was setting `set -euo pipefail` in the caller. When `gate-runner.sh` exited non-zero (intentionally — to be classified by `$?` capture), `set -e` killed the orchestrator before `result_step gate` could record the failure. Symptom: `result.json` stuck at `outcome: unknown`, orch log truncated at "running gate for ...".

**Fix:** removed `set -e` from the library file. Libraries must not mutate caller flags. Caller (`overnight-run.sh`) keeps explicit `$?` capture per phase.

### 2. Go missing on VPS (Phase 0 miss)

`gate-runner.sh` step `go build` failed because there was no `go` on PATH. Apt's `golang-go` is 1.19.8; claw-study's `go.mod` requires 1.24.1. Plan Step 0.x didn't actually check this.

**Fix:** installed Go 1.24.13 to `/usr/local/go`. Updated:
- `gate-runner.sh`: prefer `/usr/local/go/bin/go` before falling back to PATH.
- `bootstrap/12-claw-build-setup.sh`: added Go version-floor check (1.24.0) so re-running bootstrap on a fresh VPS would catch this up-front.

### 3. SSH-backgrounded process is fragile

Run 1 was launched via `ssh nanoclaw 'cmd > log 2>&1 &'`. When SSH closed, the orchestrator persisted *during* Pi execution but vanished shortly after Pi exited (likely a TTY/process-group issue when stdout pipe interaction with the orphaned shell hit a problem). The set-e bug actually masked this — but even with the set-e bug fixed, the SSH-detach pattern is unreliable for multi-stage unattended work.

**Fix:** subsequent runs launched via `systemd-run --user --unit=<name> --collect --property=Type=oneshot`. Fully detached, journaled, restart-safe. This is the same launch surface Phase 8's path-unit will use, so it's also a pre-validation of that mechanism.

### 4. Plan-doc drift caught during execution

Inline corrections recorded in commit messages:
- **Service names.** Plan referred to `claw-study.service` / `claw-study-tunnel.service`. Actual deployed unit names are `study-app.service` / `study-app-tunnel.service`. All scripts (`staging-up.sh`, `deploy-swap.sh`, `rollback.sh`) reference the correct names.
- **Skill format.** Plan called for `SKILL.md` + `PROMPT.md`. Looking at the existing `claw-study/skills/*` and `agent/memory.go:248`, the skill loader only reads `SKILL.md`. No `PROMPT.md` written.
- **Skill deployment.** Plan called for `rsync skills/implement-from-spec/ → ~/stack/study-app/skills/`. That target is for the live chat-v2 Pi. The overnight Pi runs from `~/stack/claw-build/worktree/` and loads the skill from there via `git fetch`. No rsync needed.
- **`feedback_research_niche_oss.md`.** Plan Step 0.2 referenced this memory as a prerequisite. It existed but was missing from the `MEMORY.md` index — added in this session.
- **Phase 1 ticket-1 spec.** Plan said it "already exists" — and it did, locally untracked. Committed as part of the Phase 1 commit.

### 5. Minor: `rollback-history.jsonl` reports `step_b: skipped` even when step B ran

Cosmetic. The subshell that runs `git revert` sets `revert_status="ok"` but the assignment doesn't propagate back to the parent — by the time the JSONL line is written, `revert_status` is still the initial `"skipped"`. Step B actually executed and pushed correctly to `origin/main`; only the audit log is wrong.

**Follow-up (v1.1, not blocking):** move the `revert_status` accounting out of the subshell or use a temp file to communicate state across the subshell boundary.

## Phase boundary timings

| Phase | Took | Notes |
|-------|------|-------|
| 0 — pre-flight | ~10 min | Pi upgrade + Go install + state cleanup + memory authoring |
| 1 — scaffolding | ~5 min | Bootstrap was idempotent on second attempt (SSH URL fix) |
| 2 — staging launcher | ~10 min | Spin/down validated; symlink approach for read-only sibs |
| 3 — Pi skill | ~10 min | Inline corrections to plan's skill format assumptions |
| 4 — gate runner | ~10 min | Awk block extraction; deferred deep validation to Phase 7 |
| 5 — deploy + rollback | ~10 min | Self-swap drill; jq install needed mid-phase |
| 6 — orchestrator | ~15 min | The big one; later required the -e fix to actually work |
| 7 — E2E run | ~45 min | Two failed-then-succeeded full runs + rollback + idempotency re-run |

## What I'd change if doing this again

1. **Phase 0 should check the build toolchain.** Add `go --version` (with version floor) and `jq --version` to the explicit pre-flight list. Both were missing on VPS and surfaced as mid-phase failures.

2. **Sanity-test the orchestrator with a no-op spec before ticket 1.** A spec whose `## Implementation plan` is empty and verifier is trivially `exit 1` / `exit 0` would catch wiring bugs without burning Pi time.

3. **The plan should call out library-file conventions.** `lib/log.sh` correctly didn't set `-e`; `lib/result.sh` did and broke the orchestrator. A line in `writing-scripts` doctrine would prevent this from recurring.

4. **Launch via `systemd-run --user` from day 1.** SSH-backgrounded scripts work in dev but fail under unattended cron. Test the launch surface that Phase 8 will use, from the very first manual run in Phase 7.

## Ready for Phase 8

The trigger automation (Claw cron + systemd path unit) needs:
- `claw-study-overnight.service` (analog of `overnight-run-test*.service` but path-triggered, not one-shot)
- `claw-study-overnight.path` watching `~/stack/claw-build/state/tick.requested`
- Claw cron job that writes the tick file at `0 2 * * *` America/Sao_Paulo

The orchestrator already includes `rm -f tick.requested` as its first action (per plan Step 8.3), so the path unit will re-arm cleanly.

## Open follow-ups (v2 candidates)

- Cosmetic rollback-history bug (item 5 above)
- Cycle detection for Pi looping on a single failing test
- Multi-ticket-per-night drain
- Telegram-triggered rollback
- `claw-cli web search` backend (Brave free tier)

None blocking v1.
