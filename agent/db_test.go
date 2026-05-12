package agent

import (
	"testing"
	"time"
)

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

func TestSaveAssistantMessagePersistsReasoning(t *testing.T) {
	a := newMemoryApp(t)
	s, err := a.CreateSession("ce297", "STPA")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := a.SaveAssistantMessage(s.ID, "hello", "I thought about this"); err != nil {
		t.Fatalf("save: %v", err)
	}

	msgs, err := a.GetSessionHistory(s.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Errorf("content = %q, want %q", msgs[0].Content, "hello")
	}
	if msgs[0].Reasoning != "I thought about this" {
		t.Errorf("reasoning = %q, want %q", msgs[0].Reasoning, "I thought about this")
	}
}

func TestSaveMessageDoesNotSetReasoning(t *testing.T) {
	a := newMemoryApp(t)
	s, err := a.CreateSession("ce297", "STPA")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	if err := a.SaveMessage(s.ID, "user", "what is STPA?"); err != nil {
		t.Fatalf("save: %v", err)
	}

	msgs, err := a.GetSessionHistory(s.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if msgs[0].Reasoning != "" {
		t.Errorf("expected empty reasoning for user message, got %q", msgs[0].Reasoning)
	}
}

func TestRecordEventRoundtrip(t *testing.T) {
	a := newMemoryApp(t)
	sid := int64(42)
	e := Event{
		Kind:         "chat_turn",
		SessionID:    &sid,
		CourseID:     "ce297",
		Model:        "claude-opus-4-7",
		InputTokens:  100,
		OutputTokens: 50,
		DurationMs:   3200,
		CreatedAt:    time.Now().UnixMilli(),
	}
	if err := a.RecordEvent(e); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	evs, err := a.ListRecentEvents(10)
	if err != nil {
		t.Fatalf("ListRecentEvents: %v", err)
	}
	if len(evs) != 1 {
		t.Fatalf("expected 1 event, got %d", len(evs))
	}
	got := evs[0]
	if got.Kind != "chat_turn" {
		t.Errorf("kind = %q, want chat_turn", got.Kind)
	}
	if got.CourseID != "ce297" {
		t.Errorf("course_id = %q, want ce297", got.CourseID)
	}
	if got.DurationMs != 3200 {
		t.Errorf("duration_ms = %d, want 3200", got.DurationMs)
	}
	if got.SessionID == nil || *got.SessionID != 42 {
		t.Errorf("session_id = %v, want 42", got.SessionID)
	}
}

func TestRecordEventToolUse(t *testing.T) {
	a := newMemoryApp(t)
	okTrue := true
	if err := a.RecordEvent(Event{
		Kind:      "tool_use",
		ToolName:  "rag_search",
		OK:        &okTrue,
		CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}
	evs, _ := a.ListRecentEvents(10)
	if len(evs) != 1 || evs[0].ToolName != "rag_search" {
		t.Fatalf("unexpected events: %+v", evs)
	}
	if evs[0].OK == nil || !*evs[0].OK {
		t.Errorf("ok should be true")
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
