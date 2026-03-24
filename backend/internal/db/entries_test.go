package db_test

import (
	"ato-wfh-diary/internal/db"
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

// seedCompleteWeek inserts all 7 entries for the given Monday in a single call.
func seedEntry(t *testing.T, s *db.Store, userID int64, d time.Time) {
	t.Helper()
	if err := s.UpsertEntries(context.Background(), []model.WorkDayEntry{
		{UserID: userID, EntryDate: d, DayType: model.DayTypeOffice},
	}); err != nil {
		t.Fatalf("seedEntry %s: %v", d.Format("2006-01-02"), err)
	}
}

func seedCompleteWeek(t *testing.T, s *db.Store, userID int64, weekMonday time.Time) {
	t.Helper()
	entries := make([]model.WorkDayEntry, 7)
	for i := 0; i < 7; i++ {
		entries[i] = model.WorkDayEntry{
			UserID:    userID,
			EntryDate: weekMonday.AddDate(0, 0, i),
			DayType:   model.DayTypeOffice,
		}
	}
	if err := s.UpsertEntries(context.Background(), entries); err != nil {
		t.Fatalf("seedCompleteWeek: %v", err)
	}
}

func TestGetFirstIncompleteWeek_NoEntries(t *testing.T) {
	s := newTestStore(t)
	user := seedUser(t, s, "alice", "Alice Smith")

	// FY2026: first Monday = 2025-07-07
	fromDate := monday(2025, 7, 7)
	today := date(2025, 7, 9)

	result, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, fromDate, today)
	if err != nil {
		t.Fatalf("GetFirstIncompleteWeek: %v", err)
	}
	if result == nil {
		t.Fatal("expected a week start, got nil")
	}
	if !result.Equal(fromDate) {
		t.Errorf("week start: got %v, want %v", result, fromDate)
	}
}

func TestGetFirstIncompleteWeek_AllComplete(t *testing.T) {
	s := newTestStore(t)
	user := seedUser(t, s, "alice", "Alice Smith")

	weekStart := monday(2025, 7, 7)
	today := date(2025, 7, 9) // within same week

	seedCompleteWeek(t, s, user.ID, weekStart)

	result, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, weekStart, today)
	if err != nil {
		t.Fatalf("GetFirstIncompleteWeek: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (all complete), got %v", result)
	}
}

func TestGetFirstIncompleteWeek_FirstWeekIncomplete(t *testing.T) {
	s := newTestStore(t)
	user := seedUser(t, s, "alice", "Alice Smith")

	weekStart := monday(2025, 7, 7)
	today := date(2025, 7, 14)

	// Only 3 entries in the first week
	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: date(2025, 7, 7), DayType: model.DayTypeWFH, Hours: 8},
		{UserID: user.ID, EntryDate: date(2025, 7, 8), DayType: model.DayTypeOffice},
		{UserID: user.ID, EntryDate: date(2025, 7, 9), DayType: model.DayTypeOffice},
	}
	if err := s.UpsertEntries(context.Background(), entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	result, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, weekStart, today)
	if err != nil {
		t.Fatalf("GetFirstIncompleteWeek: %v", err)
	}
	if result == nil {
		t.Fatal("expected a week start, got nil")
	}
	if !result.Equal(weekStart) {
		t.Errorf("week start: got %v, want %v", result, weekStart)
	}
}

func TestGetFirstIncompleteWeek_SecondWeekIncomplete(t *testing.T) {
	s := newTestStore(t)
	user := seedUser(t, s, "alice", "Alice Smith")

	week1 := monday(2025, 7, 7)
	week2 := monday(2025, 7, 14)
	today := date(2025, 7, 16)

	seedCompleteWeek(t, s, user.ID, week1)
	// week2 incomplete: only 2 entries
	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: date(2025, 7, 14), DayType: model.DayTypeWFH, Hours: 8},
		{UserID: user.ID, EntryDate: date(2025, 7, 15), DayType: model.DayTypeOffice},
	}
	if err := s.UpsertEntries(context.Background(), entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	result, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, week1, today)
	if err != nil {
		t.Fatalf("GetFirstIncompleteWeek: %v", err)
	}
	if result == nil {
		t.Fatal("expected a week start, got nil")
	}
	if !result.Equal(week2) {
		t.Errorf("week start: got %v, want %v", result, week2)
	}
}

func TestGetFirstIncompleteWeek_RespectsFromDate(t *testing.T) {
	s := newTestStore(t)
	user := seedUser(t, s, "alice", "Alice Smith")

	week1 := monday(2025, 7, 7)  // incomplete
	week2 := monday(2025, 7, 14) // complete
	today := date(2025, 7, 16)

	// week1 is incomplete, week2 is complete
	seedCompleteWeek(t, s, user.ID, week2)

	// Start searching from week2 — should return nil since week2 is complete
	result, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, week2, today)
	if err != nil {
		t.Fatalf("GetFirstIncompleteWeek: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (from_date skips week1), got %v", result)
	}

	// Start searching from week1 — should return week1
	result2, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, week1, today)
	if err != nil {
		t.Fatalf("GetFirstIncompleteWeek week1: %v", err)
	}
	if result2 == nil {
		t.Fatal("expected week1, got nil")
	}
	if !result2.Equal(week1) {
		t.Errorf("week start: got %v, want %v", result2, week1)
	}
}

func TestGetFirstIncompleteWeek_DoesNotCheckFutureWeeks(t *testing.T) {
	s := newTestStore(t)
	user := seedUser(t, s, "alice", "Alice Smith")

	week1 := monday(2025, 7, 7)
	week2 := monday(2025, 7, 14) // future relative to today
	today := date(2025, 7, 9)    // Wednesday in week1

	// week1 complete, week2 empty but in the future
	seedCompleteWeek(t, s, user.ID, week1)

	result, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, week1, today)
	if err != nil {
		t.Fatalf("GetFirstIncompleteWeek: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (week2 is future), got %v", result)
	}
	_ = week2
}

// TestGetFirstIncompleteWeek_CrossFYBoundary checks that the week straddling the
// FY start (e.g. Mon 30 Jun for FY2026 where Jul 1 is Tuesday) is treated as
// complete when all days *within* the FY have entries, even though the Monday
// itself belongs to the previous FY.
func TestGetFirstIncompleteWeek_CrossFYBoundary(t *testing.T) {
	s := newTestStore(t)
	user := seedUser(t, s, "alice", "Alice")

	// FY2026: Jul 1 2025 is Tuesday → first week starts Mon 30 Jun 2025.
	// Seed entries for the 6 days that fall within FY2026 (Jul 1–6 inclusive).
	// Jun 30 is FY2025 and is intentionally NOT seeded.
	fy2026Days := []time.Time{
		date(2025, 7, 1), date(2025, 7, 2), date(2025, 7, 3),
		date(2025, 7, 4), date(2025, 7, 5), date(2025, 7, 6),
	}
	for _, d := range fy2026Days {
		seedEntry(t, s, user.ID, d)
	}

	fromDate := monday(2025, 6, 30) // Mon 30 Jun — firstMondayOfFY(2026)
	today := date(2025, 7, 6)       // within the same week

	result, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, fromDate, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil (all in-FY days complete), got %v", result.Format("2006-01-02"))
	}
}

// TestGetFirstIncompleteWeek_CrossFYBoundaryIncomplete checks that the cross-FY
// first week is returned as incomplete when some in-FY days are missing.
func TestGetFirstIncompleteWeek_CrossFYBoundaryIncomplete(t *testing.T) {
	s := newTestStore(t)
	user := seedUser(t, s, "alice", "Alice")

	// Seed only 5 of the 6 FY2026 days in the first week — missing Jul 6.
	for _, d := range []time.Time{
		date(2025, 7, 1), date(2025, 7, 2), date(2025, 7, 3),
		date(2025, 7, 4), date(2025, 7, 5),
	} {
		seedEntry(t, s, user.ID, d)
	}

	fromDate := monday(2025, 6, 30)
	today := date(2025, 7, 6)

	result, err := s.GetFirstIncompleteWeek(context.Background(), user.ID, 2026, fromDate, today)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected the cross-FY week to be returned as incomplete, got nil")
	}
	got := result.Format("2006-01-02")
	if got != "2025-06-30" {
		t.Errorf("got %q, want 2025-06-30", got)
	}
}

func TestGetFirstIncompleteWeek_IsolatedByUser(t *testing.T) {
	s := newTestStore(t)
	alice := seedUser(t, s, "alice", "Alice Smith")
	bob := seedUser(t, s, "bob", "Bob Brown")

	weekStart := monday(2025, 7, 7)
	today := date(2025, 7, 9)

	seedCompleteWeek(t, s, alice.ID, weekStart)
	// Bob has no entries

	aliceResult, err := s.GetFirstIncompleteWeek(context.Background(), alice.ID, 2026, weekStart, today)
	if err != nil {
		t.Fatalf("alice: %v", err)
	}
	if aliceResult != nil {
		t.Errorf("alice: expected nil, got %v", aliceResult)
	}

	bobResult, err := s.GetFirstIncompleteWeek(context.Background(), bob.ID, 2026, weekStart, today)
	if err != nil {
		t.Fatalf("bob: %v", err)
	}
	if bobResult == nil {
		t.Fatal("bob: expected a week start, got nil")
	}
}
