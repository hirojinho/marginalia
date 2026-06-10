package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"study-app/agent"
)

func TestHandleKnowledgeList(t *testing.T) {
	h := newTestHandler(t)

	// Seed a few KCs.
	for i := 0; i < 3; i++ {
		title := "KC " + string(rune('A'+i))
		_, err := h.App.CreateKnowledgeComponent(title, "body "+string(rune('A'+i)), "", 0)
		if err != nil {
			t.Fatalf("seed kc %d: %v", i, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/api/knowledge", nil)
	rr := httptest.NewRecorder()
	h.handleKnowledge(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET /api/knowledge status = %d, body=%s", rr.Code, rr.Body.String())
	}

	var kcs []agent.KnowledgeComponent
	if err := json.NewDecoder(rr.Body).Decode(&kcs); err != nil {
		t.Fatalf("decode kcs: %v", err)
	}
	if len(kcs) != 3 {
		t.Fatalf("expected 3 KCs, got %d", len(kcs))
	}
	// Check all three are present.
	names := map[string]bool{}
	for _, kc := range kcs {
		names[kc.Title] = true
	}
	for _, want := range []string{"KC A", "KC B", "KC C"} {
		if !names[want] {
			t.Fatalf("missing KC %q in results", want)
		}
	}
}

func TestHandleKnowledgeCreate(t *testing.T) {
	h := newTestHandler(t)

	// Valid POST.
	body := `{"title":"Test KC","body":"This is a test."}`
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleKnowledge(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("POST /api/knowledge status = %d, body=%s", rr.Code, rr.Body.String())
	}

	var resp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode resp: %v", err)
	}
	if resp.ID == "" {
		t.Fatalf("expected non-empty id")
	}

	// Empty title should fail.
	body2 := `{"title":"","body":"something"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/knowledge", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	rr2 := httptest.NewRecorder()
	h.handleKnowledge(rr2, req2)

	if rr2.Code != http.StatusBadRequest {
		t.Fatalf("empty title POST status = %d, want 400; body=%s", rr2.Code, rr2.Body.String())
	}
}

func TestHandleKnowledgeCreateMaxLength(t *testing.T) {
	h := newTestHandler(t)

	longTitle := strings.Repeat("x", 501)
	body := `{"title":"` + longTitle + `","body":"ok"}`
	req := httptest.NewRequest(http.MethodPost, "/api/knowledge", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.handleKnowledge(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("long title POST status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
}
