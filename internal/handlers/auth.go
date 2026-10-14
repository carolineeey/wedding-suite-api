package handlers

import (
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/middleware"
	"github.com/carolineeey/wedding-suite-api/internal/models"
	"github.com/carolineeey/wedding-suite-api/internal/usecase"
)

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Login exchanges an admin's email and password for a bearer token to send
// on admin routes. Public, rate limited.
func Login(auth *usecase.AuthUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		session, err := auth.Login(r.Context(), req.Email, req.Password)
		if err != nil {
			writeUsecaseError(w, r, err, "admin not found", "failed to log in")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"token":      session.Token,
			"expires_at": session.ExpiresAt,
			"admin":      session.Admin,
		})
	}
}

// Logout ends the session the request's bearer token belongs to.
// Admin-only.
func Logout(auth *usecase.AuthUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := auth.Logout(r.Context(), middleware.BearerToken(r)); err != nil {
			writeUsecaseError(w, r, err, "session not found", "failed to log out")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
	}
}

// Me returns the logged-in admin and the weddings they manage, so the admin
// dashboard knows which slugs to offer after login. Admin-only.
func Me(auth *usecase.AuthUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		admin, ok := currentAdmin(w, r)
		if !ok {
			return
		}
		weddings, err := auth.Weddings(r.Context(), admin.ID)
		if err != nil {
			writeUsecaseError(w, r, err, "admin not found", "failed to load weddings")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"admin":    admin,
			"weddings": weddings,
		})
	}
}

// currentAdmin returns the admin middleware.RequireAdmin authenticated. On
// failure it writes a 401 itself and returns false; that only happens if a
// route was mounted outside the admin subrouter.
func currentAdmin(w http.ResponseWriter, r *http.Request) (models.Admin, bool) {
	admin, ok := middleware.AdminFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
	}
	return admin, ok
}
