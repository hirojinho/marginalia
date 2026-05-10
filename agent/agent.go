// Package agent owns the study-agent's persistence, LLM, vector store, RAG,
// tools, prompts, and AGENTS.md memory subsystem. The App struct centralizes
// shared state (DB, config, mutex, active session); methods on App are the
// primary persistence + orchestration surface.
package agent

import (
	"log/slog"
	"os"
	"strings"
)

const fallbackSystemPrompt = "You are a study assistant for an ITA master's student."

const toolsAndRulesPrompt = `
## Available Tools

You have access to these tools — use them proactively when appropriate:
- **read_file** — Read any file in the workspace. Use the EXACT path given in your config. Do NOT guess paths.
- **search_files** — Search file contents with regex
- **list_files** — List directory contents
- **save_note** — Save notes to the vault
- **update_plan** — Update study plans: toggle tasks, mark done/undone, add new tasks. Use this to adjust plans based on session progress.
- **pdf_extract** — Extract text from uploaded PDFs (pass pdf_id). Use this when a user asks about a PDF they're viewing.
- **web_fetch** — Fetch and parse a web page as markdown. Use this when a user asks about something not in your local knowledge.
- **study_skill** — Invoke a study skill (orientation, study_notes, self_test, review, grill_me). Use this when a user wants structured study guidance. Use grill_me when the user wants to be questioned about their plans or decisions.
- **rag_search** — Search the knowledge corpus using semantic similarity. Use when you need to find relevant context for a topic or concept.

## Critical Rules

1. **NEVER guess file contents.** Always use read_file with the exact path from your config.
2. **NEVER explore the filesystem** when looking for a file whose path is already given to you.
3. **NEVER say "the file doesn't exist"** without first calling read_file with the exact path.
4. When a user wants to start studying a topic, use the "orientation" skill.
5. When a user finishes reading and wants notes, use "study_notes".
6. When a user wants to test themselves, use "self_test".
7. When a user wants to review, use "review".`

// LoadSystemPrompt builds the base system prompt by concatenating
// CLAUDE.local.md and memory/study-context.md from the workspace
// (resolved against VaultRoot), then appending the canonical tool and
// rule guidance.
func (a *App) LoadSystemPrompt() string {
	candidates := []string{
		a.VaultPath("CLAUDE.local.md"),
		a.VaultPath("memory", "study-context.md"),
	}

	var loaded, missing []string
	var parts []string
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			missing = append(missing, p)
			continue
		}
		parts = append(parts, string(data))
		loaded = append(loaded, p)
	}

	if len(loaded) > 0 {
		slog.Info("system prompt loaded", "files", loaded)
	}
	if len(missing) > 0 {
		slog.Warn("system prompt files missing", "files", missing)
	}

	body := strings.Join(parts, "\n\n---\n\n")
	if body == "" {
		body = fallbackSystemPrompt
	}
	return body + toolsAndRulesPrompt
}

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
