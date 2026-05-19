// Package handler — chat_v2.go implements POST /chat-v2, the Pi-backed
// agent chat endpoint. It creates/reuses the per-session sandbox, acquires
// a per-session concurrency lock, spawns a Pi RPC subprocess, translates
// Pi's stdout JSONL events into SSE, and persists the assistant reply.
package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"study-app/agent"
)

const piTurnTimeout = 10 * time.Minute

// sseKeepaliveIntervalForTest bounds idle gaps on the SSE stream so intermediaries
// (Cloudflare tunnel, browser, proxies) don't reap the connection during long
// LLM/tool waits. Comment frames (": ...\n\n") are ignored by SSE clients.
// Var (not const) so tests can shorten it without sleeping.
//
//nolint:gochecknoglobals // test override for the keepalive interval; behaves as a const in production
var sseKeepaliveIntervalForTest = 15 * time.Second

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

// toolUseRecord captures a single tool invocation outcome for event logging.
type toolUseRecord struct {
	Name string
	OK   bool
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

	acquired, lockAge := h.App.AcquirePiLock(req.SessionID)
	if !acquired {
		slog.Warn("pi lock conflict", "session_id", req.SessionID, "existing_lock_age", lockAge.String())
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

	piPrompt := buildPiPrompt(sess.CourseID, h.App.Config.ClawCLIPath, req.Message)
	events, err := agent.RunPi(ctx, sandboxPath, piPrompt, model, h.App.Config.PiPath, h.App.Config.SkillsDir, h.App.Config.APIKey)
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

	turnStart := time.Now()
	assistantText, assistantReasoning, piUsage, piTools := streamPiTurn(ctx, events, w, flusher)
	durationMs := time.Since(turnStart).Milliseconds()

	if assistantText == "" && assistantReasoning == "" {
		slog.Warn("pi turn produced empty response", "session_id", req.SessionID, "ctx_err", ctx.Err())
	}

	if assistantText != "" {
		h.App.LockChat()
		err := h.App.SaveAssistantMessage(req.SessionID, assistantText, assistantReasoning)
		h.App.UnlockChat()
		if err != nil {
			slog.Error("save assistant message", "session_id", req.SessionID, "err", err)
		}
	}

	sessID := req.SessionID
	go func() {
		if err := h.App.RecordEvent(agent.Event{
			Kind:         "chat_turn",
			SessionID:    &sessID,
			CourseID:     sess.CourseID,
			Model:        model,
			InputTokens:  piUsage.Input,
			OutputTokens: piUsage.Output,
			DurationMs:   durationMs,
			CreatedAt:    time.Now().UnixMilli(),
		}); err != nil {
			slog.Warn("record chat_turn event", "err", err)
		}
		for _, tr := range piTools {
			ok := tr.OK
			if err := h.App.RecordEvent(agent.Event{
				Kind:      "tool_use",
				SessionID: &sessID,
				ToolName:  tr.Name,
				OK:        &ok,
				CreatedAt: time.Now().UnixMilli(),
			}); err != nil {
				slog.Warn("record tool_use event", "err", err)
			}
		}
	}()
}

// streamPiTurn reads PiEvents from events, writes each as an SSE frame to w,
// and returns the concatenated text and reasoning from all token/reasoning events,
// the token usage from the done event, and a record of every tool invocation.
// It flushes after every event so the browser receives data incrementally.
func streamPiTurn(ctx context.Context, events <-chan agent.PiEvent, w http.ResponseWriter, flusher http.Flusher) (text, reasoning string, usage agent.PiUsage, tools []toolUseRecord) {
	var textBuf, reasoningBuf strings.Builder
	ticker := time.NewTicker(sseKeepaliveIntervalForTest)
	defer ticker.Stop()
	var sawContent bool
	for {
		select {
		case <-ctx.Done():
			// If the deadline fired (vs. client disconnect), surface a friendly
			// SSE error before returning. Writes to a disconnected client will
			// silently fail, which is fine.
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				writeSSEEvent(w, flusher, "error", `{"message":"Turn timed out (10 min). The agent ran out of time mid-task. Try a smaller scoped request, or start a fresh session if context has grown large."}`)
			}
			return textBuf.String(), reasoningBuf.String(), usage, tools
		case <-ticker.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
			continue
		case ev, ok := <-events:
			if !ok {
				return textBuf.String(), reasoningBuf.String(), usage, tools
			}
			sawContent = handlePiEvent(ev, w, flusher, &textBuf, &reasoningBuf, &tools, &usage, sawContent)
		}
	}
}

// handlePiEvent translates one PiEvent to SSE output, accumulates text /
// reasoning / tool records / usage via the supplied pointers, and returns
// the updated sawContent flag (true once any token or reasoning has flowed).
// Extracted from streamPiTurn to keep that function under the cyclomatic
// complexity bound.
func handlePiEvent(
	ev agent.PiEvent,
	w http.ResponseWriter,
	flusher http.Flusher,
	textBuf, reasoningBuf *strings.Builder,
	tools *[]toolUseRecord,
	usage *agent.PiUsage,
	sawContent bool,
) bool {
	switch ev.Kind {
	case "token":
		textBuf.WriteString(ev.Delta)
		data, _ := json.Marshal(map[string]string{"delta": ev.Delta})
		writeSSEEvent(w, flusher, "token", string(data))
		return true
	case "reasoning":
		reasoningBuf.WriteString(ev.Delta)
		data, _ := json.Marshal(map[string]string{"delta": ev.Delta})
		writeSSEEvent(w, flusher, "reasoning", string(data))
		return true
	case "tool_start":
		data, _ := json.Marshal(map[string]string{"name": ev.ToolName, "input_summary": ev.InputSummary})
		writeSSEEvent(w, flusher, "tool_start", string(data))
	case "tool_end":
		*tools = append(*tools, toolUseRecord{Name: ev.ToolName, OK: ev.OK})
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
		*usage = ev.Usage
		if !sawContent {
			// Pi finished cleanly but emitted no token/reasoning content.
			// Surface this to the client *before* done — the frontend
			// clears its current message on done and would drop a later
			// error event.
			writeSSEEvent(w, flusher, "error", `{"message":"Agent returned no response. The model bailed without producing text — common when tool calls fail. Try rephrasing or starting a fresh session."}`)
		}
		payload := sseDonePayload{Usage: ev.Usage}
		data, _ := json.Marshal(payload)
		writeSSEEvent(w, flusher, "done", string(data))
	case "error":
		data, _ := json.Marshal(map[string]string{"message": ev.Message})
		writeSSEEvent(w, flusher, "error", string(data))
	}
	return sawContent
}

// writeSSEEvent writes one SSE frame and flushes.
func writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, eventType, data string) {
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, data)
	flusher.Flush()
}

// buildPiPrompt wraps the user message with a fresh <plan_state> block
// for course-scoped sessions, so Pi sees authoritative plan state inside
// the conversation history (where it can't be ignored, unlike the system
// prompt which Pi may skip re-reading on --continue). Returns userMessage
// unchanged if no course or claw-cli is configured, or if claw-cli fails.
func buildPiPrompt(courseID, clawCLIPath, userMessage string) string {
	if courseID == "" || clawCLIPath == "" {
		return userMessage
	}
	out, err := exec.Command(clawCLIPath, "plan", "status", "--course", courseID).Output()
	if err != nil || len(out) == 0 {
		return userMessage
	}
	var b strings.Builder
	b.WriteString("<plan_state course=\"")
	b.WriteString(courseID)
	b.WriteString("\" authoritative=\"true\">\n")
	b.WriteString("Fresh read from `data/plans/")
	b.WriteString(courseID)
	b.WriteString(".json` (the canonical store the UI shows). This supersedes any plan state earlier in this conversation. The `study-plan.md` markdown files were retired 2026-05-14 — DO NOT read or write any `.md` plan file. To update a task, call `claw-cli plan toggle --course ")
	b.WriteString(courseID)
	b.WriteString(" --task <N>` using the #N indices below.\n\n")
	b.Write(out)
	if !strings.HasSuffix(string(out), "\n") {
		b.WriteString("\n")
	}
	b.WriteString("</plan_state>\n\n")
	b.WriteString(userMessage)
	return b.String()
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
