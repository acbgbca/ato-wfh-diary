package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// respondJSON writes v as a JSON response with a 200 status code.
func respondJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// respondError writes a JSON error body with the given status code.
func respondError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// pathUserID extracts and validates the {id} path parameter.
func pathUserID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// queryInt reads a named query parameter as an int. Returns (0, false) if
// absent or not a valid integer.
func queryInt(r *http.Request, key string) (int, bool) {
	s := r.URL.Query().Get(key)
	if s == "" {
		return 0, false
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return v, true
}
