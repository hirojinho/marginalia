package agent

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

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
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			slog.Warn("schema migration", "stmt", m, "err", err)
		}
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
	rows, err := a.DB.Query("SELECT id, course_id, topic, created_at, updated_at, last_pdf_id, last_page FROM sessions ORDER BY updated_at DESC")
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var s Session
		if err := rows.Scan(&s.ID, &s.CourseID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage); err != nil {
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
		"SELECT id, course_id, topic, created_at, updated_at, last_pdf_id, last_page FROM sessions WHERE id = ?",
		id,
	).Scan(&s.ID, &s.CourseID, &s.Topic, &s.CreatedAt, &s.UpdatedAt, &s.LastPdfID, &s.LastPage)
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
			e.CourseName = CourseName(*e.CourseID)
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
		e.CourseName = CourseName(*e.CourseID)
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
