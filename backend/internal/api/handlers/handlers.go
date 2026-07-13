package handlers

import (
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/internal/push"
)

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	Store          *db.Store
	VAPIDPublicKey string // public key served to browsers for push subscription
	NotifyTimezone string // IANA timezone name used to schedule notifications
	AuthHeader     string // forward auth header name, used to extract username in no-auth endpoints
	Push           *push.Sender
}

// New creates a Handler with the given Store.
func New(store *db.Store) *Handler {
	return NewWithConfig(store, "", "", "", "", "")
}

// NewWithConfig creates a Handler with the given Store and notification config.
func NewWithConfig(store *db.Store, vapidPublicKey, vapidPrivateKey, vapidSubject, notifyTimezone, authHeader string) *Handler {
	return &Handler{
		Store:          store,
		VAPIDPublicKey: vapidPublicKey,
		NotifyTimezone: notifyTimezone,
		AuthHeader:     authHeader,
		Push: push.NewSender(store, push.Config{
			VAPIDPublicKey:  vapidPublicKey,
			VAPIDPrivateKey: vapidPrivateKey,
			VAPIDSubject:    vapidSubject,
		}),
	}
}
