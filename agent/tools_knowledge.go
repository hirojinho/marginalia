package agent

import (
	"encoding/json"
	"fmt"
)

// ToolKnowledgeCreate handles the knowledge_create LLM tool.
func (a *App) ToolKnowledgeCreate(args json.RawMessage) string {
	var p struct {
		Title        string `json:"title"`
		Body         string `json:"body"`
		SourceTaskID string `json:"source_task_id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.Title == "" || p.Body == "" {
		return "error: title and body are both required"
	}
	id, err := a.CreateKnowledgeComponent(p.Title, p.Body, p.SourceTaskID, a.ActiveSessionID())
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("created knowledge component %s (%q)", id, p.Title)
}
