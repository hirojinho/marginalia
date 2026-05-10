package agent

import (
	"strings"
	"testing"
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
		UserID:   "eduardo",
		CourseID: "ce297",
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
		{UserID: "eduardo", Kind: "feedback", Title: "abbreviations", Body: "spell out"},
		{UserID: "eduardo", Kind: "feedback", Title: "density", Body: "match existing density"},
		{UserID: "eduardo", Kind: "profile", Title: "user", Body: "Eduardo studies safety"},
	} {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	results, err := store.Search("eduardo", "density", "", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 1 || results[0].Title != "density" {
		t.Fatalf("got %+v", results)
	}
	results, err = store.Search("eduardo", "safety", "", 20)
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
		{UserID: "eduardo", CourseID: "ce297", Kind: "feedback", Title: "ce297-rule", Body: "x"},
		{UserID: "eduardo", CourseID: "dsa-interview", Kind: "feedback", Title: "dsa-rule", Body: "x"},
		{UserID: "eduardo", Kind: "feedback", Title: "global-rule", Body: "x"},
	} {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	results, err := store.Search("eduardo", "rule", "ce297", 20)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	titles := []string{}
	for _, r := range results {
		titles = append(titles, r.Title)
	}
	if got := strings.Join(titles, ","); !strings.Contains(got, "ce297-rule") || !strings.Contains(got, "global-rule") || strings.Contains(got, "dsa-rule") {
		t.Fatalf("course filter wrong, got titles: %s", got)
	}
}

func TestMemoryStoreLoadByScope(t *testing.T) {
	store := newMemoryDB(t)
	mems := []Memory{
		{UserID: "eduardo", Kind: "profile", Title: "user", Body: "PROFILE_BODY"},
		{UserID: "eduardo", CourseID: "ce297", Kind: "project", Title: "ce297-context", Body: "COURSE_BODY"},
		{UserID: "eduardo", Kind: "feedback", Title: "global-style", Body: "FB_GLOBAL"},
		{UserID: "eduardo", CourseID: "ce297", Kind: "feedback", Title: "ce297-style", Body: "FB_COURSE"},
		{UserID: "eduardo", CourseID: "dsa-interview", Kind: "feedback", Title: "dsa-style", Body: "FB_DSA"},
	}
	for _, m := range mems {
		if _, err := store.Save(m); err != nil {
			t.Fatalf("save: %v", err)
		}
	}
	scope, err := store.LoadByScope("eduardo", "ce297")
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
