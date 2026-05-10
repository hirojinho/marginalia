package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChunkFile_Missing(t *testing.T) {
	if _, err := ChunkFile("/no/such/file/zzz.md"); err == nil {
		t.Fatal("expected error")
	}
}

func TestChunkFile_EmptyReturnsNil(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "e.md")
	_ = os.WriteFile(p, []byte("   \n  "), 0644)
	chunks, err := ChunkFile(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if chunks != nil {
		t.Fatalf("expected nil, got %v", chunks)
	}
}

func TestChunkFile_NoH2ProducesSingleChunk(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "n.md")
	_ = os.WriteFile(p, []byte("# Title\n\nbody text"), 0644)
	chunks, err := ChunkFile(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].ParentHeading != "Title" {
		t.Fatalf("got parent %q", chunks[0].ParentHeading)
	}
}

func TestChunkFile_SplitsOnH2(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "m.md")
	_ = os.WriteFile(p, []byte("# Doc\n\nintro line\n\n## First\n\nalpha body\n\n## Second\n\nbravo body\n"), 0644)
	chunks, err := ChunkFile(p)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks (intro + 2 sections), got %d: %+v", len(chunks), chunks)
	}
	if chunks[1].Heading != "First" || chunks[2].Heading != "Second" {
		t.Fatalf("headings off: %+v", chunks)
	}
}

func TestAppClose_NilDB(t *testing.T) {
	a := &App{}
	if err := a.Close(); err != nil {
		t.Fatalf("nil DB close: %v", err)
	}
}

func TestAppClose_RealDB(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	// re-close should error or not, but just call to cover.
	_ = a.Close()
}
