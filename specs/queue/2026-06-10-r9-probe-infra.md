---
id: 2026-06-10-r9-probe-infra
title: R9 Practice Testing — retrieval_probe table, DB layer, and LLM grading function
max_wall_clock_minutes: 60
max_diff_lines: 250
max_retries: 1
max_tokens: 60000
requires_visual_approval: false
allow_web_search: false
---

## Goal

Lay the data and grading foundation for R9 Practice Testing — the highest-impact
remaining pedagogy item. A `retrieval_probe` table stores questions (generated from
Knowledge Components), learner answers, and SM-2 grades (0–5 from a deterministic
LLM prompt). A new `LogProbe` function writes the full probe record and feeds the
SM-2 scheduler. A lightweight `GradeProbeAnswer` function sends the KC body + learner
answer to a cheap LLM with a fixed SM-2 rubric prompt, returning a grade and
one-sentence justification.

This spec is additive — no existing tables, endpoints, or agent behavior change.
The next spec wires `claw-cli probe` subcommands on top of this.

## Implementation plan

### Step 1 — Add `retrieval_probe` table to schema (`agent/db.go`)

In `InitSchema`, after the `retrieval_queue` index (around line 235), add:

```sql
CREATE TABLE IF NOT EXISTS retrieval_probe (
  id                      INTEGER PRIMARY KEY AUTOINCREMENT,
  knowledge_component_id  TEXT NOT NULL REFERENCES knowledge_components(id),
  question                TEXT NOT NULL,
  expected_answer         TEXT NOT NULL,
  learner_answer          TEXT,
  grade                   INTEGER,
  graded_at               INTEGER,
  created_at              INTEGER NOT NULL,
  session_id              INTEGER REFERENCES sessions(id)
);
CREATE INDEX IF NOT EXISTS idx_retrieval_probe_kc ON retrieval_probe(knowledge_component_id, created_at);
```

Fields:
- `question` — cached question text; generated once per KC, reused across probes
- `expected_answer` — KC body at the time the question was generated (snapshot; the KC body may change later)
- `learner_answer` — NULL until the learner answers
- `grade` — SM-2 0–5; NULL until graded
- `graded_at` — unix millis; NULL until graded
- `session_id` — the session in which this probe was answered

### Step 2 — Add `GradeProbeAnswer` to `agent/llm.go`

New function that calls a lightweight LLM to grade an answer against the KC body:

```go
// GradeProbeAnswer prompts the LLM to grade a learner's answer against
// the expected answer on the SM-2 0–5 scale. Returns the grade and a
// one-sentence justification. Uses a separate short-timeout HTTP client.
func (c *LLMClient) GradeProbeAnswer(ctx context.Context, expectedAnswer, learnerAnswer string) (grade int, justification string, err error)
```

The function constructs a system prompt with the SM-2 rubric (same definitions as `ConfidenceToGrade`):

```
You are grading a learner's answer against the correct answer.
Return ONLY a JSON object with exactly two keys:
  "grade": integer 0–5
  "justification": one sentence explaining the grade

Grade scale:
  0 = complete blackout — nothing correct or relevant
  1 = wrong, but the learner would recognize the correct answer when shown it
  2 = wrong, but the correct answer seems easy to recall (on the tip of the tongue)
  3 = correct, but with serious difficulty or major gaps
  4 = correct, after hesitation or minor gaps
  5 = perfect, immediate recall — complete and precise

Correct answer: <expectedAnswer>

Learner answer: <learnerAnswer>
```

The user message is the learner's verbatim answer. The response is parsed as JSON:
`{"grade": <int>, "justification": "<string>"}`.

Validate: grade must be in 0–5. If JSON parse fails or grade is out of range, return an error.

Use a short timeout (15s) — grading prompts are small and should complete quickly.

Add a `GradeProbeAnswer_test.go` with a table-driven test that verifies the prompt
is constructed correctly, that grade validation works, and that JSON parse errors
are surfaced. **Do NOT call a real LLM** — only test the prompt construction and
response parsing by mocking or testing helper logic directly. If the existing
handler tests don't mock LLM calls, add a simple unit test for the grade validation
and JSON parsing logic extracted into a separate function.

### Step 3 — Add `LogProbe` and `GetProbeQuestion` to `agent/db.go`

**`LogProbe`** — stores a completed probe and updates the SM-2 schedule:

```go
// LogProbe stores a graded retrieval probe and updates the SM-2
// retrieval_queue. learnerAnswer and sessionID may be zero/empty on
// the first call (question generation only).
func (a *App) LogProbe(knowledgeComponentID, question, expectedAnswer, learnerAnswer string, grade int, sessionID int64) (probeID int64, err error)
```

Behavior:
1. If `learnerAnswer` is non-empty → INSERT a full probe row with `learner_answer`, `grade`, `graded_at` set to now, `session_id`.
2. Derive a float from grade for SM-2: `float64(grade) / 5.0`
3. Call `a.UpsertRetrievalItem(knowledgeComponentID, derivedFloat)` — same path as `LogConfidence`
4. Return the new probe's `id`

If the INSERT is a duplicate of the same KC + same question (question generation only, no answer yet), use `INSERT OR IGNORE` to avoid duplicates. For the graded probe (with answer), always INSERT — each probing is a new row.

**`GetProbeQuestion`** — returns the most recent question for a KC, or nil if none exists:

```go
// GetProbeQuestion returns the most recent cached question for a KC,
// or nil if no question has been generated yet.
func (a *App) GetProbeQuestion(knowledgeComponentID string) (probeID int64, question string, err error)
```

Query: `SELECT id, question FROM retrieval_probe WHERE knowledge_component_id = ? ORDER BY created_at DESC LIMIT 1`

### Step 4 — Add test coverage (`agent/db_test.go`)

Extend the existing `db_test.go` with tests for:

- **`TestLogProbe`** — creates a KC, logs a probe with grade 4, verifies:
  - Probe row exists with correct fields
  - `retrieval_queue` row was upserted with `last_confidence = 0.8` (4/5)
  - `session_id` is set correctly
- **`TestLogProbeNoAnswer`** — logs a probe with empty answer (question-only generation), verifies `learner_answer` and `grade` are NULL
- **`TestGetProbeQuestion`** — inserts two probes for the same KC, verifies it returns the most recent question
- **`TestGetProbeQuestionNone`** — no probes for a KC → returns nil

Use the existing test helpers (`newTestApp`, `seedTestCourse`, etc.) already in `db_test.go`.

### Step 5 — Verify build and no regressions

No changes to handlers, sandbox, static, or any existing agent behavior. The new
table and functions are additive and unused by any existing code path.

## Verification recipe

### Pre-baseline (must FAIL on current main)

The gate expects this script to exit non-zero on current main (features absent).
Exit 0 signals "feature already exists" → gate fails the run.

```bash
# Pre-baseline: features should NOT exist on current main.
# Exit 1 when features are absent (expected pre-state → gate proceeds).
# Exit 0 when any feature IS present (unexpected → gate fails, correct).

# 1. No retrieval_probe table on main.
if grep -q 'retrieval_probe' agent/db.go; then
  echo "UNEXPECTED: retrieval_probe already referenced in db.go"
  exit 0
fi

# 2. No GradeProbeAnswer function on main.
if grep -q 'GradeProbeAnswer' agent/llm.go; then
  echo "UNEXPECTED: GradeProbeAnswer already in llm.go"
  exit 0
fi

# 3. No LogProbe function on main.
if grep -q 'func.*LogProbe' agent/db.go; then
  echo "UNEXPECTED: LogProbe already in db.go"
  exit 0
fi

# 4. No GetProbeQuestion function on main.
if grep -q 'func.*GetProbeQuestion' agent/db.go; then
  echo "UNEXPECTED: GetProbeQuestion already in db.go"
  exit 0
fi

exit 1
```

### Post-acceptance (must PASS after implementation)

```bash
# 1. retrieval_probe table is referenced in schema.
grep -q 'CREATE TABLE IF NOT EXISTS retrieval_probe' agent/db.go && echo "PASS: table"

# 2. retrieval_probe index exists.
grep -q 'idx_retrieval_probe_kc' agent/db.go && echo "PASS: index"

# 3. GradeProbeAnswer exists and has correct signature.
grep -q 'func.*GradeProbeAnswer' agent/llm.go && echo "PASS: GradeProbeAnswer"

# 4. LogProbe exists and has correct signature.
grep -q 'func.*LogProbe' agent/db.go && echo "PASS: LogProbe"

# 5. GetProbeQuestion exists and has correct signature.
grep -q 'func.*GetProbeQuestion' agent/db.go && echo "PASS: GetProbeQuestion"

# 6. LogProbe calls UpsertRetrievalItem (SM-2 integration).
grep -q 'UpsertRetrievalItem' agent/db.go && echo "PASS: SM-2 integration"

# 7. Build + vet + tests.
go build ./... && echo "PASS: build"
go vet ./... && echo "PASS: vet"
go test ./... -count=1 && echo "PASS: tests"

# 8. No existing tests broken (unexpected regressions).
go test ./... -count=1 -run 'TestConfidence|TestKnowledge|TestSession|TestCourse|TestPlan' && echo "PASS: no regressions"
```

### Human-eyeball notes

- `go test ./agent -count=1 -run TestLogProbe` passes
- Opening the app (existing features) works unchanged — no new routes, no UI changes
- The `retrieval_probe` table appears in the SQLite schema after first launch

## Done criteria

- [ ] `retrieval_probe` table + index in `InitSchema`
- [ ] `GradeProbeAnswer` in `agent/llm.go` with SM-2 rubric prompt
- [ ] `LogProbe(…)` in `agent/db.go` — stores probe, feeds SM-2
- [ ] `GetProbeQuestion(…)` in `agent/db.go` — returns cached question
- [ ] Test coverage: `TestLogProbe`, `TestLogProbeNoAnswer`, `TestGetProbeQuestion`, `TestGetProbeQuestionNone`
- [ ] `go build ./...`, `go vet ./...`, `go test ./...` pass
- [ ] No existing test regressions
- [ ] Pre-baseline fails on current main; post-acceptance passes on the branch

## Rollback notes

Pure additive schema + functions. Revert the commit. The `retrieval_probe` table
has no data impact — it starts empty and no existing code writes to it. No schema
migration needed beyond the `CREATE TABLE IF NOT EXISTS` in InitSchema.
