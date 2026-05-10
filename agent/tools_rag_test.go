package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func seedCorpus(t *testing.T, a *App) {
	t.Helper()
	if err := a.InitVectorStore(); err != nil {
		t.Fatalf("init: %v", err)
	}
	now := time.Now().Format(time.RFC3339)
	rows := []struct{ path, heading, parent, content, course string }{
		{"a.md", "Replication", "DDIA", "Leader-based replication overview", "ddia"},
		{"b.md", "Sharding", "DDIA", "Range-based partitioning", "ddia"},
	}
	for _, r := range rows {
		if _, err := a.DB.Exec(
			`INSERT INTO corpus_chunks (path, heading, parent_heading, content, course_id, category, created_at, updated_at) VALUES (?, ?, ?, ?, ?, 'concept', ?, ?)`,
			r.path, r.heading, r.parent, r.content, r.course, now, now,
		); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func TestToolRAGSearch_BadJSON(t *testing.T) {
	a := newMemoryApp(t)
	if out := a.ToolRAGSearch(json.RawMessage(`bad`)); !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}

func TestToolRAGSearch_MissingQuery(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ToolRAGSearch(json.RawMessage(`{}`))
	if !strings.Contains(out, "query is required") {
		t.Fatalf("got %q", out)
	}
}

func TestToolRAGSearch_NoResults(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.InitVectorStore(); err != nil {
		t.Fatalf("init: %v", err)
	}
	out := a.ToolRAGSearch(json.RawMessage(`{"query":"nothing-matches-zzz"}`))
	if !strings.Contains(out, "No relevant results") {
		t.Fatalf("got %q", out)
	}
}

func TestToolRAGSearch_FindsResults(t *testing.T) {
	a := newMemoryApp(t)
	seedCorpus(t, a)
	out := a.ToolRAGSearch(json.RawMessage(`{"query":"replication","top_k":5}`))
	if !strings.Contains(out, "a.md") || !strings.Contains(out, "Leader-based") {
		t.Fatalf("got %q", out)
	}
}

func TestToolRAGSearch_TopKClamped(t *testing.T) {
	a := newMemoryApp(t)
	seedCorpus(t, a)
	// top_k > 10 clamped, top_k <= 0 defaulted; just ensure no error
	out := a.ToolRAGSearch(json.RawMessage(`{"query":"replication","top_k":99,"course":"ddia"}`))
	if !strings.Contains(out, "Leader-based") {
		t.Fatalf("got %q", out)
	}
	out2 := a.ToolRAGSearch(json.RawMessage(`{"query":"replication","course":"ddia"}`))
	if !strings.Contains(out2, "Leader-based") {
		t.Fatalf("got %q", out2)
	}
}

func TestLoadCorpusContext_KeywordHit(t *testing.T) {
	a := newMemoryApp(t)
	seedCorpus(t, a)
	got := a.loadCorpusContext("replication", "")
	if !strings.Contains(got, "Leader-based") {
		t.Fatalf("expected hit, got %q", got)
	}
}
