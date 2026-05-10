// Command convert is a one-shot CLI that transforms legacy plan JSON files
// from the old schema (sessions-based) to the current phases/tasks schema
// used by the study app. Run once per plan file; idempotent on already-
// converted files (no-ops if the target schema is already present).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Plan struct {
	ID       string    `json:"id"`
	Name     string    `json:"name"`
	Phases   []Phase   `json:"phases"`
	Sessions []Session `json:"sessions,omitempty"`
}

type Phase struct {
	Title    string    `json:"title"`
	Clusters []Cluster `json:"clusters,omitempty"`
	Tasks    []Task    `json:"tasks,omitempty"`
}

type Cluster struct {
	Title string `json:"title"`
	Tasks []Task `json:"tasks"`
}

type Task struct {
	Title    string `json:"title"`
	Done     bool   `json:"done"`
	Priority string `json:"priority,omitempty"`
	Notes    string `json:"notes,omitempty"`
}

type Session struct {
	Date  string `json:"date"`
	Topic string `json:"topic"`
	Time  string `json:"time"`
	Notes string `json:"notes,omitempty"`
}

var courseNames = map[string]string{
	"ce297":         "Safety Models and Techniques (CE-297)",
	"ddia":          "Designing Data-Intensive Applications",
	"dsa-interview": "DSA Interview Prep",
	"software-arch": "Software Architecture",
	"thesis":        "Thesis — Phase 1 Survey",
}

func main() {
	outDir := "/workspace/agent/data/plans"
	os.MkdirAll(outDir, 0755)

	for id := range courseNames {
		// Try memory path first
		memPath := fmt.Sprintf("/workspace/agent/memory/courses/%s/study-plan.md", id)
		if id == "thesis" {
			memPath = "/workspace/agent/memory/thesis/study-plan.md"
		}
		data, err := os.ReadFile(memPath)
		if err != nil {
			fmt.Printf("%s: no memory file found, skipping\n", id)
			continue
		}

		plan := convertPlan(id, courseNames[id], string(data))
		jsonData, _ := json.MarshalIndent(plan, "", "  ")
		outPath := filepath.Join(outDir, id+".json")
		os.WriteFile(outPath, jsonData, 0644)
		fmt.Printf("%s: %d phases, %d sessions\n", id, len(plan.Phases), len(plan.Sessions))
	}
}

func convertPlan(id, name, markdown string) Plan {
	plan := Plan{ID: id, Name: name}
	lines := strings.Split(markdown, "\n")

	var currentPhase *Phase
	var currentCluster *Cluster
	inSessionTable := false

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "|") && strings.Contains(trimmed, "Date") && strings.Contains(trimmed, "Topic") {
			inSessionTable = true
			continue
		}
		if inSessionTable {
			if strings.HasPrefix(trimmed, "|---") {
				continue
			}
			if strings.HasPrefix(trimmed, "|") && !strings.HasPrefix(trimmed, "| Date") {
				row := parseTableRow(trimmed)
				if len(row) >= 3 {
					s := Session{Date: row[0], Topic: row[1], Time: row[2]}
					if len(row) >= 4 {
						s.Notes = row[3]
					}
					plan.Sessions = append(plan.Sessions, s)
				}
				continue
			}
			if !strings.HasPrefix(trimmed, "|") {
				inSessionTable = false
			}
		}

		if strings.HasPrefix(trimmed, "## ") && !strings.HasPrefix(trimmed, "## Session") && !strings.HasPrefix(trimmed, "## Time") {
			if currentCluster != nil && len(currentCluster.Tasks) > 0 {
				if currentPhase != nil {
					currentPhase.Clusters = append(currentPhase.Clusters, *currentCluster)
				}
				currentCluster = nil
			}
			phase := Phase{Title: strings.TrimPrefix(trimmed, "## ")}
			if currentPhase != nil && len(currentPhase.Tasks) > 0 {
				plan.Phases = append(plan.Phases, *currentPhase)
			}
			if currentPhase != nil && len(currentPhase.Clusters) > 0 {
				plan.Phases = append(plan.Phases, *currentPhase)
			}
			currentPhase = &phase
			continue
		}

		if strings.HasPrefix(trimmed, "### ") && !strings.HasPrefix(trimmed, "### Deferred") {
			if currentCluster != nil && len(currentCluster.Tasks) > 0 {
				if currentPhase != nil {
					currentPhase.Clusters = append(currentPhase.Clusters, *currentCluster)
				}
			}
			cluster := Cluster{Title: strings.TrimPrefix(trimmed, "### ")}
			currentCluster = &cluster
			continue
		}

		if strings.HasPrefix(trimmed, "- [ ] ") || strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ") {
			done := strings.HasPrefix(trimmed, "- [x] ") || strings.HasPrefix(trimmed, "- [X] ")
			text := trimmed[6:]
			priority := ""
			if strings.Contains(text, "🔴") {
				priority = "high"
			} else if strings.Contains(text, "🟡") {
				priority = "medium"
			} else if strings.Contains(text, "🟢") {
				priority = "low"
			}
			t := Task{Title: text, Done: done, Priority: priority}

			// Look ahead for notes (indented continuation lines)
			var notes []string
			for j := i + 1; j < len(lines); j++ {
				next := strings.TrimSpace(lines[j])
				if next == "" || strings.HasPrefix(next, "- [") || strings.HasPrefix(next, "#") || strings.HasPrefix(next, "|") {
					// Check for non-empty continuation with notes-style formatting (multiple lines after a task)
					break
				}
				if strings.HasPrefix(next, "- ") || strings.HasPrefix(lines[j], "  ") || strings.HasPrefix(lines[j], "\t") {
					notes = append(notes, strings.TrimSpace(next))
				} else if next != "" {
					notes = append(notes, next)
				}
			}
			if len(notes) > 0 {
				t.Notes = strings.Join(notes, "\n")
			}

			if currentCluster != nil {
				currentCluster.Tasks = append(currentCluster.Tasks, t)
			} else if currentPhase != nil {
				currentPhase.Tasks = append(currentPhase.Tasks, t)
			} else {
				phase := Phase{Tasks: []Task{t}}
				currentPhase = &phase
			}
		}
		_ = i
	}

	// Flush remaining
	if currentCluster != nil && len(currentCluster.Tasks) > 0 {
		if currentPhase != nil {
			currentPhase.Clusters = append(currentPhase.Clusters, *currentCluster)
		}
	}
	if currentPhase != nil {
		if len(currentPhase.Clusters) > 0 || len(currentPhase.Tasks) > 0 {
			plan.Phases = append(plan.Phases, *currentPhase)
		}
	}

	return plan
}

func parseTableRow(line string) []string {
	line = strings.Trim(line, "|")
	cells := strings.Split(line, "|")
	var result []string
	for _, c := range cells {
		result = append(result, strings.TrimSpace(c))
	}
	return result
}
