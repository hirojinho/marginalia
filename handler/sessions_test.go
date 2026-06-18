package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"marginalia/agent"
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

	body := strings.NewReader(`{"course_id":"biology","topic":"STPA"}`)
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
	if created.ID == 0 || created.CourseID != "biology" || created.Topic != "STPA" {
		t.Fatalf("unexpected created session: %+v", created)
	}
	if created.Mode != "scratch" {
		t.Fatalf("expected default mode scratch, got %q", created.Mode)
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
	s, err := h.App.CreateSession("biology", "STPA", "scratch")
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

func TestHandleSessionMessagesIncludesReasoning(t *testing.T) {
	h := newTestHandler(t)
	s, err := h.App.CreateSession("biology", "STPA", "scratch")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}
	if err := h.App.SaveAssistantMessage(s.ID, "here is my answer", "step by step thinking"); err != nil {
		t.Fatalf("save: %v", err)
	}

	url := "/api/sessions/messages?session_id=" + jsonInt(s.ID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	h.handleSessionMessages(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	var msgs []agent.Message
	if err := json.NewDecoder(rr.Body).Decode(&msgs); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Reasoning != "step by step thinking" {
		t.Errorf("reasoning = %q, want %q", msgs[0].Reasoning, "step by step thinking")
	}
}

func TestHandleSessionsCreateRecordsSessionCreateEvent(t *testing.T) {
	h := newTestHandler(t)
	body := strings.NewReader(`{"course_id":"biology","topic":"STPA"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	rr := httptest.NewRecorder()
	h.handleSessions(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var created agent.Session
	_ = json.NewDecoder(rr.Body).Decode(&created)

	evs, err := h.App.ListRecentEvents(10)
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	var found bool
	for _, e := range evs {
		if e.Kind == "session_create" && e.CourseID == "biology" &&
			e.SessionID != nil && *e.SessionID == created.ID {
			found = true
		}
	}
	if !found {
		t.Errorf("no session_create event found for session %d; events: %+v", created.ID, evs)
	}
}

func TestHandleSessionStatsEmpty(t *testing.T) {
	h := newTestHandler(t)

	var created agent.Session
	{
		body := strings.NewReader(`{"course_id":"biology","topic":"STPA"}`)
		req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
		rr := httptest.NewRecorder()
		h.handleSessions(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("create status = %d, body=%s", rr.Code, rr.Body.String())
		}
		if err := json.NewDecoder(rr.Body).Decode(&created); err != nil {
			t.Fatalf("decode created: %v", err)
		}
	}

	url := "/api/sessions/stats?id=" + jsonInt(created.ID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	h.getSessionStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var stats agent.SessionStats
	if err := json.NewDecoder(rr.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.MessageCount != 0 {
		t.Errorf("message_count = %d, want 0", stats.MessageCount)
	}
	if stats.UserMessageCount != 0 {
		t.Errorf("user_message_count = %d, want 0", stats.UserMessageCount)
	}
	if stats.AssistantMessageCount != 0 {
		t.Errorf("assistant_message_count = %d, want 0", stats.AssistantMessageCount)
	}
	if stats.TotalReasoningChars != 0 {
		t.Errorf("total_reasoning_chars = %d, want 0", stats.TotalReasoningChars)
	}
	if stats.FirstMessageAt != nil {
		t.Errorf("first_message_at = %v, want nil", stats.FirstMessageAt)
	}
	if stats.LastMessageAt != nil {
		t.Errorf("last_message_at = %v, want nil", stats.LastMessageAt)
	}
}

func TestHandleSessionStatsWithMessages(t *testing.T) {
	h := newTestHandler(t)

	s, err := h.App.CreateSession("biology", "STPA", "scratch")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := h.App.SaveMessage(s.ID, "user", "hi"); err != nil {
		t.Fatalf("save user msg 1: %v", err)
	}
	if err := h.App.SaveMessage(s.ID, "user", "hello"); err != nil {
		t.Fatalf("save user msg 2: %v", err)
	}
	if err := h.App.SaveAssistantMessage(s.ID, "answer", "thinking-1234"); err != nil {
		t.Fatalf("save assistant msg: %v", err)
	}

	url := "/api/sessions/stats?id=" + jsonInt(s.ID)
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	h.getSessionStats(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	var stats agent.SessionStats
	if err := json.NewDecoder(rr.Body).Decode(&stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.SessionID != s.ID {
		t.Errorf("session_id = %d, want %d", stats.SessionID, s.ID)
	}
	if stats.MessageCount != 3 {
		t.Errorf("message_count = %d, want 3", stats.MessageCount)
	}
	if stats.UserMessageCount != 2 {
		t.Errorf("user_message_count = %d, want 2", stats.UserMessageCount)
	}
	if stats.AssistantMessageCount != 1 {
		t.Errorf("assistant_message_count = %d, want 1", stats.AssistantMessageCount)
	}
	if stats.TotalReasoningChars != len("thinking-1234") {
		t.Errorf("total_reasoning_chars = %d, want %d", stats.TotalReasoningChars, len("thinking-1234"))
	}
	if stats.FirstMessageAt == nil {
		t.Errorf("first_message_at is nil, want non-nil")
	}
	if stats.LastMessageAt == nil {
		t.Errorf("last_message_at is nil, want non-nil")
	}
}

func TestHandleSessionStatsNotFound(t *testing.T) {
	h := newTestHandler(t)

	url := "/api/sessions/stats?id=999999"
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rr := httptest.NewRecorder()
	h.getSessionStats(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", rr.Code, rr.Body.String())
	}
	var body map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["error"] != "session not found" {
		t.Errorf("error = %q, want 'session not found'", body["error"])
	}
}

func TestHandleSessionStatsMissingID(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/stats", nil)
	rr := httptest.NewRecorder()
	h.getSessionStats(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}

func TestHandleSessionForTask_GetThenCreate(t *testing.T) {
	h := newTestHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/sessions/for-task?course_id=cs101&task_id=t-1", nil)
	rec := httptest.NewRecorder()
	h.handleSessionForTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); !strings.Contains(got, `"id":null`) {
		t.Fatalf("GET body = %s, want {\"id\":null}", got)
	}

	body := strings.NewReader(`{"course_id":"cs101","task_id":"t-1","topic":"Systems 3.3"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/for-task", body)
	rec = httptest.NewRecorder()
	h.handleSessionForTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d, want 200", rec.Code)
	}
	var created agent.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.TaskID == nil || *created.TaskID != "t-1" {
		t.Fatalf("created.TaskID = %v, want t-1", created.TaskID)
	}

	body = strings.NewReader(`{"course_id":"cs101","task_id":"t-1","topic":"ignored"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/for-task", body)
	rec = httptest.NewRecorder()
	h.handleSessionForTask(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("second POST status = %d, want 200", rec.Code)
	}
	var second agent.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if second.ID != created.ID {
		t.Errorf("second POST id = %d, want same as first %d", second.ID, created.ID)
	}
}

func TestHandleSessionForTask_MissingParams(t *testing.T) {
	h := newTestHandler(t)
	// GET without course_id
	req := httptest.NewRequest(http.MethodGet, "/api/sessions/for-task?task_id=t-1", nil)
	rec := httptest.NewRecorder()
	h.handleSessionForTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("GET missing course_id: status = %d, want 400", rec.Code)
	}
	// POST without task_id
	req = httptest.NewRequest(http.MethodPost, "/api/sessions/for-task", strings.NewReader(`{"course_id":"cs101"}`))
	rec = httptest.NewRecorder()
	h.handleSessionForTask(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST missing task_id: status = %d, want 400", rec.Code)
	}
}

func TestCreateSessionWithTaskID(t *testing.T) {
	h := newTestHandler(t)
	body := strings.NewReader(`{"course_id":"cs101","task_id":"t-9","topic":"anchored"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	rec := httptest.NewRecorder()
	h.handleSessions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var s agent.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.TaskID == nil || *s.TaskID != "t-9" {
		t.Errorf("TaskID = %v, want t-9", s.TaskID)
	}

	// A second POST with the same course_id+task_id must return the SAME
	// session (get-or-create), not create a duplicate (1:1 invariant).
	body = strings.NewReader(`{"course_id":"cs101","task_id":"t-9","topic":"ignored"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/sessions", body)
	rec = httptest.NewRecorder()
	h.handleSessions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("second POST status = %d, want 200", rec.Code)
	}
	var second agent.Session
	if err := json.Unmarshal(rec.Body.Bytes(), &second); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if second.ID != s.ID {
		t.Errorf("second POST id = %d, want same as first %d (duplicate created)", second.ID, s.ID)
	}
}

func jsonInt(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
