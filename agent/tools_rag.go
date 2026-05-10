package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

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
