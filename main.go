package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"study-app/agent"

	_ "modernc.org/sqlite"
)

//go:embed static/*
var staticFiles embed.FS

var (
	apiKey  string
	apiURL  = "https://opencode.ai/zen/go/v1"
	model   = "qwen3.6-plus"
	ActiveSessionID int64
)

func main() {
	apiKey = os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("OPENCODE_API_KEY")
	}
	if apiKey == "" {
		log.Fatal("LLM_API_KEY or OPENCODE_API_KEY must be set")
	}
	if envURL := os.Getenv("LLM_API_URL"); envURL != "" {
		apiURL = envURL
	}
	if envModel := os.Getenv("LLM_MODEL"); envModel != "" {
		model = envModel
	}

	agent.VaultRoot = os.Getenv("VAULT_ROOT")
	if agent.VaultRoot == "" {
		agent.VaultRoot = "/workspace"
	}

	dataDir := filepath.Join(agent.VaultRoot, "data")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "pdf-files"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "plans"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "pdf-texts"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "corpus", "study-methods"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "corpus", "courses"), 0755); err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "corpus", "meta"), 0755); err != nil {
		log.Fatal(err)
	}

	dbPath := filepath.Join(dataDir, "study.db")
	if err := agent.InitSessionDB(dbPath); err != nil {
		log.Fatal(err)
	}
	log.Println("SQLite initialized")

	if err := agent.InitVectorStore(); err != nil {
		log.Printf("Warning: vector store init failed: %v", err)
	} else {
		go func() {
			if err := agent.IndexCorpus(); err != nil {
				log.Printf("Corpus indexing failed: %v", err)
			} else {
				log.Printf("Corpus indexed successfully")
			}
		}()
	}

	ActiveSessionID = agent.GetActiveSessionID()

	log.Printf("Study App listening on :8081 (model: %s, api: %s)", model, apiURL)

	http.HandleFunc("/", handleIndex)
	http.HandleFunc("/static/", handleStatic)
	http.HandleFunc("/chat", handleChat)
	http.HandleFunc("/api/sessions", handleSessions)
	http.HandleFunc("/api/sessions/active", handleSessionActive)
	http.HandleFunc("/api/sessions/messages", handleSessionMessages)
	http.HandleFunc("/api/plan", handlePlan)
	http.HandleFunc("/api/plan/toggle", handlePlanToggle)
	http.HandleFunc("/pdf/upload", handlePDFUpload)
	http.HandleFunc("/pdf/extracted/", handlePDFExtracted)
	http.HandleFunc("/pdf/list", handlePDFList)
	http.HandleFunc("/pdf/file/", handlePDFFile)
	http.HandleFunc("/pdf/progress/", handlePDFProgress)
	http.HandleFunc("/pdf/last", handlePDFLastOpened)
	http.HandleFunc("/pdf/annotations/", handlePDFAnnotations)
	http.HandleFunc("/debug/health", handleDebugHealth)

	// Startup health check
	logStartupHealth()

	log.Fatal(http.ListenAndServe(":8081", nil))
}

func handleStatic(w http.ResponseWriter, r *http.Request) {
	filePath := "static" + strings.TrimPrefix(r.URL.Path, "/static")
	data, err := staticFiles.ReadFile(filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if strings.HasSuffix(r.URL.Path, ".mjs") {
		w.Header().Set("Content-Type", "application/javascript")
	} else if strings.HasSuffix(r.URL.Path, ".wasm") {
		w.Header().Set("Content-Type", "application/wasm")
	}
	w.Write(data)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		data, err := staticFiles.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "internal error", 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(data)
		return
	}
	http.NotFound(w, r)
}

func handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	msg := strings.TrimSpace(r.FormValue("message"))
	if msg == "" {
		http.Error(w, "message is required", 400)
		return
	}

	sessionIDStr := r.FormValue("session_id")
	var sessionID int64
	if sessionIDStr != "" {
		fmt.Sscanf(sessionIDStr, "%d", &sessionID)
	}
	if sessionID == 0 {
		sessionID = ActiveSessionID
	}
	if sessionID == 0 {
		http.Error(w, "no active session", 400)
		return
	}

	agent.Mu.Lock()
	agent.SaveMessage(sessionID, "user", msg)
	history := agent.GetSessionHistoryWithSummary(sessionID)
	agent.Mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	prompt := agent.GetSessionSystemPrompt(sessionID, agent.LoadSystemPrompt())
	if err := agent.ProcessWithTools(history, prompt, model, apiKey, apiURL, w, flusher); err != nil {
		log.Printf("processWithTools error: %v", err)
	}

	agent.Mu.Lock()
	agent.SaveLastAssistantContent(sessionID)
	agent.Mu.Unlock()

	fmt.Fprintf(w, "event: done\ndata: {}\n\n")
	flusher.Flush()

	msgCount := agent.GetMessageCount(sessionID)
	var summaryAt int
	agent.DB.QueryRow("SELECT summary_at FROM sessions WHERE id = ?", sessionID).Scan(&summaryAt)
	if msgCount > summaryAt+20 && msgCount > 10 {
		go func(sid int64) {
			agent.Mu.Lock()
			history := agent.GetSessionHistory(sid)
			agent.Mu.Unlock()
			summary, err := agent.GenerateSummary(history, apiKey, apiURL, model)
			if err != nil {
				log.Printf("Summary generation failed for session %d: %v", sid, err)
				return
			}
			agent.Mu.Lock()
			agent.DB.Exec("UPDATE sessions SET summary = ?, summary_at = ? WHERE id = ?", summary, len(history), sid)
			agent.Mu.Unlock()
			log.Printf("Summary generated for session %d (%d messages)", sid, len(history))
		}(sessionID)
	}
}

func handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		rows, err := agent.DB.Query("SELECT id, course_id, topic, created_at, updated_at, last_pdf_id, last_page FROM sessions ORDER BY updated_at DESC")
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		defer rows.Close()
		var sessions []agent.Session
		for rows.Next() {
			var s agent.Session
			rows.Scan(&s.ID, &s.CourseID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage)
			if s.LastPdfID != nil {
				var name string
				agent.DB.QueryRow("SELECT original_name FROM pdfs WHERE id = ?", *s.LastPdfID).Scan(&name)
				s.PdfName = name
			}
			sessions = append(sessions, s)
		}
		if sessions == nil {
			sessions = []agent.Session{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)

	case "POST":
		var body struct {
			CourseID string `json:"course_id"`
			Topic    string `json:"topic"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		if body.Topic == "" {
			body.Topic = "General"
		}
		now := time.Now().Format(time.RFC3339)
		result, err := agent.DB.Exec(
			"INSERT INTO sessions (course_id, topic, created_at, updated_at) VALUES (?, ?, ?, ?)",
			body.CourseID, body.Topic, now, now,
		)
		if err != nil {
			http.Error(w, "db error", 500)
			return
		}
		id, _ := result.LastInsertId()
		ActiveSessionID = id
		agent.DB.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES ('last_session', ?)", fmt.Sprintf("%d", id))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(agent.Session{ID: id, CourseID: body.CourseID, Topic: body.Topic, CreatedAt: now, UpdatedAt: now, LastPage: 1})

	case "DELETE":
		idStr := r.URL.Query().Get("id")
		var id int64
		fmt.Sscanf(idStr, "%d", &id)
		if id == 0 {
			http.Error(w, "id required", 400)
			return
		}
		agent.DB.Exec("DELETE FROM messages WHERE session_id = ?", id)
		agent.DB.Exec("DELETE FROM sessions WHERE id = ?", id)
		if ActiveSessionID == id {
			ActiveSessionID = 0
			agent.DB.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES ('last_session', '0')")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func handleSessionActive(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		if ActiveSessionID == 0 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id": nil})
			return
		}
		var s agent.Session
		err := agent.DB.QueryRow("SELECT id, course_id, topic, created_at, updated_at, last_pdf_id, last_page FROM sessions WHERE id = ?", ActiveSessionID).Scan(
			&s.ID, &s.CourseID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage,
		)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{"id": nil})
			return
		}
		if s.LastPdfID != nil {
			var name string
			agent.DB.QueryRow("SELECT original_name FROM pdfs WHERE id = ?", *s.LastPdfID).Scan(&name)
			s.PdfName = name
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s)

	case "PUT":
		var body struct {
			ID int64 `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		var exists int64
		agent.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", body.ID).Scan(&exists)
		if exists == 0 {
			http.Error(w, "session not found", 404)
			return
		}
		ActiveSessionID = body.ID
		agent.SetActiveSessionID(body.ID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": body.ID})

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	sessionIDStr := r.URL.Query().Get("session_id")
	var sessionID int64
	fmt.Sscanf(sessionIDStr, "%d", &sessionID)
	if sessionID == 0 {
		http.Error(w, "session_id required", 400)
		return
	}
	msgs := agent.GetSessionHistory(sessionID)
	if msgs == nil {
		msgs = []agent.Message{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}

	view := r.URL.Query().Get("view")
	if view == "full" {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "id required", 400)
			return
		}
		p := agent.LoadPlan(id)
		if p == nil {
			http.Error(w, "plan not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(p)
		return
	}

	var response []agent.PlanSummary
	for _, c := range agent.KnownCourses {
		p := agent.LoadPlan(c.ID)
		done, total := agent.CountTasks(p)
		response = append(response, agent.PlanSummary{
			ID:      c.ID,
			Name:    c.Name,
			Done:    done,
			Total:   total,
			HasPlan: p != nil,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func handlePlanToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	course := r.FormValue("course")
	taskIdx := 0
	fmt.Sscanf(r.FormValue("index"), "%d", &taskIdx)

	p := agent.LoadPlan(course)
	if p == nil {
		http.Error(w, "plan not found", 404)
		return
	}

	count := 0
	for i := range p.Phases {
		for j := range p.Phases[i].Tasks {
			if count == taskIdx {
				p.Phases[i].Tasks[j].Done = !p.Phases[i].Tasks[j].Done
				goto found
			}
			count++
		}
		for k := range p.Phases[i].Clusters {
			for j := range p.Phases[i].Clusters[k].Tasks {
				if count == taskIdx {
					p.Phases[i].Clusters[k].Tasks[j].Done = !p.Phases[i].Clusters[k].Tasks[j].Done
					goto found
				}
				count++
			}
		}
	}
	http.Error(w, "task not found", 404)
	return

found:
	agent.SavePlan(p)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func handlePDFUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "method not allowed", 405)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", 400)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "read error", 500)
		return
	}

	courseID := r.FormValue("course_id")

	pages := agent.ExtractPDFPageCount(data)
	if pages == 0 {
		pages = 1
	}

	result, err := agent.DB.Exec(
		"INSERT INTO pdfs (filename, original_name, course_id, pages, last_page, uploaded_at) VALUES (?, ?, ?, ?, 1, ?)",
		"", header.Filename, courseID, pages, time.Now().Format(time.RFC3339),
	)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	id, _ := result.LastInsertId()
	filename := fmt.Sprintf("%d.pdf", id)

	agent.DB.Exec("UPDATE pdfs SET filename = ? WHERE id = ?", filename, id)

	filePath := filepath.Join(agent.VaultRoot, "data", "pdf-files", filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		agent.DB.Exec("DELETE FROM pdfs WHERE id = ?", id)
		http.Error(w, "save error", 500)
		return
	}

	agent.DB.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES ('last_opened_pdf', ?)", fmt.Sprintf("%d", id))

	go func() {
		pdfPath := filepath.Join(agent.VaultRoot, "data", "pdf-files", filename)
		agent.ExtractAndCachePDFText(id, pdfPath)
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":            id,
		"filename":      filename,
		"original_name": header.Filename,
		"course_id":     courseID,
		"pages":         pages,
		"last_page":     1,
		"uploaded_at":   time.Now().Format(time.RFC3339),
	})
}

func handlePDFExtracted(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/pdf/extracted/"), "/")
	idStr := parts[0]
	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "invalid id", 400)
		return
	}
	cachePath := filepath.Join(agent.VaultRoot, "data", "pdf-texts", fmt.Sprintf("%d.txt", id))
	data, err := os.ReadFile(cachePath)
	if err != nil {
		http.Error(w, "not extracted yet", 404)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func handlePDFList(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}

	courseFilter := r.URL.Query().Get("course")

	query := "SELECT id, original_name, course_id, pages, last_page, last_read_at, uploaded_at FROM pdfs ORDER BY uploaded_at DESC"
	rows, err := agent.DB.Query(query)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	var results []agent.PDFEntry
	for rows.Next() {
		var e agent.PDFEntry
		if err := rows.Scan(&e.ID, &e.OriginalName, &e.CourseID, &e.Pages, &e.LastPage, &e.LastReadAt, &e.UploadedAt); err != nil {
			continue
		}
		if courseFilter != "" {
			if e.CourseID == nil || *e.CourseID != courseFilter {
				continue
			}
		}
		if e.CourseID != nil {
			e.CourseName = agent.CourseName(*e.CourseID)
		}
		results = append(results, e)
	}

	if results == nil {
		results = []agent.PDFEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func handlePDFFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}

	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/pdf/file/"), "/")
	idStr := parts[0]
	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	var filename string
	err := agent.DB.QueryRow("SELECT filename FROM pdfs WHERE id = ?", id).Scan(&filename)
	if err != nil {
		http.Error(w, "not found", 404)
		return
	}

	filePath := filepath.Join(agent.VaultRoot, "data", "pdf-files", filename)
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, filePath)
}

func handlePDFProgress(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/pdf/progress/"), "/")
	idStr := parts[0]
	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "invalid id", 400)
		return
	}

	switch r.Method {
	case "GET":
		var lastPage int
		var lastReadAt *string
		err := agent.DB.QueryRow("SELECT last_page, last_read_at FROM pdfs WHERE id = ?", id).Scan(&lastPage, &lastReadAt)
		if err != nil {
			http.Error(w, "not found", 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":           id,
			"last_page":    lastPage,
			"last_read_at": lastReadAt,
		})

	case "PUT":
		var body struct {
			Page int `json:"page"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", 400)
			return
		}
		now := time.Now().Format(time.RFC3339)
		agent.DB.Exec("UPDATE pdfs SET last_page = ?, last_read_at = ? WHERE id = ?", body.Page, now, id)
		agent.DB.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES ('last_opened_pdf', ?)", fmt.Sprintf("%d", id))

		cachePath := filepath.Join(agent.VaultRoot, "data", "pdf-texts", fmt.Sprintf("%d.txt", id))
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			pdfPath := filepath.Join(agent.VaultRoot, "data", "pdf-files", fmt.Sprintf("%d.pdf", id))
			go agent.ExtractAndCachePDFText(int64(id), pdfPath)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":           id,
			"last_page":    body.Page,
			"last_read_at": now,
		})

	default:
		http.Error(w, "method not allowed", 405)
	}
}

func handlePDFLastOpened(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}

	var value string
	err := agent.DB.QueryRow("SELECT value FROM meta WHERE key = 'last_opened_pdf'").Scan(&value)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": nil})
		return
	}

	var id int
	fmt.Sscanf(value, "%d", &id)

	var e agent.PDFEntry
	err = agent.DB.QueryRow("SELECT id, original_name, course_id, pages, last_page, last_read_at, uploaded_at FROM pdfs WHERE id = ?", id).Scan(
		&e.ID, &e.OriginalName, &e.CourseID, &e.Pages, &e.LastPage, &e.LastReadAt, &e.UploadedAt,
	)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"id": nil})
		return
	}
	if e.CourseID != nil {
		e.CourseName = agent.CourseName(*e.CourseID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":  id,
		"pdf": e,
	})
}

func handlePDFAnnotations(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", 501)
}

// Health check types

type FileStatus struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
	Size   int64  `json:"size,omitempty"`
}

type HealthResponse struct {
	VaultRoot  string      `json:"vault_root"`
	Config     FileStatus  `json:"config"`
	Plans      []FileStatus `json:"plans"`
	Memory     []FileStatus `json:"memory"`
	DataCourses []FileStatus `json:"data_courses"`
	Issues     []string    `json:"issues"`
}

func checkFile(path string) FileStatus {
	info, err := os.Stat(path)
	return FileStatus{
		Path:   path,
		Exists: err == nil,
		Size:   info.Size(),
	}
}

func logStartupHealth() {
	issues := []string{}
	checks := []string{
		"/workspace/study-app/CLAUDE.local.md",
		"/workspace/study-app/memory/study-context.md",
	}
	for _, p := range checks {
		if _, err := os.Stat(p); err != nil {
			issues = append(issues, "MISSING: "+p)
		}
	}
	courses := []string{"ce297", "ddia", "software-arch", "dsa-interview"}
	for _, c := range courses {
		p := fmt.Sprintf("/workspace/study-app/memory/courses/%s/interests.md", c)
		if _, err := os.Stat(p); err != nil {
			issues = append(issues, "MISSING: "+p)
		}
	}
	plans := []string{"ce297.json", "ddia.json", "software-arch.json", "thesis.json"}
	for _, p := range plans {
		fp := fmt.Sprintf("/workspace/study-app/data/plans/%s", p)
		if _, err := os.Stat(fp); err != nil {
			issues = append(issues, "MISSING: "+fp)
		}
	}
	if len(issues) > 0 {
		log.Printf("⚠ Health issues at startup:")
		for _, i := range issues {
			log.Printf("  - %s", i)
		}
	} else {
		log.Printf("✓ All expected files present")
	}
}

func handleDebugHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "method not allowed", 405)
		return
	}

	resp := HealthResponse{
		VaultRoot: agent.VaultRoot,
		Config:    checkFile("/workspace/study-app/CLAUDE.local.md"),
		Plans: []FileStatus{
			checkFile("/workspace/study-app/data/plans/ce297.json"),
			checkFile("/workspace/study-app/data/plans/ddia.json"),
			checkFile("/workspace/study-app/data/plans/software-arch.json"),
			checkFile("/workspace/study-app/data/plans/thesis.json"),
		},
		Memory: []FileStatus{
			checkFile("/workspace/study-app/memory/study-context.md"),
			checkFile("/workspace/study-app/memory/courses/ce297/study-plan.md"),
			checkFile("/workspace/study-app/memory/courses/ce297/interests.md"),
			checkFile("/workspace/study-app/memory/courses/ddia/study-plan.md"),
			checkFile("/workspace/study-app/memory/courses/ddia/interests.md"),
			checkFile("/workspace/study-app/memory/courses/software-arch/study-plan.md"),
			checkFile("/workspace/study-app/memory/thesis/study-plan.md"),
		},
		DataCourses: []FileStatus{
			checkFile("/workspace/study-app/data/courses/ce297/interests.md"),
			checkFile("/workspace/study-app/data/courses/ddia/interests.md"),
			checkFile("/workspace/study-app/data/courses/software-arch/interests.md"),
		},
	}

	resp.Issues = []string{}
	allFiles := append([]FileStatus{resp.Config}, resp.Plans...)
	allFiles = append(allFiles, resp.Memory...)
	allFiles = append(allFiles, resp.DataCourses...)
	for _, f := range allFiles {
		if !f.Exists {
			resp.Issues = append(resp.Issues, "MISSING: "+f.Path)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}