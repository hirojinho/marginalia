package agent

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func SaveMessage(sessionID int64, role, content string) {
	now := time.Now().Format(time.RFC3339)
	DB.Exec("INSERT INTO messages (session_id, role, content, created_at) VALUES (?, ?, ?, ?)", sessionID, role, content, now)
	DB.Exec("UPDATE sessions SET updated_at = ? WHERE id = ?", now, sessionID)
}

func GetSessionHistory(sessionID int64) []Message {
	rows, err := DB.Query("SELECT role, content FROM messages WHERE session_id = ? ORDER BY id", sessionID)
	if err != nil {
		return nil
	}
	defer rows.Close()
	var msgs []Message
	for rows.Next() {
		var m Message
		rows.Scan(&m.Role, &m.Content)
		msgs = append(msgs, m)
	}
	return msgs
}

func SaveLastAssistantContent(sessionID int64) {
	if LastAssistantContent != "" {
		SaveMessage(sessionID, "assistant", LastAssistantContent)
		LastAssistantContent = ""
	}
}

func InitSessionDB(dbPath string) error {
	var err error
	DB, err = openDB(dbPath)
	if err != nil {
		return err
	}
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
		last_page   INTEGER DEFAULT 1
	);
	CREATE TABLE IF NOT EXISTS messages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		session_id INTEGER NOT NULL,
		role       TEXT NOT NULL,
		content    TEXT NOT NULL,
		created_at TEXT NOT NULL,
		FOREIGN KEY (session_id) REFERENCES sessions(id)
	);
	`
	_, err = DB.Exec(schema)
	if err != nil {
		return err
	}

	migrations := []string{
		"ALTER TABLE sessions ADD COLUMN summary TEXT DEFAULT ''",
		"ALTER TABLE sessions ADD COLUMN summary_at INTEGER DEFAULT 0",
	}
	for _, m := range migrations {
		if _, err := DB.Exec(m); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			log.Printf("Migration warning: %v", err)
		}
	}

	return nil
}

func openDB(path string) (*sql.DB, error) {
	return sql.Open("sqlite", path)
}

func GetActiveSessionID() int64 {
	var value string
	err := DB.QueryRow("SELECT value FROM meta WHERE key = 'last_session'").Scan(&value)
	if err != nil {
		return 0
	}
	var id int64
	fmt.Sscanf(value, "%d", &id)
	return id
}

func SetActiveSessionID(id int64) {
	var current int64
	DB.QueryRow("SELECT COUNT(*) FROM sessions WHERE id = ?", id).Scan(&current)
	if current == 0 {
		return
	}
	DB.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES ('last_session', ?)", fmt.Sprintf("%d", id))
	DB.Exec("UPDATE sessions SET updated_at = ? WHERE id = ?", time.Now().Format(time.RFC3339), id)
}

func GetMessageCount(sessionID int64) int {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM messages WHERE session_id = ?", sessionID).Scan(&count)
	return count
}

func GetSessionHistoryWithSummary(sessionID int64) []Message {
	var summary string
	var summaryAt int
	DB.QueryRow("SELECT summary, summary_at FROM sessions WHERE id = ?", sessionID).Scan(&summary, &summaryAt)

	allMessages := GetSessionHistory(sessionID)

	if summary != "" && len(allMessages) > summaryAt+10 {
		summaryMsg := Message{
			Role:    "system",
			Content: "Previous conversation summary:\n" + summary,
		}
		recentMessages := allMessages
		if len(allMessages) > 10 {
			recentMessages = allMessages[len(allMessages)-10:]
		}
		result := []Message{summaryMsg}
		result = append(result, recentMessages...)
		return result
	}

	return allMessages
}