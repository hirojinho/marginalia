package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-app/agent"
)

func writePlan(t *testing.T, h *Handler, p *agent.JSONPlan) {
	t.Helper()
	dir := h.App.VaultPath("data", "plans")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, _ := json.MarshalIndent(p, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, p.ID+".json"), data, 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
}

func TestHandlePlanListSummaries(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rr := httptest.NewRecorder()
	h.handlePlan(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var got []agent.PlanSummary
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != len(agent.KnownCourses) {
		t.Fatalf("expected %d summaries, got %d", len(agent.KnownCourses), len(got))
	}
	for _, s := range got {
		if s.HasPlan {
			t.Fatalf("expected HasPlan=false for empty vault, got %+v", s)
		}
	}
}

func TestHandlePlanFullViewMissingID(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/plan?view=full", nil)
	rr := httptest.NewRecorder()
	h.handlePlan(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandlePlanFullViewNotFound(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/plan?view=full&id=nonsense", nil)
	rr := httptest.NewRecorder()
	h.handlePlan(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestHandlePlanToggleMissingCourse(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/plan/toggle", strings.NewReader("index=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.handlePlanToggle(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandlePlanToggleBadIndex(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/plan/toggle", strings.NewReader("course=ce297&index=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.handlePlanToggle(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandlePlanToggleNoPlanFile(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/plan/toggle", strings.NewReader("course=ce297&index=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.handlePlanToggle(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rr.Code)
	}
}

func TestHandlePlanToggleSuccess(t *testing.T) {
	h := newTestHandler(t)
	plan := &agent.JSONPlan{
		ID:   "ce297",
		Name: "test",
		Phases: []agent.Phase{
			{Title: "P1", Tasks: []agent.Task{{Title: "T0"}, {Title: "T1"}}},
		},
	}
	writePlan(t, h, plan)

	req := httptest.NewRequest(http.MethodPost, "/api/plan/toggle", strings.NewReader("course=ce297&index=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.handlePlanToggle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var got agent.JSONPlan
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Phases[0].Tasks[1].Done {
		t.Fatalf("expected Task 1 toggled Done; got %+v", got)
	}

	persisted := h.App.LoadPlan("ce297")
	if persisted == nil || !persisted.Phases[0].Tasks[1].Done {
		t.Fatalf("expected toggle persisted to disk; got %+v", persisted)
	}
}

func TestHandlePlanToggleRecordsPlanToggleEvent(t *testing.T) {
	h := newTestHandler(t)
	writePlan(t, h, &agent.JSONPlan{
		ID:   "ce297",
		Name: "CE-297",
		Phases: []agent.Phase{{
			Title: "Phase 1",
			Tasks: []agent.Task{{Title: "Read chapter 1", Done: false}},
		}},
	})

	body := strings.NewReader("course=ce297&index=0")
	req := httptest.NewRequest(http.MethodPost, "/api/plan/toggle", body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.handlePlanToggle(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	evs, err := h.App.ListRecentEvents(10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var found bool
	for _, e := range evs {
		if e.Kind == "plan_toggle" && e.CourseID == "ce297" {
			found = true
		}
	}
	if !found {
		t.Errorf("no plan_toggle event found; events: %+v", evs)
	}
}
