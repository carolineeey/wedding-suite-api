package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

// getSingleWedding fetches the one wedding row that exists today. When a
// second wedding is added later, this is the only place that needs to
// change (e.g. to look the row up by slug/subdomain instead).
func getSingleWedding(db *sql.DB) (models.Wedding, error) {
	var w models.Wedding
	var weddingDate sql.NullString
	err := db.QueryRow(`
		SELECT id, slug, partner_one_name, partner_two_name, wedding_date, created_at
		FROM weddings
		ORDER BY created_at ASC
		LIMIT 1
	`).Scan(&w.ID, &w.Slug, &w.PartnerOneName, &w.PartnerTwoName, &weddingDate, &w.CreatedAt)
	if err != nil {
		return w, err
	}
	if weddingDate.Valid {
		w.WeddingDate = &weddingDate.String
	}
	return w, nil
}

func getEventsForWedding(db *sql.DB, weddingID string) ([]models.Event, error) {
	rows, err := db.Query(`
		SELECT id, wedding_id, name, starts_at, ends_at,
		       COALESCE(venue_name, ''), COALESCE(address, ''), COALESCE(notes, ''), sort_order
		FROM events
		WHERE wedding_id = $1
		ORDER BY sort_order ASC, starts_at ASC
	`, weddingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := []models.Event{}
	for rows.Next() {
		var e models.Event
		var endsAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.WeddingID, &e.Name, &e.StartsAt, &endsAt,
			&e.VenueName, &e.Address, &e.Notes, &e.SortOrder); err != nil {
			return nil, err
		}
		if endsAt.Valid {
			e.EndsAt = &endsAt.Time
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// GetWedding is the public endpoint the wedding website loads on first
// render: couple names, date, and the full event schedule.
func GetWedding(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, err := getSingleWedding(db)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "wedding has not been configured yet")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load wedding")
			return
		}

		events, err := getEventsForWedding(db, wedding.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load schedule")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"wedding": wedding,
			"events":  events,
		})
	}
}

type upsertWeddingRequest struct {
	Slug           string `json:"slug"`
	PartnerOneName string `json:"partner_one_name"`
	PartnerTwoName string `json:"partner_two_name"`
	WeddingDate    string `json:"wedding_date"` // "2027-06-12", optional
}

// UpsertWedding creates the wedding row on first call and updates it on
// subsequent calls. Admin-only. There's only ever one row today.
func UpsertWedding(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req upsertWeddingRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if req.Slug == "" || req.PartnerOneName == "" || req.PartnerTwoName == "" {
			writeError(w, http.StatusBadRequest, "slug, partner_one_name, and partner_two_name are required")
			return
		}

		var weddingDate any
		if req.WeddingDate != "" {
			if _, err := time.Parse("2006-01-02", req.WeddingDate); err != nil {
				writeError(w, http.StatusBadRequest, "wedding_date must be in YYYY-MM-DD format")
				return
			}
			weddingDate = req.WeddingDate
		}

		existing, err := getSingleWedding(db)
		switch err {
		case sql.ErrNoRows:
			var id string
			insertErr := db.QueryRow(`
				INSERT INTO weddings (slug, partner_one_name, partner_two_name, wedding_date)
				VALUES ($1, $2, $3, $4)
				RETURNING id
			`, req.Slug, req.PartnerOneName, req.PartnerTwoName, weddingDate).Scan(&id)
			if insertErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to create wedding: "+insertErr.Error())
				return
			}
		case nil:
			_, updateErr := db.Exec(`
				UPDATE weddings
				SET slug = $1, partner_one_name = $2, partner_two_name = $3, wedding_date = $4
				WHERE id = $5
			`, req.Slug, req.PartnerOneName, req.PartnerTwoName, weddingDate, existing.ID)
			if updateErr != nil {
				writeError(w, http.StatusInternalServerError, "failed to update wedding: "+updateErr.Error())
				return
			}
		default:
			writeError(w, http.StatusInternalServerError, "failed to check existing wedding")
			return
		}

		wedding, err := getSingleWedding(db)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "wedding saved but failed to reload")
			return
		}
		writeJSON(w, http.StatusOK, wedding)
	}
}
