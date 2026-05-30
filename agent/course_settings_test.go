package agent

import (
	"testing"
)

func newSettingsApp(t *testing.T) *App {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	return NewApp(Config{VaultRoot: t.TempDir()}, db)
}

func TestGetCourseSettingsReturnsDefaultsWhenNoRow(t *testing.T) {
	app := newSettingsApp(t)
	s, err := app.GetCourseSettings("ce297")
	if err != nil {
		t.Fatalf("GetCourseSettings: %v", err)
	}
	if s.CourseID != "ce297" || s.ChunkPages != 8 || !s.StopAfterTask || !s.Interleaving {
		t.Fatalf("defaults wrong: %+v", s)
	}
	if s.Framing != "" || s.ExamStyle != "" {
		t.Fatalf("expected empty text defaults: %+v", s)
	}
}

func TestSetCourseSettingPersistsAndRoundTrips(t *testing.T) {
	app := newSettingsApp(t)
	cases := []struct{ key, val string }{
		{"framing", "exam-prep first"},
		{"exam_style", "conceptual oral"},
		{"chunk_pages", "6"},
		{"stop_after_task", "false"},
		{"interleaving", "off"},
	}
	for _, c := range cases {
		if err := app.SetCourseSetting("ce297", c.key, c.val); err != nil {
			t.Fatalf("set %s=%s: %v", c.key, c.val, err)
		}
	}
	s, err := app.GetCourseSettings("ce297")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.Framing != "exam-prep first" || s.ExamStyle != "conceptual oral" ||
		s.ChunkPages != 6 || s.StopAfterTask || s.Interleaving {
		t.Fatalf("round-trip wrong: %+v", s)
	}
}

func TestSetCourseSettingRejectsBadValues(t *testing.T) {
	app := newSettingsApp(t)
	if err := app.SetCourseSetting("ce297", "chunk_pages", "999"); err == nil {
		t.Fatal("expected chunk_pages range error")
	}
	if err := app.SetCourseSetting("ce297", "chunk_pages", "notanint"); err == nil {
		t.Fatal("expected chunk_pages parse error")
	}
	if err := app.SetCourseSetting("ce297", "bogus_key", "x"); err == nil {
		t.Fatal("expected unknown-key error")
	}
	if err := app.SetCourseSetting("ce297", "stop_after_task", "maybe"); err == nil {
		t.Fatal("expected bool parse error")
	}
}

func TestValidateCourseSettings(t *testing.T) {
	if err := ValidateCourseSettings(CourseSettings{CourseID: "x", ChunkPages: 2}); err == nil {
		t.Fatal("expected chunk_pages<3 to fail validation")
	}
	if err := ValidateCourseSettings(CourseSettings{CourseID: "x", ChunkPages: 8}); err != nil {
		t.Fatalf("valid settings rejected: %v", err)
	}
}
