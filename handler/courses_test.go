package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"study-app/agent"
)

func TestCreateCourseAndGetPlan(t *testing.T) {
	h := newTestHandler(t)

	// Create a new course
	body := `{"id":"test-course","name":"Test Course"}`
	req := httptest.NewRequest(http.MethodPost, "/api/courses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleCourses(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/courses status = %d, body=%s", rr.Code, rr.Body.String())
	}

	var course agent.Course
	if err := json.NewDecoder(rr.Body).Decode(&course); err != nil {
		t.Fatalf("decode course: %v", err)
	}
	if course.ID != "test-course" || course.Name != "Test Course" {
		t.Fatalf("unexpected course: %+v", course)
	}

	// Confirm it shows in GET /api/plan
	req2 := httptest.NewRequest(http.MethodGet, "/api/plan", nil)
	rr2 := httptest.NewRecorder()
	h.handlePlan(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("GET /api/plan status = %d", rr2.Code)
	}
	var summaries []agent.PlanSummary
	if err := json.NewDecoder(rr2.Body).Decode(&summaries); err != nil {
		t.Fatalf("decode summaries: %v", err)
	}
	found := false
	for _, s := range summaries {
		if s.ID == "test-course" && s.Name == "Test Course" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("created course not found in /api/plan summaries: %+v", summaries)
	}
}

func TestCreateCourseDuplicateReturns409(t *testing.T) {
	h := newTestHandler(t)

	body := `{"id":"dup-course","name":"Dup Course"}`
	req := httptest.NewRequest(http.MethodPost, "/api/courses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleCourses(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("first POST status = %d", rr.Code)
	}

	// Duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/api/courses", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.handleCourses(rr2, req2)

	if rr2.Code != http.StatusConflict {
		t.Fatalf("duplicate POST status = %d, want 409; body=%s", rr2.Code, rr2.Body.String())
	}
}

func TestCreateCourseInvalidIDReturns400(t *testing.T) {
	h := newTestHandler(t)

	body := `{"id":"Has Space","name":"Bad ID Course"}`
	req := httptest.NewRequest(http.MethodPost, "/api/courses", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleCourses(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("invalid ID POST status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}
