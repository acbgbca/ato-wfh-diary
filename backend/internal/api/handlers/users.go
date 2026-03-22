package handlers

import (
	"ato-wfh-diary/internal/api/middleware"
	"ato-wfh-diary/internal/model"
	"net/http"
)

// GetUsers returns all users, ordered by display name.
// Used by the frontend to populate the "view entries for" selector.
func (h *Handler) GetUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.GetUsers(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve users")
		return
	}
	if users == nil {
		users = []model.User{}
	}
	respondJSON(w, users)
}

// GetMe returns the currently authenticated user, creating their record on
// first login. The username is taken from the Forward Auth request context.
func (h *Handler) GetMe(w http.ResponseWriter, r *http.Request) {
	username := middleware.UsernameFromContext(r.Context())
	user, err := h.Store.GetOrCreateUser(r.Context(), username, username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}
	respondJSON(w, user)
}
