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
