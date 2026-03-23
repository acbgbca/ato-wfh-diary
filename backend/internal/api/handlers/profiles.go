package handlers

import (
	"ato-wfh-diary/internal/api/middleware"
	"ato-wfh-diary/internal/model"
	"encoding/json"
	"net/http"
)

// profileRequest is the JSON shape accepted when upserting a profile.
type profileRequest struct {
	DefaultHours float64       `json:"default_hours"`
	MonType      model.DayType `json:"mon_type"`
	TueType      model.DayType `json:"tue_type"`
	WedType      model.DayType `json:"wed_type"`
	ThuType      model.DayType `json:"thu_type"`
	FriType      model.DayType `json:"fri_type"`
	SatType      model.DayType `json:"sat_type"`
	SunType      model.DayType `json:"sun_type"`
}

// GetProfile returns the current user's profile, or 404 if not yet configured.
func (h *Handler) GetProfile(w http.ResponseWriter, r *http.Request) {
	username := middleware.UsernameFromContext(r.Context())
	user, err := h.Store.GetUserByUsername(r.Context(), username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	profile, err := h.Store.GetProfile(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve profile")
		return
	}
	if profile == nil {
		respondError(w, http.StatusNotFound, "profile not configured")
		return
	}
	respondJSON(w, profile)
}

// UpsertProfile creates or updates the current user's profile.
func (h *Handler) UpsertProfile(w http.ResponseWriter, r *http.Request) {
	username := middleware.UsernameFromContext(r.Context())
	user, err := h.Store.GetOrCreateUser(r.Context(), username, username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}

	var req profileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.DefaultHours <= 0 || req.DefaultHours > 24 {
		respondError(w, http.StatusBadRequest, "default_hours must be between 0 and 24")
		return
	}

	dayTypes := []model.DayType{req.MonType, req.TueType, req.WedType, req.ThuType, req.FriType, req.SatType, req.SunType}
	dayNames := []string{"mon_type", "tue_type", "wed_type", "thu_type", "fri_type", "sat_type", "sun_type"}
	for i, dt := range dayTypes {
		if !dt.IsValid() {
			respondError(w, http.StatusBadRequest, dayNames[i]+" has invalid day type")
			return
		}
	}

	profile := model.UserProfile{
		UserID:       user.ID,
		DefaultHours: req.DefaultHours,
		MonType:      req.MonType,
		TueType:      req.TueType,
		WedType:      req.WedType,
		ThuType:      req.ThuType,
		FriType:      req.FriType,
		SatType:      req.SatType,
		SunType:      req.SunType,
	}
	if err := h.Store.UpsertProfile(r.Context(), profile); err != nil {
		respondError(w, http.StatusInternalServerError, "could not save profile")
		return
	}

	saved, err := h.Store.GetProfile(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve saved profile")
		return
	}
	respondJSON(w, saved)
}
