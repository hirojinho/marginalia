package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"study-app/agent"
)

const summaryThreshold = 20

func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listSessions(w, r)
	case http.MethodPost:
		h.createSession(w, r)
	case http.MethodDelete:
		h.deleteSession(w, r)
	default:
		methodNotAllowed(w, r, http.MethodGet, http.MethodPost, http.MethodDelete)
	}
}

func (h *Handler) listSessions(w http.ResponseWriter, _ *http.Request) {
	sessions, err := h.App.ListSessions()
	if err != nil {
		writeServerError(w, "list sessions", err)
		return
	}
	if sessions == nil {
		sessions = []agent.Session{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (h *Handler) createSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CourseID string `json:"course_id"`
		Topic    string `json:"topic"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	s, err := h.App.CreateSession(body.CourseID, body.Topic)
	if err != nil {
		writeServerError(w, "create session", err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) deleteSession(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(r.URL.Query().Get("id"), "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := h.App.DeleteSession(id); err != nil {
		writeServerError(w, "delete session", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) handleSessionActive(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getActiveSession(w)
	case http.MethodPut:
		h.setActiveSession(w, r)
	default:
		methodNotAllowed(w, r, http.MethodGet, http.MethodPut)
	}
}

func (h *Handler) getActiveSession(w http.ResponseWriter) {
	id := h.App.ActiveSessionID()
	if id == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": nil})
		return
	}
	s, err := h.App.GetSession(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"id": nil})
			return
		}
		writeServerError(w, "get active session", err)
		return
	}
	writeJSON(w, http.StatusOK, s)
}

func (h *Handler) setActiveSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.App.SetActiveSession(body.ID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"id": body.ID})
}

func (h *Handler) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	id, err := parseInt64(r.URL.Query().Get("session_id"), "session_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	msgs, err := h.App.GetSessionHistory(id)
	if err != nil {
		writeServerError(w, "get session history", err)
		return
	}
	if msgs == nil {
		msgs = []agent.Message{}
	}
	writeJSON(w, http.StatusOK, msgs)
}

// ---------- chat ----------

func (h *Handler) handleChat(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}

	msg := strings.TrimSpace(r.FormValue("message"))
	if msg == "" {
		writeError(w, http.StatusBadRequest, "message is required")
		return
	}
	if len(msg) > MaxMessageBytes {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("message exceeds %d bytes", MaxMessageBytes))
		return
	}

	sessionID := h.App.ActiveSessionID()
	if s := r.FormValue("session_id"); s != "" {
		if parsed, err := parseInt64(s, "session_id"); err == nil {
			sessionID = parsed
		}
	}
	if sessionID == 0 {
		writeError(w, http.StatusBadRequest, "no active session")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	h.App.LockChat()
	if err := h.App.SaveMessage(sessionID, "user", msg); err != nil {
		h.App.UnlockChat()
		writeServerError(w, "save user message", err)
		return
	}
	history, err := h.App.GetSessionHistoryWithSummary(sessionID)
	h.App.UnlockChat()
	if err != nil {
		writeServerError(w, "load history", err)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	prompt := h.App.GetSessionSystemPrompt(sessionID, h.App.LoadSystemPrompt())

	// Use the request context so the LLM call is cancelled when the
	// client disconnects.
	content, err := h.App.ProcessWithTools(r.Context(), h.LLM, history, prompt, w, flusher)
	if err != nil {
		log.Printf("processWithTools session %d: %v", sessionID, err)
	}

	// Persist whatever content was produced before any error/cancellation.
	if content != "" {
		h.App.LockChat()
		if err := h.App.SaveMessage(sessionID, "assistant", content); err != nil {
			log.Printf("save assistant message: %v", err)
		}
		h.App.UnlockChat()
	}

	fmt.Fprintf(w, "event: done\ndata: {}\n\n")
	flusher.Flush()

	h.maybeGenerateSummary(sessionID)
}

// maybeGenerateSummary kicks off background summary generation if the
// number of new messages since the last summary exceeds the threshold.
// Best-effort; errors are logged.
func (h *Handler) maybeGenerateSummary(sessionID int64) {
	count, err := h.App.GetMessageCount(sessionID)
	if err != nil {
		log.Printf("get message count: %v", err)
		return
	}
	_, summaryAt, err := h.App.GetSessionSummary(sessionID)
	if err != nil {
		log.Printf("get session summary: %v", err)
		return
	}
	if !(count > summaryAt+summaryThreshold && count > 10) {
		return
	}

	go func(sid int64) {
		h.App.LockChat()
		history, err := h.App.GetSessionHistory(sid)
		h.App.UnlockChat()
		if err != nil {
			log.Printf("summary: load history: %v", err)
			return
		}
		summary, err := h.LLM.GenerateSummary(context.Background(), history)
		if err != nil {
			log.Printf("summary: generate: %v", err)
			return
		}
		h.App.LockChat()
		err = h.App.UpdateSessionSummary(sid, summary, len(history))
		h.App.UnlockChat()
		if err != nil {
			log.Printf("summary: update: %v", err)
			return
		}
		log.Printf("summary generated for session %d (%d messages)", sid, len(history))
	}(sessionID)
}
