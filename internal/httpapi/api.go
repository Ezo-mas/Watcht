package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

// apiMux serves the /api/ routes from its own ServeMux and replaces the
// mux's plain-text 404 and 405 responses with the standard error shape, so
// API clients always get JSON, and a wrong method gets 405 with Allow.
type apiMux struct {
	mux *http.ServeMux
}

func newAPIMux() *apiMux { return &apiMux{mux: http.NewServeMux()} }

// handle registers a route, e.g. "POST /api/v1/auth/login".
func (a *apiMux) handle(pattern string, h http.HandlerFunc) { a.mux.HandleFunc(pattern, h) }

func (a *apiMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h, pattern := a.mux.Handler(r)
	if pattern != "" {
		a.mux.ServeHTTP(w, r)
		return
	}
	// No route matched: the mux's handler is its own 404 or 405 (or a
	// redirect to a cleaned path). Run it against a recorder to learn which,
	// then answer in JSON.
	rec := &discardRecorder{header: http.Header{}, status: http.StatusOK}
	h.ServeHTTP(rec, r)
	switch rec.status {
	case http.StatusMethodNotAllowed:
		w.Header().Set("Allow", rec.header.Get("Allow"))
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "this endpoint doesn't support "+r.Method)
	default:
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	}
}

// discardRecorder records a response's status and headers and drops the
// body.
type discardRecorder struct {
	header http.Header
	status int
}

func (d *discardRecorder) Header() http.Header         { return d.header }
func (d *discardRecorder) Write(b []byte) (int, error) { return len(b), nil }
func (d *discardRecorder) WriteHeader(status int)      { d.status = status }

// maxBodyBytes bounds JSON request bodies. Everything the API accepts so
// far is a few short fields.
const maxBodyBytes = 64 << 10

// decodeJSON decodes a JSON request body into v, rejecting other content
// types, unknown fields, trailing data, and oversized bodies. On failure it
// writes the error response and returns false.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "send a JSON body with Content-Type: application/json")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	dec.DisallowUnknownFields()
	err = dec.Decode(v)
	if err == nil {
		if _, extra := dec.Token(); !errors.Is(extra, io.EOF) {
			err = errors.New("unexpected data after the JSON value")
		}
	}
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			writeError(w, http.StatusRequestEntityTooLarge, "too_large", "the request body is too large")
			return false
		}
		writeError(w, http.StatusBadRequest, "invalid_json", "the request body isn't valid JSON for this endpoint: "+err.Error())
		return false
	}
	return true
}
