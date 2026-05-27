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

## Authoring principle

**The application absorbs *management* friction; the learner keeps the
*cognitive* labor.** Distilling an idea into one atomic statement is the
learning and stays with the learner. Filing, scheduling, deduping, surfacing,
and linking Knowledge Components are the app's job. See
[ADR 0007](docs/adr/0007-knowledge-component-as-atomic-note.md).
