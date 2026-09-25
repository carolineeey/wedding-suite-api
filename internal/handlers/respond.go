package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/carolineeey/wedding-suite-api/internal/middleware"
	"github.com/carolineeey/wedding-suite-api/internal/models"
	"github.com/carolineeey/wedding-suite-api/internal/usecase"
)

// maxBodyBytes caps request bodies; every payload this API accepts is a
// handful of short fields.
const maxBodyBytes = 64 << 10

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// writeUsecaseError maps an error from the usecase layer onto a response:
// validation problems become 400s carrying their message, bad credentials
// become 401s, missing records become 404s with notFoundMsg, and anything
// else is logged with the request that caused it and becomes a 500 with
// internalMsg, so database errors never reach the caller. An unknown wedding
// slug arrives here as models.ErrNotFound.
func writeUsecaseError(w http.ResponseWriter, r *http.Request, err error, notFoundMsg, internalMsg string) {
	var invalid *usecase.ValidationError
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, invalid.Message)
	case errors.Is(err, usecase.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid email or password")
	case errors.Is(err, models.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundMsg)
	default:
		middleware.LogError(r, internalMsg, err)
		writeError(w, http.StatusInternalServerError, internalMsg)
	}
}

// decodeJSON decodes a size-limited request body into dst. On failure it
// writes the error response itself and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}
