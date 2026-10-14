package handlers

import (
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/usecase"
	"github.com/gorilla/mux"
)

// GetInvitation returns everything the invitation page shows the guest the
// code belongs to: the guest, their wedding, its schedule, and gift
// accounts. Public, rate limited like the other invite-code lookups.
func GetInvitation(invitations *usecase.InvitationUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inv, err := invitations.Get(r.Context(), mux.Vars(r)["code"])
		if err != nil {
			writeUsecaseError(w, r, err, "invite code not found", "failed to load invitation")
			return
		}
		writeJSON(w, http.StatusOK, inv)
	}
}

// GetGuestByCode looks up a guest by their invite code. This is what the
// wedding website calls to greet the guest by name and pre-fill their RSVP
// form (e.g. /rsvp/AB12CDE on the frontend).
func GetGuestByCode(rsvp *usecase.RSVPUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		guest, err := rsvp.GuestByInviteCode(r.Context(), mux.Vars(r)["code"])
		if err != nil {
			writeUsecaseError(w, r, err, "invite code not found", "failed to load guest")
			return
		}
		writeJSON(w, http.StatusOK, guest)
	}
}

type submitRSVPRequest struct {
	Attending      *bool  `json:"attending"` // required; nil means the field was missing
	AttendingCount int    `json:"attending_count"`
	Message        string `json:"message,omitempty"`
}

// SubmitRSVP records a guest's response. Public.
func SubmitRSVP(rsvp *usecase.RSVPUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req submitRSVPRequest
		if !decodeJSON(w, r, &req) {
			return
		}

		result, err := rsvp.Submit(r.Context(), mux.Vars(r)["code"], usecase.SubmitRSVPInput{
			Attending:      req.Attending,
			AttendingCount: req.AttendingCount,
			Message:        req.Message,
		})
		if err != nil {
			writeUsecaseError(w, r, err, "invite code not found", "failed to save RSVP")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":          "saved",
			"rsvp_status":     result.Status,
			"attending_count": result.AttendingCount,
		})
	}
}
