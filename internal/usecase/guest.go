package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/carolineeey/wedding-suite-api/internal/errtrace"
	"github.com/carolineeey/wedding-suite-api/internal/models"
)

const inviteCodeAttempts = 5

// GuestStore is the guest storage GuestUsecase needs to manage the guest list.
// Every method is scoped by wedding so one wedding's admin can never reach
// another's rows.
type GuestStore interface {
	ListByWedding(ctx context.Context, weddingID string) ([]models.Guest, error)
	Create(ctx context.Context, g models.Guest) (string, error)
	Update(ctx context.Context, g models.Guest) error
	Delete(ctx context.Context, weddingID, id string) error
	Summary(ctx context.Context, weddingID string) (models.RSVPSummary, error)
}

type GuestUsecase struct {
	guests        GuestStore
	newInviteCode func() (string, error)
}

func NewGuestUsecase(guests GuestStore) *GuestUsecase {
	return &GuestUsecase{guests: guests, newInviteCode: generateInviteCode}
}

// List returns every guest record for the wedding, including RSVP status.
func (u *GuestUsecase) List(ctx context.Context, weddingID string) ([]models.Guest, error) {
	if err := requireWedding(weddingID); err != nil {
		return nil, err
	}
	return u.guests.ListByWedding(ctx, weddingID)
}

type GuestInput struct {
	Name      string
	GroupName string
	MaxGuests int // defaults to 1
}

func (in GuestInput) validate() (GuestInput, error) {
	if in.Name == "" {
		return in, invalid("name is required")
	}
	if in.MaxGuests <= 0 {
		in.MaxGuests = 1
	}
	return in, nil
}

// Create adds an invitee (or household/group) with a freshly generated
// invite code, retrying on the rare code collision.
func (u *GuestUsecase) Create(ctx context.Context, weddingID string, in GuestInput) (models.Guest, error) {
	if err := requireWedding(weddingID); err != nil {
		return models.Guest{}, err
	}
	in, err := in.validate()
	if err != nil {
		return models.Guest{}, err
	}

	guest := models.Guest{
		WeddingID: weddingID,
		Name:      in.Name,
		GroupName: in.GroupName,
		MaxGuests: in.MaxGuests,
	}
	for attempt := 0; attempt < inviteCodeAttempts; attempt++ {
		if guest.InviteCode, err = u.newInviteCode(); err != nil {
			return models.Guest{}, errtrace.Wrap(fmt.Errorf("generating invite code: %w", err))
		}
		guest.ID, err = u.guests.Create(ctx, guest)
		if !errors.Is(err, models.ErrDuplicate) {
			break
		}
	}
	if errors.Is(err, models.ErrDuplicate) {
		return models.Guest{}, fmt.Errorf("no unique invite code after %d attempts: %w", inviteCodeAttempts, err)
	}
	if err != nil {
		return models.Guest{}, err
	}
	return guest, nil
}

// Update edits a guest's name, group, and allowance.
func (u *GuestUsecase) Update(ctx context.Context, weddingID, id string, in GuestInput) error {
	if err := requireWedding(weddingID); err != nil {
		return err
	}
	in, err := in.validate()
	if err != nil {
		return err
	}
	return u.guests.Update(ctx, models.Guest{
		ID:        id,
		WeddingID: weddingID,
		Name:      in.Name,
		GroupName: in.GroupName,
		MaxGuests: in.MaxGuests,
	})
}

func (u *GuestUsecase) Delete(ctx context.Context, weddingID, id string) error {
	if err := requireWedding(weddingID); err != nil {
		return err
	}
	return u.guests.Delete(ctx, weddingID, id)
}

// Summary returns aggregate RSVP counts for the admin dashboard.
func (u *GuestUsecase) Summary(ctx context.Context, weddingID string) (models.RSVPSummary, error) {
	if err := requireWedding(weddingID); err != nil {
		return models.RSVPSummary{}, err
	}
	return u.guests.Summary(ctx, weddingID)
}
