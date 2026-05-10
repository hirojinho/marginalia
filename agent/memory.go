package agent

import (
	"database/sql"
	"fmt"
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
