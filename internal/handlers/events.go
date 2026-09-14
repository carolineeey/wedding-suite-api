package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type createEventRequest struct {
	Name      string `json:"name"`
	StartsAt  string `json:"starts_at"` // RFC3339
	EndsAt    string `json:"ends_at,omitempty"`
	VenueName string `json:"venue_name,omitempty"`
	Address   string `json:"address,omitempty"`
	Notes     string `json:"notes,omitempty"`
	SortOrder int    `json:"sort_order,omitempty"`
}

// CreateEvent adds an item to the wedding-day schedule (ceremony, reception, etc).
// Admin-only.
func CreateEvent(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, err := getSingleWedding(db)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusPreconditionFailed, "create the wedding record before adding events")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load wedding")
			return
		}

		var req createEventRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" || req.StartsAt == "" {
			writeError(w, http.StatusBadRequest, "name and starts_at are required")
			return
		}
		startsAt, err := time.Parse(time.RFC3339, req.StartsAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "starts_at must be RFC3339, e.g. 2027-06-12T09:00:00+07:00")
			return
		}
		var endsAt any
		if req.EndsAt != "" {
			t, err := time.Parse(time.RFC3339, req.EndsAt)
			if err != nil {
				writeError(w, http.StatusBadRequest, "ends_at must be RFC3339")
				return
			}
			endsAt = t
		}

		var id string
		err = db.QueryRow(`
			INSERT INTO events (wedding_id, name, starts_at, ends_at, venue_name, address, notes, sort_order)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id
		`, wedding.ID, req.Name, startsAt, endsAt, req.VenueName, req.Address, req.Notes, req.SortOrder).Scan(&id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create event: "+err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

// DeleteEvent removes an item from the schedule. Admin-only.
func DeleteEvent(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]
		res, err := db.Exec(`DELETE FROM events WHERE id = $1`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete event")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "event not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
