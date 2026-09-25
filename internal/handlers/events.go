package handlers

import (
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/usecase"
)

type createEventRequest struct {
	Name      string `json:"name"`
	StartsAt  string `json:"starts_at"` // RFC3339
	EndsAt    string `json:"ends_at,omitempty"`
	VenueName string `json:"venue_name,omitempty"`
	Address   string `json:"address,omitempty"`
	Notes     string `json:"notes,omitempty"`
	MapsURL   string `json:"maps_url,omitempty"`
	SortOrder int    `json:"sort_order,omitempty"`
}

// CreateEvent adds an item to the wedding-day schedule (ceremony, reception, etc).
// Admin-only.
func CreateEvent(scope *usecase.WeddingScope, events *usecase.EventUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createEventRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to create event")
		if !ok {
			return
		}

		id, err := events.Create(r.Context(), wedding, usecase.CreateEventInput{
			Name:      req.Name,
			StartsAt:  req.StartsAt,
			EndsAt:    req.EndsAt,
			VenueName: req.VenueName,
			Address:   req.Address,
			Notes:     req.Notes,
			MapsURL:   req.MapsURL,
			SortOrder: req.SortOrder,
		})
		if err != nil {
			writeUsecaseError(w, err, "event not found", "failed to create event")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

// DeleteEvent removes an item from the schedule. Admin-only.
func DeleteEvent(scope *usecase.WeddingScope, events *usecase.EventUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r, "event not found")
		if !ok {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to delete event")
		if !ok {
			return
		}
		if err := events.Delete(r.Context(), wedding, id); err != nil {
			writeUsecaseError(w, err, "event not found", "failed to delete event")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
