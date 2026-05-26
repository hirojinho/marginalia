package handler

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"study-app/agent"
)

func (h *Handler) handlePlan(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}

	if r.URL.Query().Get("view") == "full" {
		id := r.URL.Query().Get("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "id is required")
			return
		}
		p := h.App.LoadPlan(id)
		if p == nil {
			writeError(w, http.StatusNotFound, "plan not found")
			return
		}
		writeJSON(w, http.StatusOK, p)
		return
	}

	courses, err := h.App.ListCourses()
	if err != nil {
		writeServerError(w, "list courses", err)
		return
	}
	summaries := make([]agent.PlanSummary, 0, len(courses))
	for _, c := range courses {
		p := h.App.LoadPlan(c.ID)
		done, total := agent.CountTasks(p)
		summaries = append(summaries, agent.PlanSummary{
			ID:      c.ID,
			Name:    c.Name,
			Done:    done,
			Total:   total,
			HasPlan: p != nil,
		})
	}
	writeJSON(w, http.StatusOK, summaries)
}

func (h *Handler) handlePlanToggle(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}

	course := r.FormValue("course")
	if course == "" {
		writeError(w, http.StatusBadRequest, "course is required")
		return
	}

	taskIdx, err := strconv.Atoi(r.FormValue("index"))
	if err != nil || taskIdx < 0 {
		writeError(w, http.StatusBadRequest, "index must be a non-negative integer")
		return
	}

	p := h.App.LoadPlan(course)
	if p == nil {
		writeError(w, http.StatusNotFound, "plan not found")
		return
	}

	if !toggleTaskAt(p, taskIdx) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}

	if err := h.App.SavePlan(p); err != nil {
		writeServerError(w, "save plan", err)
		return
	}

	if err := h.App.RecordEvent(agent.Event{
		Kind:      "plan_toggle",
		CourseID:  course,
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		slog.Warn("record plan_toggle event", "err", err)
	}

	writeJSON(w, http.StatusOK, p)
}

// toggleTaskAt flips the Done flag of the task at the given linear
// index across all phases and clusters. Returns true if a task was
// found.
func toggleTaskAt(p *agent.JSONPlan, idx int) bool {
	count := 0
	for i := range p.Phases {
		for j := range p.Phases[i].Tasks {
			if count == idx {
				p.Phases[i].Tasks[j].Done = !p.Phases[i].Tasks[j].Done
				return true
			}
			count++
		}
		for k := range p.Phases[i].Clusters {
			for j := range p.Phases[i].Clusters[k].Tasks {
				if count == idx {
					p.Phases[i].Clusters[k].Tasks[j].Done = !p.Phases[i].Clusters[k].Tasks[j].Done
					return true
				}
				count++
			}
		}
	}
	return false
}
