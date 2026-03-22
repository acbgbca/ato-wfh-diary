package db

import (
	"database/sql"
	"fmt"
	"io/fs"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at dsn, applies PRAGMAs, and
// runs any pending migrations from fsys (an fs.FS containing *.sql files).
func Open(dsn string, fsys fs.FS) (*sql.DB, error) {
	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	if err := database.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	// SQLite is not a networked database. A connection pool offers no benefit
	// and actively harms in-memory databases, where each connection gets its
	// own independent database instance. One connection for the lifetime of the
	// process is correct for both file and in-memory usage.
	database.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA foreign_keys=ON;",
	}
	for _, p := range pragmas {
		if _, err := database.Exec(p); err != nil {
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	if err := migrate(database, fsys); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return database, nil
}

// migrate runs all *.sql files from fsys in lexicographic order, skipping any
// already recorded in the schema_migrations table.
func migrate(db *sql.DB, fsys fs.FS) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		filename   TEXT      PRIMARY KEY,
		applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		return err
	}

	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read migrations: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE filename = ?)`, name).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}

		sql, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		if _, err := db.Exec(string(sql)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}

		if _, err := db.Exec(`INSERT INTO schema_migrations (filename) VALUES (?)`, name); err != nil {
			return err
		}
	}

	return nil
}
