# Orchestrator hardening backlog (from 2026-06-14 postmortem)

> **STATUS 2026-06-15 — ALL SHIPPED.** Applied by hand on the VPS with backups
> `overnight-run.sh.bak.20260615` / `gate-runner.sh.bak.20260615`. Each fix was
> tested before deploy (lock idiom + idle integration run on VPS; `! grep` regex
> unit test; orphan + re-gate git mechanics in synthetic repos). A **fourth**
> latent bug surfaced during the check — a *re-regression* of the 2026-05-29
> lock-release fix (lost in the Jun 9 edit) — fixed in the same pass (item 0 below).
> See the per-item ✅ notes.

These three fixes came out of the r9 postmortem. **They are NOT overnight-queueable
tickets** — they change the orchestrator scripts at `~/stack/claw-build/bin/*.sh`
on the VPS, which are not version-controlled and which the overnight pipeline never
touches (v1 modifies only the claw-study repo; the pipeline implementing its own
scripts would be a recursive failure mode). Apply these by hand, with a
`*.sh.bak.YYYYMMDD` backup first (the established convention).

## 0. Stale `active.lock` — lock-release regression (found 2026-06-15)

**Where:** `overnight-run.sh` `acquire_lock`.
**Problem:** the 2026-05-29 fix (open fd with `<>`, arm `trap release_lock EXIT`)
was **silently lost** in the Jun 9 edit — the live script had `exec 200>"$LOCK_FILE"`
and **no release trap**. Every run (incl. last night's 06-15 idle) left its PID json
in `active.lock`; only the *next* run's `>`-truncate cleared it. The documented
invariant "non-empty `active.lock` ⟺ a run is active" was false for ~6 days. Harmless
to scheduling (flock auto-releases on fd close) but a misleading ops/digest signal —
exactly the "stale process model" control failure the postmortem flagged.
**Fix ✅ 2026-06-15:** restored `release_lock(){ : > "$LOCK_FILE"; }` + `trap release_lock
EXIT` armed once the lock is owned; reopened fd with `<>` (not `>`) so the contended
liveness read isn't truncated. Integration-tested: ran the orchestrator against the
empty queue → `active.lock` empty on exit, stale 928208 lock cleared.

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
**Fix ✅ 2026-06-15:** `gate-runner.sh` now hard-fails with **exit 20** (dumping the
verifier) on `^[[:space:]]*![[:space:]]*grep` — catches both `! grep` and `!grep`.
`overnight-run.sh` maps exit 20 → theory "pre-baseline verifier malformed (! grep
inversion)". Regex unit-tested against 5 verifier shapes.

## 2. Failed runs must move the spec to `failed/` (no orphans)

**Where:** `overnight-run.sh`, the failure/exit paths.
**Problem:** r9 ended in **no** spec dir at all (not `done/`, `failed/`, or `queue/`)
because the failed branch wasn't pushed and the spec move only happened on the
(unpushed) branch. The spec became invisible to the next morning's triage.
**Fix:** on any non-zero gate exit, commit the `queue/ → failed/` move **on main**
(or push the branch) so the spec always has a discoverable home. The morning digest's
"missing result" detection should also flag a queue item that vanished without a
corresponding `done/` or `failed/` entry.
**Fix ✅ 2026-06-15:** both failure paths in `overnight-run.sh` (failed-impl and
failed-gate) now `git checkout -f main && git pull --ff-only` **before** the
`git mv … failed/` + commit + push, so the move lands on the pushed `main` instead of
the throwaway agent branch. Root cause confirmed: `gate-runner.sh` leaves the worktree
checked out on the agent branch. Verified in a synthetic repo (spec ends in `failed/`
on origin/main, `in-progress/` cleared). *(Digest "vanished-queue-item" detector: not
done — lower priority now that the move is reliable.)*

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
**Fix ✅ 2026-06-15:** implemented in `overnight-run.sh`. On FF-reject it now
`git merge --no-edit main` into the agent branch, re-runs the **full trusted gate**
(`gate-runner.sh`) on the merged result (log → `results/<id>/gate-regate.log`,
`regate` step in result.json), and FF-merges only if the re-gate is green; aborts to
`failed/` (via shared `fail_to_failed`) on merge conflict, red re-gate, or a second
FF-reject. Git mechanics verified in a synthetic repo reproducing the r9 situation
(agent edits rule6, main advances on plan-active, zero overlap → clean merge → both
edits land on main). The new untested-in-integration surface is only the git merge;
the build/test/staging steps reuse the already-trusted first-gate path.

## Also worth doing
- **Strip `*.bak`/`*.orig` pre-commit in the executor flow** (belt for the new
  `implement-from-spec` "no `git add -A`" rule) — r9's run committed 5 `.bak` files.
  *(2026-06-15: SKIPPED as redundant — the postmortem's `852a9dd` already added a
  `.gitignore` `*.bak`/`*.orig` pattern, so even an accidental `git add -A` can't
  stage them. Revisit only if a non-ignored junk-file class appears.)*

## Not version-controlled — the regression vector (2026-06-15)
Item 0 (the lock-release re-regression) happened **because** `~/stack/claw-build/bin/`
is not under git: a later hand-edit silently dropped a validated fix and only `.bak`
copies recorded it. Recommend a lightweight **local git repo at `~/stack/claw-build/`**
(VPS-only, not pushed to claw-study) so orchestrator edits are diffable and fixes can't
silently regress. This is the cheap precursor to the harness control-system redesign
(`docs/harness-design/2026-06-14-session-handoff.md`), which converts these prose rules
into code controls.
