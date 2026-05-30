// Package handler holds the HTTP layer for the study app.
//
// All handlers hang off Handler, which carries the App (database +
// config) and an LLM client. Construct one with New, then call
// Register to wire the routes onto an http.ServeMux.
package handler

import (
	"embed"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"study-app/agent"
)

// MaxMessageBytes is the upper bound on a single chat message. Beyond
// this, the request is rejected before reaching the LLM. Keeps a
// runaway client from exhausting tokens or memory.
const MaxMessageBytes = 4000

// MaxPDFBytes caps PDF uploads. Larger files are rejected before
// hitting disk. SQLite metadata + per-file text caches assume bounded
// inputs.
const MaxPDFBytes = 50 * 1024 * 1024 // 50 MB

// Handler is the dependency root for HTTP handlers. Construct via New.
type Handler struct {
	App    *agent.App
	LLM    *agent.LLMClient
	Static embed.FS
}

func New(app *agent.App, llm *agent.LLMClient, static embed.FS) *Handler {
	return &Handler{App: app, LLM: llm, Static: static}
}

// Register attaches all study-app HTTP routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", h.handleIndex)
	mux.HandleFunc("/static/", h.handleStatic)
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/chat", h.handleChat)
	mux.HandleFunc("/chat-v2", h.handleChatV2)
	mux.HandleFunc("/api/sessions", h.handleSessions)
	mux.HandleFunc("/api/sessions/active", h.handleSessionActive)
	mux.HandleFunc("/api/sessions/messages", h.handleSessionMessages)
	mux.HandleFunc("/api/sessions/stats", h.getSessionStats)
	mux.HandleFunc("/api/sessions/for-task", h.handleSessionForTask)
	mux.HandleFunc("/api/plan", h.handlePlan)
	mux.HandleFunc("/api/plan/toggle", h.handlePlanToggle)
	mux.HandleFunc("/api/courses", h.handleCourses)
	mux.HandleFunc("/api/courses/settings", h.handleCourseSettings)
	mux.HandleFunc("/pdf/upload", h.handlePDFUpload)
	mux.HandleFunc("/pdf/extracted/", h.handlePDFExtracted)
	mux.HandleFunc("/pdf/list", h.handlePDFList)
	mux.HandleFunc("/pdf/file/", h.handlePDFFile)
	mux.HandleFunc("/pdf/progress/", h.handlePDFProgress)
	mux.HandleFunc("/pdf/last", h.handlePDFLastOpened)
	mux.HandleFunc("/pdf/annotations/", h.handlePDFAnnotations)
	mux.HandleFunc("/api/runtime", h.handleRuntime)
	mux.HandleFunc("/debug/health", h.handleDebugHealth)
	mux.HandleFunc("/debug/version", h.versionHandler)
	mux.HandleFunc("/debug/metrics", h.handleDebugMetrics)
	mux.HandleFunc("/debug/schema", h.schemaHandler)
}

// ---------- helpers ----------

// writeJSON serializes v as JSON. Logs (but does not surface) encoder
// errors, since at that point the response status has been written.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Warn("encode response", "err", err)
	}
}

// writeError sends a JSON error body with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeServerError logs the underlying cause and returns a generic 500.
// Use this for failures whose details should not leak to the client.
func writeServerError(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

// parseInt64 parses a positive int64 from a string. Returns an error
// if s is empty, malformed, or non-positive.
func parseInt64(s, name string) (int64, error) {
	if s == "" {
		return 0, errors.New(name + " is required")
	}
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, errors.New(name + " is invalid")
	}
	if n <= 0 {
		return 0, errors.New(name + " must be positive")
	}
	return n, nil
}

// pathSuffix returns the segment after prefix, up to the next slash.
// Used for routes like "/pdf/file/{id}" where id is the path tail.
func pathSuffix(path, prefix string) string {
	rest := strings.TrimPrefix(path, prefix)
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		return rest[:i]
	}
	return rest
}

// methodNotAllowed writes a 405 if the request method does not match
// expected. Returns true when it has handled the request.
func methodNotAllowed(w http.ResponseWriter, r *http.Request, expected ...string) bool {
	for _, m := range expected {
		if r.Method == m {
			return false
		}
	}
	w.Header().Set("Allow", strings.Join(expected, ", "))
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	return true
}
