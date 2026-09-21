package handlers

import (
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/usecase"
	"github.com/gorilla/mux"
)

// weddingID resolves the {slug} in the route to the wedding the request
// works on, and hands that ID to the usecase. An unknown slug is a 404, the
// same as any other missing record. On failure it writes the response itself
// and returns false.
func weddingID(w http.ResponseWriter, r *http.Request, scope *usecase.WeddingScope, internalMsg string) (string, bool) {
	id, err := scope.BySlug(r.Context(), mux.Vars(r)["slug"])
	if err != nil {
		writeUsecaseError(w, err, "wedding not found", internalMsg)
		return "", false
	}
	return id, true
}
