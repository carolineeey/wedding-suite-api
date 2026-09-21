package usecase

import (
	"context"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

var (
	ErrAttendingRequired = &ValidationError{Message: "attending is required (true or false)"}
	ErrExceedsMaxGuests  = &ValidationError{Message: "attending_count exceeds the number of guests allowed on this invite"}
)

// RSVPStore is the guest storage RSVPUsecase needs.
type RSVPStore interface {
	GetByInviteCode(ctx context.Context, code string) (models.Guest, error)
	SaveRSVP(ctx context.Context, id string, status models.RSVPStatus, attendingCount int, message string, respondedAt time.Time) error
}

type RSVPUsecase struct {
	guests RSVPStore
	now    func() time.Time
}

func NewRSVPUsecase(guests RSVPStore) *RSVPUsecase {
	return &RSVPUsecase{guests: guests, now: time.Now}
}

// GuestByInviteCode looks up a guest by invite code, in any case. This is
// what the wedding website uses to greet the guest and pre-fill the form.
func (u *RSVPUsecase) GuestByInviteCode(ctx context.Context, code string) (models.Guest, error) {
	return u.guests.GetByInviteCode(ctx, normalizeInviteCode(code))
}

type SubmitRSVPInput struct {
	// Attending is a pointer so a missing answer is rejected rather than
	// silently recorded as a decline.
	Attending      *bool
	AttendingCount int
	Message        string
}

type RSVPResult struct {
	Status         models.RSVPStatus
	AttendingCount int
}

// Submit records a guest's response; resubmitting overwrites it. It is
// protected only by the invite code being unguessable, which is enough for
// a wedding invite list.
func (u *RSVPUsecase) Submit(ctx context.Context, code string, in SubmitRSVPInput) (RSVPResult, error) {
	guest, err := u.GuestByInviteCode(ctx, code)
	if err != nil {
		return RSVPResult{}, err
	}
	if in.Attending == nil {
		return RSVPResult{}, ErrAttendingRequired
	}
	if tooLong(in.Message, maxMessageChars) {
		return RSVPResult{}, invalid("message is too long (max %d characters)", maxMessageChars)
	}

	result := RSVPResult{Status: models.RSVPDeclined}
	if *in.Attending {
		result.Status = models.RSVPAttending
		result.AttendingCount = max(in.AttendingCount, 1)
		if result.AttendingCount > guest.MaxGuests {
			return RSVPResult{}, ErrExceedsMaxGuests
		}
	}

	if err := u.guests.SaveRSVP(ctx, guest.ID, result.Status, result.AttendingCount, in.Message, u.now()); err != nil {
		return RSVPResult{}, err
	}
	return result, nil
}
