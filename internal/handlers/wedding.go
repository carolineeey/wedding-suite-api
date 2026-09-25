package handlers

import (
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/usecase"
	"github.com/gorilla/mux"
)

// GetWedding is the public endpoint the wedding website loads on first
// render: couple names, date, and the full event schedule.
func GetWedding(weddings *usecase.WeddingUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		wedding, events, err := weddings.Get(r.Context(), mux.Vars(r)["slug"])
		if err != nil {
			writeUsecaseError(w, err, "wedding not found", "failed to load wedding")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"wedding": wedding,
			"events":  events,
		})
	}
}

type weddingRequest struct {
	Slug           string `json:"slug"`
	PartnerOneName string `json:"partner_one_name"`
	PartnerTwoName string `json:"partner_two_name"`
	WeddingDate    string `json:"wedding_date"` // "2027-06-12", optional
	OpeningText    string `json:"opening_text"`
	Story          string `json:"story"`
	DressCode      string `json:"dress_code"`
}

func (req weddingRequest) input() usecase.WeddingInput {
	return usecase.WeddingInput{
		Slug:           req.Slug,
		PartnerOneName: req.PartnerOneName,
		PartnerTwoName: req.PartnerTwoName,
		WeddingDate:    req.WeddingDate,
		OpeningText:    req.OpeningText,
		Story:          req.Story,
		DressCode:      req.DressCode,
	}
}

// CreateWedding registers a new wedding; its slug becomes the address every
// other route hangs off. The admin creating it becomes its first admin.
// Admin-only.
func CreateWedding(weddings *usecase.WeddingUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req weddingRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		admin, ok := currentAdmin(w, r)
		if !ok {
			return
		}

		wedding, err := weddings.Create(r.Context(), admin.ID, req.input())
		if err != nil {
			writeUsecaseError(w, err, "wedding not found", "failed to create wedding")
			return
		}
		writeJSON(w, http.StatusCreated, wedding)
	}
}

// UpdateWedding edits the wedding the {slug} names. A different slug in the
// body renames it, which changes every link already shared. Admin-only.
func UpdateWedding(weddings *usecase.WeddingUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req weddingRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		wedding, err := weddings.Update(r.Context(), mux.Vars(r)["slug"], req.input())
		if err != nil {
			writeUsecaseError(w, err, "wedding not found", "failed to save wedding")
			return
		}
		writeJSON(w, http.StatusOK, wedding)
	}
}
