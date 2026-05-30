---
id: 2026-05-28-knowledge-components-entity
title: knowledge_components table + DB methods + claw-cli knowledge create|show|list
max_wall_clock_minutes: 60
max_diff_lines: 400
max_retries: 1
max_tokens: 200000
requires_visual_approval: false
allow_web_search: false
---

## Goal

Introduce the first-class **Knowledge Component** entity: a content-bearing
atomic note (`id`, `title`, `body`, provenance) that confidence and retrieval
will be tracked against. See `CONTEXT.md` and
`docs/adr/0007-knowledge-component-as-atomic-note.md` for why the unit is an
atomic, learner-authored note rather than a plan task. This ticket builds the
backend create/read path and a CLI inspection surface; the agent-facing capture
tool and the retrieval loop come in later tickets. The note body is authored by
the learner — this ticket only stores whatever body it is given (the CLI passes
it through verbatim).

**Depends on ticket `2026-05-27-rename-knowledge-component-id`** (already in the
queue ahead of this one), which adds the generic `GET /debug/schema?table=<t>`
endpoint this ticket's verifier uses. By the time this ticket runs, that
endpoint is live on prod.

## Implementation plan

### Step 1 — Add the `knowledge_components` table (`agent/db.go`)

In `InitSchema`'s schema string, alongside the other `CREATE TABLE IF NOT
EXISTS` blocks (the `confidence_log` block is around line 143), add:

```sql
CREATE TABLE IF NOT EXISTS knowledge_components (
    id                 TEXT    PRIMARY KEY,
    title              TEXT    NOT NULL,
    body               TEXT    NOT NULL,
    source_task_id     TEXT,
    source_session_id  INTEGER REFERENCES sessions(id) ON DELETE SET NULL,
    created_at         INTEGER NOT NULL,
    updated_at         INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_knowledge_components_task ON knowledge_components(source_task_id);
```

No migration-slice entry is needed (the table is new; `CREATE TABLE IF NOT
EXISTS` covers both fresh and existing databases). Do NOT add a foreign key from
`confidence_log` to this table — during the transition `confidence_log` still
holds plan-task UUIDs that won't match component ids.

### Step 2 — Add the Go struct (`agent/types.go`)

Next to `ConfidencePoint` (around line 95), add:

```go
type KnowledgeComponent struct {
    ID              string `json:"id"`
    Title           string `json:"title"`
    Body            string `json:"body"`
    SourceTaskID    string `json:"source_task_id,omitempty"`
    SourceSessionID int64  `json:"source_session_id,omitempty"`
    CreatedAt       int64  `json:"created_at"`
    UpdatedAt       int64  `json:"updated_at"`
}
```

### Step 3 — Add DB methods (`agent/db.go`)

Add three methods on `*App`, following the style of `LogConfidence` /
`GetConfidenceTrajectory` (around lines 863–906):

1. `CreateKnowledgeComponent(title, body, sourceTaskID string, sourceSessionID int64) (string, error)`:
   - Validate `title` and `body` are non-empty (return an error if either is empty).
   - Generate the id with `uuid.NewString()` (the `github.com/google/uuid`
     package is already a dependency; see `newTaskID()` in `agent/types.go` for
     the existing usage pattern — reuse `newTaskID()` if it is exported/usable,
     otherwise call `uuid.NewString()` directly).
   - `now := time.Now().UnixMilli()`; set both `created_at` and `updated_at` to `now`.
   - Insert; return the new id.
   - For `sourceSessionID == 0`, insert SQL `NULL` (use `sql.NullInt64`), not 0,
     so the `ON DELETE SET NULL` FK stays consistent. Likewise insert `NULL` for
     an empty `sourceTaskID` (use `sql.NullString`).
2. `GetKnowledgeComponent(id string) (*KnowledgeComponent, error)`:
   - `SELECT id, title, body, COALESCE(source_task_id,''), COALESCE(source_session_id,0), created_at, updated_at FROM knowledge_components WHERE id = ?`.
   - Return `nil, nil` if no row (use `errors.Is(err, sql.ErrNoRows)`); other errors propagate.
3. `ListKnowledgeComponents(limit int) ([]KnowledgeComponent, error)`:
   - Default `limit` to 50 when `<= 0`.
   - `SELECT ... ORDER BY created_at DESC LIMIT ?`; same COALESCE column list.

**Edge cases to handle:**
- Empty title or body → error, no row inserted.
- Absent `source_task_id` / `source_session_id` → stored as SQL NULL, read back as `""` / `0`.
- `GetKnowledgeComponent` on a missing id → `(nil, nil)`, not an error.

### Step 4 — Add the `knowledge` CLI subcommand (`claw-cli/main.go`)

1. In the `runWithStdin` dispatch switch (around lines 143–162, where `confidence`
   is wired at line 161), add:
   ```go
   case "knowledge":
       return runKnowledge(args[2:], stdout, stderr, dbPath)
   ```
2. Add `runKnowledge`, mirroring `runConfidence` (around line 849): switch on
   `args[0]` over `create | show | list`, with a usage line for an unknown
   subcommand (return 2).
3. `knowledgeCreate(args, stdout, stderr, dbPath)`: `flag.NewFlagSet` with
   `--title` (string, required), `--body` (string, required), `--source-task-id`
   (string, optional), `--source-session-id` (int64, optional, default 0), and
   `--db` (string). Resolve the DB with `resolveDBPath(*dbOverride, dbPath)`,
   build the app with `newAppFromEnv(resolvedDB, false)`, call
   `CreateKnowledgeComponent`, and print the new id to stdout. Missing
   `--title`/`--body` → message to stderr + return 2.
4. `knowledgeShow(args, ...)`: positional `id` (Arg(0)); on found, print the
   component as one line `<id>\t<title>\t<body>` (or tab-separated fields); on
   `nil` (not found) print `not found` to stderr and return 1.
5. `knowledgeList(args, ...)`: optional `--limit` (default 50); print one
   `<id>\t<title>` line per component.

Follow the exact flag/resolve/app pattern in `confidenceTrajectory`
(claw-cli/main.go around lines 867–905).

### Step 5 — Tests (`agent/db_test.go`)

Using the `newMemoryApp(t)` helper (db_test.go:9): add a test that
`CreateKnowledgeComponent` returns a non-empty id, `GetKnowledgeComponent`
round-trips title/body/provenance, `GetKnowledgeComponent` on a random id
returns `(nil, nil)`, and `ListKnowledgeComponents` returns inserted rows newest
first. Add a test that empty title or body is rejected.

## Verification recipe

### Pre-baseline (must FAIL on current main)

```bash
set -euo pipefail
: "${STAGING_URL:?STAGING_URL required}"
: "${STAGING_TOKEN:?STAGING_TOKEN required}"

# On current main the knowledge_components table does not exist, so
# /debug/schema returns 404 ("unknown table") and the column assertions fail.
resp="$(curl -s -H "Authorization: Bearer $STAGING_TOKEN" "$STAGING_URL/debug/schema?table=knowledge_components")"

for col in title body source_task_id source_session_id; do
  if ! printf '%s' "$resp" | grep -q "\"$col\""; then
    echo "FAIL: knowledge_components missing column $col (response: $resp)"
    exit 1
  fi
done
echo "OK: knowledge_components table present with expected columns"
exit 0
```

### Post-acceptance (must PASS after implementation)

**Same script as above.** After implementation the table exists and
`/debug/schema?table=knowledge_components` lists `title`, `body`,
`source_task_id`, `source_session_id` → all assertions pass (exit 0).

### Human-eyeball notes (NOT part of the gate)

- `go test ./...` exercises the create/read/list round-trip and the
  empty-field rejection — the gate's `go test` step is the real check on
  behaviour; the bash verifier only confirms the migration applied over HTTP.
- After deploy, smoke the CLI: `ssh nanoclaw 'cd ~/stack/study-app && bin/claw-cli knowledge create --title "STAMP control loop" --body "Accidents arise from inadequate control, not just component failure."'` then `bin/claw-cli knowledge list`.

## Done criteria

- [ ] `knowledge_components` table exists with the specified columns + index.
- [ ] `CreateKnowledgeComponent` / `GetKnowledgeComponent` / `ListKnowledgeComponents` implemented and tested.
- [ ] `claw-cli knowledge create|show|list` work end-to-end.
- [ ] `/debug/schema?table=knowledge_components` returns the column list.
- [ ] `go build ./...` and `go test ./...` pass.
- [ ] Pre-baseline fails on current main; post-acceptance passes.

## Rollback notes

`git revert` of the code leaves the `knowledge_components` table in any database
that already ran the new `InitSchema`, but the table is unreferenced by the
reverted code and harmless (no FK points at it). No manual DB step required;
drop the table only if you want a clean slate: `DROP TABLE knowledge_components`.
