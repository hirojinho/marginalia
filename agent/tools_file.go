package agent

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (a *App) toolReadFile(args json.RawMessage) string {
	var p struct{ Path string }
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	data, err := os.ReadFile(p.Path)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(data)
}

func (a *App) toolSearchFiles(args json.RawMessage) string {
	var p struct {
		Pattern string `json:"pattern"`
		Include string `json:"include"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	include := p.Include
	if include == "" {
		include = "*.md"
	}
	cmd := exec.Command("rg", "-l", "-i", p.Pattern, "--glob", include, a.Config.VaultRoot)
	out, err := cmd.Output()
	if err != nil {
		if len(out) == 0 {
			return "No matches found."
		}
		return "error: " + err.Error()
	}
	return string(out)
}

func (a *App) toolListFiles(args json.RawMessage) string {
	var p struct{ Path string }
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	if p.Path == "" {
		p.Path = a.Config.VaultRoot
	}
	entries, err := os.ReadDir(p.Path)
	if err != nil {
		return "error: " + err.Error()
	}
	var names []string
	for _, e := range entries {
		n := e.Name()
		if e.IsDir() {
			n += "/"
		}
		names = append(names, n)
	}
	return strings.Join(names, "\n")
}

// ToolSaveNote writes a note to a path relative to the vault root.
// Args JSON: {"path": "<relative-path>", "content": "<note-body>"}.
func (a *App) ToolSaveNote(args json.RawMessage) string {
	var p struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}
	full := filepath.Join(a.Config.VaultRoot, p.Path)
	dir := filepath.Dir(full)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "error: " + err.Error()
	}
	if err := os.WriteFile(full, []byte(p.Content), 0644); err != nil {
		return "error: " + err.Error()
	}
	return "saved to " + full
}
