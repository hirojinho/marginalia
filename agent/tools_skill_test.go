package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadCourseInterests_EmptyCourseID(t *testing.T) {
	a := newMemoryApp(t)
	if got := a.loadCourseInterests(""); got != "" {
		t.Fatalf("empty course should return empty, got %q", got)
	}
}

func TestLoadCourseInterests_MissingFile(t *testing.T) {
	a := newMemoryApp(t)
	if got := a.loadCourseInterests("ce297"); got != "" {
		t.Fatalf("missing file should return empty, got %q", got)
	}
}

func TestLoadCourseInterests_PresentFile(t *testing.T) {
	a := newMemoryApp(t)
	dir := a.VaultPath("data", "courses", "ce297")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "interests.md"), []byte("formal methods"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := a.loadCourseInterests("ce297")
	if !strings.Contains(got, "formal methods") {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "Course interests") {
		t.Fatalf("missing prefix: %q", got)
	}
}

func TestReadDirAsCorpus_MissingDir(t *testing.T) {
	if got := readDirAsCorpus("/no/such/dir/zzz", ""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestReadDirAsCorpus_FiltersToMarkdown(t *testing.T) {
	d := t.TempDir()
	_ = os.WriteFile(filepath.Join(d, "a.md"), []byte("alpha"), 0644)
	_ = os.WriteFile(filepath.Join(d, "b.txt"), []byte("ignored"), 0644)
	_ = os.WriteFile(filepath.Join(d, "c.md"), []byte("charlie"), 0644)
	got := readDirAsCorpus(d, "pre:")
	if !strings.Contains(got, "alpha") || !strings.Contains(got, "charlie") {
		t.Fatalf("missing md content: %q", got)
	}
	if strings.Contains(got, "ignored") {
		t.Fatalf("non-md leaked: %q", got)
	}
	if !strings.Contains(got, "pre:a") || !strings.Contains(got, "pre:c") {
		t.Fatalf("missing source prefix: %q", got)
	}
}

func TestToolStudySkill_BadJSON(t *testing.T) {
	a := newMemoryApp(t)
	if out := a.ToolStudySkill(json.RawMessage(`bad`)); !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}

func TestToolStudySkill_AllSkillBranches(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.InitVectorStore(); err != nil {
		t.Fatalf("init: %v", err)
	}
	cases := []struct {
		skill string
		want  string
	}{
		{"orientation", "Study Orientation"},
		{"study_notes", "Study Notes Template"},
		{"self_test", "Self-Test"},
		{"review", "Spaced Repetition Review"},
		{"grill_me", "Grill Me"},
	}
	for _, tc := range cases {
		t.Run(tc.skill, func(t *testing.T) {
			args := json.RawMessage(`{"skill":"` + tc.skill + `","params":{"topic":"STPA","course_id":"ce297","count":"3"}}`)
			out := a.ToolStudySkill(args)
			if !strings.Contains(out, tc.want) {
				t.Fatalf("got %q", out)
			}
		})
	}
}

func TestLoadCorpusContext_FallbackToCorpusDir(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.InitVectorStore(); err != nil {
		t.Fatalf("init: %v", err)
	}
	dir := a.VaultPath("data", "corpus", "study-methods")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "method.md"), []byte("pomodoro"), 0644)
	got := a.loadCorpusContext("anything-not-in-db", "")
	if !strings.Contains(got, "pomodoro") {
		t.Fatalf("expected fallback content, got %q", got)
	}
}

func TestLoadCorpusContext_FallbackToCourseDir(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.InitVectorStore(); err != nil {
		t.Fatalf("init: %v", err)
	}
	dir := a.VaultPath("data", "corpus", "courses", "ce297")
	_ = os.MkdirAll(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "intro.md"), []byte("STAMP"), 0644)
	got := a.loadCorpusContext("missing-topic-zz", "ce297")
	if !strings.Contains(got, "STAMP") {
		t.Fatalf("got %q", got)
	}
}

func TestToolStudySkill_SelfTestCountClamping(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.InitVectorStore(); err != nil {
		t.Fatalf("init: %v", err)
	}
	out := a.ToolStudySkill(json.RawMessage(`{"skill":"self_test","params":{"topic":"X","count":"99"}}`))
	if !strings.Contains(out, "5 exam-style") {
		t.Fatalf("expected default 5 when count > 20, got %q", out)
	}
}

func TestToolStudySkill_UnknownSkill(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ToolStudySkill(json.RawMessage(`{"skill":"floof"}`))
	if !strings.Contains(out, "unknown skill") {
		t.Fatalf("got %q", out)
	}
}
