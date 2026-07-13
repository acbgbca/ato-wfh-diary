package handlers_test

import (
	"ato-wfh-diary/internal/api/handlers"
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/migrations"
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// newPushTestServer starts a test server configured with a real VAPID key pair
// so that push sends are actually signed and dispatched to the subscription
// endpoint. It also returns the store so tests can assert on stored
// subscriptions.
func newPushTestServer(t *testing.T) (*httptest.Server, *db.Store) {
	t.Helper()
	database, err := db.Open(":memory:", migrations.FS)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	store := db.NewStore(database)

	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("generate vapid keys: %v", err)
	}
	h := handlers.NewWithConfig(store, publicKey, privateKey, "test@example.com", "Australia/Melbourne", testAuthHeader)
	srv := httptest.NewServer(handlers.NewRouter(h, testAuthHeader, nil, ""))
	t.Cleanup(func() {
		srv.Close()
		database.Close()
	})
	return srv, store
}

// fakePushService starts an HTTP server that stands in for a browser push
// service (e.g. web.push.apple.com), always replying with the given status.
// It returns the server and a counter of requests received.
func fakePushService(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// subscribeKeys returns a valid p256dh/auth key pair for a push subscription.
func subscribeKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate ecdh key: %v", err)
	}
	authSecret := make([]byte, 16)
	if _, err := rand.Read(authSecret); err != nil {
		t.Fatalf("generate auth secret: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()),
		base64.RawURLEncoding.EncodeToString(authSecret)
}

func mustSubscribe(t *testing.T, srv *httptest.Server, username, endpoint string) {
	t.Helper()
	p256dh, auth := subscribeKeys(t)
	resp := do(t, srv, http.MethodPost, "/api/notifications/subscribe", username, map[string]any{
		"endpoint": endpoint, "p256dh_key": p256dh, "auth_key": auth,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("subscribe: got status %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func subscriptionCount(t *testing.T, store *db.Store, username string) int {
	t.Helper()
	user, err := store.GetUserByUsername(context.Background(), username)
	if err != nil || user == nil {
		t.Fatalf("get user %q: %v", username, err)
	}
	subs, err := store.GetPushSubscriptionsByUserID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("get subscriptions: %v", err)
	}
	return len(subs)
}

// A subscription that the push service reports as gone (410 Unregistered — what
// happens after the PWA is deleted and reinstalled) must be pruned so it stops
// poisoning every later send.
func TestPostTestNotification_PrunesGoneSubscription(t *testing.T) {
	srv, store := newPushTestServer(t)
	mustCreateUser(t, srv, "alice")

	push, _ := fakePushService(t, http.StatusGone, `{"reason":"Unregistered"}`)
	mustSubscribe(t, srv, "alice", push.URL+"/stale")

	resp := do(t, srv, http.MethodPost, "/api/notifications/test", "alice", nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if n := subscriptionCount(t, store, "alice"); n != 0 {
		t.Errorf("expected the gone subscription to be deleted, still have %d", n)
	}
}

// One dead subscription must not stop delivery to the user's other, live
// devices — the reinstalled PWA registers a new endpoint while the old one
// lingers in the database.
func TestPostTestNotification_DeadSubscriptionDoesNotBlockLiveOne(t *testing.T) {
	srv, store := newPushTestServer(t)
	mustCreateUser(t, srv, "alice")

	dead, _ := fakePushService(t, http.StatusGone, `{"reason":"Unregistered"}`)
	live, liveHits := fakePushService(t, http.StatusCreated, "")

	// The dead subscription was registered first, so it is sent to first.
	mustSubscribe(t, srv, "alice", dead.URL+"/old-install")
	mustSubscribe(t, srv, "alice", live.URL+"/new-install")

	resp := do(t, srv, http.MethodPost, "/api/notifications/test", "alice", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if got := liveHits.Load(); got != 1 {
		t.Errorf("live push service: got %d requests, want 1", got)
	}
	if n := subscriptionCount(t, store, "alice"); n != 1 {
		t.Errorf("expected only the live subscription to remain, have %d", n)
	}
}
