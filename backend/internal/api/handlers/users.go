package handlers

import (
	"ato-wfh-diary/internal/api/middleware"
	"ato-wfh-diary/internal/model"
	"encoding/json"
	"net/http"
	"strings"
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

// CreateUser manually creates a new user. Returns 201 on success, 409 if the
// username already exists, or 400 if the request body is invalid.
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.DisplayName = strings.TrimSpace(req.DisplayName)

	if req.Username == "" || req.DisplayName == "" {
		respondError(w, http.StatusBadRequest, "username and display_name are required")
		return
	}

	existing, err := h.Store.GetUserByUsername(r.Context(), req.Username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not check user")
		return
	}
	if existing != nil {
		respondError(w, http.StatusConflict, "username already exists")
		return
	}

	user, err := h.Store.UpsertUser(r.Context(), req.Username, req.DisplayName)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not create user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
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
