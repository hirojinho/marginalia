---
name: pipeline-doctor
description: Check the overnight feature pipeline state, diagnose any failures, fix issues (spec or pipeline code), re-ship stalled tickets, and harden the pipeline to prevent recurrence. Use when the user asks to check/fix the pipeline, when a pipeline failure is reported, or when a ticket needs to be re-shipped after manual fixes.
allowed-tools: read write edit bash
---

# pipeline-doctor

## Overview

You are the overnight pipeline diagnostician and fixer. Your job is to:
1. **Check** — surface the current pipeline state: queue, last result, timer health, deployed version
2. **Diagnose** — classify what failed and why
3. **Fix** — repair the spec, re-queue, or fix pipeline code
4. **Ship** — run the gate manually, deploy to production, move spec to done
5. **Harden** — fix the pipeline itself so this class of failure doesn't recur

**Crucial rule**: make the minimal change that unblocks the pipeline. Do not rewrite specs from scratch — take over from where Pi left off. If Pi's implementation is correct, reuse it; don't re-run Pi. If the gate-runner or orchestrator has a bug, fix those scripts (in `~/Documents/ITA/infra/scripts/`) and deploy to the VPS.

## Phase 1 — Surface State

Run these checks in order. Do not skip any.

### 1.1 VPS pipeline health

```bash
ssh nanoclaw '
echo "=== TIMER ==="
systemctl --user status claw-study-overnight.timer 2>/dev/null | head -8
echo ""
echo "=== LAST SERVICE RUN ==="
systemctl --user status claw-study-overnight.service 2>/dev/null | head -10
echo ""
echo "=== QUEUE ==="
cd ~/stack/claw-build/worktree && git fetch --prune origin && git checkout main && git pull --ff-only origin main >/dev/null 2>&1
ls specs/queue/
echo ""
echo "=== RESULTS ==="
ls ~/stack/claw-build/results/ 2>/dev/null | tail -10
echo ""
echo "=== LAST DEPLOYED ==="
cat ~/stack/claw-build/state/last-deployed.json 2>/dev/null || echo "(no record)"
echo ""
echo "=== PROD HEALTH ==="
curl -sf -o /dev/null -w "HTTP %{http_code}" -H "Authorization: Bearer $(grep AUTH_TOKEN ~/stack/study-app/.env | cut -d= -f2-)" http://127.0.0.1:8081/debug/health 2>/dev/null || echo "health check failed"
'
```

If SSH fails, report "VPS unreachable" and stop. Everything else depends on this.

### 1.2 Latest result (if any)

If `~/stack/claw-build/results/` has entries, read the latest non-idle `result.json`:

```bash
ssh nanoclaw 'for d in $(ls -1dt ~/stack/claw-build/results/*/ 2>/dev/null); do
  outcome=$(jq -r .outcome "$d/result.json" 2>/dev/null)
  if [ "$outcome" != "idle" ] && [ -n "$outcome" ]; then
    echo "=== $d ==="
    cat "$d/result.json"
    break
  fi
done'
```

**Interpreting outcomes:**
- `shipped` — nothing to do. Pipeline is healthy.
- `paused-at-staging` — needs manual merge/discard decision. Gate passed; binary is staged.
- `failed-gate` (exit 10) — **pre-baseline verifier bug**. The verifier unexpectedly passed on main. Most common cause: inverted exit codes (`! grep -q` pattern).
- `failed-gate` (exit 11) — **build failed**. Pi's code doesn't compile.
- `failed-gate` (exit 12) — **test failed**. Pi's code breaks existing tests.
- `failed-gate` (exit 13) — **post-acceptance failed**. Verifier rejected the new binary.
- `failed-impl` — Pi couldn't implement. Read `theory` field for Pi's explanation.
- `failed-gate` (other) — `theory` field has the cause.

### 1.3 Local repo sync

```bash
cd ~/Documents/ITA/claw-study && git fetch origin && git pull --ff-only origin main
cd ~/Documents/ITA/infra && git fetch origin && git pull --ff-only origin main
```

Both repos must be on the latest main before any fixes.

## Phase 2 — Diagnose

Based on the outcome, follow the matching branch below.

### 2A. `failed-gate` exit 10 — pre-baseline passed unexpectedly

This is the most common failure. The pre-baseline verifier script in the spec exits 0 (success) when it should exit non-zero (features absent). Read the gate log:

```bash
ssh nanoclaw 'cat ~/stack/claw-build/results/<TICKET_ID>/gate.log'
```

Then read the spec's `### Pre-baseline` section to inspect the verifier. Common bugs:

| Pattern | Problem | Fix |
|---|---|---|
| `! grep -q ...` as last command | `! grep` exits 0 when pattern absent — inverted | Rewrite as `if grep; then echo UNEXPECTED; exit 0; fi` with `exit 1` at end |
| No `exit` statement | Script exits with last command's code, which may be 0 | Add explicit `exit 1` at end |
| Verifier greps for wrong pattern | Pattern matches something already on main | Tighten the grep pattern |

**Check if Pi's implementation is correct before rewriting anything:**

```bash
ssh nanoclaw 'cd ~/stack/claw-build/worktree && git log --oneline agent/<TICKET_ID> -3 && git diff --stat origin/main..agent/<TICKET_ID>'
```

If Pi committed code that matches the spec's `## Implementation plan` — the implementation is correct. Just fix the verifier, re-queue, and run the gate manually. Do NOT re-run Pi.

### 2B. `failed-gate` exit 11/12 — build or test failure

Read the gate log to find the exact error. Check if Pi's implementation has a bug (wrong code) or if the spec's plan has an error (referenced a non-existent file, wrong API). If simple to fix:

1. Checkout the agent branch on the VPS worktree
2. Apply the fix directly (the implementation is Pi-level work, but fixing a build break is pipeline recovery)
3. Amend the commit or create a new one
4. Re-run the gate

If the implementation is fundamentally wrong, move the spec to `failed/` with a note and let the user re-spec it.

### 2C. `failed-gate` exit 13 — post-acceptance failed

The verifier rejected the new binary. Read both the gate log and the verifier script from the spec. Possible causes:

- **`go` not on PATH** — the verifier runs `go build/test` but `/usr/local/go/bin` isn't in PATH. Fix: the gate-runner now exports Go's dir, but check if the fix is deployed.
- **Test assertion wrong** — the verifier expected a specific output format that changed.
- **Staging did not start** — check `~/stack/claw-build/staging-data/<id>-gate-post/staging.log`.

### 2D. `failed-impl` — Pi couldn't implement

Read Pi's `pi-failed.json` theory. Common causes:
- Schema invalid (exit 2) — spec frontmatter missing a required field or body section
- File reference broken (exit 3) — the spec's `## Implementation plan` references a file that doesn't exist
- Token cap hit (exit 5) — the spec's `max_tokens` is too low for the complexity

Fix the spec and re-queue. Do not patch Pi's output.

### 2E. `paused-at-staging` — needs decision

Check the paused state:

```bash
ssh nanoclaw 'cat ~/stack/claw-build/state/paused/<TICKET_ID>.json'
```

The user must decide: `merge <id>` or `discard <id>`. For merge, run `complete-merge.sh`. For discard, run `discard-paused.sh`.

## Phase 3 — Fix

### 3A. Fixing a spec (verifier or plan bug)

1. **Read the full spec** from the local repo (pull first)
2. **Fix the issue** — usually the `### Pre-baseline` verifier
3. **Move the spec back to queue** if needed: `git mv specs/failed/<id>.md specs/queue/<id>.md`
4. **Commit + push**: `git commit -m "fix(spec): <what was wrong>" && git push origin main`
5. **Verify on VPS**: `ssh nanoclaw 'cd ~/stack/claw-build/worktree && git pull --ff-only origin main && ls specs/queue/<id>.md'`

**Verifier fix template** (pre-baseline):

```bash
# Pre-baseline: features should NOT exist on current main.
# Exit 1 when features are absent (expected pre-state → gate proceeds).
# Exit 0 when any feature IS present (unexpected → gate fails, correct).

if grep -q 'FeaturePattern' path/to/file.go; then
  echo "UNEXPECTED: feature already exists on main"
  exit 0
fi
# Add one if-block per feature being checked.
# ...
# All features absent — expected. Signal "fail" (non-zero).
exit 1
```

**Critical**: the pre-baseline MUST exit non-zero when features are absent. The gate interprets non-zero as "feature doesn't exist yet — proceed." Zero means "feature already shipped — abort."

### 3B. Fixing pipeline code (gate-runner / orchestrator / etc.)

Pipeline scripts live in `~/Documents/ITA/infra/scripts/`. Fix them locally:

1. **Edit the script** — `gate-runner.sh`, `overnight-run.sh`, `staging-up.sh`, etc.
2. **Syntax check**: `bash -n scripts/<name>.sh`
3. **Commit + push**: `git commit -m "fix: <what>" && git push origin main`
4. **Deploy to VPS**: `scp scripts/<name>.sh nanoclaw:/path/to/claw-build/bin/`
5. **Verify on VPS**: `ssh nanoclaw 'bash -n ~/stack/claw-build/bin/<name>.sh'`

### 3C. Pipeline code reference

| Script | Location (laptop) | Location (VPS) | Purpose |
|---|---|---|---|
| `gate-runner.sh` | `~/Documents/ITA/infra/scripts/` | `~/stack/claw-build/bin/` | Verifier gate (pre-baseline, build, test, post-acceptance) |
| `overnight-run.sh` | `~/Documents/ITA/infra/scripts/` | `~/stack/claw-build/bin/` | Orchestrator (pick ticket, invoke Pi, run gate, deploy) |
| `deploy-swap.sh` | `~/Documents/ITA/infra/scripts/` | `~/stack/claw-build/bin/` | Atomic binary swap + restart + health probe |
| `rollback.sh` | `~/Documents/ITA/infra/scripts/` | `~/stack/claw-build/bin/` | Two-step rollback (binary swap + git revert) |
| `staging-up.sh` | `~/Documents/ITA/infra/scripts/` | `~/stack/claw-build/bin/` | Spin staging instance on :8082 |
| `staging-down.sh` | `~/Documents/ITA/infra/scripts/` | `~/stack/claw-build/bin/` | Tear down staging instance |
| `complete-merge.sh` | `~/Documents/ITA/infra/scripts/` | `~/stack/claw-build/bin/` | Merge a paused-at-staging ticket |
| `discard-paused.sh` | `~/Documents/ITA/infra/scripts/` | `~/stack/claw-build/bin/` | Discard a paused-at-staging ticket |

### 3D. Pi's implementation is correct — skip to gate

If Pi already committed correct code on the agent branch (check via `git diff --stat origin/main..agent/<id>`), do NOT re-run Pi. Run the gate manually:

```bash
ssh nanoclaw 'export XDG_RUNTIME_DIR=/run/user/$(id -u) && \
  ~/stack/claw-build/bin/gate-runner.sh \
    --spec ~/stack/claw-build/worktree/specs/queue/<id>.md \
    --worktree ~/stack/claw-build/worktree \
    --ticket-id <id> \
    --agent-branch agent/<id> \
    2>&1'
```

## Phase 4 — Ship (deploy to production)

After the gate passes (exit 0), the gated binary is at `~/stack/claw-build/bin/study-app-gated-<id>`. Deploy:

```bash
ssh nanoclaw 'export XDG_RUNTIME_DIR=/run/user/$(id -u) && \
  cd ~/stack/claw-build/worktree && \
  # If agent branch needs rebasing (main moved forward):
  git checkout agent/<id> && git rebase main && \
  git push origin agent/<id> --force-with-lease && \
  # FF-merge to main:
  git checkout main && git merge --ff-only agent/<id> && \
  git push origin main && \
  # Move spec to done:
  git mv specs/queue/<id>.md specs/done/<id>.md && \
  git -c user.email=you@example.com \
      -c user.name=your-name \
      commit -m "agent: shipped <id>" && \
  git push origin main && \
  # Deploy:
  ~/stack/claw-build/bin/deploy-swap.sh \
    --new-binary ~/stack/claw-build/bin/study-app-gated-<id> \
    --ticket-id <id> \
    --commit-sha $(git rev-parse HEAD) \
    2>&1'
```

**Verify deployment:**

```bash
ssh nanoclaw '
echo "=== Version ==="
curl -sf -H "Authorization: Bearer $(grep AUTH_TOKEN ~/stack/study-app/.env | cut -d= -f2-)" http://127.0.0.1:8081/debug/version
echo ""
echo "=== Service ==="
systemctl --user status study-app.service | head -5
'
```

## Phase 5 — Pipeline Hardening

After shipping the ticket, prevent this class of failure from recurring.

### 5A. Gate-runner diagnostics

The current gate-runner already:
- Warns on `! grep -q` patterns in pre-baseline verifiers
- Dumps the verifier script on pre-baseline unexpected-pass
- Dumps the verifier script on post-acceptance failure
- Exports Go bin dir to verifier scripts

If you encounter a NEW failure mode that the gate-runner doesn't surface clearly, add diagnostics. Common improvements:
- Log the verifier exit code explicitly on failure
- Show staging logs on staging startup failure
- Add a timeout for staging health probes beyond the 30s default

### 5B. Spec template guard

If the failure was a spec bug (not pipeline bug), consider adding a lint rule to `scripts/lint-specs.sh` that catches the pattern. For example, a rule that flags `! grep -q` in `### Pre-baseline` blocks:

```bash
# In lint-specs.sh, add after the existing checks:
if awk '/^### Pre-baseline/,/^### /' "$spec" | grep -q '^! grep -q'; then
  echo "WARN: pre-baseline uses '! grep -q' — this inverts exit codes" >&2
fi
```

### 5C. Gate-runner hardening ideas (for v2)

Patterns to add if they recur:
- **Pre-baseline self-test**: extract the pre-baseline block, inject a fake "feature present" line, run it — if it still exits non-zero, the verifier is broken regardless of what's on main
- **Verifier timeout**: kill verifier scripts that run longer than 60s
- **Stale agent branch detection**: warn if agent branch is >24h old (Pi should not run on stale branches)

## Common Failure Pattern Reference

### Pattern A: Inverted pre-baseline verifier

**Symptom**: `failed-gate` exit 10. Gate log says "pre-baseline UNEXPECTEDLY passed."

**Root cause**: The verifier uses `! grep -q <pattern>` which exits 0 when the pattern is NOT found (features absent). The gate expects non-zero for "features absent."

**Fix**: Rewrite as `if grep -q <pattern>; then echo UNEXPECTED; exit 0; fi` chains with `exit 1` at the end. See Phase 3A template.

**Prevention**: The gate-runner now warns on `! grep -q`. Spec authors: use the template in Phase 3A.

### Pattern B: Agent branch diverged from main

**Symptom**: `git merge --ff-only` fails with "Not possible to fast-forward."

**Root cause**: Pipeline lifecycle commits (spec moves, verifier fixes) landed on main after the agent branch was forked.

**Fix**: Rebase the agent branch onto main, force-push, then FF-merge:
```bash
git checkout agent/<id> && git rebase main && \
git push origin agent/<id> --force-with-lease && \
git checkout main && git merge --ff-only agent/<id>
```

### Pattern C: Spec not found after branch switch

**Symptom**: Gate-runner fails with "spec not readable" or "no bash block extracted."

**Root cause**: The gate-runner switches to the agent branch (which was forked before the spec was re-queued), and the spec file at its absolute path doesn't exist on that branch.

**Fix**: The gate-runner now extracts verifiers upfront while on main, before any branch switch. If this still occurs, the gate-runner deploy may be stale — SCP the latest gate-runner.sh to the VPS.

### Pattern D: `go` not found in verifier

**Symptom**: Post-acceptance fails with `go: comando não encontrado` (or `command not found`).

**Root cause**: The verifier script runs `go build/test` but `/usr/local/go/bin` isn't on the default PATH.

**Fix**: The gate-runner now exports `PATH="$(dirname $GO_BIN):$PATH"` before running verifier scripts. If this still occurs, the gate-runner deploy may be stale.

### Pattern E: Worktree not on expected branch

**Symptom**: Gate-runner or manual commands operate on the wrong branch.

**Root cause**: The gate-runner leaves the worktree on the agent branch. Orchestrator leaves it on main. Manual intervention can leave it anywhere.

**Fix**: Always explicitly `git checkout <branch>` before operations that depend on branch state. The gate-runner now manages its own branch checkout internally.

## Session Checklist

Before concluding a pipeline-doctor session, verify:

- [ ] Queue state is correct (ticket shipped → in `done/`; ticket failed → in `failed/` with theory)
- [ ] Local repos are synced (`git pull` both claw-study and infra)
- [ ] Production is running and `/debug/health` returns 200
- [ ] `last-deployed.json` matches the deployed commit
- [ ] Systemd timer is active and shows a next-fire time within 24h
- [ ] Remaining queue is visible and ordered correctly
- [ ] Any pipeline code fixes are committed to infra AND SCP'd to VPS
- [ ] If a new failure pattern was discovered, it's documented here in "Common Failure Pattern Reference"
