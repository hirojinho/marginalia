package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolReadFile_Success(t *testing.T) {
	a := newMemoryApp(t)
	path := filepath.Join(a.Config.VaultRoot, "hello.txt")
	if err := os.WriteFile(path, []byte("hi there"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	out := a.toolReadFile(json.RawMessage(`{"Path":"` + path + `"}`))
	if out != "hi there" {
		t.Fatalf("got %q", out)
	}
}

func TestToolReadFile_Missing(t *testing.T) {
	a := newMemoryApp(t)
	out := a.toolReadFile(json.RawMessage(`{"Path":"/no/such/file"}`))
	if !strings.HasPrefix(out, "error:") {
		t.Fatalf("expected error, got %q", out)
	}
}

func TestToolReadFile_BadJSON(t *testing.T) {
	a := newMemoryApp(t)
	out := a.toolReadFile(json.RawMessage(`bad`))
	if !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}

func TestToolListFiles_DefaultsToVaultRoot(t *testing.T) {
	a := newMemoryApp(t)
	_ = os.WriteFile(filepath.Join(a.Config.VaultRoot, "a.md"), []byte("x"), 0644)
	_ = os.Mkdir(filepath.Join(a.Config.VaultRoot, "sub"), 0755)
	out := a.toolListFiles(json.RawMessage(`{}`))
	if !strings.Contains(out, "a.md") {
		t.Fatalf("missing a.md: %q", out)
	}
	if !strings.Contains(out, "sub/") {
		t.Fatalf("expected dir suffix in: %q", out)
	}
}

func TestToolListFiles_ExplicitPath(t *testing.T) {
	a := newMemoryApp(t)
	dir := filepath.Join(a.Config.VaultRoot, "x")
	_ = os.Mkdir(dir, 0755)
	_ = os.WriteFile(filepath.Join(dir, "f1.txt"), []byte("y"), 0644)
	out := a.toolListFiles(json.RawMessage(`{"Path":"` + dir + `"}`))
	if !strings.Contains(out, "f1.txt") {
		t.Fatalf("got %q", out)
	}
}

func TestToolListFiles_Missing(t *testing.T) {
	a := newMemoryApp(t)
	out := a.toolListFiles(json.RawMessage(`{"Path":"/no/such/dir"}`))
	if !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}

func TestToolListFiles_BadJSON(t *testing.T) {
	a := newMemoryApp(t)
	if out := a.toolListFiles(json.RawMessage(`{`)); !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}

func TestToolSaveNote_CreatesNestedDirsAndWrites(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ToolSaveNote(json.RawMessage(`{"path":"fleeting/2026/note.md","content":"# hi"}`))
	if !strings.HasPrefix(out, "saved to ") {
		t.Fatalf("got %q", out)
	}
	full := filepath.Join(a.Config.VaultRoot, "fleeting/2026/note.md")
	data, err := os.ReadFile(full)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "# hi" {
		t.Fatalf("content mismatch: %q", string(data))
	}
}

func TestToolSaveNote_BadJSON(t *testing.T) {
	a := newMemoryApp(t)
	if out := a.ToolSaveNote(json.RawMessage(`bad`)); !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}

func TestToolSearchFiles_BadJSON(t *testing.T) {
	a := newMemoryApp(t)
	if out := a.toolSearchFiles(json.RawMessage(`bad`)); !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}
