// Package repository holds all SQL. Each repository maps rows to models and
// translates driver errors into models.ErrNotFound and models.ErrDuplicate,
// so callers never need to know about database/sql or lib/pq.
package repository

import (
	"database/sql"
	"errors"

	"github.com/carolineeey/wedding-suite-api/internal/errtrace"
	"github.com/carolineeey/wedding-suite-api/internal/models"
)

type rowScanner interface {
	Scan(dest ...any) error
}

// requireAffected turns an update/delete that matched no rows into
// models.ErrNotFound.
func requireAffected(res sql.Result, err error) error {
	if err != nil {
		return errtrace.Wrap(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return errtrace.Wrap(err)
	}
	if n == 0 {
		return models.ErrNotFound
	}
	return nil
}

func isUniqueViolation(err error) bool {
	// lib/pq reports unique_violation as SQLSTATE 23505.
	var pe interface{ SQLState() string }
	return errors.As(err, &pe) && pe.SQLState() == "23505"
}
