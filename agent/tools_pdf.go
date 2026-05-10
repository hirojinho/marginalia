package agent

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ledongthuc/pdf"
)

// ToolPDFExtract extracts text from a stored PDF by its record ID.
// args JSON: {"pdf_id": <int>, "pages": "<range, e.g. 1-5 or 1,3>" (optional)}.
func (a *App) ToolPDFExtract(args json.RawMessage) string {
	var p struct {
		PdfID int64  `json:"pdf_id"`
		Pages string `json:"pages"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "error: " + err.Error()
	}

	filename, originalName, err := a.GetPDFFilenameAndOriginal(p.PdfID)
	if err != nil {
		return fmt.Sprintf("error: PDF not found (id: %d)", p.PdfID)
	}

	pdfPath := a.VaultPath("data", "pdf-files", filename)
	cacheDir := a.VaultPath("data", "pdf-texts")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "error creating cache dir: " + err.Error()
	}
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d.txt", p.PdfID))

	if cached, err := os.ReadFile(cachePath); err == nil {
		allPages := strings.Split(string(cached), "\n---PAGE BREAK---\n")
		if p.Pages != "" {
			selected := parsePageSelection(p.Pages, len(allPages))
			result := pickPages(allPages, selected)
			if len(result) == 0 {
				return "error: no pages found in range"
			}
			return fmt.Sprintf("PDF: %s (%d pages, extracted %d)\n\n%s",
				originalName, len(allPages), len(result), strings.Join(result, "\n\n"))
		}
		return fmt.Sprintf("PDF: %s (%d pages)\n\n%s", originalName, len(allPages), string(cached))
	}

	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		return "error: could not open PDF: " + err.Error()
	}
	defer f.Close()

	totalPages := r.NumPage()
	pageTexts := make([]string, 0, totalPages)
	for i := 1; i <= totalPages; i++ {
		plainText, err := r.Page(i).GetPlainText(nil)
		if err != nil {
			pageTexts = append(pageTexts, fmt.Sprintf("[error extracting page %d]", i))
		} else {
			pageTexts = append(pageTexts, plainText)
		}
	}

	cached := strings.Join(pageTexts, "\n---PAGE BREAK---\n")
	if err := os.WriteFile(cachePath, []byte(cached), 0644); err != nil {
		slog.Warn("cache pdf text", "pdf_id", p.PdfID, "err", err)
	}

	if p.Pages != "" {
		selected := parsePageSelection(p.Pages, totalPages)
		result := pickPages(pageTexts, selected)
		if len(result) == 0 {
			return "error: no pages found in range"
		}
		return fmt.Sprintf("PDF: %s (%d pages, extracted %d)\n\n%s",
			originalName, totalPages, len(result), strings.Join(result, "\n\n"))
	}

	return fmt.Sprintf("PDF: %s (%d pages)\n\n%s", originalName, totalPages, cached)
}

func pickPages(pages []string, indices []int) []string {
	var result []string
	for _, idx := range indices {
		if idx >= 0 && idx < len(pages) {
			result = append(result, fmt.Sprintf("### Page %d\n%s", idx+1, pages[idx]))
		}
	}
	return result
}

// ExtractAndCachePDFText writes a flat per-page text cache for the given
// PDF if one does not already exist. Safe to call from a goroutine.
func (a *App) ExtractAndCachePDFText(id int64, pdfPath string) {
	cacheDir := a.VaultPath("data", "pdf-texts")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		slog.Error("create pdf cache dir", "err", err)
		return
	}
	cachePath := filepath.Join(cacheDir, fmt.Sprintf("%d.txt", id))
	if _, err := os.Stat(cachePath); err == nil {
		return
	}

	f, r, err := pdf.Open(pdfPath)
	if err != nil {
		slog.Error("pdf auto-extract", "pdf_id", id, "err", err)
		return
	}
	defer f.Close()

	pageTexts := make([]string, 0, r.NumPage())
	for i := 1; i <= r.NumPage(); i++ {
		text, err := r.Page(i).GetPlainText(nil)
		if err != nil {
			pageTexts = append(pageTexts, fmt.Sprintf("[error extracting page %d]", i))
		} else {
			pageTexts = append(pageTexts, text)
		}
	}

	cached := strings.Join(pageTexts, "\n---PAGE BREAK---\n")
	if err := os.WriteFile(cachePath, []byte(cached), 0644); err != nil {
		slog.Warn("pdf cache write", "pdf_id", id, "err", err)
		return
	}
	slog.Info("pdf auto-extracted", "pdf_id", id, "pages", r.NumPage())
}

// parsePageSelection turns a string like "1-5,7,10-12" into 0-based
// page indices, deduplicated and clamped to total.
func parsePageSelection(pages string, total int) []int {
	var result []int
	seen := make(map[int]bool)
	parts := strings.Split(pages, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			start, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil {
				continue
			}
			for i := start; i <= end && i <= total; i++ {
				idx := i - 1
				if idx >= 0 && idx < total && !seen[idx] {
					result = append(result, idx)
					seen[idx] = true
				}
			}
		} else {
			n, err := strconv.Atoi(part)
			if err != nil {
				continue
			}
			idx := n - 1
			if idx >= 0 && idx < total && !seen[idx] {
				result = append(result, idx)
				seen[idx] = true
			}
		}
	}
	return result
}
