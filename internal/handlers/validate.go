package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
)

// pathUUID reads the {id} route variable. IDs are checked before querying
// because Postgres rejects a malformed UUID with an error, which would
// otherwise surface as a 500 instead of a 404. On failure it writes the 404
// itself and returns false.
func pathUUID(w http.ResponseWriter, r *http.Request, notFoundMsg string) (string, bool) {
	id := mux.Vars(r)["id"]
	if !isUUID(id) {
		writeError(w, http.StatusNotFound, notFoundMsg)
		return "", false
	}
	return id, true
}

// isUUID reports whether s is a hyphenated 8-4-4-4-12 hex UUID, the form
// Postgres returns and this API hands out.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
				return false
			}
		}
	}
	return true
}
