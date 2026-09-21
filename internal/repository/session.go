package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

// SessionRepository stores admin login sessions. Tokens arrive already
// hashed; the raw token never reaches the database.
type SessionRepository struct {
	db *sql.DB
}

func NewSessionRepository(db *sql.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

func (r *SessionRepository) Create(ctx context.Context, tokenHash, adminID string, expiresAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO admin_sessions (token_hash, admin_id, expires_at) VALUES ($1, $2, $3)
	`, tokenHash, adminID, expiresAt)
	return err
}

// AdminByToken returns the admin whose session has this token hash and is
// still valid at now. now comes from the caller rather than SQL now() so
// expiry follows the same clock as the rest of the usecase.
func (r *SessionRepository) AdminByToken(ctx context.Context, tokenHash string, now time.Time) (models.Admin, error) {
	var a models.Admin
	err := r.db.QueryRowContext(ctx, `
		SELECT a.id, a.email, a.created_at
		FROM admin_sessions s
		JOIN admins a ON a.id = s.admin_id
		WHERE s.token_hash = $1 AND s.expires_at > $2
	`, tokenHash, now).Scan(&a.ID, &a.Email, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, models.ErrNotFound
	}
	return a, err
}

func (r *SessionRepository) Delete(ctx context.Context, tokenHash string) error {
	return requireAffected(r.db.ExecContext(ctx, `
		DELETE FROM admin_sessions WHERE token_hash = $1
	`, tokenHash))
}

func (r *SessionRepository) DeleteForAdmin(ctx context.Context, adminID string) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM admin_sessions WHERE admin_id = $1
	`, adminID)
	return err
}

func (r *SessionRepository) DeleteExpired(ctx context.Context, adminID string, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM admin_sessions WHERE admin_id = $1 AND expires_at <= $2
	`, adminID, now)
	return err
}
