package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSystemPrompt_FallbackWhenNoFiles(t *testing.T) {
	a := newMemoryApp(t)
	out := a.LoadSystemPrompt()
	if !strings.Contains(out, fallbackSystemPrompt) {
		t.Fatalf("expected fallback content, got %q", out)
	}
	if !strings.Contains(out, "Available Tools") {
		t.Fatalf("expected canonical tools section, got %q", out)
	}
}

func TestLoadSystemPrompt_ConcatenatesFiles(t *testing.T) {
	a := newMemoryApp(t)
	if err := os.WriteFile(filepath.Join(a.Config.VaultRoot, "CLAUDE.local.md"), []byte("local"), 0644); err != nil {
		t.Fatalf("w1: %v", err)
	}
	memDir := filepath.Join(a.Config.VaultRoot, "memory")
	_ = os.MkdirAll(memDir, 0755)
	if err := os.WriteFile(filepath.Join(memDir, "study-context.md"), []byte("ctx"), 0644); err != nil {
		t.Fatalf("w2: %v", err)
	}
	out := a.LoadSystemPrompt()
	if !strings.Contains(out, "local") || !strings.Contains(out, "ctx") {
		t.Fatalf("missing parts: %q", out)
	}
	if strings.Contains(out, fallbackSystemPrompt) {
		t.Fatalf("should not include fallback when files present, got %q", out)
	}
}

func TestReadFileWithLog_Missing(t *testing.T) {
	if got := readFileWithLog("/no/such/file/zzz"); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestReadFileWithLog_Present(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "x.md")
	_ = os.WriteFile(p, []byte("hi"), 0644)
	if got := readFileWithLog(p); got != "hi" {
		t.Fatalf("got %q", got)
	}
}
