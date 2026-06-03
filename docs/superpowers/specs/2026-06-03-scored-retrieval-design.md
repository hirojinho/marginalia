# S5 — Scored Retrieval Replaces Self-Rated Confidence (+ reliable session-open recall) — Design

**Date:** 2026-06-03
**Status:** Approved, pending implementation plan
**Follows:** the S1–S4 pedagogy-consolidation series. Prompt-only, like S4.

## Problem

Two related defects in the session-open / confidence mechanic:

1. **Self-rated confidence is the wrong signal.** Rule 3 asks "How confident are you?" and logs a free 0–1 number. Self-assessment is the least reliable signal in the learning literature — learners misjudge what they know (Kruger & Dunning 1999; Koriat 1997; Dunlosky et al. 2013 rate it *low utility*). The S2 mastery gate and the SM-2 scheduler both inherit that miscalibration.
2. **The session-open recall doesn't reliably fire.** Observed in DDIA session #57 (2026-06-03): the tutor ran `claw-cli retrieve due`, got "nothing due" (the queue is empty/future-dated under SM-2), and jumped straight to the Rule 7 prediction — skipping the fallback free-recall of the most recent completed task (#19), which *was* done. Rule 6's fallback is a buried subordinate clause sitting next to a "fresh start → skip to Rule 7" exception, so the model collapses "nothing due" into "skip."

## Goal

Replace the self-rated number with a **tutor-measured retrieval score**, and make the session-open scored recall reliably fire. Reuse all existing plumbing (`confidence_log`, `retrieval_queue`, the S2 gate, `claw-cli confidence log`) — only the *source/semantics* of the [0,1] value changes, from self-report to `(key idea-units recalled) ÷ (total)`.

## Design decisions (locked)

- **Signal = tutor-scored free recall** (option A; Karpicke & Blunt 2011), not self-report, not graded short-answer probes (R9, deferred), not delayed JOL.
- **Keep the storage name `confidence_log` / `claw-cli confidence log`.** Renaming the table/subcommand is churn for no behavioral gain. Every *learner-facing and rule-facing* word becomes "recall/retrieval score"; the internal storage name is accepted debt. (Logged `source` stays `tool_call` — provenance unchanged; only what the value *means* changes.)
- **Fold in the Rule 6 reliability fix** — same mechanic (the session-open recall is now the scoring moment).

## Design (all in `agent/sandbox.go` `studyTuningSections` + `study-step-complete` SKILL.md)

### 1. Rule 3 — score retrieval; never ask for a self-rating

Replace the current Rule 3 (the "How confident are you?" + "elicit an actual number" + log text) with intent:

> 3. **Score retrieval — never ask for a self-rating.** Do NOT ask "how confident are you?" or request a 0–1 number. After a recall or explain-back, score it yourself: read the source (`pdf extract`) to fix the handful of **key idea-units** for the task, count how many the learner actually produced, and log `value = produced ÷ total` (round to 2 dp) via:
> ```
> claw-cli confidence log --session <SESSION_ID> --kc <active task id> --value <0.0-1.0> --raw "<what they recalled, verbatim>"
> ```
> where `<SESSION_ID>` is in the Session section and `<active task id>` is the `id` from `claw-cli plan status`. Report hits/misses as a two-step reveal (Rule 11). A low score → return to the topic; do not advance. The value is a **measurement of retrieval**, not a feeling (Karpicke & Blunt 2011; self-rated confidence is miscalibrated — Dunlosky et al. 2013). If no active task is in context, skip the command.

### 2. Rule 6 — make the scored session-open recall mandatory; separate "nothing due" from "no completed task"

Rewrite so the two branches are distinct and the recall is non-skippable when a completed task exists:

> 6. **Session-open recall (MANDATORY).** Before answering anything else, run `claw-cli retrieve due`. **(a)** If items are due → run a scored retrieval round on the top 1–2 (recall in his own words → score per Rule 3). **(b)** If nothing is due BUT a task has been completed → you MUST still open with a scored free-recall of the **most recent completed task** (recall → score per Rule 3 → cue gaps per Rule 11) *before* the Rule 7 prediction. An empty queue is the normal case (SM-2 future-dates fresh items); it does NOT license skipping the recall. **(c)** ONLY if no task is completed at all (brand-new course / his very first task) do you skip to the Rule 7 pre-read prediction. Never invent or assume a completed task; never claim he has read/finished/recalled anything without evidence — if unsure whether he has started, ask. Non-negotiable; highest-evidence pedagogic move (Roediger & Karpicke 2006, testing effect).

(Preserve the `!settings.Interleaving` variant — same restructure, keeping its "stay on the most recent" clause.)

### 3. Rule 8 — fix the stale cross-reference

Rule 8 says "break it across turns with a **Rule-3 confidence check** in between." Change "confidence check" → "recall check" (Rule 3 is no longer a confidence prompt).

### 4. `study-step-complete` SKILL.md — Step 0 recall is scored

Step 0 already does free recall and (S4) the two-step reveal. Add: after the recall, the tutor **scores** it as `idea-units recalled ÷ total` and logs it via `claw-cli confidence log` (the same value the gate/scheduler consume) — never asks the learner to self-rate.

## Non-goals

- No DB/schema/Go-logic change; no rename of the table or subcommand.
- Not building graded short-answer probes (R9) or question generation.
- Not touching the S2 gate code — it already reads the logged value; it now gates on demonstrated retrieval automatically.

## Testing

`agent/sandbox_test.go` (presence tests, mirroring existing ones):
- Rule 3 no longer asks for a self-rating: assert `studyTuningSections("ddia")` does **NOT** contain `"How confident are you"` and **does** contain a distinctive scoring phrase (e.g. `"key idea-units"` or `"Score retrieval"`).
- Rule 6 reliability: assert it contains a phrase making the empty-queue recall mandatory (e.g. `"does NOT license skipping"` or `"most recent completed task"` together with `"MANDATORY"`).
- Keep the S1/S2/S4 assertions green (`claw-cli confidence log` still present; `mastery_threshold`; `cue — don't complete`). NOTE: the S1 test `TestRule3UsesClawCLIConfidenceLog` asserts the absence of `"call the log_confidence tool"` and presence of `claw-cli confidence log` — both still hold after this rewrite; verify it still passes (the command stays; only the surrounding self-rating text changes).

`go test ./... && go build .` green.

Manual acceptance: deploy; next DDIA session-open, confirm the tutor (a) opens with a recall of the last completed task even with an empty queue, (b) scores it against the section's key points and logs a value, and (c) never asks for a self-rated number. Verify a `confidence_log` row lands with a tutor-measured value.

## Deploy

Same as S4 — two artifacts: rebuild + deploy `study-app` (Rules compiled from `sandbox.go`; `AGENTS.md` regenerates next turn) AND `scp skills/study-step-complete/SKILL.md` to `SKILLS_DIR=/home/eduardo/stack/study-app/skills/study-step-complete/SKILL.md` (mounted file, not compiled in).
