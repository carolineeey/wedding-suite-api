package usecase

import (
	"context"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

// GiftStore is the gift account storage GiftUsecase needs. Every method is
// scoped by wedding so one wedding's admin can never reach another's rows.
type GiftStore interface {
	ListByWedding(ctx context.Context, weddingID string) ([]models.GiftAccount, error)
	Create(ctx context.Context, g models.GiftAccount) (string, error)
	Delete(ctx context.Context, weddingID, id string) error
}

type GiftUsecase struct {
	gifts GiftStore
}

func NewGiftUsecase(gifts GiftStore) *GiftUsecase {
	return &GiftUsecase{gifts: gifts}
}

func (u *GiftUsecase) List(ctx context.Context, weddingID string) ([]models.GiftAccount, error) {
	if err := requireWedding(weddingID); err != nil {
		return nil, err
	}
	return u.gifts.ListByWedding(ctx, weddingID)
}

type CreateGiftInput struct {
	BankName      string
	AccountName   string
	AccountNumber string
	SortOrder     int
}

// Create adds an account guests can send a digital gift to and returns its ID.
func (u *GiftUsecase) Create(ctx context.Context, weddingID string, in CreateGiftInput) (string, error) {
	if err := requireWedding(weddingID); err != nil {
		return "", err
	}
	if in.BankName == "" || in.AccountName == "" || in.AccountNumber == "" {
		return "", invalid("bank_name, account_name, and account_number are required")
	}
	if tooLong(in.BankName, maxNameChars) || tooLong(in.AccountName, maxNameChars) ||
		tooLong(in.AccountNumber, maxNameChars) {
		return "", invalid("gift account fields must be at most %d characters", maxNameChars)
	}
	return u.gifts.Create(ctx, models.GiftAccount{
		WeddingID:     weddingID,
		BankName:      in.BankName,
		AccountName:   in.AccountName,
		AccountNumber: in.AccountNumber,
		SortOrder:     in.SortOrder,
	})
}

func (u *GiftUsecase) Delete(ctx context.Context, weddingID, id string) error {
	if err := requireWedding(weddingID); err != nil {
		return err
	}
	return u.gifts.Delete(ctx, weddingID, id)
}
