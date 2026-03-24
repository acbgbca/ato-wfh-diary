package service

import (
	"ato-wfh-diary/internal/model"
	"time"
)

// ReportSummary is the top-level result of a financial year WFH report.
type ReportSummary struct {
	UserID        int64                `json:"user_id"`
	FinancialYear int                  `json:"financial_year"`
	TotalHours    float64              `json:"total_hours"`
	Entries       []model.WorkDayEntry `json:"entries"`
	AllEntries    []model.WorkDayEntry `json:"all_entries"`
}

// BuildReport aggregates WFH entries into a report summary.
// entries contains only wfh/part_wfh entries (used for totals).
// allEntries contains every entry for the financial year (used for the calendar PDF).
func BuildReport(userID int64, financialYear int, entries []model.WorkDayEntry, allEntries []model.WorkDayEntry) ReportSummary {
	var total float64
	for _, e := range entries {
		total += e.Hours
	}
	return ReportSummary{
		UserID:        userID,
		FinancialYear: financialYear,
		TotalHours:    total,
		Entries:       entries,
		AllEntries:    allEntries,
	}
}

// CurrentFinancialYear returns the financial year that contains today's date.
func CurrentFinancialYear() int {
	return model.FinancialYear(time.Now())
}

// LastFinancialYear returns the most recently completed financial year.
func LastFinancialYear() int {
	now := time.Now()
	fy := model.FinancialYear(now)
	// If we're still in the first day of the new FY (1 Jul) the current FY has
	// barely started — either way, "last FY" is always current FY minus one.
	return fy - 1
}
