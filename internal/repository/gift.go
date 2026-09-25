package repository

import (
	"context"
	"database/sql"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

type GiftRepository struct {
	db *sql.DB
}

func NewGiftRepository(db *sql.DB) *GiftRepository {
	return &GiftRepository{db: db}
}

func (r *GiftRepository) ListByWedding(ctx context.Context, weddingID string) ([]models.GiftAccount, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, wedding_id, bank_name, account_name, account_number, sort_order
		FROM gift_accounts
		WHERE wedding_id = $1
		ORDER BY sort_order ASC, created_at ASC
	`, weddingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	gifts := []models.GiftAccount{}
	for rows.Next() {
		var g models.GiftAccount
		if err := rows.Scan(&g.ID, &g.WeddingID, &g.BankName, &g.AccountName,
			&g.AccountNumber, &g.SortOrder); err != nil {
			return nil, err
		}
		gifts = append(gifts, g)
	}
	return gifts, rows.Err()
}

// Create inserts a gift account and returns its generated ID.
func (r *GiftRepository) Create(ctx context.Context, g models.GiftAccount) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO gift_accounts (wedding_id, bank_name, account_name, account_number, sort_order)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, g.WeddingID, g.BankName, g.AccountName, g.AccountNumber, g.SortOrder).Scan(&id)
	return id, err
}

// Delete removes a gift account, scoped to the caller's wedding like
// EventRepository.Delete.
func (r *GiftRepository) Delete(ctx context.Context, weddingID, id string) error {
	return requireAffected(r.db.ExecContext(ctx,
		`DELETE FROM gift_accounts WHERE id = $1 AND wedding_id = $2`, id, weddingID))
}
