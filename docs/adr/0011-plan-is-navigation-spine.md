# 0011 — The Plan is the navigation spine; a Session is a Task's workspace, not a navigable entity

- **Status:** Accepted
- **Date:** 2026-05-29
- **Supersedes:** the core thesis of [ADR 0008](0008-sidebar-course-first-launcher.md) (the session list as primary navigator)
- **Data model decided in:** [ADR 0014](0014-phase3-task-anchored-sessions-data-model.md) (this ADR defers it; see Consequences)

## Context

[ADR 0008](0008-sidebar-course-first-launcher.md) made the session list the
primary left-rail navigator — while itself describing that list as an *archive
the learner seldom revisits*. That is a contradiction: the most prominent
surface held the thing least navigated. Meanwhile the learner actually moves
through study by **plan task** ("the next task from the last checked one"), and
the Plan was demoted to a drawer. [ADR 0009](0009-session-single-task-spaced-unit.md)
(one task = one session) makes the session list matter even less and the Plan
even more.

See *Plan*, *Session*, and *Scratch* in [CONTEXT.md](../../CONTEXT.md).

## Decision

The **Plan** is the primary navigation spine. The information hierarchy is
**Course → Plan (phases → Tasks) → the work on a Task**, and the UI reflects it:

- **Left rail:** a course switcher → the selected Course's Plan (phases → tasks,
  with progress and the next task obvious). A **Scratch** area sits below for
  chats not tied to any task.
- **Center:** the **workspace** for the active task — the tutor chat.
- **Right:** reading, tied to the task's resource (see
  [ADR 0012](0012-segmented-active-reading.md)).

A **Session is a Task's workspace** (chat + reading state + notes), *not* a
top-level entity the learner browses. Opening a Task opens its Session; a fresh
Task creates one on entry. **There is no flat session list** — past work is
reached by selecting its completed Task in the Plan. Ad-hoc, non-task chats live
in **Scratch**.

## Consequences

- The "archive in the prime spot" tension is dissolved: the Plan *is* the index
  of one's work; old work is found through its task.
- The course-first **accordion** built for ADR 0008 is repurposed, not
  discarded — its tree mechanics become the Course→phase→task plan rail; the
  sidebar truncation fix stays; async session titling survives for **Scratch**
  chats (which have no task name to inherit).
- Requires anchoring Sessions to Tasks in the data model and migrating existing
  free-floating sessions (most already carry a task-derived topic) into the
  task-anchored model; non-task sessions become Scratch.
- One Course shown at a time keeps the rail short and the current task prominent;
  switching Course swaps the rail.
- Trade-off: a global cross-course "recent chats" view is lost — accepted,
  because resumption-by-recency was never how this learner navigates.
- If usage ever shifts to heavy cross-course recency browsing, revisit.
