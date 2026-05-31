package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"study-app/agent"
)

// ---------- /chat-v2 HTTP handler tests ----------

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

func TestChatV2RejectsMissingMessage(t *testing.T) {
	h := newTestHandler(t)
	sess, err := h.App.CreateSession("ce297", "test", "scratch")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	body, _ := json.Marshal(chatV2Request{SessionID: sess.ID, Message: ""})
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

func TestChatV2ReturnsPiUnconfiguredWhenNoPiPath(t *testing.T) {
	h := newTestHandler(t) // Config.PiPath is empty by default
	sess, err := h.App.CreateSession("ce297", "test", "scratch")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	body, _ := json.Marshal(chatV2Request{SessionID: sess.ID, Message: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/chat-v2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleChatV2(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestChatV2CreatesSandboxBeforeCheckingPi(t *testing.T) {
	h := newTestHandler(t) // Config.PiPath is empty — will 503
	sess, err := h.App.CreateSession("ce297", "test", "scratch")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	sandboxPath := h.App.Sandbox.Path(sess.ID)

	body, _ := json.Marshal(chatV2Request{SessionID: sess.ID, Message: "hi"})
	req := httptest.NewRequest(http.MethodPost, "/chat-v2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleChatV2(w, req)

	// Should be 503 (no Pi), but sandbox must exist.
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
	if _, err := os.Stat(sandboxPath); err != nil {
		t.Errorf("sandbox not created before Pi check: %v", err)
	}
}

func TestDeleteSessionRemovesSandbox(t *testing.T) {
	h := newTestHandler(t)

	sess, err := h.App.CreateSession("ce297", "deletion test", "scratch")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	sandboxPath := h.App.Sandbox.Path(sess.ID)
	if _, err := h.App.Sandbox.Create(sess.ID, "", "", "", "study"); err != nil {
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

// ---------- streamPiTurn unit tests ----------

func TestBuildPiPromptPassthroughWhenNoCourse(t *testing.T) {
	got := buildPiPrompt("", "/no/such/binary", "hello", nil)
	if got != "hello" {
		t.Errorf("got %q, want passthrough %q", got, "hello")
	}
}

func TestBuildPiPromptPassthroughWhenNoClawCLI(t *testing.T) {
	got := buildPiPrompt("ddia", "", "hello", nil)
	if got != "hello" {
		t.Errorf("got %q, want passthrough %q", got, "hello")
	}
}

func TestBuildPiPromptPassthroughWhenClawCLIFails(t *testing.T) {
	got := buildPiPrompt("ddia", "/no/such/binary/path", "hello", nil)
	if got != "hello" {
		t.Errorf("got %q, want passthrough on exec failure", got)
	}
}

func TestBuildPiPromptEmitsReadingState(t *testing.T) {
	rs := &readingState{PDFName: "Chapter 8 - PHI ETA", Page: 72, Total: 104}
	// No course/claw-cli, so only the reading_state block + message remain.
	got := buildPiPrompt("", "", "what's on this page?", rs)
	wantBlock := "<reading_state pdf=\"Chapter 8 - PHI ETA\" page=\"72/104\"/>"
	if !strings.Contains(got, wantBlock) {
		t.Errorf("missing reading_state block.\ngot:  %q\nwant substring: %q", got, wantBlock)
	}
	if !strings.HasSuffix(got, "what's on this page?") {
		t.Errorf("user message not preserved at end: %q", got)
	}
}

func TestStreamPiTurnEmitsKeepaliveDuringIdleGap(t *testing.T) {
	// Force a short keepalive so the test stays fast.
	orig := sseKeepaliveIntervalForTest
	sseKeepaliveIntervalForTest = 25 * time.Millisecond
	defer func() { sseKeepaliveIntervalForTest = orig }()

	events := make(chan agent.PiEvent)
	w := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		streamPiTurn(context.Background(), events, w, w)
		close(done)
	}()

	// Hold the channel open with no events to force keepalive ticks.
	time.Sleep(90 * time.Millisecond)
	close(events)
	<-done

	body := w.Body.String()
	if !strings.Contains(body, ": keepalive\n\n") {
		t.Errorf("expected keepalive comment frame, got:\n%q", body)
	}
}

func TestStreamPiTurnEmitsTokenSSE(t *testing.T) {
	events := make(chan agent.PiEvent, 2)
	events <- agent.PiEvent{Kind: "token", Delta: "hello"}
	events <- agent.PiEvent{Kind: "done"}
	close(events)

	w := httptest.NewRecorder()
	text, _, _, _ := streamPiTurn(context.Background(), events, w, w)

	body := w.Body.String()
	if !strings.Contains(body, "event: token") {
		t.Errorf("expected event: token in SSE output, got:\n%s", body)
	}
	if !strings.Contains(body, `"delta":"hello"`) {
		t.Errorf("expected delta in SSE output, got:\n%s", body)
	}
	if text != "hello" {
		t.Errorf("returned text = %q, want %q", text, "hello")
	}
}

func TestStreamPiTurnEmitsDoneSSE(t *testing.T) {
	events := make(chan agent.PiEvent, 1)
	events <- agent.PiEvent{Kind: "done", Usage: agent.PiUsage{Input: 10, Output: 5}}
	close(events)

	w := httptest.NewRecorder()
	_, _, _, _ = streamPiTurn(context.Background(), events, w, w)

	body := w.Body.String()
	if !strings.Contains(body, "event: done") {
		t.Errorf("expected event: done in SSE output, got:\n%s", body)
	}
}

func TestStreamPiTurnEmitsToolStartAndEnd(t *testing.T) {
	events := make(chan agent.PiEvent, 3)
	events <- agent.PiEvent{Kind: "tool_start", ToolName: "bash", InputSummary: `{"command":"ls"}`}
	events <- agent.PiEvent{Kind: "tool_end", ToolName: "bash", OutputSummary: "file1", OK: true}
	events <- agent.PiEvent{Kind: "done"}
	close(events)

	w := httptest.NewRecorder()
	_, _, _, _ = streamPiTurn(context.Background(), events, w, w)

	body := w.Body.String()
	if !strings.Contains(body, "event: tool_start") {
		t.Errorf("expected tool_start in output:\n%s", body)
	}
	if !strings.Contains(body, "event: tool_end") {
		t.Errorf("expected tool_end in output:\n%s", body)
	}
}

func TestStreamPiTurnEmitsErrorSSE(t *testing.T) {
	events := make(chan agent.PiEvent, 1)
	events <- agent.PiEvent{Kind: "error", Message: "pi exited without completing"}
	close(events)

	w := httptest.NewRecorder()
	_, _, _, _ = streamPiTurn(context.Background(), events, w, w)

	body := w.Body.String()
	if !strings.Contains(body, "event: error") {
		t.Errorf("expected event: error in SSE output, got:\n%s", body)
	}
}

func TestStreamPiTurnAccumulatesTokenDeltas(t *testing.T) {
	events := make(chan agent.PiEvent, 4)
	events <- agent.PiEvent{Kind: "token", Delta: "foo"}
	events <- agent.PiEvent{Kind: "token", Delta: " bar"}
	events <- agent.PiEvent{Kind: "token", Delta: " baz"}
	events <- agent.PiEvent{Kind: "done"}
	close(events)

	w := httptest.NewRecorder()
	text, _, _, _ := streamPiTurn(context.Background(), events, w, w)

	if text != "foo bar baz" {
		t.Errorf("accumulated text = %q, want %q", text, "foo bar baz")
	}
}

func TestStreamPiTurnAccumulatesReasoningDeltas(t *testing.T) {
	events := make(chan agent.PiEvent, 5)
	events <- agent.PiEvent{Kind: "reasoning", Delta: "first "}
	events <- agent.PiEvent{Kind: "token", Delta: "answer"}
	events <- agent.PiEvent{Kind: "reasoning", Delta: "second"}
	events <- agent.PiEvent{Kind: "done"}
	close(events)

	w := httptest.NewRecorder()
	text, reasoning, _, _ := streamPiTurn(context.Background(), events, w, w)

	if text != "answer" {
		t.Errorf("text = %q, want %q", text, "answer")
	}
	if reasoning != "first second" {
		t.Errorf("reasoning = %q, want %q", reasoning, "first second")
	}
}

func TestStreamPiTurnReturnsDoneUsage(t *testing.T) {
	events := make(chan agent.PiEvent, 2)
	events <- agent.PiEvent{Kind: "done", Usage: agent.PiUsage{Input: 100, Output: 40}}
	close(events)

	w := httptest.NewRecorder()
	_, _, usage, _ := streamPiTurn(context.Background(), events, w, w)

	if usage.Input != 100 || usage.Output != 40 {
		t.Errorf("usage = %+v, want Input=100 Output=40", usage)
	}
}

func TestStreamPiTurnAccumulatesToolRecords(t *testing.T) {
	events := make(chan agent.PiEvent, 4)
	events <- agent.PiEvent{Kind: "tool_start", ToolName: "rag_search"}
	events <- agent.PiEvent{Kind: "tool_end", ToolName: "rag_search", OK: true}
	events <- agent.PiEvent{Kind: "tool_end", ToolName: "read_file", OK: false}
	events <- agent.PiEvent{Kind: "done"}
	close(events)

	w := httptest.NewRecorder()
	_, _, _, tools := streamPiTurn(context.Background(), events, w, w)

	if len(tools) != 2 {
		t.Fatalf("expected 2 tool records, got %d", len(tools))
	}
	if tools[0].Name != "rag_search" || !tools[0].OK {
		t.Errorf("tools[0] = %+v", tools[0])
	}
	if tools[1].Name != "read_file" || tools[1].OK {
		t.Errorf("tools[1] = %+v", tools[1])
	}
}

func TestStreamPiTurnSSELineFormat(t *testing.T) {
	events := make(chan agent.PiEvent, 2)
	events <- agent.PiEvent{Kind: "token", Delta: "x"}
	events <- agent.PiEvent{Kind: "done"}
	close(events)

	w := httptest.NewRecorder()
	_, _, _, _ = streamPiTurn(context.Background(), events, w, w)

	// Each SSE event must be "event: <type>\ndata: <json>\n\n"
	scanner := bufio.NewScanner(strings.NewReader(w.Body.String()))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	// Find token event block.
	found := false
	for i, l := range lines {
		if l == "event: token" && i+1 < len(lines) && strings.HasPrefix(lines[i+1], "data: ") {
			found = true
		}
	}
	if !found {
		t.Errorf("SSE format wrong; got lines: %v", lines)
	}
}
