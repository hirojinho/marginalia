package agent

import (
	"encoding/json"
	"fmt"
)

// ToolLogConfidence handles the log_confidence LLM tool.
func (a *App) ToolLogConfidence(args json.RawMessage) string {
	var p struct {
		KnowledgeComponentID string  `json:"knowledge_component_id"`
		Value                float64 `json:"value"`
		Raw                  string  `json:"raw"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.KnowledgeComponentID == "" {
		return "error: knowledge_component_id is required"
	}
	sessionID := a.ActiveSessionID()
	id, err := a.LogConfidence(sessionID, p.KnowledgeComponentID, p.Value, "tool_call", p.Raw)
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("logged confidence %.2f for knowledge component %s (row %d)", p.Value, p.KnowledgeComponentID, id)
}
