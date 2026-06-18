package agent

import (
	"strings"
	"testing"
)

func TestInferCourseID(t *testing.T) {
	cases := map[string]string{
		"/vault/courses/biology/notes.md":        "biology",
		"/vault/memory/courses/cs101/plan.md":   "cs101",
		"/vault/study-methods/zettelkasten.md": "",
		"/vault/random.md":                     "",
	}
	for path, want := range cases {
		if got := inferCourseID(path); got != want {
			t.Errorf("inferCourseID(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestInferCategory(t *testing.T) {
	cases := map[string]string{
		"/vault/study-methods/zettelkasten.md": "study-method",
		"/vault/courses/biology/notes.md":        "concept",
		"/vault/meta/principles.md":            "study-method",
		"/vault/random.md":                     "concept",
	}
	for path, want := range cases {
		if got := inferCategory(path); got != want {
			t.Errorf("inferCategory(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestExtractParentHeading(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"# Title\n\nbody", "Title"},
		{"  ## Subtitle\n# Real Title\n", "Real Title"},
		{"no heading here", ""},
		{"## only h2\n", ""},
		{"# Trim me   \n", "Trim me"},
	}
	for _, tc := range cases {
		if got := extractParentHeading(tc.in); got != tc.want {
			t.Errorf("extractParentHeading(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSplitLongChunksShortPassesThrough(t *testing.T) {
	in := []Chunk{{Path: "p", Heading: "h", Content: "small body"}}
	out := splitLongChunks(in)
	if len(out) != 1 || out[0].Content != "small body" {
		t.Fatalf("expected pass-through, got %+v", out)
	}
}

func TestSplitLongChunksSplitsAtParagraphBoundary(t *testing.T) {
	para := strings.Repeat("a", 800)
	content := para + "\n\n" + para + "\n\n" + para
	in := []Chunk{{Path: "p", Heading: "h", ParentHeading: "P", Content: content, CourseID: "c", Category: "concept"}}
	out := splitLongChunks(in)
	if len(out) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(out))
	}
	for i, c := range out {
		if c.Path != "p" || c.Heading != "h" || c.ParentHeading != "P" || c.CourseID != "c" || c.Category != "concept" {
			t.Errorf("chunk %d lost metadata: %+v", i, c)
		}
		if !strings.Contains(c.Content, "a") {
			t.Errorf("chunk %d empty content", i)
		}
	}
	for i, c := range out[:len(out)-1] {
		if len(c.Content) > 1500 {
			t.Errorf("chunk %d exceeds 1500 bytes: len=%d", i, len(c.Content))
		}
	}
}
