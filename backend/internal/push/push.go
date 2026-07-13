// Package push sends Web Push notifications to a user's registered devices and
// keeps the subscription list clean by removing subscriptions that the push
// service reports as gone.
package push

import (
	"ato-wfh-diary/internal/model"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// Config holds the VAPID credentials used to sign Web Push requests.
type Config struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
}

// Store is the subset of the database the sender needs.
type Store interface {
	GetPushSubscriptionsByUserID(ctx context.Context, userID int64) ([]model.PushSubscription, error)
	DeletePushSubscription(ctx context.Context, endpoint string) error
}

// Sender delivers push notifications to all of a user's devices.
type Sender struct {
	store  Store
	config Config
}

// NewSender creates a Sender.
func NewSender(store Store, config Config) *Sender {
	return &Sender{store: store, config: config}
}

// Result summarises what happened when sending to a user's devices.
type Result struct {
	Sent    int // devices the push service accepted
	Removed int // subscriptions deleted because the push service reported them gone
	Failed  int // devices that failed for any other reason
}

// SendToUser delivers payload to every push subscription registered for userID.
//
// A device that fails never prevents delivery to the user's other devices, and
// a subscription the push service reports as gone (404/410 — e.g. the PWA was
// uninstalled, or the browser dropped the subscription) is deleted rather than
// retried forever. who is used only for logging.
//
// The returned error is non-nil only when the send could not be attempted at
// all, or when at least one device failed for a reason that may be transient
// and is worth retrying. Devices that were pruned are not an error.
func (s *Sender) SendToUser(ctx context.Context, userID int64, who string, payload []byte) (Result, error) {
	var result Result

	subs, err := s.store.GetPushSubscriptionsByUserID(ctx, userID)
	if err != nil {
		return result, fmt.Errorf("get subscriptions: %w", err)
	}

	var lastErr error
	for _, sub := range subs {
		device := endpointHost(sub.Endpoint)
		log.Printf("push notification: sending to user %s, device %q", who, device)

		resp, err := webpush.SendNotification(payload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys:     webpush.Keys{P256dh: sub.P256dhKey, Auth: sub.AuthKey},
		}, &webpush.Options{
			VAPIDPublicKey:  s.config.VAPIDPublicKey,
			VAPIDPrivateKey: s.config.VAPIDPrivateKey,
			Subscriber:      normalizeVAPIDSubscriber(s.config.VAPIDSubject),
		})
		if err != nil {
			result.Failed++
			lastErr = fmt.Errorf("send push to device %q: %w", device, err)
			log.Printf("push notification: user %s, device %q: send error: %v", who, device, err)
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		switch {
		case isGone(resp.StatusCode):
			log.Printf("push notification: user %s, device %q: subscription no longer valid (status %d): %s — removing it",
				who, device, resp.StatusCode, strings.TrimSpace(string(body)))
			if err := s.store.DeletePushSubscription(ctx, sub.Endpoint); err != nil {
				log.Printf("push notification: user %s, device %q: could not remove subscription: %v", who, device, err)
			}
			result.Removed++
		case resp.StatusCode >= 400:
			result.Failed++
			lastErr = fmt.Errorf("device %q: push rejected (status %d)", device, resp.StatusCode)
			log.Printf("push notification: user %s, device %q: push service rejected (status %d): %s",
				who, device, resp.StatusCode, strings.TrimSpace(string(body)))
		default:
			result.Sent++
			log.Printf("push notification: user %s, device %q: sent successfully (status %d)", who, device, resp.StatusCode)
		}
	}

	if result.Failed > 0 {
		return result, lastErr
	}
	return result, nil
}

// isGone reports whether the push service is telling us the subscription no
// longer exists and must not be used again. RFC 8030 mandates 404 (endpoint
// unknown) and 410 (subscription expired/unsubscribed) for this; Apple returns
// 410 with reason "Unregistered" once the PWA is uninstalled.
func isGone(status int) bool {
	return status == http.StatusNotFound || status == http.StatusGone
}

// normalizeVAPIDSubscriber strips a leading "mailto:" prefix before passing the
// subscriber to the webpush library. The library prepends "mailto:" to any value
// that does not start with "https:", so passing an already-prefixed value (e.g.
// "mailto:user@example.com") would produce "mailto:mailto:user@example.com" in
// the JWT sub claim, which Apple's push service rejects with BadJwtToken.
func normalizeVAPIDSubscriber(s string) string {
	return strings.TrimPrefix(s, "mailto:")
}

// endpointHost extracts the host from a push endpoint URL for logging.
// Falls back to the full endpoint string if parsing fails.
func endpointHost(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		return u.Host
	}
	return endpoint
}
