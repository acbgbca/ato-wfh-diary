package handlers

import (
	"ato-wfh-diary/internal/api/middleware"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

// NewRouter builds the application HTTP router.
//
// All /api routes are protected by Forward Auth using the given header name.
// Static frontend files are served from frontendFS; pass nil to skip (API-only mode).
// buildHash is injected into index.html as {{.BuildHash}} for cache-busting asset URLs.
func NewRouter(h *Handler, authHeader string, frontendFS fs.FS, buildHash string) http.Handler {
	mux := http.NewServeMux()
	auth := middleware.ForwardAuth(authHeader)

	mux.Handle("GET /api/users", auth(http.HandlerFunc(h.GetUsers)))
	mux.Handle("GET /api/me", auth(http.HandlerFunc(h.GetMe)))
	mux.Handle("GET /api/me/profile", auth(http.HandlerFunc(h.GetProfile)))
	mux.Handle("PUT /api/me/profile", auth(http.HandlerFunc(h.UpsertProfile)))

	mux.Handle("GET /api/users/{id}/entries", auth(http.HandlerFunc(h.GetWeekEntries)))
	mux.Handle("POST /api/users/{id}/entries", auth(http.HandlerFunc(h.UpsertWeekEntries)))

	mux.Handle("GET /api/users/{id}/report", auth(http.HandlerFunc(h.GetReport)))
	mux.Handle("GET /api/users/{id}/report/export", auth(http.HandlerFunc(h.ExportReport)))

	mux.Handle("GET /api/notifications/vapid-key", auth(http.HandlerFunc(h.GetVapidKey)))
	mux.Handle("GET /api/notifications/prefs", auth(http.HandlerFunc(h.GetNotificationPrefs)))
	mux.Handle("PUT /api/notifications/prefs", auth(http.HandlerFunc(h.PutNotificationPrefs)))
	mux.Handle("POST /api/notifications/subscribe", auth(http.HandlerFunc(h.PostSubscribe)))
	mux.Handle("DELETE /api/notifications/subscribe", auth(http.HandlerFunc(h.DeleteSubscribe)))

	if frontendFS != nil {
		mux.Handle("/", newStaticHandler(frontendFS, buildHash))
	}

	return mux
}

// newStaticHandler returns an http.Handler that serves embedded frontend assets with
// appropriate cache headers:
//   - index.html is rendered as a Go template with BuildHash substituted, served with Cache-Control: no-cache
//   - JS and CSS assets are served with Cache-Control: max-age=31536000, immutable
func newStaticHandler(frontendFS fs.FS, buildHash string) http.Handler {
	fileServer := http.FileServerFS(frontendFS)

	indexBytes, err := fs.ReadFile(frontendFS, "index.html")
	if err != nil {
		panic("static handler: cannot read index.html: " + err.Error())
	}
	tmpl := template.Must(template.New("index").Parse(string(indexBytes)))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/" || path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			tmpl.Execute(w, map[string]string{"BuildHash": buildHash}) //nolint:errcheck
			return
		}

		if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
			w.Header().Set("Cache-Control", "max-age=31536000, immutable")
		}

		fileServer.ServeHTTP(w, r)
	})
}
