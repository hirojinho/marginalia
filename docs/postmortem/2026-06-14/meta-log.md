## pm-smoke / preflight (2026-06-14)
- Asked: ssh to VPS, count result dirs, reply with only the number.
- Did Pi do it? Yes — ran `ssh nanoclaw ls`, enumerated all 26, replied "26".
- Off-rails / corrections: none. `-xt write,edit` held (no write attempted). Note: `timeout` absent on macOS (driver-side, not Pi). Cost not captured by extractor (usage event shape differs) — non-blocking.
- Lesson: read-only forensics invocation + ssh works first try; the 26 = 19 real tickets + 7 idle-* markers, so triage must separate idle heartbeats from real runs.

## pm-p1 (triage sweep) — HUNG, killed (2026-06-14)
- Asked: single Pi call to ssh-sweep all 19 tickets + cross-ref repo + emit triage table.
- Did Pi do it? NO. Pi child used ~1s CPU then slept 24 min in S state, zero network sockets — provider EventStream stalled and never closed (Pi print-mode hang #2381/#2119). No hung ssh child; ssh was not the cause.
- Off-rails / corrections: ran WITHOUT a timeout wrapper (macOS has no timeout(1)). Killed manually after 25 min. Dashboard "~no usage today" was correct — the stall never billed.
- Lessons: (1) NEVER drive local Pi in -p without a hard timeout+retry belt -> built raw/pirun.sh. (2) One Pi call doing ~19×N ssh round-trips in one long stream is the worst case for the hang; restructure to "Claude Code pre-stages a local evidence bundle, Pi reads+analyzes locally" so each Pi call is short and bounded. (3) extractor cost bug fixed (dedupe usage by responseId; streaming deltas report 0).

## pm-p1b (triage on local bundle) — SUCCESS (2026-06-14)
- Asked: read 66KB local bundle, emit triage table + discrepancies + primary failure.
- Did Pi do it? YES, no hang (pirun belt, rc=0, ~cents). Strong triage: caught 9 result/repo mismatches, the exit-10 cluster, orphaned r9, cost outliers; nominated r9 as primary hard failure.
- Off-rails / corrections: (a) used pi-output.jsonl SIZE as the "cost" column (no $ in result.json) instead of saying "not recorded"; (b) FALSE claim "remove-plan-drawer gate passed despite 5 post-FAILs" — result.json shows clean exit 0; (c) labeled sm2 exit "?"/wall 0 (real artifact: duration_seconds=0 on salvaged ticket).
- Lesson for Pi config/prompt: tell Pi to (1) distinguish "field absent" from a proxy, never invent a metric; (2) when flagging a gate anomaly, quote the gate_steps exit codes verbatim — the post-FAIL hallucination came from reading gate.log prose not the structured gate_steps. 128MB jsonl from thinking:high is wasteful for a bounded read — consider thinking:low for triage.

## pm-p2 (r9 deep-dive) — strong mechanics, WRONG root-cause layer (2026-06-14)
- Asked: root-cause the two go-test failures from a local diff bundle; classify layer; minimal fix; bounded?
- Did Pi do it? Mechanically excellent: pinpointed both failures, quoted exact lines, correctly inferred all 3 Rule-6 string regressions (I verified against the diff — all correct), good fix for the syntax error.
- Off-rails / corrections: (1) classified ROOT CAUSE as EXECUTOR — WRONG. Pi never read the r9 SPEC, so it assumed the Rule-6 change was an accidental regression. The spec ORDERED the rewrite verbatim → root cause is SPEC (spec↔ADR-0020 conflict). (2) "Minimal fix = restore old text" would delete r9's intended feature. (3) Pi missed that this is the drift genre.
- Lesson for Pi config/prompt: a forensics prompt that gives Pi the DIFF + GATE.LOG but not the SPEC structurally cannot tell "bug" from "spec-mandated change." The postmortem skill MUST require Pi to read the spec AND the ADRs it cites, and explicitly ask "is this regression spec-intended? does the spec conflict with an ADR/guard-test?" before classifying EXECUTOR vs SPEC. This is the single most important harvested rule.

## pm-p4 apply + Task 5 gate (2026-06-14)
- Pi apply: first call failed (--session-id + --continue conflict — invalid Pi flags); succeeded on retry with a fresh --session-id reading the proposal file. Faithful 3-edit apply, gofmt clean.
- Hidden defects: fixing the build UNMASKED 2 more failures (TestProbeShowByID NULL-scan, TestProbeRecord session_id=0 FK) that the gate.log had not listed — the claw-cli build error short-circuited its tests. LESSON: postmortem must RUN the full suite after a trivial build fix; never trust the gate.log failure list as complete when an early step aborts later ones.
- Pi fixed both (sql.NullString; nil session_id) — full suite green, vet clean.
- Gate: real gate-runner.sh on VPS PASSED exit 0 (pre-baseline fail-as-expected, build, go test, post-acceptance). Gated binary promoted. Drove the gate directly via ssh (orchestrator layer, no model) rather than through Pi — more reliable, no hang surface.
- r9 total defects: 1 SPEC (Rule6↔ADR-0020), 3 EXECUTOR (%q syntax, NULL scan, session FK). All bounded; none were the ADR-0019 foundation gap.

## Task 6 deploy + merge-base discovery (2026-06-14)
- BIG finding: gate-green is necessary but NOT sufficient — the gate validated r9's branch off a STALE base (57b886f); main had advanced on the same files (fd70cec pedagogy, acd56a9 claw-cli) at 17:02, after r9's 05:00 run. The pipeline's FF-only policy would have ABORTED. Salvage required merge + RE-GATE on the merged result before deploy.
- Live-dev racing the overnight queue is a systemic risk: r9 was authored/queued, then main moved the same day → any salvage/merge must re-gate.
- Executor pollution: r9's original run committed 5 .bak files (git add -A swept editor backups) — repo cruft now on main; hardening = .gitignore + skill rule "never git add -A; add named paths" or strip *.bak pre-commit.
- Deploy verified by health-probe + last-deployed.json (did NOT read prod AUTH_TOKEN — classifier correctly blocked a credential-read bundled with an unrequested main push).
