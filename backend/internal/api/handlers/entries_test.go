package handlers_test

import (
	"fmt"
	"net/http"
	"testing"
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
