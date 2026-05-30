package agent

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// CourseSettings holds the per-course declarative Steering knobs (ADR 0010,
// 0016). One row per course in the course_settings table; a course with no
// row resolves to DefaultCourseSettings (behavior-preserving). The agent
// never writes these into generated files — both the settings form and the
// claw-cli tool go through UpsertCourseSettings / SetCourseSetting.
type CourseSettings struct {
	CourseID      string `json:"course_id"`
	Framing       string `json:"framing"`
	ExamStyle     string `json:"exam_style"`
	ChunkPages    int    `json:"chunk_pages"`
	StopAfterTask bool   `json:"stop_after_task"`
	Interleaving  bool   `json:"interleaving"`
	UpdatedAt     int64  `json:"updated_at"`
}

// DefaultCourseSettings returns the behavior-preserving defaults that match
// the pre-Phase-4 hardcoded tutor behavior: ~8-page chunks, stop after each
// task, interleaved opener on.
func DefaultCourseSettings(courseID string) CourseSettings {
	return CourseSettings{
		CourseID:      courseID,
		ChunkPages:    8,
		StopAfterTask: true,
		Interleaving:  true,
	}
}

// ValidateCourseSettings enforces the bounds shared by both write paths.
func ValidateCourseSettings(s CourseSettings) error {
	if s.ChunkPages < 3 || s.ChunkPages > 30 {
		return fmt.Errorf("chunk_pages must be between 3 and 30, got %d", s.ChunkPages)
	}
	if len(s.Framing) > 4000 {
		return fmt.Errorf("framing too long (max 4000 chars)")
	}
	if len(s.ExamStyle) > 4000 {
		return fmt.Errorf("exam_style too long (max 4000 chars)")
	}
	return nil
}

// GetCourseSettings returns the stored settings for courseID, or the
// behavior-preserving defaults if no row exists. A DB error returns defaults
// plus the error (callers on the read path may ignore it safely).
func (a *App) GetCourseSettings(courseID string) (CourseSettings, error) {
	s := DefaultCourseSettings(courseID)
	var stop, inter int
	err := a.DB.QueryRow(
		"SELECT framing, exam_style, chunk_pages, stop_after_task, interleaving, updated_at FROM course_settings WHERE course_id = ?",
		courseID,
	).Scan(&s.Framing, &s.ExamStyle, &s.ChunkPages, &stop, &inter, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return s, nil
	}
	if err != nil {
		return DefaultCourseSettings(courseID), fmt.Errorf("query course settings %q: %w", courseID, err)
	}
	s.StopAfterTask = stop != 0
	s.Interleaving = inter != 0
	return s, nil
}

// UpsertCourseSettings writes the full settings row (insert or replace).
// Callers must validate first.
func (a *App) UpsertCourseSettings(s CourseSettings) error {
	if s.CourseID == "" {
		return fmt.Errorf("course id is required")
	}
	stop, inter := 0, 0
	if s.StopAfterTask {
		stop = 1
	}
	if s.Interleaving {
		inter = 1
	}
	_, err := a.DB.Exec(
		`INSERT INTO course_settings (course_id, framing, exam_style, chunk_pages, stop_after_task, interleaving, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(course_id) DO UPDATE SET
		   framing=excluded.framing, exam_style=excluded.exam_style, chunk_pages=excluded.chunk_pages,
		   stop_after_task=excluded.stop_after_task, interleaving=excluded.interleaving, updated_at=excluded.updated_at`,
		s.CourseID, s.Framing, s.ExamStyle, s.ChunkPages, stop, inter, time.Now().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("upsert course settings: %w", err)
	}
	return nil
}

// SetCourseSetting mutates a single knob by key, validates, and upserts.
// This is the deterministic write path used by the claw-cli tool (ADR 0016).
func (a *App) SetCourseSetting(courseID, key, value string) error {
	if courseID == "" {
		return fmt.Errorf("course id is required")
	}
	s, err := a.GetCourseSettings(courseID)
	if err != nil {
		return err
	}
	switch key {
	case "framing":
		s.Framing = value
	case "exam_style":
		s.ExamStyle = value
	case "chunk_pages":
		n, convErr := strconv.Atoi(strings.TrimSpace(value))
		if convErr != nil {
			return fmt.Errorf("chunk_pages must be an integer, got %q", value)
		}
		s.ChunkPages = n
	case "stop_after_task":
		b, bErr := parseBoolSetting(value)
		if bErr != nil {
			return bErr
		}
		s.StopAfterTask = b
	case "interleaving":
		b, bErr := parseBoolSetting(value)
		if bErr != nil {
			return bErr
		}
		s.Interleaving = b
	default:
		return fmt.Errorf("unknown setting key %q (valid: framing, exam_style, chunk_pages, stop_after_task, interleaving)", key)
	}
	if err := ValidateCourseSettings(s); err != nil {
		return err
	}
	return a.UpsertCourseSettings(s)
}

// parseBoolSetting accepts the human spellings the agent or a form might emit.
func parseBoolSetting(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "on", "yes":
		return true, nil
	case "false", "0", "off", "no":
		return false, nil
	default:
		return false, fmt.Errorf("expected a boolean (true/false/on/off), got %q", value)
	}
}
