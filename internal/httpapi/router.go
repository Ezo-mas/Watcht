// Package httpapi wires the HTTP routes: the JSON API under /api/v1, the
// health check, and the embedded single-page app for everything else.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

// Deps are the dependencies the router needs. Fields are added as
// milestones add domain packages.
type Deps struct {
	Logger *slog.Logger
	// WebUI serves the single-page app for every path that isn't an API
	// route or the health check.
	WebUI http.Handler
	// Health reports whether the server's dependencies (the database) work.
	// Nil means always healthy.
	Health func(context.Context) error
}

const healthTimeout = 2 * time.Second

// NewRouter returns the root handler for the server.
func NewRouter(d Deps) http.Handler {
	mux := http.NewServeMux()

	// Registered without a method so other methods get 405 here instead of
	// falling through to the SPA's catch-all route.
	mux.HandleFunc("/healthz", healthzHandler(d.Logger, d.Health))

	// Unknown API paths get a JSON 404 instead of falling through to the
	// SPA, so API clients never receive index.html. Because this pattern
	// matches every /api/ path, the mux never produces its own 405 for API
	// routes: see "Adding an endpoint" in docs/agent-guides/backend.md.
	mux.HandleFunc("/api/", func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusNotFound, "not_found", "no such API endpoint")
	})

	mux.Handle("/", d.WebUI)

	return logRequests(d.Logger, mux)
}

func healthzHandler(logger *slog.Logger, health func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "use GET")
			return
		}
		if health != nil {
			ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
			defer cancel()
			if err := health(ctx); err != nil {
				// The cause goes to the log, not to the unauthenticated client.
				logger.Error("health check failed", "err", err)
				writeError(w, http.StatusServiceUnavailable, "unhealthy", "a dependency is not working; see the server log")
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

// logRequests logs one line per request. It logs the path only, never the
// query string or headers, so tokens passed in either can't leak into logs.
func logRequests(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		if r.URL.Path == "/healthz" && rec.status == http.StatusOK {
			// Container health checks hit this every few seconds; only
			// failures are worth a log line.
			return
		}
		logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).Round(time.Microsecond),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
