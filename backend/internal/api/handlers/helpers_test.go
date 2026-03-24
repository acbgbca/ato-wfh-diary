package handlers

import "testing"

func TestFirstMondayOfFY(t *testing.T) {
	cases := []struct {
		financialYear int
		want          string // YYYY-MM-DD
	}{
		// FY2026: Jul 1 2025 is Tuesday → Mon 30 Jun 2025
		{2026, "2025-06-30"},
		// FY2025: Jul 1 2024 is Monday → Mon 1 Jul 2024
		{2025, "2024-07-01"},
		// FY2027: Jul 1 2026 is Wednesday → Mon 29 Jun 2026
		{2027, "2026-06-29"},
	}
	for _, tc := range cases {
		got := firstMondayOfFY(tc.financialYear).Format("2006-01-02")
		if got != tc.want {
			t.Errorf("firstMondayOfFY(%d) = %s, want %s", tc.financialYear, got, tc.want)
		}
	}
}
