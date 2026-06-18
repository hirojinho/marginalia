package agent

import (
	"testing"
)

func TestInterleaveRevisitTasks_Cadence3(t *testing.T) {
	plan := &JSONPlan{
		ID: "test-plan",
		Phases: []Phase{{
			Title: "P1",
			Tasks: []Task{
				{ID: "t1", Title: "A"},
				{ID: "t2", Title: "B"},
				{ID: "t3", Title: "C"},
				{ID: "t4", Title: "D"},
				{ID: "t5", Title: "E"},
				{ID: "t6", Title: "F"},
			},
		}},
	}

	inserted := InterleaveRevisitTasks(plan, 3)
	if inserted != 2 {
		t.Fatalf("expected inserted=2, got %d", inserted)
	}
	tasks := plan.Phases[0].Tasks
	if len(tasks) != 8 {
		t.Fatalf("expected 8 tasks, got %d", len(tasks))
	}
	revisitCount := 0
	for _, tsk := range tasks {
		if tsk.Kind == revisitKind {
			revisitCount++
		}
	}
	if revisitCount != 2 {
		t.Fatalf("expected 2 revisit tasks, got %d", revisitCount)
	}
	// First revisit at index 3 (0-indexed), title "Revisit: A"
	firstRevisit := tasks[3]
	if firstRevisit.Title != "Revisit: A" || firstRevisit.Kind != revisitKind {
		t.Fatalf("expected first revisit 'Revisit: A', got %+v", firstRevisit)
	}
	// Second revisit at index 7, title "Revisit: D"
	secondRevisit := tasks[7]
	if secondRevisit.Title != "Revisit: D" || secondRevisit.Kind != revisitKind {
		t.Fatalf("expected second revisit 'Revisit: D', got %+v", secondRevisit)
	}
}

func TestInterleaveRevisitTasks_Idempotent(t *testing.T) {
	plan := &JSONPlan{
		ID: "test-plan",
		Phases: []Phase{{
			Title: "P1",
			Tasks: []Task{
				{ID: "t1", Title: "A"},
				{ID: "t2", Title: "B"},
				{ID: "t3", Title: "C"},
				{ID: "t4", Title: "D"},
				{ID: "t5", Title: "E"},
				{ID: "t6", Title: "F"},
			},
		}},
	}

	first := InterleaveRevisitTasks(plan, 3)
	if first != 2 {
		t.Fatalf("first call: expected 2, got %d", first)
	}
	taskLen := len(plan.Phases[0].Tasks)
	second := InterleaveRevisitTasks(plan, 3)
	if second != 0 {
		t.Fatalf("second call: expected 0 (idempotent), got %d", second)
	}
	if len(plan.Phases[0].Tasks) != taskLen {
		t.Fatalf("plan mutated on idempotent re-run: %d -> %d", taskLen, len(plan.Phases[0].Tasks))
	}
}

func TestInterleaveRevisitTasks_CadenceLessThanOne(t *testing.T) {
	plan := &JSONPlan{
		ID: "test-plan",
		Phases: []Phase{{
			Title: "P1",
			Tasks: []Task{
				{ID: "t1", Title: "A"},
				{ID: "t2", Title: "B"},
			},
		}},
	}
	inserted := InterleaveRevisitTasks(plan, 0)
	if inserted != 0 {
		t.Fatalf("expected 0 for cadence=0, got %d", inserted)
	}
	if len(plan.Phases[0].Tasks) != 2 {
		t.Fatalf("plan should be unchanged, got %d tasks", len(plan.Phases[0].Tasks))
	}
}

func TestInterleaveRevisitTasks_NilPlan(t *testing.T) {
	inserted := InterleaveRevisitTasks(nil, 3)
	if inserted != 0 {
		t.Fatalf("expected 0 for nil plan, got %d", inserted)
	}
}

func TestInterleaveRevisitTasks_ZeroPhases(t *testing.T) {
	plan := &JSONPlan{ID: "empty", Phases: nil}
	inserted := InterleaveRevisitTasks(plan, 3)
	if inserted != 0 {
		t.Fatalf("expected 0 for zero phases, got %d", inserted)
	}
}
