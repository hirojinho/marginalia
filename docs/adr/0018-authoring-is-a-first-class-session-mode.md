# 0018 — Authoring is a first-class session mode with its own conversational surface

- **Status:** Accepted
- **Date:** 2026-05-30
- **Relates to:** [0009](0009-session-single-task-spaced-unit.md), [0011](0011-plan-is-navigation-spine.md), [0014](0014-phase3-task-anchored-sessions-data-model.md), [0017](0017-agent-may-restructure-plan-via-deterministic-rewrite.md)

## Context

`CONTEXT.md` defines three activities — **Studying**, **Authoring**, **Steering** — and says
"Authoring is its own conversational surface, distinct from a task-anchored Session and from
the Steering UI," and "building a course/plan is Authoring, not Scratch." But that surface was
never built. In the UI today there is **no way to start any task-less chat at all**: you can
only open a task's Session (Studying) or click an *existing* Scratch chat. So course creation —
a long, generative, grill-friendly conversation — had no home. The learner would have to
shoehorn it into a task's reading workspace (wrong) or let the agent create a course blind via
the CLI (the `critical-theory` course was in fact born outside any proper surface).

Two existing pieces are the deterministic *writes* Authoring needs but are not a *surface*:
`claw-cli course create` (ADR-less, shipped 2026-05-30) and `claw-cli plan rewrite`
([ADR 0017](0017-agent-may-restructure-plan-via-deterministic-rewrite.md)).

## Decision

Make Authoring a **first-class session mode**, not a flavored Scratch chat. A session carries a
`mode` ∈ {`study`, `scratch`, `authoring`}:

- `study` — task-anchored (the existing Session; `task_id` set).
- `scratch` — task-less ad-hoc studying.
- `authoring` — task-less *generative design* of a course/plan.

`mode` is the discriminator that the glossary's "Authoring ≠ Scratch" demands but that
`task_id`-alone cannot express (both Scratch and Authoring are task-less).

The surface is reached by **opening a new Authoring chat** (a "+ new course" entry), which
creates a task-less `authoring` session. The agent, seeing `mode=authoring` in its generated
AGENTS.md, adopts an **Authoring frame** (use the `course-study-path` skill to generatively
build the course + plan) instead of the Studying frame, and persists results through the
existing deterministic writes (`claw-cli course create`, `claw-cli plan rewrite`) — never by
narrating structure into a file (the ADR 0016/0017 invariant).

**A new-course Authoring chat re-tags to the course it produces.** It starts course-less
(`course_id=""`); when the tutor runs `claw-cli course create --session <id>`, that same write
sets the session's `course_id` to the new course. The design conversation thus becomes a
durable record under the course it built, rather than an orphan.

Delivered in two phases: **A** = the foundation (mode + migration, start-a-task-less-session,
Authoring prompt frame, "+ new course" entry, re-tag) which ships a working new-course surface;
**B** = the existing-course "design/extend a plan" entry and the dedicated rail "Design"
section.

## Consequences

- The three glossary activities now have three homes: Studying = task Session, ad-hoc = Scratch,
  generative design = Authoring — each distinguishable in code and UI by `mode`.
- Authoring reuses the deterministic writes from ADR 0016/0017; this ADR adds the *front door*,
  not new write paths. The conversation is generative; the writes stay deterministic.
- `writeAgentsMD` becomes mode-aware (an Authoring branch vs the Studying branch), and must
  handle the course-less case the Studying branch skips.
- The re-tag couples `course create` to the session: `claw-cli course create` gains an optional
  `--session` flag. Accepted as the deterministic, agent-driven mechanism (vs a fragile
  handler-side "guess which course was created this turn").
- Migration backfills existing rows (`task_id` set → `study`, else `scratch`); `authoring` is
  only ever set by the new flow, so the migration is behavior-preserving.
- Cost: a schema column + threading `mode` through the session load → sandbox → prompt path.
  Justified — it is the minimum honest model of a distinction the glossary already draws.
- Touches `claw-cli` (`course create --session`) and `study-app`, so deploys rebuild **both**
  binaries.
