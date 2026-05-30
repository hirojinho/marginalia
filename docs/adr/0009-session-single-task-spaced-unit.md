# 0009 — Study session is a single-task spaced unit; the tutor stops rather than chains

- **Status:** Accepted
- **Date:** 2026-05-29

## Context

Usage data across ~25 real sessions showed a Session does not map to a task: it
accretes several. Session 35 covered tasks #67→#68→#69; the 69-minute session 34
ran one task of studying and then half an hour of system-configuration. The
driver is `skills/study-step-complete/SKILL.md` **Step 5 ("Recommend next
step")** — on completion the tutor immediately advances to the next task, so the
learner is pulled forward instead of stopping.

The learner's stated goal is the opposite: small, daily, *finishable* sessions
that slot into a day with low activation cost. The learning-science evidence
agrees — distributed practice (spacing) produces more durable retention than
massed sessions (Cepeda et al. 2006, 2008), and it is one of the two
highest-utility techniques in the Dunlosky et al. (2013) review.

See the *Session* and *Studying vs Steering* entries in
[CONTEXT.md](../../CONTEXT.md).

## Decision

A study Session defaults to **one plan task**. On completing a task the tutor
marks a **stopping point** and invites the learner to resume on a later day
("good stopping point — come back tomorrow and we'll open with a recall check on
this"). It does **not** auto-recommend or advance to the next task. Continuing
is **opt-in** (the learner explicitly says "keep going").

Because work resumes on a later day, the session **opener** (free-recall, Rule
6) becomes a spaced retrieval across days rather than a same-sitting check. See
[ADR-adjacent] the lighter interleaving choice recorded in the plan (the opener
substitutes an older task occasionally; prompt-only, no scheduling DB yet).

## Consequences

- Sessions become small and daily; Bloom's higher levels (analyze/evaluate/
  create) distribute across sessions instead of being crammed into one sitting.
- Momentum/flow on a productive day is sacrificed — mitigated by the opt-in
  "keep going."
- Implementation: rewrite Step 5 of `study-step-complete` from "recommend next
  step" to "mark a stopping point + offer opt-in continue." A UI stop/continue
  affordance (the agent emitting a structured "stopping point" signal the
  frontend renders) is a later enhancement; the prompt change alone delivers the
  behavior.
- This sharpens, but does not contradict, [ADR 0008](0008-sidebar-course-first-launcher.md):
  the sidebar already assumes per-task sessions created fresh and seldom resumed;
  this ADR makes the tutor stop reinforcing the long-session habit.
- If the learner's usage shifts toward sustained multi-task flow being more
  valuable than spacing, revisit — the default would flip back to chaining.
