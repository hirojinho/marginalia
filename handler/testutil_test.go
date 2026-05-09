package handler

import (
	"embed"
	"testing"

	"study-app/agent"
)

// newTestHandler builds a Handler backed by an in-memory SQLite DB and
// a fresh temp directory as VaultRoot. The LLM client is nil — tests
// that hit chat endpoints must not be in this set.
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := agent.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	app := agent.NewApp(agent.Config{VaultRoot: t.TempDir()}, db)
	return New(app, nil, embed.FS{})
}
