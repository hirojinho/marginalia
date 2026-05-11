// Package handler — chat_v2.go implements the stub /chat-v2 endpoint.
// Phase 4: creates/reuses the per-session Pi sandbox and returns its path.
// Pi is not spawned yet; that is Phase 5.
package handler

import (
	"encoding/json"
	"net/http"
)

type chatV2Request struct {
	SessionID int64  `json:"session_id"`
	Message   string `json:"message"`
}

// handleChatV2 handles POST /chat-v2. It creates or reuses the per-session
// sandbox for the given session_id and returns the sandbox path.
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

	writeJSON(w, http.StatusOK, map[string]string{
		"sandbox": sandboxPath,
		"status":  "stub — Pi not yet wired",
	})
}
