package agent

import (
	"bufio"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Memory struct {
	ID        int64
	UserID    string
	CourseID  string // empty string == NULL in DB
	Kind      string // "profile" | "feedback" | "project" | "reference"
	Title     string
	Body      string
	CreatedAt int64 // Unix seconds
	UpdatedAt int64
}

type Scope struct {
	Profile        *Memory
	CourseProjects []Memory // kind='project' AND course_id=?
	Feedback       []Memory // kind='feedback' AND (course_id=? OR course_id IS NULL)
}

type MemoryStore struct {
	db *sql.DB
}

func NewMemoryStore(db *sql.DB) *MemoryStore {
	return &MemoryStore{db: db}
}

func (s *MemoryStore) Save(m Memory) (Memory, error) {
	now := time.Now().Unix()
	if m.CreatedAt == 0 {
		m.CreatedAt = now
	}
	m.UpdatedAt = now
	var courseID any
	if m.CourseID != "" {
		courseID = m.CourseID
	}
	res, err := s.db.Exec(
		`INSERT INTO agent_memory (user_id, course_id, kind, title, body, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.UserID, courseID, m.Kind, m.Title, m.Body, m.CreatedAt, m.UpdatedAt,
	)
	if err != nil {
		return Memory{}, fmt.Errorf("memory save: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Memory{}, fmt.Errorf("memory save: last id: %w", err)
	}
	m.ID = id
	return m, nil
}

func (s *MemoryStore) Search(userID, query, courseID string, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 20
	}
	pattern := "%" + query + "%"
	var rows *sql.Rows
	var err error
	if courseID == "" {
		rows, err = s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory
			 WHERE user_id = ? AND (title LIKE ? OR body LIKE ?)
			 ORDER BY updated_at DESC LIMIT ?`,
			userID, pattern, pattern, limit,
		)
	} else {
		rows, err = s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory
			 WHERE user_id = ? AND (course_id = ? OR course_id IS NULL)
			   AND (title LIKE ? OR body LIKE ?)
			 ORDER BY updated_at DESC LIMIT ?`,
			userID, courseID, pattern, pattern, limit,
		)
	}
	if err != nil {
		return nil, fmt.Errorf("memory search: %w", err)
	}
	defer rows.Close()
	out := []Memory{}
	for rows.Next() {
		var m Memory
		if err := rows.Scan(&m.ID, &m.UserID, &m.CourseID, &m.Kind, &m.Title, &m.Body, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("memory scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *MemoryStore) LoadByScope(userID, courseID string) (Scope, error) {
	var scope Scope

	row := s.db.QueryRow(
		`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
		 FROM agent_memory
		 WHERE user_id = ? AND kind = 'profile'
		 ORDER BY updated_at DESC LIMIT 1`,
		userID,
	)
	var p Memory
	if err := row.Scan(&p.ID, &p.UserID, &p.CourseID, &p.Kind, &p.Title, &p.Body, &p.CreatedAt, &p.UpdatedAt); err == nil {
		scope.Profile = &p
	} else if err != sql.ErrNoRows {
		return scope, fmt.Errorf("scope profile: %w", err)
	}

	if courseID != "" {
		rows, err := s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory
			 WHERE user_id = ? AND course_id = ? AND kind = 'project'
			 ORDER BY updated_at DESC`,
			userID, courseID,
		)
		if err != nil {
			return scope, fmt.Errorf("scope course: %w", err)
		}
		for rows.Next() {
			var m Memory
			if err := rows.Scan(&m.ID, &m.UserID, &m.CourseID, &m.Kind, &m.Title, &m.Body, &m.CreatedAt, &m.UpdatedAt); err != nil {
				rows.Close()
				return scope, fmt.Errorf("scope course scan: %w", err)
			}
			scope.CourseProjects = append(scope.CourseProjects, m)
		}
		rows.Close()
	}

	var fbRows *sql.Rows
	var err error
	if courseID == "" {
		fbRows, err = s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory WHERE user_id = ? AND kind = 'feedback' AND course_id IS NULL
			 ORDER BY updated_at DESC`,
			userID,
		)
	} else {
		fbRows, err = s.db.Query(
			`SELECT id, user_id, IFNULL(course_id,''), kind, IFNULL(title,''), body, created_at, updated_at
			 FROM agent_memory WHERE user_id = ? AND kind = 'feedback'
			   AND (course_id = ? OR course_id IS NULL)
			 ORDER BY updated_at DESC`,
			userID, courseID,
		)
	}
	if err != nil {
		return scope, fmt.Errorf("scope feedback: %w", err)
	}
	defer fbRows.Close()
	for fbRows.Next() {
		var m Memory
		if err := fbRows.Scan(&m.ID, &m.UserID, &m.CourseID, &m.Kind, &m.Title, &m.Body, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return scope, fmt.Errorf("scope feedback scan: %w", err)
		}
		scope.Feedback = append(scope.Feedback, m)
	}
	return scope, fbRows.Err()
}

type SessionDigest struct {
	Topic   string
	Summary string
}

func RecentSessionsForCourse(db *sql.DB, courseID string, limit int) ([]SessionDigest, error) {
	if limit <= 0 {
		limit = 2
	}
	rows, err := db.Query(
		`SELECT topic, summary FROM sessions
		 WHERE course_id = ?
		 ORDER BY updated_at DESC, id DESC LIMIT ?`,
		courseID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("recent sessions: %w", err)
	}
	defer rows.Close()
	out := []SessionDigest{}
	for rows.Next() {
		var d SessionDigest
		if err := rows.Scan(&d.Topic, &d.Summary); err != nil {
			return nil, fmt.Errorf("recent sessions scan: %w", err)
		}
		d.Summary = TruncateRunes(d.Summary, 200)
		out = append(out, d)
	}
	return out, rows.Err()
}

type SkillMeta struct {
	Name        string
	Description string
}

func ParseSkillsDir(dir string) ([]SkillMeta, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("skills dir: %w", err)
	}
	var out []SkillMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name(), "SKILL.md")
		fm, err := parseFrontmatter(path)
		if err != nil {
			continue
		}
		out = append(out, SkillMeta{Name: fm["name"], Description: fm["description"]})
	}
	return out, nil
}

func parseFrontmatter(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() || strings.TrimSpace(sc.Text()) != "---" {
		return nil, fmt.Errorf("no frontmatter in %s", path)
	}
	out := map[string]string{}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "---" {
			return out, nil
		}
		idx := strings.Index(line, ":")
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		out[key] = val
	}
	return nil, fmt.Errorf("unterminated frontmatter in %s", path)
}

const (
	agentsMDTotalCap = 3072
	capProfile       = 500
	capCourse        = 800
	capFeedback      = 1200
	capRecent        = 500
	capSkills        = 500
)

func AssembleAgentsMD(scope Scope, recent []SessionDigest, skills []SkillMeta, courseID string) string {
	type section struct{ title, body string }
	var sections []section

	if scope.Profile != nil {
		sections = append(sections, section{"## User profile", TruncateRunes(scope.Profile.Body, capProfile)})
	} else {
		sections = append(sections, section{"## User profile", "_(none)_"})
	}

	if courseID != "" {
		var b strings.Builder
		for _, m := range scope.CourseProjects {
			if m.Title != "" {
				b.WriteString("- **")
				b.WriteString(m.Title)
				b.WriteString("**: ")
			} else {
				b.WriteString("- ")
			}
			b.WriteString(m.Body)
			b.WriteString("\n")
		}
		body := TruncateRunes(b.String(), capCourse)
		if body == "" {
			body = "_(none)_"
		}
		sections = append(sections, section{"## Course context: " + courseID, body})
	}

	{
		var b strings.Builder
		for _, m := range scope.Feedback {
			if m.Title != "" {
				b.WriteString("- **")
				b.WriteString(m.Title)
				b.WriteString("**: ")
			} else {
				b.WriteString("- ")
			}
			b.WriteString(m.Body)
			b.WriteString("\n")
		}
		body := TruncateRunes(b.String(), capFeedback)
		if body == "" {
			body = "_(none)_"
		}
		sections = append(sections, section{"## Active feedback rules", body})
	}

	{
		var b strings.Builder
		for _, d := range recent {
			b.WriteString("- ")
			b.WriteString(d.Topic)
			if d.Summary != "" {
				b.WriteString(" — ")
				b.WriteString(d.Summary)
			}
			b.WriteString("\n")
		}
		body := TruncateRunes(b.String(), capRecent)
		if body == "" {
			body = "_(none)_"
		}
		sections = append(sections, section{"## Recent sessions", body})
	}

	{
		var b strings.Builder
		for _, sk := range skills {
			b.WriteString("- `")
			b.WriteString(sk.Name)
			b.WriteString("` — ")
			b.WriteString(sk.Description)
			b.WriteString("\n")
		}
		body := TruncateRunes(b.String(), capSkills)
		if body == "" {
			body = "_(none yet)_"
		}
		sections = append(sections, section{"## Available skills", body})
	}

	render := func(secs []section) string {
		var b strings.Builder
		b.WriteString("# AGENTS.md\n\n")
		for _, s := range secs {
			b.WriteString(s.title)
			b.WriteString("\n\n")
			b.WriteString(s.body)
			if !strings.HasSuffix(s.body, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
		return b.String()
	}

	out := render(sections)
	for len(out) > agentsMDTotalCap && len(sections) > 1 {
		sections = sections[:len(sections)-1]
		out = render(sections)
	}
	return out
}
