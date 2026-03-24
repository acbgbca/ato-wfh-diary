//go:build e2e && !e2e_docker

package e2e_test

import (
	"net/http/httptest"
	"testing"

	"ato-wfh-diary/frontend"
	"ato-wfh-diary/internal/api/handlers"
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/migrations"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

const localAuthHeader = "X-Test-User"

// newE2EServer starts a test HTTP server with a real in-memory SQLite database
// and the embedded frontend. Returns the server's base URL.
func newE2EServer(t *testing.T) string {
	t.Helper()
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

// newPage launches a headless browser page pre-authenticated as username.
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
	cleanup, err := page.SetExtraHeaders([]string{localAuthHeader, username})
	if err != nil {
		t.Fatalf("set extra headers: %v", err)
	}
	t.Cleanup(cleanup)

	return browser, page
}
