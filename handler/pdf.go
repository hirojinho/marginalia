package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"study-app/agent"
)

func (h *Handler) handlePDFUpload(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodPost) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, MaxPDFBytes)

	if err := r.ParseMultipartForm(MaxPDFBytes); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("upload too large or malformed (max %d bytes)", MaxPDFBytes))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file required")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		writeServerError(w, "read uploaded pdf", err)
		return
	}

	courseID := r.FormValue("course_id")
	pages := agent.ExtractPDFPageCount(data)
	if pages == 0 {
		pages = 1
	}

	id, err := h.App.InsertPDF(agent.PDFCreate{
		Filename:     "",
		OriginalName: header.Filename,
		CourseID:     courseID,
		Pages:        pages,
	})
	if err != nil {
		writeServerError(w, "insert pdf", err)
		return
	}

	filename := fmt.Sprintf("%d.pdf", id)
	if err := h.App.UpdatePDFFilename(id, filename); err != nil {
		writeServerError(w, "update pdf filename", err)
		return
	}

	filePath := h.App.VaultPath("data", "pdf-files", filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		// Clean up the orphaned row before returning. If the cleanup
		// itself fails, log it but report the original error to the
		// client.
		if delErr := h.App.DeletePDF(id); delErr != nil {
			fmt.Println("warning: failed to clean up orphan pdf row", id, delErr)
		}
		writeServerError(w, "save pdf", err)
		return
	}

	if err := h.App.SetLastOpenedPDF(id); err != nil {
		// Non-fatal: file is saved, row is good — only the "last
		// opened" hint is missing. Carry on.
		fmt.Println("warning: set last_opened_pdf:", err)
	}

	go h.App.ExtractAndCachePDFText(id, filePath)

	resp, err := h.App.GetPDF(id)
	if err != nil {
		writeServerError(w, "fetch pdf record", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) handlePDFExtracted(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	id, err := parseInt64(pathSuffix(r.URL.Path, "/pdf/extracted/"), "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	cachePath := h.App.VaultPath("data", "pdf-texts", fmt.Sprintf("%d.txt", id))
	data, err := os.ReadFile(cachePath)
	if err != nil {
		http.Error(w, "not extracted yet", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write(data)
}

func (h *Handler) handlePDFList(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	results, err := h.App.ListPDFs(r.URL.Query().Get("course"))
	if err != nil {
		writeServerError(w, "list pdfs", err)
		return
	}
	if results == nil {
		results = []agent.PDFEntry{}
	}
	writeJSON(w, http.StatusOK, results)
}

func (h *Handler) handlePDFFile(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	id, err := parseInt64(pathSuffix(r.URL.Path, "/pdf/file/"), "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	filename, err := h.App.PDFFilename(id)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", "inline")
	http.ServeFile(w, r, h.App.VaultPath("data", "pdf-files", filename))
}

func (h *Handler) handlePDFProgress(w http.ResponseWriter, r *http.Request) {
	id, err := parseInt64(pathSuffix(r.URL.Path, "/pdf/progress/"), "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	switch r.Method {
	case http.MethodGet:
		lastPage, lastReadAt, err := h.App.GetPDFProgress(id)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":           id,
			"last_page":    lastPage,
			"last_read_at": lastReadAt,
		})

	case http.MethodPut:
		var body struct {
			Page int `json:"page"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
		if body.Page <= 0 {
			writeError(w, http.StatusBadRequest, "page must be positive")
			return
		}
		now, err := h.App.UpdatePDFProgress(id, body.Page)
		if err != nil {
			writeServerError(w, "update pdf progress", err)
			return
		}

		// Best-effort background extraction if not yet cached.
		cachePath := h.App.VaultPath("data", "pdf-texts", fmt.Sprintf("%d.txt", id))
		if _, err := os.Stat(cachePath); os.IsNotExist(err) {
			pdfPath := h.App.VaultPath("data", "pdf-files", fmt.Sprintf("%d.pdf", id))
			go h.App.ExtractAndCachePDFText(id, pdfPath)
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"id":           id,
			"last_page":    body.Page,
			"last_read_at": now,
		})

	default:
		methodNotAllowed(w, r, http.MethodGet, http.MethodPut)
	}
}

func (h *Handler) handlePDFLastOpened(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	id, err := h.App.GetLastOpenedPDFID()
	if err != nil || id == 0 {
		writeJSON(w, http.StatusOK, map[string]interface{}{"id": nil})
		return
	}
	pdf, err := h.App.GetPDF(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusOK, map[string]interface{}{"id": nil})
			return
		}
		writeServerError(w, "get last opened pdf", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"id": id, "pdf": pdf})
}

func (h *Handler) handlePDFAnnotations(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not implemented", http.StatusNotImplemented)
}
