package store

import (
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
)

//go:embed schema/*.sql
var schemaFS embed.FS

// Migrate brings the database up to the latest schema using the embedded goose
// migrations. On a fresh DB it creates the full schema; on a DB that already holds an
// incompatible/foreign schema the plain CREATE statements error and Migrate returns that
// error so the caller fails fast (C1) -- it never mutates an existing DB in place.
func Migrate(db *sql.DB) error {
	goose.SetBaseFS(schemaFS)
	defer goose.SetBaseFS(nil)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	if err := goose.Up(db, "schema"); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
