package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

func TestCreateWedding(t *testing.T) {
	weddings := &fakeWeddings{}
	in := WeddingInput{Slug: "caroline-rafi", PartnerOneName: "A", PartnerTwoName: "B", WeddingDate: "2027-06-12"}

	got, err := NewWeddingUsecase(weddings, &fakeEvents{}).Create(context.Background(), "admin-1", in)
	if err != nil {
		t.Fatal(err)
	}
	if weddings.creates != 1 {
		t.Errorf("creates = %d, want 1", weddings.creates)
	}
	if weddings.owner != "admin-1" {
		t.Errorf("owner = %q, want the creating admin to own the wedding", weddings.owner)
	}
	if got.Slug != "caroline-rafi" || got.WeddingDate == nil || *got.WeddingDate != "2027-06-12" {
		t.Errorf("wedding = %+v, want the slug and date stored", got)
	}
}

func TestCreateWeddingRequiresOwner(t *testing.T) {
	weddings := &fakeWeddings{}
	in := WeddingInput{Slug: "s", PartnerOneName: "A", PartnerTwoName: "B"}

	_, err := NewWeddingUsecase(weddings, &fakeEvents{}).Create(context.Background(), "", in)
	if !errors.Is(err, ErrNoAdminInScope) {
		t.Errorf("err = %v, want ErrNoAdminInScope", err)
	}
	if weddings.creates != 0 {
		t.Error("wedding created with no admin to own it")
	}
}

func TestUpdateWedding(t *testing.T) {
	weddings := &fakeWeddings{wedding: testWedding()}
	in := WeddingInput{Slug: "s", PartnerOneName: "A", PartnerTwoName: "C"}

	got, err := NewWeddingUsecase(weddings, &fakeEvents{}).Update(context.Background(), "s", in)
	if err != nil {
		t.Fatal(err)
	}
	if weddings.updates != 1 || got.ID != "wedding-1" || got.PartnerTwoName != "C" {
		t.Errorf("wedding = %+v after %d updates; want wedding-1 renamed once", got, weddings.updates)
	}
}

// The slug in the path addresses the wedding, so an unknown one is a missing
// record rather than an invitation to create it.
func TestUpdateWeddingUnknownSlug(t *testing.T) {
	weddings := &fakeWeddings{wedding: testWedding()}
	in := WeddingInput{Slug: "ghost", PartnerOneName: "A", PartnerTwoName: "B"}

	_, err := NewWeddingUsecase(weddings, &fakeEvents{}).Update(context.Background(), "ghost", in)
	if !errors.Is(err, models.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
	if weddings.updates != 0 {
		t.Error("wedding updated despite an unknown slug")
	}
}

// A slug another wedding already holds must be reported, never merged into
// that wedding: the slug is its public address.
func TestCreateWeddingRejectsTakenSlug(t *testing.T) {
	weddings := &fakeWeddings{takenSlug: "taken"}
	in := WeddingInput{Slug: "taken", PartnerOneName: "A", PartnerTwoName: "B"}

	_, err := NewWeddingUsecase(weddings, &fakeEvents{}).Create(context.Background(), "admin-1", in)
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if weddings.creates != 0 || weddings.updates != 0 {
		t.Errorf("creates = %d, updates = %d; want the conflicting slug to change nothing",
			weddings.creates, weddings.updates)
	}
}

// Losing the slug to a concurrent request is reported to the caller and
// leaves the wedding that won alone, instead of quietly overwriting it.
func TestCreateWeddingLosingSlugRace(t *testing.T) {
	weddings := &fakeWeddings{loseCreateRace: true}
	in := WeddingInput{Slug: "caroline-rafi", PartnerOneName: "A", PartnerTwoName: "B"}

	_, err := NewWeddingUsecase(weddings, &fakeEvents{}).Create(context.Background(), "admin-1", in)
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if weddings.updates != 0 {
		t.Error("the wedding that won the race was overwritten")
	}
}

// Renaming onto a slug another wedding holds is a caller mistake too, not a
// raw database error leaking out as a 500.
func TestUpdateWeddingRejectsTakenSlug(t *testing.T) {
	weddings := &fakeWeddings{wedding: testWedding(), takenSlug: "taken"}
	in := WeddingInput{Slug: "taken", PartnerOneName: "A", PartnerTwoName: "B"}

	_, err := NewWeddingUsecase(weddings, &fakeEvents{}).Update(context.Background(), "s", in)
	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, want ValidationError", err)
	}
	if weddings.updates != 0 {
		t.Error("wedding renamed onto a slug it does not own")
	}
}

func TestWeddingInputValidation(t *testing.T) {
	for name, in := range map[string]WeddingInput{
		"missing slug":        {PartnerOneName: "A", PartnerTwoName: "B"},
		"blank slug":          {Slug: "   ", PartnerOneName: "A", PartnerTwoName: "B"},
		"date not ISO 8601":   {Slug: "s", PartnerOneName: "A", PartnerTwoName: "B", WeddingDate: "12/06/2027"},
		"slug with spaces":    {Slug: "caroline and rafi", PartnerOneName: "A", PartnerTwoName: "B"},
		"slug with symbols":   {Slug: "caroline&rafi", PartnerOneName: "A", PartnerTwoName: "B"},
		"slug trailing dash":  {Slug: "caroline-rafi-", PartnerOneName: "A", PartnerTwoName: "B"},
		"slug doubled dashes": {Slug: "caroline--rafi", PartnerOneName: "A", PartnerTwoName: "B"},
		"slug too long":       {Slug: strings.Repeat("a", maxSlugChars+1), PartnerOneName: "A", PartnerTwoName: "B"},
	} {
		t.Run(name, func(t *testing.T) {
			weddings := &fakeWeddings{}
			_, err := NewWeddingUsecase(weddings, &fakeEvents{}).Create(context.Background(), "admin-1", in)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Errorf("err = %v, want ValidationError", err)
			}
			if weddings.creates != 0 {
				t.Error("wedding created despite invalid input")
			}
		})
	}
}

func TestCreateWeddingNormalizesSlug(t *testing.T) {
	weddings := &fakeWeddings{}

	got, err := NewWeddingUsecase(weddings, &fakeEvents{}).Create(context.Background(), "admin-1", WeddingInput{
		Slug:           "  Caroline-Rafi  ",
		PartnerOneName: "A",
		PartnerTwoName: "B",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Slug != "caroline-rafi" {
		t.Errorf("slug = %q, want %q", got.Slug, "caroline-rafi")
	}
}

func TestGetWedding(t *testing.T) {
	weddings := &fakeWeddings{wedding: testWedding()}
	uc := NewWeddingUsecase(weddings, &fakeEvents{})

	// The slug is normalized on the way in, so a link with stray case works.
	wedding, events, err := uc.Get(context.Background(), " S ")
	if err != nil {
		t.Fatal(err)
	}
	if wedding.ID != "wedding-1" || len(events) != 0 {
		t.Errorf("wedding = %+v, events = %v", wedding, events)
	}

	if _, _, err := uc.Get(context.Background(), "ghost"); !errors.Is(err, models.ErrNotFound) {
		t.Errorf("unknown slug: err = %v, want ErrNotFound", err)
	}
}

func TestCreateEvent(t *testing.T) {
	const start = "2027-06-12T09:00:00+07:00"
	tests := []struct {
		name   string
		in     CreateEventInput
		wantOK bool
	}{
		{"valid with end time", CreateEventInput{Name: "Ceremony", StartsAt: start, EndsAt: "2027-06-12T10:00:00+07:00"}, true},
		{"missing name", CreateEventInput{StartsAt: start}, false},
		{"starts_at not RFC3339", CreateEventInput{Name: "Ceremony", StartsAt: "2027-06-12 09:00"}, false},
		{"ends_at not RFC3339", CreateEventInput{Name: "Ceremony", StartsAt: start, EndsAt: "10am"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			events := &fakeEvents{}
			_, err := NewEventUsecase(events).Create(context.Background(), "wedding-1", tt.in)
			if !tt.wantOK {
				var verr *ValidationError
				if !errors.As(err, &verr) {
					t.Errorf("err = %v, want ValidationError", err)
				}
				if len(events.created) != 0 {
					t.Error("event stored despite invalid input")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(events.created) != 1 || events.created[0].EndsAt == nil || events.created[0].WeddingID != "wedding-1" {
				t.Errorf("stored events = %+v", events.created)
			}
		})
	}
}

func TestEventDeleteScope(t *testing.T) {
	ctx := context.Background()
	events := &fakeEvents{}
	uc := NewEventUsecase(events)

	if err := uc.Delete(ctx, "", "event-1"); !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("err = %v, want ErrNoWeddingInScope", err)
	}
	if err := uc.Delete(ctx, "wedding-1", "event-1"); err != nil {
		t.Fatal(err)
	}
	if want := (scopedID{"wedding-1", "event-1"}); len(events.deleted) != 1 || events.deleted[0] != want {
		t.Errorf("deleted = %+v, want %+v", events.deleted, want)
	}
}

func TestWeddingScopeBySlug(t *testing.T) {
	ctx := context.Background()

	got, err := NewWeddingScope(&fakeWeddings{wedding: testWedding()}).BySlug(ctx, "s")
	if err != nil || got != "wedding-1" {
		t.Errorf("BySlug(\"s\") = %q, %v; want wedding-1", got, err)
	}

	if _, err := NewWeddingScope(&fakeWeddings{}).BySlug(ctx, "ghost"); !errors.Is(err, models.ErrNotFound) {
		t.Errorf("unknown slug: err = %v, want ErrNotFound", err)
	}
}
