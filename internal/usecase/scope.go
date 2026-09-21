package usecase

import (
	"context"
)

// WeddingScope resolves which wedding a request operates on from the {slug}
// in its URL. It is the one place that turns an address into a wedding ID;
// every other usecase takes that ID and never looks a wedding up itself.
type WeddingScope struct {
	weddings WeddingFinder
}

func NewWeddingScope(weddings WeddingFinder) *WeddingScope {
	return &WeddingScope{weddings: weddings}
}

// BySlug returns the ID of the wedding the slug names, reporting an unknown
// slug as models.ErrNotFound. The slug is normalized first, so a link that
// arrives with stray case still resolves.
func (s *WeddingScope) BySlug(ctx context.Context, slug string) (string, error) {
	wedding, err := s.weddings.BySlug(ctx, normalizeSlug(slug))
	if err != nil {
		return "", err
	}
	return wedding.ID, nil
}
