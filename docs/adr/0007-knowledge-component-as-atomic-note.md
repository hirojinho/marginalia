# 0007 — Model the Knowledge Component as a learner-authored atomic note

- **Status:** Accepted
- **Date:** 2026-05-27

## Context

Confidence and (soon) retrieval practice are tracked against a "knowledge
component." The first implementation (the `confidence_log` table, shipped
2026-05-26) used the **plan task UUID** as the component identifier. A study
task plainly covers several distinct ideas, so this treats a molecule as an
atom — too coarse a unit to schedule retrieval against or to reason about
mastery of.

Two traditions define the right granularity the same way. The Zettelkasten
*atomicity* principle (Luhmann; systematised by Ahrens, *How to Take Smart
Notes*, 2017): one idea per note — small enough that nothing can be removed
without breaking the idea. The Knowledge-Learning-Instruction framework
(Koedinger, Corbett & Perfetti, 2012): a knowledge component is a *sub-task*
unit of cognitive function, inferred from performance across the set of tasks
that share it. The atom lives below the task.

A second, sharper question arose: who authors the note's content? The evidence
is one-sided. Ahrens: *"The attempt to rephrase an argument in our own words
confronts us without mercy with all the gaps in our understanding"* —
rephrasing **is** the comprehension test. Recent AI-and-learning research
(2024–2025) shows that offloading this generative/evaluative work to an LLM
produces "metacognitive laziness": better short-term output, but weaker
retention and transfer (the performance paradox), with harm concentrated at the
moment of deep encoding.

## Decision

A **Knowledge Component is a content-bearing atomic note** — `id`, `title` (the
one idea), `body` (the distilled idea), and `provenance` (the task / Read /
session that spawned it) — not a bare tracking handle, and not a plan task.

The **body is authored by the learner, in the learner's own words. The agent
never writes it.** The agent's role is to elicit the articulation, store the
learner's text verbatim, critique it against the source (flag gaps, flag
non-atomic notes), and manage the components (dedup, schedule, surface, link).

Governing principle: **the application absorbs *management* friction; the
learner keeps the *cognitive* labor.** Distilling an idea into one atomic
statement is the learning and stays with the learner; filing, scheduling,
deduping, surfacing, and linking are the app's job.

## Consequences

- **Easier:** retrieval practice has a concrete referent to grade recall
  against (the body); mastery accrues per-idea instead of per-task; atomicity is
  checkable because there is a body to check; the model is faithful to both the
  Zettelkasten and KLI definitions.
- **Easier to grow:** the link graph (`knowledge_component_links`) and a visual
  inspection UI are both **additive** — they reference component ids and read
  existing rows; neither forces a migration of the components table. The atom is
  the seed of an eventual knowledge base.
- **Harder / accepted cost:** capture adds friction to the *learning* flow — the
  learner must type the note. We accept this deliberately as a desirable
  difficulty; it is the generative act we are protecting, not a UX defect.
- **Accepted:** `confidence_log` (one day old, ~empty) must migrate off the
  task-UUID identifier onto `knowledge_component_id`. Cheapest now.
- **Deferred, not designed-out:** identity drift (same idea authored twice) is
  controlled later by a search-before-create lookup; the link graph and visual
  UI are later, separate tickets. None of these blocks the first version.
- **Risk:** the agent must judge *when* an atom-worthy moment occurred and coach
  atomicity without authoring — a prompt-design problem, not a schema problem.
