// Package errtrace records the function an error came from, so a logged 500
// names the repository (or usecase) function that failed, not just the
// driver's message.
package errtrace

import (
	"errors"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

type traced struct {
	err    error
	origin string // e.g. "repository.(*GiftRepository).Create gift.go:50"
}

func (t *traced) Error() string { return t.err.Error() }
func (t *traced) Unwrap() error { return t.err }

// Wrap records the calling function on err. Wrapping nil returns nil, and an
// error already wrapped keeps its original, deeper origin. The message and
// errors.Is/As behave as for err itself.
func Wrap(err error) error {
	if err == nil {
		return nil
	}
	var t *traced
	if errors.As(err, &t) {
		return err
	}
	// CallersFrames rather than FuncForPC: it reports the function the code
	// was written in even when the compiler inlined it into its caller.
	pc := make([]uintptr, 1)
	runtime.Callers(2, pc) // skip runtime.Callers and Wrap
	f, _ := runtime.CallersFrames(pc).Next()
	// f.Function is the full import path; its last element reads
	// repository.(*GiftRepository).Create.
	name := f.Function[strings.LastIndex(f.Function, "/")+1:]
	return &traced{err: err, origin: name + " " + filepath.Base(f.File) + ":" + strconv.Itoa(f.Line)}
}

// Format returns err's message followed by the function Wrap recorded:
//
//	pq: connection refused [at repository.(*GiftRepository).ListByWedding gift.go:26]
//
// An error that was never wrapped is returned as its message alone.
func Format(err error) string {
	var t *traced
	if !errors.As(err, &t) {
		return err.Error()
	}
	return err.Error() + " [at " + t.origin + "]"
}
