package middleware

import (
	"context"
	"net/http"
)

type contextKey string

const usernameKey contextKey = "username"

// ForwardAuth extracts the authenticated username from the forwarded request
// header set by the auth proxy (e.g. X-Forwarded-User) and stores it in the
// request context. Requests without the header are rejected with 401.
func ForwardAuth(headerName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			username := r.Header.Get(headerName)
			if username == "" {
				http.Error(w, "unauthorised", http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), usernameKey, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UsernameFromContext retrieves the authenticated username from the context.
func UsernameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(usernameKey).(string)
	return v
}
