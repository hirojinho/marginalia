package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// resetWebFetchLimiter clears the package-level rate-limiter state so tests
// don't bleed into each other.
func resetWebFetchLimiter() {
	webFetchMu.Lock()
	webFetchTimes = nil
	webFetchMu.Unlock()
}

func TestReserveWebFetchSlot_AllowsFirstFive(t *testing.T) {
	resetWebFetchLimiter()
	for i := 0; i < 5; i++ {
		if wait := reserveWebFetchSlot(); wait != 0 {
			t.Fatalf("slot %d should be free, got wait=%v", i, wait)
		}
	}
}

func TestReserveWebFetchSlot_BlocksSixth(t *testing.T) {
	resetWebFetchLimiter()
	for i := 0; i < 5; i++ {
		_ = reserveWebFetchSlot()
	}
	if wait := reserveWebFetchSlot(); wait <= 0 {
		t.Fatalf("expected positive wait, got %v", wait)
	}
}

func TestReserveWebFetchSlot_StaleEntriesAreEvicted(t *testing.T) {
	resetWebFetchLimiter()
	// seed 5 stale timestamps
	stale := time.Now().Add(-2 * time.Minute)
	webFetchMu.Lock()
	webFetchTimes = []time.Time{stale, stale, stale, stale, stale}
	webFetchMu.Unlock()
	if wait := reserveWebFetchSlot(); wait != 0 {
		t.Fatalf("stale slots should evict, got wait=%v", wait)
	}
}

func TestToolWebFetch_BadJSON(t *testing.T) {
	resetWebFetchLimiter()
	out := ToolWebFetch(json.RawMessage(`bad`))
	if !strings.HasPrefix(out, "error:") {
		t.Fatalf("got %q", out)
	}
}

func TestToolWebFetch_RejectsNonHTTPScheme(t *testing.T) {
	resetWebFetchLimiter()
	out := ToolWebFetch(json.RawMessage(`{"URL":"ftp://example.com"}`))
	if !strings.Contains(out, "only http") {
		t.Fatalf("got %q", out)
	}
}

func TestToolWebFetch_RateLimited(t *testing.T) {
	resetWebFetchLimiter()
	webFetchMu.Lock()
	now := time.Now()
	webFetchTimes = []time.Time{now, now, now, now, now}
	webFetchMu.Unlock()
	out := ToolWebFetch(json.RawMessage(`{"URL":"https://example.com"}`))
	if !strings.HasPrefix(out, "rate limited:") {
		t.Fatalf("got %q", out)
	}
}
