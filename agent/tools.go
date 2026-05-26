package agent

import (
	"encoding/json"
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

var KnownCourses = []struct {
	ID   string
	Name string
}{
	{"ce297", "Safety Models and Techniques (CE-297)"},
	{"ddia", "Designing Data-Intensive Applications"},
	{"dsa-interview", "DSA Interview Prep"},
	{"software-arch", "Software Architecture"},
	{"thesis", "Thesis — Phase 1 Survey"},
	{"guitar", "🎸 Guitar — Motivation-First Consistency"},
}

// CourseName returns the display name for a course ID.
// Prefer App.CourseName for new callers.
func CourseName(id string) string {
	for _, c := range KnownCourses {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

// AppCourseName returns the display name for a course ID via DB lookup.
func (a *App) AppCourseName(id string) string {
	c, _ := a.GetCourse(id)
	return c.Name
}

func GetTools() []ToolDef {
	return []ToolDef{
		{Type: "function", Function: ToolFunc{
			Name:        "read_file",
			Description: "Read a file from the workspace or vault",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string", "description": "Absolute file path or vault-relative path to read"},
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
					"path": map[string]interface{}{"type": "string", "description": "Directory path, absolute or vault-relative"},
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
		{Type: "function", Function: ToolFunc{
			Name:        "create_course",
			Description: "Create a new course. The course will appear immediately in the UI drawer.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"course_id":   map[string]interface{}{"type": "string", "description": "Course ID in kebab-case (e.g. linear-algebra, react-patterns)"},
					"course_name": map[string]interface{}{"type": "string", "description": "Display name for the course"},
				},
				"required": []string{"course_id", "course_name"},
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
		return a.ToolSaveNote(args)
	case "update_plan":
		return a.ToolUpdatePlan(args)
	case "pdf_extract":
		return a.ToolPDFExtract(args)
	case "web_fetch":
		return ToolWebFetch(args)
	case "study_skill":
		return a.ToolStudySkill(args)
	case "rag_search":
		return a.ToolRAGSearch(args)
	case "create_course":
		return a.ToolCreateCourse(args)
	}
	return "unknown tool: " + name
}

// ToolCreateCourse handles the create_course LLM tool.
func (a *App) ToolCreateCourse(args json.RawMessage) string {
	var p struct {
		CourseID   string `json:"course_id"`
		CourseName string `json:"course_name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.CourseID == "" || p.CourseName == "" {
		return "error: course_id and course_name are required"
	}
	if err := a.CreateCourse(p.CourseID, p.CourseName); err != nil {
		return "error: " + err.Error()
	}
	return "Created course '" + p.CourseName + "' (id: " + p.CourseID + ") — visible in drawer now."
}
