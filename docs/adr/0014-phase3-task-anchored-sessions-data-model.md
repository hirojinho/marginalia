# 0014 — Phase 3 data model: task-anchored Sessions, Scratch, and clean-break migration

- **Status:** Accepted
- **Date:** 2026-05-30
- **Builds on:** [ADR 0011](0011-plan-is-navigation-spine.md) (Plan is the navigation spine; a Session is a Task's workspace). 0011 set the IA direction and explicitly deferred the data model ("requires anchoring Sessions to Tasks… migrating existing free-floating sessions"). This ADR decides that model.
- **Refines:** [ADR 0010](0010-steering-via-settings-ui.md). 0010 called "editing the plan" Steering bound for a settings UI; this work splits out **Authoring** (generative plan/course design) as a conversational activity that is *not* Steering. The Authoring/Steering line is generative-design vs. declarative-config — see CONTEXT.md.

## Context

ADR 0011 made the Plan the navigation spine and a Session a Task's workspace, but left open *how* a Session anchors to a Task, what a Scratch chat is in the data, and what happens to the ~37 existing free-floating sessions. Plan tasks live in JSON files (`data/plans/{courseID}.json`), each with a stable UUID `id` (preserved across `ToolRewritePlan` by title-match); they are **not** a database table. Sessions today carry `course_id` + an LLM-generated `topic`, no task link. Inspecting the live DB also surfaced that "ad-hoc" chats are really three different activities, and that exact-title migration matching is hopeless (session topics like `"Ch. 8 Slides: Safety Risk & Risk Matrices (#68)"` are not task titles).

See *Plan*, *Session*, *Scratch*, and *Studying / Authoring / Steering* in [CONTEXT.md](../../CONTEXT.md).

## Decision

1. **Session ↔ Task is 1:1, permanent.** One Task has exactly one Session and vice versa. A Task worked across several days (a long Read chunked over sittings) keeps appending to that same Session — there is no per-Task list of sittings.

2. **`sessions.task_id` is a soft `TEXT` reference** to the plan task's UUID. No foreign key — the referent is a JSON file, not a table. The rail resolves a task's Session by looking it up; the converse (does this task have work?) is "a Session exists for this `task_id`."

3. **Orphans are survivable, not impossible.** `ToolRewritePlan` regenerates a task's UUID when its title stops title-matching, which dangles the Session's `task_id`. A dangling Session is **not** lost: it renders in a **Detached** group within its Course's Scratch, recoverable by re-anchoring. (Hardening UUID preservation is logged in ROADMAP, not done here.)

4. **A Scratch chat is a Session with `task_id = NULL`** — no new entity. Scratch is **global-default**: its canonical contents are course-less (a paper in no plan, an interests review spanning all courses), so a Scratch chat may optionally carry a `course_id` but need not. Scratch is *Studying without a Task*, distinct from Authoring and Steering.

5. **Session rows are created lazily** — on the first message (or first reading action), not on click. So "a Session exists ⟺ real work happened on that task" holds, the rail renders untouched tasks as "not started" by finding no Session, and clicking-then-abandoning a task leaves no row.

6. **Migration is a clean break.** `task_id` populates for **new** sessions only — no retro-anchoring of history. All existing sessions become **archive**, shown in Scratch under a "Before the redesign" group (`course_id` real → that Course's Scratch; `course_id` NULL → global Scratch). Provable junk is **hidden, not deleted**: `course_id = 'verifier-stats'`, `topic LIKE 'phase5 smoke%'`, the `postship-smoke-*` course, and 0-message sessions.

7. **The rail shows only human Courses.** Synthetic/system courses (verifier output, smoke tests) are excluded via a denylist now; pipeline-created sessions should adopt a reserved `course_id` prefix the rail filters going forward.

8. **A Task's reading resource is learned, not declared.** The resource auto-opened on the right is the Session's `last_pdf_id`, set on first manual open. A never-opened task has no known resource. A *declared* task→PDF link is deferred to the Authoring flow (logged in ROADMAP).

## Considered alternatives

- **Tasks as a DB table with a real FK** (instead of a soft ref). Structurally kills orphaning, but is a multi-day migration touching the whole plan system, `claw-cli plan`, the UUID-preserving rewrite logic, and the overnight pipeline's plan reads. Rejected for Phase 3 as far out of scope for an IA rebuild; the JSON-as-source-of-truth decision is load-bearing elsewhere.
- **Best-effort migration matcher** (fuzzy/index-match old topics to tasks). Rejected: LLM-generated topics are not task titles, only ~6 of 37 sessions even hint at a task index, and a wrong match silently anchors work to the wrong task. The downside (mis-anchoring) is worse than the upside (auto-finding ~15 chats the learner navigates forward from anyway). ADR 0011 already accepted losing cross-course history.
- **Eager session creation** (click = create row). Rejected: regenerates the empty-0-message-session clutter this same migration is hiding.
- **Per-course-only Scratch.** Rejected: the real Scratch items are course-less and would have nowhere to live.

## Consequences

- A completed Task in the rail will *not* surface its pre-redesign chat (those are in Scratch's archive group); only post-redesign work is task-anchored. Accepted, per the clean-break rationale.
- The 1:1 + lazy rules make the rail's per-task state a pure function of (plan task `done`) + (does a Session exist for `task_id`) — no extra state to maintain.
- Reading auto-open is correct from the second visit to a task; the first visit needs a manual open. Acceptable; the declared-resource upgrade lives with Authoring.
- "Detached" and "Before the redesign" are Scratch sub-groups, so Scratch carries some non-ideal weight at launch; it drains as archived chats become irrelevant.
