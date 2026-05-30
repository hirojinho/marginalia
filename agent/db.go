package agent

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Event is a single observability record. Fields not relevant to a given
// kind are left at their zero value (SessionID and OK use pointers so
// NULL is distinguishable from false/0).
type Event struct {
	ID           int64
	Kind         string
	SessionID    *int64
	CourseID     string
	ToolName     string
	Model        string
	InputTokens  int
	OutputTokens int
	DurationMs   int64
	OK           *bool
	CreatedAt    int64 // unix milliseconds
}

// EventSummary holds pre-aggregated metrics over a time window.
type EventSummary struct {
	TurnCount    int
	AvgLatencyMs int64
	P95LatencyMs int64
	InputTokens  int64
	OutputTokens int64
	ToolCounts   map[string]int
	CourseCounts map[string]int
	PlanDone     int
	PlanUndone   int
	PDFOpens     int
}

// OpenDB opens the SQLite database at path and applies pragmas
// required for safe concurrent operation (WAL mode, busy timeout,
// foreign keys, balanced sync). Returns the *sql.DB ready to use.
func OpenDB(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
		"PRAGMA synchronous = NORMAL",
		"PRAGMA cache_size = -2000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	// Modernc sqlite is single-threaded per conn; let the driver pool
	// handle concurrency, but keep the upper bound modest.
	db.SetMaxOpenConns(8)
	db.SetMaxIdleConns(4)

	return db, nil
}

// InitSchema creates all tables and applies idempotent migrations.
func InitSchema(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS pdfs (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		filename      TEXT NOT NULL,
		original_name TEXT NOT NULL,
		course_id     TEXT,
		pages         INTEGER NOT NULL DEFAULT 0,
		last_page     INTEGER NOT NULL DEFAULT 1,
		uploaded_at   TEXT NOT NULL,
		last_read_at  TEXT
	);
	CREATE TABLE IF NOT EXISTS meta (
		key   TEXT PRIMARY KEY,
		value TEXT NOT NULL
	);
	CREATE TABLE IF NOT EXISTS sessions (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		course_id   TEXT,
		topic       TEXT NOT NULL,
		created_at  TEXT NOT NULL,
		updated_at  TEXT NOT NULL,
		last_pdf_id INTEGER,
		last_page   INTEGER DEFAULT 1,
		summary     TEXT NOT NULL DEFAULT '',
		summary_at  INTEGER NOT NULL DEFAULT 0
	);
	CREATE TABLE IF NOT EXISTS messages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		role       TEXT NOT NULL,
		content    TEXT NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
	);
	CREATE TABLE IF NOT EXISTS agent_memory (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id     TEXT NOT NULL,
		course_id   TEXT,
		kind        TEXT NOT NULL,
		title       TEXT,
		body        TEXT NOT NULL,
		created_at  INTEGER NOT NULL,
		updated_at  INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS agent_memory_scope ON agent_memory (user_id, course_id, kind);
	CREATE TABLE IF NOT EXISTS events (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		kind          TEXT    NOT NULL,
		session_id    INTEGER,
		course_id     TEXT,
		tool_name     TEXT,
		model         TEXT,
		input_tokens  INTEGER,
		output_tokens INTEGER,
		duration_ms   INTEGER,
		ok            INTEGER,
		created_at    INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS events_created ON events(created_at);
	CREATE INDEX IF NOT EXISTS events_kind_created ON events(kind, created_at);
	CREATE TABLE IF NOT EXISTS courses (
		id          TEXT PRIMARY KEY,
		name        TEXT NOT NULL,
		created_at  INTEGER NOT NULL
	);
	CREATE TABLE IF NOT EXISTS confidence_log (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id  INTEGER REFERENCES sessions(id) ON DELETE CASCADE,
		knowledge_component_id TEXT NOT NULL,
		value       REAL    NOT NULL CHECK (value >= 0.0 AND value <= 1.0),
		source      TEXT    NOT NULL CHECK (source IN ('tool_call','inferred','manual','verifier')),
		created_at  INTEGER NOT NULL,
		raw_text    TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_confidence_log_kc ON confidence_log(knowledge_component_id, created_at);
	CREATE INDEX IF NOT EXISTS idx_confidence_log_session ON confidence_log(session_id, created_at);
	CREATE TABLE IF NOT EXISTS knowledge_components (
	    id                 TEXT    PRIMARY KEY,
	    title              TEXT    NOT NULL,
	    body               TEXT    NOT NULL,
	    source_task_id     TEXT,
	    source_session_id  INTEGER REFERENCES sessions(id) ON DELETE SET NULL,
	    created_at         INTEGER NOT NULL,
	    updated_at         INTEGER NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_knowledge_components_task ON knowledge_components(source_task_id);
	`
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	// Idempotent migrations for tables created before the schema
	// included these columns. Suppress only the specific "duplicate
	// column" sentinel; any other error is real.
	migrations := []string{
		"ALTER TABLE sessions ADD COLUMN summary TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN summary_at INTEGER DEFAULT 0",
		"ALTER TABLE messages ADD COLUMN reasoning TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE confidence_log RENAME COLUMN kc_id TO knowledge_component_id",
		"ALTER TABLE sessions ADD COLUMN task_id TEXT",
		"ALTER TABLE sessions ADD COLUMN archived INTEGER NOT NULL DEFAULT 0",
		"ALTER TABLE sessions ADD COLUMN hidden INTEGER NOT NULL DEFAULT 0",
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") && !strings.Contains(err.Error(), "no such column") {
			slog.Warn("schema migration", "stmt", m, "err", err)
		}
	}

	// Seed courses from compile-time KnownCourses if table is empty.
	var courseCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM courses").Scan(&courseCount)
	if courseCount == 0 {
		now := time.Now().UnixMilli()
		for _, c := range KnownCourses {
			_, _ = db.Exec("INSERT INTO courses(id, name, created_at) VALUES (?, ?, ?)",
				c.ID, c.Name, now)
		}
	}

	return nil
}

// ---------- Courses ----------

// ListCourses returns all courses ordered by creation time.
func (a *App) ListCourses() ([]Course, error) {
	rows, err := a.DB.Query("SELECT id, name, created_at FROM courses ORDER BY created_at ASC")
	if err != nil {
		return nil, fmt.Errorf("query courses: %w", err)
	}
	defer rows.Close()
	var courses []Course
	for rows.Next() {
		var c Course
		if err := rows.Scan(&c.ID, &c.Name, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan course: %w", err)
		}
		courses = append(courses, c)
	}
	return courses, rows.Err()
}

// GetCourse returns a single course by ID. Returns an empty Course
// and nil error if not found.
func (a *App) GetCourse(id string) (Course, error) {
	var c Course
	err := a.DB.QueryRow("SELECT id, name, created_at FROM courses WHERE id = ?", id).
		Scan(&c.ID, &c.Name, &c.CreatedAt)
	if err == sql.ErrNoRows {
		return Course{}, nil
	}
	if err != nil {
		return Course{}, fmt.Errorf("query course %q: %w", id, err)
	}
	return c, nil
}

// CreateCourse inserts a new course. Returns an error wrapping a
// unique-constraint violation as a friendly message.
func (a *App) CreateCourse(id, name string) error {
	_, err := a.DB.Exec("INSERT INTO courses(id, name, created_at) VALUES (?, ?, ?)",
		id, name, time.Now().UnixMilli())
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("course already exists: %s", id)
		}
		return fmt.Errorf("insert course: %w", err)
	}
	return nil
}

// ---------- Sessions ----------

func (a *App) CreateSession(courseID, topic string) (Session, error) {
	if topic == "" {
		topic = "General"
	}
	now := time.Now().Format(time.RFC3339)
	res, err := a.DB.Exec(
		"INSERT INTO sessions (course_id, topic, created_at, updated_at) VALUES (?, ?, ?, ?)",
		courseID, topic, now, now,
	)
	if err != nil {
		return Session{}, fmt.Errorf("insert session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Session{}, fmt.Errorf("last insert id: %w", err)
	}
	if err := a.setMetaInt("last_session", id); err != nil {
		return Session{}, fmt.Errorf("set last_session: %w", err)
	}
	a.SetActiveSessionIDInMemory(id)
	return Session{
		ID:        id,
		CourseID:  courseID,
		Topic:     topic,
		CreatedAt: now,
		UpdatedAt: now,
		LastPage:  1,
	}, nil
}

func (a *App) ListSessions() ([]Session, error) {
	rows, err := a.DB.Query("SELECT id, course_id, task_id, topic, created_at, updated_at, last_pdf_id, last_page, archived FROM sessions WHERE hidden = 0 ORDER BY updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.CourseID, &s.TaskID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage, &s.Archived); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		if s.LastPdfID != nil {
			if name, err := a.PDFOriginalName(*s.LastPdfID); err == nil {
				s.PdfName = name
			}
		}
		sessions = append(sessions, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sessions: %w", err)
	}
	return sessions, nil
}

func (a *App) GetSession(id int64) (Session, error) {
	var s Session
	err := a.DB.QueryRow(
		"SELECT id, course_id, task_id, topic, created_at, updated_at, last_pdf_id, last_page, archived FROM sessions WHERE id = ?",
		id,
	).Scan(&s.ID, &s.CourseID, &s.TaskID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage, &s.Archived)
	if err != nil {
		return Session{}, err
	}
	if s.LastPdfID != nil {
		if name, err := a.PDFOriginalName(*s.LastPdfID); err == nil {
			s.PdfName = name
		}
	}
	return s, nil
}

func (a *App) DeleteSession(id int64) error {
	// FK cascade handles messages, but be explicit so behaviour is the
	// same regardless of whether the FK was added before the cascade
	// rule.
	if _, err := a.DB.Exec("DELETE FROM messages WHERE session_id = ?", id); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if _, err := a.DB.Exec("DELETE FROM sessions WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	if a.ActiveSessionID() == id {
		a.SetActiveSessionIDInMemory(0)
		if err := a.setMetaInt("last_session", 0); err != nil {
			return fmt.Errorf("clear last_session: %w", err)
		}
	}
	if a.Sandbox != nil {
		if sandboxErr := a.Sandbox.Delete(id); sandboxErr != nil {
			return fmt.Errorf("delete sandbox: %w", sandboxErr)
		}
	}
	return nil
}

func (a *App) SessionExists(id int64) (bool, error) {
	var n int64
	err := a.DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", id).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("count session: %w", err)
	}
	return n > 0, nil
}

// SetActiveSession updates both the in-memory active id and the
// persisted "last_session" meta entry. Returns an error if id does not
// exist (in-memory state is left unchanged).
func (a *App) SetActiveSession(id int64) error {
	exists, err := a.SessionExists(id)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("session %d not found", id)
	}
	if err := a.setMetaInt("last_session", id); err != nil {
		return err
	}
	if _, err := a.DB.Exec("UPDATE sessions SET updated_at = ? WHERE id = ?", time.Now().Format(time.RFC3339), id); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	a.SetActiveSessionIDInMemory(id)
	return nil
}

// LoadActiveSessionID reads the persisted active session id from meta
// and stores it in memory. Call once at startup.
func (a *App) LoadActiveSessionID() {
	id, err := a.getMetaInt("last_session")
	if err != nil {
		slog.Warn("load active session id", "err", err)
	}
	a.SetActiveSessionIDInMemory(id)
}

// UpdateSessionTopic sets a new topic on session id and bumps updated_at.
func (a *App) UpdateSessionTopic(id int64, topic string) error {
	now := time.Now().Format(time.RFC3339)
	res, err := a.DB.Exec("UPDATE sessions SET topic = ?, updated_at = ? WHERE id = ?", topic, now, id)
	if err != nil {
		return fmt.Errorf("update topic: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("session not found: %d", id)
	}
	return nil
}

// UpdateSessionPDF records which PDF the session is reading and the page
// reached, and bumps updated_at. Lets the tutor know the learner's reading
// position (ADR 0012).
func (a *App) UpdateSessionPDF(id, pdfID int64, page int) error {
	now := time.Now().Format(time.RFC3339)
	res, err := a.DB.Exec("UPDATE sessions SET last_pdf_id = ?, last_page = ?, updated_at = ? WHERE id = ?", pdfID, page, now, id)
	if err != nil {
		return fmt.Errorf("update session pdf: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("session not found: %d", id)
	}
	return nil
}

func (a *App) UpdateSessionSummary(id int64, summary string, summaryAt int) error {
	if _, err := a.DB.Exec("UPDATE sessions SET summary = ?, summary_at = ? WHERE id = ?", summary, summaryAt, id); err != nil {
		return fmt.Errorf("update summary: %w", err)
	}
	return nil
}

func (a *App) GetSessionSummary(id int64) (summary string, summaryAt int, err error) {
	err = a.DB.QueryRow("SELECT summary, summary_at FROM sessions WHERE id = ?", id).Scan(&summary, &summaryAt)
	return
}

func (a *App) GetSessionCourseAndTopic(id int64) (courseID, topic string, err error) {
	err = a.DB.QueryRow("SELECT course_id, topic FROM sessions WHERE id = ?", id).Scan(&courseID, &topic)
	return
}

func (a *App) GetSessionLastPDFID(id int64) (int64, error) {
	var pdfID int64
	err := a.DB.QueryRow("SELECT COALESCE(last_pdf_id, 0) FROM sessions WHERE id = ?", id).Scan(&pdfID)
	return pdfID, err
}

// ---------- Messages ----------

func (a *App) SaveMessage(sessionID int64, role, content string) error {
	now := time.Now().Format(time.RFC3339)
	if _, err := a.DB.Exec(
		"INSERT INTO messages (session_id, role, content, created_at) VALUES (?, ?, ?, ?)",
		sessionID, role, content, now,
	); err != nil {
		return fmt.Errorf("insert message: %w", err)
	}
	if _, err := a.DB.Exec("UPDATE sessions SET updated_at = ? WHERE id = ?", now, sessionID); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

func (a *App) GetSessionHistory(sessionID int64) ([]Message, error) {
	rows, err := a.DB.Query("SELECT role, content, reasoning FROM messages WHERE session_id = ? ORDER BY id", sessionID)
	if err != nil {
		return nil, fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.Role, &m.Content, &m.Reasoning); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		msgs = append(msgs, m)
	}
	return msgs, rows.Err()
}

// GetSessionStats returns per-session aggregate counters for messages.
func (a *App) GetSessionStats(sessionID int64) (SessionStats, error) {
	// Verify session exists first.
	var exists int
	err := a.DB.QueryRow("SELECT 1 FROM sessions WHERE id = ?", sessionID).Scan(&exists)
	if err == sql.ErrNoRows {
		return SessionStats{}, fmt.Errorf("session not found: %w", sql.ErrNoRows)
	}
	if err != nil {
		return SessionStats{}, fmt.Errorf("check session: %w", err)
	}

	var stats SessionStats
	stats.SessionID = sessionID

	var firstMsg, lastMsg sql.NullString
	err = a.DB.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN role = 'user' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN role = 'assistant' THEN 1 ELSE 0 END), 0),
			MIN(created_at),
			MAX(created_at),
			COALESCE(SUM(LENGTH(reasoning)), 0)
		FROM messages WHERE session_id = ?
	`, sessionID).Scan(
		&stats.MessageCount,
		&stats.UserMessageCount,
		&stats.AssistantMessageCount,
		&firstMsg,
		&lastMsg,
		&stats.TotalReasoningChars,
	)
	if err != nil {
		return SessionStats{}, fmt.Errorf("query session stats: %w", err)
	}

	if firstMsg.Valid {
		stats.FirstMessageAt = &firstMsg.String
	}
	if lastMsg.Valid {
		stats.LastMessageAt = &lastMsg.String
	}

	return stats, nil
}

// SaveAssistantMessage persists an assistant turn with its optional reasoning text.
func (a *App) SaveAssistantMessage(sessionID int64, content, reasoning string) error {
	now := time.Now().Format(time.RFC3339)
	if _, err := a.DB.Exec(
		"INSERT INTO messages (session_id, role, content, reasoning, created_at) VALUES (?, 'assistant', ?, ?, ?)",
		sessionID, content, reasoning, now,
	); err != nil {
		return fmt.Errorf("insert assistant message: %w", err)
	}
	if _, err := a.DB.Exec("UPDATE sessions SET updated_at = ? WHERE id = ?", now, sessionID); err != nil {
		return fmt.Errorf("touch session: %w", err)
	}
	return nil
}

// GetSessionHistoryWithSummary returns the conversation history,
// substituting a synthetic summary message for the older portion when
// a summary is present and the recent message count exceeds the
// summary mark.
func (a *App) GetSessionHistoryWithSummary(sessionID int64) ([]Message, error) {
	summary, summaryAt, err := a.GetSessionSummary(sessionID)
	if err != nil {
		return nil, err
	}
	all, err := a.GetSessionHistory(sessionID)
	if err != nil {
		return nil, err
	}

	if summary != "" && len(all) > summaryAt+10 {
		summaryMsg := Message{
			Role:    "system",
			Content: "Previous conversation summary:\n" + summary,
		}
		recent := all
		if len(all) > 10 {
			recent = all[len(all)-10:]
		}
		out := make([]Message, 0, 1+len(recent))
		out = append(out, summaryMsg)
		out = append(out, recent...)
		return out, nil
	}
	return all, nil
}

func (a *App) GetMessageCount(sessionID int64) (int, error) {
	var count int
	err := a.DB.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = ?", sessionID).Scan(&count)
	return count, err
}

// ---------- Events ----------

// RecordEvent inserts one observability event. SessionID and OK are stored
// as NULL when their pointer is nil.
func (a *App) RecordEvent(e Event) error {
	var sessionID interface{}
	if e.SessionID != nil {
		sessionID = *e.SessionID
	}
	var ok interface{}
	if e.OK != nil {
		if *e.OK {
			ok = 1
		} else {
			ok = 0
		}
	}
	_, err := a.DB.Exec(
		`INSERT INTO events
		 (kind, session_id, course_id, tool_name, model,
		  input_tokens, output_tokens, duration_ms, ok, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.Kind, sessionID, e.CourseID, e.ToolName, e.Model,
		e.InputTokens, e.OutputTokens, e.DurationMs, ok, e.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}
	return nil
}

// ListRecentEvents returns up to limit events ordered newest-first.
func (a *App) ListRecentEvents(limit int) ([]Event, error) {
	rows, err := a.DB.Query(
		`SELECT id, kind, session_id, course_id, tool_name, model,
		        input_tokens, output_tokens, duration_ms, ok, created_at
		 FROM events ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var evs []Event
	for rows.Next() {
		var e Event
		var sid sql.NullInt64
		var okVal sql.NullInt64
		if err := rows.Scan(
			&e.ID, &e.Kind, &sid, &e.CourseID, &e.ToolName, &e.Model,
			&e.InputTokens, &e.OutputTokens, &e.DurationMs, &okVal, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if sid.Valid {
			v := sid.Int64
			e.SessionID = &v
		}
		if okVal.Valid {
			b := okVal.Int64 == 1
			e.OK = &b
		}
		evs = append(evs, e)
	}
	return evs, rows.Err()
}

// PruneOldEvents deletes events older than before. Returns the number of rows deleted.
func (a *App) PruneOldEvents(before time.Time) (int64, error) {
	res, err := a.DB.Exec("DELETE FROM events WHERE created_at < ?", before.UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("prune events: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// QueryEventSummary returns aggregated metrics for events recorded after since.
func (a *App) QueryEventSummary(since time.Time) (EventSummary, error) {
	sinceMs := since.UnixMilli()
	s := EventSummary{
		ToolCounts:   make(map[string]int),
		CourseCounts: make(map[string]int),
	}

	// chat_turn aggregates
	row := a.DB.QueryRow(
		`SELECT COUNT(*), COALESCE(AVG(duration_ms),0),
		        COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0)
		 FROM events WHERE kind='chat_turn' AND created_at >= ?`, sinceMs)
	if err := row.Scan(&s.TurnCount, &s.AvgLatencyMs, &s.InputTokens, &s.OutputTokens); err != nil {
		return s, fmt.Errorf("chat_turn aggregates: %w", err)
	}

	// p95 latency: row at 95th percentile position
	if s.TurnCount > 0 {
		offset := int(float64(s.TurnCount)*0.95) - 1
		if offset < 0 {
			offset = 0
		}
		p95Row := a.DB.QueryRow(
			`SELECT duration_ms FROM events
			 WHERE kind='chat_turn' AND created_at >= ?
			 ORDER BY duration_ms ASC LIMIT 1 OFFSET ?`, sinceMs, offset)
		_ = p95Row.Scan(&s.P95LatencyMs)
	}

	// tool counts
	rows, err := a.DB.Query(
		`SELECT tool_name, COUNT(*) FROM events
		 WHERE kind='tool_use' AND created_at >= ? AND tool_name != ''
		 GROUP BY tool_name ORDER BY COUNT(*) DESC`, sinceMs)
	if err != nil {
		return s, fmt.Errorf("tool counts: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var name string
		var count int
		if err := rows.Scan(&name, &count); err != nil {
			return s, err
		}
		s.ToolCounts[name] = count
	}
	if err := rows.Err(); err != nil {
		return s, err
	}

	// course counts (by session_create)
	crows, err := a.DB.Query(
		`SELECT course_id, COUNT(*) FROM events
		 WHERE kind='session_create' AND created_at >= ? AND course_id != ''
		 GROUP BY course_id ORDER BY COUNT(*) DESC`, sinceMs)
	if err != nil {
		return s, fmt.Errorf("course counts: %w", err)
	}
	defer func() { _ = crows.Close() }()
	for crows.Next() {
		var cid string
		var count int
		if err := crows.Scan(&cid, &count); err != nil {
			return s, err
		}
		s.CourseCounts[cid] = count
	}
	if err := crows.Err(); err != nil {
		return s, err
	}

	// plan toggles
	row = a.DB.QueryRow(
		`SELECT
		   COALESCE(SUM(CASE WHEN ok=1 THEN 1 ELSE 0 END),0),
		   COALESCE(SUM(CASE WHEN ok=0 THEN 1 ELSE 0 END),0)
		 FROM events WHERE kind='plan_toggle' AND created_at >= ?`, sinceMs)
	if err := row.Scan(&s.PlanDone, &s.PlanUndone); err != nil {
		return s, fmt.Errorf("plan toggles: %w", err)
	}

	// pdf opens
	row = a.DB.QueryRow(
		`SELECT COUNT(*) FROM events WHERE kind='pdf_open' AND created_at >= ?`, sinceMs)
	if err := row.Scan(&s.PDFOpens); err != nil {
		return s, fmt.Errorf("pdf opens: %w", err)
	}

	return s, nil
}

// ---------- Meta ----------

func (a *App) setMetaInt(key string, value int64) error {
	_, err := a.DB.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)", key, fmt.Sprintf("%d", value))
	if err != nil {
		return fmt.Errorf("set meta %q: %w", key, err)
	}
	return nil
}

func (a *App) getMetaInt(key string) (int64, error) {
	var v string
	err := a.DB.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("get meta %q: %w", key, err)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		slog.Warn("parse meta int", "key", key, "value", v, "err", err)
		return 0, nil
	}
	return n, nil
}

// ---------- PDFs ----------

func (a *App) PDFOriginalName(id int64) (string, error) {
	var name string
	err := a.DB.QueryRow("SELECT original_name FROM pdfs WHERE id = ?", id).Scan(&name)
	return name, err
}

func (a *App) PDFFilename(id int64) (string, error) {
	var name string
	err := a.DB.QueryRow("SELECT filename FROM pdfs WHERE id = ?", id).Scan(&name)
	return name, err
}

type PDFCreate struct {
	Filename     string
	OriginalName string
	CourseID     string
	Pages        int
}

// InsertPDF creates a pdf row and returns its id. If filename is empty,
// caller is expected to call UpdatePDFFilename once it's been written
// to disk under a name based on the new id.
func (a *App) InsertPDF(p PDFCreate) (int64, error) {
	res, err := a.DB.Exec(
		"INSERT INTO pdfs (filename, original_name, course_id, pages, last_page, uploaded_at) VALUES (?, ?, ?, ?, 1, ?)",
		p.Filename, p.OriginalName, p.CourseID, p.Pages, time.Now().Format(time.RFC3339),
	)
	if err != nil {
		return 0, fmt.Errorf("insert pdf: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("last insert id: %w", err)
	}
	return id, nil
}

func (a *App) UpdatePDFFilename(id int64, filename string) error {
	if _, err := a.DB.Exec("UPDATE pdfs SET filename = ? WHERE id = ?", filename, id); err != nil {
		return fmt.Errorf("update pdf filename: %w", err)
	}
	return nil
}

func (a *App) DeletePDF(id int64) error {
	if _, err := a.DB.Exec("DELETE FROM pdfs WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete pdf: %w", err)
	}
	return nil
}

func (a *App) ListPDFs(courseFilter string) ([]PDFEntry, error) {
	rows, err := a.DB.Query("SELECT id, original_name, course_id, pages, last_page, last_read_at, uploaded_at FROM pdfs ORDER BY uploaded_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query pdfs: %w", err)
	}
	defer rows.Close()
	var results []PDFEntry
	for rows.Next() {
		var e PDFEntry
		if err := rows.Scan(&e.ID, &e.OriginalName, &e.CourseID, &e.Pages, &e.LastPage, &e.LastReadAt, &e.UploadedAt); err != nil {
			return nil, fmt.Errorf("scan pdf: %w", err)
		}
		if courseFilter != "" {
			if e.CourseID == nil || *e.CourseID != courseFilter {
				continue
			}
		}
		if e.CourseID != nil {
			e.CourseName = a.AppCourseName(*e.CourseID)
		}
		results = append(results, e)
	}
	return results, rows.Err()
}

func (a *App) GetPDF(id int64) (PDFEntry, error) {
	var e PDFEntry
	err := a.DB.QueryRow(
		"SELECT id, original_name, course_id, pages, last_page, last_read_at, uploaded_at FROM pdfs WHERE id = ?",
		id,
	).Scan(&e.ID, &e.OriginalName, &e.CourseID, &e.Pages, &e.LastPage, &e.LastReadAt, &e.UploadedAt)
	if err != nil {
		return PDFEntry{}, err
	}
	if e.CourseID != nil {
		e.CourseName = a.AppCourseName(*e.CourseID)
	}
	return e, nil
}

func (a *App) GetPDFProgress(id int64) (lastPage int, lastReadAt *string, err error) {
	err = a.DB.QueryRow("SELECT last_page, last_read_at FROM pdfs WHERE id = ?", id).Scan(&lastPage, &lastReadAt)
	return
}

func (a *App) UpdatePDFProgress(id int64, page int) (string, error) {
	now := time.Now().Format(time.RFC3339)
	if _, err := a.DB.Exec("UPDATE pdfs SET last_page = ?, last_read_at = ? WHERE id = ?", page, now, id); err != nil {
		return "", fmt.Errorf("update pdf progress: %w", err)
	}
	if err := a.setMetaInt("last_opened_pdf", id); err != nil {
		return "", err
	}
	return now, nil
}

func (a *App) GetPDFFilenameAndOriginal(id int64) (filename, originalName string, err error) {
	err = a.DB.QueryRow("SELECT filename, original_name FROM pdfs WHERE id = ?", id).Scan(&filename, &originalName)
	return
}

func (a *App) SetLastOpenedPDF(id int64) error {
	return a.setMetaInt("last_opened_pdf", id)
}

func (a *App) GetLastOpenedPDFID() (int64, error) {
	return a.getMetaInt("last_opened_pdf")
}

// LogConfidence inserts a confidence entry and returns the new row id.
func (a *App) LogConfidence(sessionID int64, knowledgeComponentID string, value float64, source, rawText string) (int64, error) {
	if value < 0.0 || value > 1.0 {
		return 0, fmt.Errorf("value must be in [0.0, 1.0], got %v", value)
	}
	switch source {
	case "tool_call", "inferred", "manual", "verifier":
	default:
		return 0, fmt.Errorf("invalid source %q: must be tool_call, inferred, manual, or verifier", source)
	}
	now := time.Now().UnixMilli()
	res, err := a.DB.Exec(
		"INSERT INTO confidence_log (session_id, knowledge_component_id, value, source, created_at, raw_text) VALUES (?, ?, ?, ?, ?, ?)",
		sessionID, knowledgeComponentID, value, source, now, rawText,
	)
	if err != nil {
		return 0, fmt.Errorf("insert confidence_log: %w", err)
	}
	return res.LastInsertId()
}

// GetConfidenceTrajectory returns confidence entries for a knowledge_component_id, most recent first.
func (a *App) GetConfidenceTrajectory(knowledgeComponentID string, limit int) ([]ConfidencePoint, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := a.DB.Query(
		"SELECT id, session_id, knowledge_component_id, value, source, created_at, COALESCE(raw_text, '') FROM confidence_log WHERE knowledge_component_id = ? ORDER BY created_at DESC LIMIT ?",
		knowledgeComponentID, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query confidence trajectory: %w", err)
	}
	defer rows.Close()
	var pts []ConfidencePoint
	for rows.Next() {
		var cp ConfidencePoint
		if err := rows.Scan(&cp.ID, &cp.SessionID, &cp.KnowledgeComponentID, &cp.Value, &cp.Source, &cp.CreatedAt, &cp.RawText); err != nil {
			return nil, fmt.Errorf("scan confidence_point: %w", err)
		}
		pts = append(pts, cp)
	}
	return pts, rows.Err()
}

// GetRecentConfidence returns confidence entries created since sinceMs, most recent first.
func (a *App) GetRecentConfidence(sinceMs int64, limit int) ([]ConfidencePoint, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := a.DB.Query(
		"SELECT id, session_id, knowledge_component_id, value, source, created_at, COALESCE(raw_text, '') FROM confidence_log WHERE created_at >= ? ORDER BY created_at DESC LIMIT ?",
		sinceMs, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query recent confidence: %w", err)
	}
	defer rows.Close()
	var pts []ConfidencePoint
	for rows.Next() {
		var cp ConfidencePoint
		if err := rows.Scan(&cp.ID, &cp.SessionID, &cp.KnowledgeComponentID, &cp.Value, &cp.Source, &cp.CreatedAt, &cp.RawText); err != nil {
			return nil, fmt.Errorf("scan confidence_point: %w", err)
		}
		pts = append(pts, cp)
	}
	return pts, rows.Err()
}

// CreateKnowledgeComponent inserts a new knowledge component and returns its id.
func (a *App) CreateKnowledgeComponent(title, body, sourceTaskID string, sourceSessionID int64) (string, error) {
	if title == "" {
		return "", fmt.Errorf("title must not be empty")
	}
	if body == "" {
		return "", fmt.Errorf("body must not be empty")
	}
	id := newTaskID()
	now := time.Now().UnixMilli()
	var taskID interface{}
	if sourceTaskID != "" {
		taskID = sourceTaskID
	}
	var sessionID interface{}
	if sourceSessionID != 0 {
		sessionID = sourceSessionID
	}
	_, err := a.DB.Exec(
		"INSERT INTO knowledge_components (id, title, body, source_task_id, source_session_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, title, body, taskID, sessionID, now, now,
	)
	if err != nil {
		return "", fmt.Errorf("insert knowledge_component: %w", err)
	}
	return id, nil
}

// GetKnowledgeComponent returns a knowledge component by id, or (nil, nil) if not found.
func (a *App) GetKnowledgeComponent(id string) (*KnowledgeComponent, error) {
	var kc KnowledgeComponent
	err := a.DB.QueryRow(
		"SELECT id, title, body, COALESCE(source_task_id,''), COALESCE(source_session_id,0), created_at, updated_at FROM knowledge_components WHERE id = ?",
		id,
	).Scan(&kc.ID, &kc.Title, &kc.Body, &kc.SourceTaskID, &kc.SourceSessionID, &kc.CreatedAt, &kc.UpdatedAt)
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query knowledge_component: %w", err)
	}
	return &kc, nil
}

// ListKnowledgeComponents returns knowledge components ordered by created_at descending.
func (a *App) ListKnowledgeComponents(limit int) ([]KnowledgeComponent, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := a.DB.Query(
		"SELECT id, title, body, COALESCE(source_task_id,''), COALESCE(source_session_id,0), created_at, updated_at FROM knowledge_components ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query knowledge_components: %w", err)
	}
	defer rows.Close()
	var components []KnowledgeComponent
	for rows.Next() {
		var kc KnowledgeComponent
		if err := rows.Scan(&kc.ID, &kc.Title, &kc.Body, &kc.SourceTaskID, &kc.SourceSessionID, &kc.CreatedAt, &kc.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan knowledge_component: %w", err)
		}
		components = append(components, kc)
	}
	return components, rows.Err()
}
