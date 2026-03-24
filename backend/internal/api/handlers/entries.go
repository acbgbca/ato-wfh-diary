package handlers

import (
	"ato-wfh-diary/internal/model"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GetFirstIncompleteWeek returns the Monday of the first week (from from_date)
// with fewer than 7 entries for the user in the given financial year.
//
// Query params:
//   - financial_year (optional): defaults to current FY
//   - from_date (optional, YYYY-MM-DD Monday): start search from this week;
//     defaults to first Monday ≥ July 1 of the FY
func (h *Handler) GetFirstIncompleteWeek(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathUserID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	today := time.Now().UTC()

	financialYear, hasFY := queryInt(r, "financial_year")
	if !hasFY {
		financialYear = model.FinancialYear(today)
	}

	var fromDate time.Time
	if fromDateStr := r.URL.Query().Get("from_date"); fromDateStr != "" {
		fromDate, err = time.Parse("2006-01-02", fromDateStr)
		if err != nil {
			respondError(w, http.StatusBadRequest, "from_date must be in YYYY-MM-DD format")
			return
		}
	} else {
		fromDate = firstMondayOfFY(financialYear)
	}

	result, err := h.Store.GetFirstIncompleteWeek(r.Context(), userID, financialYear, fromDate, today)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not determine first incomplete week")
		return
	}

	var weekStart *string
	if result != nil {
		s := result.Format("2006-01-02")
		weekStart = &s
	}
	respondJSON(w, map[string]any{"week_start": weekStart})
}

// firstMondayOfFY returns the Monday of the week containing July 1 of the
// financial year that starts in (financialYear - 1).
// e.g. FY2026 → Jul 1 2025 is a Tuesday → returns Mon 30 Jun 2025.
func firstMondayOfFY(financialYear int) time.Time {
	jul1 := time.Date(financialYear-1, time.July, 1, 0, 0, 0, 0, time.UTC)
	dow := int(jul1.Weekday())
	if dow == 0 {
		dow = 7
	}
	return jul1.AddDate(0, 0, 1-dow)
}

// entryRequest is the JSON shape accepted when upserting entries.
type entryRequest struct {
	EntryDate string        `json:"entry_date"` // YYYY-MM-DD
	DayType   model.DayType `json:"day_type"`
	Hours     float64       `json:"hours"`
	Notes     string        `json:"notes,omitempty"`
}

// entryResponse is the JSON shape returned for a work day entry.
// EntryDate is formatted as YYYY-MM-DD rather than RFC3339.
type entryResponse struct {
	ID            int64         `json:"id"`
	UserID        int64         `json:"user_id"`
	EntryDate     string        `json:"entry_date"`
	FinancialYear int           `json:"financial_year"`
	DayType       model.DayType `json:"day_type"`
	Hours         float64       `json:"hours"`
	Notes         string        `json:"notes,omitempty"`
}

func toEntryResponse(e model.WorkDayEntry) entryResponse {
	return entryResponse{
		ID:            e.ID,
		UserID:        e.UserID,
		EntryDate:     e.EntryDate.Format("2006-01-02"),
		FinancialYear: e.FinancialYear,
		DayType:       e.DayType,
		Hours:         e.Hours,
		Notes:         e.Notes,
	}
}

// GetWeekEntries returns all entries for a user in the 7-day window starting
// on week_start.
//
// Query params:
//   - week_start (required): Monday date in YYYY-MM-DD format
func (h *Handler) GetWeekEntries(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathUserID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	weekStartStr := r.URL.Query().Get("week_start")
	if weekStartStr == "" {
		respondError(w, http.StatusBadRequest, "week_start is required (YYYY-MM-DD)")
		return
	}
	weekStart, err := time.Parse("2006-01-02", weekStartStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "week_start must be in YYYY-MM-DD format")
		return
	}

	user, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	entries, err := h.Store.GetWeekEntries(r.Context(), userID, weekStart)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve entries")
		return
	}

	resp := make([]entryResponse, len(entries))
	for i, e := range entries {
		resp[i] = toEntryResponse(e)
	}
	respondJSON(w, resp)
}

// UpsertWeekEntries creates or updates a batch of day entries for a user.
// The request body is a JSON array of entryRequest objects.
func (h *Handler) UpsertWeekEntries(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathUserID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	user, err := h.Store.GetUserByID(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}
	if user == nil {
		respondError(w, http.StatusNotFound, "user not found")
		return
	}

	var reqs []entryRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(reqs) == 0 {
		respondError(w, http.StatusBadRequest, "request body must contain at least one entry")
		return
	}

	entries := make([]model.WorkDayEntry, 0, len(reqs))
	for i, req := range reqs {
		if !req.DayType.IsValid() {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("entry %d: invalid day_type %q", i, req.DayType))
			return
		}
		entryDate, err := time.Parse("2006-01-02", req.EntryDate)
		if err != nil {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("entry %d: entry_date must be in YYYY-MM-DD format", i))
			return
		}
		hours := req.Hours
		if !req.DayType.IsWFH() {
			hours = 0 // coerce non-WFH days to zero
		} else if hours <= 0 || hours > 24 {
			respondError(w, http.StatusBadRequest, fmt.Sprintf("entry %d: hours must be between 0 and 24 for WFH day types", i))
			return
		}
		entries = append(entries, model.WorkDayEntry{
			UserID:    userID,
			EntryDate: entryDate,
			DayType:   req.DayType,
			Hours:     hours,
			Notes:     req.Notes,
		})
	}

	if err := h.Store.UpsertEntries(r.Context(), entries); err != nil {
		respondError(w, http.StatusInternalServerError, "could not save entries")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
