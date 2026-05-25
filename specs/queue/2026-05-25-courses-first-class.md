---
id: 2026-05-25-courses-first-class
title: Promote courses from compile-time constant to SQLite-backed entity with create_course tool
max_wall_clock_minutes: 60
max_diff_lines: 400
max_retries: 1
max_tokens: 200000
requires_visual_approval: false
allow_web_search: false
created_at: 2026-05-25
created_by: laptop-cc + eduardo
---

## Goal

The chat agent has no way to create new courses today. `agent.KnownCourses` is a hardcoded slice of 5 entries; the only way to add one is editing the Go source, rebuilding, and deploying. This was hit live today (2026-05-25) when Eduardo asked the agent for a guitar study plan — the agent wrote `data/plans/guitar.json` correctly, but the course never appeared in the UI drawer because the drawer iterates the compile-time list. We hand-fixed it (commit `ebfef07`); this ticket makes the fix structural.

**The change:** move course definitions from a Go slice to a SQLite `courses` table, seed it once from the current `KnownCourses` on first start (idempotent migration), expose a `create_course` agent tool that inserts a row, and a `POST /api/courses` HTTP endpoint that the verifier can call deterministically. Frontend stays unchanged — the UI drawer keeps calling `GET /api/plan`, which now reads from SQLite.

## References

- `agent/tools.go:23-33` — current `KnownCourses` slice (6 entries after today's hotfix). Seed values for the migration.
- `agent/tools.go:35-43` — `CourseName(id)` linear-scan over `KnownCourses`. Must switch to DB lookup (with an in-memory cache to avoid one query per chat turn).
- `agent/db.go` — existing schema/migration site. Add `courses` table here (CREATE TABLE IF NOT EXISTS pattern; see lines 79-130 for the established style — sessions, messages, agent_memory, events).
- `agent/db.go:104-149` — pattern for ALTER-based migrations (messages.reasoning was added that way). The seed-on-empty pattern fits naturally beside `InitDB`.
- `handler/plan.go:32-44` — list-courses endpoint. Currently iterates `agent.KnownCourses`; will iterate the new DB result instead.
- `handler/handler.go:42-64` — route registration block. Add `mux.HandleFunc("/api/courses", h.handleCourses)` near `/api/plan`.
- `agent/tools.go:GetTools()` — agent tool registry (sketch only — go look at how other tools are registered, e.g., `update_plan` via `ToolUpdatePlan`).
- `agent/types.go` — add a `Course` struct mirroring the new DB shape: `{ID string \`json:"id"\`; Name string \`json:"name"\`; CreatedAt int64 \`json:"created_at"\`}`.

## Implementation plan

1. **Add the `courses` table** in `agent/db.go` schema:
   ```sql
   CREATE TABLE IF NOT EXISTS courses (
       id          TEXT PRIMARY KEY,
       name        TEXT NOT NULL,
       created_at  INTEGER NOT NULL
   );
   ```
   Place it in the same migration block as `sessions` (around line 93).

2. **Seed on empty.** Inside `InitDB` (or wherever the schema is initialized), after `CREATE TABLE IF NOT EXISTS courses ...`, run a single check:
   ```go
   var n int
   _ = a.DB.QueryRow("SELECT COUNT(*) FROM courses").Scan(&n)
   if n == 0 {
       for _, c := range KnownCourses {
           _, _ = a.DB.Exec("INSERT INTO courses(id, name, created_at) VALUES (?, ?, ?)",
               c.ID, c.Name, time.Now().UnixMilli())
       }
   }
   ```
   This runs once on first deploy after the migration lands. Subsequent restarts see `n > 0` and skip seeding.

3. **Add DB methods** in `agent/db.go`:
   - `func (a *App) ListCourses() ([]Course, error)` — `SELECT id, name, created_at FROM courses ORDER BY created_at ASC`
   - `func (a *App) GetCourse(id string) (Course, error)` — single-row lookup; return empty `Course{}` and nil error if not found (mirror `CourseName` semantics)
   - `func (a *App) CreateCourse(id, name string) error` — INSERT; return error wrapping unique-constraint violation as a friendly "course already exists"

4. **Replace `CourseName(id string) string`** in `agent/tools.go`. It currently takes no `*App` receiver — keep the package-level helper but make it a thin wrapper:
   ```go
   // App-coupled (preferred for new callers):
   func (a *App) CourseName(id string) string {
       c, _ := a.GetCourse(id)
       return c.Name
   }
   ```
   The existing free-function `CourseName(id)` at lines 35-43 is called from `agent/db.go:689,706` and `agent/tools_skill.go:31`. To avoid a 50-call ripple, keep the free function as-is but **back it with `KnownCourses` only** — document it as "compile-time fallback, prefer App.CourseName." Don't try to make the free function read DB; it has no DB handle.

   Even better, since `db.go:689,706` already operate on `*App`, switch those two call sites to `a.CourseName()`. `tools_skill.go:31` runs on `*App` too. Three callers, all migratable. Then keep the free function for tests/seeders only.

5. **Update `handler/plan.go:handlePlan`** to read from DB instead of `KnownCourses`:
   ```go
   courses, err := h.App.ListCourses()
   if err != nil { writeServerError(w, "list courses", err); return }
   summaries := make([]agent.PlanSummary, 0, len(courses))
   for _, c := range courses {
       p := h.App.LoadPlan(c.ID)
       done, total := agent.CountTasks(p)
       summaries = append(summaries, agent.PlanSummary{ID: c.ID, Name: c.Name, Done: done, Total: total, HasPlan: p != nil})
   }
   writeJSON(w, http.StatusOK, summaries)
   ```

6. **Add `POST /api/courses` handler** in a new file `handler/courses.go`:
   - `POST` only — `methodNotAllowed` for everything else
   - Decode JSON body: `{"id": "<kebab>", "name": "<display name>"}`
   - Validate: `id` must match `^[a-z0-9-]+$` (kebab-case, no spaces); both fields required
   - Call `h.App.CreateCourse(id, name)`; on unique-constraint, return 409; on other error, 500
   - On success: 201 with the created `Course` JSON
   - Register the route: `mux.HandleFunc("/api/courses", h.handleCourses)` in `handler/handler.go`

7. **Add `create_course` agent tool** in `agent/tools.go` (and dispatch in `agent/tools_dispatch.go` or wherever tools are dispatched — grep for `update_plan` to find the right file). Tool args: `{course_id: string, course_name: string}`. Implementation calls `a.CreateCourse(courseID, courseName)` and returns either a success string ("Created course X — visible in drawer now") or an error string. **Do not auto-rebuild the binary** — the new course is queryable immediately because we read from DB.

8. **Tests.** Add a unit test in `handler/plan_test.go` (or a new `handler/courses_test.go`) that:
   - Calls `POST /api/courses` with valid JSON, expects 201 + correct response body
   - Calls `GET /api/plan` and confirms the new course appears in the summaries
   - Calls `POST /api/courses` again with the same id, expects 409
   - Calls `POST /api/courses` with invalid id ("Has Space"), expects 400

   The `newTestHandler(t)` helper at `handler/testutil_test.go` already wires an in-memory DB — the migration will run during InitDB and seed the table from `KnownCourses`, so existing tests are unaffected.

9. **Run `go test ./...` locally before declaring done.** All ~70 tests must pass (including the new ones). The `len(agent.KnownCourses)` test at `handler/plan_http_test.go:41` still works because the seed makes the DB count equal to the slice count on a fresh DB.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# Generate a unique course id so reruns don't collide
COURSE_ID="verifier-c$(date +%s)"
COURSE_NAME="Verifier course (do not delete)"

# Try to create the course
http_code=$(curl -s -o /tmp/create.out -w "%{http_code}" \
  -X POST \
  -H "Authorization: Bearer $STAGING_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"$COURSE_ID\",\"name\":\"$COURSE_NAME\"}" \
  "$STAGING_URL/api/courses")

echo "POST /api/courses → $http_code"
cat /tmp/create.out
echo

if [ "$http_code" != "201" ]; then
  echo "FAIL: expected 201, got $http_code"
  exit 1
fi

# Confirm it shows in the course list
plan_list=$(curl -sf -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/api/plan")
found=$(echo "$plan_list" | python3 -c "
import sys, json
data = json.loads(sys.stdin.read())
for c in data:
    if c.get('id') == '$COURSE_ID' and c.get('name') == '$COURSE_NAME':
        print('found')
        break
")

if [ "$found" = "found" ]; then
  echo "OK: course $COURSE_ID created and visible in /api/plan"
  exit 0
else
  echo "FAIL: course $COURSE_ID created (HTTP 201) but not visible in /api/plan"
  exit 1
fi
```

### Post-acceptance (must PASS after Pi's implementation)

**Same script as above.** Pre-baseline runs against current main where `/api/courses` doesn't exist (404 ≠ 201, fails); post-acceptance runs against the new binary on staging (201 + visible, passes).

### Human-eyeball notes (NOT part of the gate)

- After deploy, ask the chat agent to create a new course (e.g., "create a course called 'Linear Algebra' with id 'linalg'"). It should call the `create_course` tool, the row should land in SQLite, and the UI drawer should immediately show "Linear Algebra" without restart.
- The 6 currently-known courses (ce297, ddia, dsa-interview, software-arch, thesis, guitar) should still be visible — the seed-on-empty migration preserves them.

## Done criteria

- [ ] `courses` table exists in SQLite with id/name/created_at columns
- [ ] Seed-on-empty migration runs idempotently from `KnownCourses`
- [ ] `App.ListCourses`, `App.GetCourse`, `App.CreateCourse` implemented
- [ ] `POST /api/courses` returns 201 + body on success, 409 on duplicate, 400 on invalid id, 500 on server error
- [ ] `GET /api/plan` reads from DB, not `KnownCourses`
- [ ] `create_course` agent tool registered and dispatched
- [ ] Three new tests: create+list, duplicate→409, invalid-id→400
- [ ] All existing tests still pass
- [ ] Diff under 400 lines
- [ ] After deploy, all 6 existing courses still appear in the drawer
- [ ] After deploy, creating a course via `POST /api/courses` makes it appear in `/api/plan` immediately

## Rollback notes

The `courses` table will persist in SQLite after this ticket lands. A `git revert` removes the code that *reads from / writes to* the table, but the table itself stays (harmless — it just becomes unused). `KnownCourses` is restored as the live source, and any course created via the new endpoint after this ticket ships **will disappear from the UI on rollback** (the slice is the only thing the reverted code reads). This is acceptable — rollback drill protects prod, not user data created with a not-yet-trusted feature.

No data migration in the destructive sense — only an additive one.
