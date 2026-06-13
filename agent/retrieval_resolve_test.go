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
	writePlanRaw(t, app, "ddia", `{"id":"ddia","name":"DDIA","phases":[
		{"title":"P3","tasks":[{"id":"wskew-id","title":"3.6 Write Skew"}]},
		{"title":"P4","clusters":[{"title":"c","tasks":[{"id":"repl-id","title":"4.1 Replication"}]}]}]}`)
	writePlanRaw(t, app, "ddia.json.bak-x", `not json`) // backup must be ignored

	idx, err := app.BuildTaskTitleIndex()
	if err != nil {
		t.Fatalf("BuildTaskTitleIndex: %v", err)
	}
	if ref, ok := idx["wskew-id"]; !ok || ref.CourseID != "ddia" || ref.Title != "3.6 Write Skew" {
		t.Fatalf("wskew-id = %+v ok=%v", ref, ok)
	}
	if ref, ok := idx["repl-id"]; !ok || ref.Title != "4.1 Replication" {
		t.Fatalf("repl-id (cluster) = %+v ok=%v", ref, ok)
	}
}
