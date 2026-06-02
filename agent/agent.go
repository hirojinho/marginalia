// Package agent owns the study-agent's persistence, LLM, vector store, RAG,
// tools, prompts, and AGENTS.md memory subsystem. The App struct centralizes
// shared state (DB, config, mutex, active session); methods on App are the
// primary persistence + orchestration surface.
package agent

import (
	"log/slog"
	"os"
)

// readFileWithLog reads a file, logging a warning if it fails. Returns
// the contents (empty string on error). Used for soft loads where
// missing files are tolerated.
func readFileWithLog(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Warn("file not found", "path", path)
		return ""
	}
	return string(data)
}
