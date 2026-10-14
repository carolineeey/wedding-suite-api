package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
	"github.com/carolineeey/wedding-suite-api/internal/usecase"
	"github.com/gorilla/mux"
)

// Logging logs method, path, status-adjacent info, and duration for every
// request. It also keeps a copy of the request body as handlers read it, so
// LogError can show the payload that led to a failure.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		body := &bodyCapture{ReadCloser: r.Body}
		r.Body = body
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), bodyKey, body)))
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

// Timeout cancels each request's context after d. Database queries run with
// that context, so a stuck query fails instead of holding the request open;
// the server's WriteTimeout alone does not cancel the context.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Authenticator resolves a bearer token to the admin it belongs to.
// usecase.AuthUsecase satisfies it.
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (models.Admin, error)
}

// WeddingAccess decides whether an admin may manage the wedding a slug
// names, returning models.ErrNotFound when not. usecase.AuthUsecase
// satisfies it.
type WeddingAccess interface {
	CanManage(ctx context.Context, adminID, slug string) error
}

type ctxKey int

const (
	adminKey ctxKey = iota
	bodyKey
)

// AdminFromContext returns the admin RequireAdmin authenticated for this
// request.
func AdminFromContext(ctx context.Context) (models.Admin, bool) {
	a, ok := ctx.Value(adminKey).(models.Admin)
	return a, ok
}

// BearerToken returns the token from an "Authorization: Bearer ..." header,
// or "" if there is none.
func BearerToken(r *http.Request) string {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return ""
	}
	return strings.TrimSpace(token)
}

// RequireAdmin rejects requests without a valid session token and puts the
// admin it belongs to in the request context for the routes behind it.
func RequireAdmin(auth Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admin, err := auth.Authenticate(r.Context(), BearerToken(r))
			if errors.Is(err, usecase.ErrInvalidCredentials) {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			if err != nil {
				LogError(r, "authenticating admin", err)
				writeError(w, http.StatusInternalServerError, "failed to authenticate")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), adminKey, admin)))
		})
	}
}

// RequireWeddingAccess guards every route under /admin/w/{slug}: the admin
// RequireAdmin authenticated must have been granted that wedding. A wedding
// they were not granted is a 404, the same as an unknown slug, so admins
// cannot probe for other couples' slugs. Doing it here rather than in each
// handler means a new admin route cannot forget the check.
func RequireWeddingAccess(access WeddingAccess) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			admin, ok := AdminFromContext(r.Context())
			if !ok {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			err := access.CanManage(r.Context(), admin.ID, mux.Vars(r)["slug"])
			if errors.Is(err, models.ErrNotFound) {
				writeError(w, http.StatusNotFound, "wedding not found")
				return
			}
			if err != nil {
				LogError(r, "checking wedding access", err)
				writeError(w, http.StatusInternalServerError, "failed to check wedding access")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RateLimit allows each client IP at most limit requests per window, shared
// across every route wrapped by the returned middleware. Counts live in
// memory in fixed windows — enough for a single instance; a restart simply
// resets them.
func RateLimit(limit int, window time.Duration) func(http.Handler) http.Handler {
	var (
		mu          sync.Mutex
		counts      = make(map[string]int)
		windowStart = time.Now()
	)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r)

			mu.Lock()
			now := time.Now()
			if now.Sub(windowStart) >= window {
				counts = make(map[string]int)
				windowStart = now
			}
			counts[ip]++
			exceeded := counts[ip] > limit
			retryAfter := windowStart.Add(window).Sub(now)
			mu.Unlock()

			if exceeded {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter/time.Second)+1))
				writeError(w, http.StatusTooManyRequests, "too many requests, please try again later")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// clientIP identifies the caller for rate limiting. Behind Render's proxy,
// RemoteAddr is the proxy itself, so the last X-Forwarded-For entry (the
// address the proxy saw) is used; earlier entries are client-supplied and
// can be forged. Revisit this if a CDN is put in front of the service.
func clientIP(r *http.Request) string {
	if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
		entries := strings.Split(values[len(values)-1], ",")
		if ip := strings.TrimSpace(entries[len(entries)-1]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// writeError writes the same {"error": "..."} body the handlers use.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
