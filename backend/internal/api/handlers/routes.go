package handlers

import (
	"ato-wfh-diary/internal/api/middleware"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	texttmpl "text/template"
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
	mux.Handle("POST /api/users", auth(http.HandlerFunc(h.CreateUser)))
	mux.Handle("GET /api/me", auth(http.HandlerFunc(h.GetMe)))
	mux.Handle("GET /api/me/profile", auth(http.HandlerFunc(h.GetProfile)))
	mux.Handle("PUT /api/me/profile", auth(http.HandlerFunc(h.UpsertProfile)))

	mux.Handle("GET /api/users/{id}/profile", auth(http.HandlerFunc(h.GetUserProfile)))
	mux.Handle("PUT /api/users/{id}/profile", auth(http.HandlerFunc(h.UpsertUserProfile)))

	mux.Handle("GET /api/users/{id}/entries", auth(http.HandlerFunc(h.GetWeekEntries)))
	mux.Handle("POST /api/users/{id}/entries", auth(http.HandlerFunc(h.UpsertWeekEntries)))
	mux.Handle("GET /api/users/{id}/entries/first-incomplete-week", auth(http.HandlerFunc(h.GetFirstIncompleteWeek)))
	mux.Handle("GET /api/users/{id}/entries/week-status", auth(http.HandlerFunc(h.GetWeekCompletionStatus)))

	mux.Handle("GET /api/users/{id}/report", auth(http.HandlerFunc(h.GetReport)))
	mux.Handle("GET /api/users/{id}/report/export", auth(http.HandlerFunc(h.ExportReport)))

	mux.Handle("GET /api/notifications/vapid-key", auth(http.HandlerFunc(h.GetVapidKey)))
	mux.Handle("GET /api/notifications/prefs", auth(http.HandlerFunc(h.GetNotificationPrefs)))
	mux.Handle("PUT /api/notifications/prefs", auth(http.HandlerFunc(h.PutNotificationPrefs)))
	mux.Handle("POST /api/notifications/subscribe", auth(http.HandlerFunc(h.PostSubscribe)))
	mux.Handle("DELETE /api/notifications/subscribe", auth(http.HandlerFunc(h.DeleteSubscribe)))
	mux.Handle("POST /api/notifications/test", auth(http.HandlerFunc(h.PostTestNotification)))

	// No auth — must be reachable even when authentication is failing.
	mux.Handle("POST /api/debug/client-error", http.HandlerFunc(h.PostClientError))

	if frontendFS != nil {
		mux.Handle("/", newStaticHandler(frontendFS, buildHash))
	}

	return mux
}

// newStaticHandler returns an http.Handler that serves embedded frontend assets with
// appropriate cache headers:
//   - index.html and sw.js are rendered as Go templates with BuildHash substituted, served with Cache-Control: no-cache
//   - Other JS and CSS assets are served with Cache-Control: max-age=31536000, immutable
func newStaticHandler(frontendFS fs.FS, buildHash string) http.Handler {
	fileServer := http.FileServerFS(frontendFS)

	indexBytes, err := fs.ReadFile(frontendFS, "index.html")
	if err != nil {
		panic("static handler: cannot read index.html: " + err.Error())
	}
	indexTmpl := template.Must(template.New("index").Parse(string(indexBytes)))

	// sw.js is served as a text template (not HTML) so the build hash is injected
	// into the cache name without HTML escaping. Cache-Control: no-cache ensures
	// the browser always fetches the latest version to detect SW updates.
	swBytes, err := fs.ReadFile(frontendFS, "sw.js")
	if err != nil {
		panic("static handler: cannot read sw.js: " + err.Error())
	}
	swTmpl := texttmpl.Must(texttmpl.New("sw").Parse(string(swBytes)))

	tmplData := map[string]string{"BuildHash": buildHash}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "/" || path == "/index.html" {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			indexTmpl.Execute(w, tmplData) //nolint:errcheck
			return
		}

		if path == "/sw.js" {
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			swTmpl.Execute(w, tmplData) //nolint:errcheck
			return
		}

		if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
			w.Header().Set("Cache-Control", "max-age=31536000, immutable")
		}

		fileServer.ServeHTTP(w, r)
	})
}
