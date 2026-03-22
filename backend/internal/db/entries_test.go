package db_test

import (
	"ato-wfh-diary/internal/model"
	"context"
	"testing"
	"time"
)

// monday returns a Monday at midnight UTC for the given year/month/day.
func monday(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// seedUser creates a test user and fails the test if it errors.
func seedUser(t *testing.T, s interface {
	UpsertUser(context.Context, string, string) (*model.User, error)
}, username, name string) *model.User {
	t.Helper()
	u, err := s.UpsertUser(context.Background(), username, name)
	if err != nil {
		t.Fatalf("seedUser %q: %v", username, err)
	}
	return u
}

func TestUpsertEntries_Create(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	weekStart := monday(2025, 1, 6) // Mon 6 Jan 2025
	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: weekStart, DayType: model.DayTypeWFH, Hours: 8},
		{UserID: user.ID, EntryDate: weekStart.AddDate(0, 0, 1), DayType: model.DayTypeOffice, Hours: 0},
		{UserID: user.ID, EntryDate: weekStart.AddDate(0, 0, 2), DayType: model.DayTypePartWFH, Hours: 4.5},
	}

	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	got, err := s.GetWeekEntries(ctx, user.ID, weekStart)
	if err != nil {
		t.Fatalf("GetWeekEntries: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	if got[0].Hours != 8 {
		t.Errorf("entry 0 hours: got %v, want 8", got[0].Hours)
	}
	if got[2].Hours != 4.5 {
		t.Errorf("entry 2 hours: got %v, want 4.5", got[2].Hours)
	}
}

func TestUpsertEntries_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	weekStart := monday(2025, 1, 6)
	date := weekStart

	initial := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: date, DayType: model.DayTypeOffice, Hours: 0},
	}
	if err := s.UpsertEntries(ctx, initial); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	// Correct the entry: was office, actually WFH.
	updated := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: date, DayType: model.DayTypeWFH, Hours: 7.5},
	}
	if err := s.UpsertEntries(ctx, updated); err != nil {
		t.Fatalf("update upsert: %v", err)
	}

	got, err := s.GetWeekEntries(ctx, user.ID, weekStart)
	if err != nil {
		t.Fatalf("GetWeekEntries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].DayType != model.DayTypeWFH {
		t.Errorf("day_type: got %q, want %q", got[0].DayType, model.DayTypeWFH)
	}
	if got[0].Hours != 7.5 {
		t.Errorf("hours: got %v, want 7.5", got[0].Hours)
	}
}

func TestGetWeekEntries_OnlyReturnsRequestedWeek(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	week1 := monday(2025, 1, 6)  // Mon 6 Jan
	week2 := monday(2025, 1, 13) // Mon 13 Jan

	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: week1, DayType: model.DayTypeWFH, Hours: 8},
		{UserID: user.ID, EntryDate: week2, DayType: model.DayTypeWFH, Hours: 8},
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	got, err := s.GetWeekEntries(ctx, user.ID, week1)
	if err != nil {
		t.Fatalf("GetWeekEntries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry for week1, got %d", len(got))
	}
	if !got[0].EntryDate.Equal(week1) {
		t.Errorf("entry date: got %v, want %v", got[0].EntryDate, week1)
	}
}

func TestGetWeekEntries_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	got, err := s.GetWeekEntries(ctx, user.ID, monday(2025, 1, 6))
	if err != nil {
		t.Fatalf("GetWeekEntries: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

func TestGetWeekEntries_IsolatedByUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := seedUser(t, s, "alice", "Alice Smith")
	bob := seedUser(t, s, "bob", "Bob Brown")

	weekStart := monday(2025, 1, 6)
	entries := []model.WorkDayEntry{
		{UserID: alice.ID, EntryDate: weekStart, DayType: model.DayTypeWFH, Hours: 8},
		{UserID: bob.ID, EntryDate: weekStart, DayType: model.DayTypeOffice, Hours: 0},
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	aliceEntries, err := s.GetWeekEntries(ctx, alice.ID, weekStart)
	if err != nil {
		t.Fatalf("GetWeekEntries alice: %v", err)
	}
	if len(aliceEntries) != 1 || aliceEntries[0].UserID != alice.ID {
		t.Errorf("alice entries: expected 1 entry owned by alice")
	}

	bobEntries, err := s.GetWeekEntries(ctx, bob.ID, weekStart)
	if err != nil {
		t.Fatalf("GetWeekEntries bob: %v", err)
	}
	if len(bobEntries) != 1 || bobEntries[0].UserID != bob.ID {
		t.Errorf("bob entries: expected 1 entry owned by bob")
	}
}

func TestGetFYWFHEntries_OnlyWFHTypes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	// FY2025: 1 Jul 2024 – 30 Jun 2025
	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: date(2024, 8, 1), DayType: model.DayTypeWFH, Hours: 8},
		{UserID: user.ID, EntryDate: date(2024, 8, 2), DayType: model.DayTypePartWFH, Hours: 3},
		{UserID: user.ID, EntryDate: date(2024, 8, 3), DayType: model.DayTypeOffice, Hours: 0},
		{UserID: user.ID, EntryDate: date(2024, 8, 4), DayType: model.DayTypeAnnualLeave, Hours: 0},
		{UserID: user.ID, EntryDate: date(2024, 8, 5), DayType: model.DayTypeSickLeave, Hours: 0},
		{UserID: user.ID, EntryDate: date(2024, 8, 6), DayType: model.DayTypePublicHoliday, Hours: 0},
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	got, err := s.GetFYWFHEntries(ctx, user.ID, 2025)
	if err != nil {
		t.Fatalf("GetFYWFHEntries: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 WFH entries, got %d", len(got))
	}
	if got[0].DayType != model.DayTypeWFH {
		t.Errorf("entry 0 type: got %q, want wfh", got[0].DayType)
	}
	if got[1].DayType != model.DayTypePartWFH {
		t.Errorf("entry 1 type: got %q, want part_wfh", got[1].DayType)
	}
}

func TestGetFYWFHEntries_FinancialYearBoundaries(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	entries := []model.WorkDayEntry{
		// FY2025: 1 Jul 2024 – 30 Jun 2025
		{UserID: user.ID, EntryDate: date(2024, 7, 1), DayType: model.DayTypeWFH, Hours: 8},  // first day of FY2025
		{UserID: user.ID, EntryDate: date(2025, 6, 30), DayType: model.DayTypeWFH, Hours: 8}, // last day of FY2025
		// FY2026: 1 Jul 2025 – 30 Jun 2026
		{UserID: user.ID, EntryDate: date(2025, 7, 1), DayType: model.DayTypeWFH, Hours: 8}, // first day of FY2026
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	fy2025, err := s.GetFYWFHEntries(ctx, user.ID, 2025)
	if err != nil {
		t.Fatalf("GetFYWFHEntries 2025: %v", err)
	}
	if len(fy2025) != 2 {
		t.Errorf("FY2025: expected 2 entries, got %d", len(fy2025))
	}

	fy2026, err := s.GetFYWFHEntries(ctx, user.ID, 2026)
	if err != nil {
		t.Fatalf("GetFYWFHEntries 2026: %v", err)
	}
	if len(fy2026) != 1 {
		t.Errorf("FY2026: expected 1 entry, got %d", len(fy2026))
	}
}

func TestGetFYWFHEntries_TotalHours(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: date(2024, 8, 1), DayType: model.DayTypeWFH, Hours: 8},
		{UserID: user.ID, EntryDate: date(2024, 8, 2), DayType: model.DayTypeWFH, Hours: 7.5},
		{UserID: user.ID, EntryDate: date(2024, 8, 5), DayType: model.DayTypePartWFH, Hours: 4},
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	got, err := s.GetFYWFHEntries(ctx, user.ID, 2025)
	if err != nil {
		t.Fatalf("GetFYWFHEntries: %v", err)
	}

	var total float64
	for _, e := range got {
		total += e.Hours
	}
	const want = 19.5
	if total != want {
		t.Errorf("total hours: got %v, want %v", total, want)
	}
}

func TestGetFYAllEntries_IncludesAllTypes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: date(2024, 8, 1), DayType: model.DayTypeWFH, Hours: 8},
		{UserID: user.ID, EntryDate: date(2024, 8, 2), DayType: model.DayTypeOffice, Hours: 0},
		{UserID: user.ID, EntryDate: date(2024, 8, 3), DayType: model.DayTypeAnnualLeave, Hours: 0},
		{UserID: user.ID, EntryDate: date(2024, 8, 4), DayType: model.DayTypeSickLeave, Hours: 0},
		{UserID: user.ID, EntryDate: date(2024, 8, 5), DayType: model.DayTypePublicHoliday, Hours: 0},
		{UserID: user.ID, EntryDate: date(2024, 8, 6), DayType: model.DayTypeWeekend, Hours: 0},
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	got, err := s.GetFYAllEntries(ctx, user.ID, 2025)
	if err != nil {
		t.Fatalf("GetFYAllEntries: %v", err)
	}
	if len(got) != 6 {
		t.Errorf("expected 6 entries, got %d", len(got))
	}
}

func TestUpsertEntries_WithNotes(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	weekStart := monday(2025, 1, 6)
	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: weekStart, DayType: model.DayTypePartWFH, Hours: 4, Notes: "morning only"},
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	got, err := s.GetWeekEntries(ctx, user.ID, weekStart)
	if err != nil {
		t.Fatalf("GetWeekEntries: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}
	if got[0].Notes != "morning only" {
		t.Errorf("notes: got %q, want %q", got[0].Notes, "morning only")
	}
}

// date is a convenience helper for creating a UTC midnight time.Time.
func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}
