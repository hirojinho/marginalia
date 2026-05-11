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

	// Always (re)write AGENTS.md so stale placeholders get refreshed.
	// Build the directory structure only on first creation.
	if _, err := os.Stat(sandboxDir); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(sandboxDir, "notes"), 0755); err != nil {
			return "", fmt.Errorf("create sandbox notes dir: %w", err)
		}
		if err := os.MkdirAll(sm.outDir, 0755); err != nil {
			return "", fmt.Errorf("create agent-out dir: %w", err)
		}
		outLink := filepath.Join(sandboxDir, "out")
		if err := os.Symlink(sm.outDir, outLink); err != nil && !os.IsExist(err) {
			return "", fmt.Errorf("create out symlink: %w", err)
		}
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
			_ = os.RemoveAll(filepath.Join(sm.baseDir, entry.Name()))
			removed++
			continue
		}
		if info.ModTime().Before(threshold) {
			_ = os.RemoveAll(filepath.Join(sm.baseDir, entry.Name()))
			removed++
		}
	}
	return removed, nil
}

// writeAgentsMD generates AGENTS.md for the sandbox. If clawCLIPath is
// set, it runs claw-cli memory load to produce the content; otherwise it
// writes a minimal placeholder so Pi can still boot.
func (sm *SandboxManager) writeAgentsMD(path, clawCLIPath string, sessionID int64, course, userID string) error {
	var content []byte

	if clawCLIPath != "" && userID != "" {
		args := []string{
			"memory", "load",
			"--session", strconv.FormatInt(sessionID, 10),
			"--user", userID,
		}
		if course != "" {
			args = append(args, "--course", course)
		}
		out, _ := exec.Command(clawCLIPath, args...).Output()
		if len(out) > 0 {
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
