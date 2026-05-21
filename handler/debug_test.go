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
	h.BuildCommit = strings.Repeat("a", 40)
	h.BuildTimestamp = "2026-05-21T12:00:00Z"

	req := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	rr := httptest.NewRecorder()
	h.handleDebugVersion(rr, req)

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
	if !strings.HasPrefix(got.BuiltAt, "2026-") {
		t.Fatalf("built_at = %q, want ISO8601 prefix", got.BuiltAt)
	}
}

func TestVersionHandler_Defaults(t *testing.T) {
	h := newTestHandler(t)
	// BuildCommit and BuildTimestamp default to empty string from newTestHandler.

	req := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	rr := httptest.NewRecorder()
	h.handleDebugVersion(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}

	var got versionResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	// When ldflags are not set, the values come through as empty from the
	// handler (they would be "unknown" from main.go's defaults, but the test
	// handler is constructed without those defaults).
	if got.Commit == "" && got.BuiltAt == "" {
		// Expected: empty values when not injected via ldflags in test.
	} else if got.Commit == "unknown" && got.BuiltAt == "unknown" {
		// Also fine: if someone wires the "unknown" defaults through.
	} else {
		t.Fatalf("expected empty or unknown defaults, got commit=%q built_at=%q", got.Commit, got.BuiltAt)
	}
}

func TestVersionHandler_MethodNotAllowed(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodPost, "/debug/version", nil)
	rr := httptest.NewRecorder()
	h.handleDebugVersion(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rr.Code)
	}
}

func TestVersionHandler_AuthRequired(t *testing.T) {
	h := newAuthHandler(t, "test-token")
	h.BuildCommit = strings.Repeat("a", 40)
	h.BuildTimestamp = "2026-05-21T12:00:00Z"

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/version", h.handleDebugVersion)
	srv := h.AuthMiddleware(mux)

	// No token → 401
	req := httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	req.Header.Set("Accept", "application/json")
	rr := httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 without token, got %d", rr.Code)
	}

	// Valid bearer → 200
	req = httptest.NewRequest(http.MethodGet, "/debug/version", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr = httptest.NewRecorder()
	srv.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("want 200 with valid token, got %d", rr.Code)
	}

	var got versionResponse
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Commit) != 40 {
		t.Fatalf("commit length = %d, want 40", len(got.Commit))
	}
}
