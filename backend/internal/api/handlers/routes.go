package handlers

import (
	"ato-wfh-diary/internal/api/middleware"
	"io/fs"
	"net/http"
)

// NewRouter builds the application HTTP router.
//
// All /api routes are protected by Forward Auth using the given header name.
// Static frontend files are served from frontendFS; pass nil to skip (API-only mode).
func NewRouter(h *Handler, authHeader string, frontendFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	auth := middleware.ForwardAuth(authHeader)

	mux.Handle("GET /api/users", auth(http.HandlerFunc(h.GetUsers)))
	mux.Handle("GET /api/me", auth(http.HandlerFunc(h.GetMe)))

	mux.Handle("GET /api/users/{id}/entries", auth(http.HandlerFunc(h.GetWeekEntries)))
	mux.Handle("POST /api/users/{id}/entries", auth(http.HandlerFunc(h.UpsertWeekEntries)))

	mux.Handle("GET /api/users/{id}/report", auth(http.HandlerFunc(h.GetReport)))
	mux.Handle("GET /api/users/{id}/report/export", auth(http.HandlerFunc(h.ExportReport)))

	if frontendFS != nil {
		mux.Handle("/", http.FileServerFS(frontendFS))
	}

	return mux
}
