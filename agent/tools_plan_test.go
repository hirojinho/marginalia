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

	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"toggle","task_index":0}`))
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

	// indices: 0,1 phase tasks; 2,3 cluster tasks
	out := a.ToolUpdatePlan(json.RawMessage(`{"plan_id":"ce297","action":"set_done","task_index":3}`))
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
