package handlers

import "ato-wfh-diary/internal/db"

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	Store           *db.Store
	VAPIDPublicKey  string // public key served to browsers for push subscription
	VAPIDPrivateKey string // private key used to sign Web Push requests
	VAPIDSubject    string // mailto: or https: contact URI sent with Web Push requests
	NotifyTimezone  string // IANA timezone name used to schedule notifications
	AuthHeader      string // forward auth header name, used to extract username in no-auth endpoints
}

// New creates a Handler with the given Store.
func New(store *db.Store) *Handler {
	return &Handler{Store: store}
}

// NewWithConfig creates a Handler with the given Store and notification config.
func NewWithConfig(store *db.Store, vapidPublicKey, vapidPrivateKey, vapidSubject, notifyTimezone, authHeader string) *Handler {
	return &Handler{
		Store:           store,
		VAPIDPublicKey:  vapidPublicKey,
		VAPIDPrivateKey: vapidPrivateKey,
		VAPIDSubject:    vapidSubject,
		NotifyTimezone:  notifyTimezone,
		AuthHeader:      authHeader,
	}
}
