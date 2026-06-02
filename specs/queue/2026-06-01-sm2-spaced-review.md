---
id: 2026-06-01-sm2-spaced-review
title: Replace crude banded interval with SM-2 expanding-interval scheduler in retrieval_queue
max_wall_clock_minutes: 45
max_diff_lines: 120
max_retries: 1
max_tokens: 80000
model: deepseek-v4-flash
thinking: off
requires_visual_approval: false
allow_web_search: false
---

## Goal

Swap the placeholder 3-band interval function (`1d` / `3d` / `7d`) in the retrieval
queue scheduler for the SuperMemo-2 (SM-2) expanding-interval algorithm. The agent
boundary does not change — the agent still calls `log_confidence` and `claw-cli
retrieve due` identically. Only the app-side scheduling math changes: intervals now
expand with each successful retrieval and contract on forgetting, producing real
spaced repetition (Cepeda et al. 2008) instead of a flat band. This is R2 from the
pedagogy backlog; the R1 retrieval loop shipped 2026-05-30 with the banding as a
documented placeholder.

## Implementation plan

### Step 1 — Add SM-2 state columns to `retrieval_queue` (inline migration)

In `agent/db.go`, append to the `migrations` slice:

```sql
ALTER TABLE retrieval_queue ADD COLUMN n INTEGER NOT NULL DEFAULT 0
ALTER TABLE retrieval_queue ADD COLUMN ef REAL NOT NULL DEFAULT 2.5
ALTER TABLE retrieval_queue ADD COLUMN interval_ms INTEGER NOT NULL DEFAULT 0
```

Existing rows default `n=0, ef=2.5, interval_ms=0` — the first SM-2 pass initialises
them naturally (n=0 → interval 1 day → n=1).

### Step 2 — Add `ConfidenceToGrade` and `SM2NextInterval` pure functions

Replace the current `RetrievalIntervalDays` function with two new functions in
`agent/db.go` (same location, after the pragma docstring):

**`ConfidenceToGrade(confidence float64) int`** — maps the learner's 0.0–1.0
confidence to an SM-2 grade 0–5:

| Confidence | Grade |
|---|---|
| ≥ 0.9 | 5 |
| 0.7 ≤ c < 0.9 | 4 |
| 0.5 ≤ c < 0.7 | 3 |
| 0.3 ≤ c < 0.5 | 2 |
| 0.1 ≤ c < 0.3 | 1 |
| < 0.1 | 0 |

**`SM2NextInterval(grade int, n int, ef float64, intervalMs int64) (nextIntervalMs int64, nextN int, nextEf float64)`**
— computes the SM-2 schedule. Returns the next due-at offset in milliseconds,
the new repetition count, and the updated easiness factor.

Algorithm:

- If `grade < 3`: reset — `nextN = 0`, `nextIntervalMs = 86400000` (1 day),
  skip EF update.
- Else (`grade ≥ 3`):
  - If `n == 0`: `nextN = 1`, `nextIntervalMs = 86400000`.
  - If `n == 1`: `nextN = 2`, `nextIntervalMs = 6 * 86400000`.
  - If `n ≥ 2`: `intervalDays = intervalMs / 86400000`; `nextIntervalMs =`
    `int64(math.Ceil(float64(intervalDays) * ef)) * 86400000`; `nextN = n + 1`.
- Update EF for `grade ≥ 3` (standard SM-2 formula):
  `nextEf = ef + (0.1 - (5.0 - float64(grade)) * (0.08 + (5.0 - float64(grade)) * 0.02))`.
  Clamp to `[1.3, 2.5]`.
- If `grade < 3`: `nextEf = ef` (unchanged).

### Step 3 — Rework `UpsertRetrievalItem` to use SM-2

Currently `UpsertRetrievalItem` computes `days := RetrievalIntervalDays(confidence)`
and upserts only `due_at`, `last_confidence`, `updated_at`. Rewrite it to:

1. Compute `grade := ConfidenceToGrade(lastConfidence)`.
2. Read the current `n`, `ef`, `interval_ms` from the existing row (or use defaults
   `0, 2.5, 0` if no row exists). Use a `SELECT n, ef, interval_ms FROM
   retrieval_queue WHERE knowledge_component_id = ?` — if `sql.ErrNoRows`, the
   defaults apply.
3. Compute `nextIntervalMs, nextN, nextEf := SM2NextInterval(grade, n, ef, intervalMs)`.
4. `dueAt := now + nextIntervalMs`.
5. Upsert all columns:
   ```sql
   INSERT INTO retrieval_queue (knowledge_component_id, due_at, last_confidence,
     n, ef, interval_ms, updated_at)
   VALUES (?, ?, ?, ?, ?, ?, ?)
   ON CONFLICT(knowledge_component_id) DO UPDATE SET
     due_at=excluded.due_at, last_confidence=excluded.last_confidence,
     n=excluded.n, ef=excluded.ef, interval_ms=excluded.interval_ms,
     updated_at=excluded.updated_at
   ```

This function keeps the same signature (`knowledgeComponentID string,
lastConfidence float64`) — callers (`LogConfidence`) are unchanged.

### Step 4 — Update tests in `agent/db_test.go`

- **Replace `TestRetrievalIntervalDays`** with:
  - `TestConfidenceToGrade` — table-driven, one case per threshold boundary.
  - `TestSM2NextInterval` — exercises the SM-2 state machine:
    - First retrieval (n=0, grade=4) → interval = 1d, n=1.
    - Second retrieval (n=1, grade=4) → interval = 6d, n=2.
    - Third retrieval (n=2, interval_ms=6d, grade=4) → interval ≈ 15d, n=3, EF updated.
    - Forgetting (grade=2) → n=0, interval=1d, EF unchanged.
    - EF floor (low grades over time don't drop below 1.3).
    - EF ceiling (doesn't exceed 2.5).

- **Update `TestLogConfidenceUpsertsRetrievalQueue`**: after `LogConfidence`,
  assert the `n`, `ef`, `interval_ms` columns have the expected values for a first
  confidence log (n=1, ef=2.5, interval_ms=86400000 for grade≥3).

- **Update `TestGetDueRetrievalItems`**: the INSERT statements must include the
  new columns (`n`, `ef`, `interval_ms`).

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
# 1. The crude-banding function still exists and returns banded values.
go test ./agent/ -run TestRetrievalIntervalDays -v 2>&1 | grep -q PASS \
  && echo "FAIL: pre-baseline 1 — RetrievalIntervalDays still active (it should be replaced)" \
  || echo "PASS: pre-baseline 1 — old function gone"

# 2. SM-2 functions do NOT exist yet.
go test ./agent/ -run 'TestConfidenceToGrade|TestSM2NextInterval' -v 2>&1 | grep -q PASS \
  && echo "FAIL: pre-baseline 2 — SM-2 tests pass (they shouldn't exist)" \
  || echo "PASS: pre-baseline 2 — no SM-2 yet"

# 3. retrieval_queue has no n/ef/interval_ms columns.
(grep -q '"n"' agent/db_test.go && grep -q '"n"' agent/db.go) \
  || (echo "PASS: pre-baseline 3 — no n column references in tests or schema"; exit 1)
```

**FAIL signature on main:** pre-baseline 1 passes (old function exists), pre-baseline
2 fails (no SM-2 tests), pre-baseline 3 fails (no n column references), or the
`grep` for `"n"` in test INSERTs returns empty — proving the queue schema is still
3-band only.

### Post-acceptance (must PASS after implementation)

```bash
# 1. SM-2 tests pass.
go test ./agent/ -run 'TestConfidenceToGrade|TestSM2NextInterval' -v

# 2. Confidence→grade mapping holds at every threshold.
go test ./agent/ -run TestConfidenceToGrade -v 2>&1 | grep -E 'PASS|FAIL'

# 3. SM-2 expanding intervals: a second high-confidence log schedules further
#    out than the old band's max (7 days / 604800000ms).
go test ./agent/ -run TestSM2NextInterval -v 2>&1 | grep -q PASS

# 4. Full pipeline: LogConfidence → retrieval_queue row has n, ef, interval_ms.
go test ./agent/ -run TestLogConfidenceUpsertsRetrievalQueue -v

# 5. Due-retrieval query includes the new columns.
go test ./agent/ -run TestGetDueRetrievalItems -v

# 6. No regressions in the full test suite.
go test ./agent/... -count=1
```

### Human-eyeball notes

- Run `sqlite3 data/study.db "SELECT knowledge_component_id, n, ef,
  interval_ms / 86400000 AS interval_days, due_at FROM retrieval_queue"` after
  a few confidence logs. Verify expanding intervals across multiple high-confidence
  retrievals, and reset on a low-confidence one.
- The `claw-cli retrieve due` output is unchanged — due items surface identically
  to the agent.

## Done criteria

- [ ] `ConfidenceToGrade` maps 0.0–1.0 to 0–5 per the threshold table.
- [ ] `SM2NextInterval` produces expanding intervals on consecutive grade ≥3, resets
      n=0 on grade <3, clamps EF to [1.3, 2.5].
- [ ] `UpsertRetrievalItem` reads current SM-2 state from the row, computes new
      state, and upserts all six columns.
- [ ] `retrieval_queue` has `n`, `ef`, `interval_ms` columns (inline migration).
- [ ] `TestConfidenceToGrade`, `TestSM2NextInterval` pass; old
      `TestRetrievalIntervalDays` is removed.
- [ ] `TestLogConfidenceUpsertsRetrievalQueue` asserts SM-2 state after log.
- [ ] `TestGetDueRetrievalItems` INSERTs include the new columns.
- [ ] `go test ./agent/...` and `go vet ./...` pass.

## Rollback notes

The migration adds columns with DEFAULTs — rolling back the binary to the previous
version leaves the extra columns unused (old code never reads or writes them).
`sqlite3` will ignore them. To fully reverse: `ALTER TABLE retrieval_queue DROP
COLUMN n; ALTER TABLE retrieval_queue DROP COLUMN ef; ALTER TABLE retrieval_queue
DROP COLUMN interval_ms;` (SQLite 3.35+). No data loss — the old banding was
lossy by definition.
