# 0015 — Silent grounding: the tutor reads the resource but withholds it

- **Status:** Accepted
- **Date:** 2026-05-30

## Context

Two earlier decisions pull in opposite directions, and the prompt never
reconciled them:

- The **agent-PDF-access** work (spec `2026-05-29-agent-pdf-access-design.md`)
  told the tutor: *"Never reconstruct slide or document content from your own
  memory — read the actual pages"* via `claw-cli pdf extract`. This fixed a real
  hallucination (the tutor claiming "the slides aren't stored" and inventing
  Chapter 8 content).
- [ADR 0012](0012-segmented-active-reading.md) says a Read task is **the
  learner's** cognitive work: predict → read the chunk → boundary recall. The
  retrieval, the prediction, the reading are theirs to do.

With only the first instruction in the always-on prompt and no caveat, the
tutor did the literal thing it was licensed to do: it read the pages and poured
them into the chat. In CE-297 session #41 (Part 11, systematic failures) the
tutor, in a single turn, asked for a prediction, *fabricated a prediction the
learner never gave*, and revealed the answer (the 43% specification-error
figure); a turn later it dumped the entire pp.11–15 fail-safe/secure/stop
taxonomy unprompted — after the learner had explicitly said "I'll be reading it
now." The learner never read anything; the tutor read it for him and lectured.

Pre-exposing content this way is not a neutral convenience — it destroys the
mechanisms the reading flow is built on. A prediction or reading attempt made
*before* seeing the material is what produces the learning (pretesting effect,
Richland, Kornell & Kao 2009; Kornell, Hays & Bjork 2009; generation effect,
Slamecka & Graf 1978). Hand the answer over first and the difficulty that does
the work is gone (desirable difficulties, Bjork & Bjork 2011), leaving passive
exposure — among the lowest-utility study behaviors (Dunlosky et al. 2013).

## Decision

The tutor reads the resource for **silent grounding only**. Reading the actual
pages stays mandatory (the anti-hallucination reason it can read at all), but
the extracted text is an internal answer key, never something poured into the
chat. There are exactly **three legitimate purposes** for reading:

1. **Orient / verify position** — know what the chunk covers and where the
   learner is (via the `<reading_state>` block).
2. **Judge the answer** — assess the learner's recall or prediction against the
   actual text and surface the specific gap.
3. **Clarify the learner's questions** — when *they* ask, ground the answer in
   the real text instead of reconstructing from memory.

The single prohibition is **push**: the tutor must not reproduce, quote,
summarize, or paraphrase a chunk's content *before the learner has read it*.

The governing line is **pull vs. push**. Learner-initiated content (a question,
"explain this equation," "summarize this page") legitimately *pulls* grounded
text out — that is purpose #3 and is always allowed, since explicit user
instructions override. Agent-initiated content-dumping before the learner reads
is *push* — the leak. "Read it interactively / together" is **not** a pull
request for content; it means smaller chunks with more frequent recall (the
[ADR 0012](0012-segmented-active-reading.md) loop), not lecturing.

## Consequences

- The withhold boundary and the pull-vs-push rule move into the **always-on
  numbered rule block** (`agent/sandbox.go` `writeAgentsMD`, mirrored into the
  legacy `CLAUDE.local.md` and `agent/agent.go` `toolsAndRulesPrompt`). Session
  #41 showed the tutor reliably obeys the numbered rules but treats skill steps
  as advisory, so enforcement cannot live only in `resource-orientation`'s
  Step 4.
- The PDF section's "read the actual pages" instruction gains a withhold caveat
  pointing here, so the anti-hallucination license and the no-leak rule are read
  together.
- Rule 7 (pre-read prediction) is amended so the prediction and any reveal
  **cannot share a turn** — ask, then STOP; never fabricate the learner's
  prediction.
- Trade-off accepted: the tutor knows the answer and must sit on it, which can
  feel withholding. That discomfort is the desirable difficulty doing its job.
- This does not weaken anti-hallucination: the tutor still reads before it
  speaks. It changes *what it does with* what it read — judge and clarify, not
  recite.

## References

Full citations and links for the evidence cited above: [docs/references.md](../references.md)
(`richland-kornell-kao-2009`, `kornell-hays-bjork-2009`, `slamecka-graf-1978`,
`bjork-bjork-2011`, `dunlosky-2013`).
