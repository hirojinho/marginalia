package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"marginalia/agent"
)

func TestGetCourseSettingsReturnsDefaults(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/courses/settings?course_id=biology", nil)
	rr := httptest.NewRecorder()
	h.handleCourseSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var s agent.CourseSettings
	if err := json.NewDecoder(rr.Body).Decode(&s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.ChunkPages != 8 || !s.StopAfterTask || !s.Interleaving {
		t.Fatalf("unexpected defaults: %+v", s)
	}
}

func TestPutCourseSettingsPersistsAndValidates(t *testing.T) {
	h := newTestHandler(t)
	body := `{"course_id":"biology","framing":"exam-prep first","exam_style":"oral","chunk_pages":6,"stop_after_task":false,"interleaving":false}`
	req := httptest.NewRequest(http.MethodPut, "/api/courses/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleCourseSettings(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body=%s", rr.Code, rr.Body.String())
	}

	got, err := h.App.GetCourseSettings("biology")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Framing != "exam-prep first" || got.ChunkPages != 6 || got.StopAfterTask || got.Interleaving {
		t.Fatalf("not persisted: %+v", got)
	}
}

func TestPutCourseSettingsRejectsBadChunk(t *testing.T) {
	h := newTestHandler(t)
	body := `{"course_id":"biology","chunk_pages":999}`
	req := httptest.NewRequest(http.MethodPut, "/api/courses/settings", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleCourseSettings(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rr.Code, rr.Body.String())
	}
}
