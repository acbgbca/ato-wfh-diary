//go:build e2e && !e2e_docker

package e2e_test

import (
	"net/http/httptest"
	"testing"
	"time"

	"ato-wfh-diary/frontend"
	"ato-wfh-diary/internal/api/handlers"
	"ato-wfh-diary/internal/clock"
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/migrations"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const localAuthHeader = "X-Test-User"

// testAuthHeader is the HTTP header used to authenticate direct API calls in tests.
var testAuthHeader = localAuthHeader

// testUsername returns the username to use for API calls, consistent with
// what newPage sets on the browser for this test.
func testUsername(_ *testing.T, fallback string) string { return fallback }

// newE2EServer starts a test HTTP server with a real in-memory SQLite database
// and the embedded frontend. Returns the server's base URL.
func newE2EServer(t *testing.T) string {
	t.Helper()
	pinServerClock(t)
	database, err := db.Open(":memory:", migrations.FS)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	store := db.NewStore(database)
	h := handlers.New(store)
	router := handlers.NewRouter(h, localAuthHeader, frontend.FS, "test")
	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		database.Close()
	})
	return srv.URL
}

// pinServerClock makes the in-process server believe today is e2eToday. The
// Docker harness does the same thing through the WFH_TEST_TODAY environment
// variable, which the container's entrypoint reads at startup.
func pinServerClock(t *testing.T) {
	t.Helper()
	today, err := time.Parse("2006-01-02", e2eToday)
	if err != nil {
		t.Fatalf("parse e2eToday: %v", err)
	}
	clock.Fix(today)
	t.Cleanup(func() { clock.Now = time.Now })
}

// newPage launches a headless browser page pre-authenticated as username.
func newPage(t *testing.T, username string) (*rod.Browser, *rod.Page) {
	t.Helper()

	// NoSandbox is required on hosts where Chrome's sandbox cannot initialise —
	// e.g. GitHub's Ubuntu 24.04 runners, which restrict unprivileged user
	// namespaces via AppArmor. rod only auto-adds it when it detects a
	// container, which is not the case on a runner VM, so set it explicitly.
	l := launcher.New().Headless(true).NoSandbox(true)
	if path, ok := launcher.LookPath(); ok {
		l = l.Bin(path)
	}
	controlURL := l.MustLaunch()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	t.Cleanup(func() { browser.MustClose() })

	page := browser.MustPage("").Timeout(15 * time.Second)
	pinBrowserClock(t, page, e2eToday)
	cleanup, err := page.SetExtraHeaders([]string{localAuthHeader, username})
	if err != nil {
		t.Fatalf("set extra headers: %v", err)
	}
	t.Cleanup(cleanup)

	return browser, page
}
