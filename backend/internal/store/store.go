// Package store keeps the data of the tool in one SQLite file.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // The pure Go SQLite driver. The binary builds with no cgo.
)

// Store is the SQLite database of the tool.
type Store struct {
	db *sql.DB
}

// Open opens the SQLite file at path and applies the migrations.
// Open makes the file and its folder if they do not exist.
func Open(ctx context.Context, path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("make database folder: %w", err)
	}

	name, err := dsn(path)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", name)
	if err != nil {
		return nil, fmt.Errorf("open database %s: %w", path, err)
	}
	// SQLite permits one writer at a time. One connection prevents "database is locked" errors.
	db.SetMaxOpenConns(1)

	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database %s: %w", path, err)
	}
	return &Store{db: db}, nil
}

// dsn returns the SQLite URI for the file at path.
// The driver gives a "file:" URI to SQLite as it is, and SQLite decodes the escaped path.
// Thus url.URL escapes the characters that have a meaning in a URI, for example '?', '#' and '%'.
func dsn(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make database path absolute: %w", err)
	}
	q := url.Values{}
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	u := url.URL{Scheme: "file", Path: filepath.ToSlash(abs), RawQuery: q.Encode()}
	return u.String(), nil
}

// Close closes the database.
func (s *Store) Close() error {
	return s.db.Close()
}

// SchemaVersion returns the number of the last applied migration.
// It also shows that the database answers.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	return schemaVersion(ctx, s.db)
}
