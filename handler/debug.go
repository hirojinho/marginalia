package handler

import (
	"net/http"
	"regexp"
	"strconv"

	"study-app/agent"
)

type versionResponse struct {
	Commit  string `json:"commit"`
	BuiltAt string `json:"built_at"`
}

var tableAllowedPat = regexp.MustCompile(`^[a-z_]+$`)

func (h *Handler) versionHandler(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, versionResponse{
		Commit:  h.App.Config.BuildCommit,
		BuiltAt: h.App.Config.BuildTimestamp,
	})
}

type schemaResponse struct {
	Table   string   `json:"table"`
	Columns []string `json:"columns"`
}

func (h *Handler) schemaHandler(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	table := r.URL.Query().Get("table")
	if table == "" || !tableAllowedPat.MatchString(table) {
		writeError(w, http.StatusBadRequest, "missing or invalid table parameter")
		return
	}
	rows, err := h.App.DB.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull int
		var dfltValue interface{}
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		cols = append(cols, name)
	}
	if len(cols) == 0 {
		writeError(w, http.StatusNotFound, "unknown table")
		return
	}
	writeJSON(w, http.StatusOK, schemaResponse{Table: table, Columns: cols})
}

type retrieveBandResponse struct {
	Confidence float64 `json:"confidence"`
	Grade      int     `json:"grade"`
}

func (h *Handler) bandHandler(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	cStr := r.URL.Query().Get("confidence")
	c, err := strconv.ParseFloat(cStr, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid confidence parameter")
		return
	}
	writeJSON(w, http.StatusOK, retrieveBandResponse{
		Confidence: c,
		Grade:      agent.ConfidenceToGrade(c),
	})
}
