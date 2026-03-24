package db

import (
	"ato-wfh-diary/internal/model"
	"context"
	"database/sql"
	"fmt"
	"time"
)

// GetOrSetAppConfig returns the value for key, inserting defaultValue if the
// key does not yet exist. Subsequent calls return the stored value unchanged.
func (s *Store) GetOrSetAppConfig(ctx context.Context, key, defaultValue string) (string, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO app_config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO NOTHING
	`, key, defaultValue); err != nil {
		return "", fmt.Errorf("init app config %q: %w", key, err)
	}
	var value string
	if err := s.db.QueryRowContext(ctx, `SELECT value FROM app_config WHERE key = ?`, key).Scan(&value); err != nil {
		return "", fmt.Errorf("read app config %q: %w", key, err)
	}
	return value, nil
}

// GetOrCreateNotificationPrefs returns the notification prefs for userID,
// creating a default row if none exists.
func (s *Store) GetOrCreateNotificationPrefs(ctx context.Context, userID int64) (*model.NotificationPrefs, error) {
	if _, err := s.db.ExecContext(ctx, `
		INSERT INTO user_notification_prefs (user_id) VALUES (?)
		ON CONFLICT(user_id) DO NOTHING
	`, userID); err != nil {
		return nil, fmt.Errorf("init notification prefs: %w", err)
	}
	return s.scanNotificationPrefs(ctx, userID)
}

// UpsertNotificationPrefs creates or updates the notification prefs for the
// user identified by p.UserID.
func (s *Store) UpsertNotificationPrefs(ctx context.Context, p model.NotificationPrefs) error {
	var nextNotifyAt *string
	if p.NextNotifyAt != nil {
		s := p.NextNotifyAt.UTC().Format(time.RFC3339)
		nextNotifyAt = &s
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_notification_prefs
		    (user_id, enabled, notify_day, notify_time, next_notify_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		    enabled        = excluded.enabled,
		    notify_day     = excluded.notify_day,
		    notify_time    = excluded.notify_time,
		    next_notify_at = excluded.next_notify_at,
		    updated_at     = CURRENT_TIMESTAMP
	`, p.UserID, p.Enabled, p.NotifyDay, p.NotifyTime, nextNotifyAt)
	if err != nil {
		return fmt.Errorf("upsert notification prefs: %w", err)
	}
	return nil
}

// SetNextNotifyAt updates only the next_notify_at column for the given user.
func (s *Store) SetNextNotifyAt(ctx context.Context, userID int64, next time.Time) error {
	nextStr := next.UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
		UPDATE user_notification_prefs
		SET    next_notify_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE  user_id = ?
	`, nextStr, userID)
	if err != nil {
		return fmt.Errorf("set next_notify_at: %w", err)
	}
	return nil
}

// GetDueNotificationPrefs returns all enabled notification prefs whose
// next_notify_at is at or before now.
func (s *Store) GetDueNotificationPrefs(ctx context.Context, now time.Time) ([]model.NotificationPrefs, error) {
	nowStr := now.UTC().Format(time.RFC3339)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, enabled, notify_day, notify_time, next_notify_at,
		       created_at, updated_at
		FROM   user_notification_prefs
		WHERE  enabled = 1
		  AND  next_notify_at IS NOT NULL
		  AND  next_notify_at <= ?
	`, nowStr)
	if err != nil {
		return nil, fmt.Errorf("get due notification prefs: %w", err)
	}
	defer rows.Close()

	var result []model.NotificationPrefs
	for rows.Next() {
		p, err := scanNotifPrefsRow(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan notification prefs: %w", err)
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// UpsertPushSubscription saves a push subscription, updating keys if the
// endpoint already exists.
func (s *Store) UpsertPushSubscription(ctx context.Context, sub model.PushSubscription) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO push_subscriptions (user_id, endpoint, p256dh_key, auth_key)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
		    p256dh_key = excluded.p256dh_key,
		    auth_key   = excluded.auth_key,
		    updated_at = CURRENT_TIMESTAMP
	`, sub.UserID, sub.Endpoint, sub.P256dhKey, sub.AuthKey)
	if err != nil {
		return fmt.Errorf("upsert push subscription: %w", err)
	}
	return nil
}

// DeletePushSubscription removes the subscription with the given endpoint.
func (s *Store) DeletePushSubscription(ctx context.Context, endpoint string) error {
	if _, err := s.db.ExecContext(ctx,
		`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint,
	); err != nil {
		return fmt.Errorf("delete push subscription: %w", err)
	}
	return nil
}

// GetPushSubscriptionsByUserID returns all push subscriptions for a user.
func (s *Store) GetPushSubscriptionsByUserID(ctx context.Context, userID int64) ([]model.PushSubscription, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, endpoint, p256dh_key, auth_key, created_at, updated_at
		FROM   push_subscriptions
		WHERE  user_id = ?
		ORDER  BY id
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("get push subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []model.PushSubscription
	for rows.Next() {
		var sub model.PushSubscription
		var createdAt, updatedAt string
		if err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256dhKey, &sub.AuthKey,
			&createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan push subscription: %w", err)
		}
		var parseErr error
		if sub.CreatedAt, parseErr = parseTimestamp(createdAt); parseErr != nil {
			return nil, fmt.Errorf("parse created_at: %w", parseErr)
		}
		if sub.UpdatedAt, parseErr = parseTimestamp(updatedAt); parseErr != nil {
			return nil, fmt.Errorf("parse updated_at: %w", parseErr)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// CountWeekEntries returns the number of work day entries for a user within
// the 7-day window starting on weekStart (inclusive).
func (s *Store) CountWeekEntries(ctx context.Context, userID int64, weekStart time.Time) (int, error) {
	weekEnd := weekStart.AddDate(0, 0, 6)
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM   work_day_entries
		WHERE  user_id    = ?
		  AND  entry_date >= ?
		  AND  entry_date <= ?
	`, userID,
		weekStart.Format("2006-01-02"),
		weekEnd.Format("2006-01-02"),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count week entries: %w", err)
	}
	return count, nil
}

// scanNotificationPrefs loads the notification prefs row for a given userID.
func (s *Store) scanNotificationPrefs(ctx context.Context, userID int64) (*model.NotificationPrefs, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, enabled, notify_day, notify_time, next_notify_at,
		       created_at, updated_at
		FROM   user_notification_prefs
		WHERE  user_id = ?
	`, userID)
	p, err := scanNotifPrefsRow(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan notification prefs: %w", err)
	}
	return &p, nil
}

func scanNotifPrefsRow(scan scanFn) (model.NotificationPrefs, error) {
	var p model.NotificationPrefs
	var nextNotifyAt *string
	var createdAt, updatedAt string
	var enabled int

	if err := scan(
		&p.ID, &p.UserID, &enabled, &p.NotifyDay, &p.NotifyTime,
		&nextNotifyAt, &createdAt, &updatedAt,
	); err != nil {
		return model.NotificationPrefs{}, err
	}
	p.Enabled = enabled != 0

	if nextNotifyAt != nil {
		t, err := parseTimestamp(*nextNotifyAt)
		if err != nil {
			return model.NotificationPrefs{}, fmt.Errorf("parse next_notify_at: %w", err)
		}
		p.NextNotifyAt = &t
	}

	var err error
	if p.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return model.NotificationPrefs{}, fmt.Errorf("parse created_at: %w", err)
	}
	if p.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return model.NotificationPrefs{}, fmt.Errorf("parse updated_at: %w", err)
	}
	return p, nil
}
