package handlers

import (
	"encoding/json"
	"net/http"
)

// GetWeekEntries returns all entries for a given user and week.
// Query params: user_id, week_start (YYYY-MM-DD).
func (h *Handler) GetWeekEntries(w http.ResponseWriter, r *http.Request) {
	// TODO: parse user_id and week_start from query params
	// TODO: query entries for the 7 days of that week
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]any{})
}

// UpsertWeekEntries creates or updates entries for a given user and week.
func (h *Handler) UpsertWeekEntries(w http.ResponseWriter, r *http.Request) {
	// TODO: decode request body (array of day entries)
	// TODO: upsert each entry (INSERT OR REPLACE)
	w.WriteHeader(http.StatusNoContent)
}
