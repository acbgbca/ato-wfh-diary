package model

import "time"

// DayType represents the classification of a working day.
type DayType string

const (
	DayTypeWFH           DayType = "wfh"
	DayTypePartWFH       DayType = "part_wfh"
	DayTypeOffice        DayType = "office"
	DayTypeAnnualLeave   DayType = "annual_leave"
	DayTypeSickLeave     DayType = "sick_leave"
	DayTypePublicHoliday DayType = "public_holiday"
	DayTypeWeekend       DayType = "weekend"
)

// User represents an authenticated family member.
type User struct {
	ID          int64     `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WorkDayEntry records how a single day was spent.
type WorkDayEntry struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	EntryDate      time.Time `json:"entry_date"`
	FinancialYear  int       `json:"financial_year"`
	DayType        DayType   `json:"day_type"`
	Hours          float64   `json:"hours"`
	Notes          string    `json:"notes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// FinancialYear returns the Australian financial year that contains t.
// e.g. 15 Aug 2024 → 2025, 3 Mar 2025 → 2025.
func FinancialYear(t time.Time) int {
	if t.Month() >= 7 {
		return t.Year() + 1
	}
	return t.Year()
}
