package handlers

import (
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/usecase"
)

// ListGuests returns every guest record for the wedding, including RSVP
// status. Admin-only.
func ListGuests(scope *usecase.WeddingScope, guests *usecase.GuestUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, ok := weddingID(w, r, scope, "failed to load guests")
		if !ok {
			return
		}
		list, err := guests.List(r.Context(), wedding)
		if err != nil {
			writeUsecaseError(w, r, err, "guest not found", "failed to load guests")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

type guestRequest struct {
	Name      string `json:"name"`
	GroupName string `json:"group_name,omitempty"`
	MaxGuests int    `json:"max_guests"`
}

func (req guestRequest) input() usecase.GuestInput {
	return usecase.GuestInput{Name: req.Name, GroupName: req.GroupName, MaxGuests: req.MaxGuests}
}

// CreateGuest adds a new invitee (or household/group) and generates a
// unique invite code for them. Admin-only.
func CreateGuest(scope *usecase.WeddingScope, guests *usecase.GuestUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req guestRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to create guest")
		if !ok {
			return
		}

		guest, err := guests.Create(r.Context(), wedding, req.input())
		if err != nil {
			writeUsecaseError(w, r, err, "guest not found", "failed to create guest")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{"id": guest.ID, "invite_code": guest.InviteCode})
	}
}

// UpdateGuest edits a guest's name/group/allowance. Admin-only.
func UpdateGuest(scope *usecase.WeddingScope, guests *usecase.GuestUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r, "guest not found")
		if !ok {
			return
		}

		var req guestRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to update guest")
		if !ok {
			return
		}

		if err := guests.Update(r.Context(), wedding, id, req.input()); err != nil {
			writeUsecaseError(w, r, err, "guest not found", "failed to update guest")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

// DeleteGuest removes a guest record. Admin-only.
func DeleteGuest(scope *usecase.WeddingScope, guests *usecase.GuestUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r, "guest not found")
		if !ok {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to delete guest")
		if !ok {
			return
		}
		if err := guests.Delete(r.Context(), wedding, id); err != nil {
			writeUsecaseError(w, r, err, "guest not found", "failed to delete guest")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}

// GetRSVPSummary returns aggregate RSVP counts for the admin dashboard.
func GetRSVPSummary(scope *usecase.WeddingScope, guests *usecase.GuestUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, ok := weddingID(w, r, scope, "failed to compute summary")
		if !ok {
			return
		}
		summary, err := guests.Summary(r.Context(), wedding)
		if err != nil {
			writeUsecaseError(w, r, err, "wedding not found", "failed to compute summary")
			return
		}
		writeJSON(w, http.StatusOK, summary)
	}
}
