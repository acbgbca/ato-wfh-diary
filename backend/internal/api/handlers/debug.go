package handlers

import (
	"encoding/json"
	"log"
	"net/http"
)

// clientErrorReport is the payload sent by the browser's global error handler.
type clientErrorReport struct {
	Message     string `json:"message"`
	Stack       string `json:"stack"`
	URL         string `json:"url"`
	Platform    string `json:"platform"`
	DisplayMode string `json:"displayMode"`
	ScreenWidth  int   `json:"screenWidth"`
	ScreenHeight int   `json:"screenHeight"`
}

// PostClientError logs a client-side error report received from the browser.
// This endpoint intentionally does NOT require authentication so that errors
// occurring before or during the auth flow (e.g. on mobile) can still be captured.
func (h *Handler) PostClientError(w http.ResponseWriter, r *http.Request) {
	var rep clientErrorReport
	if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Attempt to identify the user via the forward-auth header (may be absent if
	// the reverse proxy blocked the request before adding it).
	username := ""
	if h.AuthHeader != "" {
		username = r.Header.Get(h.AuthHeader)
	}
	userAgent := r.Header.Get("User-Agent")

	log.Printf("CLIENT ERROR user=%q ua=%q platform=%q display=%q screen=%dx%d url=%q message=%q stack=%s",
		username, userAgent, rep.Platform, rep.DisplayMode,
		rep.ScreenWidth, rep.ScreenHeight, rep.URL, rep.Message, rep.Stack)

	w.WriteHeader(http.StatusNoContent)
}
