package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeriveCourseIDFromPath(t *testing.T) {
	cases := []struct{ in, root, want string }{
		{"/m/courses/biology/safety.md", "/m", "biology"},
		{"/m/courses/algorithms/interests.md", "/m", "algorithms"},
		{"/m/feedback_dsa_descriptive_names.md", "/m", ""},
		{"/m/user_profile.md", "/m", ""},
		{"/m/study_tracks/phase1.md", "/m", ""},
	}
	for _, c := range cases {
		got := deriveCourseID(c.in, c.root)
		if got != c.want {
			t.Fatalf("deriveCourseID(%q, %q) = %q, want %q", c.in, c.root, got, c.want)
		}
	}
}

func TestParseFrontmatterMinimal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x.md")
	content := []byte(`---
name: test
description: a test
type: feedback
---
body line 1
body line 2
`)
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	fm, body, err := parseFile(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if fm["type"] != "feedback" || fm["name"] != "test" {
		t.Fatalf("frontmatter: %+v", fm)
	}
	if body == "" || !contains(body, "body line 1") {
		t.Fatalf("body: %q", body)
	}
}

func TestParseFileNoFrontmatterReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "no-fm.md")
	if err := os.WriteFile(path, []byte("just body, no frontmatter"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := parseFile(path); err == nil {
		t.Fatalf("expected error")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

func TestCollectUsesFileModTimeForCreatedAt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.md")
	content := []byte("---\nname: test\ndescription: x\ntype: feedback\n---\nbody\n")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}
	// Set the file's mtime to a fixed past timestamp.
	wantTime := int64(1700000000) // 2023-11-14
	mt := time.Unix(wantTime, 0)
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}

	rows, err := collect(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}
	if rows[0].CreatedAt != wantTime {
		t.Fatalf("CreatedAt = %d, want %d", rows[0].CreatedAt, wantTime)
	}
}
