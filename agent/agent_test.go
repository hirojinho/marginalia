package agent

import (
	"os"
	"path/filepath"
	"testing"
)

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
