package errtrace

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

var errDriver = errors.New("pq: connection refused")

func fakeRepository() error { return Wrap(errDriver) }

func fakeUsecase() error {
	if err := fakeRepository(); err != nil {
		return fmt.Errorf("loading schedule: %w", err)
	}
	return nil
}

// The logged line must name the function the error came from, not the ones
// that passed it up.
func TestFormatNamesTheOrigin(t *testing.T) {
	got := Format(fakeUsecase())

	want := "loading schedule: pq: connection refused [at errtrace.fakeRepository errtrace_test.go:"
	if !strings.HasPrefix(got, want) || !strings.HasSuffix(got, "]") {
		t.Errorf("Format = %q, want %q<line>]", got, want)
	}
	if strings.Contains(got, "fakeUsecase") {
		t.Errorf("Format = %q, want only the origin", got)
	}
}

func TestWrapKeepsTheError(t *testing.T) {
	err := fakeUsecase()
	if !errors.Is(err, errDriver) {
		t.Errorf("errors.Is(%v, errDriver) = false, want true", err)
	}
	if err.Error() != "loading schedule: pq: connection refused" {
		t.Errorf("Error() = %q, want the message without the origin", err.Error())
	}
}

func TestWrapKeepsTheFirstOrigin(t *testing.T) {
	err := Wrap(fakeRepository())
	if !strings.Contains(Format(err), "[at errtrace.fakeRepository ") {
		t.Errorf("Format = %q, want the origin from the first Wrap", Format(err))
	}
}

func TestWrapNil(t *testing.T) {
	if Wrap(nil) != nil {
		t.Error("Wrap(nil) != nil")
	}
}

func TestFormatUnwrapped(t *testing.T) {
	if got := Format(errDriver); got != errDriver.Error() {
		t.Errorf("Format = %q, want the bare message", got)
	}
}
