package handlers

import "ato-wfh-diary/internal/db"

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	Store *db.Store
}

// New creates a Handler with the given Store.
func New(store *db.Store) *Handler {
	return &Handler{Store: store}
}
