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
set. Each Course has one **Plan**. The learner thinks "I'm doing DDIA now," and
then navigates that Course's Plan.

## Plan

The ordered set of study tasks for a Course, grouped into phases (e.g.
*Chapter 8 → tasks 67, 68, 69*). The Plan is the **primary navigation spine**:
the learner moves through it task by task ("the next task from the last checked
one"), and progress is tracked against it. A Task is one item in the Plan.

The Plan — not a list of past chats — is what the learner navigates. This is the
hierarchy: **Course → Plan (phases → Tasks) → the work on a Task.**

## Session

The **workspace for a single Task**: the chat with the tutor about that Task,
plus its reading state and notes. A Session is *not* a top-level entity the
learner browses — it is reached **through its Task in the Plan**. Consequences
that follow from this definition (not implementation, just meaning):

- A Session is anchored to exactly one Task (and so to one Course). Opening a
  Task opens its Session; a fresh Task creates one on entry.
- There is **no flat session list** to navigate. Past work is reached by
  selecting its (completed) Task in the Plan. The old "archive" framing is
  retired — the Plan *is* the index of one's work.
- A Session is a **small, spaced unit**: ideally one Task, after which work
  *stops* and resumes on a later day, rather than many Tasks chained into one
  long sitting (distributed practice). The "small and daily" shape is the goal.
- A retrieval check that references an *earlier* Task at session-open still
  belongs to **today's** Session — it points back at the old Task, it does not
  reopen it.

## Scratch

A space for chats **not tied to any Task** — one-off questions, paper checks,
an interests review, exploratory threads. Scratch is where ad-hoc work lives so
the Plan stays a clean record of structured study. (A Scratch chat still earns a
short auto-title, since it has no Task name to inherit.)

## Studying vs Steering

Two different activities that have been tangled in the same chat surface:

- **Studying** — engaging with course content through the tutor: recall,
  prediction, explanation, reading, reflection. This is the learner's cognitive
  work and the thing a Session is for.
- **Steering** — adjusting *how the system behaves*: editing the plan, setting a
  Course's framing (e.g. "exam-prep first, conceptual exam"), changing tutor
  preferences (pace, chunk size). This is configuration, not learning.

These are distinct: Steering should not happen *inside* a Studying Session (it
bloats and pollutes it), and durable Steering changes are owned by the learner
directly, not produced as a side effect of a chat turn.

## Authoring principle

**The application absorbs *management* friction; the learner keeps the
*cognitive* labor.** Distilling an idea into one atomic statement is the
learning and stays with the learner. Filing, scheduling, deduping, surfacing,
and linking Knowledge Components are the app's job. See
[ADR 0007](docs/adr/0007-knowledge-component-as-atomic-note.md).
