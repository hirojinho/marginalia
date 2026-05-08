package agent

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/ledongthuc/pdf"
)

type ToolDef struct {
	Type     string   `json:"type"`
	Function ToolFunc `json:"function"`
}

type ToolFunc struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ToolCall struct {
	Name string
	Args json.RawMessage
}

// webFetchLimiter is a per-process simple rate limiter on web_fetch
// calls. It's a singleton because the rate limit is a property of the
// outbound HTTP behavior, not of any one App instance.
var (
	webFetchMu    sync.Mutex
	webFetchTimes []time.Time
)

var KnownCourses = []struct {
	ID   string
	Name string
}{
	{"ce297", "Safety Models and Techniques (CE-297)"},
	{"ddia", "Designing Data-Intensive Applications"},
	{"dsa-interview", "DSA Interview Prep"},
	{"software-arch", "Software Architecture"},
	{"thesis", "Thesis — Phase 1 Survey"},
}

func CourseName(id string) string {
	for _, c := range KnownCourses {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

func GetTools() []ToolDef {
	return []ToolDef{
		{Type: "function", Function: ToolFunc{
			Name:        "read_file",
			Description: "Read a file from the workspace or vault",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Absolute file path to read"},
				},
				"required": []string{"path"},
			},
		}},
		{Type: "function", Function: ToolFunc{
			Name:        "search_files",
			Description: "Search file contents using a regex pattern",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pattern": map[string]interface{}{"type": "string", "description": "Search pattern (case-insensitive)"},
					"include": map[string]interface{}{"type": "string", "description": "File glob pattern (e.g. *.md)"},
				},
				"required": []string{"pattern"},
			},
		}},
		{Type: "function", Function: ToolFunc{
			Name:        "list_files",
			Description: "List files in a directory",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Directory path"},
				},
				"required": []string{"path"},
			},
		}},
		{Type: "function", Function: ToolFunc{
			Name:        "save_note",
			Description: "Save a fleeting note or update an existing note",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string", "description": "Relative path under vault (e.g. fleeting/2026-05-05.md or courses/ce297/note.md)"},
					"content": map[string]interface{}{"type": "string", "description": "Note content in markdown"},
				},
				"required": []string{"path", "content"},
			},
		}},
		{Type: "function", Function: ToolFunc{
			Name:        "update_plan",
			Description: "Update a study plan: toggle tasks, add new tasks, or mark tasks as done/undone. Use this to adjust study plans based on progress or discoveries during sessions.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"plan_id":       map[string]interface{}{"type": "string", "description": "Plan ID (e.g. ce297, ddia, software-arch, thesis)"},
					"action":        map[string]interface{}{"type": "string", "description": "Action to perform: toggle (flip done state), set_done (mark done), set_undone (mark not done), add_task (add new task)"},
					"task_index":    map[string]interface{}{"type": "integer", "description": "Task index (0-based, sequential across all phases/clusters). Required for toggle, set_done, set_undone."},
					"task_title":    map[string]interface{}{"type": "string", "description": "Title for new task. Required for add_task."},
					"task_priority": map[string]interface{}{"type": "string", "description": "Priority label for new task (optional, e.g. high, medium, low)."},
				},
				"required": []string{"plan_id", "action"},
			},
		}},
		{Type: "function", Function: ToolFunc{
			Name:        "pdf_extract",
			Description: "Extract text content from an uploaded PDF. Use this to read and understand PDF content that the user has uploaded.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"pdf_id": map[string]interface{}{"type": "integer", "description": "Database ID of the PDF (from the PDF list)"},
					"pages":  map[string]interface{}{"type": "string", "description": "Optional page range, e.g. '1-5' or '1,3,7'. Default: all pages"},
				},
				"required": []string{"pdf_id"},
			},
		}},
		{Type: "function", Function: ToolFunc{
			Name:        "web_fetch",
			Description: "Fetch a web page and convert it to readable markdown. Use this to look up information not in your local knowledge.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"url": map[string]interface{}{"type": "string", "description": "URL to fetch (http:// or https://)"},
				},
				"required": []string{"url"},
			},
		}},
		{Type: "function", Function: ToolFunc{
			Name:        "study_skill",
			Description: "Invoke a study skill to get structured guidance. Available skills: orientation (pre-reading primer), study_notes (structured note generation), self_test (practice questions), review (spaced repetition assessment), grill_me (relentless interview about study plans and decisions).",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"skill": map[string]interface{}{"type": "string", "description": "Skill name: orientation, study_notes, self_test, review"},
					"params": map[string]interface{}{
						"type":        "object",
						"description": "Skill parameters",
						"properties": map[string]interface{}{
							"topic":     map[string]interface{}{"type": "string", "description": "The study topic"},
							"course_id": map[string]interface{}{"type": "string", "description": "Course ID (e.g. ce297, ddia)"},
							"content":   map[string]interface{}{"type": "string", "description": "Optional content to process (for study_notes)"},
							"count":     map[string]interface{}{"type": "integer", "description": "Number of questions (for self_test, default: 5)"},
						},
					},
				},
				"required": []string{"skill"},
			},
		}},
		{Type: "function", Function: ToolFunc{
			Name:        "rag_search",
			Description: "Search the knowledge corpus using semantic similarity. Use this when you need to find relevant context for a topic or concept.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query":  map[string]interface{}{"type": "string", "description": "Search query (topic, concept, or question)"},
					"course": map[string]interface{}{"type": "string", "description": "Optional course ID to scope search (e.g. ce297, ddia)"},
					"top_k":  map[string]interface{}{"type": "integer", "description": "Number of results (default: 3, max: 10)"},
				},
				"required": []string{"query"},
			},
		}},
	}
}

// ExecuteTool dispatches a parsed tool call to its implementation.
// Returns a human-readable result string suitable for sending back to
// the LLM as a tool message. Errors are encoded into the returned
// string rather than returned separately, since the LLM consumes it
// either way.
func (a *App) ExecuteTool(name string, args json.RawMessage) string {
	switch name {
	case "read_file":
		return a.toolReadFile(args)
	case "search_files":
		return a.toolSearchFiles(args)
	case "list_files":
		return a.toolListFiles(args)
	case "save_note":
		return a.toolSaveNote(args)
	case "update_plan":
		return a.toolUpdatePlan(args)
	case "pdf_extract":
		return a.toolPDFExtract(args)
	case "web_fetch":
		return toolWebFetch(args)
	case "study_skill":
		return a.toolStudySkill(args)
	case "rag_search":
		return a.toolRAGSearch(args)
	}
	return "unknown tool: " + name
}

func (a *App) toolReadFile(args json.RawMessage) string {
	var p struct{ Path string }
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(data)
}

func (a *App) toolSearchFiles(args json.RawMessage) string {
	var p struct {
		Pattern string `json:"pattern"`
		Include string `json:"include"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	include := p.Include
	if include == "" {
		include = "*.md"
	}
	cmd := exec.Command("rg", "-l", "-i", p.Pattern, "--glob", include, a.Config.VaultRoot)
	out, err := cmd.Output()
	if err != nil {
		if len(out) == 0 {
			return "No matches found."
		}
		return "error: " + err.Error()
	}
	return string(out)
}

func (a *App) toolListFiles(args json.RawMessage) string {
	var p struct{ Path string }
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.Path == "" {
		p.Path = a.Config.VaultRoot
	}
	entries, err := os.ReadDir(p.Path)
	if err != nil {
		return "error: " + err.Error()
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			n += "/"
		}
		names = append(names, n)
	}
	return strings.Join(names, "\n")
}

func (a *App) toolSaveNote(args json.RawMessage) string {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	full := filepath.Join(a.Config.VaultRoot, p.Path)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "error: " + err.Error()
	}
	if err := os.WriteFile(full, []byte(p.Content), 0644); err != nil {
		return "error: " + err.Error()
	}
	return "saved to " + full
}

func (a *App) toolRAGSearch(args json.RawMessage) string {
	var p struct {
		Query  string `json:"query"`
		Course string `json:"course"`
		TopK   int    `json:"top_k"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.Query == "" {
		return "error: query is required"
	}
	if p.TopK <= 0 {
		p.TopK = 3
	}
	if p.TopK > 10 {
		p.TopK = 10
	}

	results, err := a.Search(p.Query, p.Course, p.TopK)
	if err != nil {
		return "error: " + err.Error()
	}
	if len(results) == 0 {
		return "No relevant results found for: " + p.Query
	}

	var out strings.Builder
	for _, r := range results {
		heading := r.Heading
		if heading == "" {
			heading = r.ParentHeading
		}
		if heading == "" {
			heading = r.SourceFile
		}
		fmt.Fprintf(&out, "\n--- %s (%s) [score: %.3f] ---\n%s\n",
			r.SourceFile, heading, r.Score, r.Content)
	}
	return strings.TrimPrefix(out.String(), "\n")
}

func (a *App) toolUpdatePlan(args json.RawMessage) string {
	var p struct {
		PlanID       string `json:"plan_id"`
		Action       string `json:"action"`
		TaskIndex    int    `json:"task_index"`
		TaskTitle    string `json:"task_title"`
		TaskPriority string `json:"task_priority"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.PlanID == "" {
		return "error: plan_id is required"
	}

	plan := a.LoadPlan(p.PlanID)
	if plan == nil {
		return "error: plan not found: " + p.PlanID
	}

	switch p.Action {
	case "toggle", "set_done", "set_undone":
		count := 0
		for i := range plan.Phases {
			for j := range plan.Phases[i].Tasks {
				if count == p.TaskIndex {
					applyAction(&plan.Phases[i].Tasks[j].Done, p.Action)
					if err := a.SavePlan(plan); err != nil {
						return "error saving plan: " + err.Error()
					}
					return fmt.Sprintf("Task %d %q in phase %q marked as %s",
						p.TaskIndex, plan.Phases[i].Tasks[j].Title, plan.Phases[i].Title, doneState(plan.Phases[i].Tasks[j].Done))
				}
				count++
			}
			for k := range plan.Phases[i].Clusters {
				for j := range plan.Phases[i].Clusters[k].Tasks {
					if count == p.TaskIndex {
						applyAction(&plan.Phases[i].Clusters[k].Tasks[j].Done, p.Action)
						if err := a.SavePlan(plan); err != nil {
							return "error saving plan: " + err.Error()
						}
						return fmt.Sprintf("Task %d %q in cluster %q marked as %s",
							p.TaskIndex, plan.Phases[i].Clusters[k].Tasks[j].Title, plan.Phases[i].Clusters[k].Title, doneState(plan.Phases[i].Clusters[k].Tasks[j].Done))
					}
					count++
				}
			}
		}
		return fmt.Sprintf("error: task index %d not found (plan has %d tasks)", p.TaskIndex, count)

	case "add_task":
		if p.TaskTitle == "" {
			return "error: task_title is required for add_task"
		}
		if len(plan.Phases) == 0 {
			return "error: plan has no phases to add a task to"
		}
		lastPhase := &plan.Phases[len(plan.Phases)-1]
		lastPhase.Tasks = append(lastPhase.Tasks, Task{
			Title:    p.TaskTitle,
			Done:     false,
			Priority: p.TaskPriority,
		})
		if err := a.SavePlan(plan); err != nil {
			return "error saving plan: " + err.Error()
		}
		return fmt.Sprintf("Added task %q to phase %q (total tasks now: %d)",
			p.TaskTitle, lastPhase.Title, countTasksInPlan(plan))

	default:
		return "error: unknown action '" + p.Action + "'. Available: toggle, set_done, set_undone, add_task"
	}
}

func applyAction(done *bool, action string) {
	switch action {
	case "toggle":
		*done = !*done
	case "set_done":
		*done = true
	case "set_undone":
		*done = false
	}
}

func doneState(done bool) string {
	if done {
		return "done"
	}
	return "not done"
}

func countTasksInPlan(p *JSONPlan) int {
	total := 0
	for _, phase := range p.Phases {
		total += len(phase.Tasks)
		for _, cluster := range phase.Clusters {
			total += len(cluster.Tasks)
		}
	}
	return total
}

func (a *App) toolPDFExtract(args json.RawMessage) string {
	var p struct {
		PdfID int64  `json:"pdf_id"`
		Pages string `json:"pages"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}

	filename, originalName, err := a.GetPDFFilenameAndOriginal(p.PdfID)
	if err != nil {
		return fmt.Sprintf("error: PDF not found (id: %d)", p.PdfID)
	}

	pdfPath := a.VaultPath("data", "pdf-files", filename)
	cacheDir := a.VaultPath("data", "pdf-texts")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "error creating cache dir: " + err.Error()
	}
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d.txt", p.PdfID))

	if cached, err := os.ReadFile(cachePath); err == nil {
		allPages := strings.Split(string(cached), "\n---PAGE BREAK---\n")
		if p.Pages != "" {
			selected := parsePageSelection(p.Pages, len(allPages))
			result := pickPages(allPages, selected)
			if len(result) == 0 {
				return "error: no pages found in range"
			}
			return fmt.Sprintf("PDF: %s (%d pages, extracted %d)\n\n%s",
				originalName, len(allPages), len(result), strings.Join(result, "\n\n"))
		}
		return fmt.Sprintf("PDF: %s (%d pages)\n\n%s", originalName, len(allPages), string(cached))
	}

	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		return "error: could not open PDF: " + err.Error()
	}
	defer f.Close()

	totalPages := r.NumPage()
	pageTexts := make([]string, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		plainText, err := r.Page(i).GetPlainText(nil)
		if err != nil {
			pageTexts = append(pageTexts, fmt.Sprintf("[error extracting page %d]", i))
		} else {
			pageTexts = append(pageTexts, plainText)
		}
	}

	cached := strings.Join(pageTexts, "\n---PAGE BREAK---\n")
	if err := os.WriteFile(cachePath, []byte(cached), 0644); err != nil {
		log.Printf("warn: failed to cache pdf text for id %d: %v", p.PdfID, err)
	}

	if p.Pages != "" {
		selected := parsePageSelection(p.Pages, totalPages)
		result := pickPages(pageTexts, selected)
		if len(result) == 0 {
			return "error: no pages found in range"
		}
		return fmt.Sprintf("PDF: %s (%d pages, extracted %d)\n\n%s",
			originalName, totalPages, len(result), strings.Join(result, "\n\n"))
	}

	return fmt.Sprintf("PDF: %s (%d pages)\n\n%s", originalName, totalPages, cached)
}

func pickPages(pages []string, indices []int) []string {
	var result []string
	for _, idx := range indices {
		if idx >= 0 && idx < len(pages) {
			result = append(result, fmt.Sprintf("### Page %d\n%s", idx+1, pages[idx]))
		}
	}
	return result
}

// ExtractAndCachePDFText writes a flat per-page text cache for the given
// PDF if one does not already exist. Safe to call from a goroutine.
func (a *App) ExtractAndCachePDFText(id int64, pdfPath string) {
	cacheDir := a.VaultPath("data", "pdf-texts")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		log.Printf("pdf cache dir: %v", err)
		return
	}
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d.txt", id))
	if _, err := os.Stat(cachePath); err == nil {
		return
	}

	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		log.Printf("PDF auto-extract failed for id %d: %v", id, err)
		return
	}
	defer f.Close()

	pageTexts := make([]string, 0, r.NumPage())
	for i := 1; i <= r.NumPage(); i++ {
		text, err := r.Page(i).GetPlainText(nil)
		if err != nil {
			pageTexts = append(pageTexts, fmt.Sprintf("[error extracting page %d]", i))
		} else {
			pageTexts = append(pageTexts, text)
		}
	}

	cached := strings.Join(pageTexts, "\n---PAGE BREAK---\n")
	if err := os.WriteFile(cachePath, []byte(cached), 0644); err != nil {
		log.Printf("pdf cache write id %d: %v", id, err)
		return
	}
	log.Printf("PDF auto-extracted id %d (%d pages)", id, r.NumPage())
}

// parsePageSelection turns a string like "1-5,7,10-12" into 0-based
// page indices, deduplicated and clamped to total.
func parsePageSelection(pages string, total int) []int {
	var result []int
	seen := make(map[int]bool)
	parts := strings.Split(pages, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil {
				continue
			}
			for i := start; i <= end && i <= total; i++ {
				idx := i - 1
				if idx >= 0 && idx < total && !seen[idx] {
					result = append(result, idx)
					seen[idx] = true
				}
			}
		} else {
			n, err := strconv.Atoi(part)
			if err != nil {
				continue
			}
			idx := n - 1
			if idx >= 0 && idx < total && !seen[idx] {
				result = append(result, idx)
				seen[idx] = true
			}
		}
	}
	return result
}

func toolWebFetch(args json.RawMessage) string {
	var p struct{ URL string }
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if !strings.HasPrefix(p.URL, "http://") && !strings.HasPrefix(p.URL, "https://") {
		return "error: only http:// and https:// URLs are allowed"
	}

	if wait := reserveWebFetchSlot(); wait > 0 {
		return fmt.Sprintf("rate limited: try again in %s", wait)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", p.URL, nil)
	if err != nil {
		return "error: " + err.Error()
	}
	req.Header.Set("User-Agent", "StudyAgent/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "error fetching URL: " + err.Error()
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Sprintf("error: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 500000))
	if err != nil {
		return "error reading response: " + err.Error()
	}

	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		converter := md.NewConverter("", true, nil)
		markdown, err := converter.ConvertString(string(body))
		if err != nil {
			return "error converting HTML: " + err.Error()
		}
		if len(markdown) > 50000 {
			markdown = markdown[:50000] + "\n\n[...truncated at 50,000 characters]"
		}
		title := ""
		if idx := strings.Index(markdown, "# "); idx != -1 {
			end := strings.Index(markdown[idx:], "\n")
			if end != -1 {
				title = markdown[idx+2 : idx+end]
			}
		}
		result := fmt.Sprintf("Source: %s", p.URL)
		if title != "" {
			result += "\nTitle: " + title
		}
		result += "\n\n" + markdown
		return result
	}

	text := string(body)
	if len(text) > 50000 {
		text = text[:50000] + "\n\n[...truncated at 50,000 characters]"
	}
	return fmt.Sprintf("Source: %s\n\n%s", p.URL, text)
}

// reserveWebFetchSlot enforces a rolling 5-per-minute limit. Returns
// the duration to wait before retrying, or 0 if a slot was reserved.
func reserveWebFetchSlot() time.Duration {
	webFetchMu.Lock()
	defer webFetchMu.Unlock()
	now := time.Now()
	cutoff := now.Add(-time.Minute)

	recent := webFetchTimes[:0]
	for _, t := range webFetchTimes {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}
	webFetchTimes = recent

	if len(recent) >= 5 {
		return recent[0].Add(time.Minute).Sub(now).Round(time.Second)
	}
	webFetchTimes = append(webFetchTimes, now)
	return 0
}

func (a *App) toolStudySkill(args json.RawMessage) string {
	var p struct {
		Skill  string            `json:"skill"`
		Params map[string]string `json:"params"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}

	params := p.Params
	if params == nil {
		params = map[string]string{}
	}

	topic := params["topic"]
	courseID := params["course_id"]
	courseName := CourseName(courseID)

	courseInterests := a.loadCourseInterests(courseID)
	corpusContent := a.loadCorpusContext(topic, courseID)

	switch p.Skill {
	case "orientation":
		return generateOrientation(topic, courseID, courseName, courseInterests, corpusContent)
	case "study_notes":
		return generateStudyNotes(topic, courseID, courseName, params["content"], courseInterests, corpusContent)
	case "self_test":
		count := 5
		if c, err := strconv.Atoi(params["count"]); err == nil && c > 0 && c <= 20 {
			count = c
		}
		return generateSelfTest(topic, courseID, courseName, count, courseInterests, corpusContent)
	case "review":
		return generateReview(topic, courseID, courseName, courseInterests, corpusContent)
	case "grill_me":
		return generateGrillMe(topic, courseID, courseName, courseInterests, corpusContent)
	default:
		return "error: unknown skill '" + p.Skill + "'. Available skills: orientation, study_notes, self_test, review, grill_me"
	}
}

func (a *App) loadCourseInterests(courseID string) string {
	if courseID == "" {
		return ""
	}
	data := readFileWithLog(a.VaultPath("data", "courses", courseID, "interests.md"))
	if data == "" {
		return ""
	}
	return "\n\nCourse interests and focus areas:\n" + data
}

func (a *App) loadCorpusContext(topic, courseID string) string {
	query := topic
	if courseID != "" {
		query = courseID + " " + topic
	}

	results, err := a.Search(query, courseID, 3)
	if err == nil && len(results) > 0 {
		var b strings.Builder
		for _, r := range results {
			heading := r.Heading
			if heading == "" {
				heading = r.ParentHeading
			}
			fmt.Fprintf(&b, "\n\n--- %s (%s) ---\n%s", r.SourceFile, heading, r.Content)
		}
		return b.String()
	}

	// Fallback 1: study-methods corpus dir
	if content := readDirAsCorpus(a.VaultPath("data", "corpus", "study-methods"), ""); content != "" {
		return content
	}
	// Fallback 2: course-specific corpus dir
	if courseID != "" {
		return readDirAsCorpus(a.VaultPath("data", "corpus", "courses", courseID), "course:"+courseID+"/")
	}
	return ""
}

func readDirAsCorpus(dir, sourcePrefix string) string {
	files, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var b strings.Builder
	for _, f := range files {
		if !strings.HasSuffix(f.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			continue
		}
		fmt.Fprintf(&b, "\n\n--- %s%s ---\n%s", sourcePrefix, strings.TrimSuffix(f.Name(), ".md"), string(data))
	}
	return b.String()
}

// GetSessionSystemPrompt builds the per-session system prompt by
// appending course-specific interests, the most recent fleeting note
// (for ce297), and a PDF excerpt if one is associated with the
// session.
func (a *App) GetSessionSystemPrompt(sessionID int64, basePrompt string) string {
	if sessionID == 0 {
		return basePrompt
	}
	courseID, topic, err := a.GetSessionCourseAndTopic(sessionID)
	if err != nil {
		return basePrompt
	}
	if courseID == "" {
		return basePrompt + "\n\n---\n\nYou are in a general study session (no specific course)."
	}

	courseName := CourseName(courseID)
	extra := "\n\n---\n\nYou are in a study session for **" + courseName + "** (course ID: " + courseID + ")."
	if topic != "" && topic != "General" {
		extra += " Topic: " + topic + "."
	}

	if data := readFileWithLog(a.VaultPath("data", "courses", courseID, "interests.md")); data != "" {
		extra += "\n\n" + courseName + " interests:\n" + data
	}

	if courseID == "ce297" {
		fleetingGlob := a.VaultPath("data", "courses", "ce297", "fleeting", "*.md")
		if matches, _ := filepath.Glob(fleetingGlob); len(matches) > 0 {
			lastFleeting := matches[len(matches)-1]
			if data, err := os.ReadFile(lastFleeting); err == nil {
				extra += "\n\nLatest fleeting note:\n" + string(data)
			} else {
				log.Printf("fleeting note unread: %s", lastFleeting)
			}
		}
	}

	if pdfID, _ := a.GetSessionLastPDFID(sessionID); pdfID > 0 {
		cachePath := a.VaultPath("data", "pdf-texts", fmt.Sprintf("%d.txt", pdfID))
		if data, err := os.ReadFile(cachePath); err == nil {
			text := string(data)
			if len(text) > 2000 {
				text = text[:2000] + "\n...[truncated, use pdf_extract for full content]"
			}
			pdfName, _ := a.PDFOriginalName(pdfID)
			extra += fmt.Sprintf("\n\n---\n\nCurrent PDF: **%s**\n\nExcerpt:\n%s", pdfName, text)
		}
	}

	return basePrompt + extra
}

// ---------- Skill prompt templates ----------

func generateOrientation(topic, courseID, courseName, courseInterests, corpusContent string) string {
	return buildSkillPrompt(skillTemplate{
		Title: "Study Orientation: " + topic,
		Body: `You are a study orientation assistant. Based on the topic and course context below, produce a practical pre-reading guide:

1. **Prerequisites** — What should the student already know before starting?
2. **Key Concepts** — 3-5 core ideas to focus on while reading
3. **Watch Points** — Common misconceptions or tricky parts to be aware of
4. **Study Approach** — Suggested method (examples-first, read-then-solve, etc.)
5. **Questions to Ask While Reading** — 3-5 questions to keep in mind during the study session

Be specific and practical. No generic advice.`,
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

func generateStudyNotes(topic, courseID, courseName, content, courseInterests, corpusContent string) string {
	body := `Generate structured study notes using this format:

## [Topic]

### Summary (2-3 sentences capturing the essence)

### Key Concepts
- Concept 1: brief explanation
- Concept 2: brief explanation

### Formulas / Definitions (if applicable)
- Formula/definition with context

### Connections to Other Topics
- How this relates to broader concepts

### Questions for Review
1. Question that tests understanding
2. Another question

Keep notes concise and exam-focused.`

	if content != "" {
		body += "\n\n### Source material to process:\n" + content
	}

	return buildSkillPrompt(skillTemplate{
		Title: "Study Notes Template: " + topic,
		Body:  body,
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

func generateSelfTest(topic, courseID, courseName string, count int, courseInterests, corpusContent string) string {
	body := fmt.Sprintf(`Generate %d exam-style questions about this topic. Mix these types:
- Conceptual understanding (explain in your own words)
- Calculation/application (solve a problem)
- Compare and contrast (differences and similarities)
- Identify the error (spot the mistake)

For each question, provide:
1. The question
2. A hint (in parentheses)
3. The expected answer

Format as a numbered quiz. Keep questions practical and exam-relevant.`, count)

	return buildSkillPrompt(skillTemplate{
		Title: "Self-Test: " + topic,
		Body:  body,
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

func generateReview(topic, courseID, courseName, courseInterests, corpusContent string) string {
	return buildSkillPrompt(skillTemplate{
		Title: "Spaced Repetition Review: " + topic,
		Body: `Assess the student's understanding of this topic through spaced repetition review:

1. Start with 2-3 quick recall questions (one at a time)
2. Based on how well they answer:
   - If strong: suggest the next topic and mark for later review
   - If shaky: provide a focused refresher on weak areas
   - If new: recommend starting with the orientation skill

Keep it conversational. Ask one question at a time.`,
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

func generateGrillMe(topic, courseID, courseName, courseInterests, corpusContent string) string {
	return buildSkillPrompt(skillTemplate{
		Title: "Grill Me: " + topic,
		Body: `You are in "grill me" mode. Interview the student relentlessly about their study plan, design decisions, or understanding of this topic until you reach a shared understanding.

Rules:
1. Walk down each branch of the decision tree, resolving dependencies between decisions one-by-one
2. Ask questions ONE AT A TIME — do not batch them
3. For each question, provide your recommended answer or perspective
4. If a question can be answered by exploring the course material or corpus, do so instead of asking the student
5. Be thorough but conversational — this is a dialogue, not an interrogation
6. Push back gently when answers are vague or hand-wavy
7. Surface assumptions the student hasn't articulated
8. When all branches are resolved, summarize what was learned and any remaining open questions`,
		Footer: "Start by asking the first question.",
	}, topic, courseID, courseName, courseInterests, corpusContent)
}

type skillTemplate struct {
	Title  string
	Body   string
	Footer string
}

func buildSkillPrompt(t skillTemplate, topic, courseID, courseName, courseInterests, corpusContent string) string {
	var b strings.Builder
	b.WriteString("## ")
	b.WriteString(t.Title)
	if courseName != "" {
		fmt.Fprintf(&b, " (%s)", courseName)
	}
	b.WriteString("\n\n")
	b.WriteString(t.Body)
	if courseInterests != "" {
		b.WriteString(courseInterests)
	}
	if corpusContent != "" {
		b.WriteString("\n\n### Relevant reference material:")
		b.WriteString(corpusContent)
	}
	fmt.Fprintf(&b, "\n\nTopic: %s\nCourse: %s (ID: %s)", topic, courseName, courseID)
	if t.Footer != "" {
		b.WriteString("\n\n")
		b.WriteString(t.Footer)
	}
	return b.String()
}
