package handler

import (
	"embed"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"study-app/agent"
)

// TestRuntimeHandlerReturnsLegacyModeByDefault verifies that GET /api/runtime
// returns {"mode":"legacy"} when AGENT_RUNTIME is not set.
func TestRuntimeHandlerReturnsLegacyModeByDefault(t *testing.T) {
	db, err := agent.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	app := agent.NewApp(agent.Config{VaultRoot: t.TempDir()}, db)
	h := New(app, nil, embed.FS{}, "", "")

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/runtime", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got["mode"] != "legacy" {
		t.Fatalf("mode = %q, want %q", got["mode"], "legacy")
	}
}

// TestRuntimeHandlerReturnsPiModeWhenConfigured verifies that GET /api/runtime
// returns {"mode":"pi"} when AgentRuntime is set to "pi".
func TestRuntimeHandlerReturnsPiModeWhenConfigured(t *testing.T) {
	db, err := agent.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	cfg := agent.Config{
		VaultRoot:    t.TempDir(),
		AgentRuntime: "pi",
	}
	app := agent.NewApp(cfg, db)
	h := New(app, nil, embed.FS{}, "", "")

	mux := http.NewServeMux()
	h.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/runtime", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rr.Code, rr.Body.String())
	}

	var got map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got["mode"] != "pi" {
		t.Fatalf("mode = %q, want %q", got["mode"], "pi")
	}
}
