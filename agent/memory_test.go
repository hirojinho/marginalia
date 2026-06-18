package agent

import (
	"os"
	"strings"
	"testing"
	"time"
)

func newMemoryDB(t *testing.T) *MemoryStore {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	return NewMemoryStore(db)
}

func TestMemoryStoreSaveAssignsID(t *testing.T) {
	store := newMemoryDB(t)
	saved, err := store.Save(Memory{
		UserID:   "default",
		CourseID: "biology",
		Kind:     "feedback",
		Title:    "no abbreviations",
		Body:     "spell out terms",
	})
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.ID == 0 {
		t.Fatalf("expected non-zero id")
	}
	if saved.CreatedAt == 0 || saved.UpdatedAt == 0 {
		t.Fatalf("expected timestamps set, got %+v", saved)
	}
}

func TestMemoryStoreSearchMatchesTitleAndBody(t *testing.T) {
	store := newMemoryDB(t)
	for _, m := range []Memory{
		{UserID: "default", Kind: "feedback", Title: "abbreviations", Body: "spell out"},
		{UserID: "default", Kind: "feedback", Title: "density", Body: "match existing density"},
		{UserID: "default", Kind: "profile", Title: "user", Body: "the learner studies safety"},
	} {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	results, err := store.Search("default", "density", "", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Title != "density" {
		t.Fatalf("got %+v", results)
	}
	results, err = store.Search("default", "safety", "", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Kind != "profile" {
		t.Fatalf("got %+v", results)
	}
}

func TestMemoryStoreSearchCourseFilter(t *testing.T) {
	store := newMemoryDB(t)
	for _, m := range []Memory{
		{UserID: "default", CourseID: "biology", Kind: "feedback", Title: "biology-rule", Body: "x"},
		{UserID: "default", CourseID: "algorithms", Kind: "feedback", Title: "dsa-rule", Body: "x"},
		{UserID: "default", Kind: "feedback", Title: "global-rule", Body: "x"},
	} {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	results, err := store.Search("default", "rule", "biology", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	titles := []string{}
	for _, r := range results {
		titles = append(titles, r.Title)
	}
	if got := strings.Join(titles, ","); !strings.Contains(got, "biology-rule") || !strings.Contains(got, "global-rule") || strings.Contains(got, "dsa-rule") {
		t.Fatalf("course filter wrong, got titles: %s", got)
	}
}

func TestMemoryStoreLoadByScope(t *testing.T) {
	store := newMemoryDB(t)
	mems := []Memory{
		{UserID: "default", Kind: "profile", Title: "user", Body: "PROFILE_BODY"},
		{UserID: "default", CourseID: "biology", Kind: "project", Title: "biology-context", Body: "COURSE_BODY"},
		{UserID: "default", Kind: "feedback", Title: "global-style", Body: "FB_GLOBAL"},
		{UserID: "default", CourseID: "biology", Kind: "feedback", Title: "biology-style", Body: "FB_COURSE"},
		{UserID: "default", CourseID: "algorithms", Kind: "feedback", Title: "dsa-style", Body: "FB_DSA"},
	}
	for _, m := range mems {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	scope, err := store.LoadByScope("default", "biology")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if scope.Profile == nil || !strings.Contains(scope.Profile.Body, "PROFILE_BODY") {
		t.Fatalf("profile missing: %+v", scope.Profile)
	}
	if len(scope.CourseProjects) != 1 || scope.CourseProjects[0].Body != "COURSE_BODY" {
		t.Fatalf("course projects: %+v", scope.CourseProjects)
	}
	if len(scope.Feedback) != 2 {
		t.Fatalf("expected 2 feedback rows, got %+v", scope.Feedback)
	}
}

func TestRecentSessionsForCourse(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	now := time.Now().Format(time.RFC3339)
	for _, s := range []struct{ course, topic, summary string }{
		{"biology", "STAMP intro", "Read Leveson ch 4."},
		{"biology", "STPA step 2", "Hazards enumerated for elevator example."},
		{"biology", "STPA step 3", "Control structure drafted."},
		{"algorithms", "Bus Routes", "BFS over stop->buses graph."},
	} {
		if _, err := db.Exec(
			`INSERT INTO sessions (course_id, topic, created_at, updated_at, summary, summary_at)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			s.course, s.topic, now, now, s.summary, time.Now().Unix(),
		); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	got, err := RecentSessionsForCourse(db, "biology", 2)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
	if got[0].Topic != "STPA step 3" {
		t.Fatalf("expected most recent first, got %q", got[0].Topic)
	}
}

func TestParseSkillFrontmatter(t *testing.T) {
	dir := t.TempDir()
	content := []byte(`---
name: study-notes
description: Use when finishing a reading task
---
body here
`)
	skillDir := dir + "/study-notes"
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(skillDir+"/SKILL.md", content, 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	skills, err := ParseSkillsDir(dir)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(skills) != 1 || skills[0].Name != "study-notes" || !strings.Contains(skills[0].Description, "finishing") {
		t.Fatalf("got %+v", skills)
	}
}

func TestParseSkillsDirMissingReturnsEmpty(t *testing.T) {
	skills, err := ParseSkillsDir("/no/such/path")
	if err != nil {
		t.Fatalf("expected nil err for missing dir, got %v", err)
	}
	if len(skills) != 0 {
		t.Fatalf("expected empty, got %+v", skills)
	}
}

func TestAssembleAgentsMDIncludesAllSections(t *testing.T) {
	scope := Scope{
		Profile:        &Memory{Body: "The learner is a graduate student."},
		CourseProjects: []Memory{{Title: "Course arc", Body: "STAMP vs Avizienis."}},
		Feedback:       []Memory{{Title: "no abbreviations", Body: "spell out terms"}},
	}
	recent := []SessionDigest{{Topic: "STPA step 3", Summary: "Control structure drafted."}}
	skills := []SkillMeta{{Name: "study-notes", Description: "Use when finishing a reading task"}}
	out := AssembleAgentsMD(scope, recent, skills, "biology")
	for _, want := range []string{
		"## User profile", "graduate student",
		"## Course context: biology", "STAMP",
		"## Active feedback rules", "no abbreviations",
		"## Recent sessions", "STPA step 3",
		"## Available skills", "study-notes",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
	if len(out) > 3072 {
		t.Fatalf("over cap: %d bytes", len(out))
	}
}

func TestAssembleAgentsMDEmpty(t *testing.T) {
	out := AssembleAgentsMD(Scope{}, nil, nil, "")
	if !strings.Contains(out, "_(none yet)_") {
		t.Fatalf("expected fallback skill section, got:\n%s", out)
	}
}

func TestAssembleAgentsMDDropsBottomSectionsWhenOverCap(t *testing.T) {
	huge := strings.Repeat("x", 4000)
	scope := Scope{
		Profile:  &Memory{Body: huge},
		Feedback: []Memory{{Body: huge}},
	}
	recent := []SessionDigest{{Topic: "Recent", Summary: huge}}
	skills := []SkillMeta{{Name: "alpha", Description: huge}}
	out := AssembleAgentsMD(scope, recent, skills, "biology")
	if len(out) > 3072 {
		t.Fatalf("over cap: %d bytes", len(out))
	}
	if !strings.Contains(out, "## User profile") {
		t.Fatalf("profile dropped, must survive")
	}
}
