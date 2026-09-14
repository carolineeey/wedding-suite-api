package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
	"github.com/gorilla/mux"
)

// GetGuestByCode looks up a guest by their invite code. This is what the
// wedding website calls to greet the guest by name and pre-fill their RSVP
// form (e.g. /rsvp/AB12CDE on the frontend).
func GetGuestByCode(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := mux.Vars(r)["code"]

		row := db.QueryRow(`
			SELECT id, wedding_id, invite_code, name, COALESCE(group_name, ''), max_guests,
			       rsvp_status, attending_count, COALESCE(rsvp_message, ''), rsvp_responded_at, created_at
			FROM guests
			WHERE invite_code = $1
		`, code)

		guest, err := scanGuest(row)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "invite code not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load guest")
			return
		}
		writeJSON(w, http.StatusOK, guest)
	}
}

type submitRSVPRequest struct {
	Attending      bool   `json:"attending"`
	AttendingCount int    `json:"attending_count"`
	Message        string `json:"message,omitempty"`
}

// SubmitRSVP records a guest's response. Public — protected only by the
// invite code being unguessable, which is enough for a wedding invite list.
func SubmitRSVP(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := mux.Vars(r)["code"]

		var existing models.Guest
		row := db.QueryRow(`
			SELECT id, wedding_id, invite_code, name, COALESCE(group_name, ''), max_guests,
			       rsvp_status, attending_count, COALESCE(rsvp_message, ''), rsvp_responded_at, created_at
			FROM guests
			WHERE invite_code = $1
		`, code)
		existing, err := scanGuest(row)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "invite code not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load guest")
			return
		}

		var req submitRSVPRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		status := models.RSVPDeclined
		count := 0
		if req.Attending {
			status = models.RSVPAttending
			count = req.AttendingCount
			if count <= 0 {
				count = 1
			}
			if count > existing.MaxGuests {
				writeError(w, http.StatusBadRequest, "attending_count exceeds the number of guests allowed on this invite")
				return
			}
		}

		now := time.Now()
		_, err = db.Exec(`
			UPDATE guests
			SET rsvp_status = $1, attending_count = $2, rsvp_message = $3, rsvp_responded_at = $4
			WHERE id = $5
		`, status, count, req.Message, now, existing.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to save RSVP")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "saved",
			"rsvp_status":     status,
			"attending_count": count,
		})
	}
}
