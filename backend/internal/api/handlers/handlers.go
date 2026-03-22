package handlers

import "database/sql"

// Handler holds shared dependencies for all HTTP handlers.
type Handler struct {
	DB *sql.DB
}

// New creates a Handler with the given database connection.
func New(db *sql.DB) *Handler {
	return &Handler{DB: db}
}
