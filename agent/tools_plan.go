package agent

import (
	"encoding/json"
	"fmt"
)

// ToolUpdatePlan handles the update_plan LLM tool: toggle, set_done, set_undone, and add_task actions.
func (a *App) ToolUpdatePlan(args json.RawMessage) string {
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
		return a.applyToggle(plan, p.Action, p.TaskIndex)
	case "add_task":
		return a.applyAddTask(plan, p.TaskTitle, p.TaskPriority)
	default:
		return "error: unknown action '" + p.Action + "'. Available: toggle, set_done, set_undone, add_task"
	}
}

// applyToggle walks the plan tasks in sequential order and applies the action to the task at taskIndex.
func (a *App) applyToggle(plan *JSONPlan, action string, taskIndex int) string {
	count := 0
	for i := range plan.Phases {
		for j := range plan.Phases[i].Tasks {
			if count == taskIndex {
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
			if msg, found := applyToggleCluster(plan, action, taskIndex, i, k, &count); found {
				if err := a.SavePlan(plan); err != nil {
					return "error saving plan: " + err.Error()
				}
				return msg
			}
		}
	}
	return fmt.Sprintf("error: task index %d not found (plan has %d tasks)", taskIndex, count)
}

// applyToggleCluster applies the action to a task inside a cluster, if the sequential count matches taskIndex.
func applyToggleCluster(plan *JSONPlan, action string, taskIndex, phaseIdx, clusterIdx int, count *int) (string, bool) {
	for j := range plan.Phases[phaseIdx].Clusters[clusterIdx].Tasks {
		if *count == taskIndex {
			applyAction(&plan.Phases[phaseIdx].Clusters[clusterIdx].Tasks[j].Done, action)
			return fmt.Sprintf("Task %d %q in cluster %q marked as %s",
				taskIndex,
				plan.Phases[phaseIdx].Clusters[clusterIdx].Tasks[j].Title,
				plan.Phases[phaseIdx].Clusters[clusterIdx].Title,
				doneState(plan.Phases[phaseIdx].Clusters[clusterIdx].Tasks[j].Done),
			), true
		}
		*count++
	}
	return "", false
}

// applyAddTask appends a new task to the last phase of the plan.
func (a *App) applyAddTask(plan *JSONPlan, title, priority string) string {
	if title == "" {
		return "error: task_title is required for add_task"
	}
	if len(plan.Phases) == 0 {
		return "error: plan has no phases to add a task to"
	}
	lastPhase := &plan.Phases[len(plan.Phases)-1]
	lastPhase.Tasks = append(lastPhase.Tasks, Task{
		Title:    title,
		Done:     false,
		Priority: priority,
	})
	if err := a.SavePlan(plan); err != nil {
		return "error saving plan: " + err.Error()
	}
	return fmt.Sprintf("Added task %q to phase %q (total tasks now: %d)",
		title, lastPhase.Title, countTasksInPlan(plan))
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
