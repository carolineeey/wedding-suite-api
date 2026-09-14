package middleware

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// Logging logs method, path, status-adjacent info, and duration for every request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// RequireAdmin protects admin-only routes with a shared bearer token
// (set via the ADMIN_TOKEN environment variable). This is intentionally
// simple for the MVP — just the couple and maybe a planner will ever use
// these routes — and can be swapped for real auth later without touching
// the routes that use it.
func RequireAdmin(adminToken string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if adminToken == "" {
				http.Error(w, `{"error":"admin routes are disabled: ADMIN_TOKEN is not configured"}`, http.StatusServiceUnavailable)
				return
			}

			auth := r.Header.Get("Authorization")
			token := strings.TrimPrefix(auth, "Bearer ")
			if token == "" || token != adminToken || token == auth {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
