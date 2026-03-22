package handlers_test

import (
	"net/http"
	"testing"
)

func TestGetUsers_Empty(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet, "/api/users", "alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var users []map[string]any
	decodeJSON(t, resp, &users)
	if len(users) != 0 {
		t.Errorf("expected empty list, got %d users", len(users))
	}
}

func TestGetUsers_ReturnsBothUsers(t *testing.T) {
	srv := newTestServer(t)

	// Create both users via /api/me.
	mustCreateUser(t, srv, "alice")
	mustCreateUser(t, srv, "bob")

	resp := do(t, srv, http.MethodGet, "/api/users", "alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var users []map[string]any
	decodeJSON(t, resp, &users)
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}
}

func TestGetUsers_Unauthorised(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet, "/api/users", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestGetMe_CreatesUserOnFirstLogin(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet, "/api/me", "alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var user struct {
		ID          int64  `json:"id"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	decodeJSON(t, resp, &user)

	if user.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if user.Username != "alice" {
		t.Errorf("username: got %q, want %q", user.Username, "alice")
	}
	// Display name defaults to username on auto-creation.
	if user.DisplayName != "alice" {
		t.Errorf("display_name: got %q, want %q", user.DisplayName, "alice")
	}
}

func TestGetMe_ReturnsSameUserOnSubsequentCalls(t *testing.T) {
	srv := newTestServer(t)

	resp1 := do(t, srv, http.MethodGet, "/api/me", "alice", nil)
	var user1 struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, resp1, &user1)

	resp2 := do(t, srv, http.MethodGet, "/api/me", "alice", nil)
	var user2 struct {
		ID int64 `json:"id"`
	}
	decodeJSON(t, resp2, &user2)

	if user1.ID != user2.ID {
		t.Errorf("ID changed between calls: %d → %d", user1.ID, user2.ID)
	}
}

func TestGetMe_Unauthorised(t *testing.T) {
	srv := newTestServer(t)

	resp := do(t, srv, http.MethodGet, "/api/me", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
