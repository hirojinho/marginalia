package agent

import (
	"database/sql"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
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
	// AuthToken gates all non-/login routes when non-empty. When empty,
	// the auth middleware logs a warning and lets all requests through —
	// intended only for local development.
	AuthToken string

	// ClawCLIPath is the absolute path to the claw-cli binary. When set,
	// sandbox creation calls claw-cli memory load to generate AGENTS.md.
	// When empty, a placeholder AGENTS.md is written instead.
	ClawCLIPath string

	// UserID identifies the single tenant. Used when generating AGENTS.md.
	// Defaults to "eduardo" when empty in the sandbox write path.
	UserID string

	// PiPath is the absolute path to the pi binary. When empty, /chat-v2
	// returns 503 Service Unavailable.
	PiPath string

	// SkillsDir is the directory passed to pi --skills-dir. When empty,
	// Pi is launched without a skills directory.
	SkillsDir string

	// AgentModel is the model ID passed to pi --model. Falls back to
	// Config.Model when empty.
	AgentModel string

	// AgentRuntime selects the active agent backend. "pi" routes chat
	// through /chat-v2 (Pi subprocess); "legacy" uses the direct LLM
	// call at /chat. Defaults to "legacy" when empty or unrecognised.
	AgentRuntime string
}

// AgentRuntime is the typed selector for the active agent backend.
type AgentRuntime string

const (
	// AgentRuntimePi routes chat through /chat-v2 (Pi subprocess).
	AgentRuntimePi AgentRuntime = "pi"
	// AgentRuntimeLegacy routes chat through the direct LLM call at /chat.
	AgentRuntimeLegacy AgentRuntime = "legacy"
)

// ActiveRuntime returns the normalised AgentRuntime for this config.
// Returns AgentRuntimeLegacy for empty or unrecognised values.
func (c Config) ActiveRuntime() AgentRuntime {
	if AgentRuntime(c.AgentRuntime) == AgentRuntimePi {
		return AgentRuntimePi
	}
	return AgentRuntimeLegacy
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

	// Sandbox manages per-session ephemeral Pi working directories.
	Sandbox *SandboxManager

	// piActive tracks sessions with an in-flight Pi turn. sync.Map key is
	// int64 session ID; value is time.Time (lock acquisition time).
	piActive sync.Map
}

// NewApp constructs an App with all subsystems initialised. Caller is
// responsible for invoking Close on shutdown.
func NewApp(cfg Config, db *sql.DB) *App {
	return &App{
		DB:      db,
		Config:  cfg,
		Sandbox: NewSandboxManager(cfg.VaultRoot),
	}
}

// Close releases resources held by App, including the database connection.
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

// piLockTTL is the maximum age of a Pi lock before it is considered stale.
// Set slightly above the per-turn context timeout so normal turns always
// release via defer; only crashed or cancelled turns trigger expiry.
const piLockTTL = 10*time.Minute + 30*time.Second

// AcquirePiLock marks sessionID as having an active Pi turn. Returns
// (true, 0) on success. Returns (false, age) when an existing lock is
// still within TTL — age lets callers log diagnostics.
func (a *App) AcquirePiLock(sessionID int64) (bool, time.Duration) {
	now := time.Now()
	for {
		actual, loaded := a.piActive.LoadOrStore(sessionID, now)
		if !loaded {
			return true, 0
		}
		age := now.Sub(actual.(time.Time))
		if age <= piLockTTL {
			return false, age
		}
		// Stale lock: attempt a CAS-style replace.
		if a.piActive.CompareAndSwap(sessionID, actual, now) {
			return true, 0
		}
		// Another goroutine beat us; re-evaluate.
	}
}

// ReleasePiLock clears the active Pi turn marker for sessionID.
func (a *App) ReleasePiLock(sessionID int64) {
	a.piActive.Delete(sessionID)
}
