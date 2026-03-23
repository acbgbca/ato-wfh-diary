package db

import (
	"ato-wfh-diary/internal/model"
	"context"
	"database/sql"
	"fmt"
)

// GetProfile returns the profile for the given user, or nil if none exists.
func (s *Store) GetProfile(ctx context.Context, userID int64) (*model.UserProfile, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, default_hours,
		       mon_type, tue_type, wed_type, thu_type, fri_type, sat_type, sun_type,
		       created_at, updated_at
		FROM   user_profiles
		WHERE  user_id = ?
	`, userID)

	p, err := scanProfile(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}
	return &p, nil
}

// UpsertProfile creates or updates the profile for the user identified by
// p.UserID.
func (s *Store) UpsertProfile(ctx context.Context, p model.UserProfile) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO user_profiles
		    (user_id, default_hours, mon_type, tue_type, wed_type, thu_type, fri_type, sat_type, sun_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		    default_hours = excluded.default_hours,
		    mon_type      = excluded.mon_type,
		    tue_type      = excluded.tue_type,
		    wed_type      = excluded.wed_type,
		    thu_type      = excluded.thu_type,
		    fri_type      = excluded.fri_type,
		    sat_type      = excluded.sat_type,
		    sun_type      = excluded.sun_type,
		    updated_at    = CURRENT_TIMESTAMP
	`,
		p.UserID, p.DefaultHours,
		string(p.MonType), string(p.TueType), string(p.WedType),
		string(p.ThuType), string(p.FriType),
		string(p.SatType), string(p.SunType),
	)
	if err != nil {
		return fmt.Errorf("upsert profile: %w", err)
	}
	return nil
}

func scanProfile(scan scanFn) (model.UserProfile, error) {
	var p model.UserProfile
	var monType, tueType, wedType, thuType, friType, satType, sunType string
	var createdAt, updatedAt string

	if err := scan(
		&p.ID, &p.UserID, &p.DefaultHours,
		&monType, &tueType, &wedType, &thuType, &friType, &satType, &sunType,
		&createdAt, &updatedAt,
	); err != nil {
		return model.UserProfile{}, err
	}

	p.MonType = model.DayType(monType)
	p.TueType = model.DayType(tueType)
	p.WedType = model.DayType(wedType)
	p.ThuType = model.DayType(thuType)
	p.FriType = model.DayType(friType)
	p.SatType = model.DayType(satType)
	p.SunType = model.DayType(sunType)

	var err error
	if p.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return model.UserProfile{}, fmt.Errorf("parse created_at: %w", err)
	}
	if p.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return model.UserProfile{}, fmt.Errorf("parse updated_at: %w", err)
	}
	return p, nil
}
