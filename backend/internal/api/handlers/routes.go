package handlers

import (
	"ato-wfh-diary/internal/api/middleware"
	"net/http"
)

// NewRouter builds the application HTTP router.
//
// All /api routes are protected by Forward Auth using the given header name.
// Static frontend files are served from the provided fs (pass nil to skip).
func NewRouter(h *Handler, authHeader string) http.Handler {
	mux := http.NewServeMux()
	auth := middleware.ForwardAuth(authHeader)

	mux.Handle("GET /api/users", auth(http.HandlerFunc(h.GetUsers)))
	mux.Handle("GET /api/me", auth(http.HandlerFunc(h.GetMe)))

	mux.Handle("GET /api/users/{id}/entries", auth(http.HandlerFunc(h.GetWeekEntries)))
	mux.Handle("POST /api/users/{id}/entries", auth(http.HandlerFunc(h.UpsertWeekEntries)))

	mux.Handle("GET /api/users/{id}/report", auth(http.HandlerFunc(h.GetReport)))
	mux.Handle("GET /api/users/{id}/report/export", auth(http.HandlerFunc(h.ExportReport)))

	return mux
}
