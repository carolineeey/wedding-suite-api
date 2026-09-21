package repository

import (
	"context"
	"database/sql"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

type WishRepository struct {
	db *sql.DB
}

func NewWishRepository(db *sql.DB) *WishRepository {
	return &WishRepository{db: db}
}

// List returns up to limit wishes for the wedding, newest first.
func (r *WishRepository) List(ctx context.Context, weddingID string, includeUnapproved bool, limit int) ([]models.Wish, error) {
	query := `
		SELECT id, wedding_id, guest_name, message, is_approved, created_at
		FROM wishes
		WHERE wedding_id = $1
	`
	if !includeUnapproved {
		query += ` AND is_approved = true `
	}
	query += ` ORDER BY created_at DESC LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, weddingID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	wishes := []models.Wish{}
	for rows.Next() {
		var wi models.Wish
		if err := rows.Scan(&wi.ID, &wi.WeddingID, &wi.GuestName, &wi.Message, &wi.IsApproved, &wi.CreatedAt); err != nil {
			return nil, err
		}
		wishes = append(wishes, wi)
	}
	return wishes, rows.Err()
}

// Create inserts a wish and returns its generated ID.
func (r *WishRepository) Create(ctx context.Context, w models.Wish) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO wishes (wedding_id, guest_name, message, is_approved)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, w.WeddingID, w.GuestName, w.Message, w.IsApproved).Scan(&id)
	return id, err
}

// SetApproval hides or shows a wish. The wedding_id predicate keeps the
// update inside the caller's wedding, so an id from another wedding reports
// ErrNotFound instead of moderating someone else's guestbook.
func (r *WishRepository) SetApproval(ctx context.Context, weddingID, id string, approved bool) error {
	return requireAffected(r.db.ExecContext(ctx,
		`UPDATE wishes SET is_approved = $1 WHERE id = $2 AND wedding_id = $3`, approved, id, weddingID))
}

func (r *WishRepository) Delete(ctx context.Context, weddingID, id string) error {
	return requireAffected(r.db.ExecContext(ctx,
		`DELETE FROM wishes WHERE id = $1 AND wedding_id = $2`, id, weddingID))
}
