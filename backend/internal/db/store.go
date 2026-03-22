package db

import "database/sql"

// Store provides all database operations for the application.
type Store struct {
	db *sql.DB
}

// NewStore wraps an open *sql.DB in a Store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// scanFn is the common signature for (*sql.Row).Scan and (*sql.Rows).Scan,
// used to share scan logic between single-row and multi-row queries.
type scanFn func(dest ...any) error
