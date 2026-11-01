package providers

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestUnavailable(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil"},
		{name: "plain", err: errors.New("internal error")},
		{name: "caller cancelled", err: context.Canceled},
		{name: "caller deadline", err: context.DeadlineExceeded},
		{name: "bad response", err: ErrBadResponse, want: true},
		{name: "EOF", err: &transportError{err: io.EOF}, want: true},
		{name: "unexpected EOF", err: &transportError{err: io.ErrUnexpectedEOF}, want: true},
		{name: "TLS", err: &transportError{err: x509.UnknownAuthorityError{}}, want: true},
		{name: "attempt timeout", err: &transportError{err: context.DeadlineExceeded}, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := Unavailable(tc.err); got != tc.want {
				t.Fatalf("Unavailable(%v) = %v, want %v", tc.err, got, tc.want)
			}
			if tc.err != nil {
				if got := Unavailable(fmt.Errorf("adapter: %w", tc.err)); got != tc.want {
					t.Fatalf("wrapped Unavailable = %v, want %v", got, tc.want)
				}
			}
		})
	}
	for _, status := range []int{http.StatusMovedPermanently, http.StatusBadRequest, http.StatusNotFound, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusServiceUnavailable} {
		if !Unavailable(fmt.Errorf("adapter: %w", &StatusError{StatusCode: status})) {
			t.Errorf("status %d is not unavailable", status)
		}
	}
}

func TestUnavailableBadResponse(t *testing.T) {
	for _, tc := range []struct{ name, body, message string }{
		{name: "HTML", body: "<html>private</html>", message: "response is not valid JSON"},
		{name: "empty", message: "response is not valid JSON"},
		{name: "field type", body: `{"name":123}`, message: "response field name is not a string"},
		{name: "root type", body: `[]`, message: "response is not a providers.payload"},
		{name: "too large", body: strings.Repeat("x", maxBodyBytes+1), message: fmt.Sprintf("response is larger than %d bytes", maxBodyBytes)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &script{replies: []reply{{status: http.StatusOK, body: tc.body}}}
			e := newEnv(t, s, nil)
			_, err := e.get("/body", nil, 0)
			if !Unavailable(err) || !errors.Is(err, ErrBadResponse) {
				t.Fatalf("bad response classification: %v", err)
			}
			if want := "test: GET /body: " + tc.message; err.Error() != want {
				t.Fatalf("error = %q, want unchanged %q", err, want)
			}
		})
	}
	// Invalid destination errors use the generic shape message, not decoder text.
	e := newEnv(t, okScript("test"), nil)
	var out payload
	err := e.client.GetJSON(context.Background(), "/shape", nil, 0, out)
	if !errors.Is(err, ErrBadResponse) || !Unavailable(err) || err.Error() != "test: GET /shape: response doesn't match the expected shape" {
		t.Fatalf("decode shape: %v", err)
	}
}

func TestUnavailableCallerContext(t *testing.T) {
	e := newEnv(t, okScript("test"), nil)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer stop()
	for _, ctx := range []context.Context{cancelled, expired} {
		var out payload
		err := e.client.GetJSON(ctx, "/context", nil, 0, &out)
		if !errors.Is(err, ctx.Err()) || Unavailable(err) {
			t.Fatalf("caller context classified as provider failure: %v", err)
		}
	}
}
