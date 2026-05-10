package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteTool_Unknown(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ExecuteTool("does_not_exist", json.RawMessage(`{}`))
	if !strings.HasPrefix(out, "unknown tool: ") {
		t.Fatalf("got %q", out)
	}
}

func TestExecuteTool_DispatchesReadFile(t *testing.T) {
	a := newMemoryApp(t)
	path := filepath.Join(a.Config.VaultRoot, "x.txt")
	_ = os.WriteFile(path, []byte("abc"), 0644)
	out := a.ExecuteTool("read_file", json.RawMessage(`{"Path":"`+path+`"}`))
	if out != "abc" {
		t.Fatalf("got %q", out)
	}
}

func TestExecuteTool_DispatchesSaveNote(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ExecuteTool("save_note", json.RawMessage(`{"path":"n.md","content":"x"}`))
	if !strings.HasPrefix(out, "saved to ") {
		t.Fatalf("got %q", out)
	}
}

func TestExecuteTool_DispatchesUpdatePlan(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ExecuteTool("update_plan", json.RawMessage(`{}`))
	if !strings.Contains(out, "plan_id is required") {
		t.Fatalf("got %q", out)
	}
}

func TestExecuteTool_DispatchesRAG(t *testing.T) {
	a := newMemoryApp(t)
	// missing query branch
	out := a.ExecuteTool("rag_search", json.RawMessage(`{}`))
	if !strings.Contains(out, "query is required") {
		t.Fatalf("got %q", out)
	}
}

func TestExecuteTool_DispatchesPDF(t *testing.T) {
	a := newMemoryApp(t)
	// missing PDF
	out := a.ExecuteTool("pdf_extract", json.RawMessage(`{"pdf_id":99999}`))
	if !strings.Contains(out, "PDF not found") {
		t.Fatalf("got %q", out)
	}
}

func TestExecuteTool_DispatchesWebFetch(t *testing.T) {
	resetWebFetchLimiter()
	a := newMemoryApp(t)
	out := a.ExecuteTool("web_fetch", json.RawMessage(`{"URL":"ftp://nope"}`))
	if !strings.Contains(out, "only http") {
		t.Fatalf("got %q", out)
	}
}

func TestExecuteTool_DispatchesStudySkill(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ExecuteTool("study_skill", json.RawMessage(`{"skill":"junk"}`))
	if !strings.Contains(out, "unknown skill") {
		t.Fatalf("got %q", out)
	}
}

func TestCourseName(t *testing.T) {
	if got := CourseName("ce297"); !strings.Contains(got, "CE-297") {
		t.Fatalf("ce297 got %q", got)
	}
	if got := CourseName("ddia"); got == "" {
		t.Fatalf("ddia should be known")
	}
	if got := CourseName("unknown-course-xyz"); got != "" {
		t.Fatalf("unknown should be empty, got %q", got)
	}
}

func TestGetTools_NotEmpty(t *testing.T) {
	tools := GetTools()
	if len(tools) < 5 {
		t.Fatalf("expected several tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, td := range tools {
		names[td.Function.Name] = true
	}
	for _, want := range []string{"read_file", "save_note", "update_plan", "pdf_extract", "rag_search", "web_fetch", "study_skill", "search_files", "list_files"} {
		if !names[want] {
			t.Fatalf("missing tool %q", want)
		}
	}
}
