package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

// WeddingStore is the wedding storage WeddingUsecase needs.
type WeddingStore interface {
	WeddingFinder
	// SlugTaken reports whether a wedding other than excludeID already uses
	// this slug. excludeID is "" when creating.
	SlugTaken(ctx context.Context, slug, excludeID string) (bool, error)
	// Create inserts the wedding and grants ownerID access to it, together,
	// so a wedding never exists without an admin who can manage it.
	Create(ctx context.Context, w models.Wedding, ownerID string) error
	Update(ctx context.Context, w models.Wedding) error
}

// EventLister lists a wedding's schedule.
type EventLister interface {
	ListByWedding(ctx context.Context, weddingID string) ([]models.Event, error)
}

type WeddingUsecase struct {
	weddings WeddingStore
	events   EventLister
}

func NewWeddingUsecase(weddings WeddingStore, events EventLister) *WeddingUsecase {
	return &WeddingUsecase{weddings: weddings, events: events}
}

// Get returns the wedding named by the slug and its full event schedule.
// An unknown slug is models.ErrNotFound.
func (u *WeddingUsecase) Get(ctx context.Context, slug string) (models.Wedding, []models.Event, error) {
	wedding, err := u.weddings.BySlug(ctx, normalizeSlug(slug))
	if err != nil {
		return models.Wedding{}, nil, err
	}
	events, err := u.events.ListByWedding(ctx, wedding.ID)
	if err != nil {
		return models.Wedding{}, nil, fmt.Errorf("loading schedule: %w", err)
	}
	return wedding, events, nil
}

type WeddingInput struct {
	Slug           string
	PartnerOneName string
	PartnerTwoName string
	WeddingDate    string // YYYY-MM-DD, optional
}

// validate normalizes the slug and checks every field, returning the wedding
// the caller described (without an ID).
func (in WeddingInput) validate() (models.Wedding, error) {
	slug := normalizeSlug(in.Slug)
	if slug == "" || in.PartnerOneName == "" || in.PartnerTwoName == "" {
		return models.Wedding{}, invalid("slug, partner_one_name, and partner_two_name are required")
	}
	if tooLong(slug, maxSlugChars) {
		return models.Wedding{}, invalid("slug must be at most %d characters", maxSlugChars)
	}
	if !slugPattern.MatchString(slug) {
		return models.Wedding{}, invalid("slug must contain only lowercase letters, numbers, and single hyphens between them")
	}
	wedding := models.Wedding{
		Slug:           slug,
		PartnerOneName: in.PartnerOneName,
		PartnerTwoName: in.PartnerTwoName,
	}
	if in.WeddingDate != "" {
		if _, err := time.Parse("2006-01-02", in.WeddingDate); err != nil {
			return models.Wedding{}, invalid("wedding_date must be in YYYY-MM-DD format")
		}
		wedding.WeddingDate = &in.WeddingDate
	}
	return wedding, nil
}

// Create registers a new wedding owned by the admin creating it. The slug
// becomes its address, so a slug another wedding already holds is rejected
// rather than merged into it.
func (u *WeddingUsecase) Create(ctx context.Context, ownerID string, in WeddingInput) (models.Wedding, error) {
	if ownerID == "" {
		return models.Wedding{}, ErrNoAdminInScope
	}
	wedding, err := in.validate()
	if err != nil {
		return models.Wedding{}, err
	}
	if err := u.checkSlugFree(ctx, wedding.Slug, ""); err != nil {
		return models.Wedding{}, err
	}

	if err := u.weddings.Create(ctx, wedding, ownerID); err != nil {
		return models.Wedding{}, u.slugConflict(err)
	}
	return u.weddings.BySlug(ctx, wedding.Slug)
}

// Update edits the wedding the slug names. The body may carry a different
// slug, which renames it; every link already handed to guests then breaks,
// so this is the caller's decision to make deliberately.
func (u *WeddingUsecase) Update(ctx context.Context, slug string, in WeddingInput) (models.Wedding, error) {
	existing, err := u.weddings.BySlug(ctx, normalizeSlug(slug))
	if err != nil {
		return models.Wedding{}, err
	}
	wedding, err := in.validate()
	if err != nil {
		return models.Wedding{}, err
	}
	if err := u.checkSlugFree(ctx, wedding.Slug, existing.ID); err != nil {
		return models.Wedding{}, err
	}

	wedding.ID = existing.ID
	if err := u.weddings.Update(ctx, wedding); err != nil {
		return models.Wedding{}, u.slugConflict(err)
	}
	return u.weddings.BySlug(ctx, wedding.Slug)
}

// checkSlugFree reports a slug held by a different wedding as a caller
// mistake rather than letting the write fail on the unique index, so the
// message names the field the caller has to change.
func (u *WeddingUsecase) checkSlugFree(ctx context.Context, slug, excludeID string) error {
	taken, err := u.weddings.SlugTaken(ctx, slug, excludeID)
	if err != nil {
		return fmt.Errorf("checking slug: %w", err)
	}
	if taken {
		return invalid("slug is already taken")
	}
	return nil
}

// slugConflict translates a duplicate from the store. slug is the only
// unique column on weddings, so a duplicate means a concurrent request took
// the slug between checkSlugFree and the write.
func (u *WeddingUsecase) slugConflict(err error) error {
	if errors.Is(err, models.ErrDuplicate) {
		return invalid("slug is already taken")
	}
	return err
}
