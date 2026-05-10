package agent

import (
	"encoding/json"
	"fmt"
)

func (a *App) toolUpdatePlan(args json.RawMessage) string {
	var p struct {
		PlanID       string `json:"plan_id"`
		Action       string `json:"action"`
		TaskIndex    int    `json:"task_index"`
		TaskTitle    string `json:"task_title"`
		TaskPriority string `json:"task_priority"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.PlanID == "" {
		return "error: plan_id is required"
	}

	plan := a.LoadPlan(p.PlanID)
	if plan == nil {
		return "error: plan not found: " + p.PlanID
	}

	switch p.Action {
	case "toggle", "set_done", "set_undone":
		count := 0
		for i := range plan.Phases {
			for j := range plan.Phases[i].Tasks {
				if count == p.TaskIndex {
					applyAction(&plan.Phases[i].Tasks[j].Done, p.Action)
					if err := a.SavePlan(plan); err != nil {
						return "error saving plan: " + err.Error()
					}
					return fmt.Sprintf("Task %d %q in phase %q marked as %s",
						p.TaskIndex, plan.Phases[i].Tasks[j].Title, plan.Phases[i].Title, doneState(plan.Phases[i].Tasks[j].Done))
				}
				count++
			}
			for k := range plan.Phases[i].Clusters {
				for j := range plan.Phases[i].Clusters[k].Tasks {
					if count == p.TaskIndex {
						applyAction(&plan.Phases[i].Clusters[k].Tasks[j].Done, p.Action)
						if err := a.SavePlan(plan); err != nil {
							return "error saving plan: " + err.Error()
						}
						return fmt.Sprintf("Task %d %q in cluster %q marked as %s",
							p.TaskIndex, plan.Phases[i].Clusters[k].Tasks[j].Title, plan.Phases[i].Clusters[k].Title, doneState(plan.Phases[i].Clusters[k].Tasks[j].Done))
					}
					count++
				}
			}
		}
		return fmt.Sprintf("error: task index %d not found (plan has %d tasks)", p.TaskIndex, count)

	case "add_task":
		if p.TaskTitle == "" {
			return "error: task_title is required for add_task"
		}
		if len(plan.Phases) == 0 {
			return "error: plan has no phases to add a task to"
		}
		lastPhase := &plan.Phases[len(plan.Phases)-1]
		lastPhase.Tasks = append(lastPhase.Tasks, Task{
			Title:    p.TaskTitle,
			Done:     false,
			Priority: p.TaskPriority,
		})
		if err := a.SavePlan(plan); err != nil {
			return "error saving plan: " + err.Error()
		}
		return fmt.Sprintf("Added task %q to phase %q (total tasks now: %d)",
			p.TaskTitle, lastPhase.Title, countTasksInPlan(plan))

	default:
		return "error: unknown action '" + p.Action + "'. Available: toggle, set_done, set_undone, add_task"
	}
}

func applyAction(done *bool, action string) {
	switch action {
	case "toggle":
		*done = !*done
	case "set_done":
		*done = true
	case "set_undone":
		*done = false
	}
}

func doneState(done bool) string {
	if done {
		return "done"
	}
	return "not done"
}

func countTasksInPlan(p *JSONPlan) int {
	total := 0
	for _, phase := range p.Phases {
		total += len(phase.Tasks)
		for _, cluster := range phase.Clusters {
			total += len(cluster.Tasks)
		}
	}
	return total
}
