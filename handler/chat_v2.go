// Package handler — chat_v2.go implements POST /chat-v2, the Pi-backed
// agent chat endpoint. It creates/reuses the per-session sandbox, acquires
// a per-session concurrency lock, spawns a Pi RPC subprocess, translates
// Pi's stdout JSONL events into SSE, and persists the assistant reply.
package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"study-app/agent"
)

const piTurnTimeout = 60 * time.Second

type chatV2Request struct {
	SessionID int64  `json:"session_id"`
	Message   string `json:"message"`
}

type sseToolEndPayload struct {
	Name          string `json:"name"`
	OutputSummary string `json:"output_summary"`
	OK            bool   `json:"ok"`
}

type sseDonePayload struct {
	Usage agent.PiUsage `json:"usage"`
}

// handleChatV2 handles POST /chat-v2. It creates or reuses the per-session
// Pi sandbox, spawns a Pi RPC subprocess, translates events to SSE, and
// persists the assistant reply. Returns 503 when PI_PATH is not configured.
func (h *Handler) handleChatV2(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}

	var req chatV2Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.SessionID <= 0 {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if req.Message == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}

	sess, err := h.App.GetSession(req.SessionID)
	if err != nil {
		writeServerError(w, "get session", err)
		return
	}

	sandboxPath, err := h.App.Sandbox.Create(
		req.SessionID,
		h.App.Config.ClawCLIPath,
		sess.CourseID,
		h.App.Config.UserID,
	)
	if err != nil {
		writeServerError(w, "create sandbox", err)
		return
	}

	if h.App.Config.PiPath == "" {
		writeError(w, http.StatusServiceUnavailable, "Pi agent not configured (set PI_PATH)")
		return
	}

	if !h.App.AcquirePiLock(req.SessionID) {
		writeError(w, http.StatusConflict, "session already has an active Pi turn")
		return
	}
	defer h.App.ReleasePiLock(req.SessionID)

	if err := h.App.SaveMessage(req.SessionID, "user", req.Message); err != nil {
		writeServerError(w, "save user message", err)
		return
	}

	model := h.App.Config.AgentModel
	if model == "" {
		model = h.App.Config.Model
	}

	ctx, cancel := context.WithTimeout(r.Context(), piTurnTimeout)
	defer cancel()

	events, err := agent.RunPi(ctx, sandboxPath, req.Message, model, h.App.Config.PiPath, h.App.Config.SkillsDir, h.App.Config.APIKey)
	if err != nil {
		writeServerError(w, "start pi", err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	assistantText := streamPiTurn(events, w, flusher)

	if assistantText != "" {
		if err := h.App.SaveMessage(req.SessionID, "assistant", assistantText); err != nil {
			slog.Error("save assistant message", "session_id", req.SessionID, "err", err)
		}
	}
}

// streamPiTurn reads PiEvents from events, writes each as an SSE frame to w,
// and returns the concatenated text from all token events. It flushes after
// every event so the browser receives data incrementally.
func streamPiTurn(events <-chan agent.PiEvent, w http.ResponseWriter, flusher http.Flusher) string {
	var sb strings.Builder
	for ev := range events {
		switch ev.Kind {
		case "token":
			sb.WriteString(ev.Delta)
			data, _ := json.Marshal(map[string]string{"delta": ev.Delta})
			writeSSEEvent(w, flusher, "token", string(data))
		case "reasoning":
			data, _ := json.Marshal(map[string]string{"delta": ev.Delta})
			writeSSEEvent(w, flusher, "reasoning", string(data))
		case "tool_start":
			data, _ := json.Marshal(map[string]string{"name": ev.ToolName, "input_summary": ev.InputSummary})
			writeSSEEvent(w, flusher, "tool_start", string(data))
		case "tool_end":
			payload := sseToolEndPayload{Name: ev.ToolName, OutputSummary: ev.OutputSummary, OK: ev.OK}
			data, _ := json.Marshal(payload)
			writeSSEEvent(w, flusher, "tool_end", string(data))
		case "skill_start":
			data, _ := json.Marshal(map[string]string{"name": ev.SkillName})
			writeSSEEvent(w, flusher, "skill_start", string(data))
		case "compaction":
			data, _ := json.Marshal(map[string]string{"reason": ev.Reason})
			writeSSEEvent(w, flusher, "compaction", string(data))
		case "model_change":
			data, _ := json.Marshal(map[string]string{"from": ev.From, "to": ev.To})
			writeSSEEvent(w, flusher, "model_change", string(data))
		case "done":
			payload := sseDonePayload{Usage: ev.Usage}
			data, _ := json.Marshal(payload)
			writeSSEEvent(w, flusher, "done", string(data))
		case "error":
			data, _ := json.Marshal(map[string]string{"message": ev.Message})
			writeSSEEvent(w, flusher, "error", string(data))
		}
	}
	return sb.String()
}

// writeSSEEvent writes one SSE frame and flushes.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
	flusher.Flush()
}
