//go:build e2e || e2e_docker

package e2e_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// seedWeekEntries makes a direct API call to seed 7 entries for the given weekMonday (YYYY-MM-DD).
func seedWeekEntries(t *testing.T, serverURL, username string, userID int64, weekMonday string) {
	t.Helper()
	// Parse weekMonday
	base, err := time.Parse("2006-01-02", weekMonday)
	if err != nil {
		t.Fatalf("seedWeekEntries: parse date: %v", err)
	}
	entries := make([]map[string]any, 7)
	for i := 0; i < 7; i++ {
		entries[i] = map[string]any{
			"entry_date": base.AddDate(0, 0, i).Format("2006-01-02"),
			"day_type":   "office",
			"hours":      0,
		}
	}
	body, _ := json.Marshal(entries)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/users/%d/entries", serverURL, userID),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", username)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("seedWeekEntries %s: status %v err %v", weekMonday, resp.StatusCode, err)
	}
}

// getOrCreateUser calls GET /api/me to get the user ID for the given username.
func getUserID(t *testing.T, serverURL, username string) int64 {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, serverURL+"/api/me", nil)
	req.Header.Set("X-Test-User", username)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("getUserID: status %v err %v", resp.StatusCode, err)
	}
	defer resp.Body.Close()
	var user struct {
		ID int64 `json:"id"`
	}
	json.NewDecoder(resp.Body).Decode(&user)
	return user.ID
}

// waitFor polls a JS predicate (arrow function returning bool) until true or timeout.
func waitFor(t *testing.T, page *rod.Page, jsExpr string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		result := page.MustEval(jsExpr)
		if result.Bool() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for: %s", jsExpr)
}

// TestE2E_PageLoads verifies the app loads and shows the diary view.
func TestE2E_PageLoads(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	// Diary view must be present and report view hidden.
	page.MustElement("#entry-tbody")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	reportSection := page.MustElement("#view-report")
	visible, err := reportSection.Visible()
	if err != nil {
		t.Fatal(err)
	}
	if visible {
		t.Error("report section should be hidden on initial load")
	}
}

// TestE2E_SaveAndReloadEntry enters a WFH day, saves it, reloads the page,
// and verifies the entry persists.
// Uses ?week= to navigate to a specific week so smart-init does not redirect
// to a different week after reload.
func TestE2E_SaveAndReloadEntry(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Use a specific week (current week 2026-03-23) so ?week= takes
	// precedence over smart-init on both initial load and reload.
	weekURL := serverURL + "?week=2026-03-23"
	page.MustNavigate(weekURL)

	// Wait for the week rows to render.
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Set Monday (first row) to WFH with 7.5 hours.
	firstRow := page.MustElement("#entry-tbody tr:first-child")
	firstRow.MustElement(".day-type-select").MustSelect("Work From Home")
	firstRow.MustElement(".hours-input").MustInput("7.5")

	// Save (current week, so no auto-advance).
	page.MustElement("#save-entries").MustClick()
	waitFor(t, page, `() => document.getElementById('save-status').textContent === 'Saved'`)

	// Reload with same ?week= and verify data persisted.
	page.MustNavigate(weekURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Allow the async getEntries call to complete and populate the inputs.
	time.Sleep(500 * time.Millisecond)

	firstRow2 := page.MustElement("#entry-tbody tr:first-child")

	dtype := firstRow2.MustElement(".day-type-select").MustProperty("value").Str()
	if dtype != "wfh" {
		t.Errorf("day_type after reload: got %q, want wfh", dtype)
	}

	hours := firstRow2.MustElement(".hours-input").MustProperty("value").Str()
	if hours != "7.5" {
		t.Errorf("hours after reload: got %q, want 7.5", hours)
	}
}

// TestE2E_ReportShowsTotals saves a WFH entry then checks the report view
// reflects the correct total hours.
func TestE2E_ReportShowsTotals(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Enter an 8-hour WFH day.
	firstRow := page.MustElement("#entry-tbody tr:first-child")
	firstRow.MustElement(".day-type-select").MustSelect("Work From Home")
	firstRow.MustElement(".hours-input").MustInput("8")
	page.MustElement("#save-entries").MustClick()
	waitFor(t, page, `() => document.getElementById('save-status').textContent === 'Saved'`)

	// Switch to Report view.
	page.MustElement("#nav-report").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-report').hidden`)

	// Wait for the report summary to populate.
	waitFor(t, page, `() => document.getElementById('report-summary').textContent.includes('hours')`)

	summary := page.MustElement("#report-summary").MustText()
	if summary == "" {
		t.Error("report summary is empty")
	}

	// The report total should reflect the 8 hours saved.
	total := page.MustElement("#report-total").MustText()
	if total == "—" || total == "" {
		t.Errorf("report total not updated, got %q", total)
	}
}

// TestE2E_Settings_SaveAndReload saves a user profile via the Settings page
// and verifies it persists after reload.
func TestE2E_Settings_SaveAndReload(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Navigate to Settings.
	page.MustElement("#nav-settings").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-settings').hidden`)

	// Set default hours to 7.5 and Wednesday to office.
	page.MustElement("#profile-default-hours").MustInput("7.5")
	page.MustElement("#profile-wed-type").MustSelect("Office")

	// Save.
	page.MustElement("#save-profile").MustClick()
	waitFor(t, page, `() => document.getElementById('profile-status').textContent === 'Saved'`)

	// Reload and navigate back to Settings.
	page.MustReload()
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)
	time.Sleep(500 * time.Millisecond)

	page.MustElement("#nav-settings").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-settings').hidden`)
	time.Sleep(500 * time.Millisecond)

	hours := page.MustElement("#profile-default-hours").MustProperty("value").Str()
	if hours != "7.5" {
		t.Errorf("default_hours after reload: got %q, want 7.5", hours)
	}

	wedType := page.MustElement("#profile-wed-type").MustProperty("value").Str()
	if wedType != "office" {
		t.Errorf("wed_type after reload: got %q, want office", wedType)
	}
}

// TestE2E_WeekDefaults_FromProfile verifies that navigating to an empty week
// pre-populates entries from the user's profile.
func TestE2E_WeekDefaults_FromProfile(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Set up profile: Wednesday = office, default hours = 6.
	page.MustElement("#nav-settings").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-settings').hidden`)

	page.MustElement("#profile-default-hours").MustInput("6")
	page.MustElement("#profile-wed-type").MustSelect("Office")
	page.MustElement("#save-profile").MustClick()
	waitFor(t, page, `() => document.getElementById('profile-status').textContent === 'Saved'`)

	// Navigate back to diary — this week is empty, should apply profile defaults.
	page.MustElement("#nav-diary").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-diary').hidden`)
	time.Sleep(500 * time.Millisecond)

	rows := page.MustElements("#entry-tbody tr.day-row")
	if len(rows) != 7 {
		t.Fatalf("expected 7 rows, got %d", len(rows))
	}

	// Monday (index 0) should be WFH with hours=6.
	monType := rows[0].MustElement(".day-type-select").MustProperty("value").Str()
	if monType != "wfh" {
		t.Errorf("monday type: got %q, want wfh", monType)
	}
	monHours := rows[0].MustElement(".hours-input").MustProperty("value").Str()
	if monHours != "6" {
		t.Errorf("monday hours: got %q, want 6", monHours)
	}

	// Wednesday (index 2) should be office with no hours.
	wedType := rows[2].MustElement(".day-type-select").MustProperty("value").Str()
	if wedType != "office" {
		t.Errorf("wednesday type: got %q, want office", wedType)
	}
	wedHours := rows[2].MustElement(".hours-input").MustProperty("value").Str()
	if wedHours != "" {
		t.Errorf("wednesday hours: got %q, want empty", wedHours)
	}
}

// TestE2E_PrintPDFButton verifies the Print PDF button is present in the
// report view and is clickable without errors.
func TestE2E_PrintPDFButton(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Navigate to Report view.
	page.MustElement("#nav-report").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-report').hidden`)
	waitFor(t, page, `() => document.getElementById('report-summary').textContent.includes('Total')`)

	// Print PDF button must exist.
	btn := page.MustElement("#print-pdf")
	visible, err := btn.Visible()
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("print-pdf button should be visible in the report view")
	}
}

// TestE2E_DiaryWeekStartsOnMonday verifies that the diary week always starts on
// Monday regardless of timezone. The browser is set to Australia/Melbourne
// (UTC+11 in March) to reproduce a bug where formatDate() used toISOString()
// (UTC) causing midnight local time to appear as the previous day.
// Uses a known Tuesday (2026-03-24) via the ?week= query param.
// Expected: week label starts "Mon" and first row data-date is "2026-03-23".
func TestE2E_DiaryWeekStartsOnMonday(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Emulate Australian Eastern time (UTC+11 in March) — at this offset,
	// midnight local time is 1pm the previous day in UTC, which caused
	// formatDate(weekStart) to return Sunday instead of Monday.
	tz := proto.EmulationSetTimezoneOverride{TimezoneID: "Australia/Melbourne"}
	if err := tz.Call(page); err != nil {
		t.Fatalf("set timezone override: %v", err)
	}

	page.MustNavigate(serverURL + "?week=2026-03-24") // Tuesday 24 Mar 2026

	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Week label should open with "Mon"
	label := page.MustElement("#week-label").MustText()
	if !strings.HasPrefix(label, "Mon") {
		t.Errorf("week label should start with Mon (Monday), got %q", label)
	}

	// First row data-date should be the Monday of that week (2026-03-23)
	rows := page.MustElements("#entry-tbody tr.day-row")
	firstDate, err := rows[0].Attribute("data-date")
	if err != nil || firstDate == nil {
		t.Fatal("could not read data-date from first row")
	}
	if *firstDate != "2026-03-23" {
		t.Errorf("first row date: got %q, want 2026-03-23", *firstDate)
	}
}

// TestE2E_WFHAutoPopulatesHours verifies that changing a day type to wfh
// auto-populates the hours field with the user's default_hours from their profile,
// and that changing to part_wfh leaves the hours field blank.
func TestE2E_WFHAutoPopulatesHours(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Set default hours = 7.5 in profile.
	page.MustElement("#nav-settings").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-settings').hidden`)
	page.MustElement("#profile-default-hours").MustInput("7.5")
	page.MustElement("#save-profile").MustClick()
	waitFor(t, page, `() => document.getElementById('profile-status').textContent === 'Saved'`)

	// Reload so userProfile is updated in the app.
	page.MustReload()
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)
	time.Sleep(500 * time.Millisecond)

	rows := page.MustElements("#entry-tbody tr.day-row")

	// Change first row to wfh — hours should auto-populate with 7.5.
	rows[0].MustElement(".day-type-select").MustSelect("Work From Home")

	hours := rows[0].MustElement(".hours-input").MustProperty("value").Str()
	if hours != "7.5" {
		t.Errorf("hours after selecting wfh: got %q, want 7.5", hours)
	}

	// Change Saturday (index 5, defaults to weekend with no hours) to part_wfh.
	// Hours should NOT auto-populate since part_wfh has no default fill.
	rows[5].MustElement(".day-type-select").MustSelect("Part WFH")

	hoursPartial := rows[5].MustElement(".hours-input").MustProperty("value").Str()
	if hoursPartial != "" {
		t.Errorf("hours after selecting part_wfh: got %q, want empty", hoursPartial)
	}
}

// TestE2E_WeekNavigation verifies that Prev/Next week buttons update the
// week label and reload entries.
func TestE2E_WeekNavigation(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)

	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	initial := page.MustElement("#week-label").MustText()

	// Go to next week.
	page.MustElement("#next-week").MustClick()
	waitFor(t, page, `() => document.getElementById('week-label').textContent !== `+"`"+initial+"`")

	next := page.MustElement("#week-label").MustText()
	if next == initial {
		t.Errorf("week label unchanged after next-week click")
	}

	// Go back — should return to the original week.
	page.MustElement("#prev-week").MustClick()
	waitFor(t, page, `() => document.getElementById('week-label').textContent !== `+"`"+next+"`")

	back := page.MustElement("#week-label").MustText()
	if back != initial {
		t.Errorf("week label after round-trip: got %q, want %q", back, initial)
	}
}

// TestE2E_WeekStatusIndicator_NotSubmitted verifies that loading a week with
// fewer than 7 entries displays the "not submitted" status indicator.
func TestE2E_WeekStatusIndicator_NotSubmitted(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Navigate to a specific past week with no entries
	page.MustNavigate(serverURL + "?week=2025-07-07")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)
	time.Sleep(300 * time.Millisecond)

	status := page.MustElement("#week-status").MustText()
	if !strings.Contains(status, "not submitted") {
		t.Errorf("week status: got %q, want text containing 'not submitted'", status)
	}
}

// TestE2E_WeekStatusIndicator_Submitted verifies that loading a week with all
// 7 entries displays the "submitted" status indicator.
func TestE2E_WeekStatusIndicator_Submitted(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Create user and seed a complete week
	userID := getUserID(t, serverURL, "alice")
	seedWeekEntries(t, serverURL, "alice", userID, "2025-07-07")

	page.MustNavigate(serverURL + "?week=2025-07-07")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)
	time.Sleep(300 * time.Millisecond)

	status := page.MustElement("#week-status").MustText()
	if !strings.Contains(status, "submitted") || strings.Contains(status, "not submitted") {
		t.Errorf("week status: got %q, want text containing 'submitted' but not 'not submitted'", status)
	}
}

// TestE2E_SmartInit_LoadsFirstIncompleteWeek verifies that on initial load
// (without ?week= param) the app navigates to the oldest incomplete week.
func TestE2E_SmartInit_LoadsFirstIncompleteWeek(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Get user ID and seed complete weeks for the first 3 weeks of FY2026.
	// FY2026 starts on the week of Mon 30 Jun 2025 (the week containing Jul 1).
	// 2025-06-30, 2025-07-07 and 2025-07-14 are complete; 2025-07-21 is incomplete.
	userID := getUserID(t, serverURL, "alice")
	seedWeekEntries(t, serverURL, "alice", userID, "2025-06-30")
	seedWeekEntries(t, serverURL, "alice", userID, "2025-07-07")
	seedWeekEntries(t, serverURL, "alice", userID, "2025-07-14")
	// 2025-07-21 is left incomplete

	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)
	time.Sleep(500 * time.Millisecond)

	// The first row should have data-date of 2025-07-21 (first incomplete week)
	rows := page.MustElements("#entry-tbody tr.day-row")
	firstDate, err := rows[0].Attribute("data-date")
	if err != nil || firstDate == nil {
		t.Fatal("could not read data-date from first row")
	}
	if *firstDate != "2025-07-21" {
		t.Errorf("smart init: first row date got %q, want 2025-07-21", *firstDate)
	}
}

// TestE2E_SmartInit_FallsBackToCurrentWeek verifies that when all weeks up to
// the current week are complete, the app falls back to the current week.
func TestE2E_SmartInit_FallsBackToCurrentWeek(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Load app without seeding — no entries means the first Monday of FY2026
	// (2025-07-07) is incomplete. Since today (2026-03-24) is well past that,
	// the smart init should navigate there, NOT the current week.
	// So this test just verifies it doesn't always land on the current week.
	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)
	time.Sleep(500 * time.Millisecond)

	label := page.MustElement("#week-label").MustText()
	// Current week label would contain "Mar" (March 2026); the first incomplete
	// week (2025-07-07) would contain "Jul".
	if strings.Contains(label, "Mar 2026") {
		t.Errorf("smart init loaded current week when incomplete weeks exist: %q", label)
	}
}

// TestE2E_AutoAdvance_AfterSavingPastWeek verifies that after saving a past
// week the app automatically advances to the next incomplete week.
func TestE2E_AutoAdvance_AfterSavingPastWeek(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	userID := getUserID(t, serverURL, "alice")
	// Seed 2025-07-14 as complete; leave 2025-07-07 and 2025-07-21 empty.
	seedWeekEntries(t, serverURL, "alice", userID, "2025-07-14")

	// Navigate directly to the first incomplete week (2025-07-07)
	page.MustNavigate(serverURL + "?week=2025-07-07")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Set all 7 rows to "office" and save
	page.MustEval(`() => {
		document.querySelectorAll('#entry-tbody tr.day-row').forEach(row => {
			row.querySelector('.day-type-select').value = 'office';
		});
	}`)
	page.MustElement("#save-entries").MustClick()
	waitFor(t, page, `() => document.getElementById('save-status').textContent === 'Saved'`)

	// After saving a past week, app should advance to the next incomplete week.
	// 2025-07-14 is complete, so next incomplete is 2025-07-21.
	time.Sleep(800 * time.Millisecond)

	rows := page.MustElements("#entry-tbody tr.day-row")
	firstDate, err := rows[0].Attribute("data-date")
	if err != nil || firstDate == nil {
		t.Fatal("could not read data-date from first row after auto-advance")
	}
	if *firstDate != "2025-07-21" {
		t.Errorf("auto-advance: first row date got %q, want 2025-07-21", *firstDate)
	}
}
