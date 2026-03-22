package db_test

import (
	"ato-wfh-diary/internal/db"
	"ato-wfh-diary/migrations"
	"testing"
)

// newTestStore opens a fresh in-memory SQLite database, runs all migrations,
// and registers a cleanup to close it when the test finishes.
func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	database, err := db.Open(":memory:", migrations.FS)
	if err != nil {
		t.Fatalf("newTestStore: open: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Errorf("newTestStore: close: %v", err)
		}
	})
	return db.NewStore(database)
}
