package db

import (
	"ato-wfh-diary/internal/model"
	"context"
	"database/sql"
	"fmt"
	"time"
)

// UpsertUser inserts a new user or, if the username already exists, updates
// their display name. Returns the current user record.
func (s *Store) UpsertUser(ctx context.Context, username, displayName string) (*model.User, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO users (username, display_name)
		VALUES (?, ?)
		ON CONFLICT(username) DO UPDATE SET
			display_name = excluded.display_name,
			updated_at   = CURRENT_TIMESTAMP
	`, username, displayName)
	if err != nil {
		return nil, fmt.Errorf("upsert user: %w", err)
	}
	return s.GetUserByUsername(ctx, username)
}

// GetUsers returns all users ordered by display name.
func (s *Store) GetUsers(ctx context.Context) ([]model.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, username, display_name, created_at, updated_at
		FROM users
		ORDER BY display_name
	`)
	if err != nil {
		return nil, fmt.Errorf("get users: %w", err)
	}
	defer rows.Close()

	var users []model.User
	for rows.Next() {
		u, err := scanUser(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetUserByUsername returns the user with the given username, or nil if not found.
func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, created_at, updated_at
		FROM users
		WHERE username = ?
	`, username)
	u, err := scanUser(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by username: %w", err)
	}
	return &u, nil
}

// GetUserByID returns the user with the given ID, or nil if not found.
func (s *Store) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, username, display_name, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id)
	u, err := scanUser(row.Scan)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

// scanUser reads a User from a single row using the provided scan function.
func scanUser(scan scanFn) (model.User, error) {
	var u model.User
	var createdAt, updatedAt string
	if err := scan(&u.ID, &u.Username, &u.DisplayName, &createdAt, &updatedAt); err != nil {
		return model.User{}, err
	}
	var err error
	if u.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return model.User{}, fmt.Errorf("parse created_at: %w", err)
	}
	if u.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return model.User{}, fmt.Errorf("parse updated_at: %w", err)
	}
	return u, nil
}

// parseTimestamp handles the formats that modernc.org/sqlite returns for
// TIMESTAMP columns: RFC3339 ("2006-01-02T15:04:05Z") and the classic SQLite
// text format ("2006-01-02 15:04:05").
func parseTimestamp(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse timestamp %q", s)
}

// parseDate handles the formats modernc.org/sqlite returns for DATE columns:
// RFC3339 with a zero time component ("2006-01-02T00:00:00Z") or plain date
// ("2006-01-02"). Returns a UTC midnight time.Time.
func parseDate(s string) (time.Time, error) {
	for _, layout := range []string{"2006-01-02", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}
