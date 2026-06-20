// Package store provides SQLite-backed persistence for the StatShed server.
package store

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// connectionPragmas are applied to every connection via the DSN. journal_mode=WAL is a
// database-level setting (set once, persists); busy_timeout/synchronous/foreign_keys are
// per-connection, so applying them on every connection is what makes the pool correct
// (behavioral-map §5 + the foreign_keys=ON / NORMAL hardening, spec.md §6).
var connectionPragmas = []string{
	"busy_timeout(5000)",
	"journal_mode(WAL)",
	"synchronous(NORMAL)",
	"foreign_keys(ON)",
}

// Store holds a serialized write handle and a concurrent read handle over one SQLite DB.
type Store struct {
	read  *sql.DB
	write *sql.DB
}

func dsn(path string) string {
	var b strings.Builder
	b.WriteString("file:")
	b.WriteString(path)
	sep := "?"
	for _, p := range connectionPragmas {
		b.WriteString(sep)
		b.WriteString("_pragma=")
		b.WriteString(p)
		sep = "&"
	}
	return b.String()
}

// Open opens the SQLite database at path and verifies connectivity. Writes are serialized
// onto a single connection (D7 — one writer, the SQLite requirement, without the
// gunicorn-single-worker constraint); reads use a WAL-concurrent pool. The schema is NOT
// created here — the caller runs Migrate (fresh-DB-only, C1).
func Open(path string) (*Store, error) {
	return openWithDriver("sqlite", path)
}

// openWithDriver is Open parameterized by the SQL driver name, so tests can substitute a
// statement-counting driver wrapping modernc.
func openWithDriver(driverName, path string) (*Store, error) {
	d := dsn(path)

	write, err := sql.Open(driverName, d)
	if err != nil {
		return nil, fmt.Errorf("open write handle: %w", err)
	}
	write.SetMaxOpenConns(1)
	if err := write.Ping(); err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("ping write handle: %w", err)
	}

	read, err := sql.Open(driverName, d)
	if err != nil {
		_ = write.Close()
		return nil, fmt.Errorf("open read handle: %w", err)
	}
	if err := read.Ping(); err != nil {
		_ = write.Close()
		_ = read.Close()
		return nil, fmt.Errorf("ping read handle: %w", err)
	}

	return &Store{read: read, write: write}, nil
}

// Write returns the serialized write handle (SetMaxOpenConns(1)).
func (s *Store) Write() *sql.DB { return s.write }

// Read returns the WAL-concurrent read handle.
func (s *Store) Read() *sql.DB { return s.read }

// Close closes both handles, returning the first error.
func (s *Store) Close() error {
	werr := s.write.Close()
	rerr := s.read.Close()
	if werr != nil {
		return werr
	}
	return rerr
}
