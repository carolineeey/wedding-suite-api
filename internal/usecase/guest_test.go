package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

func TestCreateGuestRetriesOnCodeCollision(t *testing.T) {
	guests := &fakeGuests{createErrs: []error{models.ErrDuplicate, models.ErrDuplicate}}
	uc := NewGuestUsecase(guests)
	codes := []string{"AAAAAAA", "BBBBBBB", "CCCCCCC"}
	uc.newInviteCode = func() (string, error) {
		code := codes[0]
		codes = codes[1:]
		return code, nil
	}

	got, err := uc.Create(context.Background(), "wedding-1", GuestInput{Name: "Guest"})
	if err != nil {
		t.Fatal(err)
	}
	if got.InviteCode != "CCCCCCC" || got.ID == "" {
		t.Errorf("guest = %+v, want third code and an ID", got)
	}
	if got.WeddingID != "wedding-1" || got.MaxGuests != 1 {
		t.Errorf("guest = %+v, want wedding-1 and MaxGuests defaulted to 1", got)
	}
}

func TestCreateGuestGivesUpAfterRepeatedCollisions(t *testing.T) {
	guests := &fakeGuests{}
	for i := 0; i < inviteCodeAttempts; i++ {
		guests.createErrs = append(guests.createErrs, models.ErrDuplicate)
	}
	uc := NewGuestUsecase(guests)
	uc.newInviteCode = func() (string, error) { return "AAAAAAA", nil }

	_, err := uc.Create(context.Background(), "wedding-1", GuestInput{Name: "Guest"})
	if !errors.Is(err, models.ErrDuplicate) {
		t.Errorf("err = %v, want ErrDuplicate", err)
	}
	if guests.createCalls != inviteCodeAttempts {
		t.Errorf("Create called %d times, want %d", guests.createCalls, inviteCodeAttempts)
	}
}

func TestCreateGuestPreconditions(t *testing.T) {
	ctx := context.Background()

	_, err := NewGuestUsecase(&fakeGuests{}).Create(ctx, "", GuestInput{Name: "Guest"})
	if !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("without wedding: err = %v, want ErrNoWeddingInScope", err)
	}

	var verr *ValidationError
	_, err = NewGuestUsecase(&fakeGuests{}).Create(ctx, "wedding-1", GuestInput{})
	if !errors.As(err, &verr) {
		t.Errorf("without name: err = %v, want ValidationError", err)
	}
}

// Reads are scoped too: without a wedding they are refused rather than
// falling back to a query across every wedding.
func TestGuestReadsRequireWedding(t *testing.T) {
	ctx := context.Background()
	uc := NewGuestUsecase(&fakeGuests{})

	if _, err := uc.List(ctx, ""); !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("List: err = %v, want ErrNoWeddingInScope", err)
	}
	if _, err := uc.Summary(ctx, ""); !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("Summary: err = %v, want ErrNoWeddingInScope", err)
	}
}

// Writes must never fall back to an unscoped query: without a wedding in
// scope they are refused outright.
func TestGuestWritesRequireWedding(t *testing.T) {
	ctx := context.Background()
	guests := &fakeGuests{}
	uc := NewGuestUsecase(guests)

	if err := uc.Update(ctx, "", "guest-1", GuestInput{Name: "Guest"}); !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("Update: err = %v, want ErrNoWeddingInScope", err)
	}
	if err := uc.Delete(ctx, "", "guest-1"); !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("Delete: err = %v, want ErrNoWeddingInScope", err)
	}
	if len(guests.updated) != 0 || len(guests.deleted) != 0 {
		t.Error("store written without a wedding in scope")
	}
}

// The wedding has to reach the store, which is what keeps one wedding's
// admin from editing another wedding's rows by id.
func TestGuestWritesCarryWeddingScope(t *testing.T) {
	ctx := context.Background()
	guests := &fakeGuests{}
	uc := NewGuestUsecase(guests)

	if err := uc.Update(ctx, "wedding-1", "guest-1", GuestInput{Name: "Guest"}); err != nil {
		t.Fatal(err)
	}
	if err := uc.Delete(ctx, "wedding-1", "guest-1"); err != nil {
		t.Fatal(err)
	}
	if len(guests.updated) != 1 || guests.updated[0].WeddingID != "wedding-1" {
		t.Errorf("updated = %+v, want the guest scoped to wedding-1", guests.updated)
	}
	if want := (scopedID{"wedding-1", "guest-1"}); len(guests.deleted) != 1 || guests.deleted[0] != want {
		t.Errorf("deleted = %+v, want %+v", guests.deleted, want)
	}
}

func TestGenerateInviteCode(t *testing.T) {
	for i := 0; i < 100; i++ {
		code, err := generateInviteCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != inviteCodeLength || strings.Trim(code, inviteCodeAlphabet) != "" {
			t.Fatalf("invalid invite code %q", code)
		}
	}
}
