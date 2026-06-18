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

	s, err := a.CreateSession("biology", "STPA", "scratch")
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
	s, err := a.CreateSession("biology", "STPA", "scratch")
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
	s, err := a.CreateSession("biology", "STPA", "scratch")
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
	s, err := a.CreateSession("biology", "STPA", "scratch")
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
	s, err := a.CreateSession("biology", "STPA", "scratch")
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
		CourseID:     "biology",
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
	if got.CourseID != "biology" {
		t.Errorf("course_id = %q, want biology", got.CourseID)
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
	_ = a.RecordEvent(Event{Kind: "chat_turn", SessionID: &sid, CourseID: "biology", DurationMs: 2000, InputTokens: 100, OutputTokens: 50, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "chat_turn", SessionID: &sid, CourseID: "biology", DurationMs: 4000, InputTokens: 200, OutputTokens: 80, CreatedAt: now})
	// 1 tool_use ok, 1 tool_use failed
	_ = a.RecordEvent(Event{Kind: "tool_use", ToolName: "rag_search", OK: &okTrue, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "tool_use", ToolName: "rag_search", OK: &okFalse, CreatedAt: now})
	// 1 plan_toggle done, 1 undone
	_ = a.RecordEvent(Event{Kind: "plan_toggle", CourseID: "biology", OK: &okTrue, CreatedAt: now})
	_ = a.RecordEvent(Event{Kind: "plan_toggle", CourseID: "biology", OK: &okFalse, CreatedAt: now})
	// 1 pdf_open
	_ = a.RecordEvent(Event{Kind: "pdf_open", CourseID: "biology", CreatedAt: now})
	// 1 session_create
	_ = a.RecordEvent(Event{Kind: "session_create", CourseID: "cs101", CreatedAt: now})

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
	if s.CourseCounts["cs101"] != 1 {
		t.Errorf("CourseCounts[cs101] = %d, want 1", s.CourseCounts["cs101"])
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
	sess, _ := a.CreateSession("test-course", "test topic", "scratch")
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
	sess, _ := a.CreateSession("test-course", "test topic", "scratch")
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
	s, err := a.CreateSession("biology", "STPA", "scratch")
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
	visible, err := a.CreateSession("biology", "real work", "scratch")
	if err != nil {
		t.Fatalf("create visible: %v", err)
	}
	hidden, err := a.CreateSession("verifier-stats", "stats-verifier", "scratch")
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

func TestGetSessionByTaskExcludesArchived(t *testing.T) {
	a := newMemoryApp(t)
	s, err := a.CreateSessionForTask("cs101", "task-arch", "anchored then archived")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Visible while live.
	if _, ok, err := a.GetSessionByTask("cs101", "task-arch"); err != nil || !ok {
		t.Fatalf("expected found before archive, ok=%v err=%v", ok, err)
	}
	if _, err := a.DB.Exec("UPDATE sessions SET archived = 1 WHERE id = ?", s.ID); err != nil {
		t.Fatalf("archive: %v", err)
	}
	// Once archived, the task should resolve as having no live session.
	if _, ok, err := a.GetSessionByTask("cs101", "task-arch"); err != nil || ok {
		t.Errorf("expected not-found after archive, ok=%v err=%v", ok, err)
	}
}

func TestGetSessionByTaskIgnoresHidden(t *testing.T) {
	a := newMemoryApp(t)
	created, err := a.CreateSessionForTask("cs101", "task-hidden", "to be hidden")
	if err != nil {
		t.Fatalf("create for task: %v", err)
	}
	// Visible immediately after creation.
	if _, ok, err := a.GetSessionByTask("cs101", "task-hidden"); err != nil || !ok {
		t.Fatalf("expected found before hiding, got ok=%v err=%v", ok, err)
	}
	// Once hidden, GetSessionByTask must not return it.
	if _, err := a.DB.Exec("UPDATE sessions SET hidden = 1 WHERE id = ?", created.ID); err != nil {
		t.Fatalf("mark hidden: %v", err)
	}
	if _, ok, err := a.GetSessionByTask("cs101", "task-hidden"); err != nil || ok {
		t.Errorf("expected not-found after hiding, got ok=%v err=%v", ok, err)
	}
}

func TestCreateAndGetSessionByTask(t *testing.T) {
	a := newMemoryApp(t)

	if _, ok, err := a.GetSessionByTask("cs101", "task-uuid-1"); err != nil || ok {
		t.Fatalf("expected (not found), got ok=%v err=%v", ok, err)
	}

	created, err := a.CreateSessionForTask("cs101", "task-uuid-1", "Systems 3.3 Weak Isolation")
	if err != nil {
		t.Fatalf("create for task: %v", err)
	}
	if created.TaskID == nil || *created.TaskID != "task-uuid-1" {
		t.Fatalf("TaskID = %v, want task-uuid-1", created.TaskID)
	}
	if created.CourseID != "cs101" {
		t.Errorf("CourseID = %q, want cs101", created.CourseID)
	}

	got, ok, err := a.GetSessionByTask("cs101", "task-uuid-1")
	if err != nil || !ok {
		t.Fatalf("expected (found), got ok=%v err=%v", ok, err)
	}
	if got.ID != created.ID {
		t.Errorf("GetSessionByTask returned id %d, want %d", got.ID, created.ID)
	}

	if _, ok, _ := a.GetSessionByTask("cs101", "task-uuid-2"); ok {
		t.Errorf("unexpected session for task-uuid-2")
	}
}

func TestMigratePhase3Sessions(t *testing.T) {
	a := newMemoryApp(t)

	realID := mustSession(t, a, "biology", "Ch.8 Event Tree Analysis")
	verifierID := mustSession(t, a, "verifier-stats", "stats-verifier")
	smokeID := mustSession(t, a, "biology", "phase5 smoke v3")
	emptyID := mustSession(t, a, "cs101", "General")
	if err := a.SaveMessage(realID, "user", "hello"); err != nil {
		t.Fatalf("seed message: %v", err)
	}

	n, err := a.MigratePhase3Sessions()
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if n == 0 {
		t.Fatalf("expected migration to touch rows, got 0")
	}

	assertHidden := func(id int64, want bool) {
		var hidden int
		if err := a.DB.QueryRow("SELECT hidden FROM sessions WHERE id = ?", id).Scan(&hidden); err != nil {
			t.Fatalf("scan hidden %d: %v", id, err)
		}
		if (hidden == 1) != want {
			t.Errorf("session %d hidden=%d, want hidden=%v", id, hidden, want)
		}
	}
	assertArchived := func(id int64, want bool) {
		var archived int
		if err := a.DB.QueryRow("SELECT archived FROM sessions WHERE id = ?", id).Scan(&archived); err != nil {
			t.Fatalf("scan archived %d: %v", id, err)
		}
		if (archived == 1) != want {
			t.Errorf("session %d archived=%d, want archived=%v", id, archived, want)
		}
	}

	assertHidden(verifierID, true)
	assertHidden(smokeID, true)
	assertHidden(emptyID, true)
	assertHidden(realID, false)
	assertArchived(realID, true)
	assertArchived(verifierID, false)

	freshID := mustSession(t, a, "cs101", "new scratch")
	if err := a.SaveMessage(freshID, "user", "post-migration"); err != nil {
		t.Fatalf("seed fresh message: %v", err)
	}
	again, err := a.MigratePhase3Sessions()
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if again != 0 {
		t.Errorf("second migration touched %d rows, want 0 (guard failed)", again)
	}
	assertArchived(freshID, false)
}

// mustSession creates a session or fails the test.
func mustSession(t *testing.T, a *App, course, topic string) int64 {
	t.Helper()
	s, err := a.CreateSession(course, topic, "scratch")
	if err != nil {
		t.Fatalf("create session (%s/%s): %v", course, topic, err)
	}
	return s.ID
}

func TestMigrateSessionModeBackfills(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	app := NewApp(Config{VaultRoot: t.TempDir()}, db)
	now := "2026-05-30T00:00:00Z"
	if _, err := db.Exec("INSERT INTO sessions (course_id, topic, created_at, updated_at) VALUES ('c','scratchy',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO sessions (course_id, task_id, topic, created_at, updated_at) VALUES ('c','t1','studyy',?,?)", now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := app.MigrateSessionMode(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	var scratchMode, studyMode string
	if err := db.QueryRow("SELECT mode FROM sessions WHERE task_id IS NULL").Scan(&scratchMode); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("SELECT mode FROM sessions WHERE task_id = 't1'").Scan(&studyMode); err != nil {
		t.Fatal(err)
	}
	if scratchMode != "scratch" {
		t.Fatalf("task-less row should be scratch, got %q", scratchMode)
	}
	if studyMode != "study" {
		t.Fatalf("task row should stay study, got %q", studyMode)
	}
	n, err := app.MigrateSessionMode()
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if n != 0 {
		t.Fatalf("second run should change 0 rows, changed %d", n)
	}

	// Guard: a task-less authoring row created AFTER the migration flag is set
	// must survive a re-run (the flag short-circuits the backfill).
	now2 := "2026-05-30T01:00:00Z"
	if _, err := db.Exec("INSERT INTO sessions (course_id, topic, mode, created_at, updated_at) VALUES ('c','authoringy','authoring',?,?)", now2, now2); err != nil {
		t.Fatal(err)
	}
	if _, err := app.MigrateSessionMode(); err != nil {
		t.Fatalf("third migrate: %v", err)
	}
	var authMode string
	if err := db.QueryRow("SELECT mode FROM sessions WHERE topic = 'authoringy'").Scan(&authMode); err != nil {
		t.Fatal(err)
	}
	if authMode != "authoring" {
		t.Fatalf("authoring row clobbered after re-run, got %q", authMode)
	}
}

func TestGetSessionReturnsMode(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	app := NewApp(Config{VaultRoot: t.TempDir()}, db)
	now := "2026-05-30T00:00:00Z"
	res, err := db.Exec("INSERT INTO sessions (course_id, topic, mode, created_at, updated_at) VALUES ('c','t','authoring',?,?)", now, now)
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	s, err := app.GetSession(id)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if s.Mode != "authoring" {
		t.Fatalf("expected mode authoring, got %q", s.Mode)
	}
}

func TestCreateSessionPersistsMode(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	app := NewApp(Config{VaultRoot: t.TempDir()}, db)
	s, err := app.CreateSession("", "Design a new course", "authoring")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Mode != "authoring" {
		t.Fatalf("returned session mode = %q, want authoring", s.Mode)
	}
	got, err := app.GetSession(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "authoring" {
		t.Fatalf("persisted mode = %q, want authoring", got.Mode)
	}
}

func TestConfidenceToGrade(t *testing.T) {
	tests := []struct {
		confidence float64
		want       int
	}{
		{1.0, 5},
		{0.95, 5},
		{0.9, 5},
		{0.8, 4},
		{0.7, 4},
		{0.6, 3},
		{0.5, 3},
		{0.4, 2},
		{0.3, 2},
		{0.2, 1},
		{0.1, 1},
		{0.05, 0},
		{0.0, 0},
	}
	for _, tt := range tests {
		got := ConfidenceToGrade(tt.confidence)
		if got != tt.want {
			t.Errorf("ConfidenceToGrade(%v) = %d, want %d", tt.confidence, got, tt.want)
		}
	}
}

func TestSM2NextInterval(t *testing.T) {
	t.Run("first retrieval (n=0, grade=4) -> 1d, n=1", func(t *testing.T) {
		intervalMs, n, ef := SM2NextInterval(4, 0, 2.5, 0)
		if intervalMs != 86400000 {
			t.Errorf("interval = %d, want 86400000", intervalMs)
		}
		if n != 1 {
			t.Errorf("n = %d, want 1", n)
		}
		if ef != 2.5 {
			t.Errorf("ef = %f, want 2.5", ef)
		}
	})

	t.Run("second retrieval (n=1, grade=4) -> 6d, n=2", func(t *testing.T) {
		intervalMs, n, ef := SM2NextInterval(4, 1, 2.5, 86400000)
		if intervalMs != 6*86400000 {
			t.Errorf("interval = %d, want %d", intervalMs, 6*86400000)
		}
		if n != 2 {
			t.Errorf("n = %d, want 2", n)
		}
		if ef != 2.5 {
			t.Errorf("ef = %f, want 2.5", ef)
		}
	})

	t.Run("third retrieval (n=2, interval=6d, grade=4) -> ~15d, n=3, EF updated", func(t *testing.T) {
		intervalMs, n, ef := SM2NextInterval(4, 2, 2.5, 6*86400000)
		// SM-2: ef' = 2.5 + (0.1 - (5-4)*(0.08 + (5-4)*0.02)) = 2.5 + (0.1 - 1*0.1) = 2.5
		// interval = ceil(6 * 2.5) = 15 days
		expectedInterval := int64(15 * 86400000)
		if intervalMs != expectedInterval {
			t.Errorf("interval = %d, want %d", intervalMs, expectedInterval)
		}
		if n != 3 {
			t.Errorf("n = %d, want 3", n)
		}
		if ef != 2.5 {
			t.Errorf("ef = %f, want 2.5", ef)
		}
	})

	t.Run("forgetting (grade=2) -> n=0, interval=1d, EF unchanged", func(t *testing.T) {
		intervalMs, n, ef := SM2NextInterval(2, 2, 2.5, 6*86400000)
		if intervalMs != 86400000 {
			t.Errorf("interval = %d, want 86400000", intervalMs)
		}
		if n != 0 {
			t.Errorf("n = %d, want 0", n)
		}
		if ef != 2.5 {
			t.Errorf("ef = %f, want 2.5 (unchanged)", ef)
		}
	})

	t.Run("EF floor: repeated grade=5 does not drop below 1.3", func(t *testing.T) {
		// Grade 5: ef' = ef + (0.1 - (5-5)*(0.08 + (5-5)*0.02)) = ef + 0.1
		// Start with ef=1.3, add 0.1 repeatedly but we want to test
		// that a low-grade series doesn't go below 1.3.
		// Start with a very low ef (1.0) and grade=5 (which adds 0.1), should clamp to 1.3.
		_, _, ef := SM2NextInterval(5, 2, 1.0, 86400000)
		if ef < 1.3 {
			t.Errorf("ef = %f, want >= 1.3", ef)
		}
	})

	t.Run("EF ceiling: high EF does not exceed 2.5", func(t *testing.T) {
		// Start with ef=2.5, grade=4: ef' = 2.5 + (0.1 - (5-4)*(0.08+(5-4)*0.02)) = 2.5 + 0.0 = 2.5
		_, _, ef := SM2NextInterval(4, 2, 2.5, 86400000)
		if ef > 2.5 {
			t.Errorf("ef = %f, want <= 2.5", ef)
		}
	})
}

func TestLogConfidenceUpsertsRetrievalQueue(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	app := NewApp(Config{VaultRoot: t.TempDir()}, db)

	// Create a course and session so the FK on confidence_log(session_id) passes.
	if err := app.CreateCourse("test-course", "Test Course"); err != nil {
		t.Fatalf("create course: %v", err)
	}
	sess, err := app.CreateSession("test-course", "topic", "study")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Log confidence for "kc1" (0.5 -> grade 3) — should insert a retrieval_queue row.
	_, err = app.LogConfidence(sess.ID, "kc1", 0.5, "manual", "")
	if err != nil {
		t.Fatalf("log confidence: %v", err)
	}

	var dueAt int64
	var lastConf float64
	var n int
	var ef float64
	var intervalMs int64
	err = db.QueryRow("SELECT due_at, last_confidence, n, ef, interval_ms FROM retrieval_queue WHERE knowledge_component_id = ?", "kc1").Scan(&dueAt, &lastConf, &n, &ef, &intervalMs)
	if err != nil {
		t.Fatalf("query retrieval_queue: %v", err)
	}
	if lastConf != 0.5 {
		t.Errorf("last_confidence = %v, want 0.5", lastConf)
	}
	if n != 1 {
		t.Errorf("n = %d, want 1 (first retrieval with grade=3)", n)
	}
	if ef != 2.36 {
		t.Errorf("ef = %f, want 2.36 (grade=3 updates EF to 2.36)", ef)
	}
	if intervalMs != 86400000 {
		t.Errorf("interval_ms = %d, want 86400000 (1 day)", intervalMs)
	}

	// Log confidence again for "kc1" with higher confidence — should update, not duplicate.
	_, err = app.LogConfidence(sess.ID, "kc1", 0.8, "manual", "")
	if err != nil {
		t.Fatalf("log confidence second: %v", err)
	}
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM retrieval_queue WHERE knowledge_component_id = ?", "kc1").Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("retrieval_queue rows for kc1 = %d, want 1", count)
	}
	err = db.QueryRow("SELECT last_confidence, n, ef, interval_ms FROM retrieval_queue WHERE knowledge_component_id = ?", "kc1").Scan(&lastConf, &n, &ef, &intervalMs)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if lastConf != 0.8 {
		t.Errorf("last_confidence after update = %v, want 0.8", lastConf)
	}
	// Second retrieval: n=1, grade=4 (0.8 confidence), so next n=2, interval=6d, EF unchanged (grade 4 delta = 0)
	if n != 2 {
		t.Errorf("n after second retrieval = %d, want 2", n)
	}
	if intervalMs != 6*86400000 {
		t.Errorf("interval_ms after second retrieval = %d, want %d", intervalMs, 6*86400000)
	}
	if ef != 2.36 {
		t.Errorf("ef after second retrieval = %f, want 2.36", ef)
	}
}

func TestGetDueRetrievalItems(t *testing.T) {
	db, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()
	if err := InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	app := NewApp(Config{VaultRoot: t.TempDir()}, db)

	now := time.Now().UnixMilli()
	// Insert one due item (past due_at) and one not-due item (future due_at).
	db.Exec("INSERT INTO retrieval_queue (knowledge_component_id, due_at, last_confidence, n, ef, interval_ms, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"kc-due", now-1000, 0.3, 0, 2.5, 0, now)
	db.Exec("INSERT INTO retrieval_queue (knowledge_component_id, due_at, last_confidence, n, ef, interval_ms, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		"kc-notdue", now+86400000, 0.8, 0, 2.5, 0, now)

	items, err := app.GetDueRetrievalItems(now, 50)
	if err != nil {
		t.Fatalf("GetDueRetrievalItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 due item, got %d", len(items))
	}
	if items[0].KnowledgeComponentID != "kc-due" {
		t.Errorf("due item = %q, want kc-due", items[0].KnowledgeComponentID)
	}
	if items[0].LastConfidence != 0.3 {
		t.Errorf("last_confidence = %v, want 0.3", items[0].LastConfidence)
	}
}

func TestHasConfidenceAtLeast(t *testing.T) {
	a := newMemoryApp(t)
	sess, err := a.CreateSession("cs101", "t", "study")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	ok, err := a.HasConfidenceAtLeast("task-1", 0.7)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if ok {
		t.Fatalf("expected false with no confidence logged")
	}

	if _, err := a.LogConfidence(sess.ID, "task-1", 0.5, "manual", ""); err != nil {
		t.Fatalf("log 0.5: %v", err)
	}
	ok, _ = a.HasConfidenceAtLeast("task-1", 0.7)
	if ok {
		t.Fatalf("expected false at 0.5 < 0.7")
	}

	if _, err := a.LogConfidence(sess.ID, "task-1", 0.8, "manual", ""); err != nil {
		t.Fatalf("log 0.8: %v", err)
	}
	ok, _ = a.HasConfidenceAtLeast("task-1", 0.7)
	if !ok {
		t.Fatalf("expected true at latest 0.8 ≥ 0.7")
	}
}

func TestLogProbe(t *testing.T) {
	a := newMemoryApp(t)

	// Create a session so FK on session_id passes.
	if err := a.CreateCourse("c1", "Course 1"); err != nil {
		t.Fatalf("create course: %v", err)
	}
	sess, err := a.CreateSession("c1", "topic", "study")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	// Insert a knowledge component so the FK passes.
	kcID, err := a.CreateKnowledgeComponent("KC Title", "KC Body", "task-1", 0)
	if err != nil {
		t.Fatalf("create kc: %v", err)
	}

	probeID, err := a.LogProbe(kcID, "What is X?", "X is Y", "X is Z", 4, sess.ID)
	if err != nil {
		t.Fatalf("log probe: %v", err)
	}
	if probeID == 0 {
		t.Fatal("expected non-zero probe ID")
	}

	// Verify the probe row exists with correct fields.
	var learnerAnswer string
	var grade int
	var gradedAt int64
	var sessionID int64
	err = a.DB.QueryRow(
		"SELECT learner_answer, grade, graded_at, session_id FROM retrieval_probe WHERE id = ?", probeID,
	).Scan(&learnerAnswer, &grade, &gradedAt, &sessionID)
	if err != nil {
		t.Fatalf("query probe: %v", err)
	}
	if learnerAnswer != "X is Z" {
		t.Errorf("learner_answer = %q, want %q", learnerAnswer, "X is Z")
	}
	if grade != 4 {
		t.Errorf("grade = %d, want 4", grade)
	}
	if gradedAt == 0 {
		t.Error("graded_at should be set")
	}
	if sessionID != sess.ID {
		t.Errorf("session_id = %d, want %d", sessionID, sess.ID)
	}

	// Verify retrieval_queue was upserted with last_confidence = 0.8 (4/5).
	var lastConf float64
	err = a.DB.QueryRow(
		"SELECT last_confidence FROM retrieval_queue WHERE knowledge_component_id = ?", kcID,
	).Scan(&lastConf)
	if err != nil {
		t.Fatalf("query retrieval_queue: %v", err)
	}
	if lastConf != 0.8 {
		t.Errorf("last_confidence = %v, want 0.8", lastConf)
	}
}

func TestLogProbeNoAnswer(t *testing.T) {
	a := newMemoryApp(t)

	if err := a.CreateCourse("c1", "Course 1"); err != nil {
		t.Fatalf("create course: %v", err)
	}
	sess, err := a.CreateSession("c1", "topic", "study")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	kcID, err := a.CreateKnowledgeComponent("KC Title", "KC Body", "task-1", 0)
	if err != nil {
		t.Fatalf("create kc: %v", err)
	}

	// Log a question-only probe (no answer).
	probeID, err := a.LogProbe(kcID, "What is X?", "X is Y", "", 0, sess.ID)
	if err != nil {
		t.Fatalf("log probe: %v", err)
	}
	if probeID == 0 {
		t.Fatal("expected non-zero probe ID")
	}

	// Verify learner_answer and grade are NULL.
	var learnerAnswer *string
	var grade *int
	err = a.DB.QueryRow(
		"SELECT learner_answer, grade FROM retrieval_probe WHERE id = ?", probeID,
	).Scan(&learnerAnswer, &grade)
	if err != nil {
		t.Fatalf("query probe: %v", err)
	}
	if learnerAnswer != nil {
		t.Errorf("learner_answer = %v, want nil", learnerAnswer)
	}
	if grade != nil {
		t.Errorf("grade = %v, want nil", grade)
	}
}

func TestGetProbeQuestion(t *testing.T) {
	a := newMemoryApp(t)

	if err := a.CreateCourse("c1", "Course 1"); err != nil {
		t.Fatalf("create course: %v", err)
	}
	sess, err := a.CreateSession("c1", "topic", "study")
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	kcID, err := a.CreateKnowledgeComponent("KC Title", "KC Body", "task-1", 0)
	if err != nil {
		t.Fatalf("create kc: %v", err)
	}

	// Insert two probes for the same KC.
	if _, err := a.LogProbe(kcID, "Q1", "A1", "", 0, sess.ID); err != nil {
		t.Fatalf("log probe 1: %v", err)
	}
	if _, err := a.LogProbe(kcID, "Q2", "A2", "", 0, sess.ID); err != nil {
		t.Fatalf("log probe 2: %v", err)
	}

	// Should return the most recent question.
	probeID, question, err := a.GetProbeQuestion(kcID)
	if err != nil {
		t.Fatalf("get probe question: %v", err)
	}
	if question != "Q2" {
		t.Errorf("question = %q, want %q", question, "Q2")
	}
	if probeID == 0 {
		t.Error("expected non-zero probe ID")
	}
}

func TestGetProbeQuestionNone(t *testing.T) {
	a := newMemoryApp(t)

	probeID, question, err := a.GetProbeQuestion("nonexistent-kc")
	if err != nil {
		t.Fatalf("get probe question: %v", err)
	}
	if probeID != 0 {
		t.Errorf("probeID = %d, want 0", probeID)
	}
	if question != "" {
		t.Errorf("question = %q, want empty", question)
	}
}

func TestSearchKnowledgeComponents(t *testing.T) {
	app := newMemoryApp(t)
	_, _ = app.CreateKnowledgeComponent("Write skew cross-row invariant", "two txns, different rows", "", 0)
	_, _ = app.CreateKnowledgeComponent("Leader-based replication trade-off", "sync vs async", "", 0)

	hits, err := app.SearchKnowledgeComponents("skew", 10)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(hits) != 1 || hits[0].Title != "Write skew cross-row invariant" {
		t.Fatalf("hits = %+v", hits)
	}
	// Matches body too, case-insensitively.
	if h, _ := app.SearchKnowledgeComponents("SYNC", 10); len(h) != 1 {
		t.Fatalf("body match failed: %+v", h)
	}
}

func TestRebuildRetrievalQueueAtomsOnly(t *testing.T) {
	app := newMemoryApp(t)
	atomID, _ := app.CreateKnowledgeComponent("atom A", "b", "", 0)
	seed := func(kc string, v float64, at int64) {
		if _, err := app.DB.Exec(
			"INSERT INTO confidence_log (session_id, knowledge_component_id, value, source, created_at, raw_text) VALUES (NULL,?,?,?,?,?)",
			kc, v, "tool_call", at, ""); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	seed(atomID, 0.4, 1_000)           // atom: grade<3 → reset, 1 day
	seed(atomID, 0.9, 2_000)           // atom: grade 5, n0→1, 1 day
	seed("task-legacy-id", 0.8, 3_000) // NOT an atom → must be excluded

	n, err := app.RebuildRetrievalQueue()
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 atom queued, got %d", n)
	}
	items, _ := app.GetDueRetrievalItems(1<<62, 50)
	if len(items) != 1 || items[0].KnowledgeComponentID != atomID {
		t.Fatalf("queue = %+v", items)
	}
	if items[0].LastConfidence != 0.9 { // last event, not the poisoned earlier one
		t.Fatalf("last_confidence = %v", items[0].LastConfidence)
	}
	if items[0].DueAt != 2_000+int64(86_400_000) { // from event ts, not wall-clock
		t.Fatalf("due_at = %d", items[0].DueAt)
	}
}
