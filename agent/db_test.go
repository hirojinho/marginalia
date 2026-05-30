package agent

import (
	"strings"
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

func TestUpdateSessionPDF(t *testing.T) {
	a := newMemoryApp(t)
	s, err := a.CreateSession("ce297", "STPA")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := a.UpdateSessionPDF(s.ID, 42, 17); err != nil {
		t.Fatalf("update session pdf: %v", err)
	}

	got, err := a.GetSession(s.ID)
	if err != nil {
		t.Fatalf("get session: %v", err)
	}
	if got.LastPdfID == nil || *got.LastPdfID != 42 {
		t.Errorf("last_pdf_id = %v, want 42", got.LastPdfID)
	}
	if got.LastPage != 17 {
		t.Errorf("last_page = %d, want 17", got.LastPage)
	}

	if err := a.UpdateSessionPDF(99999, 1, 1); err == nil {
		t.Error("expected error updating a non-existent session")
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

func TestQueryEventSummaryCountsAndAggregates(t *testing.T) {
	a := newMemoryApp(t)
	now := time.Now().UnixMilli()

	okTrue, okFalse := true, false
	sid := int64(1)

	// 2 chat turns
	_ = a.RecordEvent(Event{Kind: "chat_turn", SessionID: &sid, CourseID: "ce297", DurationMs: 2000, InputTokens: 100, OutputTokens: 50, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "chat_turn", SessionID: &sid, CourseID: "ce297", DurationMs: 4000, InputTokens: 200, OutputTokens: 80, CreatedAt: now})
	// 1 tool_use ok, 1 tool_use failed
	_ = a.RecordEvent(Event{Kind: "tool_use", ToolName: "rag_search", OK: &okTrue, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "tool_use", ToolName: "rag_search", OK: &okFalse, CreatedAt: now})
	// 1 plan_toggle done, 1 undone
	_ = a.RecordEvent(Event{Kind: "plan_toggle", CourseID: "ce297", OK: &okTrue, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "plan_toggle", CourseID: "ce297", OK: &okFalse, CreatedAt: now})
	// 1 pdf_open
	_ = a.RecordEvent(Event{Kind: "pdf_open", CourseID: "ce297", CreatedAt: now})
	// 1 session_create
	_ = a.RecordEvent(Event{Kind: "session_create", CourseID: "ddia", CreatedAt: now})

	since := time.UnixMilli(now - 1000)
	s, err := a.QueryEventSummary(since)
	if err != nil {
		t.Fatalf("QueryEventSummary: %v", err)
	}

	if s.TurnCount != 2 {
		t.Errorf("TurnCount = %d, want 2", s.TurnCount)
	}
	if s.AvgLatencyMs != 3000 {
		t.Errorf("AvgLatencyMs = %d, want 3000", s.AvgLatencyMs)
	}
	if s.InputTokens != 300 {
		t.Errorf("InputTokens = %d, want 300", s.InputTokens)
	}
	if s.OutputTokens != 130 {
		t.Errorf("OutputTokens = %d, want 130", s.OutputTokens)
	}
	if s.ToolCounts["rag_search"] != 2 {
		t.Errorf("ToolCounts[rag_search] = %d, want 2", s.ToolCounts["rag_search"])
	}
	if s.CourseCounts["ddia"] != 1 {
		t.Errorf("CourseCounts[ddia] = %d, want 1", s.CourseCounts["ddia"])
	}
	if s.PlanDone != 1 || s.PlanUndone != 1 {
		t.Errorf("PlanDone=%d PlanUndone=%d, want 1 1", s.PlanDone, s.PlanUndone)
	}
	if s.PDFOpens != 1 {
		t.Errorf("PDFOpens = %d, want 1", s.PDFOpens)
	}
}

func TestPruneOldEventsRemovesOldRows(t *testing.T) {
	a := newMemoryApp(t)
	now := time.Now()
	old := now.Add(-100 * 24 * time.Hour).UnixMilli()
	recent := now.UnixMilli()

	for i := 0; i < 3; i++ {
		_ = a.RecordEvent(Event{Kind: "session_create", CreatedAt: old})
	}
	for i := 0; i < 2; i++ {
		_ = a.RecordEvent(Event{Kind: "session_create", CreatedAt: recent})
	}

	cutoff := now.Add(-90 * 24 * time.Hour)
	deleted, err := a.PruneOldEvents(cutoff)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if deleted != 3 {
		t.Errorf("deleted = %d, want 3", deleted)
	}
	evs, _ := a.ListRecentEvents(100)
	if len(evs) != 2 {
		t.Errorf("remaining = %d, want 2", len(evs))
	}
}

func TestLogConfidence_ValidRow(t *testing.T) {
	a := newMemoryApp(t)
	sess, _ := a.CreateSession("test-course", "test topic")
	id, err := a.LogConfidence(sess.ID, "task-uuid-1", 0.75, "tool_call", "pretty confident")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}
	points, err := a.GetConfidenceTrajectory("task-uuid-1", 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 row, got %d", len(points))
	}
	if points[0].Value != 0.75 {
		t.Errorf("value = %v, want 0.75", points[0].Value)
	}
	if points[0].Source != "tool_call" {
		t.Errorf("source = %q, want tool_call", points[0].Source)
	}
	if points[0].RawText != "pretty confident" {
		t.Errorf("raw_text = %q, want 'pretty confident'", points[0].RawText)
	}
}

func TestLogConfidence_RejectsOutOfRange(t *testing.T) {
	a := newMemoryApp(t)
	if _, err := a.LogConfidence(0, "k1", -0.1, "tool_call", ""); err == nil {
		t.Error("expected error for value=-0.1")
	}
	if _, err := a.LogConfidence(0, "k1", 1.5, "tool_call", ""); err == nil {
		t.Error("expected error for value=1.5")
	}
}

func TestLogConfidence_RejectsInvalidSource(t *testing.T) {
	a := newMemoryApp(t)
	if _, err := a.LogConfidence(0, "k1", 0.5, "bad_source", ""); err == nil {
		t.Error("expected error for invalid source")
	}
}

func TestGetConfidenceTrajectory_OrderingAndLimit(t *testing.T) {
	a := newMemoryApp(t)
	sess, _ := a.CreateSession("test-course", "test topic")
	for i := 0; i < 5; i++ {
		_, err := a.LogConfidence(sess.ID, "kc-ord", 0.5, "tool_call", "")
		if err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	points, err := a.GetConfidenceTrajectory("kc-ord", 3)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(points))
	}
	// Must be descending by created_at
	for i := 1; i < len(points); i++ {
		if points[i].CreatedAt > points[i-1].CreatedAt {
			t.Errorf("row %d (%d) is newer than row %d (%d)", i, points[i].CreatedAt, i-1, points[i-1].CreatedAt)
		}
	}
}

func TestToolLogConfidence_DispatchedRoundTrip(t *testing.T) {
	a := newMemoryApp(t)
	sess, _ := a.CreateSession("test-course", "test topic")
	input := `{"knowledge_component_id":"test-kc","value":0.8,"raw":"8/10"}`
	_ = sess // session set in memory via CreateSession
	result := a.ToolLogConfidence([]byte(input))
	if strings.Contains(result, "error") {
		t.Fatalf("tool returned error: %s", result)
	}
	points, err := a.GetConfidenceTrajectory("test-kc", 10)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 row, got %d", len(points))
	}
	if points[0].Value != 0.8 {
		t.Errorf("value = %v, want 0.8", points[0].Value)
	}
}

func TestKnowledgeComponentCRUD(t *testing.T) {
	a := newMemoryApp(t)

	// Create with provenance
	id, err := a.CreateKnowledgeComponent("Test Title", "Test Body", "task-123", 0)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	// Get round-trip
	kc, err := a.GetKnowledgeComponent(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if kc == nil {
		t.Fatal("expected component, got nil")
	}
	if kc.Title != "Test Title" {
		t.Errorf("title = %q, want %q", kc.Title, "Test Title")
	}
	if kc.Body != "Test Body" {
		t.Errorf("body = %q, want %q", kc.Body, "Test Body")
	}
	if kc.SourceTaskID != "task-123" {
		t.Errorf("source_task_id = %q, want %q", kc.SourceTaskID, "task-123")
	}
	if kc.SourceSessionID != 0 {
		t.Errorf("source_session_id = %d, want 0", kc.SourceSessionID)
	}

	// Get missing
	missing, err := a.GetKnowledgeComponent("nonexistent-id")
	if err != nil {
		t.Fatalf("get missing: %v", err)
	}
	if missing != nil {
		t.Fatal("expected nil for missing id")
	}

	// List
	id2, err := a.CreateKnowledgeComponent("Second", "Body 2", "", 0)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	_ = id2
	// Ensure second record is newer by bumping its updated_at
	_, _ = a.DB.Exec("UPDATE knowledge_components SET created_at = created_at + 1, updated_at = updated_at + 1 WHERE id = ?", id2)
	list, err := a.ListKnowledgeComponents(50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 components, got %d", len(list))
	}
	if list[0].Title != "Second" {
		t.Errorf("expected newest first, got title %q", list[0].Title)
	}
}

func TestKnowledgeComponentEmptyTitleOrBodyRejected(t *testing.T) {
	a := newMemoryApp(t)
	_, err := a.CreateKnowledgeComponent("", "body", "", 0)
	if err == nil {
		t.Fatal("expected error for empty title")
	}
	_, err = a.CreateKnowledgeComponent("title", "", "", 0)
	if err == nil {
		t.Fatal("expected error for empty body")
	}
}

func TestSessionTaskIDRoundTrips(t *testing.T) {
	a := newMemoryApp(t)
	s, err := a.CreateSession("ce297", "STPA")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := a.GetSession(s.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.TaskID != nil {
		t.Errorf("TaskID = %v, want nil", got.TaskID)
	}
	if got.Archived {
		t.Errorf("Archived = true, want false")
	}
}

func TestListSessionsExcludesHidden(t *testing.T) {
	a := newMemoryApp(t)
	visible, err := a.CreateSession("ce297", "real work")
	if err != nil {
		t.Fatalf("create visible: %v", err)
	}
	hidden, err := a.CreateSession("verifier-stats", "stats-verifier")
	if err != nil {
		t.Fatalf("create hidden: %v", err)
	}
	if _, err := a.DB.Exec("UPDATE sessions SET hidden = 1 WHERE id = ?", hidden.ID); err != nil {
		t.Fatalf("mark hidden: %v", err)
	}

	list, err := a.ListSessions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	for _, s := range list {
		if s.ID == hidden.ID {
			t.Errorf("hidden session %d appeared in ListSessions", hidden.ID)
		}
	}
	var sawVisible bool
	for _, s := range list {
		if s.ID == visible.ID {
			sawVisible = true
		}
	}
	if !sawVisible {
		t.Errorf("visible session %d missing from ListSessions", visible.ID)
	}
}
