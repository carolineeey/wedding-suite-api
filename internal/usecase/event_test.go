package usecase

import (
	"context"
	"errors"
	"testing"
)

// maps_url is rendered as a link on the guest page, so only real web links
// may be stored.
func TestCreateEventMapsURL(t *testing.T) {
	cases := map[string]bool{
		"https://maps.app.goo.gl/abc":     true,
		"http://maps.google.com/?q=venue": true,
		"javascript:alert(1)":             false,
		"maps.google.com":                 false,
		"https://":                        false,
	}
	for url, ok := range cases {
		t.Run(url, func(t *testing.T) {
			events := &fakeEvents{}
			_, err := NewEventUsecase(events).Create(context.Background(), "wedding-1", CreateEventInput{
				Name: "Akad", StartsAt: "2027-06-12T09:00:00+07:00", MapsURL: url,
			})
			var verr *ValidationError
			if ok && err != nil {
				t.Errorf("err = %v, want the link accepted", err)
			}
			if !ok && !errors.As(err, &verr) {
				t.Errorf("err = %v, want a ValidationError", err)
			}
			if ok && (len(events.created) != 1 || events.created[0].MapsURL != url) {
				t.Errorf("created = %+v, want the link stored", events.created)
			}
		})
	}
}
