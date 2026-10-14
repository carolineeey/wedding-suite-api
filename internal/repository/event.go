package repository

import (
	"context"
	"database/sql"

	"github.com/carolineeey/wedding-suite-api/internal/errtrace"
	"github.com/carolineeey/wedding-suite-api/internal/models"
)

type EventRepository struct {
	db *sql.DB
}

func NewEventRepository(db *sql.DB) *EventRepository {
	return &EventRepository{db: db}
}

func (r *EventRepository) ListByWedding(ctx context.Context, weddingID string) ([]models.Event, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, wedding_id, name, starts_at, ends_at,
		       COALESCE(venue_name, ''), COALESCE(address, ''), COALESCE(notes, ''),
		       COALESCE(maps_url, ''), sort_order
		FROM events
		WHERE wedding_id = $1
		ORDER BY sort_order ASC, starts_at ASC
	`, weddingID)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	defer rows.Close()

	events := []models.Event{}
	for rows.Next() {
		var e models.Event
		var endsAt sql.NullTime
		if err := rows.Scan(&e.ID, &e.WeddingID, &e.Name, &e.StartsAt, &endsAt,
			&e.VenueName, &e.Address, &e.Notes, &e.MapsURL, &e.SortOrder); err != nil {
			return nil, errtrace.Wrap(err)
		}
		if endsAt.Valid {
			e.EndsAt = &endsAt.Time
		}
		events = append(events, e)
	}
	return events, errtrace.Wrap(rows.Err())
}

// Create inserts an event and returns its generated ID.
func (r *EventRepository) Create(ctx context.Context, e models.Event) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO events (wedding_id, name, starts_at, ends_at, venue_name, address, notes, maps_url, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NULLIF($8, ''), $9)
		RETURNING id
	`, e.WeddingID, e.Name, e.StartsAt, e.EndsAt, e.VenueName, e.Address, e.Notes, e.MapsURL, e.SortOrder).Scan(&id)
	return id, errtrace.Wrap(err)
}

// Delete removes an event. The wedding_id predicate keeps the delete inside
// the caller's wedding, so an id from another wedding reports ErrNotFound
// instead of deleting someone else's row.
func (r *EventRepository) Delete(ctx context.Context, weddingID, id string) error {
	return requireAffected(r.db.ExecContext(ctx,
		`DELETE FROM events WHERE id = $1 AND wedding_id = $2`, id, weddingID))
}
