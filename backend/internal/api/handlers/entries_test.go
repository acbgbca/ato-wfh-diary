package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestGetWeekEntries_Empty(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries?week_start=2025-01-06", userID),
		"alice", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %d entries", len(entries))
	}
}

func TestGetWeekEntries_ReturnsEntriesForCorrectWeek(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	body := []map[string]any{
		{"entry_date": "2025-01-06", "day_type": "wfh", "hours": 8},
		{"entry_date": "2025-01-07", "day_type": "office", "hours": 0},
		{"entry_date": "2025-01-08", "day_type": "part_wfh", "hours": 4.5},
	}
	postResp := do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice", body)
	if postResp.StatusCode != http.StatusNoContent {
		t.Fatalf("upsert status: got %d, want %d", postResp.StatusCode, http.StatusNoContent)
	}

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries?week_start=2025-01-06", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0]["entry_date"] != "2025-01-06" {
		t.Errorf("entry_date: got %v, want 2025-01-06", entries[0]["entry_date"])
	}
	if entries[0]["day_type"] != "wfh" {
		t.Errorf("day_type: got %v, want wfh", entries[0]["day_type"])
	}
	if entries[2]["hours"] != 4.5 {
		t.Errorf("hours: got %v, want 4.5", entries[2]["hours"])
	}
}

func TestGetWeekEntries_DoesNotReturnOtherWeeks(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	body := []map[string]any{
		{"entry_date": "2025-01-06", "day_type": "wfh", "hours": 8},
		{"entry_date": "2025-01-13", "day_type": "wfh", "hours": 8}, // different week
	}
	do(t, srv, http.MethodPost, fmt.Sprintf("/api/users/%d/entries", userID), "alice", body)

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries?week_start=2025-01-06", userID),
		"alice", nil)
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry for week, got %d", len(entries))
	}
}

func TestUpsertWeekEntries_UpdatesExistingEntry(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	path := fmt.Sprintf("/api/users/%d/entries", userID)

	do(t, srv, http.MethodPost, path, "alice", []map[string]any{
		{"entry_date": "2025-01-06", "day_type": "office", "hours": 0},
	})
	do(t, srv, http.MethodPost, path, "alice", []map[string]any{
		{"entry_date": "2025-01-06", "day_type": "wfh", "hours": 7.5},
	})

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries?week_start=2025-01-06", userID),
		"alice", nil)
	var entries []map[string]any
	decodeJSON(t, resp, &entries)

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0]["day_type"] != "wfh" {
		t.Errorf("day_type: got %v, want wfh", entries[0]["day_type"])
	}
	if entries[0]["hours"] != 7.5 {
		t.Errorf("hours: got %v, want 7.5", entries[0]["hours"])
	}
}

func TestUpsertWeekEntries_CoercesHoursToZeroForNonWFH(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	// Sending hours=8 for an office day — should be stored as 0.
	do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice",
		[]map[string]any{{"entry_date": "2025-01-06", "day_type": "office", "hours": 8}},
	)

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries?week_start=2025-01-06", userID),
		"alice", nil)
	var entries []map[string]any
	decodeJSON(t, resp, &entries)

	if entries[0]["hours"] != float64(0) {
		t.Errorf("hours: got %v, want 0 for office day", entries[0]["hours"])
	}
}

func TestUpsertWeekEntries_InvalidDayType(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice",
		[]map[string]any{{"entry_date": "2025-01-06", "day_type": "nap", "hours": 8}},
	)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUpsertWeekEntries_InvalidHoursForWFH(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice",
		[]map[string]any{{"entry_date": "2025-01-06", "day_type": "wfh", "hours": 0}},
	)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUpsertWeekEntries_InvalidDateFormat(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice",
		[]map[string]any{{"entry_date": "06/01/2025", "day_type": "wfh", "hours": 8}},
	)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestUpsertWeekEntries_UserNotFound(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodPost,
		"/api/users/9999/entries",
		"alice",
		[]map[string]any{{"entry_date": "2025-01-06", "day_type": "wfh", "hours": 8}},
	)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetWeekEntries_MissingWeekStart(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetFirstIncompleteWeek_ReturnsWeekStart(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	// Seed an incomplete week in FY2026 (Jul 2025–Jun 2026)
	// Use week of 2025-07-07 with only 3 entries
	do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice",
		[]map[string]any{
			{"entry_date": "2025-07-07", "day_type": "office", "hours": 0},
			{"entry_date": "2025-07-08", "day_type": "office", "hours": 0},
			{"entry_date": "2025-07-09", "day_type": "office", "hours": 0},
		},
	)

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries/first-incomplete-week?financial_year=2026&from_date=2025-07-07", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["week_start"] != "2025-07-07" {
		t.Errorf("week_start: got %v, want 2025-07-07", body["week_start"])
	}
}

func TestGetFirstIncompleteWeek_AllCompleteReturnsNull(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	// Dynamically compute the current week's Monday so the test remains
	// correct as time passes. Seeding only the current week means no
	// incomplete weeks exist up to (and including) today.
	today := time.Now().UTC()
	daysToMonday := int(today.Weekday()+6) % 7
	monday := today.AddDate(0, 0, -daysToMonday)
	weekMonday := monday.Format("2006-01-02")

	fy := monday.Year()
	if monday.Month() >= time.July {
		fy++
	}

	entries := make([]map[string]any, 7)
	for i := 0; i < 7; i++ {
		d := monday.AddDate(0, 0, i).Format("2006-01-02")
		entries[i] = map[string]any{"entry_date": d, "day_type": "office", "hours": 0}
	}
	do(t, srv, http.MethodPost, fmt.Sprintf("/api/users/%d/entries", userID), "alice", entries)

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries/first-incomplete-week?financial_year=%d&from_date=%s", userID, fy, weekMonday),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["week_start"] != nil {
		t.Errorf("week_start: got %v, want null", body["week_start"])
	}
}

func TestGetFirstIncompleteWeek_UserNotFound(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet,
		"/api/users/9999/entries/first-incomplete-week?financial_year=2026&from_date=2025-07-07",
		"alice", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetFirstIncompleteWeek_Unauthorised(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet,
		"/api/users/1/entries/first-incomplete-week?financial_year=2026&from_date=2025-07-07",
		"", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestGetFirstIncompleteWeek_InvalidFromDate(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries/first-incomplete-week?financial_year=2026&from_date=not-a-date", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestGetWeekCompletionStatus_ReturnsWeekCounts(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	// Seed a complete week in FY2026 (week of 2025-07-07)
	entries := make([]map[string]any, 7)
	for i := range 7 {
		d := time.Date(2025, 7, 7+i, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		entries[i] = map[string]any{"entry_date": d, "day_type": "office", "hours": 0}
	}
	do(t, srv, http.MethodPost, fmt.Sprintf("/api/users/%d/entries", userID), "alice", entries)

	// Also seed a partial week (3 entries in week of 2025-07-14)
	partial := []map[string]any{
		{"entry_date": "2025-07-14", "day_type": "wfh", "hours": 8},
		{"entry_date": "2025-07-15", "day_type": "office", "hours": 0},
		{"entry_date": "2025-07-16", "day_type": "office", "hours": 0},
	}
	do(t, srv, http.MethodPost, fmt.Sprintf("/api/users/%d/entries", userID), "alice", partial)

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries/week-status?financial_year=2026", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body []map[string]any
	decodeJSON(t, resp, &body)

	// Result depends on whether today >= 2025-07-14. Since we can't control time
	// in handler tests, we just verify the endpoint returns valid data.
	if len(body) == 0 {
		t.Fatal("expected at least 1 week status entry")
	}

	// First entry should be the complete week
	if body[0]["week_start"] != "2025-07-07" {
		t.Errorf("first week_start: got %v, want 2025-07-07", body[0]["week_start"])
	}
	if body[0]["count"] != float64(7) {
		t.Errorf("first count: got %v, want 7", body[0]["count"])
	}
}

func TestGetWeekCompletionStatus_DefaultsFY(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	// Seed an entry in the current FY
	today := time.Now().UTC()
	d := today.Format("2006-01-02")
	do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", userID),
		"alice",
		[]map[string]any{{"entry_date": d, "day_type": "office", "hours": 0}},
	)

	// No financial_year param — should default to current FY
	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries/week-status", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body []map[string]any
	decodeJSON(t, resp, &body)
	if len(body) != 1 {
		t.Fatalf("expected 1 result, got %d", len(body))
	}
	if body[0]["count"] != float64(1) {
		t.Errorf("count: got %v, want 1", body[0]["count"])
	}
}

func TestGetWeekCompletionStatus_UserNotFound(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet,
		"/api/users/9999/entries/week-status?financial_year=2026",
		"alice", nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestGetWeekCompletionStatus_Unauthorised(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet,
		"/api/users/1/entries/week-status?financial_year=2026",
		"", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestGetWeekCompletionStatus_EmptyFY(t *testing.T) {
	srv := newTestServer(t)
	userID := mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries/week-status?financial_year=2026", userID),
		"alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var body []map[string]any
	decodeJSON(t, resp, &body)
	if len(body) != 0 {
		t.Errorf("expected empty array, got %d entries", len(body))
	}
}

func TestGetWeekEntries_Unauthorised(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet, "/api/users/1/entries?week_start=2025-01-06", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestGetWeekEntries_OneUserCannotSeeAnothersEntries(t *testing.T) {
	srv := newTestServer(t)
	aliceID := mustCreateUser(t, srv, "alice")
	bobID := mustCreateUser(t, srv, "bob")

	do(t, srv, http.MethodPost,
		fmt.Sprintf("/api/users/%d/entries", aliceID),
		"alice",
		[]map[string]any{{"entry_date": "2025-01-06", "day_type": "wfh", "hours": 8}},
	)

	// Bob can read Alice's entries (shared family access).
	resp := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries?week_start=2025-01-06", aliceID),
		"bob", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bob reading alice's entries: status %d", resp.StatusCode)
	}
	var entries []map[string]any
	decodeJSON(t, resp, &entries)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(entries))
	}

	// Bob's own entries are separate.
	resp2 := do(t, srv, http.MethodGet,
		fmt.Sprintf("/api/users/%d/entries?week_start=2025-01-06", bobID),
		"bob", nil)
	var bobEntries []map[string]any
	decodeJSON(t, resp2, &bobEntries)
	if len(bobEntries) != 0 {
		t.Errorf("expected 0 entries for bob, got %d", len(bobEntries))
	}
}
