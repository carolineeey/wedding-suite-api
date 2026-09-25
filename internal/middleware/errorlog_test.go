package middleware

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/carolineeey/wedding-suite-api/internal/errtrace"
)

// logFailure sends body through Logging to a handler that reads it and then
// fails, and returns the ERROR line LogError wrote.
func logFailure(t *testing.T, body string) string {
	t.Helper()
	var out bytes.Buffer
	log.SetOutput(&out)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	failing := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		LogError(r, "failed to save", errtrace.Wrap(errors.New("pq: boom")))
	})
	Logging(failing).ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/api/v1/things?x=1", strings.NewReader(body)))

	for _, line := range strings.Split(out.String(), "\n") {
		if strings.Contains(line, "ERROR ") {
			return line
		}
	}
	t.Fatalf("no ERROR line in %q", out.String())
	return ""
}

func TestLogErrorNamesRequestOriginAndPayload(t *testing.T) {
	line := logFailure(t, `{"bank_name":"BCA"}`)
	for _, want := range []string{
		"ERROR POST /api/v1/things?x=1: failed to save: pq: boom",
		"[at middleware.logFailure.",
		`payload={"bank_name":"BCA"}`,
	} {
		if !strings.Contains(line, want) {
			t.Errorf("log = %q, want it to contain %q", line, want)
		}
	}
}

func TestLogErrorRedactsSecrets(t *testing.T) {
	line := logFailure(t, `{"email":"a@b.c","password":"hunter22","new_password":"x","token":"t"}`)
	if strings.Contains(line, "hunter22") || strings.Contains(line, `"t"`) || strings.Contains(line, `"x"`) {
		t.Errorf("log = %q, leaked a secret", line)
	}
	if !strings.Contains(line, `"email":"a@b.c"`) || !strings.Contains(line, `"password":"[REDACTED]"`) {
		t.Errorf("log = %q, want the email kept and the password redacted", line)
	}
}

// A body that is not a JSON object cannot be redacted field by field, so it
// is not logged at all.
func TestLogErrorSkipsUnparseableBodies(t *testing.T) {
	for _, body := range []string{`password=hunter22`, `{"password":"hunter22"` + strings.Repeat(" ", maxLoggedBody)} {
		line := logFailure(t, body)
		if strings.Contains(line, "hunter22") || !strings.Contains(line, "bytes, not logged>") {
			t.Errorf("log = %q, want the body left out", line)
		}
	}
}

func TestLogErrorWithoutBody(t *testing.T) {
	if line := logFailure(t, ""); strings.Contains(line, "payload=") {
		t.Errorf("log = %q, want no payload for an empty body", line)
	}
}
