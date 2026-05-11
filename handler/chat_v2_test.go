package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestChatV2CreatesSandboxOnFirstCall(t *testing.T) {
	h := newTestHandler(t)

	sess, err := h.App.CreateSession("ce297", "test topic")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	body, _ := json.Marshal(chatV2Request{SessionID: sess.ID, Message: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/chat-v2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleChatV2(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["sandbox"] == "" {
		t.Errorf("response missing sandbox path: %v", resp)
	}
}

func TestChatV2ReusesExistingSandbox(t *testing.T) {
	h := newTestHandler(t)

	sess, err := h.App.CreateSession("ddia", "reuse test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	call := func() string {
		body, _ := json.Marshal(chatV2Request{SessionID: sess.ID, Message: "ping"})
		req := httptest.NewRequest(http.MethodPost, "/chat-v2", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.handleChatV2(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var resp map[string]string
		_ = json.NewDecoder(w.Body).Decode(&resp)
		return resp["sandbox"]
	}

	first := call()
	second := call()
	if first != second {
		t.Errorf("sandbox path changed on second call: %q vs %q", first, second)
	}
}

func TestChatV2RejectsMissingSessionID(t *testing.T) {
	h := newTestHandler(t)
	body := []byte(`{"message":"hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/chat-v2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleChatV2(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestChatV2RejectsMethodNotPost(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/chat-v2", nil)
	w := httptest.NewRecorder()
	h.handleChatV2(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

func TestDeleteSessionRemovesSandbox(t *testing.T) {
	h := newTestHandler(t)

	sess, err := h.App.CreateSession("ce297", "deletion test")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	sandboxPath := h.App.Sandbox.Path(sess.ID)
	if _, err := h.App.Sandbox.Create(sess.ID, "", "", ""); err != nil {
		t.Fatalf("Create sandbox: %v", err)
	}

	if _, err := os.Stat(sandboxPath); err != nil {
		t.Fatalf("sandbox not created: %v", err)
	}

	if err := h.App.DeleteSession(sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, err := os.Stat(sandboxPath); !os.IsNotExist(err) {
		t.Errorf("sandbox dir still exists after session deletion")
	}
}
