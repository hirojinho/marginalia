package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writePlan(t *testing.T, a *App, p *JSONPlan) {
	t.Helper()
	dir := a.VaultPath("data", "plans")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, p.ID+".json"), data, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func samplePlan() *JSONPlan {
	return &JSONPlan{
		ID:   "biology",
		Name: "Safety",
		Phases: []Phase{
			{
				Title: "Foundations",
				Tasks: []Task{
					{Title: "Read Avizienis", Done: false},
					{Title: "Read Leveson", Done: true},
				},
				Clusters: []Cluster{
					{
						Title: "STPA",
						Tasks: []Task{
							{Title: "Step 1", Done: false},
							{Title: "Step 2", Done: false},
						},
					},
				},
			},
		},
	}
}

func TestApplyAction(t *testing.T) {
	tests := []struct {
		name   string
		start  bool
		action string
		want   bool
	}{
		{"toggle false to true", false, "toggle", true},
		{"toggle true to false", true, "toggle", false},
		{"set_done from false", false, "set_done", true},
		{"set_done from true", true, "set_done", true},
		{"set_undone from true", true, "set_undone", false},
		{"set_undone from false", false, "set_undone", false},
		{"unknown action no-op", true, "garbage", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			done := tc.start
			applyAction(&done, tc.action)
			if done != tc.want {
				t.Fatalf("got %v want %v", done, tc.want)
			}
		})
	}
}

func TestDoneState(t *testing.T) {
	if doneState(true) != "done" {
		t.Fatalf("true should be 'done'")
	}
	if doneState(false) != "not done" {
		t.Fatalf("false should be 'not done'")
	}
}

func TestCountTasksInPlan(t *testing.T) {
	if got := countTasksInPlan(samplePlan()); got != 4 {
		t.Fatalf("got %d, want 4", got)
	}
	empty := &JSONPlan{ID: "x"}
	if got := countTasksInPlan(empty); got != 0 {
		t.Fatalf("empty got %d", got)
	}
}

func TestToolUpdatePlan_BadJSON(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ToolUpdatePlan(json.RawMessage("not json"))
	if !strings.HasPrefix(out, "error:") {
		t.Fatalf("expected error, got %q", out)
	}
}

func TestToolUpdatePlan_MissingPlanID(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ToolUpdatePlan(json.RawMessage(`{}`))
	if !strings.Contains(out, "plan_id is required") {
		t.Fatalf("got %q", out)
	}
}

func TestToolUpdatePlan_PlanNotFound(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"missing","action":"toggle","task_index":0}`))
	if !strings.Contains(out, "plan not found") {
		t.Fatalf("got %q", out)
	}
}

func TestToolUpdatePlan_Toggle(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())

	// force bypasses the atomicity gate so this exercises toggle mechanics only
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"biology","action":"toggle","task_index":0,"force":true}`))
	if !strings.Contains(out, "marked as done") {
		t.Fatalf("expected toggled-to-done, got %q", out)
	}

	loaded := a.LoadPlan("biology")
	if !loaded.Phases[0].Tasks[0].Done {
		t.Fatalf("expected Tasks[0].Done=true, plan: %+v", loaded.Phases[0].Tasks[0])
	}
}

func TestToolUpdatePlan_SetDoneOnClusterTask(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())

	// indices: 0,1 phase tasks; 2,3 cluster tasks. force bypasses the atomicity gate.
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"biology","action":"set_done","task_index":3,"force":true}`))
	if !strings.Contains(out, "cluster") || !strings.Contains(out, "done") {
		t.Fatalf("got %q", out)
	}
	loaded := a.LoadPlan("biology")
	if !loaded.Phases[0].Clusters[0].Tasks[1].Done {
		t.Fatalf("cluster task not marked done")
	}
}

func TestToolUpdatePlan_TaskIndexOutOfRange(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"biology","action":"toggle","task_index":99}`))
	if !strings.Contains(out, "not found") {
		t.Fatalf("got %q", out)
	}
}

func TestToolUpdatePlan_AddTask(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())

	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"biology","action":"add_task","task_title":"New thing","task_priority":"high"}`))
	if !strings.Contains(out, "Added task") {
		t.Fatalf("got %q", out)
	}
	loaded := a.LoadPlan("biology")
	last := loaded.Phases[0].Tasks[len(loaded.Phases[0].Tasks)-1]
	if last.Title != "New thing" || last.Priority != "high" {
		t.Fatalf("task not appended correctly: %+v", last)
	}
}

func TestToolUpdatePlan_AddTaskMissingTitle(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"biology","action":"add_task"}`))
	if !strings.Contains(out, "task_title is required") {
		t.Fatalf("got %q", out)
	}
}

func TestToolUpdatePlan_AddTaskNoPhases(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, &JSONPlan{ID: "empty", Name: "Empty"})
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"empty","action":"add_task","task_title":"X"}`))
	if !strings.Contains(out, "no phases") {
		t.Fatalf("got %q", out)
	}
}

func TestToolUpdatePlan_UnknownAction(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"biology","action":"floof","task_index":0}`))
	if !strings.Contains(out, "unknown action") {
		t.Fatalf("got %q", out)
	}
}

func TestCountTasks(t *testing.T) {
	done, total := CountTasks(samplePlan())
	if total != 4 {
		t.Fatalf("total got %d want 4", total)
	}
	if done != 1 {
		t.Fatalf("done got %d want 1", done)
	}
	d, tot := CountTasks(nil)
	if d != 0 || tot != 0 {
		t.Fatalf("nil plan should be 0,0; got %d,%d", d, tot)
	}
}

func TestLoadPlan_Missing(t *testing.T) {
	a := newMemoryApp(t)
	if a.LoadPlan("nope") != nil {
		t.Fatalf("expected nil for missing")
	}
}

func TestLoadPlan_Malformed(t *testing.T) {
	a := newMemoryApp(t)
	dir := a.VaultPath("data", "plans")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0644)
	if a.LoadPlan("bad") != nil {
		t.Fatalf("expected nil for malformed")
	}
}

func TestRewritePlan_PreservesUUIDsForMatchingTitles(t *testing.T) {
	a := newMemoryApp(t)
	old := &JSONPlan{
		ID:   "biology",
		Name: "Safety",
		Phases: []Phase{
			{
				Title: "Foundations",
				Tasks: []Task{
					{ID: "aaa-111", Title: "Read Avizienis", Done: false},
					{ID: "bbb-222", Title: "Read Leveson", Done: true},
				},
				Clusters: []Cluster{
					{
						Title: "STPA",
						Tasks: []Task{
							{ID: "ccc-333", Title: "Step 1", Done: false},
						},
					},
				},
			},
		},
	}
	writePlan(t, a, old)

	newJSON := `{"id":"biology","name":"Safety","phases":[{"title":"Foundations","tasks":[{"title":"Read Avizienis"},{"title":"Read Leveson"}],"clusters":[{"title":"STPA","tasks":[{"title":"Step 1"}]}]}]}`
	// plan_json must be a JSON string, so we need to escape the inner JSON
	escapedJSON, _ := json.Marshal(newJSON)
	out := a.ToolRewritePlan(json.RawMessage(`{"plan_id":"biology","plan_json":` + string(escapedJSON) + `}`))
	if !strings.Contains(out, "3 inherited UUIDs") {
		t.Fatalf("expected 3 inherited UUIDs, got %q", out)
	}

	loaded := a.LoadPlan("biology")
	if loaded.Phases[0].Tasks[0].ID != "aaa-111" {
		t.Fatalf("UUID not preserved for task 0: got %q", loaded.Phases[0].Tasks[0].ID)
	}
	if loaded.Phases[0].Tasks[1].ID != "bbb-222" {
		t.Fatalf("UUID not preserved for task 1: got %q", loaded.Phases[0].Tasks[1].ID)
	}
	if loaded.Phases[0].Clusters[0].Tasks[0].ID != "ccc-333" {
		t.Fatalf("UUID not preserved for cluster task: got %q", loaded.Phases[0].Clusters[0].Tasks[0].ID)
	}
}

func TestRewritePlan_AssignsNewUUIDsForNewTitles(t *testing.T) {
	a := newMemoryApp(t)
	old := &JSONPlan{
		ID:   "biology",
		Name: "Safety",
		Phases: []Phase{
			{Title: "P1", Tasks: []Task{
				{ID: "aaa-111", Title: "Existing Task"},
			}},
		},
	}
	writePlan(t, a, old)

	newJSON := `{"id":"biology","name":"Safety","phases":[{"title":"P1","tasks":[{"title":"Existing Task"},{"title":"Brand New Task"}]}]}`
	escapedJSON, _ := json.Marshal(newJSON)
	out := a.ToolRewritePlan(json.RawMessage(`{"plan_id":"biology","plan_json":` + string(escapedJSON) + `}`))
	if !strings.Contains(out, "1 inherited UUIDs") || !strings.Contains(out, "1 new UUIDs") {
		t.Fatalf("expected 1 inherited + 1 new, got %q", out)
	}

	loaded := a.LoadPlan("biology")
	if loaded.Phases[0].Tasks[0].ID != "aaa-111" {
		t.Fatalf("existing UUID not preserved: got %q", loaded.Phases[0].Tasks[0].ID)
	}
	if loaded.Phases[0].Tasks[1].ID == "" || loaded.Phases[0].Tasks[1].ID == "aaa-111" {
		t.Fatalf("new task should have fresh UUID, got %q", loaded.Phases[0].Tasks[1].ID)
	}
}

func TestRewritePlan_DropsTasksMissingFromNew(t *testing.T) {
	a := newMemoryApp(t)
	old := &JSONPlan{
		ID:   "biology",
		Name: "Safety",
		Phases: []Phase{
			{Title: "P1", Tasks: []Task{
				{Title: "T1"}, {Title: "T2"}, {Title: "T3"}, {Title: "T4"},
			}},
		},
	}
	writePlan(t, a, old)

	newJSON := `{"id":"biology","name":"Safety","phases":[{"title":"P1","tasks":[{"title":"T1"},{"title":"T3"}]}]}`
	escapedJSON, _ := json.Marshal(newJSON)
	_ = a.ToolRewritePlan(json.RawMessage(`{"plan_id":"biology","plan_json":` + string(escapedJSON) + `}`))

	loaded := a.LoadPlan("biology")
	total := countTasksInPlan(loaded)
	if total != 2 {
		t.Fatalf("expected 2 tasks after rewrite, got %d", total)
	}
}

func TestRewritePlan_RejectsMismatchedID(t *testing.T) {
	a := newMemoryApp(t)
	newJSON := `{"id":"guitar","name":"Guitar","phases":[]}`
	escapedJSON, _ := json.Marshal(newJSON)
	out := a.ToolRewritePlan(json.RawMessage(`{"plan_id":"biology","plan_json":` + string(escapedJSON) + `}`))
	if !strings.Contains(out, "does not match plan_id") {
		t.Fatalf("expected mismatch error, got %q", out)
	}
}

func TestRewritePlan_FirstTimeCreate(t *testing.T) {
	a := newMemoryApp(t)
	newJSON := `{"id":"newcourse","name":"New Course","phases":[{"title":"P1","tasks":[{"title":"T1"}]}]}`
	escapedJSON, _ := json.Marshal(newJSON)
	out := a.ToolRewritePlan(json.RawMessage(`{"plan_id":"newcourse","plan_json":` + string(escapedJSON) + `}`))
	if !strings.Contains(out, "0 inherited UUIDs") || !strings.Contains(out, "1 new UUIDs") {
		t.Fatalf("expected 0 inherited + 1 new for first-time create, got %q", out)
	}

	loaded := a.LoadPlan("newcourse")
	if loaded == nil {
		t.Fatal("plan should exist after first-time create")
	}
	if loaded.Phases[0].Tasks[0].ID == "" {
		t.Fatal("task should have a generated UUID")
	}
}

// a plan whose tasks carry ids, so the gate can look up confidence by id
func gatedPlan() *JSONPlan {
	// Read-titled tasks are the ones the atomicity gate applies to (ADR 0019).
	return &JSONPlan{
		ID:   "gate-course",
		Name: "Gate",
		Phases: []Phase{{
			Title: "P1",
			Tasks: []Task{
				{ID: "t-0", Title: "Read: Task zero", Done: false},
				{ID: "t-1", Title: "Read: Task one", Done: false},
			},
		}},
	}
}

func TestAtomicityGate(t *testing.T) {
	app := newMemoryApp(t)
	read := &Task{ID: "read-task", Title: "🔴 Read: Ch.7 Write Skew", Done: false}

	// No atom yet → completion refused.
	if msg := app.atomicityGateRefusal("cs101", read, "set_done", false); msg == "" {
		t.Fatalf("expected refusal with no atom")
	}
	// --force bypasses.
	if msg := app.atomicityGateRefusal("cs101", read, "set_done", true); msg != "" {
		t.Fatalf("force should bypass, got %q", msg)
	}
	// Authoring an atom for the task → allowed (NO confidence needed).
	if _, err := app.CreateKnowledgeComponent("an atom", "body", "read-task", 0); err != nil {
		t.Fatalf("create atom: %v", err)
	}
	if msg := app.atomicityGateRefusal("cs101", read, "set_done", false); msg != "" {
		t.Fatalf("expected allow after atom authored, got %q", msg)
	}
	// A non-Read task is never atom-gated.
	watch := &Task{ID: "watch-task", Title: "Watch: Kleppmann talk", Done: false}
	if msg := app.atomicityGateRefusal("cs101", watch, "set_done", false); msg != "" {
		t.Fatalf("watch task should not be gated, got %q", msg)
	}
	// set_undone is never gated.
	if msg := app.atomicityGateRefusal("cs101", read, "set_undone", false); msg != "" {
		t.Fatalf("undo never gated, got %q", msg)
	}
}

func TestAtomicityGate_BlocksSetDoneWithoutAtom(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedPlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-course","action":"set_done","task_index":0}`))
	if !strings.Contains(out, "atomicity gate") {
		t.Fatalf("expected atomicity-gate refusal, got %q", out)
	}
	if a.LoadPlan("gate-course").Phases[0].Tasks[0].Done {
		t.Fatalf("task should remain undone after refusal")
	}
}

func TestAtomicityGate_AllowsWithAtom(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedPlan())
	// t-0 is the auto-assigned id only if empty; here ids are explicit. Resolve
	// the live id LoadPlan exposes (assignMissingTaskIDs may rewrite empties).
	taskID := a.LoadPlan("gate-course").Phases[0].Tasks[0].ID
	if _, err := a.CreateKnowledgeComponent("an atom", "body", taskID, 0); err != nil {
		t.Fatalf("create atom: %v", err)
	}
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-course","action":"set_done","task_index":0}`))
	if !strings.Contains(out, "done") || strings.Contains(out, "atomicity gate") {
		t.Fatalf("expected success, got %q", out)
	}
	if !a.LoadPlan("gate-course").Phases[0].Tasks[0].Done {
		t.Fatalf("task should be done")
	}
}

func TestAtomicityGate_ForceBypasses(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedPlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-course","action":"set_done","task_index":1,"force":true}`))
	if strings.Contains(out, "atomicity gate") {
		t.Fatalf("force should bypass, got %q", out)
	}
	if !a.LoadPlan("gate-course").Phases[0].Tasks[1].Done {
		t.Fatalf("task should be done with force")
	}
}

func TestAtomicityGate_SetUndoneNotGated(t *testing.T) {
	a := newMemoryApp(t)
	p := gatedPlan()
	p.Phases[0].Tasks[0].Done = true
	writePlan(t, a, p)
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-course","action":"set_undone","task_index":0}`))
	if strings.Contains(out, "atomicity gate") {
		t.Fatalf("set_undone must never be gated, got %q", out)
	}
	if a.LoadPlan("gate-course").Phases[0].Tasks[0].Done {
		t.Fatalf("task should be undone")
	}
}

func TestAtomicityGate_EmptyIDAllowed(t *testing.T) {
	// LoadPlan auto-assigns IDs to empty-id tasks (assignMissingTaskIDs migration),
	// so an empty-id task never reaches the gate through ToolUpdatePlan. Exercise the
	// ungateable-empty-id contract directly on the gate helper.
	a := newMemoryApp(t)
	task := &Task{ID: "", Title: "Read: no id", Done: false}
	if refusal := a.atomicityGateRefusal("biology", task, "set_done", false); refusal != "" {
		t.Fatalf("empty-id task must be ungateable, got %q", refusal)
	}
}

func gatedClusterPlan() *JSONPlan {
	return &JSONPlan{
		ID:   "gate-cluster",
		Name: "GateCluster",
		Phases: []Phase{{
			Title: "P1",
			Clusters: []Cluster{{
				Title: "C1",
				Tasks: []Task{
					{ID: "c-0", Title: "Read: Cluster task zero", Done: false},
				},
			}},
		}},
	}
}

func TestAtomicityGate_BlocksClusterTaskWithoutAtom(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedClusterPlan())
	// index 0 is the cluster task (no phase tasks precede it)
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-cluster","action":"set_done","task_index":0}`))
	if !strings.Contains(out, "atomicity gate") {
		t.Fatalf("expected cluster-task atomicity-gate refusal, got %q", out)
	}
	loaded := a.LoadPlan("gate-cluster")
	if loaded.Phases[0].Clusters[0].Tasks[0].Done {
		t.Fatalf("cluster task should remain undone after refusal")
	}
}

func TestAtomicityGate_AllowsClusterTaskWithAtom(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedClusterPlan())
	taskID := a.LoadPlan("gate-cluster").Phases[0].Clusters[0].Tasks[0].ID
	if _, err := a.CreateKnowledgeComponent("an atom", "body", taskID, 0); err != nil {
		t.Fatalf("create atom: %v", err)
	}
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-cluster","action":"set_done","task_index":0}`))
	if strings.Contains(out, "atomicity gate") {
		t.Fatalf("expected success with atom, got %q", out)
	}
	if !a.LoadPlan("gate-cluster").Phases[0].Clusters[0].Tasks[0].Done {
		t.Fatalf("cluster task should be done")
	}
}

// --- Bloom enforcement tests ---

func bloomIncompletePlan() *JSONPlan {
	// Missing evaluate and create
	return &JSONPlan{
		ID:   "bloom-incomplete",
		Name: "BloomIncomplete",
		Phases: []Phase{{
			Title: "Phase A",
			Tasks: []Task{
				{ID: "b-0", Title: "Understand", Done: false, BloomLevel: "understand"},
				{ID: "b-1", Title: "Apply", Done: false, BloomLevel: "apply"},
				{ID: "b-2", Title: "Analyze", Done: false, BloomLevel: "analyze"},
			},
		}},
	}
}

func bloomCompletePlan() *JSONPlan {
	return &JSONPlan{
		ID:   "bloom-complete",
		Name: "BloomComplete",
		Phases: []Phase{{
			Title: "Phase B",
			Tasks: []Task{
				{ID: "b-0", Title: "Understand", Done: false, BloomLevel: "understand"},
				{ID: "b-1", Title: "Apply", Done: false, BloomLevel: "apply"},
				{ID: "b-2", Title: "Analyze", Done: false, BloomLevel: "analyze"},
				{ID: "b-3", Title: "Evaluate", Done: false, BloomLevel: "evaluate"},
				{ID: "b-4", Title: "Create", Done: false, BloomLevel: "create"},
			},
		}},
	}
}

func TestBloomEnforcementRefusesIncompletePhase(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, bloomIncompletePlan())
	sess, _ := a.CreateSession("bloom-incomplete", "t", "study")
	for _, id := range []string{"b-0", "b-1", "b-2"} {
		a.LogConfidence(sess.ID, id, 0.8, "manual", "")
	}
	// Mark first two tasks done so the third is the last undone
	a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"bloom-incomplete","action":"set_done","task_index":0}`))
	a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"bloom-incomplete","action":"set_done","task_index":1}`))
	// Now task_index 2 is the last undone — this should trigger bloom check
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"bloom-incomplete","action":"set_done","task_index":2}`))
	if !strings.Contains(out, "cannot be completed") {
		t.Fatalf("expected bloom refusal, got %q", out)
	}
	if !strings.Contains(out, "evaluate") || !strings.Contains(out, "create") {
		t.Fatalf("refusal should name missing levels, got %q", out)
	}
	if a.LoadPlan("bloom-incomplete").Phases[0].Tasks[2].Done {
		t.Fatalf("task should remain undone after refusal")
	}
}

func TestBloomEnforcementAllowsCompletePhase(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, bloomCompletePlan())
	sess, _ := a.CreateSession("bloom-complete", "t", "study")
	for _, id := range []string{"b-0", "b-1", "b-2", "b-3", "b-4"} {
		a.LogConfidence(sess.ID, id, 0.8, "manual", "")
	}
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"bloom-complete","action":"set_done","task_index":4}`))
	if strings.Contains(out, "cannot be completed") {
		t.Fatalf("should not refuse complete phase, got %q", out)
	}
	if !a.LoadPlan("bloom-complete").Phases[0].Tasks[4].Done {
		t.Fatalf("last task should be done")
	}
}

func TestBloomEnforcementSkippedWhenUntagged(t *testing.T) {
	a := newMemoryApp(t)
	// Phase with one untagged task — enforcement should be skipped
	plan := &JSONPlan{
		ID:   "bloom-untagged",
		Name: "BloomUntagged",
		Phases: []Phase{{
			Title: "Phase C",
			Tasks: []Task{
				{ID: "u-0", Title: "Tagged", Done: false, BloomLevel: "understand"},
				{ID: "u-1", Title: "Untagged", Done: false},
			},
		}},
	}
	writePlan(t, a, plan)
	sess, _ := a.CreateSession("bloom-untagged", "t", "study")
	a.LogConfidence(sess.ID, "u-1", 0.8, "manual", "")
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"bloom-untagged","action":"set_done","task_index":1}`))
	if strings.Contains(out, "cannot be completed") {
		t.Fatalf("should skip enforcement when tasks are untagged, got %q", out)
	}
	if !a.LoadPlan("bloom-untagged").Phases[0].Tasks[1].Done {
		t.Fatalf("task should be done")
	}
}

func TestBloomEnforcementForceBypasses(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, bloomIncompletePlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"bloom-incomplete","action":"set_done","task_index":2,"force":true}`))
	if strings.Contains(out, "cannot be completed") {
		t.Fatalf("force should bypass bloom check, got %q", out)
	}
	if !a.LoadPlan("bloom-incomplete").Phases[0].Tasks[2].Done {
		t.Fatalf("task should be done with force")
	}
}

func TestBloomEnforcementUndoAlwaysAllowed(t *testing.T) {
	a := newMemoryApp(t)
	plan := bloomIncompletePlan()
	plan.Phases[0].Tasks[2].Done = true // pre-set done so undo is possible
	writePlan(t, a, plan)
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"bloom-incomplete","action":"set_undone","task_index":2}`))
	if strings.Contains(out, "cannot be completed") {
		t.Fatalf("undo should never trigger bloom check, got %q", out)
	}
	if a.LoadPlan("bloom-incomplete").Phases[0].Tasks[2].Done {
		t.Fatalf("task should be undone")
	}
}

// --- Positional/type UUID fallback tests ---

func planWithTasks(tasks ...Task) *JSONPlan {
	return &JSONPlan{ID: "p", Phases: []Phase{{Title: "P1", Tasks: tasks}}}
}

func inheritIDs(old, new *JSONPlan) {
	inheritOrGenerateIDs(new, buildTitleToIDMap(old), buildPositionalIDMap(old))
}

func TestInheritIDs_PositionalFallbackRephraseInPlace(t *testing.T) {
	old := planWithTasks(Task{ID: "keepme", Title: "🔴 **Read** — Original prose"})
	new := planWithTasks(Task{Title: "🔴 **Read** — Reworded prose"})
	inheritIDs(old, new)
	if got := new.Phases[0].Tasks[0].ID; got != "keepme" {
		t.Fatalf("rephrase-in-place did not inherit: got %q, want keepme", got)
	}
}

func TestInheritIDs_ExactTitleWinsNoDoubleAssign(t *testing.T) {
	old := planWithTasks(
		Task{ID: "a", Title: "🔴 **Read** — X"},
		Task{ID: "b", Title: "🔴 **Read** — Y"},
	)
	new := planWithTasks(
		Task{Title: "🔴 **Read** — Y"},
		Task{Title: "🔴 **Read** — Z"},
	)
	inheritIDs(old, new)
	// Task 0: exact title Y → id "b"; Task 1: no match, "b" already used → fresh.
	if got := new.Phases[0].Tasks[0].ID; got != "b" {
		t.Fatalf("task 0: got %q, want b", got)
	}
	id1 := new.Phases[0].Tasks[1].ID
	if id1 == "" || id1 == "a" || id1 == "b" {
		t.Fatalf("task 1 should be fresh, got %q", id1)
	}
	if new.Phases[0].Tasks[0].ID == id1 {
		t.Fatalf("duplicate ids: %q", id1)
	}
}

func TestInheritIDs_TypeMismatchNewID(t *testing.T) {
	old := planWithTasks(Task{ID: "a", Title: "🔴 **Read** — X"})
	new := planWithTasks(Task{Title: "🔴 **Reflect** — X"})
	inheritIDs(old, new)
	if got := new.Phases[0].Tasks[0].ID; got == "" || got == "a" {
		t.Fatalf("type mismatch: got %q, want fresh id", got)
	}
}

func TestInheritIDs_ExplicitIDPreserved(t *testing.T) {
	old := planWithTasks(Task{ID: "old-a", Title: "🔴 **Read** — X"})
	new := planWithTasks(Task{ID: "explicit", Title: "🔴 **Read** — X"})
	inheritIDs(old, new)
	if got := new.Phases[0].Tasks[0].ID; got != "explicit" {
		t.Fatalf("explicit id: got %q, want explicit", got)
	}
}

func TestTaskType(t *testing.T) {
	for _, tc := range []struct{ title, want string }{
		{"🔴 **Read** — Ch. 2", "read"},
		{"🟡 **Reflect** — Thoughts", "reflect"},
		{"🔵 **Watch** — Video", "watch"},
		{"just prose no bold", ""},
		{"", ""},
	} {
		if got := taskType(tc.title); got != tc.want {
			t.Errorf("taskType(%q) = %q, want %q", tc.title, got, tc.want)
		}
	}
}

func TestBuildPositionalIDMap(t *testing.T) {
	p := &JSONPlan{ID: "p", Phases: []Phase{
		{Title: "P1", Tasks: []Task{
			{ID: "id-read", Title: "🔴 **Read** — R1"},
			{ID: "id-watch", Title: "🔵 **Watch** — W1"},
		}},
		{Title: "P2", Tasks: []Task{
			{ID: "id-reflect", Title: "🟡 **Reflect** — F1"},
		}, Clusters: []Cluster{{
			Title: "C1", Tasks: []Task{{ID: "id-cluster", Title: "🔴 **Read** — Cluster task"}},
		}}},
	}}
	m := buildPositionalIDMap(p)
	if m["0:0:read"] != "id-read" || m["0:1:watch"] != "id-watch" || m["1:0:reflect"] != "id-reflect" {
		t.Fatalf("positional map: %v", m)
	}
	if _, ok := m["1:0:read"]; ok {
		t.Fatal("cluster task should not be in positional map")
	}
}

func TestReconcilePlanIDs(t *testing.T) {
	old := planWithTasks(Task{ID: "keepme", Title: "🔴 **Read** — Original"})
	new := planWithTasks(
		Task{Title: "🔴 **Read** — Reworded"},
		Task{Title: "🔴 **Read** — Brand new"},
	)
	plan, inherited, generated := ReconcilePlanIDs(old, new)
	if plan.Phases[0].Tasks[0].ID != "keepme" {
		t.Fatalf("expected keepme, got %q", plan.Phases[0].Tasks[0].ID)
	}
	if plan.Phases[0].Tasks[1].ID == "" || plan.Phases[0].Tasks[1].ID == "keepme" {
		t.Fatalf("second task should be fresh, got %q", plan.Phases[0].Tasks[1].ID)
	}
	if inherited != 1 || generated != 1 {
		t.Fatalf("inherited=%d generated=%d, want 1,1", inherited, generated)
	}
}
