package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDebugMetricsReturnsOK(t *testing.T) {
	h := newTestHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/debug/metrics", nil)
	rr := httptest.NewRecorder()
	h.handleDebugMetrics(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Metrics") {
		t.Errorf("expected Metrics in response, got:\n%s", rr.Body.String())
	}
}

func TestDebugMetricsWindowParam(t *testing.T) {
	h := newTestHandler(t)
	for _, w := range []string{"7d", "30d", "90d"} {
		req := httptest.NewRequest(http.MethodGet, "/debug/metrics?window="+w, nil)
		rr := httptest.NewRecorder()
		h.handleDebugMetrics(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("window=%s status=%d", w, rr.Code)
		}
	}
}
