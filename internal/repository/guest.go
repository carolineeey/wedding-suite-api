package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/errtrace"
	"github.com/carolineeey/wedding-suite-api/internal/models"
)

const guestColumns = `
	id, wedding_id, invite_code, name, COALESCE(group_name, ''), max_guests,
	rsvp_status, attending_count, COALESCE(rsvp_message, ''), rsvp_responded_at, created_at
`

type GuestRepository struct {
	db *sql.DB
}

func NewGuestRepository(db *sql.DB) *GuestRepository {
	return &GuestRepository{db: db}
}

func scanGuest(row rowScanner) (models.Guest, error) {
	var g models.Guest
	var respondedAt sql.NullTime
	if err := row.Scan(&g.ID, &g.WeddingID, &g.InviteCode, &g.Name, &g.GroupName, &g.MaxGuests,
		&g.RSVPStatus, &g.AttendingCount, &g.RSVPMessage, &respondedAt, &g.CreatedAt); err != nil {
		return g, err
	}
	if respondedAt.Valid {
		g.RSVPRespondedAt = &respondedAt.Time
	}
	return g, nil
}

func (r *GuestRepository) ListByWedding(ctx context.Context, weddingID string) ([]models.Guest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+guestColumns+`
		FROM guests
		WHERE wedding_id = $1
		ORDER BY created_at ASC
	`, weddingID)
	if err != nil {
		return nil, errtrace.Wrap(err)
	}
	defer rows.Close()

	guests := []models.Guest{}
	for rows.Next() {
		g, err := scanGuest(rows)
		if err != nil {
			return nil, errtrace.Wrap(err)
		}
		guests = append(guests, g)
	}
	return guests, errtrace.Wrap(rows.Err())
}

func (r *GuestRepository) GetByInviteCode(ctx context.Context, code string) (models.Guest, error) {
	g, err := scanGuest(r.db.QueryRowContext(ctx, `
		SELECT `+guestColumns+`
		FROM guests
		WHERE invite_code = $1
	`, code))
	if errors.Is(err, sql.ErrNoRows) {
		return g, models.ErrNotFound
	}
	return g, errtrace.Wrap(err)
}

// Create inserts a guest and returns its generated ID. It returns
// models.ErrDuplicate if the invite code is already taken.
func (r *GuestRepository) Create(ctx context.Context, g models.Guest) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO guests (wedding_id, invite_code, name, group_name, max_guests)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, g.WeddingID, g.InviteCode, g.Name, g.GroupName, g.MaxGuests).Scan(&id)
	if isUniqueViolation(err) {
		return "", models.ErrDuplicate
	}
	return id, errtrace.Wrap(err)
}

// Update saves a guest's name, group, and allowance. The wedding_id
// predicate keeps the update inside the caller's wedding, so an id from
// another wedding reports ErrNotFound instead of editing someone else's row.
func (r *GuestRepository) Update(ctx context.Context, g models.Guest) error {
	return requireAffected(r.db.ExecContext(ctx, `
		UPDATE guests SET name = $1, group_name = $2, max_guests = $3
		WHERE id = $4 AND wedding_id = $5
	`, g.Name, g.GroupName, g.MaxGuests, g.ID, g.WeddingID))
}

func (r *GuestRepository) Delete(ctx context.Context, weddingID, id string) error {
	return requireAffected(r.db.ExecContext(ctx,
		`DELETE FROM guests WHERE id = $1 AND wedding_id = $2`, id, weddingID))
}

// SaveRSVP records a guest's response. It needs no wedding_id predicate: the
// id always comes from a GetByInviteCode lookup, so it already belongs to the
// wedding that owns the code.
func (r *GuestRepository) SaveRSVP(ctx context.Context, id string, status models.RSVPStatus, attendingCount int, message string, respondedAt time.Time) error {
	return requireAffected(r.db.ExecContext(ctx, `
		UPDATE guests
		SET rsvp_status = $1, attending_count = $2, rsvp_message = $3, rsvp_responded_at = $4
		WHERE id = $5
	`, status, attendingCount, message, respondedAt, id))
}

func (r *GuestRepository) Summary(ctx context.Context, weddingID string) (models.RSVPSummary, error) {
	var s models.RSVPSummary
	err := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(max_guests), 0),
			COALESCE(SUM(CASE WHEN rsvp_status = 'pending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rsvp_status = 'attending' THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rsvp_status = 'attending' THEN attending_count ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN rsvp_status = 'declined' THEN 1 ELSE 0 END), 0)
		FROM guests
		WHERE wedding_id = $1
	`, weddingID).Scan(
		&s.TotalGuestRecords,
		&s.TotalInvitedPeople,
		&s.Pending,
		&s.Attending,
		&s.AttendingPeople,
		&s.Declined,
	)
	return s, errtrace.Wrap(err)
}
