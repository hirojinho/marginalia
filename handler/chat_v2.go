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

const piTurnTimeout = 5 * time.Minute

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

	msgCount, _ := h.App.GetMessageCount(req.SessionID)
	isFirstTurn := msgCount == 0

	h.App.LockChat()
	err = h.App.SaveMessage(req.SessionID, "user", req.Message)
	h.App.UnlockChat()
	if err != nil {
		writeServerError(w, "save user message", err)
		return
	}

	var autoSetTopic string
	if isFirstTurn && sess.Topic == "General" {
		if t := autoTopic(req.Message); t != "" {
			if err := h.App.UpdateSessionTopic(req.SessionID, t); err != nil {
				slog.Warn("auto-set session topic", "session_id", req.SessionID, "err", err)
			} else {
				autoSetTopic = t
			}
		}
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

	if autoSetTopic != "" {
		data, _ := json.Marshal(map[string]string{"topic": autoSetTopic})
		writeSSEEvent(w, flusher, "session_topic", string(data))
	}

	assistantText, assistantReasoning := streamPiTurn(events, w, flusher)

	if assistantText != "" {
		h.App.LockChat()
		err := h.App.SaveAssistantMessage(req.SessionID, assistantText, assistantReasoning)
		h.App.UnlockChat()
		if err != nil {
			slog.Error("save assistant message", "session_id", req.SessionID, "err", err)
		}
	}
}

// streamPiTurn reads PiEvents from events, writes each as an SSE frame to w,
// and returns the concatenated text and reasoning from all token/reasoning events.
// It flushes after every event so the browser receives data incrementally.
func streamPiTurn(events <-chan agent.PiEvent, w http.ResponseWriter, flusher http.Flusher) (text, reasoning string) {
	var textBuf, reasoningBuf strings.Builder
	for ev := range events {
		switch ev.Kind {
		case "token":
			textBuf.WriteString(ev.Delta)
			data, _ := json.Marshal(map[string]string{"delta": ev.Delta})
			writeSSEEvent(w, flusher, "token", string(data))
		case "reasoning":
			reasoningBuf.WriteString(ev.Delta)
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
	return textBuf.String(), reasoningBuf.String()
}

// writeSSEEvent writes one SSE frame and flushes.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
	flusher.Flush()
}

// autoTopic derives a short session title from the first user message.
// Returns "" if msg is empty. Truncates to 60 runes at a word boundary.
func autoTopic(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return ""
	}
	const maxRunes = 60
	runes := []rune(msg)
	if len(runes) <= maxRunes {
		return msg
	}
	s := string(runes[:maxRunes])
	if idx := strings.LastIndex(s, " "); idx > 20 {
		s = s[:idx]
	}
	return s + "…"
}
