# 0012 — Segmented active reading with a position-aware tutor

- **Status:** Accepted
- **Date:** 2026-05-29

## Context

Reading happens **inside the app** and heavily — the learner is on page 72 of the
Chapter 8 risk reading and page 258 of 611 of DDIA. (An earlier read of the
metrics suggested the viewer was dead; that was an analytics artifact —
`pdf_open` only logs on *upload*, and `session.last_pdf_id` is never wired, so
real reading is invisible to both the metrics and the tutor.)

The current reading flow is good — pre-read prediction (generation effect,
Slamecka & Graf 1978) then post-read free recall (testing effect, Roediger &
Karpicke 2006). But it reads a whole task in one go before a single recall,
which piles up cognitive load on dense material and produces long sessions. The
evidence-backed upgrades are **segmenting** dense readings with a recall at each
boundary (load management, Sweller 1988) and **self-explanation** while reading
(Chi et al. 1989). Passive rereading/highlighting is low-utility (Dunlosky et
al. 2013) and should not be encouraged.

## Decision

A Read task runs as **segmented active reading**: the tutor breaks a long
reading into chunks and loops *predict → read chunk → boundary recall /
self-explain → next chunk*, ending with a full recall + confidence check. Short
readings stay whole.

The tutor is **position-aware**: a `<reading_state>` block (current PDF + page),
prepended to the turn like the existing `<plan_state>` block, tells the tutor
where the learner is. Segment boundaries are **manual + position-verified**: the
learner says they're done with a chunk; the tutor confirms against the actual
page before prompting the boundary recall. No auto-advance on page change (it
would misfire on skimming).

## Consequences

- Higher retention on dense material and naturally smaller sessions (each chunk
  is a small unit), reinforcing [ADR 0009](0009-session-single-task-spaced-unit.md).
- Requires plumbing that is **also** needed by [ADR 0011](0011-plan-is-navigation-spine.md)'s
  "reading tied to task": (1) fix `pdf_open` to log real opens/reads, not just
  uploads; (2) wire `session.last_pdf_id` + page; (3) emit the `<reading_state>`
  block into the prompt. This plumbing is the shared dependency between the
  reading-flow and the IA rebuild, so it lands once and serves both.
- Trade-off: more structure/interruption than free reading — accepted because it
  trades passive reading for active retrieval, the higher-utility behavior. The
  learner can still opt out per reading.
- Segment sizing is the tutor's judgment from reading length; if that proves
  clumsy, a future version could let the learner set chunk size in Steering
  settings ([ADR 0010](0010-steering-via-settings-ui.md)).
- The reading is **the learner's** cognitive work, so the tutor must withhold
  the chunk's content until they have read it — reading the pages itself for
  silent grounding only. See [ADR 0015](0015-silent-grounding-tutor-withholds-resource.md).

## References

Full citations and links for the evidence cited above: [docs/references.md](../references.md)
(`slamecka-graf-1978`, `roediger-karpicke-2006`, `sweller-1988`, `chi-1989`,
`dunlosky-2013`).
