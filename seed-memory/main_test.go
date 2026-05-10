package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeriveCourseIDFromPath(t *testing.T) {
	cases := []struct{ in, root, want string }{
		{"/m/courses/ce297/safety.md", "/m", "ce297"},
		{"/m/courses/dsa-interview/interests.md", "/m", "dsa-interview"},
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
