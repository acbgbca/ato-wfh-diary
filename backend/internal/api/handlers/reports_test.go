package handlers_test

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func seedEntries(t *testing.T, srv *httptest.Server, userID int64, entries []map[string]any) {
	t.Helper()
	resp := do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice", entries)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("seedEntries: status %d", resp.StatusCode)
	}
}

func TestGetReport_EmptyYear(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/report?financial_year=2025", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var report map[string]any
	decodeJSON(t, resp, &report)
	if report["total_hours"] != float64(0) {
		t.Errorf("total_hours: got %v, want 0", report["total_hours"])
	}
	if len(report["entries"].([]any)) != 0 {
		t.Errorf("expected empty entries")
	}
}

func TestGetReport_SumsTotalHours(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	seedEntries(t, srv, userID, []map[string]any{
		{"entry_date": "2024-08-01", "day_type": "wfh", "hours": 8},
		{"entry_date": "2024-08-02", "day_type": "wfh", "hours": 7.5},
		{"entry_date": "2024-08-05", "day_type": "part_wfh", "hours": 4},
	})

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/report?financial_year=2025", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var report map[string]any
	decodeJSON(t, resp, &report)
	if report["total_hours"] != 19.5 {
		t.Errorf("total_hours: got %v, want 19.5", report["total_hours"])
	}
	if len(report["entries"].([]any)) != 3 {
		t.Errorf("entries: got %d, want 3", len(report["entries"].([]any)))
	}
}

func TestGetReport_ExcludesNonWFHEntries(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	seedEntries(t, srv, userID, []map[string]any{
		{"entry_date": "2024-08-01", "day_type": "wfh", "hours": 8},
		{"entry_date": "2024-08-02", "day_type": "office", "hours": 0},
		{"entry_date": "2024-08-03", "day_type": "annual_leave", "hours": 0},
		{"entry_date": "2024-08-04", "day_type": "sick_leave", "hours": 0},
		{"entry_date": "2024-08-05", "day_type": "public_holiday", "hours": 0},
	})

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/report?financial_year=2025", userID),
		"alice", nil)
	var report map[string]any
	decodeJSON(t, resp, &report)

	if report["total_hours"] != float64(8) {
		t.Errorf("total_hours: got %v, want 8", report["total_hours"])
	}
	if len(report["entries"].([]any)) != 1 {
		t.Errorf("entries: got %d, want 1 (only WFH)", len(report["entries"].([]any)))
	}
}

func TestGetReport_IsolatedByFinancialYear(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	seedEntries(t, srv, userID, []map[string]any{
		{"entry_date": "2024-06-30", "day_type": "wfh", "hours": 8}, // FY2024
		{"entry_date": "2024-07-01", "day_type": "wfh", "hours": 8}, // FY2025
		{"entry_date": "2025-06-30", "day_type": "wfh", "hours": 8}, // FY2025
		{"entry_date": "2025-07-01", "day_type": "wfh", "hours": 8}, // FY2026
	})

	check := func(fy int, wantEntries int, wantTotal float64) {
		t.Helper()
		resp := do(t, srv, http.MethodGet,
			fmt.Sprintf("/api/users/%d/report?financial_year=%d", userID, fy),
			"alice", nil)
		var report map[string]any
		decodeJSON(t, resp, &report)
		if got := len(report["entries"].([]any)); got != wantEntries {
			t.Errorf("FY%d entries: got %d, want %d", fy, got, wantEntries)
		}
		if report["total_hours"] != wantTotal {
			t.Errorf("FY%d total_hours: got %v, want %v", fy, report["total_hours"], wantTotal)
		}
	}

	check(2024, 1, 8)
	check(2025, 2, 16)
	check(2026, 1, 8)
}

func TestGetReport_UserNotFound(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet, "/api/users/9999/report?financial_year=2025", "alice", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetReport_IncludesDisplayName(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/report?financial_year=2025", userID),
		"alice", nil)
	var report map[string]any
	decodeJSON(t, resp, &report)

	if report["display_name"] != "alice" {
		t.Errorf("display_name: got %v, want alice", report["display_name"])
	}
}

func TestExportReport_CSVFormat(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	seedEntries(t, srv, userID, []map[string]any{
		{"entry_date": "2024-08-01", "day_type": "wfh", "hours": 8},
		{"entry_date": "2024-08-02", "day_type": "part_wfh", "hours": 3.5, "notes": "afternoon only"},
	})

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/report/export?financial_year=2025&format=csv", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type: got %q, want text/csv", ct)
	}

	cd := resp.Header.Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Errorf("Content-Disposition missing attachment: %q", cd)
	}
	if !strings.Contains(cd, "wfh-report-fy2025") {
		t.Errorf("Content-Disposition missing filename: %q", cd)
	}

	defer resp.Body.Close()
	var lines []string
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// Verify header block is present.
	if len(lines) == 0 || !strings.Contains(lines[0], "ATO Work From Home Report") {
		t.Errorf("missing report title in CSV, got: %v", lines)
	}

	// Verify total hours appear in the header block.
	found := false
	for _, l := range lines {
		if strings.Contains(l, "11.50") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("total hours 11.50 not found in CSV output: %v", lines)
	}

	// Verify detail rows are present.
	found2024 := false
	for _, l := range lines {
		if strings.Contains(l, "2024-08-01") {
			found2024 = true
			break
		}
	}
	if !found2024 {
		t.Errorf("entry date 2024-08-01 not found in CSV output")
	}
}

func TestExportReport_UnsupportedFormat(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/report/export?financial_year=2025&format=pdf", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestExportReport_UserNotFound(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet,
		"/api/users/9999/report/export?financial_year=2025&format=csv",
		"alice", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetReport_IncludesAllEntries(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	seedEntries(t, srv, userID, []map[string]any{
		{"entry_date": "2024-08-01", "day_type": "wfh", "hours": 8},
		{"entry_date": "2024-08-02", "day_type": "office", "hours": 0},
		{"entry_date": "2024-08-03", "day_type": "annual_leave", "hours": 0},
	})

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/report?financial_year=2025", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var report map[string]any
	decodeJSON(t, resp, &report)

	// entries (WFH only) should have 1 entry
	if len(report["entries"].([]any)) != 1 {
		t.Errorf("entries: got %d, want 1 (WFH only)", len(report["entries"].([]any)))
	}

	// all_entries should have all 3 entries
	allEntries, ok := report["all_entries"].([]any)
	if !ok {
		t.Fatalf("all_entries field missing or wrong type in response")
	}
	if len(allEntries) != 3 {
		t.Errorf("all_entries: got %d, want 3 (all day types)", len(allEntries))
	}
}

func TestGetReport_AllEntriesEmptyWhenNoData(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/report?financial_year=2025", userID),
		"alice", nil)

	var report map[string]any
	decodeJSON(t, resp, &report)

	allEntries, ok := report["all_entries"].([]any)
	if !ok {
		// nil slice marshalled as null — treat as empty
		if report["all_entries"] != nil {
			t.Fatalf("all_entries: unexpected value %v", report["all_entries"])
		}
		return
	}
	if len(allEntries) != 0 {
		t.Errorf("all_entries: got %d, want 0", len(allEntries))
	}
}
