package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"study-app/agent"
)

func TestHandleSessionsListEmpty(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	rr := httptest.NewRecorder()

	h.handleSessions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var got []agent.Session
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty list, got %v", got)
	}
}

func TestHandleSessionsCreateThenList(t *testing.T) {
	h := newTestHandler(t)

	body := strings.NewReader(`{"course_id":"ce297","topic":"STPA"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	rr := httptest.NewRecorder()
	h.handleSessions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var created agent.Session
	if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.ID == 0 || created.CourseID != "ce297" || created.Topic != "STPA" {
		t.Fatalf("unexpected created session: %+v", created)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	rr2 := httptest.NewRecorder()
	h.handleSessions(rr2, req2)

	var listed []agent.Session
	if err := json.NewDecoder(rr2.Body).Decode(&listed); err != nil {
		t.Fatalf("decode listed: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != created.ID {
		t.Fatalf("expected 1 session matching created, got %+v", listed)
	}
}

func TestHandleSessionsCreateInvalidJSON(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", strings.NewReader("not json"))
	rr := httptest.NewRecorder()
	h.handleSessions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleSessionsDeleteRequiresID(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodDelete, "/api/sessions", nil)
	rr := httptest.NewRecorder()
	h.handleSessions(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleSessionsMethodNotAllowed(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPut, "/api/sessions", nil)
	rr := httptest.NewRecorder()
	h.handleSessions(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestHandleSessionActiveDefaultsNull(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/active", nil)
	rr := httptest.NewRecorder()
	h.handleSessionActive(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var got map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["id"] != nil {
		t.Fatalf("expected id=null, got %v", got["id"])
	}
}

func TestHandleSessionActiveSetNonexistent(t *testing.T) {
	h := newTestHandler(t)
	body := strings.NewReader(`{"id":9999}`)
	req := httptest.NewRequest(http.MethodPut, "/api/sessions/active", body)
	rr := httptest.NewRecorder()
	h.handleSessionActive(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleSessionMessagesRequiresSessionID(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/messages", nil)
	rr := httptest.NewRecorder()
	h.handleSessionMessages(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rr.Code)
	}
}

func TestHandleSessionMessagesEmptyForNewSession(t *testing.T) {
	h := newTestHandler(t)
	s, err := h.App.CreateSession("ce297", "STPA")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	url := "/api/sessions/messages?session_id=" + jsonInt(s.ID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	h.handleSessionMessages(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if strings.TrimSpace(rr.Body.String()) != "[]" {
		t.Fatalf("expected empty array, got %q", rr.Body.String())
	}
}

func jsonInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
