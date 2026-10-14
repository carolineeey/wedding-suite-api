package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

// GuestFinder looks a guest up by invite code.
type GuestFinder interface {
	GetByInviteCode(ctx context.Context, code string) (models.Guest, error)
}

// WeddingByID loads a wedding by ID.
type WeddingByID interface {
	ByID(ctx context.Context, id string) (models.Wedding, error)
}

// GiftLister lists a wedding's gift accounts.
type GiftLister interface {
	ListByWedding(ctx context.Context, weddingID string) ([]models.GiftAccount, error)
}

// InvitationStores is the storage InvitationUsecase reads from.
type InvitationStores struct {
	Guests   GuestFinder
	Weddings WeddingByID
	Events   EventLister
	Gifts    GiftLister
}

// InvitationUsecase assembles the page a guest opens from their personal
// link. The invite code alone identifies the guest and, through them, the
// wedding, so the link needs no slug.
type InvitationUsecase struct {
	stores InvitationStores
}

func NewInvitationUsecase(stores InvitationStores) *InvitationUsecase {
	return &InvitationUsecase{stores: stores}
}

// Invitation is everything the invitation page shows one guest. Gift
// accounts appear only here: an invite code is required to see them.
type Invitation struct {
	Guest   models.Guest         `json:"guest"`
	Wedding models.Wedding       `json:"wedding"`
	Events  []models.Event       `json:"events"`
	Gifts   []models.GiftAccount `json:"gifts"`
}

// Get loads the invitation for an invite code, in any case. An unknown code
// is models.ErrNotFound.
func (u *InvitationUsecase) Get(ctx context.Context, code string) (Invitation, error) {
	guest, err := u.stores.Guests.GetByInviteCode(ctx, normalizeInviteCode(code))
	if err != nil {
		return Invitation{}, err
	}
	wedding, err := u.stores.Weddings.ByID(ctx, guest.WeddingID)
	if errors.Is(err, models.ErrNotFound) {
		// guests.wedding_id is a cascading foreign key, so a guest always has
		// a wedding; a miss here is a real failure, not a 404. %v drops the
		// ErrNotFound so the handler answers 500.
		return Invitation{}, fmt.Errorf("guest %s has no wedding: %v", guest.ID, err)
	}
	if err != nil {
		return Invitation{}, fmt.Errorf("loading wedding: %w", err)
	}
	events, err := u.stores.Events.ListByWedding(ctx, wedding.ID)
	if err != nil {
		return Invitation{}, fmt.Errorf("loading schedule: %w", err)
	}
	gifts, err := u.stores.Gifts.ListByWedding(ctx, wedding.ID)
	if err != nil {
		return Invitation{}, fmt.Errorf("loading gift accounts: %w", err)
	}
	return Invitation{Guest: guest, Wedding: wedding, Events: events, Gifts: gifts}, nil
}
