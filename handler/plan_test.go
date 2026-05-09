package handler

import (
	"testing"

	"study-app/agent"
)

func samplePlan() *agent.JSONPlan {
	return &agent.JSONPlan{
		ID:   "p1",
		Name: "test",
		Phases: []agent.Phase{
			{
				Title: "Phase 1",
				Tasks: []agent.Task{
					{Title: "T0"},
					{Title: "T1"},
				},
				Clusters: []agent.Cluster{
					{Title: "C0", Tasks: []agent.Task{{Title: "T2"}, {Title: "T3"}}},
				},
			},
			{
				Title: "Phase 2",
				Tasks: []agent.Task{{Title: "T4"}},
			},
		},
	}
}

func TestToggleTaskAtPhaseLevelTask(t *testing.T) {
	p := samplePlan()
	if !toggleTaskAt(p, 0) {
		t.Fatal("expected idx 0 to toggle")
	}
	if !p.Phases[0].Tasks[0].Done {
		t.Fatal("Phase 0 Task 0 should be Done after toggle")
	}
	// Toggle back.
	toggleTaskAt(p, 0)
	if p.Phases[0].Tasks[0].Done {
		t.Fatal("Phase 0 Task 0 should be back to !Done")
	}
}

func TestToggleTaskAtClusterTask(t *testing.T) {
	p := samplePlan()
	// Indices: 0,1 phase tasks, 2,3 cluster tasks, 4 phase 2 task.
	if !toggleTaskAt(p, 3) {
		t.Fatal("expected idx 3 to toggle")
	}
	if !p.Phases[0].Clusters[0].Tasks[1].Done {
		t.Fatalf("expected cluster task 1 toggled, plan=%+v", p)
	}
}

func TestToggleTaskAtCrossesPhases(t *testing.T) {
	p := samplePlan()
	if !toggleTaskAt(p, 4) {
		t.Fatal("expected idx 4 to toggle")
	}
	if !p.Phases[1].Tasks[0].Done {
		t.Fatal("Phase 1 Task 0 should be Done")
	}
}

func TestToggleTaskAtOutOfRange(t *testing.T) {
	p := samplePlan()
	if toggleTaskAt(p, 99) {
		t.Fatal("expected false for out-of-range idx")
	}
	if toggleTaskAt(p, -1) {
		t.Fatal("expected false for negative idx")
	}
}

func TestToggleTaskAtEmptyPlan(t *testing.T) {
	p := &agent.JSONPlan{}
	if toggleTaskAt(p, 0) {
		t.Fatal("expected false on empty plan")
	}
}
