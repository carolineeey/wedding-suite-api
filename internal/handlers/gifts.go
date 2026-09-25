package handlers

import (
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/usecase"
)

// ListGifts returns the wedding's gift accounts. Admin-only; guests see them
// through GetInvitation.
func ListGifts(scope *usecase.WeddingScope, gifts *usecase.GiftUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, ok := weddingID(w, r, scope, "failed to load gift accounts")
		if !ok {
			return
		}
		list, err := gifts.List(r.Context(), wedding)
		if err != nil {
			writeUsecaseError(w, err, "gift account not found", "failed to load gift accounts")
			return
		}
		writeJSON(w, http.StatusOK, list)
	}
}

type createGiftRequest struct {
	BankName      string `json:"bank_name"`
	AccountName   string `json:"account_name"`
	AccountNumber string `json:"account_number"`
	SortOrder     int    `json:"sort_order,omitempty"`
}

// CreateGift adds an account guests can send a digital gift to. Admin-only.
func CreateGift(scope *usecase.WeddingScope, gifts *usecase.GiftUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createGiftRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to create gift account")
		if !ok {
			return
		}

		id, err := gifts.Create(r.Context(), wedding, usecase.CreateGiftInput{
			BankName:      req.BankName,
			AccountName:   req.AccountName,
			AccountNumber: req.AccountNumber,
			SortOrder:     req.SortOrder,
		})
		if err != nil {
			writeUsecaseError(w, err, "gift account not found", "failed to create gift account")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"id": id})
	}
}

// DeleteGift removes a gift account. Admin-only.
func DeleteGift(scope *usecase.WeddingScope, gifts *usecase.GiftUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, ok := pathUUID(w, r, "gift account not found")
		if !ok {
			return
		}
		wedding, ok := weddingID(w, r, scope, "failed to delete gift account")
		if !ok {
			return
		}
		if err := gifts.Delete(r.Context(), wedding, id); err != nil {
			writeUsecaseError(w, err, "gift account not found", "failed to delete gift account")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
	}
}
