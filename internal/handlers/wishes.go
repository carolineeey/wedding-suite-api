package handlers

import (
	"net/http"
	"strconv"

	"github.com/carolineeey/wedding-suite-api/internal/usecase"
)

// ListWishes returns guestbook messages, newest first (?limit=, default 50,
// max 200). The public route lists approved messages only; the admin route
// sets includeUnapproved.
func ListWishes(scope *usecase.WeddingScope, wishes *usecase.WishUsecase, includeUnapproved bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// An unparseable limit is 0, which the usecase replaces with the default.
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))

		wedding, ok := weddingID(w, r, scope, "failed to load wishes")
		if !ok {
			return
		}
		list, err := wishes.List(r.Context(), wedding, includeUnapproved, limit)
		if err != nil {
			writeUsecaseError(w, err, "wish not found", "failed to load wishes")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

type createWishRequest struct {
	GuestName string `json:"guest_name"`
	Message   string `json:"message"`
}

// CreateWish lets a guest leave a guestbook message. Public.
func CreateWish(scope *usecase.WeddingScope, wishes *usecase.WishUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createWishRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to save wish")
		if !ok {
			return
		}

		wish, err := wishes.Create(r.Context(), wedding, usecase.CreateWishInput{
			GuestName: req.GuestName,
			Message:   req.Message,
		})
		if err != nil {
			writeUsecaseError(w, err, "wish not found", "failed to save wish")
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{"id": wish.ID, "is_approved": wish.IsApproved})
	}
}

// SetWishApproval lets an admin hide/show a guestbook message without
// deleting it (moderation for spam or anything inappropriate).
func SetWishApproval(scope *usecase.WeddingScope, wishes *usecase.WishUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r, "wish not found")
		if !ok {
			return
		}

		var req struct {
			IsApproved bool `json:"is_approved"`
		}
		if !decodeJSON(w, r, &req) {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to update wish")
		if !ok {
			return
		}

		if err := wishes.SetApproval(r.Context(), wedding, id, req.IsApproved); err != nil {
			writeUsecaseError(w, err, "wish not found", "failed to update wish")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

// DeleteWish permanently removes a guestbook message. Admin-only.
func DeleteWish(scope *usecase.WeddingScope, wishes *usecase.WishUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r, "wish not found")
		if !ok {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to delete wish")
		if !ok {
			return
		}
		if err := wishes.Delete(r.Context(), wedding, id); err != nil {
			writeUsecaseError(w, err, "wish not found", "failed to delete wish")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
