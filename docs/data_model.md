# Data Model

## Overview

The application tracks daily work-from-home hours for two users (a family) to support Australian Tax Office (ATO) WFH expense claims. The Australian financial year runs from **1 July to 30 June**.

Data is stored in a SQLite database accessed by a Go backend.

---

## Entities

### `users`

Represents a person who can log and view WFH entries. Authentication is handled externally via **Forward Auth** — the application trusts the username passed in the forwarded request header and looks up or creates the corresponding user record.

| Column         | Type      | Constraints              | Description                              |
|----------------|-----------|--------------------------|------------------------------------------|
| `id`           | INTEGER   | PK, AUTOINCREMENT        | Internal identifier                      |
| `username`     | TEXT      | NOT NULL, UNIQUE         | Username from the Forward Auth header    |
| `display_name` | TEXT      | NOT NULL                 | Human-readable name shown in the UI      |
| `created_at`   | TIMESTAMP | NOT NULL, DEFAULT NOW    | Record creation time                     |
| `updated_at`   | TIMESTAMP | NOT NULL, DEFAULT NOW    | Last update time                         |

### `work_day_entries`

One row per user per calendar date. Captures the type of day and the number of hours worked from home. Users enter data a week at a time via the UI, but the data is stored at the daily granularity.

| Column           | Type         | Constraints                    | Description                                                    |
|------------------|--------------|--------------------------------|----------------------------------------------------------------|
| `id`             | INTEGER      | PK, AUTOINCREMENT              | Internal identifier                                            |
| `user_id`        | INTEGER      | NOT NULL, FK → `users.id`      | The user this entry belongs to                                 |
| `entry_date`     | DATE         | NOT NULL                       | The calendar date (YYYY-MM-DD)                                 |
| `financial_year` | INTEGER      | NOT NULL, GENERATED STORED     | FY ending year (e.g. 2025 for 1 Jul 2024–30 Jun 2025)         |
| `day_type`       | TEXT         | NOT NULL, see enum below       | Classification of the day                                      |
| `hours`          | DECIMAL(4,2) | NOT NULL, DEFAULT 0.00         | Hours worked from home (0.00–24.00)                            |
| `notes`          | TEXT         | NULL                           | Optional free-text note                                        |
| `created_at`     | TIMESTAMP    | NOT NULL, DEFAULT NOW          | Record creation time                                           |
| `updated_at`     | TIMESTAMP    | NOT NULL, DEFAULT NOW          | Last update time                                               |

**Unique constraint:** `(user_id, entry_date)` — one entry per person per day.

`financial_year` is a computed column derived from `entry_date`: months July–December belong to the FY ending the following calendar year; months January–June belong to the FY ending that calendar year. For example, 15 August 2024 → FY 2025; 3 March 2025 → FY 2025.

#### `day_type` enum

| Value            | Description                                                      | Expected `hours` |
|------------------|------------------------------------------------------------------|------------------|
| `wfh`            | Full day worked from home                                        | > 0              |
| `part_wfh`       | Part of the day worked from home                                 | > 0              |
| `office`         | Full day worked from the office                                  | 0                |
| `annual_leave`   | Annual (holiday) leave                                           | 0                |
| `sick_leave`     | Sick leave                                                       | 0                |
| `public_holiday` | Public holiday                                                   | 0                |
| `weekend`        | Weekend day (Saturday or Sunday)                                 | 0                |

> Only `wfh` and `part_wfh` entries contribute to the ATO WFH claim total.

---

### `user_profiles`

Optional per-user settings that provide default values when pre-populating an empty week in the diary. A user who has never saved their profile simply gets the hard-coded defaults (office for Mon–Fri, weekend for Sat–Sun).

| Column         | Type         | Constraints                                   | Description                                      |
|----------------|--------------|-----------------------------------------------|--------------------------------------------------|
| `id`           | INTEGER      | PK, AUTOINCREMENT                             | Internal identifier                              |
| `user_id`      | INTEGER      | NOT NULL, UNIQUE, FK → `users.id`             | One profile per user                             |
| `default_hours`| DECIMAL(4,2) | NOT NULL, CHECK > 0 AND ≤ 24                  | Hours pre-filled for `wfh` days                 |
| `mon_type`     | TEXT         | NOT NULL, `day_type` enum                     | Default day type for Monday                      |
| `tue_type`     | TEXT         | NOT NULL, `day_type` enum                     | Default day type for Tuesday                     |
| `wed_type`     | TEXT         | NOT NULL, `day_type` enum                     | Default day type for Wednesday                   |
| `thu_type`     | TEXT         | NOT NULL, `day_type` enum                     | Default day type for Thursday                    |
| `fri_type`     | TEXT         | NOT NULL, `day_type` enum                     | Default day type for Friday                      |
| `sat_type`     | TEXT         | NOT NULL, `day_type` enum                     | Default day type for Saturday                    |
| `sun_type`     | TEXT         | NOT NULL, `day_type` enum                     | Default day type for Sunday                      |
| `created_at`   | TIMESTAMP    | NOT NULL, DEFAULT NOW                         | Record creation time                             |
| `updated_at`   | TIMESTAMP    | NOT NULL, DEFAULT NOW                         | Last update time                                 |

---

### `app_config`

Application-level key/value store for persistent configuration that is generated or derived at runtime. Used to store the auto-generated VAPID key pair so it survives server restarts.

| Column       | Type      | Constraints           | Description                            |
|--------------|-----------|-----------------------|----------------------------------------|
| `key`        | TEXT      | PK                    | Config key                             |
| `value`      | TEXT      | NOT NULL              | Config value                           |
| `created_at` | DATETIME  | NOT NULL, DEFAULT NOW | Record creation time                   |
| `updated_at` | DATETIME  | NOT NULL, DEFAULT NOW | Last update time                       |

---

### `user_notification_prefs`

Stores each user's push notification schedule. A row is created on-demand with defaults when the user first visits the Settings page.

| Column           | Type      | Constraints                         | Description                                                                    |
|------------------|-----------|-------------------------------------|--------------------------------------------------------------------------------|
| `id`             | INTEGER   | PK, AUTOINCREMENT                   | Internal identifier                                                            |
| `user_id`        | INTEGER   | NOT NULL, UNIQUE, FK → `users.id`   | One preference row per user                                                    |
| `enabled`        | INTEGER   | NOT NULL, DEFAULT 0                 | Boolean: 1 = notifications enabled                                             |
| `notify_day`     | INTEGER   | NOT NULL, DEFAULT 0                 | Day of week to send: 0 = Sunday, 1 = Monday                                   |
| `notify_time`    | TEXT      | NOT NULL, DEFAULT `17:00`           | Time to send in app timezone (HH:MM)                                           |
| `next_notify_at` | DATETIME  | NULL                                | Next scheduled send time; scheduler sends when this ≤ now; NULL = not scheduled|
| `created_at`     | DATETIME  | NOT NULL, DEFAULT NOW               | Record creation time                                                           |
| `updated_at`     | DATETIME  | NOT NULL, DEFAULT NOW               | Last update time                                                               |

---

### `push_subscriptions`

Holds Web Push API subscription objects, one per user/device. A user who has installed the PWA on multiple devices may have multiple rows.

| Column       | Type      | Constraints                       | Description                                          |
|--------------|-----------|-----------------------------------|------------------------------------------------------|
| `id`         | INTEGER   | PK, AUTOINCREMENT                 | Internal identifier                                  |
| `user_id`    | INTEGER   | NOT NULL, FK → `users.id`         | The user this subscription belongs to                |
| `endpoint`   | TEXT      | NOT NULL, UNIQUE                  | Push service endpoint URL                            |
| `p256dh_key` | TEXT      | NOT NULL                          | ECDH public key for payload encryption               |
| `auth_key`   | TEXT      | NOT NULL                          | Auth secret for payload encryption                   |
| `created_at` | DATETIME  | NOT NULL, DEFAULT NOW             | Record creation time                                 |
| `updated_at` | DATETIME  | NOT NULL, DEFAULT NOW             | Last update time                                     |

---

## SQL Schema

```sql
CREATE TABLE users (
    id           INTEGER   PRIMARY KEY AUTOINCREMENT,
    username     TEXT      NOT NULL UNIQUE,
    display_name TEXT      NOT NULL,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE work_day_entries (
    id             INTEGER       PRIMARY KEY AUTOINCREMENT,
    user_id        INTEGER       NOT NULL REFERENCES users(id),
    entry_date     DATE          NOT NULL,
    financial_year INTEGER       NOT NULL GENERATED ALWAYS AS (
                                     CASE WHEN CAST(strftime('%m', entry_date) AS INTEGER) >= 7
                                          THEN CAST(strftime('%Y', entry_date) AS INTEGER) + 1
                                          ELSE CAST(strftime('%Y', entry_date) AS INTEGER)
                                     END
                                 ) STORED,
    day_type       TEXT          NOT NULL
                                 CHECK(day_type IN ('wfh', 'part_wfh', 'office', 'annual_leave', 'sick_leave', 'public_holiday', 'weekend')),
    hours          DECIMAL(4,2)  NOT NULL DEFAULT 0.00
                                 CHECK(hours >= 0.00 AND hours <= 24.00),
    notes          TEXT,
    created_at     TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     TIMESTAMP     NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, entry_date)
);

-- Supports weekly entry UI: fetch a user's entries for a given date range
CREATE INDEX idx_wde_user_date ON work_day_entries(user_id, entry_date);

-- Supports report generation: fetch all entries for a user in a given financial year
CREATE INDEX idx_wde_user_fy ON work_day_entries(user_id, financial_year);

CREATE TABLE user_profiles (
    id            INTEGER      PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER      NOT NULL UNIQUE REFERENCES users(id),
    default_hours DECIMAL(4,2) NOT NULL
                               CHECK(default_hours > 0.00 AND default_hours <= 24.00),
    mon_type      TEXT         NOT NULL
                               CHECK(mon_type IN ('wfh', 'part_wfh', 'office', 'annual_leave', 'sick_leave', 'public_holiday', 'weekend')),
    tue_type      TEXT         NOT NULL
                               CHECK(tue_type IN ('wfh', 'part_wfh', 'office', 'annual_leave', 'sick_leave', 'public_holiday', 'weekend')),
    wed_type      TEXT         NOT NULL
                               CHECK(wed_type IN ('wfh', 'part_wfh', 'office', 'annual_leave', 'sick_leave', 'public_holiday', 'weekend')),
    thu_type      TEXT         NOT NULL
                               CHECK(thu_type IN ('wfh', 'part_wfh', 'office', 'annual_leave', 'sick_leave', 'public_holiday', 'weekend')),
    fri_type      TEXT         NOT NULL
                               CHECK(fri_type IN ('wfh', 'part_wfh', 'office', 'annual_leave', 'sick_leave', 'public_holiday', 'weekend')),
    sat_type      TEXT         NOT NULL
                               CHECK(sat_type IN ('wfh', 'part_wfh', 'office', 'annual_leave', 'sick_leave', 'public_holiday', 'weekend')),
    sun_type      TEXT         NOT NULL
                               CHECK(sun_type IN ('wfh', 'part_wfh', 'office', 'annual_leave', 'sick_leave', 'public_holiday', 'weekend')),
    created_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at    TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE app_config (
    key        TEXT     PRIMARY KEY,
    value      TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE user_notification_prefs (
    id             INTEGER  PRIMARY KEY,
    user_id        INTEGER  NOT NULL UNIQUE REFERENCES users(id),
    enabled        INTEGER  NOT NULL DEFAULT 0,
    notify_day     INTEGER  NOT NULL DEFAULT 0,
    notify_time    TEXT     NOT NULL DEFAULT '17:00',
    next_notify_at DATETIME,
    created_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE push_subscriptions (
    id         INTEGER  PRIMARY KEY,
    user_id    INTEGER  NOT NULL REFERENCES users(id),
    endpoint   TEXT     NOT NULL UNIQUE,
    p256dh_key TEXT     NOT NULL,
    auth_key   TEXT     NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

---

## Financial Year Reporting

To query all WFH hours for a user in a given financial year (e.g. FY2025 = 1 Jul 2024 – 30 Jun 2025):

```sql
SELECT
    entry_date,
    day_type,
    hours
FROM work_day_entries
WHERE user_id = :user_id
  AND financial_year = :financial_year   -- e.g. 2025
  AND day_type IN ('wfh', 'part_wfh')
ORDER BY entry_date;
```

The report will include:
- **Summary:** total WFH hours for the financial year
- **Detail:** each WFH day, its type, and hours worked

Reports are exportable (format TBD — CSV and/or PDF).

---

## Notes & Future Considerations

- **Forward Auth header:** The Go service will read the authenticated username from the forwarded request header (e.g. `X-Forwarded-User` or `X-Remote-User`, depending on the auth proxy in use). The first time a username is seen, a `users` row is created automatically.
- **Shared access:** Both users can view and edit each other's `work_day_entries`. No row-level ownership restriction is enforced at the database layer — access control is at the application layer.
- **Week start day:** Currently assumed to be Monday. The UI groups entry by week, but data is stored per day. A user preference for configurable week start day may be added in a future iteration.
- **Decimal hours:** Stored as `DECIMAL(4,2)` supporting values like `7.50`. The UI should validate input to two decimal places.
