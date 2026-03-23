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
