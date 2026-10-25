package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
)

const sessionCookie = "watcht_session"

// Login rate limit, per client IP (per /64 for IPv6): 10 attempts, then one
// more a minute. Successful logins don't count.
const (
	loginBurst = 10
	loginEvery = time.Minute
)

// loginWait bounds a whole login, which is mostly waiting for a free
// password-hashing slot (two run at once). Under a flood, requests fail
// with 503 instead of queueing without limit.
const loginWait = 10 * time.Second

type authHandlers struct {
	auth    *auth.Service
	limiter *auth.Limiter
	proxies proxies
	// secureCookies is set when WATCHT_BASE_URL is https, so the cookie is
	// Secure even if the proxy doesn't send X-Forwarded-Proto.
	secureCookies bool
	loginWait     time.Duration
	logger        *slog.Logger

	warnProxyOnce sync.Once
}

func (h *authHandlers) register(api *apiMux) {
	api.handle("GET /api/v1/auth/session", h.session)
	api.handle("POST /api/v1/auth/setup", h.setup)
	api.handle("POST /api/v1/auth/login", h.login)
	api.handle("POST /api/v1/auth/logout", h.logout)
}

type userJSON struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

func toUserJSON(u auth.User) *userJSON {
	return &userJSON{ID: u.ID, Username: u.Username, IsAdmin: u.IsAdmin}
}

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// session reports whether setup is needed and who is signed in. It never
// returns 401, so the SPA can call it on startup to decide where to go.
func (h *authHandlers) session(w http.ResponseWriter, r *http.Request) {
	needsSetup, err := h.auth.NeedsSetup(r.Context())
	if err != nil {
		h.internalError(w, "check setup", err)
		return
	}
	resp := struct {
		SetupRequired bool      `json:"setup_required"`
		User          *userJSON `json:"user"`
	}{SetupRequired: needsSetup}
	sess, ok, err := h.currentSession(w, r)
	if err != nil {
		h.internalError(w, "authenticate", err)
		return
	}
	if ok {
		resp.User = toUserJSON(sess.User)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *authHandlers) setup(w http.ResponseWriter, r *http.Request) {
	var c credentials
	if !decodeJSON(w, r, &c) {
		return
	}
	sess, err := h.auth.Setup(r.Context(), c.Username, c.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
		return
	case errors.Is(err, auth.ErrSetupDone):
		writeError(w, http.StatusConflict, "setup_done", "setup is already complete; sign in instead")
		return
	case err != nil:
		h.internalError(w, "setup", err)
		return
	}
	h.setSessionCookie(w, r, sess.Token, sess.ExpiresAt)
	writeJSON(w, http.StatusCreated, struct {
		User *userJSON `json:"user"`
	}{toUserJSON(sess.User)})
}

func (h *authHandlers) login(w http.ResponseWriter, r *http.Request) {
	if h.proxies.forwardedFromUntrusted(r) {
		// Every client then shares the proxy's limit, so one person guessing
		// passwords locks everyone out.
		h.warnProxyOnce.Do(func() {
			h.logger.Warn("login request has X-Forwarded-For from an address that isn't a trusted proxy; if watcht is behind a reverse proxy, set WATCHT_TRUSTED_PROXIES to its address", "peer", remoteAddr(r).String())
		})
	}
	key := rateLimitKey(h.proxies.clientIP(r))
	ok, retry := h.limiter.Take(key)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many sign-in attempts; try again later")
		return
	}
	var c credentials
	if !decodeJSON(w, r, &c) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), h.loginWait)
	defer cancel()
	sess, err := h.auth.Login(ctx, c.Username, c.Password)
	switch {
	case r.Context().Err() != nil:
		// The client went away; there's no one to answer.
		h.limiter.Refund(key)
		return
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "invalid_credentials", "wrong username or password")
		return
	case errors.Is(err, context.DeadlineExceeded):
		h.limiter.Refund(key) // not the client's fault
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusServiceUnavailable, "busy", "the server is busy; try again in a few seconds")
		return
	case err != nil:
		h.internalError(w, "login", err)
		return
	}
	h.limiter.Refund(key)

	// Signing in again replaces the session this browser already had.
	if old, err := r.Cookie(sessionCookie); err == nil {
		if err := h.auth.Logout(r.Context(), old.Value); err != nil {
			h.logger.Warn("end previous session at login", "err", err)
		}
	}
	h.setSessionCookie(w, r, sess.Token, sess.ExpiresAt)
	writeJSON(w, http.StatusOK, struct {
		User *userJSON `json:"user"`
	}{toUserJSON(sess.User)})
}

func (h *authHandlers) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if err := h.auth.Logout(r.Context(), c.Value); err != nil {
			h.internalError(w, "logout", err)
			return
		}
	}
	h.clearSessionCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

// currentSession returns the request's session, if its cookie holds a
// valid one. It refreshes the cookie when the session was renewed, and
// clears a cookie that no longer matches a session.
func (h *authHandlers) currentSession(w http.ResponseWriter, r *http.Request) (auth.Session, bool, error) {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return auth.Session{}, false, nil
	}
	sess, err := h.auth.Authenticate(r.Context(), c.Value)
	if errors.Is(err, auth.ErrNoSession) {
		h.clearSessionCookie(w, r)
		return auth.Session{}, false, nil
	}
	if err != nil {
		return auth.Session{}, false, err
	}
	if sess.Renewed {
		h.setSessionCookie(w, r, c.Value, sess.ExpiresAt)
	}
	return sess, true, nil
}

func (h *authHandlers) setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expires time.Time) {
	// Secure can't always be set: many installs are plain HTTP on a LAN,
	// where a Secure cookie would never be sent back.
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: Secure is set whenever the request came over HTTPS
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies || h.proxies.isHTTPS(r),
	})
}

func (h *authHandlers) clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{ //nolint:gosec // G124: as in setSessionCookie
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.secureCookies || h.proxies.isHTTPS(r),
	})
}

// internalError logs err and sends a generic 500; the cause isn't shown to
// the client.
func (h *authHandlers) internalError(w http.ResponseWriter, op string, err error) {
	h.logger.Error(op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal", "something went wrong; see the server log")
}
