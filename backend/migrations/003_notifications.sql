-- app_config stores application-level key/value settings (e.g. auto-generated VAPID keys).
CREATE TABLE app_config (
    key        TEXT     PRIMARY KEY,
    value      TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- user_notification_prefs stores each user's push notification schedule.
-- next_notify_at is set when notifications are enabled and advanced by one week
-- after each successful send. NULL means no send is scheduled.
CREATE TABLE user_notification_prefs (
    id             INTEGER  PRIMARY KEY,
    user_id        INTEGER  NOT NULL UNIQUE REFERENCES users(id),
    enabled        INTEGER  NOT NULL DEFAULT 0,
    notify_day     INTEGER  NOT NULL DEFAULT 0,    -- 0 = Sunday, 1 = Monday
    notify_time    TEXT     NOT NULL DEFAULT '17:00', -- HH:MM in app timezone
    next_notify_at DATETIME,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- push_subscriptions holds Web Push API subscription objects for each user/device.
-- A user may have multiple subscriptions (one per installed browser/device).
CREATE TABLE push_subscriptions (
    id         INTEGER  PRIMARY KEY,
    user_id    INTEGER  NOT NULL REFERENCES users(id),
    endpoint   TEXT     NOT NULL UNIQUE,
    p256dh_key TEXT     NOT NULL,
    auth_key   TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
