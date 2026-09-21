package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// Create inserts an admin and returns its ID. It returns models.ErrDuplicate
// if the email is already registered.
func (r *AdminRepository) Create(ctx context.Context, email, passwordHash string) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO admins (email, password_hash) VALUES ($1, $2)
		RETURNING id
	`, email, passwordHash).Scan(&id)
	if isUniqueViolation(err) {
		return "", models.ErrDuplicate
	}
	return id, err
}

// ByEmail returns the admin with this (already lowercased) email and its
// password hash.
func (r *AdminRepository) ByEmail(ctx context.Context, email string) (models.Admin, string, error) {
	var a models.Admin
	var hash string
	err := r.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, created_at
		FROM admins
		WHERE email = $1
	`, email).Scan(&a.ID, &a.Email, &hash, &a.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return a, "", models.ErrNotFound
	}
	return a, hash, err
}

func (r *AdminRepository) SetPassword(ctx context.Context, adminID, passwordHash string) error {
	return requireAffected(r.db.ExecContext(ctx, `
		UPDATE admins SET password_hash = $1 WHERE id = $2
	`, passwordHash, adminID))
}
