package handlers

import (
	"database/sql"
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/models"
	"github.com/gorilla/mux"
)

// ListGuests returns every guest record for the wedding, including RSVP
// status. Admin-only.
func ListGuests(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, err := getSingleWedding(db)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusOK, []models.Guest{})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load wedding")
			return
		}

		rows, err := db.Query(`
			SELECT id, wedding_id, invite_code, name, COALESCE(group_name, ''), max_guests,
			       rsvp_status, attending_count, COALESCE(rsvp_message, ''), rsvp_responded_at, created_at
			FROM guests
			WHERE wedding_id = $1
			ORDER BY created_at ASC
		`, wedding.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load guests")
			return
		}
		defer rows.Close()

		guests := []models.Guest{}
		for rows.Next() {
			g, err := scanGuest(rows)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to read guest row")
				return
			}
			guests = append(guests, g)
		}
		writeJSON(w, http.StatusOK, guests)
	}
}

type guestRowScanner interface {
	Scan(dest ...any) error
}

func scanGuest(row guestRowScanner) (models.Guest, error) {
	var g models.Guest
	var respondedAt sql.NullTime
	err := row.Scan(&g.ID, &g.WeddingID, &g.InviteCode, &g.Name, &g.GroupName, &g.MaxGuests,
		&g.RSVPStatus, &g.AttendingCount, &g.RSVPMessage, &respondedAt, &g.CreatedAt)
	if respondedAt.Valid {
		g.RSVPRespondedAt = &respondedAt.Time
	}
	return g, err
}

type createGuestRequest struct {
	Name      string `json:"name"`
	GroupName string `json:"group_name,omitempty"`
	MaxGuests int    `json:"max_guests"`
}

// CreateGuest adds a new invitee (or household/group) and generates a
// unique invite code for them. Admin-only.
func CreateGuest(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, err := getSingleWedding(db)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusPreconditionFailed, "create the wedding record before adding guests")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load wedding")
			return
		}

		var req createGuestRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if req.MaxGuests <= 0 {
			req.MaxGuests = 1
		}

		var id, code string
		for attempt := 0; attempt < 5; attempt++ {
			code, err = generateInviteCode(7)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to generate invite code")
				return
			}
			err = db.QueryRow(`
				INSERT INTO guests (wedding_id, invite_code, name, group_name, max_guests)
				VALUES ($1, $2, $3, $4, $5)
				RETURNING id
			`, wedding.ID, code, req.Name, req.GroupName, req.MaxGuests).Scan(&id)
			if err == nil {
				break
			}
			// retry on invite_code collision, otherwise bail out
			if !isUniqueViolation(err) {
				writeError(w, http.StatusInternalServerError, "failed to create guest: "+err.Error())
				return
			}
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to generate a unique invite code, try again")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"id": id, "invite_code": code})
	}
}

type updateGuestRequest struct {
	Name      string `json:"name"`
	GroupName string `json:"group_name,omitempty"`
	MaxGuests int    `json:"max_guests"`
}

// UpdateGuest edits a guest's name/group/allowance. Admin-only.
func UpdateGuest(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]

		var req updateGuestRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if req.MaxGuests <= 0 {
			req.MaxGuests = 1
		}

		res, err := db.Exec(`
			UPDATE guests SET name = $1, group_name = $2, max_guests = $3
			WHERE id = $4
		`, req.Name, req.GroupName, req.MaxGuests, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to update guest")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "guest not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

// DeleteGuest removes a guest record. Admin-only.
func DeleteGuest(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := mux.Vars(r)["id"]
		res, err := db.Exec(`DELETE FROM guests WHERE id = $1`, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to delete guest")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeError(w, http.StatusNotFound, "guest not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// GetRSVPSummary returns aggregate RSVP counts for the admin dashboard.
func GetRSVPSummary(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, err := getSingleWedding(db)
		if err == sql.ErrNoRows {
			writeJSON(w, http.StatusOK, models.RSVPSummary{})
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load wedding")
			return
		}

		var summary models.RSVPSummary
		err = db.QueryRow(`
			SELECT
				COUNT(*),
				COALESCE(SUM(max_guests), 0),
				COALESCE(SUM(CASE WHEN rsvp_status = 'pending' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN rsvp_status = 'attending' THEN 1 ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN rsvp_status = 'attending' THEN attending_count ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN rsvp_status = 'declined' THEN 1 ELSE 0 END), 0)
			FROM guests
			WHERE wedding_id = $1
		`, wedding.ID).Scan(
			&summary.TotalGuestRecords,
			&summary.TotalInvitedPeople,
			&summary.Pending,
			&summary.Attending,
			&summary.AttendingPeople,
			&summary.Declined,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to compute summary")
			return
		}
		writeJSON(w, http.StatusOK, summary)
	}
}

func isUniqueViolation(err error) bool {
	// lib/pq wraps the error in *pq.Error with Code 23505 for unique_violation.
	type pqErrorCoder interface{ SQLState() string }
	if pe, ok := err.(pqErrorCoder); ok {
		return pe.SQLState() == "23505"
	}
	return false
}
