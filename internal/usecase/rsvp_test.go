package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

var fixedNow = time.Date(2027, 1, 2, 3, 4, 5, 0, time.UTC)

func newTestRSVPUsecase() (*RSVPUsecase, *fakeRSVPStore) {
	store := &fakeRSVPStore{byCode: map[string]models.Guest{
		"ABC2345": {ID: "guest-1", InviteCode: "ABC2345", MaxGuests: 2},
	}}
	uc := NewRSVPUsecase(store)
	uc.now = func() time.Time { return fixedNow }
	return uc, store
}

func TestSubmitRSVP(t *testing.T) {
	tests := []struct {
		name    string
		code    string
		in      SubmitRSVPInput
		want    RSVPResult
		wantErr error
	}{
		{
			name: "attending defaults count to one",
			code: "ABC2345",
			in:   SubmitRSVPInput{Attending: ptr(true)},
			want: RSVPResult{Status: models.RSVPAttending, AttendingCount: 1},
		},
		{
			name: "attending up to the allowance",
			code: "ABC2345",
			in:   SubmitRSVPInput{Attending: ptr(true), AttendingCount: 2},
			want: RSVPResult{Status: models.RSVPAttending, AttendingCount: 2},
		},
		{
			name: "declining ignores the count",
			code: "ABC2345",
			in:   SubmitRSVPInput{Attending: ptr(false), AttendingCount: 2},
			want: RSVPResult{Status: models.RSVPDeclined, AttendingCount: 0},
		},
		{
			name: "invite code ignores case and whitespace",
			code: " abc2345 ",
			in:   SubmitRSVPInput{Attending: ptr(false)},
			want: RSVPResult{Status: models.RSVPDeclined},
		},
		{
			name:    "missing attending is rejected",
			code:    "ABC2345",
			in:      SubmitRSVPInput{},
			wantErr: ErrAttendingRequired,
		},
		{
			name:    "more people than the allowance is rejected",
			code:    "ABC2345",
			in:      SubmitRSVPInput{Attending: ptr(true), AttendingCount: 3},
			wantErr: ErrExceedsMaxGuests,
		},
		{
			name:    "unknown invite code",
			code:    "ZZZ9999",
			in:      SubmitRSVPInput{Attending: ptr(true)},
			wantErr: models.ErrNotFound,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, store := newTestRSVPUsecase()
			got, err := uc.Submit(context.Background(), tt.code, tt.in)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr != nil {
				if store.saved != nil {
					t.Errorf("RSVP saved despite error: %+v", *store.saved)
				}
				return
			}
			if got != tt.want {
				t.Errorf("result = %+v, want %+v", got, tt.want)
			}
			want := savedRSVP{id: "guest-1", status: tt.want.Status, attendingCount: tt.want.AttendingCount, respondedAt: fixedNow}
			if store.saved == nil || *store.saved != want {
				t.Errorf("saved = %+v, want %+v", store.saved, want)
			}
		})
	}
}

func TestSubmitRSVPMessageLength(t *testing.T) {
	uc, _ := newTestRSVPUsecase()
	ctx := context.Background()

	atLimit := SubmitRSVPInput{Attending: ptr(false), Message: strings.Repeat("é", maxMessageChars)}
	if _, err := uc.Submit(ctx, "ABC2345", atLimit); err != nil {
		t.Errorf("message at the limit: %v", err)
	}

	overLimit := SubmitRSVPInput{Attending: ptr(false), Message: strings.Repeat("é", maxMessageChars+1)}
	var verr *ValidationError
	if _, err := uc.Submit(ctx, "ABC2345", overLimit); !errors.As(err, &verr) {
		t.Errorf("message over the limit: err = %v, want ValidationError", err)
	}
}
