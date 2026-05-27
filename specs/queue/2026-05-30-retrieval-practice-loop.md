---
id: 2026-05-30-retrieval-practice-loop
title: retrieval_queue + upsert on confidence + banded due_at + claw-cli retrieve due
max_wall_clock_minutes: 60
max_diff_lines: 350
max_retries: 1
max_tokens: 200000
requires_visual_approval: false
allow_web_search: false
model: glm-5.1
thinking: low
---

## Goal

Persist and schedule retrieval practice (the R1 item, highest single-ROI
pedagogy intervention: Roediger & Karpicke 2006; Karpicke & Blunt 2011, *Science*;
Dunlosky et al. 2013). A `retrieval_queue` table tracks, per Knowledge Component,
when it is next due for review. It is upserted on every confidence log, with a
confidence-banded interval (a deliberate placeholder for the SM-2 scheduler that
the deferred R2 ticket will swap in). A `claw-cli retrieve due` subcommand
surfaces what's due, and Rule 6 (session-open retrieval) is extended to open each
session with the due items.

**Depends on the prior arc tickets** (queued ahead): the renamed
`knowledge_component_id` column (`2026-05-27-...`), the `knowledge_components`
entity (`2026-05-28-...`), and the capture tool (`2026-05-29-...`). All live on
prod by the time this runs.

The banding is intentionally crude and will be replaced by SM-2 (deferred R2):

| confidence | interval |
|---|---|
| `< 0.4` | 1 day |
| `0.4 ≤ c < 0.7` | 3 days |
| `c ≥ 0.7` | 7 days |

## Implementation plan

### Step 1 — `retrieval_queue` table (`agent/db.go`)

In `InitSchema`'s schema string add:

```sql
CREATE TABLE IF NOT EXISTS retrieval_queue (
    knowledge_component_id  TEXT    PRIMARY KEY,
    due_at                  INTEGER NOT NULL,
    last_confidence         REAL    NOT NULL,
    updated_at              INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_retrieval_queue_due ON retrieval_queue(due_at);
```

New table; no migration-slice entry. No FK to `knowledge_components` (the logged
id may still be a plan-task id during the transition).

### Step 2 — Banding helper (`agent/db.go` or `agent/types.go`)

Add a pure function — it is the unit the `/debug/retrieve-band` endpoint and the
upsert both call, so keep it standalone:

```go
// RetrievalIntervalDays maps a confidence value to the next-review interval.
// Crude banding; replaced by SM-2 in a later ticket.
func RetrievalIntervalDays(confidence float64) int {
    switch {
    case confidence < 0.4:
        return 1
    case confidence < 0.7:
        return 3
    default:
        return 7
    }
}
```

**Exact boundaries (bake into tests):** `0.3→1`, `0.4→3`, `0.5→3`, `0.7→7`,
`0.8→7`. Note `0.4` and `0.7` fall into the UPPER band (`<` not `<=`).

### Step 3 — Upsert method + wire into `LogConfidence` (`agent/db.go`)

1. Add `UpsertRetrievalItem(knowledgeComponentID string, lastConfidence float64) error`:
   - `now := time.Now().UnixMilli()`; `days := RetrievalIntervalDays(lastConfidence)`;
     `dueAt := now + int64(days)*86400000`.
   - `INSERT INTO retrieval_queue (knowledge_component_id, due_at, last_confidence, updated_at) VALUES (?,?,?,?) ON CONFLICT(knowledge_component_id) DO UPDATE SET due_at=excluded.due_at, last_confidence=excluded.last_confidence, updated_at=excluded.updated_at`.
2. In `LogConfidence` (around line 863, after the successful `INSERT INTO
   confidence_log` and obtaining the row id), call
   `if err := a.UpsertRetrievalItem(knowledgeComponentID, value); err != nil { return 0, fmt.Errorf("upsert retrieval_queue: %w", err) }` before returning the id.
   (`knowledgeComponentID` is the parameter name after the `2026-05-27` rename
   ticket; if the parameter is still named `kcID` in the deployed code, use that
   name.)

### Step 4 — Query due items (`agent/db.go`)

Add `GetDueRetrievalItems(now int64, limit int) ([]RetrievalItem, error)` where:

```go
type RetrievalItem struct {
    KnowledgeComponentID string  `json:"knowledge_component_id"`
    DueAt                int64   `json:"due_at"`
    LastConfidence       float64 `json:"last_confidence"`
}
```
(put the struct in `agent/types.go` near `ConfidencePoint`). Default `limit` to
50 when `<= 0`. `SELECT knowledge_component_id, due_at, last_confidence FROM
retrieval_queue WHERE due_at <= ? ORDER BY due_at ASC LIMIT ?`.

### Step 5 — `claw-cli retrieve due` (`claw-cli/main.go`)

1. In the `runWithStdin` dispatch switch (around line 161), add
   `case "retrieve": return runRetrieve(args[2:], stdout, stderr, dbPath)`.
2. `runRetrieve`: switch on `args[0]`; `due` → `retrieveDue`; unknown → usage, return 2.
3. `retrieveDue`: flags `--limit` (default 50), `--db`; resolve DB; build app
   with `newAppFromEnv(resolvedDB, false)`; call `GetDueRetrievalItems(time.Now().UnixMilli(), limit)`;
   print one line per item: `<knowledge_component_id>\t<last_confidence>`.
   Mirror `confidenceTrajectory`'s flag/resolve/app pattern (main.go ~867).

### Step 6 — `/debug/retrieve-band` endpoint (`handler/debug.go`, `handler/handler.go`)

1. `bandHandler`: GET only; read `confidence` query param; parse as float
   (`strconv.ParseFloat`); on parse error return 400 via `writeError`; respond
   200 with `writeJSON` of `{ "confidence": <float>, "interval_days": <int> }`
   computed via `agent.RetrievalIntervalDays(c)`.
2. Register `mux.HandleFunc("/debug/retrieve-band", h.bandHandler)` in
   `handler/handler.go` (the `/debug/*` block ~line 65).

### Step 7 — Extend Rule 6 (`agent/sandbox.go`)

Rewrite Rule 6 (around line 169) so the session-open retrieval check is driven
by the queue: at session start, run `claw-cli retrieve due` to get the
Knowledge Components due for review, and open the session with a retrieval round
on the top 1–2 due items (ask him to recall each in his own words, compare
silently, surface gaps) before anything else. Keep the existing citation
(Roediger & Karpicke 2006) and the "non-negotiable" framing. If nothing is due,
fall back to the current behaviour (recall the most recent completed task).

### Step 8 — Tests

- `agent/` (`db_test.go`): table-driven test of `RetrievalIntervalDays` for the
  exact boundaries in Step 2. A test that `LogConfidence` upserts a
  `retrieval_queue` row (one per component; logging the same component twice
  updates `due_at`/`last_confidence`, does not duplicate). A test that
  `GetDueRetrievalItems` returns only rows with `due_at <= now`, newest-due
  first — insert one due (past `due_at`) and one not-due (future) and assert only
  the due one returns.
- `handler/` test: `/debug/retrieve-band?confidence=0.3` → `interval_days` 1;
  `0.5` → 3; `0.8` → 7; malformed `confidence` → 400; POST → 405.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# /debug/retrieve-band does not exist on current main (404) → the band
# assertions fail.
check_band() {
  local c="$1" want="$2" resp
  resp="$(curl -s -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/debug/retrieve-band?confidence=$c")"
  if ! printf '%s' "$resp" | grep -q "\"interval_days\":$want"; then
    echo "FAIL: confidence=$c expected interval_days=$want, got: $resp"
    exit 1
  fi
}

check_band 0.3 1
check_band 0.5 3
check_band 0.8 7

# Confirm the retrieval_queue table exists via the schema endpoint.
schema="$(curl -s -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/debug/schema?table=retrieval_queue")"
if ! printf '%s' "$schema" | grep -q '"due_at"'; then
  echo "FAIL: retrieval_queue table missing (response: $schema)"
  exit 1
fi

echo "OK: banding correct and retrieval_queue present"
exit 0
```

### Post-acceptance (must PASS after implementation)

**Same script as above.** After implementation `/debug/retrieve-band` returns
the banded interval for each confidence and `retrieval_queue` exists with a
`due_at` column → all assertions pass (exit 0).

### Human-eyeball notes (NOT part of the gate)

- `go test ./...` is the real check on the upsert (one row per component,
  update-not-duplicate) and the due-window query. The bash verifier pins the
  banding boundaries and the table's existence over HTTP.
- After deploy: `ssh nanoclaw 'cd ~/stack/study-app && bin/claw-cli knowledge create --title T --body B'` then log confidence against that id (via a session or a future manual path) and `bin/claw-cli retrieve due` once `due_at` passes — though with 1-day minimum interval, nothing is due immediately; eyeball the row directly with `sqlite3 ... "SELECT * FROM retrieval_queue"`.

## Done criteria

- [ ] `retrieval_queue` table + `due_at` index exist.
- [ ] `RetrievalIntervalDays` matches the boundary table; covered by tests.
- [ ] `LogConfidence` upserts exactly one `retrieval_queue` row per component.
- [ ] `GetDueRetrievalItems` returns only due rows, soonest-due first.
- [ ] `claw-cli retrieve due` lists due components.
- [ ] `/debug/retrieve-band` returns the banded interval; Rule 6 calls `retrieve due` at session open.
- [ ] `go build ./...` and `go test ./...` pass.
- [ ] Pre-baseline fails on current main; post-acceptance passes.

## Rollback notes

`git revert` restores the code. The `retrieval_queue` table persists in any
migrated database but is unreferenced by reverted code and harmless; the
`LogConfidence` upsert call is removed by the revert, so no further rows are
written. Drop the table for a clean slate if desired: `DROP TABLE retrieval_queue`.
