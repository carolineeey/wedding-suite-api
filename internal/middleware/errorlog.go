package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/carolineeey/wedding-suite-api/internal/errtrace"
)

// maxLoggedBody caps the copy Logging keeps of each request body. Every
// payload this API accepts is a handful of short fields.
const maxLoggedBody = 8 << 10

// bodyCapture copies what the handler reads from the request body, up to
// maxLoggedBody bytes.
type bodyCapture struct {
	io.ReadCloser
	buf       bytes.Buffer
	truncated bool
}

func (b *bodyCapture) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if room := maxLoggedBody - b.buf.Len(); room < n {
		b.buf.Write(p[:max(room, 0)])
		b.truncated = true
	} else {
		b.buf.Write(p[:n])
	}
	return n, err
}

// LogError logs a request that failed with an internal error: the request,
// the function the error came from, and the payload the caller sent, e.g.
//
//	ERROR POST /api/v1/admin/w/budi-ani/gifts: failed to create gift account:
//	pq: ... [at repository.(*GiftRepository).Create gift.go:50]
//	payload={"account_name":"Ani","account_number":"123","bank_name":"BCA"}
//
// all on one line.
func LogError(r *http.Request, msg string, err error) {
	line := "ERROR " + r.Method + " " + r.URL.RequestURI() + ": " + msg + ": " + errtrace.Format(err)
	if body, ok := r.Context().Value(bodyKey).(*bodyCapture); ok && body.buf.Len() > 0 {
		line += " payload=" + redactedPayload(body)
	}
	log.Print(line)
}

// redactedPayload renders the body for the log with secret fields masked.
// Anything that is not a whole JSON object is left out rather than risk
// logging a password from a malformed or truncated body.
func redactedPayload(body *bodyCapture) string {
	var fields map[string]any
	if body.truncated || json.Unmarshal(body.buf.Bytes(), &fields) != nil {
		return "<" + strconv.Itoa(body.buf.Len()) + " bytes, not logged>"
	}
	for k := range fields {
		if isSecretField(k) {
			fields[k] = "[REDACTED]"
		}
	}
	out, err := json.Marshal(fields)
	if err != nil {
		return "<unprintable>"
	}
	return string(out)
}

func isSecretField(name string) bool {
	name = strings.ToLower(name)
	return strings.Contains(name, "password") || strings.Contains(name, "token")
}
