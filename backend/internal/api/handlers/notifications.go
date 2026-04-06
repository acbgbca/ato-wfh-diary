package handlers

import (
	"ato-wfh-diary/internal/api/middleware"
	"ato-wfh-diary/internal/model"
	"ato-wfh-diary/internal/service"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// GetVapidKey returns the application's VAPID public key so the browser can
// create a Web Push subscription.
func (h *Handler) GetVapidKey(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, map[string]string{"vapid_public_key": h.VAPIDPublicKey})
}

// GetNotificationPrefs returns the current user's notification preferences,
// creating default prefs if none exist yet.
func (h *Handler) GetNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	username := middleware.UsernameFromContext(r.Context())
	user, err := h.Store.GetUserByUsername(r.Context(), username)
	if err != nil || user == nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}

	prefs, err := h.Store.GetOrCreateNotificationPrefs(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve notification prefs")
		return
	}
	respondJSON(w, prefs)
}

// notificationPrefsRequest is the JSON body for PUT /api/notifications/prefs.
type notificationPrefsRequest struct {
	Enabled    bool   `json:"enabled"`
	NotifyDay  int    `json:"notify_day"`  // 0 = Sunday, 1 = Monday
	NotifyTime string `json:"notify_time"` // HH:MM
}

// PutNotificationPrefs creates or updates the current user's notification prefs.
// When enabled, next_notify_at is computed as the next occurrence of the chosen
// day+time. When disabled, next_notify_at is cleared.
func (h *Handler) PutNotificationPrefs(w http.ResponseWriter, r *http.Request) {
	username := middleware.UsernameFromContext(r.Context())
	user, err := h.Store.GetOrCreateUser(r.Context(), username, username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}

	var req notificationPrefsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.NotifyDay < 0 || req.NotifyDay > 1 {
		respondError(w, http.StatusBadRequest, "notify_day must be 0 (Sunday) or 1 (Monday)")
		return
	}

	if _, err := time.Parse("15:04", req.NotifyTime); err != nil {
		respondError(w, http.StatusBadRequest, "notify_time must be HH:MM")
		return
	}

	prefs := model.NotificationPrefs{
		UserID:     user.ID,
		Enabled:    req.Enabled,
		NotifyDay:  req.NotifyDay,
		NotifyTime: req.NotifyTime,
	}

	if req.Enabled {
		tz := h.NotifyTimezone
		if tz == "" {
			tz = "Australia/Melbourne"
		}
		loc, err := time.LoadLocation(tz)
		if err != nil {
			loc = time.UTC
		}
		next := service.ComputeNextNotifyAt(req.NotifyDay, req.NotifyTime, loc, time.Now())
		prefs.NextNotifyAt = &next
	}

	if err := h.Store.UpsertNotificationPrefs(r.Context(), prefs); err != nil {
		respondError(w, http.StatusInternalServerError, "could not save notification prefs")
		return
	}

	saved, err := h.Store.GetOrCreateNotificationPrefs(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve saved prefs")
		return
	}
	respondJSON(w, saved)
}

// subscribeRequest is the JSON body for POST /api/notifications/subscribe.
type subscribeRequest struct {
	Endpoint  string `json:"endpoint"`
	P256dhKey string `json:"p256dh_key"`
	AuthKey   string `json:"auth_key"`
}

// PostSubscribe saves (or updates) a Web Push subscription for the current user.
func (h *Handler) PostSubscribe(w http.ResponseWriter, r *http.Request) {
	username := middleware.UsernameFromContext(r.Context())
	user, err := h.Store.GetOrCreateUser(r.Context(), username, username)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}

	var req subscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" || req.P256dhKey == "" || req.AuthKey == "" {
		respondError(w, http.StatusBadRequest, "endpoint, p256dh_key, and auth_key are required")
		return
	}

	sub := model.PushSubscription{
		UserID:    user.ID,
		Endpoint:  req.Endpoint,
		P256dhKey: req.P256dhKey,
		AuthKey:   req.AuthKey,
	}
	if err := h.Store.UpsertPushSubscription(r.Context(), sub); err != nil {
		respondError(w, http.StatusInternalServerError, "could not save subscription")
		return
	}
	respondJSON(w, map[string]string{"status": "ok"})
}

// deleteSubscribeRequest is the JSON body for DELETE /api/notifications/subscribe.
type deleteSubscribeRequest struct {
	Endpoint string `json:"endpoint"`
}

// PostTestNotification sends a test push notification to all of the current
// user's registered push subscriptions.
func (h *Handler) PostTestNotification(w http.ResponseWriter, r *http.Request) {
	username := middleware.UsernameFromContext(r.Context())
	user, err := h.Store.GetUserByUsername(r.Context(), username)
	if err != nil || user == nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve user")
		return
	}

	subs, err := h.Store.GetPushSubscriptionsByUserID(r.Context(), user.ID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not retrieve subscriptions")
		return
	}
	if len(subs) == 0 {
		respondError(w, http.StatusBadRequest, "no push subscriptions registered for this user")
		return
	}

	payload, err := json.Marshal(map[string]string{
		"title": "WFH Diary",
		"body":  "Test notification — push notifications are working!",
	})
	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not build notification payload")
		return
	}

	for _, sub := range subs {
		device := endpointHost(sub.Endpoint)
		log.Printf("push notification: sending test notification to user %q, device %q", username, device)
		pushSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dhKey,
				Auth:   sub.AuthKey,
			},
		}
		resp, err := webpush.SendNotification(payload, pushSub, &webpush.Options{
			VAPIDPublicKey:  h.VAPIDPublicKey,
			VAPIDPrivateKey: h.VAPIDPrivateKey,
			Subscriber:      h.VAPIDSubject,
		})
		if err != nil {
			log.Printf("push notification: test to user %q, device %q: send error: %v", username, device, err)
			respondError(w, http.StatusBadGateway, "failed to send notification: "+err.Error())
			return
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			log.Printf("push notification: test to user %q, device %q: push service rejected (status %d): %s", username, device, resp.StatusCode, string(body))
			respondError(w, http.StatusBadGateway, "push service rejected notification")
			return
		}
		log.Printf("push notification: test to user %q, device %q: sent successfully (status %d)", username, device, resp.StatusCode)
	}

	respondJSON(w, map[string]string{"status": "ok"})
}

// endpointHost extracts the host from a push endpoint URL for logging.
// Falls back to the full endpoint string if parsing fails.
func endpointHost(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		return u.Host
	}
	return endpoint
}

// DeleteSubscribe removes a push subscription by endpoint.
func (h *Handler) DeleteSubscribe(w http.ResponseWriter, r *http.Request) {
	var req deleteSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" {
		respondError(w, http.StatusBadRequest, "endpoint is required")
		return
	}

	if err := h.Store.DeletePushSubscription(r.Context(), req.Endpoint); err != nil {
		respondError(w, http.StatusInternalServerError, "could not remove subscription")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
