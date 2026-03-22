package db_test

import (
	"context"
	"testing"
)

func TestUpsertUser_Create(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user, err := s.UpsertUser(ctx, "alice", "Alice Smith")
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}

	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if user.Username != "alice" {
		t.Errorf("username: got %q, want %q", user.Username, "alice")
	}
	if user.DisplayName != "Alice Smith" {
		t.Errorf("display_name: got %q, want %q", user.DisplayName, "Alice Smith")
	}
	if user.CreatedAt.IsZero() {
		t.Error("expected non-zero created_at")
	}
}

func TestUpsertUser_UpdateDisplayName(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	first, err := s.UpsertUser(ctx, "alice", "Alice Smith")
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	updated, err := s.UpsertUser(ctx, "alice", "Alice Jones")
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if updated.ID != first.ID {
		t.Errorf("ID changed: got %d, want %d", updated.ID, first.ID)
	}
	if updated.DisplayName != "Alice Jones" {
		t.Errorf("display_name: got %q, want %q", updated.DisplayName, "Alice Jones")
	}
}

func TestGetUsers(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Empty initially.
	users, err := s.GetUsers(ctx)
	if err != nil {
		t.Fatalf("GetUsers (empty): %v", err)
	}
	if len(users) != 0 {
		t.Errorf("expected 0 users, got %d", len(users))
	}

	if _, err := s.UpsertUser(ctx, "bob", "Bob Brown"); err != nil {
		t.Fatalf("upsert bob: %v", err)
	}
	if _, err := s.UpsertUser(ctx, "alice", "Alice Smith"); err != nil {
		t.Fatalf("upsert alice: %v", err)
	}

	users, err = s.GetUsers(ctx)
	if err != nil {
		t.Fatalf("GetUsers: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
	// Should be ordered by display_name: Alice before Bob.
	if users[0].Username != "alice" {
		t.Errorf("first user: got %q, want %q", users[0].Username, "alice")
	}
	if users[1].Username != "bob" {
		t.Errorf("second user: got %q, want %q", users[1].Username, "bob")
	}
}

func TestGetUserByUsername_Found(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	if _, err := s.UpsertUser(ctx, "alice", "Alice Smith"); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	user, err := s.GetUserByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.Username != "alice" {
		t.Errorf("username: got %q, want %q", user.Username, "alice")
	}
}

func TestGetUserByUsername_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user, err := s.GetUserByUsername(ctx, "nobody")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil, got %+v", user)
	}
}

func TestGetUserByID_Found(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	created, err := s.UpsertUser(ctx, "alice", "Alice Smith")
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	user, err := s.GetUserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetUserByID: %v", err)
	}
	if user == nil {
		t.Fatal("expected user, got nil")
	}
	if user.ID != created.ID {
		t.Errorf("ID: got %d, want %d", user.ID, created.ID)
	}
}

func TestGetUserByID_NotFound(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	user, err := s.GetUserByID(ctx, 9999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user != nil {
		t.Errorf("expected nil, got %+v", user)
	}
}
