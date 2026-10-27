package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
)

func (h *authHandlers) registerTokens(api *apiMux) {
	api.handle("GET /api/v1/me", h.withUser(h.me))
	api.handle("GET /api/v1/tokens", h.withSession(h.listTokens))
	api.handle("POST /api/v1/tokens", h.withSession(h.createToken))
	api.handle("DELETE /api/v1/tokens/{id}", h.withSession(h.revokeToken))
}

// userHandler is a handler for the data API, reached with a session
// cookie or an API token.
type userHandler func(http.ResponseWriter, *http.Request, auth.User)

// withUser authenticates a data API request: with a watcht token
// (Authorization: Bearer wt_...), by that token only, so a bad one is 401,
// never a fallback to the cookie; otherwise by session cookie.
func (h *authHandlers) withUser(next userHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if token, ok := apiToken(r); ok {
			u, err := h.auth.AuthenticateAPIToken(r.Context(), token)
			if errors.Is(err, auth.ErrNoToken) {
				w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
				writeError(w, http.StatusUnauthorized, "unauthenticated", "the API token is invalid or revoked")
				return
			}
			if err != nil {
				h.internalError(w, "authenticate API token", err)
				return
			}
			next(w, r, u)
			return
		}
		sess, ok, err := h.currentSession(w, r)
		if err != nil {
			h.internalError(w, "authenticate", err)
			return
		}
		if !ok {
			w.Header().Set("WWW-Authenticate", `Bearer`)
			writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in first, or send an API token")
			return
		}
		next(w, r, sess.User)
	}
}

// apiToken returns the watcht API token in the request's Authorization
// header ("Bearer wt_...", the scheme case-insensitive per RFC 9110), if
// there is one. Only such a header makes a request a token request. Any
// other Authorization header is ignored and the session cookie decides:
// a reverse proxy's HTTP Basic auth, or an oauth2-proxy Bearer JWT, comes
// with every request from the browser (maintainer decision, 2026-10-26).
func apiToken(r *http.Request) (string, bool) {
	scheme, token, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}
	token = strings.TrimSpace(token)
	return token, strings.HasPrefix(token, "wt_")
}

// me returns the authenticated user, e.g. for a script checking its token.
func (h *authHandlers) me(w http.ResponseWriter, _ *http.Request, u auth.User) {
	writeJSON(w, http.StatusOK, struct {
		User *userJSON `json:"user"`
	}{toUserJSON(u)})
}

type tokenJSON struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

func toTokenJSON(t auth.APIToken) tokenJSON {
	return tokenJSON{ID: t.ID, Name: t.Name, CreatedAt: t.CreatedAt.UTC().Format(time.RFC3339)}
}

func (h *authHandlers) listTokens(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	tokens, err := h.auth.ListAPITokens(r.Context(), sess)
	if err != nil {
		h.tokenError(w, r, "list API tokens", err)
		return
	}
	out := make([]tokenJSON, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, toTokenJSON(t))
	}
	writeJSON(w, http.StatusOK, struct {
		Tokens []tokenJSON `json:"tokens"`
	}{out})
}

// createToken returns the new token's secret. It's the only time the
// secret is ever shown; watcht stores only its hash. It needs the current
// password, which is a password guess, so it shares the per-account limit
// with password changes.
func (h *authHandlers) createToken(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	key, ok := h.takeAccountAttempt(w, sess)
	if !ok {
		return
	}
	var req struct {
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginWait)
	defer cancel()
	tok, secret, err := h.auth.CreateAPIToken(ctx, sess, req.Password, req.Name)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusBadRequest, "invalid_credentials", "the password is wrong")
		return
	}
	if err != nil {
		if r.Context().Err() == nil {
			h.limiter.Refund(key)
		}
		h.tokenError(w, r, "create API token", err)
		return
	}
	h.limiter.Refund(key)
	writeJSON(w, http.StatusCreated, struct {
		Token  tokenJSON `json:"token"`
		Secret string    `json:"secret"`
	}{toTokenJSON(tok), secret})
}

func (h *authHandlers) revokeToken(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	id, ok := pathID(w, r, "no such token")
	if !ok {
		return
	}
	if err := h.auth.RevokeAPIToken(r.Context(), sess, id); err != nil {
		h.tokenError(w, r, "revoke API token", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) tokenError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidTokenName):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, auth.ErrTokenNameTaken):
		writeError(w, http.StatusConflict, "name_taken", "you already have a token with that name")
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "no such token")
	default:
		h.accountError(w, r, op, err)
	}
}
