package clock

import (
	"testing"
	"time"
)

func TestFixPinsToMiddayUTC(t *testing.T) {
	t.Cleanup(func() { Now = time.Now })

	Fix(time.Date(2026, time.March, 24, 0, 0, 0, 0, time.UTC))

	got := Now()
	want := time.Date(2026, time.March, 24, 12, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("Now() = %v, want %v", got, want)
	}

	// Midday UTC keeps the calendar date intact for any offset within ±11h,
	// which spans every timezone the app runs in (Australia/Melbourne is +11
	// at its furthest, during daylight saving).
	for _, offset := range []int{-11, 0, 10, 11} {
		loc := time.FixedZone("test", offset*3600)
		if d := Now().In(loc).Day(); d != 24 {
			t.Errorf("pinned date at UTC%+d = day %d, want 24", offset, d)
		}
	}
}

func TestFixFromEnv(t *testing.T) {
	t.Cleanup(func() { Now = time.Now })

	t.Run("unset leaves the real clock alone", func(t *testing.T) {
		_, pinned, err := FixFromEnv()
		if err != nil || pinned {
			t.Fatalf("FixFromEnv() = pinned %v, err %v; want pinned false, no error", pinned, err)
		}
	})

	t.Run("valid date pins the clock", func(t *testing.T) {
		t.Setenv(TestTodayEnv, "2026-03-24")

		today, pinned, err := FixFromEnv()
		if err != nil || !pinned {
			t.Fatalf("FixFromEnv() = pinned %v, err %v; want pinned true, no error", pinned, err)
		}
		if got := today.Format("2006-01-02"); got != "2026-03-24" {
			t.Errorf("pinned date = %s, want 2026-03-24", got)
		}
		if !Now().Equal(today) {
			t.Errorf("Now() = %v, want %v", Now(), today)
		}
	})

	t.Run("unparseable date is an error, not a silent fallback", func(t *testing.T) {
		t.Setenv(TestTodayEnv, "24/03/2026")

		if _, pinned, err := FixFromEnv(); err == nil || pinned {
			t.Errorf("FixFromEnv() = pinned %v, err %v; want an error", pinned, err)
		}
	})
}
