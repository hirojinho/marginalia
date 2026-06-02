# Hard Mastery Gate — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refuse to mark a plan task *done* unless a `confidence_log` row ≥ a per-course `mastery_threshold` (default 0.7) exists for that task's `id`, with a `--force` override.

**Architecture:** Add `mastery_threshold` to `CourseSettings` (struct + schema + CRUD + validation). Add a `HasConfidenceAtLeast` read helper. Gate the completion transition inside `agent/tools_plan.go` (`applyToggle`/`applyToggleCluster`), threaded a `force` flag from `ToolUpdatePlan`. Add `--force` to `claw-cli plan toggle`. The UI checkbox path (`handler/plan.go`) stays untouched/ungated.

**Tech Stack:** Go 1.26 (`/opt/homebrew/bin/go`), SQLite, `study-app/agent` package.

**Spec:** `docs/superpowers/specs/2026-06-02-mastery-gate-design.md`

**Build/test:** always `/opt/homebrew/bin/go`. **Commit identity:** `git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "..."`. Work on `main` (solo project).

---

### Task 1: `mastery_threshold` course setting (struct, schema, CRUD, validation)

**Files:**
- Modify: `agent/course_settings.go` (struct, `DefaultCourseSettings`, `ValidateCourseSettings`, `SetCourseSetting`)
- Modify: `agent/db.go` (`course_settings` CREATE TABLE ~line 178, migrations slice in `InitSchema`, `GetCourseSettings` SELECT/Scan, `UpsertCourseSettings` INSERT/UPDATE)
- Test: `agent/course_settings_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `agent/course_settings_test.go`:

```go
func TestMasteryThresholdDefaultsTo07(t *testing.T) {
	s := DefaultCourseSettings("ddia")
	if s.MasteryThreshold != 0.7 {
		t.Fatalf("default MasteryThreshold = %v, want 0.7", s.MasteryThreshold)
	}
}

func TestSetMasteryThresholdRoundTrips(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.SetCourseSetting("ddia", "mastery_threshold", "0.85"); err != nil {
		t.Fatalf("set: %v", err)
	}
	s, err := a.GetCourseSettings("ddia")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.MasteryThreshold != 0.85 {
		t.Fatalf("MasteryThreshold = %v, want 0.85", s.MasteryThreshold)
	}
}

func TestSetMasteryThresholdRejectsOutOfRange(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.SetCourseSetting("ddia", "mastery_threshold", "1.5"); err == nil {
		t.Fatalf("expected error for 1.5")
	}
	if err := a.SetCourseSetting("ddia", "mastery_threshold", "-0.1"); err == nil {
		t.Fatalf("expected error for -0.1")
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `/opt/homebrew/bin/go test ./agent/ -run "TestMasteryThreshold|TestSetMasteryThreshold" -v`
Expected: FAIL — `MasteryThreshold` field does not exist (compile error) / unknown key.

- [ ] **Step 3: Add the struct field + default**

In `agent/course_settings.go`, add to the `CourseSettings` struct after `Interleaving bool ...`:
```go
	MasteryThreshold float64 `json:"mastery_threshold"`
```
In `DefaultCourseSettings`, add to the returned literal:
```go
		MasteryThreshold: 0.7,
```

- [ ] **Step 4: Add validation + keyed setter**

In `ValidateCourseSettings`, before `return nil`:
```go
	if s.MasteryThreshold < 0.0 || s.MasteryThreshold > 1.0 {
		return fmt.Errorf("mastery_threshold must be between 0.0 and 1.0, got %v", s.MasteryThreshold)
	}
```
In `SetCourseSetting`, add a case before `default:`:
```go
	case "mastery_threshold":
		f, convErr := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if convErr != nil {
			return fmt.Errorf("mastery_threshold must be a number, got %q", value)
		}
		s.MasteryThreshold = f
```
And update the `default:` error message's valid-keys list to include `mastery_threshold`.

- [ ] **Step 5: Add the DB column (schema + migration) and wire read/write**

In `agent/db.go`, in the `course_settings` CREATE TABLE (~line 178), add after the `interleaving` line:
```sql
	mastery_threshold REAL NOT NULL DEFAULT 0.7,
```
(Place it before `updated_at`; keep valid comma syntax.)

In the `InitSchema` migrations slice, add:
```go
	"ALTER TABLE course_settings ADD COLUMN mastery_threshold REAL NOT NULL DEFAULT 0.7",
```

In `GetCourseSettings`, extend the SELECT and Scan:
```go
	err := a.DB.QueryRow(
		"SELECT framing, exam_style, chunk_pages, stop_after_task, interleaving, mastery_threshold, updated_at FROM course_settings WHERE course_id = ?",
		courseID,
	).Scan(&s.Framing, &s.ExamStyle, &s.ChunkPages, &stop, &inter, &s.MasteryThreshold, &s.UpdatedAt)
```

In `UpsertCourseSettings`, add `mastery_threshold` to the column list, the `VALUES` placeholders, the `ON CONFLICT ... DO UPDATE SET` clause (`mastery_threshold=excluded.mastery_threshold`), and pass `s.MasteryThreshold` in the args (in the matching position — between `inter` and `time.Now().UnixMilli()`).

- [ ] **Step 6: Run to verify they pass**

Run: `/opt/homebrew/bin/go test ./agent/ -run "TestMasteryThreshold|TestSetMasteryThreshold" -v`
Expected: PASS (all three).

- [ ] **Step 7: Full suite + build**

Run: `/opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build .`
Expected: green (the migration is idempotent; existing course_settings tests still pass).

- [ ] **Step 8: Commit**

```bash
git add agent/course_settings.go agent/db.go agent/course_settings_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(settings): add mastery_threshold course setting (default 0.7)"
```

---

### Task 2: `HasConfidenceAtLeast` helper

**Files:**
- Modify: `agent/db.go` (add method near `GetConfidenceTrajectory`, ~line 1155)
- Test: `agent/db_test.go` (or `agent/confidence_test.go` if one exists — use `agent/db_test.go`)

- [ ] **Step 1: Write the failing test**

Add to `agent/db_test.go`:

```go
func TestHasConfidenceAtLeast(t *testing.T) {
	a := newMemoryApp(t)
	// seed a session so the FK on confidence_log.session_id is satisfied
	sess, err := a.CreateSession("ddia", "t", "study")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// no confidence yet → false
	ok, err := a.HasConfidenceAtLeast("task-1", 0.7)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected false with no confidence logged")
	}

	// log 0.5 → still below 0.7
	if _, err := a.LogConfidence(sess.ID, "task-1", 0.5, "manual", ""); err != nil {
		t.Fatalf("log 0.5: %v", err)
	}
	ok, _ = a.HasConfidenceAtLeast("task-1", 0.7)
	if ok {
		t.Fatalf("expected false at 0.5 < 0.7")
	}

	// log 0.8 (latest) → now ≥ 0.7
	if _, err := a.LogConfidence(sess.ID, "task-1", 0.8, "manual", ""); err != nil {
		t.Fatalf("log 0.8: %v", err)
	}
	ok, _ = a.HasConfidenceAtLeast("task-1", 0.7)
	if !ok {
		t.Fatalf("expected true at latest 0.8 ≥ 0.7")
	}
}
```

Note: confirm `CreateSession`'s signature by grepping (`grep -n "func (a \*App) CreateSession" agent/*.go`); it returns `(int64, error)`. Adjust the two args if the real signature differs (e.g. `(course, topic string)`).

- [ ] **Step 2: Run to verify it fails**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestHasConfidenceAtLeast -v`
Expected: FAIL — `HasConfidenceAtLeast` undefined.

- [ ] **Step 3: Implement the helper**

In `agent/db.go`, after `GetConfidenceTrajectory`:

```go
// HasConfidenceAtLeast reports whether the most recent logged confidence for
// knowledgeComponentID is ≥ threshold. Returns false when none is logged.
func (a *App) HasConfidenceAtLeast(knowledgeComponentID string, threshold float64) (bool, error) {
	pts, err := a.GetConfidenceTrajectory(knowledgeComponentID, 1)
	if err != nil {
		return false, err
	}
	return len(pts) > 0 && pts[0].Value >= threshold, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestHasConfidenceAtLeast -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add agent/db.go agent/db_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(confidence): add HasConfidenceAtLeast threshold check"
```

---

### Task 3: Gate the completion transition in `tools_plan.go`

**Files:**
- Modify: `agent/tools_plan.go` (`ToolUpdatePlan` arg struct + dispatch, `applyToggle`, `applyToggleCluster`, new `masteryGateRefusal`)
- Test: `agent/tools_plan_test.go`

Context: `samplePlan()` tasks have **empty `ID`** — by design those are ungateable (allowed), so existing toggle tests keep passing. New gate tests must use a plan whose tasks have IDs. `newMemoryApp`, `writePlan`, `samplePlan` already exist in the test package.

- [ ] **Step 1: Write the failing tests**

Add to `agent/tools_plan_test.go`:

```go
// a plan whose tasks carry ids, so the gate can look up confidence by id
func gatedPlan() *JSONPlan {
	return &JSONPlan{
		ID:   "gate-course",
		Name: "Gate",
		Phases: []Phase{{
			Title: "P1",
			Tasks: []Task{
				{ID: "t-0", Title: "Task zero", Done: false},
				{ID: "t-1", Title: "Task one", Done: false},
			},
		}},
	}
}

func TestMasteryGate_BlocksSetDoneWithoutConfidence(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedPlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-course","action":"set_done","task_index":0}`))
	if !strings.Contains(out, "mastery gate") {
		t.Fatalf("expected mastery-gate refusal, got %q", out)
	}
	loaded := a.LoadPlan("gate-course")
	if loaded.Phases[0].Tasks[0].Done {
		t.Fatalf("task should remain undone after refusal")
	}
}

func TestMasteryGate_AllowsWithConfidence(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedPlan())
	sess, err := a.CreateSession("gate-course", "t", "study")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if _, err := a.LogConfidence(sess.ID, "t-0", 0.8, "manual", ""); err != nil {
		t.Fatalf("log: %v", err)
	}
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-course","action":"set_done","task_index":0}`))
	if !strings.Contains(out, "done") || strings.Contains(out, "mastery gate") {
		t.Fatalf("expected success, got %q", out)
	}
	if !a.LoadPlan("gate-course").Phases[0].Tasks[0].Done {
		t.Fatalf("task should be done")
	}
}

func TestMasteryGate_ForceBypasses(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedPlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-course","action":"set_done","task_index":1,"force":true}`))
	if strings.Contains(out, "mastery gate") {
		t.Fatalf("force should bypass, got %q", out)
	}
	if !a.LoadPlan("gate-course").Phases[0].Tasks[1].Done {
		t.Fatalf("task should be done with force")
	}
}

func TestMasteryGate_SetUndoneNotGated(t *testing.T) {
	a := newMemoryApp(t)
	p := gatedPlan()
	p.Phases[0].Tasks[0].Done = true
	writePlan(t, a, p)
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-course","action":"set_undone","task_index":0}`))
	if strings.Contains(out, "mastery gate") {
		t.Fatalf("set_undone must never be gated, got %q", out)
	}
	if a.LoadPlan("gate-course").Phases[0].Tasks[0].Done {
		t.Fatalf("task should be undone")
	}
}

func TestMasteryGate_EmptyIDAllowed(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan()) // sample tasks have empty ID
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"set_done","task_index":0}`))
	if strings.Contains(out, "mastery gate") {
		t.Fatalf("empty-id task must be ungateable, got %q", out)
	}
	if !a.LoadPlan("ce297").Phases[0].Tasks[0].Done {
		t.Fatalf("empty-id task should complete")
	}
}
```

- [ ] **Step 2: Run to verify they fail**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestMasteryGate -v`
Expected: FAIL — no gate yet (`set_done` succeeds where a refusal is expected; `force` key is ignored).

- [ ] **Step 3: Add `force` to the arg struct + thread it through**

In `agent/tools_plan.go`, in `ToolUpdatePlan`, add `Force bool \`json:"force"\`` to the anonymous struct, and pass it into the toggle path:
```go
	switch p.Action {
	case "toggle", "set_done", "set_undone":
		return a.applyToggle(plan, p.Action, p.TaskIndex, p.Force)
	case "add_task":
		return a.applyAddTask(plan, p.TaskTitle, p.TaskPriority)
```

Change `applyToggle`'s signature to accept `force bool` and add the gate check right before `applyAction` in the phase-task branch:
```go
func (a *App) applyToggle(plan *JSONPlan, action string, taskIndex int, force bool) string {
	count := 0
	for i := range plan.Phases {
		for j := range plan.Phases[i].Tasks {
			if count == taskIndex {
				if refusal := a.masteryGateRefusal(plan.ID, &plan.Phases[i].Tasks[j], action, force); refusal != "" {
					return refusal
				}
				applyAction(&plan.Phases[i].Tasks[j].Done, action)
				if err := a.SavePlan(plan); err != nil {
					return "error saving plan: " + err.Error()
				}
				return fmt.Sprintf("Task %d %q in phase %q marked as %s",
					taskIndex, plan.Phases[i].Tasks[j].Title, plan.Phases[i].Title, doneState(plan.Phases[i].Tasks[j].Done))
			}
			count++
		}
		for k := range plan.Phases[i].Clusters {
			if msg, found := a.applyToggleCluster(plan, action, taskIndex, i, k, &count, force); found {
				if err := a.SavePlan(plan); err != nil {
					return "error saving plan: " + err.Error()
				}
				return msg
			}
		}
	}
	return fmt.Sprintf("error: task index %d not found (plan has %d tasks)", taskIndex, count)
}
```

Note: `applyToggleCluster` becomes a method on `*App` (was a free function) so it can call the gate. Update its signature and the gate check, and **return the refusal as the `found=true` message without mutating** (so the caller does not `SavePlan` a no-op — return the refusal and `true` so it short-circuits, OR return refusal with `found=true` and let the caller print it; to avoid an unwanted save, handle the save inside applyToggle only when no refusal). Implement as:
```go
func (a *App) applyToggleCluster(plan *JSONPlan, action string, taskIndex, phaseIdx, clusterIdx int, count *int, force bool) (string, bool) {
	for j := range plan.Phases[phaseIdx].Clusters[clusterIdx].Tasks {
		if *count == taskIndex {
			task := &plan.Phases[phaseIdx].Clusters[clusterIdx].Tasks[j]
			if refusal := a.masteryGateRefusal(plan.ID, task, action, force); refusal != "" {
				return refusal, true // found, but refused — caller must NOT save on a refusal
			}
			applyAction(&task.Done, action)
			return fmt.Sprintf("Task %d %q in cluster %q marked as %s",
				taskIndex, task.Title, plan.Phases[phaseIdx].Clusters[clusterIdx].Title, doneState(task.Done)), true
		}
		*count++
	}
	return "", false
}
```

**Important correctness fix:** because a cluster refusal returns `found=true`, the caller in `applyToggle` would still `SavePlan`. That is acceptable (no fields changed → SavePlan writes an identical plan) but to be clean, guard it: in `applyToggle`'s cluster loop, only save when the returned message does NOT start with `"refused:"`:
```go
		for k := range plan.Phases[i].Clusters {
			if msg, found := a.applyToggleCluster(plan, action, taskIndex, i, k, &count, force); found {
				if !strings.HasPrefix(msg, "refused:") {
					if err := a.SavePlan(plan); err != nil {
						return "error saving plan: " + err.Error()
					}
				}
				return msg
			}
		}
```
(Add `"strings"` to the `agent/tools_plan.go` imports if not present.)

- [ ] **Step 4: Implement `masteryGateRefusal`**

Add to `agent/tools_plan.go`:
```go
// masteryGateRefusal returns a non-empty "refused: ..." message when completing
// task must be blocked (no logged confidence ≥ the course's mastery_threshold),
// or "" when the action is allowed. Empty-id tasks are ungateable (allowed).
func (a *App) masteryGateRefusal(planID string, task *Task, action string, force bool) string {
	if force {
		return ""
	}
	completing := action == "set_done" || (action == "toggle" && !task.Done)
	if !completing || task.ID == "" {
		return ""
	}
	s, _ := a.GetCourseSettings(planID)
	ok, err := a.HasConfidenceAtLeast(task.ID, s.MasteryThreshold)
	if err != nil {
		return "" // never block on a read error
	}
	if !ok {
		return fmt.Sprintf("refused: mastery gate — task %q has no logged confidence ≥ %.2f. Ask the learner to rate confidence and run `claw-cli confidence log`, or pass --force to override.",
			task.Title, s.MasteryThreshold)
	}
	return ""
}
```
(The tests assert the substring `"mastery gate"`, which this message contains.)

- [ ] **Step 5: Run gate tests + existing toggle tests**

Run: `/opt/homebrew/bin/go test ./agent/ -run "TestMasteryGate|TestToolUpdatePlan|TestApplyAction" -v`
Expected: PASS — new gate tests pass AND the pre-existing toggle/set_done tests still pass (their tasks have empty ids → ungated).

- [ ] **Step 6: Full suite + build**

Run: `/opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build .`
Expected: green.

- [ ] **Step 7: Commit**

```bash
git add agent/tools_plan.go agent/tools_plan_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(plan): hard mastery gate on task completion (force-overridable)"
```

---

### Task 4: `--force` flag on `claw-cli plan toggle`

**Files:**
- Modify: `claw-cli/main.go` (`planToggle`, ~line 516)
- Test: `claw-cli/main_test.go`

- [ ] **Step 1: Write the failing test**

Add to `claw-cli/main_test.go`. This drives the full path through `ToolUpdatePlan`. It writes a plan with an id-bearing task into the temp vault, then toggles with `--force`.

```go
func TestPlanToggleForceBypassesGate(t *testing.T) {
	dbPath := newTempDB(t)
	// The CLI resolves plan files from VAULT_ROOT (env) → resolveStudyRoot() →
	// "/workspace". Pin it to the temp dir so LoadPlan finds our plan deterministically.
	vault := filepath.Dir(dbPath) // newTempDB puts study.db at <vault>/study.db
	t.Setenv("VAULT_ROOT", vault)
	plansDir := filepath.Join(vault, "data", "plans")
	if err := os.MkdirAll(plansDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	planJSON := `{"id":"gate-course","name":"Gate","phases":[{"title":"P1","tasks":[{"id":"t-0","title":"Task zero","done":false}]}]}`
	if err := os.WriteFile(filepath.Join(plansDir, "gate-course.json"), []byte(planJSON), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "plan", "toggle",
		"--course", "gate-course", "--task", "0", "--force",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "mastery gate") {
		t.Fatalf("force should bypass gate, stdout: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "done") {
		t.Fatalf("expected task marked done, stdout: %s", stdout.String())
	}
}
```

Note (verified): `newAppFromEnv` sets `VaultRoot` from `VAULT_ROOT` env → `resolveStudyRoot()` → `/workspace`. The test pins `VAULT_ROOT` to the temp dir via `t.Setenv`, and `App.VaultPath("data","plans")` then resolves under it. `main_test.go` already imports `os` and `path/filepath`.

- [ ] **Step 2: Run to verify it fails**

Run: `/opt/homebrew/bin/go test ./claw-cli/ -run TestPlanToggleForceBypassesGate -v`
Expected: FAIL — `--force` is an unknown flag (exit 2) until added.

- [ ] **Step 3: Add the flag and thread it into the JSON**

In `claw-cli/main.go`, in `planToggle`, add after the other flag declarations:
```go
	force := fs.Bool("force", false, "bypass the mastery gate (use only on explicit user request)")
```
And include it in the marshaled map:
```go
	argsJSON, _ := json.Marshal(map[string]any{
		"plan_id":    *course,
		"action":     "toggle",
		"task_index": *taskIndex,
		"force":      *force,
	})
```

- [ ] **Step 4: Run to verify it passes**

Run: `/opt/homebrew/bin/go test ./claw-cli/ -run TestPlanToggleForceBypassesGate -v`
Expected: PASS. (If the vault-path assumption is wrong, fix per the Step 1 note and re-run.)

- [ ] **Step 5: Full suite + build**

Run: `/opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build . && /opt/homebrew/bin/go build ./claw-cli`
Expected: green.

- [ ] **Step 6: Commit**

```bash
git add claw-cli/main.go claw-cli/main_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(claw-cli): plan toggle --force to bypass the mastery gate"
```

---

### Task 5: Make the gate legible to Pi (AGENTS.md touch-ups)

**Files:**
- Modify: `agent/sandbox.go` (steering key list ~line 210; add a gate note in the study-plan section)
- Test: `agent/sandbox_test.go`

- [ ] **Step 1: Write the failing test**

Add to `agent/sandbox_test.go`:
```go
func TestAgentsMDMentionsMasteryGate(t *testing.T) {
	var sm SandboxManager
	out := string(sm.studyTuningSections("ddia"))
	if !strings.Contains(out, "mastery_threshold") {
		t.Fatalf("steering key list must include mastery_threshold")
	}
	if !strings.Contains(out, "mastery gate") {
		t.Fatalf("must explain the plan-toggle mastery gate to the agent")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestAgentsMDMentionsMasteryGate -v`
Expected: FAIL.

- [ ] **Step 3: Edit the steering key list**

In `agent/sandbox.go` line 210, change the key list
`<framing|exam_style|chunk_pages|stop_after_task|interleaving>` to
`<framing|exam_style|chunk_pages|stop_after_task|interleaving|mastery_threshold>`.

- [ ] **Step 4: Add a gate note**

Append a sentence to the study-plan / completion guidance block in `writeAgentsMD` (the section that documents `claw-cli plan status`/`plan toggle`). Add a line such as:
```go
	"\n**Mastery gate:** `claw-cli plan toggle` will refuse with a \"mastery gate\" message if the task has no logged confidence ≥ the course's mastery_threshold. The fix is to elicit and log confidence (Rule 3) first. Pass `--force` ONLY when Eduardo explicitly says to mark it done anyway.\n"
```
Place this where the plan tools are described (search for the existing `claw-cli plan status` string in `writeAgentsMD`/`studyTuningSections` and add the note adjacent to it, so it lands in the same generated section the test inspects via `studyTuningSections`). If the plan-tools text is built outside `studyTuningSections`, ensure the "mastery gate" string is reachable from `studyTuningSections` output so the test passes — e.g. add the note inside the pedagogy/steering block that `studyTuningSections` returns.

- [ ] **Step 5: Run to verify it passes**

Run: `/opt/homebrew/bin/go test ./agent/ -run TestAgentsMDMentionsMasteryGate -v`
Expected: PASS.

- [ ] **Step 6: Full suite + build**

Run: `/opt/homebrew/bin/go test ./... && /opt/homebrew/bin/go build .`
Expected: green.

- [ ] **Step 7: Commit**

```bash
git add agent/sandbox.go agent/sandbox_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho commit -m "feat(pedagogy): surface the mastery gate + mastery_threshold to the agent"
```

---

### Task 6: Build, deploy, live-verify

**Files:** none (operational).

- [ ] **Step 1: Cross-compile both binaries**
```bash
cd ~/Documents/ITA/claw-study
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/study-app-linux .
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o /tmp/claw-cli-linux ./claw-cli
ls -la /tmp/study-app-linux /tmp/claw-cli-linux
```

- [ ] **Step 2: Deploy (back up both, restart)**
```bash
scp /tmp/study-app-linux nanoclaw:/home/eduardo/stack/study-app/bin/study-app.new
scp /tmp/claw-cli-linux nanoclaw:/home/eduardo/stack/study-app/bin/claw-cli.new
ssh nanoclaw 'cd ~/stack/study-app/bin && cp study-app study-app.bak && cp claw-cli claw-cli.bak && mv study-app.new study-app && mv claw-cli.new claw-cli && chmod +x study-app claw-cli && export XDG_RUNTIME_DIR=/run/user/$(id -u) && systemctl --user restart study-app.service && sleep 2 && systemctl --user is-active study-app.service'
```
Expected: `active`.

- [ ] **Step 3: Confirm the migration ran**
```bash
ssh nanoclaw 'cd ~/stack/study-app && sqlite3 data/study.db "SELECT sql FROM sqlite_master WHERE name=\"course_settings\";" | grep -c mastery_threshold'
```
Expected: `1` (column present).

- [ ] **Step 4: Live gate test on a real task**

Pick a real DDIA task index with NO logged confidence (use `claw-cli plan status --course ddia` to find one with an id), then:
```bash
ssh nanoclaw 'cd ~/stack/study-app && ./bin/claw-cli plan toggle --course ddia --task <IDX> --db data/study.db'
```
Expected: a `refused: mastery gate ...` message and the task stays undone (verify via `plan status`). Then log confidence for that task id and confirm the toggle succeeds; then re-undo it with `--force`/`set_undone` to restore original state. Do NOT leave a test task in a changed state — restore it.

- [ ] **Step 5: Confirm UI checkbox still ungated**

In the app, toggle a task with no confidence via the sidebar checkbox — it should flip freely (the `/api/plan/toggle` path is intentionally ungated).

---

## Self-Review

- **Spec coverage:** §1 setting → Task 1; §2 helper → Task 2; §3 gate → Task 3; §4 CLI `--force` → Task 4; §5 prompt legibility → Task 5; deploy/manual → Task 6. UI-ungated is honored (no `handler/plan.go` edits). Empty-id-allowed → `TestMasteryGate_EmptyIDAllowed`.
- **Placeholders:** none — every step has concrete code; `<IDX>` in Task 6 is a runtime value the operator picks from `plan status`, not a code placeholder.
- **Type consistency:** `applyToggle(plan *JSONPlan, action string, taskIndex int, force bool) string` updated at definition AND call site (`ToolUpdatePlan`). `applyToggleCluster` promoted to `*App` method with `force bool` and updated at its one call site. `masteryGateRefusal(planID string, task *Task, action string, force bool) string`, `HasConfidenceAtLeast(string, float64) (bool, error)`, `MasteryThreshold float64` — names used identically across tasks. Refusal messages start with `"refused:"` and contain `"mastery gate"` (matches the save-guard prefix check AND the test substring).
- **Verified facts:** `CreateSession(courseID, topic, mode string) (Session, error)` (use `sess.ID`). CLI vault resolves from `VAULT_ROOT` — Task 4 pins it with `t.Setenv`. Task 5's note tells the implementer to ensure the "mastery gate" string lands in `studyTuningSections` output (what the test inspects).
