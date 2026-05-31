# Authoring Surface — Phase A (conversational new-course creation) — design

**Date:** 2026-05-30
**Status:** Approved (grill-with-docs; ADR 0018 written; ready for implementation plan)
**Phase:** A of 2. Phase B (existing-course "design plan" entry + rail Design section) is a separate later spec.

## Problem

There is no conversational home for **Authoring** (generative course/plan design). The UI can
only open a task's Session (Studying) or an existing Scratch chat — there is no way to start a
task-less chat at all. So creating a course is either shoehorned into a task's reading workspace
or done blind by the agent via CLI. `CONTEXT.md` already says Authoring is "its own
conversational surface, distinct from a task-anchored Session and from the Steering UI" and
"building a course/plan is Authoring, not Scratch" — but it was never built. (ADR 0018.)

## Goal (Phase A)

A learner can click **"+ new course"**, land in a conversational Authoring session, and design a
new course + plan with the tutor; the tutor creates the course and seeds the plan through the
deterministic writes we already shipped; the chat re-binds to the course it produced.

## Decisions (locked via grill)

- **First-class Authoring**, not a flavored Scratch chat (ADR 0018). Distinguished by a session
  `mode` ∈ {`study`, `scratch`, `authoring`}.
- **Full Authoring surface** is the eventual scope (new + existing course), but **Phase A builds
  new-course only**, modeled generally (the `mode` flag, not a "new-course" special case) so the
  existing-course flow reuses it in Phase B.
- **New-course chat re-tags** to the course it creates (via `claw-cli course create --session`).
- Reuses `claw-cli course create` and `claw-cli plan rewrite` (already shipped) as the writes.

## Design

### 1. Data model — `sessions.mode`

Add `mode TEXT NOT NULL DEFAULT 'study'` to `sessions` via an idempotent `ALTER TABLE ... ADD
COLUMN` in `InitSchema` (the established pattern, `agent/db.go:183-189`). Add a one-time guarded
backfill (the `MigratePhase3Sessions` meta-flag pattern, `agent/db.go:215`) that sets
`mode='scratch'` where `task_id IS NULL` (task-anchored rows keep the `'study'` default).
`'authoring'` is only ever written by the new flow → migration is behavior-preserving. Add a
`Mode` field to the `Session` struct and include `mode` in the session `SELECT` column lists so
it is populated on load.

### 2. Session creation — start a task-less session with a mode

Extend `POST /api/sessions` (`handler/sessions.go` `createSession`) to accept an optional
`"mode"` in the body. Resolution: if `task_id` set → `CreateSessionForTask` (mode `study`);
else → `CreateSession` with the body's mode, defaulting to `scratch` when absent. Thread `mode`
into `CreateSession` (signature gains a `mode` param; update all callers) so the row is inserted
with the right mode.

### 3. Entry point — "+ new course"

A control at the top of the rail beside the course switcher (`static/rail.js`). On click:
`POST /api/sessions {course_id: "", task_id: null, mode: "authoring", topic: "Design a new
course"}` → set the returned session active (`/api/sessions/active` PUT) → clear the workspace →
focus the chat box. This is also the first frontend caller of `POST /api/sessions` for a
task-less session (the primitive existed, unused).

### 4. Agent behavior — Authoring frame

Thread the session's `mode` from the chat handler (which loads the session) → `SandboxManager.
Create` → `writeAgentsMD` (all three signatures gain `mode string`). In `writeAgentsMD`, when
`mode == "authoring"`, emit an **Authoring frame** in place of the Studying-oriented sections:

- Tell the agent it is in a course-design conversation with Eduardo.
- Use the `course-study-path` skill (already wired to create the course first) to grill the
  intent, research, and build the plan.
- Create the course with `claw-cli course create --id <kebab> --name "<…>" --session <sessionID>`
  (the `--session` re-tags this chat to the new course), then seed tasks with
  `claw-cli plan rewrite --course <id> --plan-file <tmp>`.
- Handle the course-less case (`course_id == ""`) the Studying frame skips (it gates plan/slide
  sections on `course != ""`).

The Studying frame (plan status/toggle/rewrite, slides, settings) is unchanged for
`mode != "authoring"`.

### 5. Re-tag — `claw-cli course create --session`

`claw-cli course create` gains an optional `--session <id>` flag. After a successful
`CreateCourse`, if `--session` is set, call a new `App.UpdateSessionCourse(sessionID, courseID)`
(`UPDATE sessions SET course_id=? WHERE id=?`) so the Authoring session re-binds to the new
course. Without `--session`, behavior is unchanged (course-creation feature stays intact).

### 6. Interim rail

Phase A keeps rail treatment minimal: the Authoring session is reachable (active immediately
after "+ new course"; appears under its course once re-tagged, via the existing course-scoped
session loading). The dedicated per-course **"Design"** rail section is **Phase B**. Until then
an authoring session shows wherever a course-tagged task-less session currently renders (an
acceptable interim, like the 3b-1 "Other chats" stub).

## Testing

- **DB:** `mode` column present after `InitSchema`; backfill sets task-less rows to `scratch`,
  task rows to `study`; migration is idempotent (running twice changes nothing). New
  `CreateSession(..., "authoring")` persists `mode='authoring'`. `UpdateSessionCourse` changes
  `course_id` and leaves other fields intact.
- **CLI:** `claw-cli course create --id X --name Y --session N` creates the course AND sets
  session N's `course_id` to X (assert via a session read); without `--session`, course created
  and no session touched (existing tests stay green).
- **Handler:** `POST /api/sessions {mode:"authoring", task_id:""}` returns a session with
  `mode=authoring`, `task_id` empty; `{task_id:"t1"}` ignores mode and is `study`.
- **Prompt:** `writeAgentsMD` with `mode="authoring"` emits the Authoring frame (contains
  `course create --session` and the course-study-path pointer) and omits the Studying plan
  toggle block; with `mode="study"` the output is unchanged from today.
- **Frontend:** manual/CDP smoke — "+ new course" opens a chat, a message sends to the new
  task-less session.

## Deploy notes

- Touches `claw-cli` (`course create --session`) AND `study-app` → **rebuild + deploy BOTH
  binaries.** Migration runs on boot (column add + guarded backfill).
- Live smoke: open "+ new course", confirm a task-less `authoring` session is created
  (`/api/sessions` shows `mode=authoring`); have the tutor create a throwaway course with
  `--session` and confirm the session re-tagged; clean up the throwaway course + session.
- Concurrency: `git fetch` + re-check `origin/main` before merge/push/deploy.

## References

- ADR 0018 — Authoring is a first-class session mode (this feature's decision record).
- ADR 0017 — `plan rewrite`; ADR 0016 — deterministic Steering write; ADR 0014 — task-anchored sessions.
- CONTEXT.md — Studying / Authoring / Steering; Scratch; Session.
- Patterns: column add `agent/db.go:183`; guarded migration `agent/db.go:215`
  (`MigratePhase3Sessions`); `CreateSession` `agent/db.go:311`; `writeAgentsMD` /
  `Create` `agent/sandbox.go:46,117`; course create `claw-cli/main.go` (`courseCreate`).
- Phase B (separate spec): existing-course "design/extend plan" entry + rail "Design" section.
