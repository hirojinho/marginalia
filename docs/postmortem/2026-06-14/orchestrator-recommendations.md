# Orchestrator hardening backlog (from 2026-06-14 postmortem)

These three fixes came out of the r9 postmortem. **They are NOT overnight-queueable
tickets** — they change the orchestrator scripts at `~/stack/claw-build/bin/*.sh`
on the VPS, which are not version-controlled and which the overnight pipeline never
touches (v1 modifies only the claw-study repo; the pipeline implementing its own
scripts would be a recursive failure mode). Apply these by hand, with a
`*.sh.bak.YYYYMMDD` backup first (the established convention).

## 1. Hard-fail `! grep` pre-baseline verifiers (don't just warn)

**Where:** `gate-runner.sh`, the pre-baseline block (it already emits
`[WARN] pre-baseline verifier uses '! grep -q' pattern`).
**Problem:** exit 10 ("pre-baseline passed unexpectedly") is ambiguous — it looks the
same whether the spec is genuinely already-shipped or the verifier is inverted. Six
tickets hit this (fix-tool-panel-wipe, r11-r12, r10-pretesting, bloom-enforcement,
kc-capture-box, + the pattern).
**Fix:** when the verifier matches `! grep`, exit with a distinct non-zero code
(e.g. 20 = "verifier malformed") and a clear message, instead of running it and
landing on the ambiguous exit 10. Pairs with the new `AGENTS.md` author-side rule.

## 2. Failed runs must move the spec to `failed/` (no orphans)

**Where:** `overnight-run.sh`, the failure/exit paths.
**Problem:** r9 ended in **no** spec dir at all (not `done/`, `failed/`, or `queue/`)
because the failed branch wasn't pushed and the spec move only happened on the
(unpushed) branch. The spec became invisible to the next morning's triage.
**Fix:** on any non-zero gate exit, commit the `queue/ → failed/` move **on main**
(or push the branch) so the spec always has a discoverable home. The morning digest's
"missing result" detection should also flag a queue item that vanished without a
corresponding `done/` or `failed/` entry.

## 3. Re-gate when the merge base moved (don't trust stale-base green)

**Where:** `overnight-run.sh`, the deploy/merge path (currently FF-only-or-abort).
**Problem:** r9 was gated off `57b886f`, but main advanced to `fd70cec`/`acd56a9`
(same-day live-dev) before the salvage. Gate-green on a stale base is not ship-safe.
The current FF-only policy *aborts* in this case (predictable, but loses the run).
**Fix (optional enhancement):** when FF is rejected because main moved, instead of
only aborting, attempt `merge origin/main` into the agent branch and **re-run the
full gate on the merged result**; ship only if the re-gate is green. This is exactly
the manual salvage we did for r9 (merge → re-gate `8e211c0` → deploy `1ee685d`).
Keep abort as the fallback if the merge conflicts non-trivially.

## Also worth doing
- **Strip `*.bak`/`*.orig` pre-commit in the executor flow** (belt for the new
  `implement-from-spec` "no `git add -A`" rule) — r9's run committed 5 `.bak` files.
