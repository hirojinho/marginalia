package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ToolUpdatePlan handles the update_plan LLM tool: toggle, set_done, set_undone, and add_task actions.
func (a *App) ToolUpdatePlan(args json.RawMessage) string {
	var p struct {
		PlanID       string `json:"plan_id"`
		Action       string `json:"action"`
		TaskIndex    int    `json:"task_index"`
		TaskTitle    string `json:"task_title"`
		TaskPriority string `json:"task_priority"`
		Force        bool   `json:"force"`
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
		return a.applyToggle(plan, p.Action, p.TaskIndex, p.Force)
	case "add_task":
		return a.applyAddTask(plan, p.TaskTitle, p.TaskPriority)
	default:
		return "error: unknown action '" + p.Action + "'. Available: toggle, set_done, set_undone, add_task"
	}
}

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
	s, _ := a.GetCourseSettings(planID) // defaults are safe on error (GetCourseSettings returns behavior-preserving defaults)
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

// isLastUndoneInPhase returns true if task is the only undone task remaining in the phase.
// It collects all tasks from the phase (direct tasks + cluster tasks).
func isLastUndoneInPhase(phase *Phase, task *Task) bool {
	undone := 0
	for _, t := range phase.Tasks {
		if !t.Done {
			undone++
		}
	}
	for _, c := range phase.Clusters {
		for _, t := range c.Tasks {
			if !t.Done {
				undone++
			}
		}
	}
	return undone == 1 && !task.Done
}

// phaseCompletionBloomCheck returns a refusal message if completing this task
// would finish the phase without covering all required Bloom levels. Returns ""
// if the check passes or is skipped.
func phaseCompletionBloomCheck(phase *Phase) string {
	// Collect bloom levels from all tasks in the phase.
	levels := map[string]bool{}
	allTagged := true
	for _, t := range phase.Tasks {
		if t.BloomLevel == "" {
			allTagged = false
		} else {
			levels[t.BloomLevel] = true
		}
	}
	for _, c := range phase.Clusters {
		for _, t := range c.Tasks {
			if t.BloomLevel == "" {
				allTagged = false
			} else {
				levels[t.BloomLevel] = true
			}
		}
	}
	// Skip enforcement if ANY task lacks a bloom_level (backward compat).
	if !allTagged {
		return ""
	}
	required := []string{"analyze", "evaluate", "create"}
	var missing []string
	for _, r := range required {
		if !levels[r] {
			missing = append(missing, r)
		}
	}
	if len(missing) > 0 {
		return fmt.Sprintf("Phase %q cannot be completed: missing Bloom levels — %s. Add a task at each missing level then try again, or pass --force to override.",
			phase.Title, strings.Join(missing, ", "))
	}
	return ""
}

// applyToggle walks the plan tasks in sequential order and applies the action to the task at taskIndex.
func (a *App) applyToggle(plan *JSONPlan, action string, taskIndex int, force bool) string {
	count := 0
	for i := range plan.Phases {
		for j := range plan.Phases[i].Tasks {
			if count == taskIndex {
				if refusal := a.masteryGateRefusal(plan.ID, &plan.Phases[i].Tasks[j], action, force); refusal != "" {
					return refusal
				}
				completing := action == "toggle" || action == "set_done"
				if completing && !force && isLastUndoneInPhase(&plan.Phases[i], &plan.Phases[i].Tasks[j]) {
					if bloomRefusal := phaseCompletionBloomCheck(&plan.Phases[i]); bloomRefusal != "" {
						return bloomRefusal
					}
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
			if msg, found, refused := a.applyToggleCluster(plan, action, taskIndex, i, k, &count, force); found {
				if !refused {
					if err := a.SavePlan(plan); err != nil {
						return "error saving plan: " + err.Error()
					}
				}
				return msg
			}
		}
	}
	return fmt.Sprintf("error: task index %d not found (plan has %d tasks)", taskIndex, count)
}

// applyToggleCluster applies the action to a task inside a cluster, if the sequential count matches taskIndex.
func (a *App) applyToggleCluster(plan *JSONPlan, action string, taskIndex, phaseIdx, clusterIdx int, count *int, force bool) (msg string, found bool, refused bool) {
	for j := range plan.Phases[phaseIdx].Clusters[clusterIdx].Tasks {
		if *count == taskIndex {
			task := &plan.Phases[phaseIdx].Clusters[clusterIdx].Tasks[j]
			if refusal := a.masteryGateRefusal(plan.ID, task, action, force); refusal != "" {
				return refusal, true, true
			}
			completing := action == "toggle" || action == "set_done"
			if completing && !force && isLastUndoneInPhase(&plan.Phases[phaseIdx], task) {
				if bloomRefusal := phaseCompletionBloomCheck(&plan.Phases[phaseIdx]); bloomRefusal != "" {
					return bloomRefusal, true, true
				}
			}
			applyAction(&task.Done, action)
			return fmt.Sprintf("Task %d %q in cluster %q marked as %s",
				taskIndex, task.Title, plan.Phases[phaseIdx].Clusters[clusterIdx].Title, doneState(task.Done)), true, false
		}
		*count++
	}
	return "", false, false
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
		ID:       newTaskID(),
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

// ToolRewritePlan replaces the entire plan JSON for a course with new content.
// It preserves task UUIDs across rewrites when titles match exactly (case-insensitive
// after trim), so confidence/retrieval data stays anchored.
func (a *App) ToolRewritePlan(args json.RawMessage) string {
	var p struct {
		PlanID   string `json:"plan_id"`
		PlanJSON string `json:"plan_json"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.PlanID == "" {
		return "error: plan_id is required"
	}
	if p.PlanJSON == "" {
		return "error: plan_json is required"
	}

	var newPlan JSONPlan
	if err := json.Unmarshal([]byte(p.PlanJSON), &newPlan); err != nil {
		return "error: plan_json failed to parse as JSONPlan: " + err.Error()
	}
	if newPlan.ID != p.PlanID {
		return fmt.Sprintf("error: plan_json.id (%q) does not match plan_id arg (%q)", newPlan.ID, p.PlanID)
	}

	oldPlan := a.LoadPlan(p.PlanID)
	titleToID := buildTitleToIDMap(oldPlan)
	inheritOrGenerateIDs(&newPlan, titleToID)

	// Ensure the plans directory exists (needed for first-time creates)
	dir := a.VaultPath("data", "plans")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "error creating plans directory: " + err.Error()
	}
	if err := a.SavePlan(&newPlan); err != nil {
		return "error saving plan: " + err.Error()
	}
	preserved, generated := countIDOrigins(&newPlan, titleToID)
	return fmt.Sprintf("rewrote plan %q: %d tasks, %d inherited UUIDs, %d new UUIDs",
		p.PlanID, preserved+generated, preserved, generated)
}

func normalizeTitle(t string) string {
	return strings.ToLower(strings.TrimSpace(t))
}

func buildTitleToIDMap(p *JSONPlan) map[string]string {
	m := make(map[string]string)
	if p == nil {
		return m
	}
	walk := func(t Task) {
		if t.ID != "" {
			m[normalizeTitle(t.Title)] = t.ID
		}
	}
	for _, ph := range p.Phases {
		for _, t := range ph.Tasks {
			walk(t)
		}
		for _, cl := range ph.Clusters {
			for _, t := range cl.Tasks {
				walk(t)
			}
		}
	}
	return m
}

func inheritOrGenerateIDs(p *JSONPlan, titleToID map[string]string) {
	walk := func(t *Task) {
		if t.ID != "" {
			return // explicitly provided
		}
		if id, ok := titleToID[normalizeTitle(t.Title)]; ok {
			t.ID = id
		} else {
			t.ID = newTaskID()
		}
	}
	for i := range p.Phases {
		for j := range p.Phases[i].Tasks {
			walk(&p.Phases[i].Tasks[j])
		}
		for k := range p.Phases[i].Clusters {
			for j := range p.Phases[i].Clusters[k].Tasks {
				walk(&p.Phases[i].Clusters[k].Tasks[j])
			}
		}
	}
}

func countIDOrigins(p *JSONPlan, titleToID map[string]string) (preserved, generated int) {
	count := func(t Task) {
		if titleToID[normalizeTitle(t.Title)] == t.ID {
			preserved++
		} else {
			generated++
		}
	}
	for _, ph := range p.Phases {
		for _, t := range ph.Tasks {
			count(t)
		}
		for _, cl := range ph.Clusters {
			for _, t := range cl.Tasks {
				count(t)
			}
		}
	}
	return
}
