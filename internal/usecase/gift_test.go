package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCreateGift(t *testing.T) {
	gifts := &fakeGifts{}
	in := CreateGiftInput{BankName: "BCA", AccountName: "Esther", AccountNumber: "1234567890"}

	if _, err := NewGiftUsecase(gifts).Create(context.Background(), "wedding-1", in); err != nil {
		t.Fatal(err)
	}
	if len(gifts.created) != 1 || gifts.created[0].WeddingID != "wedding-1" {
		t.Errorf("created = %+v, want one account in wedding-1", gifts.created)
	}
}

func TestCreateGiftValidation(t *testing.T) {
	long := strings.Repeat("x", maxNameChars+1)
	cases := map[string]CreateGiftInput{
		"missing number": {BankName: "BCA", AccountName: "Esther"},
		"missing bank":   {AccountName: "Esther", AccountNumber: "1"},
		"too long":       {BankName: long, AccountName: "Esther", AccountNumber: "1"},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			gifts := &fakeGifts{}
			_, err := NewGiftUsecase(gifts).Create(context.Background(), "wedding-1", in)
			var verr *ValidationError
			if !errors.As(err, &verr) {
				t.Errorf("err = %v, want a ValidationError", err)
			}
			if len(gifts.created) != 0 {
				t.Error("invalid gift account was stored")
			}
		})
	}
}

func TestDeleteGiftIsScoped(t *testing.T) {
	gifts := &fakeGifts{}
	if err := NewGiftUsecase(gifts).Delete(context.Background(), "wedding-1", "gift-9"); err != nil {
		t.Fatal(err)
	}
	if len(gifts.deleted) != 1 || gifts.deleted[0] != (scopedID{"wedding-1", "gift-9"}) {
		t.Errorf("deleted = %+v, want gift-9 scoped to wedding-1", gifts.deleted)
	}
}
