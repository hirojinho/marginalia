package agent

import "testing"

func newMemoryApp(t *testing.T) *App {
	t.Helper()
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	return NewApp(Config{VaultRoot: t.TempDir()}, db)
}

func TestGetMetaIntCorruptValueReturnsZero(t *testing.T) {
	a := newMemoryApp(t)
	if _, err := a.DB.Exec("INSERT INTO meta (key, value) VALUES (?, ?)", "last_session", "not-a-number"); err != nil {
		t.Fatalf("seed corrupt meta: %v", err)
	}
	got, err := a.getMetaInt("last_session")
	if err != nil {
		t.Fatalf("expected nil error on corrupt value, got %v", err)
	}
	if got != 0 {
		t.Fatalf("expected 0 on corrupt value, got %d", got)
	}
}

func TestGetMetaIntMissingKeyReturnsZero(t *testing.T) {
	a := newMemoryApp(t)
	got, err := a.getMetaInt("never_set")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestSetMetaIntRoundtrip(t *testing.T) {
	a := newMemoryApp(t)
	if err := a.setMetaInt("k", 42); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := a.getMetaInt("k")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != 42 {
		t.Fatalf("got %d", got)
	}
	// Overwrite via INSERT OR REPLACE.
	if err := a.setMetaInt("k", 7); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	got, _ = a.getMetaInt("k")
	if got != 7 {
		t.Fatalf("after overwrite got %d", got)
	}
}

func TestLoadActiveSessionIDDefaultsZero(t *testing.T) {
	a := newMemoryApp(t)
	a.LoadActiveSessionID()
	if id := a.ActiveSessionID(); id != 0 {
		t.Fatalf("expected 0, got %d", id)
	}
}

func TestSessionExistsLifecycle(t *testing.T) {
	a := newMemoryApp(t)
	exists, err := a.SessionExists(123)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if exists {
		t.Fatal("expected non-existent session")
	}

	s, err := a.CreateSession("ce297", "STPA")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	exists, err = a.SessionExists(s.ID)
	if err != nil || !exists {
		t.Fatalf("expected exists, got exists=%v err=%v", exists, err)
	}

	if err := a.DeleteSession(s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	exists, _ = a.SessionExists(s.ID)
	if exists {
		t.Fatal("expected gone after delete")
	}
}

func TestDeleteSessionClearsActive(t *testing.T) {
	a := newMemoryApp(t)
	s, err := a.CreateSession("ce297", "STPA")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := a.SetActiveSession(s.ID); err != nil {
		t.Fatalf("set active: %v", err)
	}
	if a.ActiveSessionID() != s.ID {
		t.Fatalf("active not set")
	}
	if err := a.DeleteSession(s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if a.ActiveSessionID() != 0 {
		t.Fatalf("expected active cleared, got %d", a.ActiveSessionID())
	}
}

func TestInitSchemaCreatesAgentMemoryTable(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	var name string
	row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='agent_memory'`)
	if err := row.Scan(&name); err != nil {
		t.Fatalf("agent_memory table missing: %v", err)
	}
	if name != "agent_memory" {
		t.Fatalf("got %q, want agent_memory", name)
	}
	row = db.QueryRow(`SELECT name FROM sqlite_master WHERE type='index' AND name='agent_memory_scope'`)
	if err := row.Scan(&name); err != nil {
		t.Fatalf("agent_memory_scope index missing: %v", err)
	}
}
