package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolPDFExtract_BadJSON(t *testing.T) {
	a := newMemoryApp(t)
	if out := a.ToolPDFExtract(json.RawMessage(`bad`)); !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}

func TestToolPDFExtract_NotFound(t *testing.T) {
	a := newMemoryApp(t)
	out := a.ToolPDFExtract(json.RawMessage(`{"pdf_id":4242}`))
	if !strings.Contains(out, "PDF not found") {
		t.Fatalf("got %q", out)
	}
}

func TestToolPDFExtract_CacheHitAllPages(t *testing.T) {
	a := newMemoryApp(t)
	// Insert a PDF row.
	res, err := a.DB.Exec(`INSERT INTO pdfs (filename, original_name, pages, uploaded_at) VALUES (?, ?, ?, datetime('now'))`,
		"file-1.pdf", "Original.pdf", 3)
	if err != nil {
		t.Fatalf("insert pdf: %v", err)
	}
	id, _ := res.LastInsertId()

	// Pre-populate cache so we hit the cached branch and avoid pdf parsing.
	cacheDir := a.VaultPath("data", "pdf-texts")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	cached := "page1text\n---PAGE BREAK---\npage2text\n---PAGE BREAK---\npage3text"
	if err := os.WriteFile(filepath.Join(cacheDir, "1.txt"), []byte(cached), 0644); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	_ = id // id should be 1 since it's :memory: and first insert

	out := a.ToolPDFExtract(json.RawMessage(`{"pdf_id":1}`))
	if !strings.Contains(out, "Original.pdf") || !strings.Contains(out, "3 pages") {
		t.Fatalf("got %q", out)
	}
	if !strings.Contains(out, "page1text") {
		t.Fatalf("missing content: %q", out)
	}
}

func TestToolPDFExtract_CacheHitPageSelection(t *testing.T) {
	a := newMemoryApp(t)
	_, err := a.DB.Exec(`INSERT INTO pdfs (filename, original_name, pages, uploaded_at) VALUES (?, ?, ?, datetime('now'))`,
		"file-1.pdf", "Original.pdf", 3)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	cacheDir := a.VaultPath("data", "pdf-texts")
	_ = os.MkdirAll(cacheDir, 0755)
	cached := "alpha\n---PAGE BREAK---\nbravo\n---PAGE BREAK---\ncharlie"
	_ = os.WriteFile(filepath.Join(cacheDir, "1.txt"), []byte(cached), 0644)

	out := a.ToolPDFExtract(json.RawMessage(`{"pdf_id":1,"pages":"1,3"}`))
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "charlie") {
		t.Fatalf("missing content: %q", out)
	}
	if strings.Contains(out, "bravo") {
		t.Fatalf("page 2 should not be present: %q", out)
	}
	if !strings.Contains(out, "extracted 2") {
		t.Fatalf("expected count: %q", out)
	}
}

func TestToolPDFExtract_CacheHitOutOfRangePages(t *testing.T) {
	a := newMemoryApp(t)
	_, err := a.DB.Exec(`INSERT INTO pdfs (filename, original_name, pages, uploaded_at) VALUES (?, ?, ?, datetime('now'))`,
		"file-1.pdf", "Original.pdf", 1)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	cacheDir := a.VaultPath("data", "pdf-texts")
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "1.txt"), []byte("only-page"), 0644)

	out := a.ToolPDFExtract(json.RawMessage(`{"pdf_id":1,"pages":"5-10"}`))
	if !strings.Contains(out, "no pages found") {
		t.Fatalf("got %q", out)
	}
}
