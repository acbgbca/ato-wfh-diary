package main

import (
	"ato-wfh-diary/frontend"
	"ato-wfh-diary/internal/api/handlers"
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/migrations"
	"log"
	"net/http"
	"os"
)

// version is set at build time via -ldflags="-X main.version=<tag>".
// It defaults to "dev" for local builds without a Git tag.
var version = "dev"

func main() {
	dbPath := envOr("DB_PATH", "./data/wfh.db")
	authHeader := envOr("FORWARD_AUTH_HEADER", "X-Forwarded-User")
	addr := envOr("ADDR", ":8080")
	devUser := os.Getenv("DEV_USER")

	database, err := db.Open(dbPath, migrations.FS)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	store := db.NewStore(database)
	handler := handlers.New(store)
	router := handlers.NewRouter(handler, authHeader, frontend.FS)

	if devUser != "" {
		log.Printf("WARNING: DEV_USER=%q — all requests authenticated as that user (development mode only)", devUser)
		router = injectUser(authHeader, devUser, router)
	}

	log.Printf("ATO WFH Diary %s listening on %s", version, addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}

// injectUser wraps a handler so that every request is pre-authenticated as
// username. Used only when DEV_USER is set (development mode).
func injectUser(header, username string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set(header, username)
		next.ServeHTTP(w, r)
	})
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
