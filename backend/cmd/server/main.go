package main

import (
	"ato-wfh-diary/frontend"
	"ato-wfh-diary/internal/api/handlers"
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/internal/service"
	"ato-wfh-diary/migrations"
	"context"
	"log"
	"net/http"
	"os"
	"time"
	_ "time/tzdata" // embed IANA timezone database so LoadLocation works in minimal containers

	webpush "github.com/SherClockHolmes/webpush-go"
)

// version is set at build time via -ldflags="-X main.version=<tag>".
// It defaults to "dev" for local builds without a Git tag.
var version = "dev"

// buildHash is set at build time via -ldflags="-X main.buildHash=<git-short-sha>".
// It is injected into index.html asset URLs for cache-busting.
var buildHash = "dev"

func main() {
	dbPath := envOr("DB_PATH", "./data/wfh.db")
	authHeader := envOr("FORWARD_AUTH_HEADER", "X-Forwarded-User")
	addr := envOr("ADDR", ":8080")
	devUser := os.Getenv("DEV_USER")

	notifyTimezone := envOr("NOTIFICATION_TIMEZONE", "Australia/Melbourne")
	notifyTitle := envOr("NOTIFICATION_TITLE", "WFH Diary")
	notifyBody := envOr("NOTIFICATION_BODY", "Time to log your hours for this week")
	notifySchedulerInterval := envOr("NOTIFICATION_SCHEDULER_INTERVAL", "10m")
	vapidSubject := envOr("VAPID_SUBJECT", "mailto:admin@example.com")

	schedulerInterval, err := time.ParseDuration(notifySchedulerInterval)
	if err != nil {
		log.Fatalf("invalid NOTIFICATION_SCHEDULER_INTERVAL %q: %v", notifySchedulerInterval, err)
	}

	database, err := db.Open(dbPath, migrations.FS)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer database.Close()

	store := db.NewStore(database)

	// Load or auto-generate VAPID keys on first run.
	ctx := context.Background()
	generatedPrivate, generatedPublic, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		log.Fatalf("generate vapid keys: %v", err)
	}
	vapidPrivate, err := store.GetOrSetAppConfig(ctx, "vapid_private_key", generatedPrivate)
	if err != nil {
		log.Fatalf("load vapid private key: %v", err)
	}
	vapidPublic, err := store.GetOrSetAppConfig(ctx, "vapid_public_key", generatedPublic)
	if err != nil {
		log.Fatalf("load vapid public key: %v", err)
	}

	handler := handlers.NewWithConfig(store, vapidPublic, notifyTimezone)
	router := handlers.NewRouter(handler, authHeader, frontend.FS, buildHash)

	if devUser != "" {
		log.Printf("WARNING: DEV_USER=%q — all requests authenticated as that user (development mode only)", devUser)
		router = injectUser(authHeader, devUser, router)
	}

	// Start the push notification scheduler.
	notifService := service.NewNotificationService(store, service.NotificationConfig{
		VAPIDPublicKey:  vapidPublic,
		VAPIDPrivateKey: vapidPrivate,
		VAPIDSubject:    vapidSubject,
		Timezone:        notifyTimezone,
		Title:           notifyTitle,
		Body:            notifyBody,
		Interval:        schedulerInterval,
	})
	go notifService.Run(context.Background())

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
