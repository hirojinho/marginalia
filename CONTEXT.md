# Context — claw-study glossary

Canonical terms for the study-app domain. This is a glossary, not a spec: it
defines *what words mean*, not how anything is built.

## Knowledge Component

An **atomic unit of knowledge** — one idea, in the Zettelkasten sense: small
enough that you cannot remove anything without leaving the idea incomplete, and
nothing essential is missing. It lives **below the task**: a single study task
or Read typically exercises several Knowledge Components.

This is the unit that mastery, confidence, and retrieval are tracked *against*.

- **Not** a study task. (Earlier the code conflated the two — a task UUID was
  used as the component identifier. That treated a molecule as an atom and is
  being corrected.)
- Aligns with both the Zettelkasten *atomicity* principle (one idea per note)
  and the Knowledge-Learning-Instruction framework's definition of a knowledge
  component as a sub-task "unit of cognitive function or structure" inferred
  from performance across the set of tasks that share it (Koedinger, Corbett &
  Perfetti, 2012).
- Identifier in code: `knowledge_component_id` (spelled out — not `kc_id`).
- A Knowledge Component is a **content-bearing atomic note**: `title` (the one
  idea), `body` (the distilled idea), `provenance`. Not a bare handle. See
  [ADR 0007](docs/adr/0007-knowledge-component-as-atomic-note.md).
- The **body is authored by the learner, in their own words** — the agent never
  writes it. The agent elicits, stores verbatim, critiques, and manages.

## Course

A subject the learner studies — e.g. *CE-297 Safety*, *DDIA*, *DSA Interview*,
*Software Arch*, *Thesis*, plus *General* for the uncategorised. A small, fixed
set. The Course is the **primary axis along which Sessions are organised**: the
learner thinks "I'm doing DDIA now," not "open my most recent chat."

## Session

A single chat thread scoped to **one study task**. The learner starts a *new*
Session for each task and **seldom returns to an old one** — so a Session is
closer to a disposable work surface than to a durable document. Consequences
that follow from this definition (not implementation, just meaning):

- A Session belongs to exactly one Course (*General* when uncategorised).
- Because creation is the dominant act and resumption is rare, the Session list
  is an **archive**, not a navigator: its job is to let the rare lookup succeed,
  not to be lived in.
- A Session carries a short **title** naming its task. The title is the app's
  responsibility to produce, not friction the learner must absorb up front —
  consistent with the Authoring principle below.

## Authoring principle

**The application absorbs *management* friction; the learner keeps the
*cognitive* labor.** Distilling an idea into one atomic statement is the
learning and stays with the learner. Filing, scheduling, deduping, surfacing,
and linking Knowledge Components are the app's job. See
[ADR 0007](docs/adr/0007-knowledge-component-as-atomic-note.md).
