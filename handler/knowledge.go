package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"marginalia/agent"
)

func (h *Handler) handleKnowledge(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleKnowledgeList(w, r)
	case http.MethodPost:
		h.handleKnowledgeCreate(w, r)
	default:
		methodNotAllowed(w, r, http.MethodGet, http.MethodPost)
	}
}

func (h *Handler) handleKnowledgeList(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err == nil && n > 0 {
			limit = n
		}
	}
	kcs, err := h.App.ListKnowledgeComponents(limit)
	if err != nil {
		writeServerError(w, "list knowledge components", err)
		return
	}
	if kcs == nil {
		writeJSON(w, http.StatusOK, []agent.KnowledgeComponent{})
		return
	}
	writeJSON(w, http.StatusOK, kcs)
}

func (h *Handler) handleKnowledgeCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title           string `json:"title"`
		Body            string `json:"body"`
		SourceTaskID    string `json:"source_task_id"`
		SourceSessionID int64  `json:"source_session_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	if req.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}
	if len(req.Title) > 500 {
		writeError(w, http.StatusBadRequest, "title must be 500 characters or less")
		return
	}
	if len(req.Body) > 500 {
		writeError(w, http.StatusBadRequest, "body must be 500 characters or less")
		return
	}
	id, err := h.App.CreateKnowledgeComponent(req.Title, req.Body, req.SourceTaskID, req.SourceSessionID)
	if err != nil {
		writeServerError(w, "create knowledge component", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}
