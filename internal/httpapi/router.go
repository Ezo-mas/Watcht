// Package httpapi wires the HTTP routes: the JSON API under /api/v1, the
// health check, and the embedded single-page app for everything else.
package httpapi

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
)

// Deps are the dependencies the router needs. Fields are added as
// milestones add domain packages.
type Deps struct {
	Logger *slog.Logger
	// Catalog and Search are nil when TMDB is not configured.
	Catalog Catalog
	Search  CatalogSearch
	// WebUI serves the single-page app for every path that isn't an API
	// route or the health check.
	WebUI http.Handler
	// Health reports whether the server's dependencies (the database) work.
	// Nil means always healthy.
	Health func(context.Context) error
	// Auth handles setup, sign-in, and sessions. Required.
	Auth *auth.Service
	// LoginLimiter limits login attempts per client. Nil means the default
	// (10 attempts, then one a minute).
	LoginLimiter *auth.Limiter
	// BaseOrigin is WATCHT_BASE_URL's scheme://host, or "". Requests from
	// it pass the cross-origin check, and an https origin makes the session
	// cookie Secure.
	BaseOrigin string
	// TrustedProxies are the reverse proxies whose X-Forwarded-For and
	// X-Forwarded-Proto headers are believed (WATCHT_TRUSTED_PROXIES).
	TrustedProxies []netip.Prefix

	// loginWait overrides the login deadline, for tests.
	loginWait time.Duration
}

const healthTimeout = 2 * time.Second

// NewRouter returns the root handler for the server.
func NewRouter(d Deps) (http.Handler, error) {
	if d.Auth == nil {
		return nil, errors.New("httpapi: Deps.Auth is required")
	}
	limiter := d.LoginLimiter
	if limiter == nil {
		limiter = auth.NewLimiter(loginBurst, loginEvery, nil)
	}
	api := newAPIMux()
	authAPI := &authHandlers{
		auth:          d.Auth,
		limiter:       limiter,
		proxies:       proxies(d.TrustedProxies),
		secureCookies: strings.HasPrefix(d.BaseOrigin, "https://"),
		loginWait:     cmp.Or(d.loginWait, loginWait),
		logger:        d.Logger,
	}
	authAPI.register(api)
	authAPI.registerAccounts(api)
	authAPI.registerTokens(api)
	(&catalogHandlers{auth: authAPI, catalog: d.Catalog, search: d.Search}).register(api)

	mux := http.NewServeMux()

	// Registered without a method so other methods get 405 here instead of
	// falling through to the SPA's catch-all route.
	mux.HandleFunc("/healthz", healthzHandler(d.Logger, d.Health))

	// Every /api/ path goes to the API mux, so API clients never receive
	// index.html: unknown paths get a JSON 404, wrong methods a JSON 405.
	mux.Handle("/api/", api)

	mux.Handle("/", d.WebUI)

	csrf, err := crossOriginProtection(d.BaseOrigin)
	if err != nil {
		return nil, err
	}
	return logRequests(d.Logger, csrf.Handler(mux)), nil
}

// crossOriginProtection rejects cross-site browser requests that change
// state (anything but GET, HEAD, and OPTIONS), which is the CSRF defense
// alongside SameSite=Lax cookies. It trusts Sec-Fetch-Site, falling back to
// comparing Origin with Host for older browsers, plus baseOrigin for
// proxies that rewrite Host. Requests with neither header aren't from a
// browser and pass.
func crossOriginProtection(baseOrigin string) (*http.CrossOriginProtection, error) {
	c := http.NewCrossOriginProtection()
	if baseOrigin != "" {
		if err := c.AddTrustedOrigin(baseOrigin); err != nil {
			return nil, fmt.Errorf("WATCHT_BASE_URL: %w", err)
		}
	}
	c.SetDenyHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeError(w, http.StatusForbidden, "cross_origin", "cross-origin request blocked")
	}))
	return c, nil
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
