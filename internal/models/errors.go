package models

import "errors"

// Sentinel errors shared across layers. The repository layer returns them and
// the usecase and handler layers check for them with errors.Is, so neither
// needs to import the repository package.
var (
	// ErrNotFound is returned when the requested record does not exist.
	ErrNotFound = errors.New("not found")

	// ErrDuplicate is returned when a write violates a uniqueness rule.
	ErrDuplicate = errors.New("duplicate")
)
