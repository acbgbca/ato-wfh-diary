package db

import (
	"ato-wfh-diary/internal/model"
	"context"
	"fmt"
	"time"
)

// UpsertEntries inserts or updates a batch of work day entries within a single
// transaction. On conflict (same user + date) the existing row is updated.
func (s *Store) UpsertEntries(ctx context.Context, entries []model.WorkDayEntry) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO work_day_entries (user_id, entry_date, day_type, hours, notes)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(user_id, entry_date) DO UPDATE SET
			day_type   = excluded.day_type,
			hours      = excluded.hours,
			notes      = excluded.notes,
			updated_at = CURRENT_TIMESTAMP
	`)
	if err != nil {
		return fmt.Errorf("prepare upsert: %w", err)
	}
	defer stmt.Close()

	for _, e := range entries {
		var notes *string
		if e.Notes != "" {
			notes = &e.Notes
		}
		if _, err := stmt.ExecContext(ctx,
			e.UserID,
			e.EntryDate.Format("2006-01-02"),
			string(e.DayType),
			e.Hours,
			notes,
		); err != nil {
			return fmt.Errorf("upsert entry %s: %w", e.EntryDate.Format("2006-01-02"), err)
		}
	}

	return tx.Commit()
}

// GetWeekEntries returns all entries for a user within the 7-day window
// starting on weekStart (inclusive). Days with no entry are not included —
// the caller is responsible for filling gaps if needed.
func (s *Store) GetWeekEntries(ctx context.Context, userID int64, weekStart time.Time) ([]model.WorkDayEntry, error) {
	weekEnd := weekStart.AddDate(0, 0, 6)
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, entry_date, financial_year, day_type, hours,
		       COALESCE(notes, ''), created_at, updated_at
		FROM   work_day_entries
		WHERE  user_id    = ?
		  AND  entry_date >= ?
		  AND  entry_date <= ?
		ORDER  BY entry_date
	`,
		userID,
		weekStart.Format("2006-01-02"),
		weekEnd.Format("2006-01-02"),
	)
	if err != nil {
		return nil, fmt.Errorf("get week entries: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// GetFYWFHEntries returns all wfh and part_wfh entries for a user in the given
// financial year, ordered by date. Used for report generation.
func (s *Store) GetFYWFHEntries(ctx context.Context, userID int64, financialYear int) ([]model.WorkDayEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, entry_date, financial_year, day_type, hours,
		       COALESCE(notes, ''), created_at, updated_at
		FROM   work_day_entries
		WHERE  user_id        = ?
		  AND  financial_year = ?
		  AND  day_type       IN ('wfh', 'part_wfh')
		ORDER  BY entry_date
	`, userID, financialYear)
	if err != nil {
		return nil, fmt.Errorf("get fy wfh entries: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// GetFYAllEntries returns every entry for a user in the given financial year,
// regardless of day type. Useful for displaying a full yearly overview.
func (s *Store) GetFYAllEntries(ctx context.Context, userID int64, financialYear int) ([]model.WorkDayEntry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, entry_date, financial_year, day_type, hours,
		       COALESCE(notes, ''), created_at, updated_at
		FROM   work_day_entries
		WHERE  user_id        = ?
		  AND  financial_year = ?
		ORDER  BY entry_date
	`, userID, financialYear)
	if err != nil {
		return nil, fmt.Errorf("get fy all entries: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows)
}

// GetFirstIncompleteWeek returns the Monday of the first week, starting from
// fromDate, that does not have entries for every day that falls within the
// given financial year. It only checks weeks up to and including the week
// that contains today. Returns nil if all such weeks are complete.
//
// For the first week of a financial year the week may straddle the FY
// boundary (e.g. the week of Mon 30 Jun when FY starts on Tue 1 Jul).
// In that case only the days on or after 1 July are required.
func (s *Store) GetFirstIncompleteWeek(ctx context.Context, userID int64, financialYear int, fromDate time.Time, today time.Time) (*time.Time, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT date(entry_date) FROM work_day_entries
		WHERE  user_id = ? AND financial_year = ?
		ORDER  BY entry_date
	`, userID, financialYear)
	if err != nil {
		return nil, fmt.Errorf("get fy entry dates: %w", err)
	}
	defer rows.Close()

	entryDates := make(map[string]bool)
	for rows.Next() {
		var dateStr string
		if err := rows.Scan(&dateStr); err != nil {
			return nil, fmt.Errorf("scan entry date: %w", err)
		}
		entryDates[dateStr] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// fyStart is the first day of this financial year (e.g. 1 Jul 2025 for FY2026).
	fyStart := time.Date(financialYear-1, time.July, 1, 0, 0, 0, 0, time.UTC)
	todayMonday := mondayOf(today)

	week := fromDate
	for !week.After(todayMonday) {
		weekEnd := week.AddDate(0, 0, 6)

		// For the first (possibly partial) week, only count days within the FY.
		firstDay := week
		if firstDay.Before(fyStart) {
			firstDay = fyStart
		}
		required := int(weekEnd.Sub(firstDay).Hours()/24) + 1

		count := 0
		for i := 0; i < 7; i++ {
			d := week.AddDate(0, 0, i)
			if !d.Before(firstDay) && entryDates[d.Format("2006-01-02")] {
				count++
			}
		}
		if count < required {
			result := week
			return &result, nil
		}
		week = week.AddDate(0, 0, 7)
	}

	return nil, nil
}

// mondayOf returns the Monday at midnight UTC of the week containing d.
func mondayOf(d time.Time) time.Time {
	weekday := int(d.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	offset := 1 - weekday
	return time.Date(d.Year(), d.Month(), d.Day()+offset, 0, 0, 0, 0, d.Location())
}

// scanEntries drains a *sql.Rows cursor into a slice of WorkDayEntry.
func scanEntries(rows interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
}) ([]model.WorkDayEntry, error) {
	var entries []model.WorkDayEntry
	for rows.Next() {
		e, err := scanEntry(rows.Scan)
		if err != nil {
			return nil, fmt.Errorf("scan entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// scanEntry reads a single WorkDayEntry using the provided scan function.
func scanEntry(scan scanFn) (model.WorkDayEntry, error) {
	var e model.WorkDayEntry
	var entryDateStr, createdAt, updatedAt string
	var dayType string

	if err := scan(
		&e.ID, &e.UserID, &entryDateStr, &e.FinancialYear,
		&dayType, &e.Hours, &e.Notes, &createdAt, &updatedAt,
	); err != nil {
		return model.WorkDayEntry{}, err
	}

	e.DayType = model.DayType(dayType)

	var err error
	if e.EntryDate, err = parseDate(entryDateStr); err != nil {
		return model.WorkDayEntry{}, fmt.Errorf("parse entry_date: %w", err)
	}
	if e.CreatedAt, err = parseTimestamp(createdAt); err != nil {
		return model.WorkDayEntry{}, fmt.Errorf("parse created_at: %w", err)
	}
	if e.UpdatedAt, err = parseTimestamp(updatedAt); err != nil {
		return model.WorkDayEntry{}, fmt.Errorf("parse updated_at: %w", err)
	}

	return e, nil
}
