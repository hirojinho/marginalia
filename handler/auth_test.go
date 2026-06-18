package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newAuthHandler(t *testing.T, token string) *Handler {
	h := newTestHandler(t)
	h.App.Config.AuthToken = token
	return h
}

func wrappedMux(h *Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/login", h.handleLogin)
	mux.HandleFunc("/test/probe", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return h.AuthMiddleware(mux)
}

func TestAuth_NoTokenConfigured_LetsThrough(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, ""))
	req := httptest.NewRequest(http.MethodGet, "/test/probe", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestAuth_RejectsMissingToken(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, "secret"))
	req := httptest.NewRequest(http.MethodGet, "/test/probe", nil)
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestAuth_AcceptsBearer(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, "secret"))
	req := httptest.NewRequest(http.MethodGet, "/test/probe", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestAuth_RejectsWrongBearer(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, "secret"))
	req := httptest.NewRequest(http.MethodGet, "/test/probe", nil)
	req.Header.Set("Authorization", "Bearer nope")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
}

func TestAuth_AcceptsCookie(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, "secret"))
	req := httptest.NewRequest(http.MethodGet, "/test/probe", nil)
	req.AddCookie(&http.Cookie{Name: authCookieName, Value: "secret"})
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", rec.Code)
	}
}

func TestAuth_BrowserGetRedirectsToLogin(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, "secret"))
	req := httptest.NewRequest(http.MethodGet, "/test/probe", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Fatalf("want redirect to /login, got %q", loc)
	}
}

func TestAuth_LoginAlwaysReachable(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, "secret"))
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 (form), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<form") {
		t.Fatalf("expected login form in body")
	}
}

func TestLogin_ValidTokenSetsCookieAndRedirects(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, "secret"))
	req := httptest.NewRequest(http.MethodGet, "/login?token=secret", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("want 302, got %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	var got *http.Cookie
	for _, c := range cookies {
		if c.Name == authCookieName {
			got = c
		}
	}
	if got == nil {
		t.Fatal("auth cookie not set")
	}
	if got.Value != "secret" {
		t.Fatalf("cookie value = %q, want secret", got.Value)
	}
	if !got.HttpOnly || !got.Secure || got.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie security flags wrong: %+v", got)
	}
}

func TestLogin_InvalidTokenShowsForm(t *testing.T) {
	srv := wrappedMux(newAuthHandler(t, "secret"))
	req := httptest.NewRequest(http.MethodGet, "/login?token=wrong", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid token") {
		t.Fatalf("expected error message in body")
	}
}
