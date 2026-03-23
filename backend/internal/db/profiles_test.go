package db_test

import (
	"ato-wfh-diary/internal/model"
	"context"
	"testing"
)

func TestGetProfile_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	profile, err := s.GetProfile(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if profile != nil {
		t.Errorf("expected nil profile, got %+v", profile)
	}
}

func TestUpsertProfile_Create(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	p := model.UserProfile{
		UserID:       user.ID,
		DefaultHours: 7.5,
		MonType:      model.DayTypeWFH,
		TueType:      model.DayTypeWFH,
		WedType:      model.DayTypeOffice,
		ThuType:      model.DayTypeWFH,
		FriType:      model.DayTypeWFH,
		SatType:      model.DayTypeWeekend,
		SunType:      model.DayTypeWeekend,
	}
	if err := s.UpsertProfile(ctx, p); err != nil {
		t.Fatalf("UpsertProfile: %v", err)
	}

	got, err := s.GetProfile(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got == nil {
		t.Fatal("expected profile, got nil")
	}
	if got.UserID != user.ID {
		t.Errorf("user_id: got %d, want %d", got.UserID, user.ID)
	}
	if got.DefaultHours != 7.5 {
		t.Errorf("default_hours: got %v, want 7.5", got.DefaultHours)
	}
	if got.MonType != model.DayTypeWFH {
		t.Errorf("mon_type: got %q, want wfh", got.MonType)
	}
	if got.WedType != model.DayTypeOffice {
		t.Errorf("wed_type: got %q, want office", got.WedType)
	}
	if got.SatType != model.DayTypeWeekend {
		t.Errorf("sat_type: got %q, want weekend", got.SatType)
	}
}

func TestUpsertProfile_Update(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	user := seedUser(t, s, "alice", "Alice Smith")

	initial := model.UserProfile{
		UserID:       user.ID,
		DefaultHours: 8.0,
		MonType:      model.DayTypeWFH,
		TueType:      model.DayTypeWFH,
		WedType:      model.DayTypeWFH,
		ThuType:      model.DayTypeWFH,
		FriType:      model.DayTypeWFH,
		SatType:      model.DayTypeWeekend,
		SunType:      model.DayTypeWeekend,
	}
	if err := s.UpsertProfile(ctx, initial); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}

	updated := model.UserProfile{
		UserID:       user.ID,
		DefaultHours: 6.0,
		MonType:      model.DayTypeWFH,
		TueType:      model.DayTypeOffice,
		WedType:      model.DayTypeWFH,
		ThuType:      model.DayTypeOffice,
		FriType:      model.DayTypeWFH,
		SatType:      model.DayTypeWeekend,
		SunType:      model.DayTypeWeekend,
	}
	if err := s.UpsertProfile(ctx, updated); err != nil {
		t.Fatalf("update upsert: %v", err)
	}

	got, err := s.GetProfile(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.DefaultHours != 6.0 {
		t.Errorf("default_hours: got %v, want 6.0", got.DefaultHours)
	}
	if got.TueType != model.DayTypeOffice {
		t.Errorf("tue_type: got %q, want office", got.TueType)
	}
	if got.ThuType != model.DayTypeOffice {
		t.Errorf("thu_type: got %q, want office", got.ThuType)
	}
}

func TestGetProfile_IsolatedByUser(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()
	alice := seedUser(t, s, "alice", "Alice Smith")
	bob := seedUser(t, s, "bob", "Bob Brown")

	aliceProfile := model.UserProfile{
		UserID:       alice.ID,
		DefaultHours: 7.5,
		MonType:      model.DayTypeWFH,
		TueType:      model.DayTypeWFH,
		WedType:      model.DayTypeWFH,
		ThuType:      model.DayTypeWFH,
		FriType:      model.DayTypeWFH,
		SatType:      model.DayTypeWeekend,
		SunType:      model.DayTypeWeekend,
	}
	if err := s.UpsertProfile(ctx, aliceProfile); err != nil {
		t.Fatalf("upsert alice profile: %v", err)
	}

	// Bob has no profile.
	bobProfile, err := s.GetProfile(ctx, bob.ID)
	if err != nil {
		t.Fatalf("GetProfile bob: %v", err)
	}
	if bobProfile != nil {
		t.Errorf("expected nil profile for bob, got %+v", bobProfile)
	}

	// Alice's profile is unchanged.
	gotAlice, err := s.GetProfile(ctx, alice.ID)
	if err != nil {
		t.Fatalf("GetProfile alice: %v", err)
	}
	if gotAlice == nil || gotAlice.DefaultHours != 7.5 {
		t.Errorf("alice profile unexpected: %+v", gotAlice)
	}
}
