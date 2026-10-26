package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ezo-mas/watcht/internal/auth"
)

func (h *authHandlers) registerAccounts(api *apiMux) {
	api.handle("PUT /api/v1/me/password", h.withSession(h.changePassword))
	api.handle("GET /api/v1/users", h.withAdmin(h.listUsers))
	api.handle("POST /api/v1/users", h.withAdmin(h.createUser))
	api.handle("DELETE /api/v1/users/{id}", h.withAdmin(h.deleteUser))
	api.handle("PUT /api/v1/users/{id}/password", h.withAdmin(h.resetPassword))
}

// sessionHandler is a handler for a signed-in browser session.
type sessionHandler func(http.ResponseWriter, *http.Request, auth.Session)

// withSession requires a valid session cookie: 401 without one. Account
// management needs a browser session, so an API token (slice 4) never
// gets here, even for an admin.
func (h *authHandlers) withSession(next sessionHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess, ok, err := h.currentSession(w, r)
		if err != nil {
			h.internalError(w, "authenticate", err)
			return
		}
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in first")
			return
		}
		next(w, r, sess)
	}
}

// withAdmin requires a session of an admin account: 403 for other users.
// The admin flag is read from the database on every request, so a change
// applies at once.
func (h *authHandlers) withAdmin(next sessionHandler) http.HandlerFunc {
	return h.withSession(func(w http.ResponseWriter, r *http.Request, sess auth.Session) {
		if !sess.User.IsAdmin {
			writeError(w, http.StatusForbidden, "forbidden", "only admins can manage accounts")
			return
		}
		next(w, r, sess)
	})
}

type accountJSON struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
}

func toAccountJSON(a auth.Account) accountJSON {
	return accountJSON{ID: a.ID, Username: a.Username, IsAdmin: a.IsAdmin, CreatedAt: a.CreatedAt.UTC().Format(time.RFC3339)}
}

func (h *authHandlers) listUsers(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	accounts, err := h.auth.ListAccounts(r.Context(), sess)
	if err != nil {
		h.accountError(w, r, "list users", err)
		return
	}
	users := make([]accountJSON, 0, len(accounts))
	for _, a := range accounts {
		users = append(users, toAccountJSON(a))
	}
	writeJSON(w, http.StatusOK, struct {
		Users []accountJSON `json:"users"`
	}{users})
}

func (h *authHandlers) createUser(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		IsAdmin  bool   `json:"is_admin"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginWait)
	defer cancel()
	a, err := h.auth.CreateAccount(ctx, sess, req.Username, req.Password, req.IsAdmin)
	if err != nil {
		h.accountError(w, r, "create user", err)
		return
	}
	writeJSON(w, http.StatusCreated, struct {
		User accountJSON `json:"user"`
	}{toAccountJSON(a)})
}

func (h *authHandlers) deleteUser(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.auth.DeleteAccount(r.Context(), sess, id); err != nil {
		h.accountError(w, r, "delete user", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *authHandlers) resetPassword(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var req struct {
		Password string `json:"password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginWait)
	defer cancel()
	if err := h.auth.ResetPassword(ctx, sess, id, req.Password); err != nil {
		h.accountError(w, r, "reset password", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// changePassword changes the caller's own password. Checking the current
// password is a password guess, so it shares the login limiter, keyed by
// account: someone at an unlocked browser can't use it to brute-force.
func (h *authHandlers) changePassword(w http.ResponseWriter, r *http.Request, sess auth.Session) {
	key := "account:" + strconv.FormatInt(sess.User.ID, 10)
	ok, retry := h.limiter.Take(key)
	if !ok {
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())))
		writeError(w, http.StatusTooManyRequests, "rate_limited", "too many attempts; try again later")
		return
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginWait)
	defer cancel()
	err := h.auth.ChangePassword(ctx, sess, req.CurrentPassword, req.NewPassword)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		// 400, not 401: the session is fine, and the SPA treats 401 as
		// "signed out".
		writeError(w, http.StatusBadRequest, "invalid_credentials", "the current password is wrong")
		return
	}
	if err != nil {
		// No guess was judged (bad new password, busy, session gone), so
		// the attempt doesn't count. A client that left mid-request isn't
		// refunded: it may have been checking a guess.
		if r.Context().Err() == nil {
			h.limiter.Refund(key)
		}
		h.accountError(w, r, "change password", err)
		return
	}
	h.limiter.Refund(key)
	w.WriteHeader(http.StatusNoContent)
}

// accountError maps account-management errors to responses.
func (h *authHandlers) accountError(w http.ResponseWriter, r *http.Request, op string, err error) {
	switch {
	case r.Context().Err() != nil:
		// The client went away; there's no one to answer.
	case errors.Is(err, context.DeadlineExceeded):
		w.Header().Set("Retry-After", "5")
		writeError(w, http.StatusServiceUnavailable, "busy", "the server is busy; try again in a few seconds")
	case errors.Is(err, auth.ErrNoSession):
		h.clearSessionCookie(w, r)
		writeError(w, http.StatusUnauthorized, "unauthenticated", "your session has ended; sign in again")
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrInvalidPassword):
		writeError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, auth.ErrUsernameTaken):
		writeError(w, http.StatusConflict, "username_taken", "another account already has that username")
	case errors.Is(err, auth.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "no such account")
	case errors.Is(err, auth.ErrSelf):
		writeError(w, http.StatusBadRequest, "own_account", "you can't do that to your own account here")
	case errors.Is(err, auth.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "only admins can manage accounts")
	default:
		h.internalError(w, op, err)
	}
}

// pathID parses the {id} path segment. Anything but a positive integer is
// a 404, like an ID that doesn't exist.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	s := r.PathValue("id")
	id, err := strconv.ParseInt(s, 10, 64)
	if err != nil || id <= 0 || strings.HasPrefix(s, "+") {
		writeError(w, http.StatusNotFound, "not_found", "no such account")
		return 0, false
	}
	return id, true
}
