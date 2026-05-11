# Phase 4 — Per-Session Sandbox Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add per-session ephemeral sandboxes for Pi agent workspaces: `agent/sandbox.go` with tests, a stub `/chat-v2` handler that creates/reuses the sandbox, session-deletion wiring, and a sweep goroutine for idle cleanup.

**Architecture:** A `SandboxManager` (in `agent/`) owns `data/agent-sessions/<id>/` dirs. `App` holds a `*SandboxManager`; `NewApp` constructs it. The stub `/chat-v2` POST handler calls `Create`, returning the sandbox path as JSON. `DeleteSession` calls `Sandbox.Delete`. A background goroutine in `main.go` sweeps stale sandboxes every 24 h. No Pi spawning yet — that's Phase 5.

**Tech Stack:** Go 1.24, stdlib only (`os`, `os/exec`, `path/filepath`, `time`). Tests use `testing` + `t.TempDir()`. Lint via golangci-lint with `--new-from-rev HEAD`.

---

## File Map

| Action | Path | Responsibility |
|--------|------|----------------|
| Create | `agent/sandbox.go` | `SandboxManager` type + `NewSandboxManager`, `Create`, `Path`, `Delete`, `Sweep` |
| Create | `agent/sandbox_test.go` | Unit tests for all `SandboxManager` methods |
| Modify | `agent/app.go` | Add `ClawCLIPath`, `UserID` to `Config`; add `Sandbox *SandboxManager` field; wire in `NewApp` |
| Modify | `agent/db.go` | Call `a.Sandbox.Delete(id)` inside `DeleteSession` |
| Modify | `config.go` | Read `CLAW_CLI_PATH` and `USER_ID` env vars |
| Modify | `main.go` | Add `agent-sessions` and `agent-out` to `dataSubdirs`; start sweep goroutine |
| Modify | `handler/handler.go` | Add `/chat-v2` route |
| Create | `handler/chat_v2.go` | Stub `POST /chat-v2` handler |
| Create | `handler/chat_v2_test.go` | HTTP handler tests for the stub |

---

## Task 1: `agent/sandbox.go` — SandboxManager + App wiring

**Files:**
- Create: `agent/sandbox.go`
- Create: `agent/sandbox_test.go`
- Modify: `agent/app.go`

### Step-by-step

- [ ] **Step 1: Write the failing tests**

Create `agent/sandbox_test.go`:

```go
package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSandboxManagerCreateBuildsStructure(t *testing.T) {
	vaultRoot := t.TempDir()
	sm := NewSandboxManager(vaultRoot)

	path, err := sm.Create(42, "", "", "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	for _, rel := range []string{"notes", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(path, rel)); err != nil {
			t.Errorf("expected %s to exist: %v", rel, err)
		}
	}

	// out symlink should resolve to data/agent-out
	outLink := filepath.Join(path, "out")
	target, err := os.Readlink(outLink)
	if err != nil {
		t.Fatalf("readlink out: %v", err)
	}
	expected := filepath.Join(vaultRoot, "data", "agent-out")
	if target != expected {
		t.Errorf("out symlink = %q, want %q", target, expected)
	}
}

func TestSandboxManagerCreateIsIdempotent(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())

	path1, err := sm.Create(7, "", "", "")
	if err != nil {
		t.Fatalf("first Create: %v", err)
	}
	path2, err := sm.Create(7, "", "", "")
	if err != nil {
		t.Fatalf("second Create: %v", err)
	}
	if path1 != path2 {
		t.Errorf("idempotent: got %q and %q", path1, path2)
	}
}

func TestSandboxManagerPathMatchesCreate(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, _ := sm.Create(99, "", "", "")
	if sm.Path(99) != path {
		t.Errorf("Path(99) = %q, Create returned %q", sm.Path(99), path)
	}
}

func TestSandboxManagerDeleteRemovesDir(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	path, _ := sm.Create(5, "", "", "")

	if err := sm.Delete(5); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("sandbox dir still exists after Delete")
	}
}

func TestSandboxManagerDeleteMissingIsNoop(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	if err := sm.Delete(999); err != nil {
		t.Errorf("Delete of non-existent sandbox: %v", err)
	}
}

func TestSandboxManagerSweepRemovesStale(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())

	// Create two sandboxes.
	_, _ = sm.Create(1, "", "", "")
	_, _ = sm.Create(2, "", "", "")

	// Back-date AGENTS.md for sandbox 1 to simulate staleness.
	staleTime := time.Now().Add(-8 * 24 * time.Hour)
	agentsMD := filepath.Join(sm.Path(1), "AGENTS.md")
	if err := os.Chtimes(agentsMD, staleTime, staleTime); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}

	removed, err := sm.Sweep(7)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if removed != 1 {
		t.Errorf("Sweep removed %d, want 1", removed)
	}

	// Sandbox 2 must still exist.
	if _, err := os.Stat(sm.Path(2)); err != nil {
		t.Errorf("sandbox 2 missing after sweep: %v", err)
	}
	// Sandbox 1 must be gone.
	if _, err := os.Stat(sm.Path(1)); !os.IsNotExist(err) {
		t.Errorf("stale sandbox 1 still exists after sweep")
	}
}

func TestSandboxManagerSweepKeepsFresh(t *testing.T) {
	sm := NewSandboxManager(t.TempDir())
	_, _ = sm.Create(3, "", "", "")

	removed, err := sm.Sweep(7)
	if err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if removed != 0 {
		t.Errorf("Sweep removed %d fresh sandboxes, want 0", removed)
	}
}
```

- [ ] **Step 2: Run tests to confirm they fail**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
/opt/homebrew/bin/go test ./agent/ -run TestSandboxManager -v 2>&1 | head -30
```

Expected: compile error (SandboxManager not defined yet).

- [ ] **Step 3: Implement `agent/sandbox.go`**

```go
// Package agent — sandbox.go manages per-session ephemeral working
// directories for the Pi agent runtime. Each sandbox lives under
// data/agent-sessions/<sessionID>/ inside the vault root and contains
// the generated AGENTS.md, a notes/ scratch dir, and an out symlink
// pointing at the shared data/agent-out/ drop zone.
package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// SandboxManager creates, reuses, and cleans up per-session sandboxes.
// Construct with NewSandboxManager; the zero value is invalid.
type SandboxManager struct {
	baseDir string // <vaultRoot>/data/agent-sessions
	outDir  string // <vaultRoot>/data/agent-out
}

// NewSandboxManager returns a manager rooted at vaultRoot.
func NewSandboxManager(vaultRoot string) *SandboxManager {
	return &SandboxManager{
		baseDir: filepath.Join(vaultRoot, "data", "agent-sessions"),
		outDir:  filepath.Join(vaultRoot, "data", "agent-out"),
	}
}

// Path returns the sandbox directory for sessionID (may not exist yet).
func (sm *SandboxManager) Path(sessionID int64) string {
	return filepath.Join(sm.baseDir, strconv.FormatInt(sessionID, 10))
}

// Create ensures the sandbox for sessionID exists and returns its path.
// If the sandbox already exists, AGENTS.md mtime is updated (so the
// sweep treats it as recently used) and the path is returned as-is.
// clawCLIPath may be empty; if so, a placeholder AGENTS.md is written.
func (sm *SandboxManager) Create(sessionID int64, clawCLIPath, course, userID string) (string, error) {
	sandboxDir := sm.Path(sessionID)

	agentsMD := filepath.Join(sandboxDir, "AGENTS.md")
	if _, err := os.Stat(agentsMD); err == nil {
		// Sandbox exists — touch AGENTS.md to reset the idle clock.
		now := time.Now()
		if touchErr := os.Chtimes(agentsMD, now, now); touchErr != nil {
			return "", fmt.Errorf("touch agents.md: %w", touchErr)
		}
		return sandboxDir, nil
	}

	// New sandbox — build the full structure.
	if err := os.MkdirAll(filepath.Join(sandboxDir, "notes"), 0755); err != nil {
		return "", fmt.Errorf("create sandbox notes dir: %w", err)
	}

	// Ensure agent-out exists before symlinking.
	if err := os.MkdirAll(sm.outDir, 0755); err != nil {
		return "", fmt.Errorf("create agent-out dir: %w", err)
	}
	outLink := filepath.Join(sandboxDir, "out")
	if err := os.Symlink(sm.outDir, outLink); err != nil {
		return "", fmt.Errorf("create out symlink: %w", err)
	}

	if err := sm.writeAgentsMD(agentsMD, clawCLIPath, sessionID, course, userID); err != nil {
		return "", err
	}

	return sandboxDir, nil
}

// Delete removes the sandbox directory for sessionID. No error if absent.
func (sm *SandboxManager) Delete(sessionID int64) error {
	if err := os.RemoveAll(sm.Path(sessionID)); err != nil {
		return fmt.Errorf("remove sandbox: %w", err)
	}
	return nil
}

// Sweep removes sandboxes whose AGENTS.md was not modified within the
// last maxIdleDays days. Returns the number of sandboxes removed.
func (sm *SandboxManager) Sweep(maxIdleDays int) (int, error) {
	entries, err := os.ReadDir(sm.baseDir)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("read sandbox base dir: %w", err)
	}

	threshold := time.Now().AddDate(0, 0, -maxIdleDays)
	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		agentsMD := filepath.Join(sm.baseDir, entry.Name(), "AGENTS.md")
		info, err := os.Stat(agentsMD)
		if err != nil {
			// Missing or unreadable AGENTS.md — treat as stale.
			if removeErr := os.RemoveAll(filepath.Join(sm.baseDir, entry.Name())); removeErr == nil {
				removed++
			}
			continue
		}
		if info.ModTime().Before(threshold) {
			if removeErr := os.RemoveAll(filepath.Join(sm.baseDir, entry.Name())); removeErr == nil {
				removed++
			}
		}
	}
	return removed, nil
}

// writeAgentsMD generates AGENTS.md for the sandbox. If clawCLIPath is
// set, it runs claw-cli memory load to produce the content; otherwise it
// writes a minimal placeholder so Pi can still boot.
func (sm *SandboxManager) writeAgentsMD(path, clawCLIPath string, sessionID int64, course, userID string) error {
	var content []byte

	if clawCLIPath != "" && course != "" && userID != "" {
		out, err := exec.Command(
			clawCLIPath, "memory", "load",
			"--session", strconv.FormatInt(sessionID, 10),
			"--course", course,
			"--user", userID,
		).Output()
		if err == nil {
			content = out
		}
		// On error fall through to placeholder — don't fail sandbox creation.
	}

	if len(content) == 0 {
		content = []byte("# Agent context\n\nNo memory loaded for this session.\n")
	}

	if err := os.WriteFile(path, content, 0644); err != nil {
		return fmt.Errorf("write agents.md: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests to confirm they pass**

```bash
/opt/homebrew/bin/go test ./agent/ -run TestSandboxManager -v
```

Expected: all 7 tests PASS.

- [ ] **Step 5: Add `Sandbox *SandboxManager` to `App`, wire in `NewApp`**

In `agent/app.go`:

Add `Sandbox *SandboxManager` field to `App` struct after `activeSessionID`:

```go
type App struct {
	DB     *sql.DB
	Config Config

	chatMu          sync.Mutex
	activeSessionID atomic.Int64

	// Sandbox manages per-session ephemeral Pi working directories.
	Sandbox *SandboxManager
}
```

Update `NewApp` to construct the manager:

```go
func NewApp(cfg Config, db *sql.DB) *App {
	return &App{
		DB:      db,
		Config:  cfg,
		Sandbox: NewSandboxManager(cfg.VaultRoot),
	}
}
```

- [ ] **Step 6: Run full agent tests**

```bash
/opt/homebrew/bin/go test ./agent/ -v 2>&1 | tail -20
```

Expected: PASS (no regressions).

- [ ] **Step 7: Commit**

```bash
cd /Users/eduardohiroji/Documents/ITA/claw-study
git add agent/sandbox.go agent/sandbox_test.go agent/app.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "$(cat <<'EOF'
feat(agent): add SandboxManager for per-session Pi working directories

NewSandboxManager creates/reuses data/agent-sessions/<id>/ dirs with
AGENTS.md (generated via claw-cli or placeholder), notes/ subdir, and
out -> data/agent-out symlink. Sweep removes idle dirs past maxIdleDays.
App.Sandbox wired in NewApp.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2: Config additions + `main.go` + stub `/chat-v2` handler

**Files:**
- Modify: `agent/app.go` (add `ClawCLIPath`, `UserID` to `Config`)
- Modify: `config.go` (read env vars)
- Modify: `main.go` (add subdirs; start sweep goroutine)
- Modify: `handler/handler.go` (register `/chat-v2` route)
- Create: `handler/chat_v2.go`
- Create: `handler/chat_v2_test.go`

### Step-by-step

- [ ] **Step 1: Write failing handler tests**

Create `handler/chat_v2_test.go`:

```go
package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChatV2CreatesSandboxOnFirstCall(t *testing.T) {
	h := newTestHandler(t)

	// Create a session to reference.
	sess, err := h.App.CreateSession("ce297", "test topic")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"session_id": sess.ID, "message": "hi"})
	req := httptest.NewRequest(http.MethodPost, "/chat-v2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleChatV2(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["sandbox"] == "" {
		t.Errorf("response missing sandbox path: %v", resp)
	}
}

func TestChatV2ReusesExistingSandbox(t *testing.T) {
	h := newTestHandler(t)

	sess, _ := h.App.CreateSession("ddia", "reuse test")

	call := func() string {
		body, _ := json.Marshal(map[string]any{"session_id": sess.ID, "message": "ping"})
		req := httptest.NewRequest(http.MethodPost, "/chat-v2", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.handleChatV2(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("status = %d", w.Code)
		}
		var resp map[string]string
		_ = json.NewDecoder(w.Body).Decode(&resp)
		return resp["sandbox"]
	}

	first := call()
	second := call()
	if first != second {
		t.Errorf("sandbox path changed on second call: %q vs %q", first, second)
	}
}

func TestChatV2RejectsMissingSessionID(t *testing.T) {
	h := newTestHandler(t)
	body := []byte(`{"message":"hi"}`)
	req := httptest.NewRequest(http.MethodPost, "/chat-v2", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.handleChatV2(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestChatV2RejectsMethodNotPost(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/chat-v2", nil)
	w := httptest.NewRecorder()
	h.handleChatV2(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}
```

- [ ] **Step 2: Run to confirm they fail**

```bash
/opt/homebrew/bin/go test ./handler/ -run TestChatV2 -v 2>&1 | head -20
```

Expected: compile error (handleChatV2 not defined yet).

- [ ] **Step 3: Add `ClawCLIPath` and `UserID` to `Config`**

In `agent/app.go`, add two new fields to the `Config` struct after `AuthToken`:

```go
// ClawCLIPath is the absolute path to the claw-cli binary. When set,
// sandbox creation calls claw-cli memory load to generate AGENTS.md.
// When empty, a placeholder AGENTS.md is written instead.
ClawCLIPath string

// UserID identifies the single tenant. Used when generating AGENTS.md.
// Defaults to "eduardo" when empty in the sandbox write path.
UserID string
```

- [ ] **Step 4: Wire `ClawCLIPath` and `UserID` in `config.go`**

Add to the `return agent.Config{...}` block in `loadConfig`:

```go
ClawCLIPath: os.Getenv("CLAW_CLI_PATH"),
UserID:      firstNonEmpty(os.Getenv("USER_ID"), "eduardo"),
```

- [ ] **Step 5: Extend `dataSubdirs` and add sweep goroutine in `main.go`**

In `main.go`, change `dataSubdirs` to add two new entries:

```go
var dataSubdirs = []string{
	"pdf-files",
	"plans",
	"pdf-texts",
	"agent-sessions",
	"agent-out",
	filepath.Join("corpus", "study-methods"),
	filepath.Join("corpus", "courses"),
	filepath.Join("corpus", "meta"),
}
```

In `main()`, after `app := agent.NewApp(cfg, db)` and the corpus indexing goroutine, add the sweep goroutine:

```go
go func() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		if n, err := app.Sandbox.Sweep(sandboxIdleDays); err != nil {
			slog.Error("sandbox sweep", "err", err)
		} else if n > 0 {
			slog.Info("sandbox sweep removed stale dirs", "count", n)
		}
	}
}()
```

Add the constant at the top of `main.go` (with the other constants):

```go
sandboxIdleDays = 7
```

- [ ] **Step 6: Implement `handler/chat_v2.go`**

```go
// Package handler — chat_v2.go implements the stub /chat-v2 endpoint.
// Phase 4: creates/reuses the per-session Pi sandbox and returns its path.
// Pi is not spawned yet; that is Phase 5.
package handler

import (
	"encoding/json"
	"net/http"
)

type chatV2Request struct {
	SessionID int64  `json:"session_id"`
	Message   string `json:"message"`
}

// handleChatV2 handles POST /chat-v2. It creates or reuses the per-session
// sandbox for the given session_id and returns the sandbox path.
func (h *Handler) handleChatV2(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}

	var req chatV2Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.SessionID <= 0 {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	sess, err := h.App.GetSession(req.SessionID)
	if err != nil {
		writeServerError(w, "get session", err)
		return
	}

	sandboxPath, err := h.App.Sandbox.Create(
		req.SessionID,
		h.App.Config.ClawCLIPath,
		sess.CourseID,
		h.App.Config.UserID,
	)
	if err != nil {
		writeServerError(w, "create sandbox", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"sandbox": sandboxPath,
		"status":  "stub — Pi not yet wired",
	})
}
```

- [ ] **Step 7: Register the route in `handler/handler.go`**

In the `Register` method, add after the `/chat` line:

```go
mux.HandleFunc("/chat-v2", h.handleChatV2)
```

- [ ] **Step 8: Run all tests**

```bash
/opt/homebrew/bin/go test ./... -v 2>&1 | tail -30
```

Expected: all tests PASS including the four new `TestChatV2*` tests.

- [ ] **Step 9: Run golangci-lint**

```bash
golangci-lint run --new-from-rev HEAD ./...
```

Expected: no new violations.

- [ ] **Step 10: Commit**

```bash
git add agent/app.go config.go main.go handler/handler.go handler/chat_v2.go handler/chat_v2_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "$(cat <<'EOF'
feat: add /chat-v2 stub handler with per-session sandbox wiring

Adds Config.ClawCLIPath + Config.UserID, agent-sessions and agent-out
data subdirs, a 24h sweep goroutine, and the stub POST /chat-v2 that
creates or reuses the Pi sandbox. Returns sandbox path; Pi not spawned
yet (Phase 5).

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3: Session deletion wiring + VPS deploy + smoke

**Files:**
- Modify: `agent/db.go` (call `Sandbox.Delete` inside `DeleteSession`)
- VPS: build + rsync + restart + curl smokes

### Step-by-step

- [ ] **Step 1: Write a failing test for deletion wiring**

Add to `handler/chat_v2_test.go` (append at the bottom):

```go
func TestDeleteSessionRemovesSandbox(t *testing.T) {
	h := newTestHandler(t)

	sess, _ := h.App.CreateSession("ce297", "deletion test")

	// Create the sandbox first.
	sandboxPath := h.App.Sandbox.Path(sess.ID)
	if _, err := h.App.Sandbox.Create(sess.ID, "", "", ""); err != nil {
		t.Fatalf("Create sandbox: %v", err)
	}

	// Confirm it exists.
	if _, err := os.Stat(sandboxPath); err != nil {
		t.Fatalf("sandbox not created: %v", err)
	}

	// Delete the session — must also remove the sandbox.
	if err := h.App.DeleteSession(sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	if _, err := os.Stat(sandboxPath); !os.IsNotExist(err) {
		t.Errorf("sandbox dir still exists after session deletion")
	}
}
```

Add `"os"` to the import list in `handler/chat_v2_test.go`.

- [ ] **Step 2: Run test to confirm it fails**

```bash
/opt/homebrew/bin/go test ./handler/ -run TestDeleteSessionRemovesSandbox -v
```

Expected: FAIL — sandbox dir still exists (Delete not wired yet).

- [ ] **Step 3: Wire `Sandbox.Delete` in `agent/db.go` `DeleteSession`**

In `agent/db.go`, at the end of `DeleteSession` before the final `return nil`:

```go
if a.Sandbox != nil {
	if sandboxErr := a.Sandbox.Delete(id); sandboxErr != nil {
		return fmt.Errorf("delete sandbox: %w", sandboxErr)
	}
}
return nil
```

The full `DeleteSession` body becomes:

```go
func (a *App) DeleteSession(id int64) error {
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
```

- [ ] **Step 4: Run all tests**

```bash
/opt/homebrew/bin/go test ./... -v 2>&1 | tail -20
```

Expected: all tests PASS including `TestDeleteSessionRemovesSandbox`.

- [ ] **Step 5: Run golangci-lint**

```bash
golangci-lint run --new-from-rev HEAD ./...
```

Expected: no new violations.

- [ ] **Step 6: Commit**

```bash
git add agent/db.go handler/chat_v2_test.go
git -c user.email=eduardo.hiroji@brendi.com.br -c user.name=hirojinho \
  commit -m "$(cat <<'EOF'
feat(agent): wire Sandbox.Delete in DeleteSession

Ensures the ephemeral sandbox directory is removed when a session is
deleted, keeping data/agent-sessions/ tidy without a manual sweep.

Co-Authored-By: Claude Sonnet 4.6 <noreply@anthropic.com>
EOF
)"
```

- [ ] **Step 7: Push to main**

```bash
git push origin main
```

- [ ] **Step 8: Build and deploy to VPS**

```bash
GOOS=linux GOARCH=amd64 /opt/homebrew/bin/go build -o bin/study-app-linux .
rsync -av bin/study-app-linux nanoclaw:~/stack/study-app/bin/study-app
ssh nanoclaw "export XDG_RUNTIME_DIR=/run/user/\$(id -u); systemctl --user restart study-app.service && systemctl --user is-active study-app.service"
```

Expected: `active`

- [ ] **Step 9: Create a session and smoke `/chat-v2`**

```bash
# Create a session via the existing API.
SESSION_ID=$(ssh nanoclaw "curl -s -X POST http://localhost:8081/api/sessions \
  -H 'Content-Type: application/json' \
  -d '{\"course_id\":\"ce297\",\"topic\":\"phase4 smoke\"}' | jq .id")
echo "Session ID: $SESSION_ID"

# First call — creates sandbox.
ssh nanoclaw "curl -s -X POST http://localhost:8081/chat-v2 \
  -H 'Content-Type: application/json' \
  -d \"{\\\"session_id\\\":$SESSION_ID,\\\"message\\\":\\\"hi\\\"}\" | jq ."
```

Expected: `{"sandbox": "/home/eduardo/stack/study-app/data/agent-sessions/<N>", "status": "stub — Pi not yet wired"}`

```bash
# Second call — reuses sandbox (same path).
ssh nanoclaw "curl -s -X POST http://localhost:8081/chat-v2 \
  -H 'Content-Type: application/json' \
  -d \"{\\\"session_id\\\":$SESSION_ID,\\\"message\\\":\\\"ping\\\"}\" | jq .sandbox"
```

Expected: same path as first call.

```bash
# Verify sandbox structure on VPS.
ssh nanoclaw "ls -la ~/stack/study-app/data/agent-sessions/$SESSION_ID/"
```

Expected: `AGENTS.md`, `notes/`, `out -> /home/eduardo/stack/study-app/data/agent-out`

```bash
# Delete session — sandbox must disappear.
ssh nanoclaw "curl -s -X DELETE \"http://localhost:8081/api/sessions?id=$SESSION_ID\" | jq ."
ssh nanoclaw "ls ~/stack/study-app/data/agent-sessions/ 2>&1"
```

Expected: delete returns `{"ok":true}`; agent-sessions dir is empty.

- [ ] **Step 10: Update deploy log**

Append a Phase 4 section to `docs/specs/proposals/phase1-deploy-log.md`:

```markdown
## Phase 4 — Sandbox manager (2026-05-10)

Deployed commit range <start>..<end> to VPS.

Smoke results:
- POST /chat-v2 creates sandbox at data/agent-sessions/<id>/ ✅
- Second POST reuses same path ✅
- DELETE /api/sessions?id=<N> removes sandbox ✅
- Sandbox contains AGENTS.md, notes/, out -> agent-out ✅
- /debug/health 200 ✅
```

---

## Self-Review

**Spec coverage check:**

| Spec requirement | Covered by |
|---|---|
| `agent.SandboxManager` with `Create`, `Delete`, `Sweep` | Task 1 `agent/sandbox.go` |
| Create `data/agent-sessions/<id>/` | `SandboxManager.Create` |
| Generate AGENTS.md via `claw-cli memory load` | `writeAgentsMD` (clawCLIPath path) |
| Set up `out` symlink → `agent-out` | `SandboxManager.Create` |
| Create `notes/` subdir | `SandboxManager.Create` |
| Reuse on subsequent turns | Idempotent `Create` — touches AGENTS.md mtime |
| Delete on session deletion | Task 3 `agent/db.go` wiring |
| Sweep job for stale sandboxes | `Sweep(maxIdleDays)` + goroutine in `main.go` |
| Stub `/chat-v2` handler (no Pi yet) | Task 2 `handler/chat_v2.go` |
| Unit tests | `agent/sandbox_test.go`, `handler/chat_v2_test.go` |
| VPS deploy + smoke | Task 3 Steps 8–10 |

**Placeholder scan:** No TBDs or stubs in code blocks. All commands include expected output. ✅

**Type consistency:** `SandboxManager.Create` signature matches all call sites (`sessionID int64, clawCLIPath, course, userID string`). `Config.ClawCLIPath` and `Config.UserID` added in both `agent/app.go` and `config.go`. ✅
