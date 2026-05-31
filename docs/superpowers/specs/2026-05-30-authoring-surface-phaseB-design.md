# Authoring Surface — Phase B (existing-course design + rail Design section) — design

**Date:** 2026-05-30
**Status:** Approved (grill-with-docs; no new ADR — completes ADR 0018; ready for implementation plan)
**Phase:** B of 2. Phase A (new-course conversational surface) shipped + live 2026-05-30.

## Problem

Phase A shipped the new-course Authoring surface ("+ new course" → course-less `authoring`
session). Two gaps remain from ADR 0018's full scope:

1. **No existing-course Authoring entry.** You can create a course conversationally, but cannot
   open an Authoring chat to *design/extend an existing course's plan* ("plan chapter 8's tasks
   from this PDF" — the glossary's other Authoring example).
2. **Authoring sessions live in the Scratch bucket** (Phase A interim) — no dedicated home, and
   the `authoringFrameSection` is hardcoded for the new-course case (it tells the agent to
   *create* a course, wrong for an existing-course design chat).
3. **Study framing leaks into Authoring.** For an existing-course authoring session `course_id`
   is set, so the study-tuned AGENTS.md sections (pedagogy Rules 1–10, the plan toggle block,
   Steering) all fire — contradicting "this is a design session." Phase A *appended* the
   authoring frame; ADR 0018 §Design.4 intended it "in place of" the studying sections.

## Goal (Phase B)

A learner can open a per-course **"Design plan"** Authoring chat; the tutor designs/extends that
course's existing plan (not create a course); authoring sessions get a dedicated rail **Design**
section and a focused, study-framing-free AGENTS.md.

## Decisions (locked via grill)

- **One generic per-course "Design plan" entry** (not scoped per-phase) — the conversation
  scopes what to design. Opens an `authoring` session tagged to the selected course.
- **Rail "Design" section below the plan, above Scratch** — plan stays the navigation spine
  (ADR 0011); Design is the "how this plan was built/extended" group beneath it.
- **`authoringFrameSection` branches on `course`:** course set → extend the EXISTING plan
  (`plan show` → `plan rewrite`, keep `id`s, do NOT create a course); course empty → today's
  new-course frame.
- **Suppress study framing for `mode=authoring`:** skip pedagogy Rules 1–10, the study plan
  block, Steering framing + steerTool, and the generic "Creating a course" block; KEEP the
  session/skills/PDF sections + the (branched) Authoring frame. This delivers ADR 0018 §Design.4's
  "in place of" intent (recorded here, not a new ADR — it completes 0018).

## Design

### 1. Mode-aware AGENTS.md (`agent/sandbox.go` `writeAgentsMD`)

Gate the study-only sections behind `mode != "authoring"`:
- `planSection` (line 157) — gate add `&& mode != "authoring"` to its `if course != ""`.
- The contiguous block lines 178–235 (Steering settings resolution + `steeringFramingSection` +
  `steerTool` + `createCourse` + Rules 6/9/10 construction + `pedagogySection` append) — wrap in
  `if mode != "authoring" { … }`.
- KEEP ungated: `sessionSection`, `skillsSection`, `pdfSection` (designing from a PDF needs slide
  access), and the `authoringFrameSection` call.

`authoringFrameSection` gains a `course string` param (call site passes `course`):
- `mode != "authoring"` → nil (unchanged).
- `course == ""` → current new-course frame (create via `course create --session`, then seed via
  `plan rewrite`).
- `course != ""` → existing-course frame: "You're extending the plan for course `<course>`.
  Read it with `claw-cli plan show --course <course>`, edit the JSON, submit the whole plan with
  `claw-cli plan rewrite --course <course> --plan-file <tmp>`. Keep each task's `id` stable; new
  tasks get empty `id`. Do NOT create a course — it already exists. Confirm and ask Eduardo to
  review." Uses the `course-study-path` skill for the generative design.

### 2. Per-course "Design plan" entry + rail Design section (`static/rail.js`, `static/app.js`)

**Split authoring out of Scratch** in `loadRailData` (line 65 else-branch): add a
`designSessions` array; for a task-less session, if `s.mode === 'authoring'` push to
`designSessions` when `s.course_id === selectedCourse` (course-less authoring shows under General,
`'' === ''`); otherwise keep the existing Scratch rule (`!s.course_id || s.course_id ===
selectedCourse`). A re-tagged new-course chat now surfaces under its course's Design section.

**Render the Design bucket** in `renderOther` (line 169) as the FIRST bucket (above Scratch).
When `selectedCourse !== ''`, render a "Design" header with a **"+ design plan"** action
(`data-action="design-plan"`); list `designSessions` via `renderSessionLine`. (Plan stays primary;
Design sits between plan and Scratch because `renderOther()` is appended after the plan.)

**Wire the click** (`static/app.js` dispatcher, beside the Phase A `new-course` case): add
`case 'design-plan'` → `startDesignPlan()` (new fn in `rail.js`, exported): `POST /api/sessions`
with `{ course_id: selectedCourse, task_id: '', mode: 'authoring', topic: 'Design '+
courseMeta[selectedCourse].name+' plan' }` via `apiFetch`; on failure → `showErrorBanner`; on
success → `switchSession(session.id)` → `loadRail()` → focus the chat input. Mirrors the Phase A
`startNewCourseAuthoring` exactly, differing only in `course_id` (selected, not empty) and topic.

## Testing

- **Prompt (`agent/sandbox_test.go`):** authoring session WITH a course (`sm.Create(id,"","ce297","","authoring")`) → AGENTS.md contains `plan rewrite --course ce297` and "extend"-style wording, does NOT contain "course create --session", and does NOT contain the pedagogy marker (e.g. "Pedagogical Rules (MANDATORY)") nor the steerTool marker. Authoring session WITHOUT a course → contains "course create --session" (Phase A test stays green). A study session → still contains the pedagogy rules (gating didn't over-reach).
- **Frontend:** local preview + curl/CDP — `POST /api/sessions {course_id:"ce297", mode:"authoring", task_id:""}` returns `mode:authoring, course_id:ce297`; reading the rail JS confirms the Design bucket renders for a selected course with the "+ design plan" action and lists course-scoped authoring sessions.
- No new Go write logic (reuses CreateSession/the frame); the gating is conditional assembly.

## Deploy notes

- Touches `study-app` only (sandbox.go + static JS, both in the server binary) — **claw-cli
  unchanged this phase**, but per the established recipe rebuild + deploy `study-app` (rebuild
  claw-cli too only if convenient; not required).
- Live smoke: select a course → "+ design plan" opens a chat; confirm `/api/sessions` shows a
  `mode:authoring` session tagged to that course; confirm a NEW authoring session's AGENTS.md (via
  a quick study of the sandbox dir, or by trusting the unit test) lacks pedagogy rules. Clean up.
- Concurrency: `git fetch` + re-check `origin/main` before merge/push/deploy.

## References

- ADR 0018 — Authoring is a first-class session mode (this phase completes its §Design.4).
- ADR 0017/0016 — `plan rewrite` / deterministic writes (the writes Authoring drives).
- ADR 0011 — Plan is the navigation spine (why Design sits *below* the plan).
- Phase A spec/plan: `docs/superpowers/{specs,plans}/2026-05-30-authoring-surface-phaseA*`.
- Code: `agent/sandbox.go` `writeAgentsMD` (gate 157, 178–235) + `authoringFrameSection` (276);
  `static/rail.js` `loadRailData` (65) / `renderOther` (169); `static/app.js` dispatcher.
