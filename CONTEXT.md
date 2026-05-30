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

- A Session is anchored to exactly one Task (and so to one Course), and a Task
  has exactly one Session — the relationship is **1:1**. Opening a Task opens its
  (one) Session; a fresh Task creates it on entry. A Task worked across several
  days (e.g. a long Read chunked over sittings) keeps appending to that *same*
  Session — "resume next day" reopens it, it does not spawn a second Session.
  There is therefore no per-Task list of sittings to browse.
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

A space for chats **not tied to any Task** — one-off questions, a paper check
("help me understand this paper from the ITA masters call"), an interests review
("find a pattern across my saved interests"), exploratory threads. Scratch is
where ad-hoc *studying* lives so the Plan stays a clean record of structured
study. (A Scratch chat still earns a short auto-title, since it has no Task name
to inherit.)

Scratch is **global by default**: its canonical contents are course-*less* (a
paper in no plan; an interests review that spans every course), so they have no
Course to sit under. A Scratch chat *may* optionally be tagged to a Course (an
aside while studying it), but it is never tied to a Task. Scratch is **studying,
not Authoring or Steering** — building a course/plan is [Authoring](#studying-authoring-and-steering),
not Scratch.

## Studying, Authoring, and Steering

Three different activities that have been tangled in the same chat surface. They
differ by *what they touch* and *whether they are generative*:

- **Studying** — engaging with course *content* through the tutor: recall,
  prediction, explanation, reading, reflection. The learner's cognitive work and
  the thing a Session is for. (Course-less studying is [Scratch](#scratch).)
- **Authoring** — *generatively designing study structure* with the agent:
  creating a Course, building a Plan, or designing a new phase/chapter's tasks
  from an intent ("I'm reading Debord — build me a critical-theory course";
  "plan chapter 8's tasks from this PDF"). It is conversational **because it is
  generative** — it needs the agent's help to produce something that did not
  exist, and would be destroyed by a form. Authoring *produces* the Course/Plan
  that Studying then moves through.
- **Steering** — *mechanical, declarative configuration* of how the system
  behaves: toggling a task done, reordering, renaming, **splitting one task into
  several, adding or removing a task** (declarative restructuring the learner
  directs), setting a Course's framing ("exam-prep first, conceptual exam"), tutor
  preferences (pace, chunk size). Not generative; it belongs in a settings/management
  UI, owned by the learner directly, not produced as a side effect of a chat turn.

The line between Authoring and Steering is **generative design vs. declarative
config**, not create-vs-edit: designing a new chapter's tasks is Authoring even
though it edits an existing Plan; ticking a task done is Steering even though it
changes the Plan too.

Steering has a dedicated **Steering UI** (a settings form) and is also reachable
conversationally: the tutor may change a setting *anywhere*, including inside a
Studying Session, by making one deterministic write and resuming — what must not
happen is a Studying Session *accreting* into an open-ended config conversation.
The earlier "no Steering inside a Studying Session" rule was about that
accretion (and about the old unreliability of chat-driven config); a single
one-shot setting change is fine. Authoring is its own conversational surface,
distinct from both a task-anchored Session and from the Steering UI.

The same persistence rule covers **plan structure**, not just settings knobs: a
declarative restructure the learner directs (split this task, add/rename/reorder
one) is persisted by the tutor through a single deterministic write to the plan
store (`claw-cli plan rewrite`), in any surface, then confirmed and resumed —
never narrated raw into a file, never accreting. The *generative* design that may
precede it (the tutor working out a chunk map from a PDF) is Studying/Authoring;
the *write* that lands it is the Steering-style one-shot. See
[ADR 0017](docs/adr/0017-agent-may-restructure-plan-via-deterministic-rewrite.md).

## Authoring principle

**The application absorbs *management* friction; the learner keeps the
*cognitive* labor.** Distilling an idea into one atomic statement is the
learning and stays with the learner. Filing, scheduling, deduping, surfacing,
and linking Knowledge Components are the app's job. See
[ADR 0007](docs/adr/0007-knowledge-component-as-atomic-note.md).
