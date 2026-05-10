package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"study-app/agent"
)

func newTempDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "study.db")
	db, err := agent.OpenDB(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := agent.InitSchema(db); err != nil {
		t.Fatalf("init: %v", err)
	}
	db.Close()
	return path
}

func TestRunUnknownSubcommandExits2(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"clawcli", "wat"}, &stdout, &stderr, "")
	if code != 2 {
		t.Fatalf("exit code: %d", code)
	}
	if !strings.Contains(stderr.String(), "unknown") {
		t.Fatalf("stderr: %s", stderr.String())
	}
}

func TestRunMemorySaveJSONOutput(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
		"--course", "ce297",
		"--title", "no abbreviations",
		"--body", "spell out Software Control Category not SCC",
	}, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
	var got map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("parse: %v\n%s", err, stdout.String())
	}
	if got["kind"] != "feedback" || got["title"] != "no abbreviations" {
		t.Fatalf("got %+v", got)
	}
	if id, ok := got["id"].(float64); !ok || id == 0 {
		t.Fatalf("expected non-zero id, got %v", got["id"])
	}
}

func TestRunMemorySaveBodyFromStdin(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader("body from stdin\nsecond line")
	code := runWithStdin([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
		"--title", "stdin-test",
		"--body", "-",
	}, stdin, &stdout, &stderr, dbPath)
	if code != 0 {
		t.Fatalf("exit %d, stderr: %s", code, stderr.String())
	}
}

func TestRunMissingRequiredFlagExits2(t *testing.T) {
	dbPath := newTempDB(t)
	var stdout, stderr bytes.Buffer
	code := run([]string{
		"clawcli", "memory", "save",
		"--kind", "feedback",
	}, &stdout, &stderr, dbPath)
	if code != 2 {
		t.Fatalf("exit code: %d", code)
	}
}

var _ = os.Stdout
