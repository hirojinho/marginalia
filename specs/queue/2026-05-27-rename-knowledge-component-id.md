---
id: 2026-05-27-rename-knowledge-component-id
title: Rename confidence_log.kc_id to knowledge_component_id and add /debug/schema
max_wall_clock_minutes: 60
max_diff_lines: 300
max_retries: 1
max_tokens: 200000
requires_visual_approval: false
allow_web_search: false
---

## Goal

The confidence subsystem identifies a unit of knowledge with the column
`kc_id`, an abbreviation, and the value stored there is a plan task UUID. We are
standardising the domain vocabulary on the spelled-out `knowledge_component_id`
(see `CONTEXT.md` and `docs/adr/0007-knowledge-component-as-atomic-note.md`)
ahead of building a first-class knowledge-component entity. This ticket is the
mechanical first step: rename the column, the Go struct field, the JSON tag, and
the `log_confidence` tool parameter end-to-end, with an idempotent SQLite
migration so the already-deployed prod table is renamed in place. It also adds a
small, reusable `/debug/schema` endpoint so the rename (and future schema
changes) can be verified over HTTP. No behaviour changes beyond the rename.

## Implementation plan

### Step 1 — Rename the DB column and add the migration (`agent/db.go`)

1. In the `InitSchema` schema string, the `confidence_log` table definition
   (around line 146): change `kc_id       TEXT    NOT NULL,` to
   `knowledge_component_id TEXT NOT NULL,`.
2. In the same schema string (around line 152): change the index
   `CREATE INDEX IF NOT EXISTS idx_confidence_log_kc ON confidence_log(kc_id, created_at);`
   to reference `knowledge_component_id` instead of `kc_id`. **Keep the index
   name `idx_confidence_log_kc` unchanged** (renaming it would create a
   duplicate index on already-deployed databases).
3. In the `migrations` slice (around lines 162–166), add this entry:
   `"ALTER TABLE confidence_log RENAME COLUMN kc_id TO knowledge_component_id"`.
4. The migration loop (around line 168) currently suppresses only the
   `"duplicate column"` sentinel. Broaden the condition so it ALSO suppresses
   `"no such column"` (the idempotency error this RENAME throws on a database
   that has already been migrated). Concrete change: replace
   `!strings.Contains(err.Error(), "duplicate column")` with
   `!strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "no such column")`.
5. Update the SQL string literals that name the column:
   - `LogConfidence` INSERT (around line 875): `kc_id` → `knowledge_component_id` in the column list.
   - `GetConfidenceTrajectory` SELECT (around line 890): both the selected column and the `WHERE kc_id = ?` clause → `knowledge_component_id`.
   - `GetRecentConfidence` SELECT (around line 914): the selected column → `knowledge_component_id`.
6. Rename the Go parameter `kcID` to `knowledgeComponentID` in the signatures
   of `LogConfidence` (line 864) and `GetConfidenceTrajectory` (line 885), and
   update the in-body references to it.

**Edge cases to handle:**
- Fresh database: `CREATE TABLE` already produces `knowledge_component_id`; the
  `RENAME COLUMN` migration then throws `no such column: kc_id` — suppressed by
  step 4. Correct and benign.
- Already-deployed prod database: table has `kc_id`; the migration renames it
  once. SQLite (>= 3.25) auto-updates the existing index to track the renamed
  column.
- Re-run on a migrated database: `no such column: kc_id` again — suppressed.

### Step 2 — Rename the struct field (`agent/types.go`)

In the `ConfidencePoint` struct (around line 98), change
`KCID      string  ` + "`json:\"kc_id\"`" + ` to
`KnowledgeComponentID string ` + "`json:\"knowledge_component_id\"`" + `.
Then update the two `rows.Scan(...)` targets in `agent/db.go` (around lines 900
and 924) from `&cp.KCID` to `&cp.KnowledgeComponentID`.

### Step 3 — Rename the tool parameter (`agent/tools.go`)

In the `log_confidence` tool definition (around lines 159–171): rename the
property key `kc_id` to `knowledge_component_id`, update its description, change
the `"required"` list from `[]string{"kc_id", "value"}` to
`[]string{"knowledge_component_id", "value"}`, and update the tool's top-level
`Description` text that currently says "Pass kc_id = ...".

### Step 4 — Rename in the tool handler (`agent/tools_confidence.go`)

In `ToolLogConfidence`: rename the local struct field `KCID` (json tag
`kc_id`) to `KnowledgeComponentID` (json tag `knowledge_component_id`); change
the empty-check error string from `"error: kc_id is required"` to
`"error: knowledge_component_id is required"`; pass `p.KnowledgeComponentID` to
`LogConfidence`; and change the success string `"... for kc %s ..."` to
`"... for knowledge component %s ..."`.

### Step 5 — Update the pedagogy rule text (`agent/sandbox.go`)

In Rule 3 (around line 166), change the phrase `call the log_confidence tool
with kc_id = the active task's id field from the plan` to
`call the log_confidence tool with knowledge_component_id = the active task's id
field from the plan`. Leave the rest of the rule unchanged (the binding to the
task id is intentional and changes in a later ticket).

### Step 6 — Update CLI user-facing text (`claw-cli/main.go`)

In `confidenceTrajectory`: update the usage/argument text so the positional
argument and the "is required" error read `knowledge_component_id` rather than
`kc_id`. Do NOT change `confidenceSchema` — it does not reference the column.

### Step 7 — Add the `/debug/schema` endpoint (`handler/debug.go`, `handler/handler.go`)

1. In `handler/debug.go`, add a `schemaHandler`:
   - GET only (use the existing `methodNotAllowed(w, r, http.MethodGet)` guard).
   - Read the `table` query parameter (`r.URL.Query().Get("table")`).
   - Validate it matches `^[a-z_]+$` (reject otherwise with HTTP 400 via
     `writeError`). This prevents SQL injection since `PRAGMA` cannot use a
     bound parameter for the table name.
   - Run `PRAGMA table_info(<table>)` (interpolate the validated name) against
     `h.App.DB`. Collect the `name` column (the 2nd column of `table_info`,
     index 1) of each row into a `[]string`.
   - If the column slice is empty (unknown table), respond 404 via
     `writeError(w, http.StatusNotFound, "unknown table")`.
   - Otherwise respond 200 with `writeJSON` of a struct
     `{ Table string `+"`json:\"table\"`"+`; Columns []string `+"`json:\"columns\"`"+` }`.
2. In `handler/handler.go` `Register` (around line 65), add
   `mux.HandleFunc("/debug/schema", h.schemaHandler)` next to the other
   `/debug/*` routes. It inherits the same auth as the rest of the mux.

**Edge cases to handle:**
- Missing or malformed `table` param → 400.
- Unknown table name (valid format, no such table) → 404 (empty `table_info`).
- Empty table → still returns the column names, because `PRAGMA table_info`
  reports schema, not rows. This is exactly why the verifier uses it.

### Step 8 — Tests

1. In `agent/db_test.go`: update any references to `ConfidencePoint.KCID` to
   `KnowledgeComponentID`. Add/adjust a test that inserts via `LogConfidence`
   and reads back via `GetConfidenceTrajectory`, asserting the round-trip still
   works after the rename (the column is now `knowledge_component_id`).
2. In `handler/debug_test.go` (or a new `handler/schema_test.go`): add a test
   that registers `/debug/schema`, GETs `?table=confidence_log` against an
   initialised in-memory DB, asserts HTTP 200 and that the returned `columns`
   contains `"knowledge_component_id"` and does NOT contain `"kc_id"`. Add a
   case asserting POST returns 405 and an unknown table returns 404.

Match the existing test style: `newMemoryApp(t)` for the agent package; the
`httptest.NewRequest` + `mux` pattern already in `handler/debug_test.go`.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# /debug/schema does not exist on current main (404), and even if it did the
# column is still kc_id. Either way the assertion below fails → exit 1.
resp="$(curl -s -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/debug/schema?table=confidence_log")"

if printf '%s' "$resp" | grep -q '"knowledge_component_id"' && ! printf '%s' "$resp" | grep -q '"kc_id"'; then
  echo "OK: confidence_log column is knowledge_component_id (and kc_id is gone)"
  exit 0
else
  echo "FAIL: rename not present (response: $resp)"
  exit 1
fi
```

### Post-acceptance (must PASS after implementation)

**Same script as above.** On current main the `/debug/schema` endpoint is
absent so the curl returns a 404 body and the assertion fails (exit 1). After
implementation, the endpoint returns the `confidence_log` columns including
`knowledge_component_id` and excluding `kc_id`, so the assertion passes (exit 0).

### Human-eyeball notes (NOT part of the gate)

- `go test ./...` covers the read/write path: `LogConfidence` →
  `GetConfidenceTrajectory` round-trips against the renamed column, and the
  handler test exercises the new endpoint. The bash verifier only confirms the
  migration applied to a live database over HTTP.
- After deploy, confirm the tool still logs: open a session, answer a Rule-3
  confidence question, then `ssh nanoclaw 'sqlite3 ~/stack/study-app/data/study.db "SELECT knowledge_component_id, value FROM confidence_log ORDER BY id DESC LIMIT 3"'`.

## Done criteria

- [ ] `confidence_log` column is `knowledge_component_id` on a fresh DB and after migration of an existing DB.
- [ ] `ConfidencePoint.KnowledgeComponentID` with JSON tag `knowledge_component_id`; no `KCID` / `kc_id` remain in the Go code or tool schema.
- [ ] `log_confidence` tool parameter is `knowledge_component_id`.
- [ ] `GET /debug/schema?table=confidence_log` returns 200 with the column list; POST → 405; unknown table → 404.
- [ ] `go build ./...` and `go test ./...` pass.
- [ ] Pre-baseline verifier fails on current main; post-acceptance passes on the new binary.

## Rollback notes

The `RENAME COLUMN` migration is not reverted by `git revert` alone: reverting
the code restores the `kc_id` schema string, but a database already migrated to
`knowledge_component_id` would then mismatch the reverted code's SQL. If a
rollback is needed after deploy, also run
`sqlite3 ~/stack/study-app/data/study.db "ALTER TABLE confidence_log RENAME COLUMN knowledge_component_id TO kc_id"`
before restarting the reverted binary. The table is ~empty, so no data is at risk.
