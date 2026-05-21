package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVersionHandler(t *testing.T) {
	h := newTestHandler(t)
	h.App.Config.BuildCommit = strings.Repeat("a", 40)
	h.App.Config.BuildTimestamp = "2026-05-21T12:00:00Z"

	req := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	rr := httptest.NewRecorder()
	h.versionHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}

	var got versionResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Commit) != 40 {
		t.Errorf("commit length = %d, want 40", len(got.Commit))
	}
	if !strings.Contains(got.BuiltAt, "T") {
		t.Errorf("built_at = %q, want ISO 8601 format", got.BuiltAt)
	}
}

func TestVersionHandler_Defaults(t *testing.T) {
	h := newTestHandler(t)
	// BuildCommit and BuildTimestamp default to empty string (zero value)

	req := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	rr := httptest.NewRecorder()
	h.versionHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}

	var got versionResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Commit != "" {
		t.Errorf("commit = %q, want empty when not injected", got.Commit)
	}
}

func TestVersionHandler_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/debug/version", nil)
	rr := httptest.NewRecorder()
	h.versionHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestVersionHandler_AuthRequired(t *testing.T) {
	h := newAuthHandler(t, "secret")
	h.App.Config.BuildCommit = strings.Repeat("a", 40)
	h.App.Config.BuildTimestamp = "2026-05-21T12:00:00Z"

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/version", h.versionHandler)
	srv := h.AuthMiddleware(mux)

	// Missing token → 401
	req := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}

	// Valid bearer → 200
	req = httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rr.Code)
	}

	// Wrong bearer → 401
	req = httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rr.Code)
	}
}
