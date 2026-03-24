package service

import (
	"ato-wfh-diary/internal/db"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

// NotificationConfig holds the configuration for the push notification service.
type NotificationConfig struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
	Timezone        string
	Title           string
	Body            string
	Interval        time.Duration
}

// NotificationService sends scheduled push notifications to users.
type NotificationService struct {
	store  *db.Store
	config NotificationConfig
	loc    *time.Location
}

// NewNotificationService creates a NotificationService.
// If config.Timezone cannot be loaded, UTC is used.
func NewNotificationService(store *db.Store, config NotificationConfig) *NotificationService {
	loc, err := time.LoadLocation(config.Timezone)
	if err != nil {
		log.Printf("notification service: unknown timezone %q, falling back to UTC", config.Timezone)
		loc = time.UTC
	}
	return &NotificationService{store: store, config: config, loc: loc}
}

// Run starts the scheduler loop, blocking until ctx is cancelled.
func (s *NotificationService) Run(ctx context.Context) {
	interval := s.config.Interval
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *NotificationService) tick(ctx context.Context) {
	now := time.Now().UTC()
	due, err := s.store.GetDueNotificationPrefs(ctx, now)
	if err != nil {
		log.Printf("notification scheduler: get due prefs: %v", err)
		return
	}

	for _, prefs := range due {
		if err := s.processUser(ctx, prefs.UserID, prefs.NotifyDay, prefs.NotifyTime, now); err != nil {
			log.Printf("notification scheduler: user %d: %v", prefs.UserID, err)
			// Leave next_notify_at unchanged so we retry on the next tick.
			continue
		}
		// Advance to next week regardless of whether a notification was sent
		// (the week may have been complete).
		next := ComputeNextNotifyAt(prefs.NotifyDay, prefs.NotifyTime, s.loc, now)
		if err := s.store.SetNextNotifyAt(ctx, prefs.UserID, next); err != nil {
			log.Printf("notification scheduler: advance next_notify_at user %d: %v", prefs.UserID, err)
		}
	}
}

// processUser sends a push notification to the user if their target week
// has incomplete entries.
func (s *NotificationService) processUser(ctx context.Context, userID int64, notifyDay int, notifyTime string, now time.Time) error {
	weekStart := targetWeekStart(notifyDay, now, s.loc)

	count, err := s.store.CountWeekEntries(ctx, userID, weekStart)
	if err != nil {
		return fmt.Errorf("count week entries: %w", err)
	}
	if count >= 7 {
		// Week is complete — no notification needed.
		return nil
	}

	subs, err := s.store.GetPushSubscriptionsByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("get subscriptions: %w", err)
	}
	if len(subs) == 0 {
		return nil
	}

	payload, err := json.Marshal(map[string]string{
		"title":      s.config.Title,
		"body":       s.config.Body,
		"week_start": weekStart.Format("2006-01-02"),
	})
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	for _, sub := range subs {
		pushSub := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dhKey,
				Auth:   sub.AuthKey,
			},
		}
		resp, err := webpush.SendNotification(payload, pushSub, &webpush.Options{
			VAPIDPublicKey:  s.config.VAPIDPublicKey,
			VAPIDPrivateKey: s.config.VAPIDPrivateKey,
			Subscriber:      s.config.VAPIDSubject,
		})
		if err != nil {
			return fmt.Errorf("send push to endpoint: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode >= 400 {
			return fmt.Errorf("push rejected (status %d)", resp.StatusCode)
		}
	}
	return nil
}

// ComputeNextNotifyAt returns the next occurrence of (weekday=notifyDay,
// time=notifyTime) in loc that is strictly after from.
// notifyDay: 0 = Sunday, 1 = Monday (matching time.Weekday).
// notifyTime: "HH:MM".
func ComputeNextNotifyAt(notifyDay int, notifyTime string, loc *time.Location, from time.Time) time.Time {
	parts := strings.SplitN(notifyTime, ":", 2)
	hour, _ := strconv.Atoi(parts[0])
	minute := 0
	if len(parts) == 2 {
		minute, _ = strconv.Atoi(parts[1])
	}

	target := time.Weekday(notifyDay)
	fromInLoc := from.In(loc)

	for i := 0; i <= 7; i++ {
		candidate := time.Date(
			fromInLoc.Year(), fromInLoc.Month(), fromInLoc.Day()+i,
			hour, minute, 0, 0, loc,
		)
		if candidate.Weekday() == target && candidate.After(from) {
			return candidate
		}
	}
	// Unreachable: iterating 8 days always finds a match.
	panic("ComputeNextNotifyAt: no match found within 8 days")
}

// targetWeekStart returns the Monday of the week that the notification covers.
// For Sunday notifications (notifyDay=0) → the current week's Monday.
// For Monday notifications (notifyDay=1) → the previous week's Monday.
func targetWeekStart(notifyDay int, now time.Time, loc *time.Location) time.Time {
	nowInLoc := now.In(loc)
	today := time.Date(nowInLoc.Year(), nowInLoc.Month(), nowInLoc.Day(), 0, 0, 0, 0, loc)

	// Days since Monday (Sunday = 6, Monday = 0, ..., Saturday = 5).
	daysSinceMonday := int(today.Weekday()) - 1
	if daysSinceMonday < 0 {
		daysSinceMonday = 6
	}
	currentMonday := today.AddDate(0, 0, -daysSinceMonday)

	if notifyDay == 1 {
		// Monday notification → previous week
		return currentMonday.AddDate(0, 0, -7)
	}
	// Sunday notification → current week
	return currentMonday
}
