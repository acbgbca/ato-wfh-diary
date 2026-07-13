// Package clock is the application's source of "today".
//
// Almost everything the app decides — which financial year is current, which
// weeks are in the past, which week to open on — is derived from the current
// date. Reading time.Now() directly at those call sites makes the behaviour
// untestable: the E2E suite drives the real UI against real dates, so on
// 1 July its expectations silently became wrong when the financial year rolled
// over. Routing "today" through this package lets the E2E runs pin it.
package clock

import (
	"fmt"
	"os"
	"time"
)

// TestTodayEnv pins "today" to the given YYYY-MM-DD date when set. It is read
// once at startup and is intended for E2E runs against the container image;
// production deployments leave it unset.
const TestTodayEnv = "WFH_TEST_TODAY"

// Now returns the current time. Tests replace it to pin a fixed date.
var Now = time.Now

// Fix pins Now to midday UTC on the given date. Midday (rather than midnight)
// keeps the pinned calendar date the same in every timezone the app runs in.
func Fix(date time.Time) {
	fixed := time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, time.UTC)
	Now = func() time.Time { return fixed }
}

// FixFromEnv pins Now if TestTodayEnv is set. It reports whether the clock was
// pinned, and errors if the variable is set but unparseable.
func FixFromEnv() (time.Time, bool, error) {
	v := os.Getenv(TestTodayEnv)
	if v == "" {
		return time.Time{}, false, nil
	}
	date, err := time.Parse("2006-01-02", v)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%s must be a YYYY-MM-DD date: %w", TestTodayEnv, err)
	}
	Fix(date)
	return Now(), true, nil
}
