package handlers

import (
	"encoding/json"
	"net/http"
)

// GetReport returns the WFH report for a given user and financial year.
// Query params: user_id, financial_year (e.g. 2025).
func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	// TODO: parse user_id and financial_year from query params
	// TODO: query wfh/part_wfh entries for that user and FY
	// TODO: aggregate total hours and build detail list
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{})
}

// ExportReport exports the WFH report as a downloadable file (CSV or PDF).
// Query params: user_id, financial_year, format (csv|pdf).
func (h *Handler) ExportReport(w http.ResponseWriter, r *http.Request) {
	// TODO: generate report data
	// TODO: serialise to requested format and write response with appropriate Content-Type
	w.WriteHeader(http.StatusNotImplemented)
}
