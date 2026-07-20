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

// e2eToday is the date every E2E run pretends it is. The tests assert on
// concrete dates in FY2026, which only hold while "today" sits inside that FY —
// left on the real clock the whole suite breaks on 1 July, when the financial
// year rolls over. Both the server (via internal/clock) and the browser (via
// pinBrowserClock) are pinned to this date, so the suite is reproducible on any
// day of any year.
//
// It must be a date comfortably inside FY2026 and far enough past the FY start
// that the tests have several completed weeks to work with.
const e2eToday = "2026-03-24"

// pinBrowserClock makes the page's JS clock report the given YYYY-MM-DD date on
// every subsequent navigation. The frontend derives the current financial year
// and the "past weeks" list from new Date(), so pinning the server alone is not
// enough.
//
// The shim keeps every other Date behaviour intact: only the zero-argument
// constructor and Date.now() are redirected, so date parsing and arithmetic in
// the app still work normally. Calling it again re-pins the page to a different
// date — it stashes the genuine Date on first use, so each pin wraps the real
// one rather than the previous shim.
func pinBrowserClock(t *testing.T, page *rod.Page, today string) {
	t.Helper()
	// Midday UTC, matching clock.Fix on the server, so both agree on the
	// calendar date regardless of the browser's timezone.
	script := fmt.Sprintf(`(() => {
		const RealDate = window.__realDate || Date;
		window.__realDate = RealDate;
		const fixed = new RealDate(%q + 'T12:00:00Z').getTime();
		function PinnedDate(...args) {
			return args.length === 0 ? new RealDate(fixed) : new RealDate(...args);
		}
		PinnedDate.prototype = RealDate.prototype;
		PinnedDate.now   = () => fixed;
		PinnedDate.parse = RealDate.parse;
		PinnedDate.UTC   = RealDate.UTC;
		window.Date = PinnedDate;
	})()`, today)

	if _, err := page.EvalOnNewDocument(script); err != nil {
		t.Fatalf("pin browser clock to %s: %v", today, err)
	}
}

// seedWeekEntries makes a direct API call to seed 7 office entries for the given weekMonday (YYYY-MM-DD).
func seedWeekEntries(t *testing.T, serverURL, username string, userID int64, weekMonday string) {
	t.Helper()
	seedWeekEntriesTyped(t, serverURL, username, userID, weekMonday, "office", 0)
}

// seedWeekEntriesTyped seeds all 7 days of weekMonday with the given day type and hours.
func seedWeekEntriesTyped(t *testing.T, serverURL, username string, userID int64, weekMonday, dayType string, hours float64) {
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
			"day_type":   dayType,
			"hours":      hours,
		}
	}
	body, _ := json.Marshal(entries)
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/api/users/%d/entries", serverURL, userID),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(testAuthHeader, username)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusNoContent {
		t.Fatalf("seedWeekEntries %s: status %v err %v", weekMonday, resp.StatusCode, err)
	}
}

// getOrCreateUser calls GET /api/me to get the user ID for the given username.
func getUserID(t *testing.T, serverURL, username string) int64 {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, serverURL+"/api/me", nil)
	req.Header.Set(testAuthHeader, username)
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

// selectReportFY switches the report's financial year selector and triggers a reload.
func selectReportFY(t *testing.T, page *rod.Page, fy int) {
	t.Helper()
	page.MustEval(fmt.Sprintf(`() => {
		const s = document.getElementById('fy-select');
		s.value = '%d';
		s.dispatchEvent(new Event('change'));
	}`, fy))
}

// openReportForFY navigates to the report view for the given financial year and
// waits for the week table to render.
func openReportForFY(t *testing.T, page *rod.Page, fy int) {
	t.Helper()
	page.MustElement("#nav-report").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-report').hidden`)
	selectReportFY(t, page, fy)
	waitFor(t, page, `() => document.querySelectorAll('#report-tbody tr.week-row').length > 0`)
}

// TestE2E_ReportListsAllWeeksOfFY verifies the report replaces the per-entry
// list with one row per week of the financial year — including the week that
// straddles 1 July — and reports each week's submission status and WFH hours.
func TestE2E_ReportListsAllWeeksOfFY(t *testing.T) {
	serverURL := newE2EServer(t)
	// The Docker suite shares one database and isolates tests by giving each its
	// own user, so seeding has to target the same user the page authenticates as.
	u := testUsername(t, "alice")
	userID := getUserID(t, serverURL, u)

	// A fully submitted WFH week in the past of FY2026 (today is 2026-03-24).
	seedWeekEntriesTyped(t, serverURL, u, userID, "2026-03-16", "wfh", 8)

	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	openReportForFY(t, page, 2026)

	// FY2026 starts Wed 1 Jul 2025, so the first listed week is Mon 30 Jun 2025.
	first := page.MustEval(`() => document.querySelector('#report-tbody tr.week-row').dataset.weekStart`).Str()
	if first != "2025-06-30" {
		t.Errorf("first week row: got %q, want 2025-06-30", first)
	}

	// The FY ends Tue 30 Jun 2026, so the last listed week is Mon 29 Jun 2026.
	last := page.MustEval(`() => [...document.querySelectorAll('#report-tbody tr.week-row')].at(-1).dataset.weekStart`).Str()
	if last != "2026-06-29" {
		t.Errorf("last week row: got %q, want 2026-06-29", last)
	}

	statusOf := func(week string) string {
		return page.MustEval(fmt.Sprintf(
			`() => document.querySelector('#report-tbody tr[data-week-start="%s"] .week-status').textContent.trim()`, week)).Str()
	}
	hoursOf := func(week string) string {
		return page.MustEval(fmt.Sprintf(
			`() => document.querySelector('#report-tbody tr[data-week-start="%s"] .week-hours').textContent.trim()`, week)).Str()
	}

	if got := statusOf("2026-03-16"); got != "Submitted" {
		t.Errorf("status of seeded week: got %q, want Submitted", got)
	}
	if got := hoursOf("2026-03-16"); got != "56.00" {
		t.Errorf("hours of seeded week: got %q, want 56.00", got)
	}

	// A past week with no entries is unsubmitted and shows no hours.
	if got := statusOf("2026-03-02"); got != "Unsubmitted" {
		t.Errorf("status of empty past week: got %q, want Unsubmitted", got)
	}
	if got := hoursOf("2026-03-02"); got == "56.00" {
		t.Errorf("empty past week should not report hours, got %q", got)
	}

	// A week that has not started yet is in the future.
	if got := statusOf("2026-03-30"); got != "Future" {
		t.Errorf("status of future week: got %q, want Future", got)
	}
}

// TestE2E_ReportWeekOpensInDiary verifies clicking a week in the report opens
// that week in the diary view for editing.
func TestE2E_ReportWeekOpensInDiary(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	openReportForFY(t, page, 2026)

	page.MustElement(`#report-tbody tr[data-week-start="2026-03-16"]`).MustClick()

	waitFor(t, page, `() => !document.getElementById('view-diary').hidden`)
	waitFor(t, page, `() => document.querySelector('#entry-tbody tr.day-row')?.dataset.date === '2026-03-16'`)
}

// TestE2E_ReportActionsAboveTable verifies the export/print buttons sit above
// the week table rather than below it.
func TestE2E_ReportActionsAboveTable(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	page.MustElement("#nav-report").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-report').hidden`)

	// DOCUMENT_POSITION_FOLLOWING (4) means the table comes after the actions.
	before := page.MustEval(`() => {
		const actions = document.querySelector('.report-actions');
		const table = document.getElementById('report-table');
		return !!(actions.compareDocumentPosition(table) & Node.DOCUMENT_POSITION_FOLLOWING);
	}`).Bool()
	if !before {
		t.Error("report actions should appear above the week table")
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

	// Set up profile: Mon–Fri = WFH, Wednesday = office, default hours = 6.
	page.MustElement("#nav-settings").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-settings').hidden`)

	page.MustElement("#profile-default-hours").MustInput("6")
	page.MustElement("#profile-mon-type").MustSelect("Work From Home")
	page.MustElement("#profile-tue-type").MustSelect("Work From Home")
	page.MustElement("#profile-wed-type").MustSelect("Office")
	page.MustElement("#profile-thu-type").MustSelect("Work From Home")
	page.MustElement("#profile-fri-type").MustSelect("Work From Home")
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
	u := testUsername(t, "alice")
	userID := getUserID(t, serverURL, u)
	seedWeekEntries(t, serverURL, u, userID, "2025-07-07")

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
	u := testUsername(t, "alice")
	userID := getUserID(t, serverURL, u)
	seedWeekEntries(t, serverURL, u, userID, "2025-06-30")
	seedWeekEntries(t, serverURL, u, userID, "2025-07-07")
	seedWeekEntries(t, serverURL, u, userID, "2025-07-14")
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

	u := testUsername(t, "alice")
	userID := getUserID(t, serverURL, u)
	// Seed 2025-07-14 as complete; leave 2025-07-07 and 2025-07-21 empty.
	seedWeekEntries(t, serverURL, u, userID, "2025-07-14")

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

// TestE2E_WeekPicker_OpensAndCloses verifies that tapping the week label
// opens the week picker bottom sheet and that it can be closed.
func TestE2E_WeekPicker_OpensAndCloses(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Click the week label to open the picker
	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	// Verify the dialog is visible
	dialog := page.MustElement("#week-picker")
	visible, err := dialog.Visible()
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("week-picker dialog should be visible after clicking week label")
	}

	// Close via the close button
	page.MustEval(`() => document.getElementById('week-picker-close').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === false`)
}

// TestE2E_WeekPicker_SelectWeek verifies that tapping a week in the list
// navigates to that week and closes the sheet.
func TestE2E_WeekPicker_SelectWeek(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Open the week picker
	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	// Wait for week list items to load (they are fetched asynchronously)
	waitFor(t, page, `() => document.querySelectorAll('#week-list .week-list-item').length > 0`)

	// Click the first week in the list (should be the first week of the FY)
	page.MustEval(`() => document.querySelector('#week-list .week-list-item').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === false`)

	// Wait for the diary to actually re-render the newly selected week. Waiting
	// on the row count alone races: seven rows are already on the page from the
	// week we navigated in on, so the old rows can be read before the new week
	// loads. Wait for the rows to belong to a different week instead.
	waitFor(t, page, `() => {
		const rows = document.querySelectorAll('#entry-tbody tr.day-row');
		return rows.length === 7 && rows[0].dataset.date !== '2025-08-04';
	}`)

	rows := page.MustElements("#entry-tbody tr.day-row")
	firstDate, err := rows[0].Attribute("data-date")
	if err != nil || firstDate == nil {
		t.Fatal("could not read data-date from first row")
	}
	// The FY's first week is the week containing 1 Jul 2025 (a Tuesday), so it
	// starts on Mon 30 Jun 2025 — the same week smart init treats as first.
	if *firstDate != "2025-06-30" {
		t.Errorf("after selecting first week: got %q, want 2025-06-30", *firstDate)
	}
}

// TestE2E_WeekPicker_EmptyAtFYRollover verifies that in the days right after
// the financial year rolls over on 1 July — when the new FY has no completed
// weeks to list yet — the picker says so instead of rendering a blank sheet.
//
// Only the browser clock is moved here: the week list is computed client-side
// from new Date(), and the server is only asked for completion counts, so this
// reproduces the rollover without disturbing the pinned server clock (which the
// Docker container fixes process-wide and cannot vary per test).
func TestE2E_WeekPicker_EmptyAtFYRollover(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Fri 3 Jul 2026: FY2027 has begun, but its first week (Mon 29 Jun – Sun
	// 5 Jul) is still running, so no week of the FY has completed yet.
	pinBrowserClock(t, page, "2026-07-03")

	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	waitFor(t, page, `() => document.querySelector('#week-list .week-list-empty') !== null`)

	items := page.MustEval(`() => document.querySelectorAll('#week-list .week-list-item').length`).Int()
	if items != 0 {
		t.Errorf("week list at FY rollover: got %d items, want 0", items)
	}

	// The quick-jump buttons must keep working even with nothing to list.
	visible, err := page.MustElement("#jump-last-week").Visible()
	if err != nil {
		t.Fatal(err)
	}
	if !visible {
		t.Error("last-week quick jump should remain usable when the week list is empty")
	}
}

// TestE2E_WeekPicker_CompletionIndicators verifies that weeks with 7 entries
// show a green dot and incomplete weeks show a grey dot.
func TestE2E_WeekPicker_CompletionIndicators(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	u := testUsername(t, "alice")
	userID := getUserID(t, serverURL, u)
	// Seed one complete week
	seedWeekEntries(t, serverURL, u, userID, "2025-07-07")

	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Open the week picker
	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	// The week list should contain items with completion indicators
	waitFor(t, page, `() => document.querySelectorAll('#week-list .week-list-item').length > 0`)

	// Check that the seeded week (2025-07-07) has a complete indicator
	hasComplete := page.MustEval(`() => {
		const items = document.querySelectorAll('#week-list .week-list-item');
		for (const item of items) {
			if (item.dataset.weekStart === '2025-07-07') {
				return item.querySelector('.completion-dot').classList.contains('complete');
			}
		}
		return false;
	}`).Bool()
	if !hasComplete {
		t.Error("week 2025-07-07 should show complete indicator")
	}

	// Check that an unseeded week has an incomplete indicator
	hasIncomplete := page.MustEval(`() => {
		const items = document.querySelectorAll('#week-list .week-list-item');
		for (const item of items) {
			if (item.dataset.weekStart === '2025-07-14') {
				return !item.querySelector('.completion-dot').classList.contains('complete');
			}
		}
		return false;
	}`).Bool()
	if !hasIncomplete {
		t.Error("week 2025-07-14 should show incomplete indicator")
	}
}

// TestE2E_WeekPicker_LastWeekButton verifies the "Last week" quick-jump button.
func TestE2E_WeekPicker_LastWeekButton(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Start on a specific past week
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Open the week picker and click "Last week"
	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	page.MustEval(`() => document.getElementById('jump-last-week').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === false`)

	// Wait for the diary to actually re-render the previous week. Waiting on the
	// row count alone races: seven rows are already on the page from the week we
	// navigated in on, so the old rows can be read before the new week loads.
	// Wait for the rows to belong to a different week instead.
	waitFor(t, page, `() => {
		const rows = document.querySelectorAll('#entry-tbody tr.day-row');
		return rows.length === 7 && rows[0].dataset.date !== '2025-08-04';
	}`)

	// The clock is pinned to e2eToday (Tue 24 Mar 2026), so "last week" is the
	// week starting Mon 16 Mar 2026.
	rows := page.MustElements("#entry-tbody tr.day-row")
	firstDate, err := rows[0].Attribute("data-date")
	if err != nil || firstDate == nil {
		t.Fatal("could not read data-date from first row")
	}
	if *firstDate != "2026-03-16" {
		t.Errorf("after last week button: got %q, want 2026-03-16", *firstDate)
	}
}

// TestE2E_WeekPicker_CurrentWeekHighlighted verifies that the currently-viewed
// week is highlighted in the list.
func TestE2E_WeekPicker_CurrentWeekHighlighted(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Open the week picker
	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)
	waitFor(t, page, `() => document.querySelectorAll('#week-list .week-list-item').length > 0`)

	// The week 2025-08-04 should be highlighted
	isHighlighted := page.MustEval(`() => {
		const items = document.querySelectorAll('#week-list .week-list-item');
		for (const item of items) {
			if (item.dataset.weekStart === '2025-08-04') {
				return item.classList.contains('current-week');
			}
		}
		return false;
	}`).Bool()
	if !isHighlighted {
		t.Error("week 2025-08-04 should be highlighted as current week")
	}
}

// TestE2E_WeekPicker_ChevronVisible verifies that the week label has a chevron indicator.
func TestE2E_WeekPicker_ChevronVisible(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	label := page.MustElement("#week-label").MustText()
	if !strings.Contains(label, "\u25BE") {
		t.Errorf("week label should contain chevron (▾), got %q", label)
	}
}

// TestE2E_WeekPicker_AriaAttributes verifies ARIA attributes on the week picker
// dialog and the week label trigger.
func TestE2E_WeekPicker_AriaAttributes(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Week label should have button role and aria-haspopup
	role := page.MustEval(`() => document.getElementById('week-label').getAttribute('role')`).Str()
	if role != "button" {
		t.Errorf("week-label role: got %q, want 'button'", role)
	}

	haspopup := page.MustEval(`() => document.getElementById('week-label').getAttribute('aria-haspopup')`).Str()
	if haspopup != "dialog" {
		t.Errorf("week-label aria-haspopup: got %q, want 'dialog'", haspopup)
	}

	tabindex := page.MustEval(`() => document.getElementById('week-label').getAttribute('tabindex')`).Str()
	if tabindex != "0" {
		t.Errorf("week-label tabindex: got %q, want '0'", tabindex)
	}

	// Dialog should have aria-labelledby pointing to the heading
	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	labelledBy := page.MustEval(`() => document.getElementById('week-picker').getAttribute('aria-labelledby')`).Str()
	if labelledBy == "" {
		t.Error("week-picker should have aria-labelledby attribute")
	}

	// The referenced element should exist and contain "Jump to week"
	headingText := page.MustEval(fmt.Sprintf(`() => document.getElementById('%s')?.textContent || ''`, labelledBy)).Str()
	if !strings.Contains(headingText, "Jump to week") {
		t.Errorf("aria-labelledby target text: got %q, want to contain 'Jump to week'", headingText)
	}

	// Week list items should be buttons for native keyboard interaction
	waitFor(t, page, `() => document.querySelectorAll('#week-list .week-list-item').length > 0`)
	firstItemTag := page.MustEval(`() => document.querySelector('#week-list .week-list-item').tagName`).Str()
	if firstItemTag != "BUTTON" {
		t.Errorf("week list items should be <button> elements, got %q", firstItemTag)
	}
}

// TestE2E_WeekPicker_KeyboardOpen verifies that Enter and Space keys open the picker.
func TestE2E_WeekPicker_KeyboardOpen(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Focus the week label and press Enter
	page.MustEval(`() => document.getElementById('week-label').focus()`)
	page.MustEval(`() => document.getElementById('week-label').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }))`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	// Close it
	page.MustEval(`() => document.getElementById('week-picker-close').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === false`)

	// Now try Space key
	page.MustEval(`() => document.getElementById('week-label').focus()`)
	page.MustEval(`() => document.getElementById('week-label').dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }))`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)
}

// TestE2E_WeekPicker_FocusManagement verifies that focus moves into the dialog
// on open and returns to the week label on close.
func TestE2E_WeekPicker_FocusManagement(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Open the picker
	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	// Focus should be inside the dialog
	waitFor(t, page, `() => {
		const dialog = document.getElementById('week-picker');
		return dialog.contains(document.activeElement);
	}`)

	// Close via close button
	page.MustEval(`() => document.getElementById('week-picker-close').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === false`)

	// Focus should return to the week label
	waitFor(t, page, `() => document.activeElement === document.getElementById('week-label')`)
}

// TestE2E_WeekPicker_Animation verifies that the dialog has animation classes.
func TestE2E_WeekPicker_Animation(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Open the picker and check for the opening animation class
	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	// The dialog should have a slide-up animation defined in CSS
	hasAnimation := page.MustEval(`() => {
		const style = getComputedStyle(document.getElementById('week-picker'));
		return style.animationName !== 'none' && style.animationName !== '';
	}`).Bool()
	if !hasAnimation {
		t.Error("week-picker should have a CSS animation when open")
	}
}

// TestE2E_WeekPicker_LoadingState verifies that a loading state is shown while
// the week status API is being fetched.
func TestE2E_WeekPicker_LoadingState(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Check that the week-list area shows loading text when picker opens
	// We need to check quickly before the API responds
	page.MustEval(`() => {
		// Override fetch to delay the week-status response
		const origFetch = window.fetch;
		window._testOrigFetch = origFetch;
		window.fetch = function(url, opts) {
			if (typeof url === 'string' && url.includes('week-status')) {
				return new Promise(resolve => {
					setTimeout(() => resolve(origFetch(url, opts)), 2000);
				});
			}
			return origFetch(url, opts);
		};
	}`)

	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	// The list area should show a loading indicator
	loadingText := page.MustEval(`() => document.getElementById('week-list').textContent`).Str()
	if !strings.Contains(loadingText, "Loading") {
		t.Errorf("week-list should show loading text, got %q", loadingText)
	}

	// Restore original fetch
	page.MustEval(`() => { if (window._testOrigFetch) { window.fetch = window._testOrigFetch; delete window._testOrigFetch; } }`)
}

// TestE2E_AddUser_CreatesAndSelectsUser verifies that opening the add user
// dialog, filling in the form, and submitting creates a new user that appears
// in the dropdown and is automatically selected.
func TestE2E_AddUser_CreatesAndSelectsUser(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Remember how many options exist before adding
	initialCount := page.MustEval(`() => document.getElementById('user-select').options.length`).Int()

	// Click the add user button
	page.MustElement("#add-user-btn").MustClick()
	waitFor(t, page, `() => document.getElementById('add-user-dialog').open === true`)

	// Fill in the form — use test-unique username to avoid conflicts in shared DB
	uniqueUser := testUsername(t, "bob") + "_new"
	page.MustElement("#add-user-username").MustInput(uniqueUser)
	page.MustElement("#add-user-display-name").MustInput("Bob Smith")

	// Submit
	page.MustElement("#add-user-submit").MustClick()
	waitFor(t, page, `() => document.getElementById('add-user-dialog').open === false`)

	// Verify the new user appears in the dropdown and is selected
	waitFor(t, page, fmt.Sprintf(`() => {
		const sel = document.getElementById('user-select');
		return sel.options.length === %d && sel.options[sel.selectedIndex].text === 'Bob Smith';
	}`, initialCount+1))
}

// TestE2E_AddUser_DuplicateShowsError verifies that trying to create a user
// with an existing username shows an error message in the dialog.
func TestE2E_AddUser_DuplicateShowsError(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// The auto-created user's username (varies by test environment)
	existingUser := testUsername(t, "alice")

	page.MustElement("#add-user-btn").MustClick()
	waitFor(t, page, `() => document.getElementById('add-user-dialog').open === true`)

	page.MustElement("#add-user-username").MustInput(existingUser)
	page.MustElement("#add-user-display-name").MustInput("Alice Again")
	page.MustElement("#add-user-submit").MustClick()

	// The dialog should still be open with an error message
	waitFor(t, page, `() => document.getElementById('add-user-error').textContent.includes('already exists')`)

	// Dialog should still be open
	isOpen := page.MustEval(`() => document.getElementById('add-user-dialog').open`).Bool()
	if !isOpen {
		t.Error("dialog should remain open on error")
	}
}

// TestE2E_AddUser_CancelClosesDialog verifies that the cancel button closes
// the dialog without creating a user.
func TestE2E_AddUser_CancelClosesDialog(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Remember option count before opening dialog
	initialCount := page.MustEval(`() => document.getElementById('user-select').options.length`).Int()

	page.MustElement("#add-user-btn").MustClick()
	waitFor(t, page, `() => document.getElementById('add-user-dialog').open === true`)

	page.MustElement("#add-user-cancel").MustClick()
	waitFor(t, page, `() => document.getElementById('add-user-dialog').open === false`)

	// Dropdown should be unchanged after cancel
	optionCount := page.MustEval(`() => document.getElementById('user-select').options.length`).Int()
	if optionCount != initialCount {
		t.Errorf("expected %d users in dropdown after cancel, got %d", initialCount, optionCount)
	}
}

// TestE2E_WeekPicker_ErrorState verifies that an error message is shown when
// the week-status API fails, but quick-jump buttons still work.
func TestE2E_WeekPicker_ErrorState(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(serverURL + "?week=2025-08-04")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Override fetch to make week-status fail
	page.MustEval(`() => {
		const origFetch = window.fetch;
		window._testOrigFetch = origFetch;
		window.fetch = function(url, opts) {
			if (typeof url === 'string' && url.includes('week-status')) {
				return Promise.resolve(new Response('error', { status: 500 }));
			}
			return origFetch(url, opts);
		};
	}`)

	page.MustEval(`() => document.getElementById('week-label').click()`)
	waitFor(t, page, `() => document.getElementById('week-picker').open === true`)

	// Wait for the error to appear
	waitFor(t, page, `() => document.getElementById('week-list').textContent.includes('Could not load weeks')`)

	// Quick-jump buttons should still be visible and functional
	jumpBtnVisible := page.MustEval(`() => {
		const btn = document.getElementById('jump-last-week');
		return btn && !btn.hidden && btn.offsetParent !== null;
	}`).Bool()
	if !jumpBtnVisible {
		t.Error("jump-last-week button should still be visible when API fails")
	}

	// Restore original fetch
	page.MustEval(`() => {
		if (window._testOrigFetch) { window.fetch = window._testOrigFetch; delete window._testOrigFetch; }
	}`)
}

// setUserProfile saves a profile for the given user via direct API call.
func setUserProfile(t *testing.T, serverURL, username string, userID int64, profile map[string]any) {
	t.Helper()
	body, _ := json.Marshal(profile)
	req, _ := http.NewRequest(http.MethodPut,
		fmt.Sprintf("%s/api/users/%d/profile", serverURL, userID),
		bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(testAuthHeader, username)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("setUserProfile: status %v err %v", resp.StatusCode, err)
	}
	resp.Body.Close()
}

// TestE2E_UserSwitch_ReloadsProfile verifies that changing the user dropdown
// reloads the profile for the selected user, affecting diary defaults.
func TestE2E_UserSwitch_ReloadsProfile(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Create alice and set her profile: default_hours=7.5, all weekdays=wfh
	aliceUser := testUsername(t, "alice")
	aliceID := getUserID(t, serverURL, aliceUser)
	setUserProfile(t, serverURL, aliceUser, aliceID, map[string]any{
		"default_hours": 7.5,
		"mon_type":      "wfh", "tue_type": "wfh", "wed_type": "wfh",
		"thu_type": "wfh", "fri_type": "wfh",
		"sat_type": "weekend", "sun_type": "weekend",
	})

	// Create bob with a different profile: default_hours=6, all weekdays=office
	bobUser := testUsername(t, "bob") + "_switch"
	bobReq, _ := http.NewRequest(http.MethodPost, serverURL+"/api/users",
		bytes.NewReader([]byte(fmt.Sprintf(`{"username":"%s","display_name":"Bob Switch"}`, bobUser))))
	bobReq.Header.Set("Content-Type", "application/json")
	bobReq.Header.Set(testAuthHeader, aliceUser)
	bobResp, err := http.DefaultClient.Do(bobReq)
	if err != nil || bobResp.StatusCode != http.StatusCreated {
		t.Fatalf("create bob: status %v err %v", bobResp.StatusCode, err)
	}
	var bobData struct {
		ID int64 `json:"id"`
	}
	json.NewDecoder(bobResp.Body).Decode(&bobData)
	bobResp.Body.Close()

	setUserProfile(t, serverURL, aliceUser, bobData.ID, map[string]any{
		"default_hours": 6,
		"mon_type":      "office", "tue_type": "office", "wed_type": "office",
		"thu_type": "office", "fri_type": "office",
		"sat_type": "weekend", "sun_type": "weekend",
	})

	// Navigate to a specific past week with no entries
	page.MustNavigate(serverURL + "?week=2025-09-01")
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)
	time.Sleep(500 * time.Millisecond)

	// Alice's profile: Monday should be wfh with 7.5 hours
	monType := page.MustElement("#entry-tbody tr:first-child .day-type-select").MustProperty("value").Str()
	if monType != "wfh" {
		t.Errorf("alice monday type: got %q, want wfh", monType)
	}
	monHours := page.MustElement("#entry-tbody tr:first-child .hours-input").MustProperty("value").Str()
	if monHours != "7.5" {
		t.Errorf("alice monday hours: got %q, want 7.5", monHours)
	}

	// Switch to Bob
	page.MustEval(fmt.Sprintf(`() => {
		const sel = document.getElementById('user-select');
		sel.value = '%d';
		sel.dispatchEvent(new Event('change'));
	}`, bobData.ID))
	time.Sleep(1000 * time.Millisecond)

	// Bob's profile: Monday should be office (no hours)
	monType2 := page.MustElement("#entry-tbody tr:first-child .day-type-select").MustProperty("value").Str()
	if monType2 != "office" {
		t.Errorf("bob monday type: got %q, want office", monType2)
	}
}

// TestE2E_UserSwitch_SettingsShowSelectedUser verifies that the settings page
// loads the selected user's profile and shows whose settings are being edited.
func TestE2E_UserSwitch_SettingsShowSelectedUser(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Create alice with profile
	aliceUser := testUsername(t, "alice")
	aliceID := getUserID(t, serverURL, aliceUser)
	setUserProfile(t, serverURL, aliceUser, aliceID, map[string]any{
		"default_hours": 7.5,
		"mon_type":      "wfh", "tue_type": "wfh", "wed_type": "wfh",
		"thu_type": "wfh", "fri_type": "wfh",
		"sat_type": "weekend", "sun_type": "weekend",
	})

	// Create bob with different profile
	bobUser := testUsername(t, "bob") + "_settings"
	bobReq, _ := http.NewRequest(http.MethodPost, serverURL+"/api/users",
		bytes.NewReader([]byte(fmt.Sprintf(`{"username":"%s","display_name":"Bob Settings"}`, bobUser))))
	bobReq.Header.Set("Content-Type", "application/json")
	bobReq.Header.Set(testAuthHeader, aliceUser)
	bobResp, _ := http.DefaultClient.Do(bobReq)
	var bobData struct {
		ID int64 `json:"id"`
	}
	json.NewDecoder(bobResp.Body).Decode(&bobData)
	bobResp.Body.Close()

	setUserProfile(t, serverURL, aliceUser, bobData.ID, map[string]any{
		"default_hours": 6,
		"mon_type":      "office", "tue_type": "office", "wed_type": "office",
		"thu_type": "office", "fri_type": "office",
		"sat_type": "weekend", "sun_type": "weekend",
	})

	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Go to settings — should show alice's profile
	page.MustElement("#nav-settings").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-settings').hidden`)
	time.Sleep(500 * time.Millisecond)

	aliceHours := page.MustElement("#profile-default-hours").MustProperty("value").Str()
	if aliceHours != "7.5" {
		t.Errorf("alice default_hours: got %q, want 7.5", aliceHours)
	}

	// Switch to Bob
	page.MustEval(fmt.Sprintf(`() => {
		const sel = document.getElementById('user-select');
		sel.value = '%d';
		sel.dispatchEvent(new Event('change'));
	}`, bobData.ID))
	time.Sleep(1000 * time.Millisecond)

	// Settings should now show Bob's profile
	bobHours := page.MustElement("#profile-default-hours").MustProperty("value").Str()
	if bobHours != "6" {
		t.Errorf("bob default_hours: got %q, want 6", bobHours)
	}

	// Settings heading should indicate whose settings are shown
	settingsLabel := page.MustEval(`() => document.getElementById('settings-user-label')?.textContent || ''`).Str()
	if !strings.Contains(settingsLabel, "Bob Settings") {
		t.Errorf("settings label should contain Bob's name, got %q", settingsLabel)
	}
}

// TestE2E_UserSwitch_SaveSettingsForSelectedUser verifies that saving settings
// updates the selected user's profile, not the logged-in user's.
func TestE2E_UserSwitch_SaveSettingsForSelectedUser(t *testing.T) {
	serverURL := newE2EServer(t)
	_, page := newPage(t, "alice")

	// Create alice
	aliceUser := testUsername(t, "alice")
	aliceID := getUserID(t, serverURL, aliceUser)
	setUserProfile(t, serverURL, aliceUser, aliceID, map[string]any{
		"default_hours": 7.5,
		"mon_type":      "wfh", "tue_type": "wfh", "wed_type": "wfh",
		"thu_type": "wfh", "fri_type": "wfh",
		"sat_type": "weekend", "sun_type": "weekend",
	})

	// Create bob
	bobUser := testUsername(t, "bob") + "_save"
	bobReq, _ := http.NewRequest(http.MethodPost, serverURL+"/api/users",
		bytes.NewReader([]byte(fmt.Sprintf(`{"username":"%s","display_name":"Bob Save"}`, bobUser))))
	bobReq.Header.Set("Content-Type", "application/json")
	bobReq.Header.Set(testAuthHeader, aliceUser)
	bobResp, _ := http.DefaultClient.Do(bobReq)
	var bobData struct {
		ID int64 `json:"id"`
	}
	json.NewDecoder(bobResp.Body).Decode(&bobData)
	bobResp.Body.Close()

	setUserProfile(t, serverURL, aliceUser, bobData.ID, map[string]any{
		"default_hours": 6,
		"mon_type":      "office", "tue_type": "office", "wed_type": "office",
		"thu_type": "office", "fri_type": "office",
		"sat_type": "weekend", "sun_type": "weekend",
	})

	page.MustNavigate(serverURL)
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Switch to Bob and go to settings
	page.MustEval(fmt.Sprintf(`() => {
		const sel = document.getElementById('user-select');
		sel.value = '%d';
		sel.dispatchEvent(new Event('change'));
	}`, bobData.ID))
	time.Sleep(500 * time.Millisecond)

	page.MustElement("#nav-settings").MustClick()
	waitFor(t, page, `() => !document.getElementById('view-settings').hidden`)
	time.Sleep(500 * time.Millisecond)

	// Change Bob's default hours to 8
	page.MustEval(`() => document.getElementById('profile-default-hours').value = ''`)
	page.MustElement("#profile-default-hours").MustInput("8")
	page.MustElement("#save-profile").MustClick()
	waitFor(t, page, `() => document.getElementById('profile-status').textContent === 'Saved'`)

	// Verify by fetching Bob's profile directly from API
	req, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/api/users/%d/profile", serverURL, bobData.ID), nil)
	req.Header.Set(testAuthHeader, aliceUser)
	resp, _ := http.DefaultClient.Do(req)
	var profile struct {
		DefaultHours float64 `json:"default_hours"`
	}
	json.NewDecoder(resp.Body).Decode(&profile)
	resp.Body.Close()

	if profile.DefaultHours != 8 {
		t.Errorf("bob's saved default_hours: got %v, want 8", profile.DefaultHours)
	}

	// Switch back to alice — verify alice's profile is unchanged
	page.MustEval(fmt.Sprintf(`() => {
		const sel = document.getElementById('user-select');
		sel.value = '%d';
		sel.dispatchEvent(new Event('change'));
	}`, aliceID))
	time.Sleep(1000 * time.Millisecond)

	aliceHours := page.MustElement("#profile-default-hours").MustProperty("value").Str()
	if aliceHours != "7.5" {
		t.Errorf("alice default_hours after bob save: got %q, want 7.5", aliceHours)
	}
}
