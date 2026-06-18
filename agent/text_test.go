package agent

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateRunesShortString(t *testing.T) {
	if got := TruncateRunes("hello", 100); got != "hello" {
		t.Fatalf("got %q, want hello", got)
	}
}

func TestTruncateRunesEmptyInput(t *testing.T) {
	if got := TruncateRunes("", 10); got != "" {
		t.Fatalf("got %q, want empty", got)
	}
}

func TestTruncateRunesAsciiBudgetEnforced(t *testing.T) {
	in := strings.Repeat("a", 300)
	got := TruncateRunes(in, 100)
	if len(got) > 100 {
		t.Fatalf("over budget: %d bytes", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("missing ellipsis: %q", got)
	}
}

func TestTruncateRunesMultiByteSafe(t *testing.T) {
	in := strings.Repeat("ção ", 100)
	got := TruncateRunes(in, 50)
	if len(got) > 50 {
		t.Fatalf("over budget: %d bytes", len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8 in output: %q", got)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("missing ellipsis: %q", got)
	}
}

func TestTruncateRunesExactBoundaryNoTrim(t *testing.T) {
	in := "hello world"
	if got := TruncateRunes(in, len(in)); got != in {
		t.Fatalf("got %q, want unchanged", got)
	}
}

func TestTruncateRunesBudgetSmallerThanEllipsis(t *testing.T) {
	got := TruncateRunes("hello", 2)
	if len(got) > 2 {
		t.Fatalf("over budget: %d bytes", len(got))
	}
	if !utf8.ValidString(got) {
		t.Fatalf("invalid UTF-8: %q", got)
	}
}
