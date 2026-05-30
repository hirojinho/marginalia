package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlanFingerprintDetectsChange(t *testing.T) {
	app := NewApp(Config{VaultRoot: t.TempDir()}, nil)
	id := "ce297"

	// Absent plan → empty fingerprint (the before-state when a turn creates a plan).
	if fp := app.PlanFingerprint(id); fp != "" {
		t.Fatalf("absent plan should fingerprint to empty, got %q", fp)
	}

	dir := app.VaultPath("data", "plans")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, id+".json")
	if err := os.WriteFile(path, []byte(`{"id":"ce297","phases":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	fp1 := app.PlanFingerprint(id)
	if fp1 == "" {
		t.Fatal("present plan should fingerprint non-empty")
	}

	// Identical content → identical fingerprint (no false-positive refresh on a
	// redundant rewrite of the same plan).
	if fp := app.PlanFingerprint(id); fp != fp1 {
		t.Fatalf("identical content should fingerprint identically: %q vs %q", fp, fp1)
	}

	// Changed content → different fingerprint (the case that must trigger a rail refresh).
	if err := os.WriteFile(path, []byte(`{"id":"ce297","phases":[{"title":"P1"}]}`), 0644); err != nil {
		t.Fatal(err)
	}
	if fp2 := app.PlanFingerprint(id); fp2 == fp1 {
		t.Fatal("changed content should change the fingerprint")
	}
}
