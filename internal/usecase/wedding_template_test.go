package usecase

import (
	"context"
	"errors"
	"testing"
)

func TestWeddingTemplateCreate(t *testing.T) {
	for _, id := range []string{"ulos-heritage", "botanical"} {
		t.Run(id, func(t *testing.T) {
			store := &fakeWeddings{}
			got, err := NewWeddingUsecase(store, &fakeEvents{}).Create(context.Background(), "admin-1", WeddingInput{Slug: "s", PartnerOneName: "A", PartnerTwoName: "B", TemplateID: &id})
			if err != nil || got.TemplateID != id {
				t.Fatalf("template = %q, err = %v; want %q", got.TemplateID, err, id)
			}
		})
	}
	store := &fakeWeddings{}
	got, err := NewWeddingUsecase(store, &fakeEvents{}).Create(context.Background(), "admin-1", WeddingInput{Slug: "s", PartnerOneName: "A", PartnerTwoName: "B"})
	if err != nil || got.TemplateID != "ulos-heritage" {
		t.Fatalf("default template = %q, err = %v", got.TemplateID, err)
	}
}

func TestWeddingTemplateUpdateAndOmission(t *testing.T) {
	w := testWedding()
	w.TemplateID = "botanical"
	store := &fakeWeddings{wedding: w}
	uc := NewWeddingUsecase(store, &fakeEvents{})
	in := WeddingInput{Slug: "s", PartnerOneName: "A", PartnerTwoName: "B"}
	got, err := uc.Update(context.Background(), "s", in)
	if err != nil || got.TemplateID != "botanical" {
		t.Fatalf("omitted template = %q, err = %v", got.TemplateID, err)
	}
	id := "ulos-heritage"
	in.TemplateID = &id
	got, err = uc.Update(context.Background(), "s", in)
	if err != nil || got.TemplateID != id {
		t.Fatalf("changed template = %q, err = %v", got.TemplateID, err)
	}
	// A new read (as used by public and invitation handlers) sees the stored choice.
	got, _, err = uc.Get(context.Background(), "s")
	if err != nil || got.TemplateID != id {
		t.Fatalf("read template = %q, err = %v", got.TemplateID, err)
	}
}

func TestWeddingTemplateRejectsUnavailable(t *testing.T) {
	for _, id := range []string{"", "huta-monokrom", "unknown", "../../../bad"} {
		t.Run(id, func(t *testing.T) {
			store := &fakeWeddings{wedding: testWedding()}
			_, err := NewWeddingUsecase(store, &fakeEvents{}).Update(context.Background(), "s", WeddingInput{Slug: "s", PartnerOneName: "A", PartnerTwoName: "B", TemplateID: &id})
			var invalid *ValidationError
			if !errors.As(err, &invalid) || store.updates != 0 {
				t.Fatalf("err = %v, updates = %d; want validation failure without write", err, store.updates)
			}
		})
	}
}
