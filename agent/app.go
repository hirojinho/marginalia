package agent

import (
	"database/sql"
	"path/filepath"
	"sync"
	"sync/atomic"
)

// Config holds all runtime configuration. Constructed once at startup
// from environment variables; immutable thereafter.
type Config struct {
	VaultRoot      string
	APIKey         string
	APIURL         string
	Model          string
	EmbeddingModel string
	ListenAddr     string
}

// App owns all shared mutable state for the study app: the database
// connection, configuration, and synchronization primitives. Pass *App
// into any function that needs to reach into the database or vault.
type App struct {
	DB     *sql.DB
	Config Config

	// chatMu serializes operations that read-then-write session state
	// during a chat turn (saving messages, computing summaries).
	chatMu sync.Mutex

	// activeSessionID is the currently selected session. Stored as
	// atomic so cross-handler reads don't need a mutex.
	activeSessionID atomic.Int64
}

// NewApp constructs an App. Caller is responsible for invoking Close
// on shutdown.
func NewApp(cfg Config, db *sql.DB) *App {
	return &App{DB: db, Config: cfg}
}

func (a *App) Close() error {
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}

// VaultPath joins parts under VaultRoot.
func (a *App) VaultPath(parts ...string) string {
	return filepath.Join(append([]string{a.Config.VaultRoot}, parts...)...)
}

// ActiveSessionID returns the currently selected session id, or 0.
func (a *App) ActiveSessionID() int64 {
	return a.activeSessionID.Load()
}

// SetActiveSessionIDInMemory updates the in-memory active session id
// without touching the database. Use SetActiveSession to persist.
func (a *App) SetActiveSessionIDInMemory(id int64) {
	a.activeSessionID.Store(id)
}

// LockChat acquires the chat-turn mutex.
func (a *App) LockChat()   { a.chatMu.Lock() }
func (a *App) UnlockChat() { a.chatMu.Unlock() }
