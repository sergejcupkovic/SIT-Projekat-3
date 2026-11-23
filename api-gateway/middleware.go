package main

import (
	"net/http"
	"net/url"
	"strings"
)

// corsMiddleware allows the browser frontend to call the gateway directly
// (https://localhost:8443) in addition to going through nginx. Preflight
// OPTIONS requests are answered here, before reaching the gRPC gateway mux
// (which has no OPTIONS routes).
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// securityHeaders adds hardening headers to every response.
//
// Strict-Transport-Security (HSTS) is intentionally NOT set: browsers apply it
// to the whole host name on every port, so on "localhost" it would force other
// local dev tools (e.g. Mailpit on http://localhost:8025) onto https. Add it
// only when the gateway is served under a real domain.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// verifyRedirect keeps old verification links working: when a browser opens
// GET /api/users/verify?token=... directly (Accept: text/html), send it to the
// frontend, which then calls the API itself. This is purely an HTTP concern,
// so it lives in the gateway, not in the gRPC users-service.
func verifyRedirect(next http.Handler, frontendURL string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet &&
			r.URL.Path == "/api/users/verify" &&
			strings.Contains(r.Header.Get("Accept"), "text/html") {
			if token := r.URL.Query().Get("token"); token != "" {
				http.Redirect(w, r, frontendURL+"/?token="+url.QueryEscape(token), http.StatusFound)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
