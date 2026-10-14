package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

func newTestInvitationUsecase(weddings *fakeWeddings) *InvitationUsecase {
	return NewInvitationUsecase(InvitationStores{
		Guests: &fakeRSVPStore{byCode: map[string]models.Guest{
			"ABC2345": {ID: "guest-1", WeddingID: "wedding-1", InviteCode: "ABC2345", Name: "Budi"},
		}},
		Weddings: weddings,
		Events:   &fakeEvents{created: []models.Event{{Name: "Akad"}}},
		Gifts:    &fakeGifts{created: []models.GiftAccount{{BankName: "BCA"}}},
	})
}

// The invite code alone must lead to the whole page: guest, their wedding,
// its schedule, and the gift accounts.
func TestGetInvitation(t *testing.T) {
	inv, err := newTestInvitationUsecase(&fakeWeddings{wedding: testWedding()}).
		Get(context.Background(), " abc2345 ")
	if err != nil {
		t.Fatal(err)
	}
	if inv.Guest.Name != "Budi" || inv.Wedding.ID != "wedding-1" {
		t.Errorf("guest/wedding = %q/%q, want Budi's wedding", inv.Guest.Name, inv.Wedding.ID)
	}
	if len(inv.Events) != 1 || len(inv.Gifts) != 1 {
		t.Errorf("events/gifts = %d/%d, want 1/1", len(inv.Events), len(inv.Gifts))
	}
}

func TestGetInvitationUnknownCode(t *testing.T) {
	_, err := newTestInvitationUsecase(&fakeWeddings{wedding: testWedding()}).
		Get(context.Background(), "NOPE234")
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

// A guest whose wedding is missing breaks the foreign key the schema
// guarantees; that is a server fault, and must not look like a bad code.
func TestGetInvitationMissingWeddingIsNotA404(t *testing.T) {
	_, err := newTestInvitationUsecase(&fakeWeddings{}).Get(context.Background(), "ABC2345")
	if err == nil || errors.Is(err, models.ErrNotFound) {
		t.Errorf("err = %v, want a non-ErrNotFound error", err)
	}
}
