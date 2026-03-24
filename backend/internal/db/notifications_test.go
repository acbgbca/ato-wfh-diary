package db_test

import (
	"ato-wfh-diary/internal/model"
	"context"
	"testing"
	"time"
)

// ---- app_config ----

func TestGetOrSetAppConfig_CreatesOnFirstCall(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	val, err := s.GetOrSetAppConfig(ctx, "vapid_public_key", "default-pub")
	if err != nil {
		t.Fatalf("GetOrSetAppConfig: %v", err)
	}
	if val != "default-pub" {
		t.Errorf("value: got %q, want %q", val, "default-pub")
	}
}

func TestGetOrSetAppConfig_ReturnsExistingOnSecondCall(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.GetOrSetAppConfig(ctx, "vapid_public_key", "original"); err != nil {
		t.Fatalf("first call: %v", err)
	}

	val, err := s.GetOrSetAppConfig(ctx, "vapid_public_key", "should-not-overwrite")
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if val != "original" {
		t.Errorf("value: got %q, want %q", val, "original")
	}
}

// ---- notification prefs ----

func TestGetOrCreateNotificationPrefs_DefaultsForNewUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	prefs, err := s.GetOrCreateNotificationPrefs(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetOrCreateNotificationPrefs: %v", err)
	}
	if prefs.UserID != user.ID {
		t.Errorf("user_id: got %d, want %d", prefs.UserID, user.ID)
	}
	if prefs.Enabled {
		t.Error("expected enabled=false by default")
	}
	if prefs.NotifyDay != 0 {
		t.Errorf("notify_day: got %d, want 0 (Sunday)", prefs.NotifyDay)
	}
	if prefs.NotifyTime != "17:00" {
		t.Errorf("notify_time: got %q, want %q", prefs.NotifyTime, "17:00")
	}
	if prefs.NextNotifyAt != nil {
		t.Errorf("next_notify_at: expected nil, got %v", prefs.NextNotifyAt)
	}
}

func TestGetOrCreateNotificationPrefs_IdempotentSecondCall(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	first, err := s.GetOrCreateNotificationPrefs(ctx, user.ID)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	second, err := s.GetOrCreateNotificationPrefs(ctx, user.ID)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if first.ID != second.ID {
		t.Errorf("IDs differ: %d vs %d", first.ID, second.ID)
	}
}

func TestUpsertNotificationPrefs_UpdatesFields(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	// Create defaults first
	if _, err := s.GetOrCreateNotificationPrefs(ctx, user.ID); err != nil {
		t.Fatalf("create defaults: %v", err)
	}

	next := time.Date(2025, 6, 15, 17, 0, 0, 0, time.UTC)
	p := model.NotificationPrefs{
		UserID:       user.ID,
		Enabled:      true,
		NotifyDay:    1,
		NotifyTime:   "09:00",
		NextNotifyAt: &next,
	}
	if err := s.UpsertNotificationPrefs(ctx, p); err != nil {
		t.Fatalf("UpsertNotificationPrefs: %v", err)
	}

	got, err := s.GetOrCreateNotificationPrefs(ctx, user.ID)
	if err != nil {
		t.Fatalf("get after upsert: %v", err)
	}
	if !got.Enabled {
		t.Error("expected enabled=true")
	}
	if got.NotifyDay != 1 {
		t.Errorf("notify_day: got %d, want 1", got.NotifyDay)
	}
	if got.NotifyTime != "09:00" {
		t.Errorf("notify_time: got %q, want %q", got.NotifyTime, "09:00")
	}
	if got.NextNotifyAt == nil {
		t.Fatal("expected next_notify_at to be set")
	}
	if !got.NextNotifyAt.Equal(next) {
		t.Errorf("next_notify_at: got %v, want %v", got.NextNotifyAt, next)
	}
}

func TestSetNextNotifyAt(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	if _, err := s.GetOrCreateNotificationPrefs(ctx, user.ID); err != nil {
		t.Fatalf("create: %v", err)
	}

	next := time.Date(2025, 7, 6, 17, 0, 0, 0, time.UTC)
	if err := s.SetNextNotifyAt(ctx, user.ID, next); err != nil {
		t.Fatalf("SetNextNotifyAt: %v", err)
	}

	got, err := s.GetOrCreateNotificationPrefs(ctx, user.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.NextNotifyAt == nil {
		t.Fatal("expected next_notify_at to be set")
	}
	if !got.NextNotifyAt.Equal(next) {
		t.Errorf("next_notify_at: got %v, want %v", got.NextNotifyAt, next)
	}
}

func TestGetDueNotificationPrefs_ReturnsDue(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := seedUser(t, s, "alice", "Alice Smith")
	bob := seedUser(t, s, "bob", "Bob Brown")

	now := time.Now().UTC()
	past := now.Add(-time.Minute)
	future := now.Add(time.Hour)

	// Alice: enabled, next_notify_at in the past → should be returned
	aliceNext := past
	if _, err := s.GetOrCreateNotificationPrefs(ctx, alice.ID); err != nil {
		t.Fatalf("create alice prefs: %v", err)
	}
	if err := s.UpsertNotificationPrefs(ctx, model.NotificationPrefs{
		UserID: alice.ID, Enabled: true, NotifyDay: 0, NotifyTime: "17:00", NextNotifyAt: &aliceNext,
	}); err != nil {
		t.Fatalf("upsert alice: %v", err)
	}

	// Bob: enabled, next_notify_at in the future → should NOT be returned
	bobNext := future
	if _, err := s.GetOrCreateNotificationPrefs(ctx, bob.ID); err != nil {
		t.Fatalf("create bob prefs: %v", err)
	}
	if err := s.UpsertNotificationPrefs(ctx, model.NotificationPrefs{
		UserID: bob.ID, Enabled: true, NotifyDay: 0, NotifyTime: "17:00", NextNotifyAt: &bobNext,
	}); err != nil {
		t.Fatalf("upsert bob: %v", err)
	}

	due, err := s.GetDueNotificationPrefs(ctx, now)
	if err != nil {
		t.Fatalf("GetDueNotificationPrefs: %v", err)
	}
	if len(due) != 1 {
		t.Fatalf("expected 1 due pref, got %d", len(due))
	}
	if due[0].UserID != alice.ID {
		t.Errorf("expected alice (id=%d), got user_id=%d", alice.ID, due[0].UserID)
	}
}

func TestGetDueNotificationPrefs_IgnoresDisabled(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	past := time.Now().UTC().Add(-time.Minute)
	if _, err := s.GetOrCreateNotificationPrefs(ctx, user.ID); err != nil {
		t.Fatalf("create: %v", err)
	}
	// enabled=false even though next_notify_at is in the past
	if err := s.UpsertNotificationPrefs(ctx, model.NotificationPrefs{
		UserID: user.ID, Enabled: false, NotifyDay: 0, NotifyTime: "17:00", NextNotifyAt: &past,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	due, err := s.GetDueNotificationPrefs(ctx, time.Now().UTC())
	if err != nil {
		t.Fatalf("GetDueNotificationPrefs: %v", err)
	}
	if len(due) != 0 {
		t.Errorf("expected 0 due prefs, got %d", len(due))
	}
}

// ---- push subscriptions ----

func TestUpsertPushSubscription_Create(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	sub := model.PushSubscription{
		UserID:    user.ID,
		Endpoint:  "https://push.example.com/sub/abc123",
		P256dhKey: "key123",
		AuthKey:   "auth456",
	}
	if err := s.UpsertPushSubscription(ctx, sub); err != nil {
		t.Fatalf("UpsertPushSubscription: %v", err)
	}

	subs, err := s.GetPushSubscriptionsByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetPushSubscriptionsByUserID: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription, got %d", len(subs))
	}
	if subs[0].Endpoint != sub.Endpoint {
		t.Errorf("endpoint: got %q, want %q", subs[0].Endpoint, sub.Endpoint)
	}
	if subs[0].P256dhKey != sub.P256dhKey {
		t.Errorf("p256dh_key: got %q, want %q", subs[0].P256dhKey, sub.P256dhKey)
	}
}

func TestUpsertPushSubscription_UpdatesKeys(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	endpoint := "https://push.example.com/sub/abc123"
	if err := s.UpsertPushSubscription(ctx, model.PushSubscription{
		UserID: user.ID, Endpoint: endpoint, P256dhKey: "old-key", AuthKey: "old-auth",
	}); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := s.UpsertPushSubscription(ctx, model.PushSubscription{
		UserID: user.ID, Endpoint: endpoint, P256dhKey: "new-key", AuthKey: "new-auth",
	}); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	subs, err := s.GetPushSubscriptionsByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("expected 1 subscription after update, got %d", len(subs))
	}
	if subs[0].P256dhKey != "new-key" {
		t.Errorf("p256dh_key: got %q, want %q", subs[0].P256dhKey, "new-key")
	}
}

func TestDeletePushSubscription(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	endpoint := "https://push.example.com/sub/abc123"
	if err := s.UpsertPushSubscription(ctx, model.PushSubscription{
		UserID: user.ID, Endpoint: endpoint, P256dhKey: "key", AuthKey: "auth",
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := s.DeletePushSubscription(ctx, endpoint); err != nil {
		t.Fatalf("DeletePushSubscription: %v", err)
	}

	subs, err := s.GetPushSubscriptionsByUserID(ctx, user.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected 0 subscriptions after delete, got %d", len(subs))
	}
}

func TestGetPushSubscriptionsByUserID_IsolatedByUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := seedUser(t, s, "alice", "Alice Smith")
	bob := seedUser(t, s, "bob", "Bob Brown")

	if err := s.UpsertPushSubscription(ctx, model.PushSubscription{
		UserID: alice.ID, Endpoint: "https://push.example.com/alice", P256dhKey: "k", AuthKey: "a",
	}); err != nil {
		t.Fatalf("upsert alice sub: %v", err)
	}

	subs, err := s.GetPushSubscriptionsByUserID(ctx, bob.ID)
	if err != nil {
		t.Fatalf("get bob subs: %v", err)
	}
	if len(subs) != 0 {
		t.Errorf("expected 0 subs for bob, got %d", len(subs))
	}
}

// ---- CountWeekEntries ----

func TestCountWeekEntries_Empty(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	weekStart := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	count, err := s.CountWeekEntries(ctx, user.ID, weekStart)
	if err != nil {
		t.Fatalf("CountWeekEntries: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}

func TestCountWeekEntries_PartialWeek(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	weekStart := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	entries := []model.WorkDayEntry{
		{UserID: user.ID, EntryDate: weekStart, DayType: model.DayTypeWFH, Hours: 7.5},
		{UserID: user.ID, EntryDate: weekStart.AddDate(0, 0, 1), DayType: model.DayTypeWFH, Hours: 7.5},
		{UserID: user.ID, EntryDate: weekStart.AddDate(0, 0, 2), DayType: model.DayTypeOffice, Hours: 7.5},
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	count, err := s.CountWeekEntries(ctx, user.ID, weekStart)
	if err != nil {
		t.Fatalf("CountWeekEntries: %v", err)
	}
	if count != 3 {
		t.Errorf("count: got %d, want 3", count)
	}
}

func TestCountWeekEntries_FullWeek(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	weekStart := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	var entries []model.WorkDayEntry
	for i := 0; i < 7; i++ {
		entries = append(entries, model.WorkDayEntry{
			UserID:    user.ID,
			EntryDate: weekStart.AddDate(0, 0, i),
			DayType:   model.DayTypeWFH,
			Hours:     7.5,
		})
	}
	if err := s.UpsertEntries(ctx, entries); err != nil {
		t.Fatalf("UpsertEntries: %v", err)
	}

	count, err := s.CountWeekEntries(ctx, user.ID, weekStart)
	if err != nil {
		t.Fatalf("CountWeekEntries: %v", err)
	}
	if count != 7 {
		t.Errorf("count: got %d, want 7", count)
	}
}

func TestCountWeekEntries_DoesNotCountOtherUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := seedUser(t, s, "alice", "Alice Smith")
	bob := seedUser(t, s, "bob", "Bob Brown")

	weekStart := time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC)
	// Add entries for Bob only
	if err := s.UpsertEntries(ctx, []model.WorkDayEntry{
		{UserID: bob.ID, EntryDate: weekStart, DayType: model.DayTypeWFH, Hours: 7.5},
	}); err != nil {
		t.Fatalf("UpsertEntries bob: %v", err)
	}

	count, err := s.CountWeekEntries(ctx, alice.ID, weekStart)
	if err != nil {
		t.Fatalf("CountWeekEntries alice: %v", err)
	}
	if count != 0 {
		t.Errorf("count: got %d, want 0", count)
	}
}
