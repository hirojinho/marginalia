# Concept-Level Retrieval: Implement the Atom as the Spaced Unit — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finally implement [ADR 0007](../../adr/0007-knowledge-component-as-atomic-note.md) + [ADR 0019](../../adr/0019-atom-is-the-spaced-unit-formative-recall-and-atomicity-gate.md): retrieval, confidence, and the completion gate all key on the **learner-authored Knowledge Component (atom)** — not the plan task — and the live data is repaired clean.

**Architecture:** The code drifted from ADR 0007 and kept keying confidence/retrieval/gate on the plan task UUID, which let four incompatible id schemes accrete and let recalls mis-log against the wrong key. This plan re-keys the whole subsystem onto `knowledge_components.id`: `confidence log` validates that `--kc` is a real atom; `retrieve due` resolves atom titles + provenance course; a search-before-create command stops re-fragmentation; the completion gate becomes an *atomicity* gate (≥1 atom distilled, no confidence threshold); and a deterministic rebuild + operator-run clean-start migration repair the live DB without fabricating any atoms.

**Tech Stack:** Go 1.26 (`/opt/homebrew/bin/go`), SQLite (modernc), `claw-cli`, in-memory test apps via `newMemoryApp(t)`.

---

## Read this first — what changed from the original draft

The first version of this plan canonicalized on the **plan task id**. The grill against ADR 0007 + `CONTEXT.md` overturned that: the **atom** is the canonical unit (it always was, in the docs — the code just never implemented it). ADR 0019 records the resolved forks:

| Fork | Decision baked into this plan |
|---|---|
| Recall → queue | **C** — in-session recall is formative (unscheduled); only authored atoms enter the queue, distilled at task completion |
| Completion gate | **Atomicity gate** — needs ≥1 atom for the task; **no confidence threshold** (retires the S2 mastery gate on completion) |
| Existing data | **Clean start** — re-key the 2 real authored atoms onto their atom ids; drop task-keyed history from the queue; fabricate nothing |
| Re-fragmentation | **Search-before-create** (`knowledge search` + Rule-9 search-first) |
| Surfacing | **Active-course-first**, cross-course offered |
| Links | **Deferred** |

**Identifier facts (from the audit):**
- The 2 real authored atoms: `6dcdbed5-d2ca-4c5c-9c51-09d226686ff4` ("Leader-based replication…", provenance task `433dd1cf-…`) and `5fb829af-c6ce-4e71-bb40-7d1be90838d1` ("FHA as target-setting gateway…", provenance task `c40384c3-…`). Both currently have their confidence logged against the **task id**, not the atom id — the migration re-keys them.
- One mis-log to delete: an FHA recall written under the write-skew key `a377894f-…` (session 63, `raw_text LIKE 'fha%'`).
- All other `confidence_log` rows are task-keyed formative recalls with no atom behind them — retained as history, dropped from the queue.

---

## One open decision for Eduardo (resolve before Task 5)

**Does the atomicity gate apply to every task type, or only Read tasks?** Watch / Reflect / Deploy tasks don't introduce new readings to distill — a Reflect *operates on* existing atoms. Recommended: **gate only tasks whose title contains "Read"** (the content-bearing ones); Watch/Reflect/Deploy complete without requiring a new atom. The plan implements that default (Task 5) behind a single helper `taskRequiresAtom(task)` you can flip. Confirm or override.

---

## File Structure

| File | Responsibility | Action |
|---|---|---|
| `agent/retrieval_resolve.go` | Plan enumeration, task-id→(course,title) index, atom-label + provenance-course resolution, atom-id validation | **Create** |
| `agent/retrieval_resolve_test.go` | Tests for the above | **Create** |
| `agent/db.go` | `HasAtomForTask`, `SearchKnowledgeComponents`, `RebuildRetrievalQueue` (atom-filtered) | **Modify** |
| `agent/db_test.go` | Tests for the three | **Modify** |
| `agent/tools_plan.go` | `masteryGateRefusal` → `atomicityGateRefusal` | **Modify** (40–61) |
| `agent/tools_plan_test.go` | Gate test | **Modify/Create** |
| `claw-cli/main.go` | `confidence log` atom validation; `retrieve due` titles + `--course`; `retrieve rebuild`; `knowledge search` | **Modify** |
| `agent/sandbox.go` | Rule 3/6/9/10 + atomicity/formative wording | **Modify** (235–272) |
| `agent/sandbox_rules_test.go` | AGENTS.md content assertions | **Create** |
| `CLAUDE.local.md` (repo + VPS) | Mirror rule edits | **Modify** |
| `docs/ops/2026-06-13-retrieval-data-migration.md` | Operator-run clean-start migration runbook | **Create** |

---

## Phase 1 — Resolution + atom validation

### Task 1: Task-title index + provenance helpers

**Files:** Create `agent/retrieval_resolve.go`, `agent/retrieval_resolve_test.go`

- [ ] **Step 1: Write the failing test**

```go
package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func writePlan(t *testing.T, app *App, courseID, body string) {
	t.Helper()
	dir := app.VaultPath("data", "plans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, courseID+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
}

func TestBuildTaskTitleIndex(t *testing.T) {
	app := newMemoryApp(t)
	writePlan(t, app, "ddia", `{"id":"ddia","name":"DDIA","phases":[
		{"title":"P3","tasks":[{"id":"wskew-id","title":"3.6 Write Skew"}]},
		{"title":"P4","clusters":[{"title":"c","tasks":[{"id":"repl-id","title":"4.1 Replication"}]}]}]}`)
	writePlan(t, app, "ddia.json.bak-x", `not json`) // backup must be ignored

	idx, err := app.BuildTaskTitleIndex()
	if err != nil {
		t.Fatalf("BuildTaskTitleIndex: %v", err)
	}
	if ref, ok := idx["wskew-id"]; !ok || ref.CourseID != "ddia" || ref.Title != "3.6 Write Skew" {
		t.Fatalf("wskew-id = %+v ok=%v", ref, ok)
	}
	if ref, ok := idx["repl-id"]; !ok || ref.Title != "4.1 Replication" {
		t.Fatalf("repl-id (cluster) = %+v ok=%v", ref, ok)
	}
}
```

- [ ] **Step 2: Run — expect FAIL** (`undefined: BuildTaskTitleIndex`/`TaskRef`)

Run: `cd ~/Documents/ITA/claw-study && /opt/homebrew/bin/go test ./agent/ -run TestBuildTaskTitleIndex -v`

- [ ] **Step 3: Implement**

```go
package agent

import (
	"os"
	"strings"
)

// TaskRef identifies a plan task by its course and human-readable title.
type TaskRef struct {
	CourseID string
	Title    string
}

// ListPlanCourseIDs returns the course id of every plan file under data/plans
// (base filename without ".json"), skipping directories and backups like
// "ce297.json.bak-...".
func (a *App) ListPlanCourseIDs() ([]string, error) {
	entries, err := os.ReadDir(a.VaultPath("data", "plans"))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") || strings.Count(name, ".") != 1 {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	return ids, nil
}

// BuildTaskTitleIndex maps every plan task id to its TaskRef across all plans.
func (a *App) BuildTaskTitleIndex() (map[string]TaskRef, error) {
	ids, err := a.ListPlanCourseIDs()
	if err != nil {
		return nil, err
	}
	idx := make(map[string]TaskRef)
	add := func(courseID string, tasks []Task) {
		for _, tk := range tasks {
			if tk.ID != "" {
				idx[tk.ID] = TaskRef{CourseID: courseID, Title: tk.Title}
			}
		}
	}
	for _, courseID := range ids {
		plan := a.LoadPlan(courseID)
		if plan == nil {
			continue
		}
		for _, ph := range plan.Phases {
			add(courseID, ph.Tasks)
			for _, c := range ph.Clusters {
				add(courseID, c.Tasks)
			}
		}
	}
	return idx, nil
}
```

- [ ] **Step 4: Run — expect PASS**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestBuildTaskTitleIndex -v`

- [ ] **Step 5: Commit**

```bash
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho add agent/retrieval_resolve.go agent/retrieval_resolve_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(retrieval): plan task-title index"
```

### Task 2: Atom-label resolution + atom-id validation

**Files:** Modify `agent/retrieval_resolve.go`, `agent/retrieval_resolve_test.go`

Atoms are the canonical key. An atom's *title* comes from `knowledge_components`; its *provenance course* comes from its `source_task_id` resolved through the task index.

- [ ] **Step 1: Write the failing test**

```go
func TestResolveAtomLabelAndIsAtom(t *testing.T) {
	app := newMemoryApp(t)
	writePlan(t, app, "ddia", `{"id":"ddia","name":"DDIA","phases":[
		{"title":"P3","tasks":[{"id":"wskew-task","title":"3.6 Write Skew"}]}]}`)
	atomID, err := app.CreateKnowledgeComponent("Write skew breaks a cross-row invariant", "body", "wskew-task", 0)
	if err != nil {
		t.Fatalf("create atom: %v", err)
	}
	idx, _ := app.BuildTaskTitleIndex()

	title, course, ok := app.ResolveAtomLabel(atomID, idx)
	if !ok || title != "Write skew breaks a cross-row invariant" || course != "ddia" {
		t.Fatalf("label = (%q,%q,%v)", title, course, ok)
	}
	// A plan task id is NOT a valid atom key.
	if app.IsAtom("wskew-task") {
		t.Fatalf("task id must not validate as an atom")
	}
	if !app.IsAtom(atomID) {
		t.Fatalf("atom id must validate")
	}
	if app.IsAtom("22") {
		t.Fatalf("legacy key must not validate")
	}
}
```

- [ ] **Step 2: Run — expect FAIL**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestResolveAtomLabelAndIsAtom -v`

- [ ] **Step 3: Implement** (append to `agent/retrieval_resolve.go`)

```go
// ResolveAtomLabel returns an atom's title and its provenance course (via the
// atom's source_task_id resolved through idx). ok=false if id is not an atom.
func (a *App) ResolveAtomLabel(atomID string, idx map[string]TaskRef) (title, course string, ok bool) {
	kc, err := a.GetKnowledgeComponent(atomID)
	if err != nil || kc == nil {
		return "", "", false
	}
	if ref, found := idx[kc.SourceTaskID]; found {
		return kc.Title, ref.CourseID, true
	}
	return kc.Title, "", true
}

// IsAtom reports whether id is a real knowledge_components row. Used to reject a
// --kc that is a plan task id, integer index, or invented string.
func (a *App) IsAtom(id string) bool {
	kc, err := a.GetKnowledgeComponent(id)
	return err == nil && kc != nil
}
```

> Implementer note: confirm the `knowledge_components` row struct field for `source_task_id` (the `GetKnowledgeComponent` return type). If it is named differently (e.g. `SourceTask`), use that. The migration/audit showed the column is `source_task_id`.

- [ ] **Step 4: Run — expect PASS**; **Step 5: Commit** (`feat(retrieval): resolve atom labels + validate atom ids`)

---

## Phase 2 — `retrieve due` shows titles, filters by course

### Task 3: Re-key-aware `retrieve due`

**Files:** Modify `claw-cli/main.go:1475` (`retrieveDue`)

- [ ] **Step 1: Replace the body**

```go
func retrieveDue(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("retrieve due", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("limit", 50, "max items")
	courseFilter := fs.String("course", "", "only atoms whose provenance course matches")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()

	idx, err := app.BuildTaskTitleIndex()
	if err != nil {
		idx = map[string]agent.TaskRef{}
	}
	items, err := app.GetDueRetrievalItems(time.Now().UnixMilli(), *limit)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	for _, item := range items {
		title, course, ok := app.ResolveAtomLabel(item.KnowledgeComponentID, idx)
		if *courseFilter != "" && course != *courseFilter {
			continue
		}
		if !ok {
			title = "(legacy non-atom key — will be dropped on rebuild)"
		}
		_, _ = fmt.Fprintf(stdout, "%s\t%.2f\t%s\t%s\n",
			item.KnowledgeComponentID, item.LastConfidence, course, title)
	}
	return 0
}
```

- [ ] **Step 2: Build** — `/opt/homebrew/bin/go build ./...`
- [ ] **Step 3: Commit** (`feat(cli): retrieve due resolves atom titles + --course`)

---

## Phase 3 — Search-before-create + atom validation on log

### Task 4: `SearchKnowledgeComponents` + `knowledge search`

**Files:** Modify `agent/db.go`, `agent/db_test.go`, `claw-cli/main.go` (`runKnowledge`)

- [ ] **Step 1: Write the failing test** (`agent/db_test.go`)

```go
func TestSearchKnowledgeComponents(t *testing.T) {
	app := newMemoryApp(t)
	_, _ = app.CreateKnowledgeComponent("Write skew cross-row invariant", "two txns, different rows", "", 0)
	_, _ = app.CreateKnowledgeComponent("Leader-based replication trade-off", "sync vs async", "", 0)

	hits, err := app.SearchKnowledgeComponents("skew", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Title != "Write skew cross-row invariant" {
		t.Fatalf("hits = %+v", hits)
	}
	// Matches body too, case-insensitively.
	if h, _ := app.SearchKnowledgeComponents("SYNC", 10); len(h) != 1 {
		t.Fatalf("body match failed: %+v", h)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**
- [ ] **Step 3: Implement** (`agent/db.go`)

```go
// SearchKnowledgeComponents returns atoms whose title or body contains q
// (case-insensitive), most-recent first. Used for search-before-create dedup.
func (a *App) SearchKnowledgeComponents(q string, limit int) ([]KnowledgeComponent, error) {
	if limit <= 0 {
		limit = 20
	}
	like := "%" + strings.ToLower(q) + "%"
	rows, err := a.DB.Query(
		`SELECT id, title, body, source_task_id, source_session_id, created_at, updated_at
		 FROM knowledge_components
		 WHERE lower(title) LIKE ? OR lower(body) LIKE ?
		 ORDER BY created_at DESC LIMIT ?`, like, like, limit)
	if err != nil {
		return nil, fmt.Errorf("search knowledge_components: %w", err)
	}
	defer rows.Close()
	var out []KnowledgeComponent
	for rows.Next() {
		var kc KnowledgeComponent
		// Match the Scan column order/types used by GetKnowledgeComponent.
		if err := rows.Scan(&kc.ID, &kc.Title, &kc.Body, &kc.SourceTaskID, &kc.SourceSessionID, &kc.CreatedAt, &kc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		out = append(out, kc)
	}
	return out, rows.Err()
}
```

> Implementer note: match the exact `KnowledgeComponent` struct + Scan signature used by `GetKnowledgeComponent` (nullable `source_*` columns may need `sql.Null*` or pointer scanning — copy that function's pattern verbatim). Ensure `strings` is imported in db.go.

- [ ] **Step 4: Run — expect PASS**
- [ ] **Step 5: Add the CLI subcommand** — in `runKnowledge`, add `case "search":` → `knowledgeSearch`:

```go
func knowledgeSearch(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("knowledge search", flag.ContinueOnError)
	fs.SetOutput(stderr)
	limit := fs.Int("limit", 20, "max hits")
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	q := strings.Join(fs.Args(), " ")
	if q == "" {
		_, _ = fmt.Fprintln(stderr, "knowledge search: query required")
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()
	hits, err := app.SearchKnowledgeComponents(q, *limit)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	for _, kc := range hits {
		_, _ = fmt.Fprintf(stdout, "%s\t%s\n", kc.ID, kc.Title)
	}
	return 0
}
```

- [ ] **Step 6: Build + Commit** (`feat: knowledge search for search-before-create dedup`)

### Task 5: `confidence log` rejects any `--kc` that is not a real atom

**Files:** Modify `claw-cli/main.go:1288` (`confidenceLog`)

- [ ] **Step 1: After `newAppFromEnv`, before `LogConfidence`, insert the gate**

```go
	if !app.IsAtom(*kc) {
		_, _ = fmt.Fprintf(stderr,
			"confidence log: --kc %q is not a knowledge component. Confidence now keys on an "+
				"ATOM, not a task. Create the atom first (`claw-cli knowledge create` — or search "+
				"`knowledge search` to reuse one), then log against its id.\n", *kc)
		return 2
	}
```

(No `--allow-unknown` escape: under ADR 0019 the key must be a real atom.)

- [ ] **Step 2: Build + manual check on a DB copy**

```bash
scp nanoclaw:/home/eduardo/stack/study-app/data/study.db /tmp/study-copy.db
# task id rejected:
/opt/homebrew/bin/go run ./claw-cli confidence log --db /tmp/study-copy.db --session 1 --kc a377894f-5b2e-4338-b659-90663877e84f --value 0.5 --raw x; echo exit=$?  # expect exit=2
```

- [ ] **Step 3: Commit** (`feat(cli): confidence log requires a real atom id`)

---

## Phase 4 — Atomicity gate (retires the confidence-threshold gate)

### Task 6: `HasAtomForTask` + `atomicityGateRefusal`

**Files:** Modify `agent/db.go`, `agent/tools_plan.go:40-61`, `agent/tools_plan_test.go`

- [ ] **Step 1: Write the failing test** (`agent/tools_plan_test.go`)

```go
func TestAtomicityGate(t *testing.T) {
	app := newMemoryApp(t)
	read := &Task{ID: "read-task", Title: "🔴 Read: Ch.7 Write Skew", Done: false}

	// No atom yet → completion refused.
	if msg := app.atomicityGateRefusal("ddia", read, "set_done", false); msg == "" {
		t.Fatalf("expected refusal with no atom")
	}
	// --force bypasses.
	if msg := app.atomicityGateRefusal("ddia", read, "set_done", true); msg != "" {
		t.Fatalf("force should bypass, got %q", msg)
	}
	// Authoring an atom for the task → allowed (NO confidence needed).
	if _, err := app.CreateKnowledgeComponent("an atom", "body", "read-task", 0); err != nil {
		t.Fatalf("create atom: %v", err)
	}
	if msg := app.atomicityGateRefusal("ddia", read, "set_done", false); msg != "" {
		t.Fatalf("expected allow after atom authored, got %q", msg)
	}
	// A non-Read task is never atom-gated.
	watch := &Task{ID: "watch-task", Title: "Watch: Kleppmann talk", Done: false}
	if msg := app.atomicityGateRefusal("ddia", watch, "set_done", false); msg != "" {
		t.Fatalf("watch task should not be gated, got %q", msg)
	}
	// set_undone is never gated.
	if msg := app.atomicityGateRefusal("ddia", read, "set_undone", false); msg != "" {
		t.Fatalf("undo never gated, got %q", msg)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**
- [ ] **Step 3: Implement `HasAtomForTask`** (`agent/db.go`)

```go
// HasAtomForTask reports whether at least one knowledge_components row was
// authored with this task as its provenance.
func (a *App) HasAtomForTask(taskID string) (bool, error) {
	var n int
	err := a.DB.QueryRow(
		"SELECT count(*) FROM knowledge_components WHERE source_task_id = ?", taskID,
	).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("count atoms for task: %w", err)
	}
	return n > 0, nil
}
```

- [ ] **Step 4: Replace `masteryGateRefusal`** in `agent/tools_plan.go` (rename + new body; update its call sites in `applyToggle`/`applyToggleCluster` to `atomicityGateRefusal`)

```go
// taskRequiresAtom reports whether completing this task should require a
// distilled atom. Only content-bearing Read tasks do; Watch/Reflect/Deploy
// operate on existing atoms or introduce no new reading. (See ADR 0019.)
func taskRequiresAtom(task *Task) bool {
	return strings.Contains(strings.ToLower(task.Title), "read")
}

// atomicityGateRefusal returns a non-empty "refused: ..." message when a task
// completion must be blocked because no atom has been distilled from it, or ""
// when allowed. Replaces the retired confidence-threshold mastery gate (ADR
// 0019): completion no longer depends on any confidence value.
func (a *App) atomicityGateRefusal(planID string, task *Task, action string, force bool) string {
	if force {
		return ""
	}
	completing := action == "set_done" || (action == "toggle" && !task.Done)
	if !completing || task.ID == "" || !taskRequiresAtom(task) {
		return ""
	}
	has, err := a.HasAtomForTask(task.ID)
	if err != nil {
		return "" // never block on a read error
	}
	if !has {
		return fmt.Sprintf("refused: atomicity gate — no Knowledge Component has been distilled "+
			"from task %q. Ask the learner to state one atomic idea in their own words and capture "+
			"it (`knowledge create`), or pass --force to override.", task.Title)
	}
	return ""
}
```

> Implementer note: ensure `strings` is imported in tools_plan.go. Remove the now-unused `GetCourseSettings`/`HasConfidenceAtLeast`/`MasteryThreshold` references *from this gate only* — leave those functions and the `mastery_threshold` setting in place (harmless; avoids a settings migration). Confirm the call sites pass the same `(planID, task, action, force)` args.

- [ ] **Step 5: Run — expect PASS** (`/opt/homebrew/bin/go test ./agent/ -run TestAtomicityGate -v`)
- [ ] **Step 6: Commit** (`feat(gate): atomicity gate replaces confidence-threshold mastery gate (ADR 0019)`)

---

## Phase 5 — Deterministic, atom-filtered queue rebuild

### Task 7: `RebuildRetrievalQueue` (only real atoms)

**Files:** Modify `agent/db.go`, `agent/db_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestRebuildRetrievalQueueAtomsOnly(t *testing.T) {
	app := newMemoryApp(t)
	atomID, _ := app.CreateKnowledgeComponent("atom A", "b", "", 0)
	seed := func(kc string, v float64, at int64) {
		if _, err := app.DB.Exec(
			"INSERT INTO confidence_log (session_id, knowledge_component_id, value, source, created_at, raw_text) VALUES (NULL,?,?,?,?,?)",
			kc, v, "tool_call", at, ""); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	seed(atomID, 0.4, 1_000) // atom: grade<3 → reset, 1 day
	seed(atomID, 0.9, 2_000) // atom: grade 5, n0→1, 1 day
	seed("task-legacy-id", 0.8, 3_000) // NOT an atom → must be excluded

	n, err := app.RebuildRetrievalQueue()
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 atom queued, got %d", n)
	}
	items, _ := app.GetDueRetrievalItems(1<<62, 50)
	if len(items) != 1 || items[0].KnowledgeComponentID != atomID {
		t.Fatalf("queue = %+v", items)
	}
	if items[0].LastConfidence != 0.9 { // last event, not the poisoned earlier one
		t.Fatalf("last_confidence = %v", items[0].LastConfidence)
	}
	if items[0].DueAt != 2_000+int64(86_400_000) { // from event ts, not wall-clock
		t.Fatalf("due_at = %d", items[0].DueAt)
	}
}
```

- [ ] **Step 2: Run — expect FAIL**
- [ ] **Step 3: Implement** (`agent/db.go`)

```go
// RebuildRetrievalQueue wipes retrieval_queue and reconstructs it from
// confidence_log, INCLUDING ONLY rows whose key is a real knowledge_components
// id (atoms). SM-2 state is threaded per atom in chronological order;
// deterministic — due_at derives from each event's created_at, not wall-clock.
// Returns the number of atoms queued.
func (a *App) RebuildRetrievalQueue() (int, error) {
	if _, err := a.DB.Exec("DELETE FROM retrieval_queue"); err != nil {
		return 0, fmt.Errorf("clear retrieval_queue: %w", err)
	}
	rows, err := a.DB.Query(
		`SELECT c.knowledge_component_id, c.value, c.created_at
		 FROM confidence_log c
		 JOIN knowledge_components k ON k.id = c.knowledge_component_id
		 ORDER BY c.created_at ASC, c.id ASC`)
	if err != nil {
		return 0, fmt.Errorf("read confidence_log: %w", err)
	}
	defer rows.Close()

	type state struct {
		n          int
		ef         float64
		intervalMs int64
		lastConf   float64
		lastAt     int64
	}
	acc := map[string]*state{}
	order := []string{}
	for rows.Next() {
		var kc string
		var value float64
		var at int64
		if err := rows.Scan(&kc, &value, &at); err != nil {
			return 0, fmt.Errorf("scan: %w", err)
		}
		st, ok := acc[kc]
		if !ok {
			st = &state{n: 0, ef: 2.5, intervalMs: 0}
			acc[kc] = st
			order = append(order, kc)
		}
		st.intervalMs, st.n, st.ef = SM2NextInterval(ConfidenceToGrade(value), st.n, st.ef, st.intervalMs)
		st.lastConf, st.lastAt = value, at
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	count := 0
	for _, kc := range order {
		st := acc[kc]
		if _, err := a.DB.Exec(
			`INSERT INTO retrieval_queue (knowledge_component_id, due_at, last_confidence, n, ef, interval_ms, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			kc, st.lastAt+st.intervalMs, st.lastConf, st.n, st.ef, st.intervalMs, st.lastAt); err != nil {
			return count, fmt.Errorf("insert %q: %w", kc, err)
		}
		count++
	}
	return count, nil
}
```

- [ ] **Step 4: Run — expect PASS**; **Step 5: Commit** (`feat(retrieval): atom-filtered deterministic queue rebuild`)

### Task 8: `retrieve rebuild` subcommand

**Files:** Modify `claw-cli/main.go` (`runRetrieve`)

- [ ] **Step 1: Add `case "rebuild":` → `retrieveRebuild`**

```go
func retrieveRebuild(args []string, stdout, stderr io.Writer, dbPath string) int {
	fs := flag.NewFlagSet("retrieve rebuild", flag.ContinueOnError)
	fs.SetOutput(stderr)
	dbOverride := fs.String("db", "", "path to study.db")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	resolvedDB, err := resolveDBPath(*dbOverride, dbPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	app, err := newAppFromEnv(resolvedDB, false)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer func() { _ = app.Close() }()
	n, err := app.RebuildRetrievalQueue()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "rebuilt %d atom(s) into retrieval_queue\n", n)
	return 0
}
```

- [ ] **Step 2: Build + Commit** (`feat(cli): retrieve rebuild`)

---

## Phase 6 — Pedagogy rules (ADR 0019 + the friction fixes)

### Task 9: Rewrite Rules 3, 6, 9, 10 in AGENTS.md

**Files:** Modify `agent/sandbox.go:235-272`, `CLAUDE.local.md`, create `agent/sandbox_rules_test.go`

- [ ] **Step 1: Rule 3 (scoring)** — key on the atom:

> Score retrieval, never ask for a self-rating. Confidence is logged **against a Knowledge Component (atom), never a task**: `claw-cli confidence log --session <SESSION_ID> --kc <ATOM_ID> --value <0-1> --raw "..."`. The `--kc` MUST be an atom id from `knowledge create`/`knowledge search`/`retrieve due` — the CLI rejects task ids and unknown strings. In-session explain-back and boundary checks are **formative** (coaching) and are NOT logged; only recall of an authored atom is scored.

- [ ] **Step 2: Rule 6 (session-open recall)**:

> Run `claw-cli retrieve due --course <active course>` to lead with this course's due atoms; lines are `atom_id<TAB>confidence<TAB>course<TAB>title` — quiz on the **title** and log with `--kc <atom_id>` (column 1). If other courses also have due atoms, *name them and offer* to fold them in — do not front-load them.

- [ ] **Step 3: Rule 9 (atom capture, search-first)**:

> The atom is the spaced unit. Before creating one, **search**: `claw-cli knowledge search "<keywords>"` — if a near-match exists, reuse or refine it rather than minting a duplicate. The learner authors the body in their own words (never the agent). A captured atom enters the spaced queue; a task is **complete once ≥1 atom is distilled from it** (atomicity gate) — there is no confidence threshold blocking completion.

- [ ] **Step 4: Rule 10 (stop-after-task)** — append to the ON-state guidance at `agent/sandbox.go:244`:

> Do NOT read, `pdf extract`, or fetch page boundaries for the next task before the current task is marked done — finish and stop first.

- [ ] **Step 5: Add the AGENTS.md content test** (`agent/sandbox_rules_test.go`, using `newMemoryApp` + the real `writeAgentsMD` signature found via `grep -rn writeAgentsMD agent/`)

```go
func TestAgentsMDConceptLevelRules(t *testing.T) {
	app := newMemoryApp(t)
	dir := t.TempDir()
	// Call writeAgentsMD with its real signature (session/course/settings).
	if err := app.writeAgentsMD(dir /*, ...fixtures */); err != nil {
		t.Fatalf("writeAgentsMD: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	s := string(data)
	for _, want := range []string{
		"against a Knowledge Component", // Rule 3 keys on atom
		"knowledge search",              // Rule 9 search-first
		"atomicity gate",                // completion model
		"before the current task is marked done", // Rule 10 guard
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("AGENTS.md missing %q", want)
		}
	}
}
```

- [ ] **Step 6: Run — expect PASS**; mirror edits into `CLAUDE.local.md`; **Commit** (`feat(pedagogy): atom-keyed scoring, search-first, atomicity gate, no next-task preview`)

---

## Phase 7 — Live data migration (operator-run, clean start)

### Task 10: Migration runbook

**Files:** Create `docs/ops/2026-06-13-retrieval-data-migration.md`

- [ ] **Step 1: Author with this content**

````markdown
# Retrieval clean-start migration (2026-06-13, ADR 0019)

Prereq: patched `study-app` + `claw-cli` deployed to the VPS.

## 0. Back up
```bash
ssh nanoclaw 'cp ~/stack/study-app/data/study.db ~/stack/study-app/data/study.db.bak-pre-atom-migration'
```

## 1. Dry-run
```bash
ssh nanoclaw "sqlite3 ~/stack/study-app/data/study.db \"
-- The two real authored atoms and their (currently task-keyed) confidence:
SELECT k.id AS atom, k.source_task_id AS task,
  (SELECT count(*) FROM confidence_log c WHERE c.knowledge_component_id=k.source_task_id) AS task_keyed_rows
  FROM knowledge_components k;
-- The FHA mis-log to delete:
SELECT id, session_id, substr(raw_text,1,40) FROM confidence_log
  WHERE knowledge_component_id='a377894f-5b2e-4338-b659-90663877e84f' AND raw_text LIKE 'fha%';\""
```
Expected: two atoms (`6dcdbed5-…`/task `433dd1cf-…`, `5fb829af-…`/task `c40384c3-…`) each with task-keyed rows; exactly one FHA mis-log row.

## 2. Apply (single transaction)
```bash
ssh nanoclaw "sqlite3 ~/stack/study-app/data/study.db \"
BEGIN;
-- Delete the FHA recall mis-logged under the write-skew key.
DELETE FROM confidence_log
  WHERE knowledge_component_id='a377894f-5b2e-4338-b659-90663877e84f' AND raw_text LIKE 'fha%';
-- Re-key the two real authored atoms' confidence from task id onto atom id.
UPDATE confidence_log SET knowledge_component_id='6dcdbed5-d2ca-4c5c-9c51-09d226686ff4'
  WHERE knowledge_component_id='433dd1cf-1024-47e3-9564-988235d46b84';   -- Leader-based replication atom
UPDATE confidence_log SET knowledge_component_id='5fb829af-c6ce-4e71-bb40-7d1be90838d1'
  WHERE knowledge_component_id='c40384c3-6364-485e-9716-3b093624f8b1';   -- FHA atom
COMMIT;\""
```
(All other task-keyed rows are left as inert history — the rebuild ignores non-atom keys.)

## 3. Rebuild the queue (atom-filtered)
```bash
ssh nanoclaw 'export VAULT_ROOT=$HOME/stack/study-app; /usr/local/bin/claw-cli retrieve rebuild'
```
Expected: "rebuilt 2 atom(s) into retrieval_queue".

## 4. Verify
```bash
ssh nanoclaw 'export VAULT_ROOT=$HOME/stack/study-app; /usr/local/bin/claw-cli retrieve due --limit 50'
```
Expected: exactly the two atoms, each with a real title + provenance course; no integers/namespaced/legacy keys; no `(legacy non-atom key …)`.

## Rollback
```bash
ssh nanoclaw 'cp ~/stack/study-app/data/study.db.bak-pre-atom-migration ~/stack/study-app/data/study.db && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service'
```
````

- [ ] **Step 2: Commit** (`docs(ops): atom clean-start migration runbook`)

---

## Phase 8 — Verify & deploy

### Task 11: Suite + deploy + migrate

- [ ] **Step 1:** `/opt/homebrew/bin/go test ./...` — all pass.
- [ ] **Step 2:** Build both binaries for linux/amd64:

```bash
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
```

- [ ] **Step 3:** Deploy both binaries + synced `CLAUDE.local.md` (per `claw_study_service` ops cheat sheet); restart `study-app.service`.
- [ ] **Step 4:** Run the Phase-7 migration runbook (now that the patched CLI is live).
- [ ] **Step 5:** Confirm end-to-end: start a real session → session-open quizzes the two due atoms (active-course-first) → completing a Read task requires distilling an atom → `confidence log` against a task id is rejected.

---

## Out of scope / follow-ups

1. **Knowledge-component link graph** (`knowledge_component_links` + `knowledge link` + a Rule to suggest links) — deferred per ADR 0007/0019. The Zettelkasten payoff, but premature at ~2 atoms.
2. **Semantic dedup** (embedding similarity over atom bodies) — keyword search-before-create is enough at this corpus size.
3. **critical-theory plan task-id scheme** (shared base-UUID + letter suffix) — fragile, its own remap ticket. Not touched here.
4. **Scoring double-jeopardy** — boundary recall scored after exhaustive interactive drilling understates understanding; now mostly moot since boundary recall is *formative* (unscored) under ADR 0019, but revisit how the at-creation atom score is taken.

---

## Self-Review

- **Spec/ADR coverage:** atom-keyed confidence (Task 5) ✓; resolution/titles (Tasks 1–3) ✓; search-before-create (Task 4) ✓; atomicity gate retiring the threshold gate (Task 6) ✓ [ADR 0019 §2]; clean-start migration, no fabricated atoms (Tasks 7,10) ✓ [§3]; active-course-first surfacing (Task 3 `--course`, Rule 6) ✓ [§5]; formative-vs-spaced wording (Task 9) ✓ [§1]; links deferred ✓.
- **Placeholder scan:** all code steps carry concrete code. Flagged soft spots: the `KnowledgeComponent` Scan signature (Task 2/4 — copy `GetKnowledgeComponent`) and `writeAgentsMD`'s call signature (Task 9) — both explicitly call out matching the real symbol, since they weren't captured verbatim.
- **Type consistency:** `TaskRef{CourseID,Title}`, `KnowledgeComponent{ID,Title,Body,SourceTaskID,SourceSessionID,...}`, `App.IsAtom`, `App.ResolveAtomLabel`, `App.HasAtomForTask`, `App.SearchKnowledgeComponents`, `App.RebuildRetrievalQueue`, `atomicityGateRefusal`, `taskRequiresAtom` used consistently across tasks; CLI helpers `newAppFromEnv`/`resolveDBPath`/`newMemoryApp` match the real codebase.
- **Open decision** (atomicity gate on Read-only vs all tasks) surfaced at top for Eduardo; default implemented behind `taskRequiresAtom`.
