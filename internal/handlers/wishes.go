package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/carolineeey/wedding-suite-api/internal/models"
	"github.com/gorilla/mux"
)

// ListWishes returns approved guestbook messages, newest first. Public.
// Set ?all=true (used by the admin dashboard, still requires the admin
// token via the admin route) to include unapproved messages too.
func ListWishes(db *sql.DB, includeUnapproved bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, err := getSingleWedding(db)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusOK, []models.Wish{})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load wedding")
			return
		}

		limit := 50
		if l, parseErr := strconv.Atoi(r.URL.Query().Get("limit")); parseErr == nil && l > 0 && l <= 200 {
			limit = l
		}

		query := `
			SELECT id, wedding_id, guest_name, message, is_approved, created_at
			FROM wishes
			WHERE wedding_id = $1
		`
		if !includeUnapproved {
			query += ` AND is_approved = true `
		}
		query += ` ORDER BY created_at DESC LIMIT $2`

		rows, err := db.Query(query, wedding.ID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load wishes")
			return
		}
		defer rows.Close()

		wishes := []models.Wish{}
		for rows.Next() {
			var wi models.Wish
			if err := rows.Scan(&wi.ID, &wi.WeddingID, &wi.GuestName, &wi.Message, &wi.IsApproved, &wi.CreatedAt); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to read wish row")
				return
			}
			wishes = append(wishes, wi)
		}
		writeJSON(w, http.StatusOK, wishes)
	}
}

type createWishRequest struct {
	GuestName string `json:"guest_name"`
	Message   string `json:"message"`
}

// CreateWish lets a guest leave a guestbook message. Public.
func CreateWish(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, err := getSingleWedding(db)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusPreconditionFailed, "wedding has not been configured yet")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load wedding")
			return
		}

		var req createWishRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.GuestName == "" || req.Message == "" {
			writeError(w, http.StatusBadRequest, "guest_name and message are required")
			return
		}
		if len(req.Message) > 1000 {
			writeError(w, http.StatusBadRequest, "message is too long (max 1000 characters)")
			return
		}

		var id string
		err = db.QueryRow(`
			INSERT INTO wishes (wedding_id, guest_name, message)
			VALUES ($1, $2, $3)
			RETURNING id
		`, wedding.ID, req.GuestName, req.Message).Scan(&id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save wish")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

// SetWishApproval lets an admin hide/show a guestbook message without
// deleting it (moderation for spam or anything inappropriate).
func SetWishApproval(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]

		var req struct {
			IsApproved bool `json:"is_approved"`
		}
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		res, err := db.Exec(`UPDATE wishes SET is_approved = $1 WHERE id = $2`, req.IsApproved, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update wish")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "wish not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

// DeleteWish permanently removes a guestbook message. Admin-only.
func DeleteWish(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]
		res, err := db.Exec(`DELETE FROM wishes WHERE id = $1`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete wish")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "wish not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
