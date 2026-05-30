# Conversational Course Creation — design

**Date:** 2026-05-30
**Status:** Approved (brainstorming complete; ready for implementation plan)

## Problem

Course creation is impossible in the live Pi (`/chat-v2`) path. The Go `create_course`
tool (`agent/tools.go`) is only wired into the legacy `/chat` tool loop, which Pi does not
use — Pi only has `claw-cli` + bash in its sandbox. There is no `claw-cli course create`, so
the live tutor cannot create a course. Existing courses are seeded from `KnownCourses`
(`agent/tools.go:23`) into the `courses` table on first boot (`InitSchema`).

`POST /api/courses` already exists (`handler/courses.go`) and works, but no UI calls it, and
that endpoint is irrelevant to the conversational path anyway.

Per ADR 0014 / CONTEXT.md, course creation is **Authoring = conversational**, not a form. So
the fix is a CLI command the Pi agent can call during the Authoring flow — **no new UI**.

## Goal

Let the Pi tutor create a course conversationally, with optional initial steering settings,
by composing existing validated DB functions. Close the gap where `course-study-path` assumed
courses already existed.

## Scope

In scope:
- New `claw-cli course create` subcommand.
- Two prose wiring edits so the agent discovers and uses it (`writeAgentsMD`, `course-study-path` skill).

Out of scope:
- Frontend "+ new course" UI (explicitly deferred; creation is conversational).
- Changes to the legacy `/chat` Go `create_course` tool (kept as-is for parity).
- Plan-file / memory scaffolding — that remains the `course-study-path` skill's job.

## Design

### 1. CLI command (`claw-cli/main.go`)

Add a `create` case to `runCourse()` dispatch (line 546), alongside `interests` and `settings`,
and update the usage string to `course <interests|settings|create>`:

```
claw-cli course create --id <kebab-case> --name <display name> [--framing <text>] [--exam-style <text>] [--db <path>]
```

New `courseCreate(args, stdout, stderr, dbPath)` function, modeled exactly on
`courseSettingsSet` (`claw-cli/main.go:643`):

1. Parse flags: `--id` (required), `--name` (required), `--framing` (optional),
   `--exam-style` (optional), `--db` (override).
2. Validate `--id` against `^[a-z0-9-]+$` (same rule as `POST /api/courses`); non-empty `--name`.
   On invalid input → message to stderr, exit 2.
3. `resolveDBPath` → `newAppFromEnv(resolvedDB, false)` → `defer app.Close()` (same as settings set).
4. `app.CreateCourse(id, name)`. On the existing "course already exists" error → stderr message,
   exit 1 (idempotency surfaced, not swallowed).
5. If `--framing` provided → `app.SetCourseSetting(id, "framing", value)`.
   If `--exam-style` provided → `app.SetCourseSetting(id, "exam_style", value)`.
   Reuses the **same** validated single-knob path as `course settings set` — no duplicate write logic.
6. stdout: `Created course '<name>' (id: <id>)`, plus one line per setting applied; exit 0.

**Boundary:** writes only the `courses` row + optional `course_settings`. Does not touch the
plan file or memory.

### 2. Agent discovery & Authoring wiring

**a. `agent/sandbox.go` → `writeAgentsMD`** — add a line to the existing course/CLI guidance:

> To create a new course, run `claw-cli course create --id <kebab> --name <…>` (optionally
> `--framing` / `--exam-style`). Do this when the user starts studying a subject that isn't
> already a course. Course IDs are permanent — pick a stable kebab-case slug.

**b. `skills/course-study-path/SKILL.md`** — currently assumes courses exist. Add an explicit
first step in **Plan mode** and **Self-study mode**: before `claw-cli memory save --course X`
or writing the plan, check whether the course exists; if not, run `claw-cli course create`
first. Closes the gap where the skill saved memory against a never-created course.

No change to the legacy `/chat` `create_course` Go tool.

## Testing

- **CLI test** (`claw-cli/main_test.go` or focused new test):
  - happy path: `course create --id … --name …` inserts the row (assert via `ListCourses`/`GetCourse`).
  - duplicate id → exit 1 + message.
  - invalid id (non-kebab) → exit 2.
  - `--framing` / `--exam-style` land in `course_settings` (assert by reading settings back).
- No new DB code: `CreateCourse`, `SetCourseSetting`, `UpsertCourseSettings` are already covered;
  this composes them.
- Prose edits (AGENTS.md / skill) verified at deploy via `strings` / disk grep.

## Deploy notes

- Touches `claw-cli` → **rebuild + deploy BOTH binaries** (`study-app` and `claw-cli`).
- `skills/course-study-path/SKILL.md` is disk-mounted on the VPS — **scp it separately**
  (binaries don't carry it). `writeAgentsMD` is in the `study-app` binary.
- Live smoke: `claw-cli course create --id test-xyz --name "Test"` → `GET /api/courses` shows it
  → `course settings get --course test-xyz` → clean up the test course.
- Concurrency: this repo has concurrent sessions committing to `main` — `git fetch` + re-check
  `origin/main` before merge/push/deploy.

## References

- ADR 0014 — task-anchored sessions / Studying-Authoring-Steering (`docs/adr/0014-…`).
- ADR 0016 — agent may write steering via deterministic tool (`docs/adr/0016-…`).
- CONTEXT.md glossary — Authoring vs Studying vs Steering.
- Pattern template: `courseSettingsSet` at `claw-cli/main.go:643`.
- Seed list: `KnownCourses` at `agent/tools.go:23`.
