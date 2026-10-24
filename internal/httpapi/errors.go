package httpapi

import (
	"encoding/json"
	"net/http"
)

// errorBody is the one error shape every API response uses:
// {"error": {"code": "...", "message": "..."}}.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeError writes an API error. code is a stable, machine-readable
// identifier (e.g. "not_found"); message is for humans.
func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: message}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The status is already sent, so an encoding error can't be reported to
	// the client; the values passed here are plain structs that always encode.
	_ = json.NewEncoder(w).Encode(v)
}
