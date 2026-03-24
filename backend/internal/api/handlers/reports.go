package handlers

import (
	"ato-wfh-diary/internal/model"
	"ato-wfh-diary/internal/service"
	"encoding/csv"
	"fmt"
	"net/http"
)

// reportResponse is the JSON shape returned by GetReport.
type reportResponse struct {
	UserID        int64           `json:"user_id"`
	DisplayName   string          `json:"display_name"`
	FinancialYear int             `json:"financial_year"`
	TotalHours    float64         `json:"total_hours"`
	Entries       []entryResponse `json:"entries"`
	AllEntries    []entryResponse `json:"all_entries"`
}

// GetReport returns the WFH report for a user and financial year.
//
// Query params:
//   - financial_year (optional): defaults to the last completed financial year
func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
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

	fy, ok := queryInt(r, "financial_year")
	if !ok {
		fy = service.LastFinancialYear()
	}

	entries, err := h.Store.GetFYWFHEntries(r.Context(), userID, fy)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve entries")
		return
	}

	allEntries, err := h.Store.GetFYAllEntries(r.Context(), userID, fy)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve all entries")
		return
	}

	summary := service.BuildReport(userID, fy, entries, allEntries)

	resp := reportResponse{
		UserID:        userID,
		DisplayName:   user.DisplayName,
		FinancialYear: fy,
		TotalHours:    summary.TotalHours,
		Entries:       make([]entryResponse, len(entries)),
		AllEntries:    make([]entryResponse, len(allEntries)),
	}
	for i, e := range entries {
		resp.Entries[i] = toEntryResponse(e)
	}
	for i, e := range allEntries {
		resp.AllEntries[i] = toEntryResponse(e)
	}
	respondJSON(w, resp)
}

// ExportReport exports the WFH report as a downloadable CSV file.
//
// Query params:
//   - financial_year (optional): defaults to the last completed financial year
//   - format (required): currently only "csv" is supported
func (h *Handler) ExportReport(w http.ResponseWriter, r *http.Request) {
	userID, ok := pathUserID(r)
	if !ok {
		respondError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	format := r.URL.Query().Get("format")
	if format == "" {
		format = "csv"
	}
	if format != "csv" {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("unsupported format %q — only \"csv\" is supported", format))
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

	fy, ok := queryInt(r, "financial_year")
	if !ok {
		fy = service.LastFinancialYear()
	}

	entries, err := h.Store.GetFYWFHEntries(r.Context(), userID, fy)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve entries")
		return
	}

	summary := service.BuildReport(userID, fy, entries, nil)

	filename := fmt.Sprintf("wfh-report-fy%d-%s.csv", fy, user.Username)
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))

	cw := csv.NewWriter(w)

	// Header block
	cw.Write([]string{"ATO Work From Home Report"})
	cw.Write([]string{"User", user.DisplayName})
	cw.Write([]string{"Financial Year", fyLabel(fy)})
	cw.Write([]string{"Total WFH Hours", fmt.Sprintf("%.2f", summary.TotalHours)})
	cw.Write([]string{})

	// Detail
	cw.Write([]string{"Date", "Day Type", "Hours", "Notes"})
	for _, e := range entries {
		cw.Write([]string{
			e.EntryDate.Format("2006-01-02"),
			dayTypeLabel(e.DayType),
			fmt.Sprintf("%.2f", e.Hours),
			e.Notes,
		})
	}
	cw.Flush()
}

// fyLabel returns a human-readable financial year label, e.g. "FY2025 (1 Jul 2024 to 30 Jun 2025)".
func fyLabel(fy int) string {
	return fmt.Sprintf("FY%d (1 Jul %d to 30 Jun %d)", fy, fy-1, fy)
}

// dayTypeLabel returns a human-readable label for a day type.
func dayTypeLabel(d model.DayType) string {
	switch d {
	case model.DayTypeWFH:
		return "Work from home"
	case model.DayTypePartWFH:
		return "Part day WFH"
	case model.DayTypeOffice:
		return "Office"
	case model.DayTypeAnnualLeave:
		return "Annual leave"
	case model.DayTypeSickLeave:
		return "Sick leave"
	case model.DayTypePublicHoliday:
		return "Public holiday"
	case model.DayTypeWeekend:
		return "Weekend"
	default:
		return string(d)
	}
}
