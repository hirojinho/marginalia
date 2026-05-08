package handler

import (
	"log"
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
		log.Printf("write index.html: %v", err)
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
	case strings.HasSuffix(r.URL.Path, ".mjs"):
		w.Header().Set("Content-Type", "application/javascript")
	case strings.HasSuffix(r.URL.Path, ".wasm"):
		w.Header().Set("Content-Type", "application/wasm")
	}
	if _, err := w.Write(data); err != nil {
		log.Printf("write static %s: %v", filePath, err)
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
	resp.Config = checkFile(h.App.VaultPath("study-app", "CLAUDE.local.md"))

	for _, c := range agent.KnownCourses {
		resp.Plans = append(resp.Plans, checkFile(h.App.VaultPath("data", "plans", c.ID+".json")))
	}
	resp.Memory = []fileStatus{
		checkFile(h.App.VaultPath("study-app", "memory", "study-context.md")),
	}
	for _, c := range agent.KnownCourses {
		resp.Memory = append(resp.Memory, checkFile(h.App.VaultPath("study-app", "memory", "courses", c.ID, "study-plan.md")))
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
		app.VaultPath("study-app", "CLAUDE.local.md"),
		app.VaultPath("study-app", "memory", "study-context.md"),
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
		log.Printf("startup health: %d issue(s)", len(issues))
		for _, i := range issues {
			log.Printf("  - %s", i)
		}
	} else {
		log.Print("startup health: all expected files present")
	}
}
