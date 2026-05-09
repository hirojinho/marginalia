package handler

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"strings"
)

const authCookieName = "auth"

// loginPage is shown when /login is hit without a (valid) token. The
// inline form posts the token via query string; on success the
// middleware sets the cookie and redirects to /.
const loginPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>study-app login</title>
<style>body{font-family:system-ui;margin:8em auto;max-width:24em;text-align:center}
input{width:100%;padding:.6em;font-size:1em;box-sizing:border-box}
button{margin-top:.6em;padding:.6em 1em;font-size:1em}</style></head>
<body><h2>study-app</h2>
<form method="GET" action="/login">
<input name="token" type="password" placeholder="token" autofocus>
<button type="submit">enter</button>
</form>%s</body></html>`

// AuthMiddleware gates all requests except /login when AuthToken is
// configured. Accepts either a cookie or an Authorization: Bearer
// header. Constant-time compares the token.
func (h *Handler) AuthMiddleware(next http.Handler) http.Handler {
	token := h.App.Config.AuthToken
	if token == "" {
		slog.Warn("AUTH_TOKEN not set — auth middleware will let all requests through")
		return next
	}
	tokenBytes := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/login" {
			next.ServeHTTP(w, r)
			return
		}
		if authorized(r, tokenBytes) {
			next.ServeHTTP(w, r)
			return
		}
		// HTML clients get a redirect to /login; everything else gets 401.
		if strings.Contains(r.Header.Get("Accept"), "text/html") && r.Method == http.MethodGet {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		writeError(w, http.StatusUnauthorized, "unauthorized")
	})
}

func authorized(r *http.Request, token []byte) bool {
	if c, err := r.Cookie(authCookieName); err == nil {
		if subtle.ConstantTimeCompare([]byte(c.Value), token) == 1 {
			return true
		}
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(h, "Bearer ")), token) == 1 {
			return true
		}
	}
	return false
}

// handleLogin accepts ?token=... and, on match, sets the auth cookie
// and redirects to /. Otherwise renders the login form. GET-only — the
// cookie is HttpOnly+Secure+SameSite=Strict, so embedding-attack risk
// from the GET is acceptable for a single-user app.
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if methodNotAllowed(w, r, http.MethodGet) {
		return
	}
	configured := h.App.Config.AuthToken
	supplied := r.URL.Query().Get("token")
	if configured == "" {
		// Auth disabled — just bounce to /.
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	if supplied != "" && subtle.ConstantTimeCompare([]byte(supplied), []byte(configured)) == 1 {
		http.SetCookie(w, &http.Cookie{
			Name:     authCookieName,
			Value:    configured,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   60 * 60 * 24 * 365, // 1 year
		})
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	msg := ""
	if supplied != "" {
		w.WriteHeader(http.StatusUnauthorized)
		msg = "<p style='color:#a00'>invalid token</p>"
	}
	if _, err := w.Write([]byte(strings.Replace(loginPage, "%s", msg, 1))); err != nil {
		slog.Warn("write login page", "err", err)
	}
}
