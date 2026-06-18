package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func writePlanRaw(t *testing.T, app *App, courseID, body string) {
	t.Helper()
	dir := app.VaultPath("data", "plans")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, courseID+".json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
}

func TestBuildTaskTitleIndex(t *testing.T) {
	app := newMemoryApp(t)
	writePlanRaw(t, app, "cs101", `{"id":"cs101","name":"Systems","phases":[
		{"title":"P3","tasks":[{"id":"wskew-id","title":"3.6 Write Skew"}]},
		{"title":"P4","clusters":[{"title":"c","tasks":[{"id":"repl-id","title":"4.1 Replication"}]}]}]}`)
	writePlanRaw(t, app, "cs101.json.bak-x", `not json`) // backup must be ignored

	idx, err := app.BuildTaskTitleIndex()
	if err != nil {
		t.Fatalf("BuildTaskTitleIndex: %v", err)
	}
	if ref, ok := idx["wskew-id"]; !ok || ref.CourseID != "cs101" || ref.Title != "3.6 Write Skew" {
		t.Fatalf("wskew-id = %+v ok=%v", ref, ok)
	}
	if ref, ok := idx["repl-id"]; !ok || ref.Title != "4.1 Replication" {
		t.Fatalf("repl-id (cluster) = %+v ok=%v", ref, ok)
	}
}

func TestResolveAtomLabelAndIsAtom(t *testing.T) {
	app := newMemoryApp(t)
	writePlanRaw(t, app, "cs101", `{"id":"cs101","name":"Systems","phases":[
		{"title":"P3","tasks":[{"id":"wskew-task","title":"3.6 Write Skew"}]}]}`)
	atomID, err := app.CreateKnowledgeComponent("Write skew breaks a cross-row invariant", "body", "wskew-task", 0)
	if err != nil {
		t.Fatalf("create atom: %v", err)
	}
	idx, _ := app.BuildTaskTitleIndex()

	title, course, ok := app.ResolveAtomLabel(atomID, idx)
	if !ok || title != "Write skew breaks a cross-row invariant" || course != "cs101" {
		t.Fatalf("label = (%q,%q,%v)", title, course, ok)
	}
	// A plan task id is NOT a valid atom key.
	if app.IsAtom("wskew-task") {
		t.Fatalf("task id must not validate as an atom")
	}
	if !app.IsAtom(atomID) {
		t.Fatalf("atom id must validate")
	}
	if app.IsAtom("22") {
		t.Fatalf("legacy key must not validate")
	}
}
