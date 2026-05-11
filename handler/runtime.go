// Package handler — runtime.go implements GET /api/runtime, which
// returns the active agent backend mode so the browser can select
// the correct chat endpoint at boot.
package handler

import "net/http"

type runtimeResponse struct {
	Mode string `json:"mode"`
}

func (h *Handler) handleRuntime(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	mode := string(h.App.Config.ActiveRuntime())
	writeJSON(w, http.StatusOK, runtimeResponse{Mode: mode})
}
