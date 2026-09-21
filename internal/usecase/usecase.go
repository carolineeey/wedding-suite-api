// Package usecase holds the application's business rules. It knows nothing
// about HTTP: handlers translate requests into calls here and map the
// returned errors onto status codes. Each usecase declares the small storage
// interface it needs next to itself; the repository package satisfies them.
package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/carolineeey/wedding-suite-api/internal/models"
)

// WeddingFinder loads a wedding by its slug, which is how requests address
// one. Only WeddingScope and WeddingUsecase need it: every other usecase
// works from the ID the scope resolved.
type WeddingFinder interface {
	BySlug(ctx context.Context, slug string) (models.Wedding, error)
}

// ErrNoWeddingInScope guards the scoped usecases against running a query
// with no wedding to scope it to. Routing resolves the slug to a real ID or
// fails with models.ErrNotFound first, so reaching this means a caller
// skipped the scope.
var ErrNoWeddingInScope = errors.New("no wedding in scope")

// ValidationError is a problem with caller input. Its message is safe to
// show to the caller.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

func invalid(format string, args ...any) error {
	return &ValidationError{Message: fmt.Sprintf(format, args...)}
}

// requireWedding rejects an operation that arrived without a wedding to
// scope it to.
func requireWedding(weddingID string) error {
	if weddingID == "" {
		return ErrNoWeddingInScope
	}
	return nil
}
