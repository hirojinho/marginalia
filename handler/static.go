package handler

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"study-app/agent"
)

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := h.Static.ReadFile("static/index.html")
	if err != nil {
		writeServerError(w, "read index.html", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if _, err := w.Write(data); err != nil {
		slog.Error("write index.html", "err", err)
	}
}

func (h *Handler) handleStatic(w http.ResponseWriter, r *http.Request) {
	filePath := "static" + strings.TrimPrefix(r.URL.Path, "/static")
	data, err := h.Static.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(r.URL.Path, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
	case strings.HasSuffix(r.URL.Path, ".js"), strings.HasSuffix(r.URL.Path, ".mjs"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
	case strings.HasSuffix(r.URL.Path, ".wasm"):
		w.Header().Set("Content-Type", "application/wasm")
	case strings.HasSuffix(r.URL.Path, ".html"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
	}
	if _, err := w.Write(data); err != nil {
		slog.Error("write static asset", "path", filePath, "err", err)
	}
}

// ---------- health ----------

type fileStatus struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Size   int64  `json:"size,omitempty"`
}

type healthResponse struct {
	VaultRoot   string       `json:"vault_root"`
	Config      fileStatus   `json:"config"`
	Plans       []fileStatus `json:"plans"`
	Memory      []fileStatus `json:"memory"`
	DataCourses []fileStatus `json:"data_courses"`
	Issues      []string     `json:"issues"`
}

func checkFile(path string) fileStatus {
	info, err := os.Stat(path)
	st := fileStatus{Path: path, Exists: err == nil}
	if err == nil {
		st.Size = info.Size()
	}
	return st
}

func (h *Handler) handleDebugHealth(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	resp := healthResponse{VaultRoot: h.App.Config.VaultRoot}
	resp.Config = checkFile(h.App.VaultPath("CLAUDE.local.md"))

	for _, c := range agent.KnownCourses {
		resp.Plans = append(resp.Plans, checkFile(h.App.VaultPath("data", "plans", c.ID+".json")))
	}
	resp.Memory = []fileStatus{
		checkFile(h.App.VaultPath("memory", "study-context.md")),
	}
	for _, c := range agent.KnownCourses {
		resp.Memory = append(resp.Memory, checkFile(h.App.VaultPath("memory", "courses", c.ID, "study-plan.md")))
	}
	for _, c := range agent.KnownCourses {
		resp.DataCourses = append(resp.DataCourses, checkFile(h.App.VaultPath("data", "courses", c.ID, "interests.md")))
	}

	resp.Issues = []string{}
	all := append([]fileStatus{resp.Config}, resp.Plans...)
	all = append(all, resp.Memory...)
	all = append(all, resp.DataCourses...)
	for _, f := range all {
		if !f.Exists {
			resp.Issues = append(resp.Issues, "MISSING: "+f.Path)
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// LogStartupHealth emits a one-line summary of expected files at boot.
// Pure logging — does not affect startup.
func LogStartupHealth(app *agent.App) {
	checks := []string{
		app.VaultPath("CLAUDE.local.md"),
		app.VaultPath("memory", "study-context.md"),
	}
	for _, c := range agent.KnownCourses {
		checks = append(checks, app.VaultPath("data", "plans", c.ID+".json"))
	}

	var issues []string
	for _, p := range checks {
		if _, err := os.Stat(p); err != nil {
			issues = append(issues, "MISSING: "+p)
		}
	}
	if len(issues) > 0 {
		slog.Warn("startup health issues", "count", len(issues), "issues", issues)
	} else {
		slog.Info("startup health ok")
	}
}
