// Package migrations embeds all SQL migration files so the binary is
// self-contained and tests can run migrations without filesystem path tricks.
package migrations

import "embed"

// FS contains all *.sql migration files in lexicographic order.
//
//go:embed *.sql
var FS embed.FS
