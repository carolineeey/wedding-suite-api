package usecase

import (
	"context"
	"strings"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

const (
	defaultWishLimit = 50
	maxWishLimit     = 200
)

// WishStore is the guestbook storage WishUsecase needs. Every method is
// scoped by wedding so one wedding's admin can never reach another's rows.
type WishStore interface {
	List(ctx context.Context, weddingID string, includeUnapproved bool, limit int) ([]models.Wish, error)
	Create(ctx context.Context, w models.Wish) (string, error)
	SetApproval(ctx context.Context, weddingID, id string, approved bool) error
	Delete(ctx context.Context, weddingID, id string) error
}

type WishUsecase struct {
	wishes          WishStore
	requireApproval bool
}

// NewWishUsecase builds the guestbook usecase. When requireApproval is set,
// new messages stay hidden from the public list until an admin approves them.
func NewWishUsecase(wishes WishStore, requireApproval bool) *WishUsecase {
	return &WishUsecase{wishes: wishes, requireApproval: requireApproval}
}

// List returns guestbook messages, newest first. includeUnapproved is for
// the admin dashboard. A limit outside 1..200 falls back to 50.
func (u *WishUsecase) List(ctx context.Context, weddingID string, includeUnapproved bool, limit int) ([]models.Wish, error) {
	if err := requireWedding(weddingID); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > maxWishLimit {
		limit = defaultWishLimit
	}
	return u.wishes.List(ctx, weddingID, includeUnapproved, limit)
}

type CreateWishInput struct {
	GuestName string
	Message   string
}

// Create saves a guestbook message.
func (u *WishUsecase) Create(ctx context.Context, weddingID string, in CreateWishInput) (models.Wish, error) {
	if err := requireWedding(weddingID); err != nil {
		return models.Wish{}, err
	}

	wish := models.Wish{
		WeddingID:  weddingID,
		GuestName:  strings.TrimSpace(in.GuestName),
		Message:    strings.TrimSpace(in.Message),
		IsApproved: !u.requireApproval,
	}
	if wish.GuestName == "" || wish.Message == "" {
		return models.Wish{}, invalid("guest_name and message are required")
	}
	if tooLong(wish.GuestName, maxNameChars) {
		return models.Wish{}, invalid("guest_name is too long (max %d characters)", maxNameChars)
	}
	if tooLong(wish.Message, maxMessageChars) {
		return models.Wish{}, invalid("message is too long (max %d characters)", maxMessageChars)
	}

	id, err := u.wishes.Create(ctx, wish)
	if err != nil {
		return models.Wish{}, err
	}
	wish.ID = id
	return wish, nil
}

// SetApproval hides or shows a message without deleting it.
func (u *WishUsecase) SetApproval(ctx context.Context, weddingID, id string, approved bool) error {
	if err := requireWedding(weddingID); err != nil {
		return err
	}
	return u.wishes.SetApproval(ctx, weddingID, id, approved)
}

func (u *WishUsecase) Delete(ctx context.Context, weddingID, id string) error {
	if err := requireWedding(weddingID); err != nil {
		return err
	}
	return u.wishes.Delete(ctx, weddingID, id)
}
