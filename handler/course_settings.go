package handler

import (
	"encoding/json"
	"net/http"

	"study-app/agent"
)

// handleCourseSettings serves GET (read, ?course_id=) and PUT (write, JSON
// body) for per-course Steering settings. The same validated write path the
// claw-cli tool uses (ADR 0010/0016).
func (h *Handler) handleCourseSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		courseID := r.URL.Query().Get("course_id")
		if courseID == "" {
			writeError(w, http.StatusBadRequest, "course_id is required")
			return
		}
		s, err := h.App.GetCourseSettings(courseID)
		if err != nil {
			writeServerError(w, "get course settings", err)
			return
		}
		writeJSON(w, http.StatusOK, s)

	case http.MethodPut:
		var s agent.CourseSettings
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON")
			return
		}
		if s.CourseID == "" {
			s.CourseID = r.URL.Query().Get("course_id")
		}
		if s.CourseID == "" {
			writeError(w, http.StatusBadRequest, "course_id is required")
			return
		}
		if err := agent.ValidateCourseSettings(s); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if err := h.App.UpsertCourseSettings(s); err != nil {
			writeServerError(w, "save course settings", err)
			return
		}
		out, err := h.App.GetCourseSettings(s.CourseID)
		if err != nil {
			writeServerError(w, "read back course settings", err)
			return
		}
		writeJSON(w, http.StatusOK, out)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}
