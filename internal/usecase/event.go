package usecase

import (
	"context"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

// EventStore is the event storage EventUsecase needs. Delete is scoped by
// wedding so one wedding's admin can never reach another's rows.
type EventStore interface {
	Create(ctx context.Context, e models.Event) (string, error)
	Delete(ctx context.Context, weddingID, id string) error
}

type EventUsecase struct {
	events EventStore
}

func NewEventUsecase(events EventStore) *EventUsecase {
	return &EventUsecase{events: events}
}

type CreateEventInput struct {
	Name      string
	StartsAt  string // RFC3339
	EndsAt    string // RFC3339, optional
	VenueName string
	Address   string
	Notes     string
	MapsURL   string // optional, http(s) only
	SortOrder int
}

// Create adds an item to the wedding-day schedule and returns its ID.
func (u *EventUsecase) Create(ctx context.Context, weddingID string, in CreateEventInput) (string, error) {
	if err := requireWedding(weddingID); err != nil {
		return "", err
	}
	if in.Name == "" || in.StartsAt == "" {
		return "", invalid("name and starts_at are required")
	}
	if in.MapsURL != "" && (tooLong(in.MapsURL, maxURLChars) || !isWebURL(in.MapsURL)) {
		return "", invalid("maps_url must be an http(s) link, e.g. https://maps.app.goo.gl/...")
	}
	startsAt, err := time.Parse(time.RFC3339, in.StartsAt)
	if err != nil {
		return "", invalid("starts_at must be RFC3339, e.g. 2027-06-12T09:00:00+07:00")
	}
	event := models.Event{
		WeddingID: weddingID,
		Name:      in.Name,
		StartsAt:  startsAt,
		VenueName: in.VenueName,
		Address:   in.Address,
		Notes:     in.Notes,
		MapsURL:   in.MapsURL,
		SortOrder: in.SortOrder,
	}
	if in.EndsAt != "" {
		endsAt, err := time.Parse(time.RFC3339, in.EndsAt)
		if err != nil {
			return "", invalid("ends_at must be RFC3339")
		}
		event.EndsAt = &endsAt
	}
	return u.events.Create(ctx, event)
}

func (u *EventUsecase) Delete(ctx context.Context, weddingID, id string) error {
	if err := requireWedding(weddingID); err != nil {
		return err
	}
	return u.events.Delete(ctx, weddingID, id)
}
