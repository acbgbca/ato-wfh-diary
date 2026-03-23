//go:build e2e

package e2e_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"ato-wfh-diary/frontend"
	"ato-wfh-diary/internal/api/handlers"
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/migrations"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// newE2EServer starts a test HTTP server with a real in-memory SQLite database
// and the embedded frontend. The auth header name is "X-Test-User".
func newE2EServer(t *testing.T) *httptest.Server {
	t.Helper()
	database, err := db.Open(":memory:", migrations.FS)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	store := db.NewStore(database)
	h := handlers.New(store)
	router := handlers.NewRouter(h, "X-Test-User", frontend.FS)
	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		database.Close()
	})
	return srv
}

// newPage launches a headless browser page with the Forward Auth header preset
// to username so every request is automatically authenticated.
func newPage(t *testing.T, username string) (*rod.Browser, *rod.Page) {
	t.Helper()

	l := launcher.New().Headless(true)
	if path, ok := launcher.LookPath(); ok {
		l = l.Bin(path)
	}
	controlURL := l.MustLaunch()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	t.Cleanup(func() { browser.MustClose() })

	page := browser.MustPage("")

	// Inject the Forward Auth header for every request from this page,
	// including fetch() calls made by JavaScript.  The high-level
	// SetExtraHeaders enables the network domain before applying the headers,
	// which is required for them to be attached to XHR/fetch requests.
	cleanup, err := page.SetExtraHeaders([]string{"X-Test-User", username})
	if err != nil {
		t.Fatalf("set extra headers: %v", err)
	}
	t.Cleanup(cleanup)

	return browser, page
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
	srv := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(srv.URL)

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
func TestE2E_SaveAndReloadEntry(t *testing.T) {
	srv := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(srv.URL)

	// Wait for the week rows to render.
	waitFor(t, page, `() => document.querySelectorAll('#entry-tbody tr.day-row').length === 7`)

	// Set Monday (first row) to WFH with 7.5 hours.
	firstRow := page.MustElement("#entry-tbody tr:first-child")
	firstRow.MustElement(".day-type-select").MustSelect("Work From Home")
	firstRow.MustElement(".hours-input").MustInput("7.5")

	// Save.
	page.MustElement("#save-entries").MustClick()
	waitFor(t, page, `() => document.getElementById('save-status').textContent === 'Saved'`)

	// Reload and verify data persisted.
	page.MustReload()
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
	srv := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(srv.URL)

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
	srv := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(srv.URL)

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
	srv := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(srv.URL)

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

// TestE2E_WeekNavigation verifies that Prev/Next week buttons update the
// week label and reload entries.
func TestE2E_WeekNavigation(t *testing.T) {
	srv := newE2EServer(t)
	_, page := newPage(t, "alice")
	page.MustNavigate(srv.URL)

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
