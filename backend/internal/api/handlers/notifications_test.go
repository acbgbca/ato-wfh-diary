package handlers_test

import (
	"net/http"
	"testing"
)

func TestGetVapidKey_ReturnsKey(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet, "/api/notifications/vapid-key", "alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)
	key, ok := body["vapid_public_key"].(string)
	if !ok || key == "" {
		t.Errorf("expected non-empty vapid_public_key, got %v", body["vapid_public_key"])
	}
}

func TestGetVapidKey_Unauthorized(t *testing.T) {
	srv := newTestServer(t)
	resp := do(t, srv, http.MethodGet, "/api/notifications/vapid-key", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestGetNotificationPrefs_DefaultsForNewUser(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodGet, "/api/notifications/prefs", "alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["enabled"] != false {
		t.Errorf("enabled: got %v, want false", body["enabled"])
	}
	if body["notify_day"] != float64(0) {
		t.Errorf("notify_day: got %v, want 0", body["notify_day"])
	}
	if body["notify_time"] != "17:00" {
		t.Errorf("notify_time: got %v, want 17:00", body["notify_time"])
	}
}

func TestGetNotificationPrefs_Unauthorized(t *testing.T) {
	srv := newTestServer(t)
	resp := do(t, srv, http.MethodGet, "/api/notifications/prefs", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestPutNotificationPrefs_EnableSunday(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	body := map[string]any{
		"enabled":     true,
		"notify_day":  0,
		"notify_time": "17:00",
	}
	resp := do(t, srv, http.MethodPut, "/api/notifications/prefs", "alice", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got map[string]any
	decodeJSON(t, resp, &got)
	if got["enabled"] != true {
		t.Errorf("enabled: got %v, want true", got["enabled"])
	}
	if got["notify_day"] != float64(0) {
		t.Errorf("notify_day: got %v, want 0", got["notify_day"])
	}
	// next_notify_at should be set when enabled
	if got["next_notify_at"] == nil {
		t.Error("expected next_notify_at to be set when enabled")
	}
}

func TestPutNotificationPrefs_EnableMonday(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	body := map[string]any{
		"enabled":     true,
		"notify_day":  1,
		"notify_time": "09:00",
	}
	resp := do(t, srv, http.MethodPut, "/api/notifications/prefs", "alice", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got map[string]any
	decodeJSON(t, resp, &got)
	if got["notify_day"] != float64(1) {
		t.Errorf("notify_day: got %v, want 1", got["notify_day"])
	}
	if got["notify_time"] != "09:00" {
		t.Errorf("notify_time: got %v, want 09:00", got["notify_time"])
	}
}

func TestPutNotificationPrefs_DisableClearsNextNotifyAt(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	// Enable first
	do(t, srv, http.MethodPut, "/api/notifications/prefs", "alice", map[string]any{
		"enabled": true, "notify_day": 0, "notify_time": "17:00",
	})

	// Then disable
	resp := do(t, srv, http.MethodPut, "/api/notifications/prefs", "alice", map[string]any{
		"enabled": false, "notify_day": 0, "notify_time": "17:00",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got map[string]any
	decodeJSON(t, resp, &got)
	if got["enabled"] != false {
		t.Errorf("enabled: got %v, want false", got["enabled"])
	}
	if got["next_notify_at"] != nil {
		t.Errorf("expected next_notify_at to be nil when disabled, got %v", got["next_notify_at"])
	}
}

func TestPutNotificationPrefs_Unauthorized(t *testing.T) {
	srv := newTestServer(t)
	resp := do(t, srv, http.MethodPut, "/api/notifications/prefs", "", map[string]any{
		"enabled": false, "notify_day": 0, "notify_time": "17:00",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestPutNotificationPrefs_InvalidDay(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodPut, "/api/notifications/prefs", "alice", map[string]any{
		"enabled": true, "notify_day": 5, "notify_time": "17:00",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPutNotificationPrefs_InvalidTime(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	resp := do(t, srv, http.MethodPut, "/api/notifications/prefs", "alice", map[string]any{
		"enabled": true, "notify_day": 0, "notify_time": "bad-time",
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestPostSubscribe_Creates(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	body := map[string]any{
		"endpoint":   "https://push.example.com/abc",
		"p256dh_key": "key123",
		"auth_key":   "auth456",
	}
	resp := do(t, srv, http.MethodPost, "/api/notifications/subscribe", "alice", body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestPostSubscribe_Unauthorized(t *testing.T) {
	srv := newTestServer(t)
	body := map[string]any{
		"endpoint": "https://push.example.com/abc", "p256dh_key": "k", "auth_key": "a",
	}
	resp := do(t, srv, http.MethodPost, "/api/notifications/subscribe", "", body)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}

func TestDeleteSubscribe_Removes(t *testing.T) {
	srv := newTestServer(t)
	mustCreateUser(t, srv, "alice")

	// Create then delete
	do(t, srv, http.MethodPost, "/api/notifications/subscribe", "alice", map[string]any{
		"endpoint": "https://push.example.com/abc", "p256dh_key": "k", "auth_key": "a",
	})
	resp := do(t, srv, http.MethodDelete, "/api/notifications/subscribe", "alice", map[string]any{
		"endpoint": "https://push.example.com/abc",
	})
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
}

func TestDeleteSubscribe_Unauthorized(t *testing.T) {
	srv := newTestServer(t)
	resp := do(t, srv, http.MethodDelete, "/api/notifications/subscribe", "", map[string]any{
		"endpoint": "https://push.example.com/abc",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusUnauthorized)
	}
}
