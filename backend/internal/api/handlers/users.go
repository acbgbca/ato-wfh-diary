package handlers

import (
	"encoding/json"
	"net/http"
)

// GetUsers returns all users. Used to populate the "view entries for" selector.
func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	// TODO: query all users from db
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]any{})
}
