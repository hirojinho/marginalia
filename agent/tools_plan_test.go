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
		ID:   "ce297",
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

	// force bypasses the mastery gate so this exercises toggle mechanics only
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"toggle","task_index":0,"force":true}`))
	if !strings.Contains(out, "marked as done") {
		t.Fatalf("expected toggled-to-done, got %q", out)
	}

	loaded := a.LoadPlan("ce297")
	if !loaded.Phases[0].Tasks[0].Done {
		t.Fatalf("expected Tasks[0].Done=true, plan: %+v", loaded.Phases[0].Tasks[0])
	}
}

func TestToolUpdatePlan_SetDoneOnClusterTask(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())

	// indices: 0,1 phase tasks; 2,3 cluster tasks. force bypasses the mastery gate.
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"set_done","task_index":3,"force":true}`))
	if !strings.Contains(out, "cluster") || !strings.Contains(out, "done") {
		t.Fatalf("got %q", out)
	}
	loaded := a.LoadPlan("ce297")
	if !loaded.Phases[0].Clusters[0].Tasks[1].Done {
		t.Fatalf("cluster task not marked done")
	}
}

func TestToolUpdatePlan_TaskIndexOutOfRange(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"toggle","task_index":99}`))
	if !strings.Contains(out, "not found") {
		t.Fatalf("got %q", out)
	}
}

func TestToolUpdatePlan_AddTask(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())

	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"add_task","task_title":"New thing","task_priority":"high"}`))
	if !strings.Contains(out, "Added task") {
		t.Fatalf("got %q", out)
	}
	loaded := a.LoadPlan("ce297")
	last := loaded.Phases[0].Tasks[len(loaded.Phases[0].Tasks)-1]
	if last.Title != "New thing" || last.Priority != "high" {
		t.Fatalf("task not appended correctly: %+v", last)
	}
}

func TestToolUpdatePlan_AddTaskMissingTitle(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, samplePlan())
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"add_task"}`))
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
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"floof","task_index":0}`))
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
		ID:   "ce297",
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

	newJSON := `{"id":"ce297","name":"Safety","phases":[{"title":"Foundations","tasks":[{"title":"Read Avizienis"},{"title":"Read Leveson"}],"clusters":[{"title":"STPA","tasks":[{"title":"Step 1"}]}]}]}`
	// plan_json must be a JSON string, so we need to escape the inner JSON
	escapedJSON, _ := json.Marshal(newJSON)
	out := a.ToolRewritePlan(json.RawMessage(`{"plan_id":"ce297","plan_json":` + string(escapedJSON) + `}`))
	if !strings.Contains(out, "3 inherited UUIDs") {
		t.Fatalf("expected 3 inherited UUIDs, got %q", out)
	}

	loaded := a.LoadPlan("ce297")
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
		ID:   "ce297",
		Name: "Safety",
		Phases: []Phase{
			{Title: "P1", Tasks: []Task{
				{ID: "aaa-111", Title: "Existing Task"},
			}},
		},
	}
	writePlan(t, a, old)

	newJSON := `{"id":"ce297","name":"Safety","phases":[{"title":"P1","tasks":[{"title":"Existing Task"},{"title":"Brand New Task"}]}]}`
	escapedJSON, _ := json.Marshal(newJSON)
	out := a.ToolRewritePlan(json.RawMessage(`{"plan_id":"ce297","plan_json":` + string(escapedJSON) + `}`))
	if !strings.Contains(out, "1 inherited UUIDs") || !strings.Contains(out, "1 new UUIDs") {
		t.Fatalf("expected 1 inherited + 1 new, got %q", out)
	}

	loaded := a.LoadPlan("ce297")
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
		ID:   "ce297",
		Name: "Safety",
		Phases: []Phase{
			{Title: "P1", Tasks: []Task{
				{Title: "T1"}, {Title: "T2"}, {Title: "T3"}, {Title: "T4"},
			}},
		},
	}
	writePlan(t, a, old)

	newJSON := `{"id":"ce297","name":"Safety","phases":[{"title":"P1","tasks":[{"title":"T1"},{"title":"T3"}]}]}`
	escapedJSON, _ := json.Marshal(newJSON)
	_ = a.ToolRewritePlan(json.RawMessage(`{"plan_id":"ce297","plan_json":` + string(escapedJSON) + `}`))

	loaded := a.LoadPlan("ce297")
	total := countTasksInPlan(loaded)
	if total != 2 {
		t.Fatalf("expected 2 tasks after rewrite, got %d", total)
	}
}

func TestRewritePlan_RejectsMismatchedID(t *testing.T) {
	a := newMemoryApp(t)
	newJSON := `{"id":"guitar","name":"Guitar","phases":[]}`
	escapedJSON, _ := json.Marshal(newJSON)
	out := a.ToolRewritePlan(json.RawMessage(`{"plan_id":"ce297","plan_json":` + string(escapedJSON) + `}`))
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
	if a.LoadPlan("gate-course").Phases[0].Tasks[0].Done {
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
	// LoadPlan auto-assigns IDs to empty-id tasks (assignMissingTaskIDs migration),
	// so an empty-id task never reaches the gate through ToolUpdatePlan. Exercise the
	// ungateable-empty-id contract directly on the gate helper.
	a := newMemoryApp(t)
	task := &Task{ID: "", Title: "no id", Done: false}
	if refusal := a.masteryGateRefusal("ce297", task, "set_done", false); refusal != "" {
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
					{ID: "c-0", Title: "Cluster task zero", Done: false},
				},
			}},
		}},
	}
}

func TestMasteryGate_BlocksClusterTaskWithoutConfidence(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedClusterPlan())
	// index 0 is the cluster task (no phase tasks precede it)
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-cluster","action":"set_done","task_index":0}`))
	if !strings.Contains(out, "mastery gate") {
		t.Fatalf("expected cluster-task mastery-gate refusal, got %q", out)
	}
	loaded := a.LoadPlan("gate-cluster")
	if loaded.Phases[0].Clusters[0].Tasks[0].Done {
		t.Fatalf("cluster task should remain undone after refusal")
	}
}

func TestMasteryGate_AllowsClusterTaskWithConfidence(t *testing.T) {
	a := newMemoryApp(t)
	writePlan(t, a, gatedClusterPlan())
	sess, err := a.CreateSession("gate-cluster", "t", "study")
	if err != nil {
		t.Fatalf("session: %v", err)
	}
	if _, err := a.LogConfidence(sess.ID, "c-0", 0.9, "manual", ""); err != nil {
		t.Fatalf("log: %v", err)
	}
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"gate-cluster","action":"set_done","task_index":0}`))
	if strings.Contains(out, "mastery gate") {
		t.Fatalf("expected success with confidence, got %q", out)
	}
	if !a.LoadPlan("gate-cluster").Phases[0].Clusters[0].Tasks[0].Done {
		t.Fatalf("cluster task should be done")
	}
}
