package handler

import "net/http"

type versionResponse struct {
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
}

func (h *Handler) versionHandler(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, versionResponse{
		Commit:  h.App.Config.BuildCommit,
		BuiltAt: h.App.Config.BuildTimestamp,
	})
}
