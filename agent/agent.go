package agent

import (
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	VaultRoot string
	DB        *sql.DB
	Mu        sync.Mutex
)

func VaultPath(parts ...string) string {
	return filepath.Join(append([]string{VaultRoot}, parts...)...)
}

func LoadSystemPrompt() string {
	var parts []string
	var loaded []string
	var missing []string

	readFile := func(path string) string {
		data, err := os.ReadFile(path)
		if err != nil {
			missing = append(missing, path)
			return ""
		}
		loaded = append(loaded, path)
		return string(data)
	}

	if s := readFile("/workspace/study-app/CLAUDE.local.md"); s != "" {
		parts = append(parts, s)
	}
	if s := readFile("/workspace/study-app/memory/study-context.md"); s != "" {
		parts = append(parts, s)
	}

	if len(loaded) > 0 {
		log.Printf("✓ System prompt loaded: %v", loaded)
	}
	if len(missing) > 0 {
		log.Printf("⚠ System prompt files missing: %v", missing)
	}

	prompt := joinNonEmpty("\n\n---\n\n", parts...)
	if prompt == "" {
		prompt = "You are a study assistant for an ITA master's student."
	}

	prompt += `
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

	return prompt
}

func joinNonEmpty(sep string, strs ...string) string {
	var nonEmpty []string
	for _, s := range strs {
		if s != "" {
			nonEmpty = append(nonEmpty, s)
		}
	}
	result := ""
	for i, s := range nonEmpty {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}
