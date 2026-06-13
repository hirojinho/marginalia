package agent

import (
	"os"
	"strings"
)

// TaskRef identifies a plan task by its course and human-readable title.
type TaskRef struct {
	CourseID string
	Title    string
}

// ListPlanCourseIDs returns the course id of every plan file under data/plans
// (base filename without ".json"), skipping directories and backups like
// "ce297.json.bak-...".
func (a *App) ListPlanCourseIDs() ([]string, error) {
	entries, err := os.ReadDir(a.VaultPath("data", "plans"))
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") || strings.Count(name, ".") != 1 {
			continue
		}
		ids = append(ids, strings.TrimSuffix(name, ".json"))
	}
	return ids, nil
}

// BuildTaskTitleIndex maps every plan task id to its TaskRef across all plans.
func (a *App) BuildTaskTitleIndex() (map[string]TaskRef, error) {
	ids, err := a.ListPlanCourseIDs()
	if err != nil {
		return nil, err
	}
	idx := make(map[string]TaskRef)
	add := func(courseID string, tasks []Task) {
		for _, tk := range tasks {
			if tk.ID != "" {
				idx[tk.ID] = TaskRef{CourseID: courseID, Title: tk.Title}
			}
		}
	}
	for _, courseID := range ids {
		plan := a.LoadPlan(courseID)
		if plan == nil {
			continue
		}
		for _, ph := range plan.Phases {
			add(courseID, ph.Tasks)
			for _, c := range ph.Clusters {
				add(courseID, c.Tasks)
			}
		}
	}
	return idx, nil
}
