package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCreateWishApproval(t *testing.T) {
	for _, requireApproval := range []bool{false, true} {
		wishes := &fakeWishes{}
		uc := NewWishUsecase(wishes, requireApproval)

		got, err := uc.Create(context.Background(), "wedding-1", CreateWishInput{GuestName: "  Ana ", Message: " Congrats! "})
		if err != nil {
			t.Fatal(err)
		}
		if got.IsApproved == requireApproval {
			t.Errorf("requireApproval=%v: IsApproved = %v", requireApproval, got.IsApproved)
		}
		if got.GuestName != "Ana" || got.Message != "Congrats!" {
			t.Errorf("wish = %+v, want trimmed name and message", got)
		}
		if got.ID == "" || len(wishes.created) != 1 {
			t.Errorf("wish not stored: %+v", got)
		}
	}
}

func TestCreateWishValidation(t *testing.T) {
	tests := []struct {
		name   string
		in     CreateWishInput
		wantOK bool
	}{
		{"blank name", CreateWishInput{GuestName: "   ", Message: "hi"}, false},
		{"blank message", CreateWishInput{GuestName: "Ana", Message: ""}, false},
		{"name at limit", CreateWishInput{GuestName: strings.Repeat("é", maxNameChars), Message: "hi"}, true},
		{"name over limit", CreateWishInput{GuestName: strings.Repeat("é", maxNameChars+1), Message: "hi"}, false},
		{"message at limit", CreateWishInput{GuestName: "Ana", Message: strings.Repeat("é", maxMessageChars)}, true},
		{"message over limit", CreateWishInput{GuestName: "Ana", Message: strings.Repeat("é", maxMessageChars+1)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := NewWishUsecase(&fakeWishes{}, false)
			_, err := uc.Create(context.Background(), "wedding-1", tt.in)
			var verr *ValidationError
			switch {
			case tt.wantOK && err != nil:
				t.Errorf("unexpected error: %v", err)
			case !tt.wantOK && !errors.As(err, &verr):
				t.Errorf("err = %v, want ValidationError", err)
			}
		})
	}
}

func TestListWishesLimit(t *testing.T) {
	for requested, want := range map[int]int{-1: 50, 0: 50, 1: 1, 200: 200, 201: 50} {
		wishes := &fakeWishes{}
		uc := NewWishUsecase(wishes, false)
		if _, err := uc.List(context.Background(), "wedding-1", false, requested); err != nil {
			t.Fatal(err)
		}
		if wishes.listedLimit != want {
			t.Errorf("limit %d: repository got %d, want %d", requested, wishes.listedLimit, want)
		}
	}
}

func TestListWishesRequiresWedding(t *testing.T) {
	_, err := NewWishUsecase(&fakeWishes{}, false).List(context.Background(), "", false, 0)
	if !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("err = %v, want ErrNoWeddingInScope", err)
	}
}

// Moderation is scoped: the wedding reaches the store, and without one the
// write is refused rather than run unscoped.
func TestWishModerationCarriesWeddingScope(t *testing.T) {
	ctx := context.Background()
	wishes := &fakeWishes{}
	uc := NewWishUsecase(wishes, false)

	if err := uc.SetApproval(ctx, "", "wish-1", true); !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("SetApproval: err = %v, want ErrNoWeddingInScope", err)
	}
	if err := uc.Delete(ctx, "", "wish-1"); !errors.Is(err, ErrNoWeddingInScope) {
		t.Errorf("Delete: err = %v, want ErrNoWeddingInScope", err)
	}

	if err := uc.SetApproval(ctx, "wedding-1", "wish-1", true); err != nil {
		t.Fatal(err)
	}
	if err := uc.Delete(ctx, "wedding-1", "wish-1"); err != nil {
		t.Fatal(err)
	}
	want := scopedID{"wedding-1", "wish-1"}
	if len(wishes.approved) != 1 || wishes.approved[0] != want {
		t.Errorf("approved = %+v, want %+v", wishes.approved, want)
	}
	if len(wishes.deleted) != 1 || wishes.deleted[0] != want {
		t.Errorf("deleted = %+v, want %+v", wishes.deleted, want)
	}
}
