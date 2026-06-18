# 0020 — The session opener: one understanding-first move, generative, conversationally scored

- **Status:** Accepted
- **Date:** 2026-06-13

## Context

Review of real sessions (a cs101 session #62, a Biology session #63) plus the learner's direct
feedback showed the session *opener* had become a wall. Three evidence-based
moves stacked before any reading: session-open retrieval (Rule 6), pre-testing
(Rule 6 / the pretesting rule), and the pre-read prediction (Rule 7). Session #62
opened with ~12 question-exchanges (a 3-approach recall, **7 rounds** of
recursion cueing, 3 pre-testing questions, a prediction) before a word was
read.

Two felt problems, both confirmed against the transcripts and against a
standing preference ([feedback] "conversational, not exam-style"):

1. **Too many stacked questioning phases** — a wall, not a warm-up.
2. **It felt like memorization, not learning** — the questions probed *recall of
   facts* ("list the three approaches") and *recall of the source's specific
   examples* ("what's the canonical write-skew example?"), which is
   instance-memorization, not understanding.

[ADR 0019](0019-atom-is-the-spaced-unit-formative-recall-and-atomicity-gate.md)
already removed part of the burden (in-session recall is now formative;
session-open quizzes only *due atoms*). This ADR addresses the remaining
opener friction without losing the pedagogical gains (retrieval practice,
spacing, the generation effect).

## Decision

The session opener is **one move, chosen by situation**, and its questions are
**understanding-first and generative**, scored **conversationally and
silently**:

1. **One opener move.** A genuinely due atom → a light retrieval of it.
   Otherwise (a fresh 🔴 Read, the normal case) → the **pre-read prediction**.
   Pre-testing is **folded into the prediction** (a prediction *is* a pretest —
   same generation effect, Kornell et al. 2009). The three stacked phases
   collapse to one.

2. **Understanding-first questions.** Scoped *why / how / when / what-breaks*
   questions, answerable only by retrieving-and-reasoning — never "list the N
   things." Retrieval is preserved (the answer is unrecoverable without
   recalling the substance) but the *experience* is reasoning, not reciting.
   Rides the Bloom level (Rule 5) and composes with elaborative interrogation
   (Rule 13).

3. **Generate, don't recite examples.** Never quiz the source's specific
   example; ask the learner to produce *their own* example/application, or to
   reason about the principle. (Generation effect; transfer over instance
   recall.)

4. **Conversational, silently scored.** Credit the idea in the learner's own
   words (paraphrase = full credit; never penalize wording). Score the *gist*
   silently for the SM-2 schedule — no announced number, no enumerated
   miss-list, and **at most one cue** if the learner stalls (the 7-round
   drilling is banned at the opener).

5. **The probe loop (R9) inherits this.** The practice-testing probe generator
   produces understanding-first, generate-your-own questions and gives
   conversational feedback rather than a stated 0–5 grade.

## Consequences

- Touches prompt rules only (`agent/sandbox.go` AGENTS.md template + the R9
  spec): Rule 3 (gist/silent/conversational), Rule 6 (one situational move),
  Rule 7 + the pretesting rule (merged), Rule 11 (cueing capped at the opener).
- **Accepted trade-off:** less exhaustive *coverage-checking* at the opener
  (did you retain every piece) in exchange for flow and deeper engagement on the
  load-bearing idea. Coverage is recovered over time by spaced resurfacing, not
  by front-loaded enumeration.
- The highest-evidence move (retrieval) is **not** lost — it is reframed as
  retrieval-through-reasoning, which is at least as durable and feels like
  learning.
- If the learner later wants coverage-checking back for exam prep, the
  exam-style framing setting can carry it without reverting the default.

## References

- [ADR 0019](0019-atom-is-the-spaced-unit-formative-recall-and-atomicity-gate.md) — atoms, formative recall
- [ADR 0009](0009-session-single-task-spaced-unit.md), [ADR 0012](0012-segmented-active-reading.md)
- Roediger & Karpicke 2006 (testing effect); Kornell/Richland/Kao 2009 (pretesting + generation); Chi 1989, Pressley 1987 (self-explanation / elaborative interrogation); Slamecka & Graf 1978 (generation effect)
