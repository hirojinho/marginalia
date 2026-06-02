package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyChatRouteRemoved(t *testing.T) {
	h := newTestHandler(t)
	mux := http.NewServeMux()
	h.Register(mux)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/chat", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("/chat should be 404 after removal (falls through to handleIndex), got %d", resp.StatusCode)
	}
}
