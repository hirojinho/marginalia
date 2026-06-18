package handler

import (
	"encoding/json"
	"net/http"
	"regexp"
)

var kebabRe = regexp.MustCompile(`^[a-z0-9-]+$`)

func (h *Handler) handleCourses(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		courses, err := h.App.ListCourses()
		if err != nil {
			writeServerError(w, "list courses", err)
			return
		}
		writeJSON(w, http.StatusOK, courses)
		return
	}
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}

	var req struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.ID == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "id and name are required")
		return
	}

	if !kebabRe.MatchString(req.ID) {
		writeError(w, http.StatusBadRequest, "id must be kebab-case (lowercase letters, digits, hyphens only)")
		return
	}

	if err := h.App.CreateCourse(req.ID, req.Name); err != nil {
		if err.Error() == "course already exists: "+req.ID {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeServerError(w, "create course", err)
		return
	}

	course, _ := h.App.GetCourse(req.ID)
	writeJSON(w, http.StatusCreated, course)
}
