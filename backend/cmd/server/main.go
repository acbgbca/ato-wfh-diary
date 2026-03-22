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

func main() {
	dbPath := envOr("DB_PATH", "./data/wfh.db")
	authHeader := envOr("FORWARD_AUTH_HEADER", "X-Forwarded-User")
	addr := envOr("ADDR", ":8080")

	database, err := db.Open(dbPath, migrations.FS)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	store := db.NewStore(database)
	handler := handlers.New(store)
	router := handlers.NewRouter(handler, authHeader, frontend.FS)

	log.Printf("ATO WFH Diary listening on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
