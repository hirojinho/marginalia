package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionHandler(t *testing.T) {
	h := newTestHandler(t)
	h.App.Config.BuildCommit = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	h.App.Config.BuildTimestamp = "2026-05-21T03:00:00Z"

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
		t.Fatalf("commit length = %d, want 40", len(got.Commit))
	}
	if got.BuiltAt != "2026-05-21T03:00:00Z" {
		t.Fatalf("built_at = %q, want 2026-05-21T03:00:00Z", got.BuiltAt)
	}
}

func TestVersionHandlerUnknownDefaults(t *testing.T) {
	h := newTestHandler(t)
	// Config defaults BuildCommit and BuildTimestamp to "".
	req := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	rr := httptest.NewRecorder()
	h.versionHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d", rr.Code)
	}
	var got versionResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Commit != "" {
		t.Fatalf("commit = %q, want empty (default)", got.Commit)
	}
}

func TestVersionHandlerMethodNotAllowed(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/debug/version", nil)
	rr := httptest.NewRecorder()
	h.versionHandler(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestVersionHandlerUnauthorized(t *testing.T) {
	h := newAuthHandler(t, "secret")
	mux := http.NewServeMux()
	mux.HandleFunc("/debug/version", h.versionHandler)
	srv := h.AuthMiddleware(mux)

	// Request without token — expect 401.
	req := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	// Request with correct token — expect 200.
	h.App.Config.BuildCommit = "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	h.App.Config.BuildTimestamp = "2026-05-21T03:00:00Z"
	req2 := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	req2.Header.Set("Authorization", "Bearer secret")
	rec2 := httptest.NewRecorder()
	srv.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with valid token", rec2.Code)
	}
}
