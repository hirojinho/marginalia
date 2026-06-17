package agent

import "fmt"

const revisitKind = "revisit"

// InterleaveRevisitTasks weaves spaced-retrieval "revisit" tasks into the plan's
// phase task lists. Counting only new-content tasks (Kind != revisitKind) across the
// whole plan, after every `cadence`-th new-content task it inserts one revisit task
// pointing back at the earliest task of the block just completed.
//
// Idempotent under a fixed cadence: if a revisit task already immediately follows the
// cadence boundary, no new one is inserted (re-running returns 0). Existing revisit
// tasks reset the running counter and are never counted as new content.
//
// Only Phase.Tasks are processed; tasks nested in Phase.Clusters are left untouched
// in v1. cadence < 1 is a no-op. Returns the number of revisit tasks inserted.
func InterleaveRevisitTasks(plan *JSONPlan, cadence int) int {
	if plan == nil || cadence < 1 {
		return 0
	}
	inserted := 0
	sinceLast := 0
	var newContent []Task // new-content tasks seen so far, in plan order

	for pi := range plan.Phases {
		tasks := plan.Phases[pi].Tasks
		rebuilt := make([]Task, 0, len(tasks)+1)
		for i := 0; i < len(tasks); i++ {
			t := tasks[i]
			if t.Kind == revisitKind {
				rebuilt = append(rebuilt, t)
				sinceLast = 0
				continue
			}
			rebuilt = append(rebuilt, t)
			newContent = append(newContent, t)
			sinceLast++
			if sinceLast == cadence {
				// Peek: if a revisit already follows in the original list, leave it
				// to the existing revisit (idempotency); otherwise insert one now.
				if i+1 < len(tasks) && tasks[i+1].Kind == revisitKind {
					// existing revisit will reset the counter next iteration
				} else {
					target := newContent[len(newContent)-cadence]
					rebuilt = append(rebuilt, Task{
						ID:    newTaskID(),
						Title: "Revisit: " + target.Title,
						Kind:  revisitKind,
						Notes: "Interleaved spaced retrieval of an earlier task.",
					})
					inserted++
					sinceLast = 0
				}
			}
		}
		plan.Phases[pi].Tasks = rebuilt
	}
	return inserted
}

// InterleavePlan loads a course's plan, inserts revisit tasks at the given cadence,
// persists the result, and returns the updated plan plus the count inserted.
func (a *App) InterleavePlan(courseID string, cadence int) (*JSONPlan, int, error) {
	plan := a.LoadPlan(courseID)
	if plan == nil {
		return nil, 0, fmt.Errorf("plan not found: %s", courseID)
	}
	n := InterleaveRevisitTasks(plan, cadence)
	if err := a.SavePlan(plan); err != nil {
		return nil, 0, err
	}
	return plan, n, nil
}
