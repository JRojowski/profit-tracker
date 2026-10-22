package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"
)

// ErrDatabaseNewer tells that a newer binary applied migrations that this binary does not know.
var ErrDatabaseNewer = errors.New("database is newer than this binary")

// migrationFiles holds the numbered migrations, for example 0001_accounts.sql.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// migration is one numbered schema step.
type migration struct {
	version int
	name    string
	sql     string
}

// migrate applies each migration with a number above PRAGMA user_version.
// Each migration runs in its own transaction, together with the new user_version.
func migrate(ctx context.Context, db *sql.DB) error {
	migrations, err := loadMigrations(migrationFiles)
	if err != nil {
		return err
	}

	current, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}
	if latest := len(migrations); current > latest {
		return fmt.Errorf("%w: database schema version %d, newest known migration %d", ErrDatabaseNewer, current, latest)
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if err := apply(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

// apply runs one migration and records its number in user_version.
func apply(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %s: %w", m.name, err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return fmt.Errorf("run migration %s: %w", m.name, err)
	}
	// PRAGMA does not accept a bound parameter. The version is an int from the file name.
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = "+strconv.Itoa(m.version)); err != nil {
		return fmt.Errorf("set user_version for migration %s: %w", m.name, err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %s: %w", m.name, err)
	}
	return nil
}

// loadMigrations reads the migration files and sorts them by number.
// The numbers must start at 1 and have no gaps.
func loadMigrations(fsys fs.FS) ([]migration, error) {
	names, err := fs.Glob(fsys, "migrations/*.sql")
	if err != nil {
		return nil, fmt.Errorf("list migrations: %w", err)
	}

	migrations := make([]migration, 0, len(names))
	for _, name := range names {
		base := strings.TrimPrefix(name, "migrations/")
		prefix, _, ok := strings.Cut(base, "_")
		if !ok {
			return nil, fmt.Errorf("migration %s: name must be <number>_<description>.sql", base)
		}
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return nil, fmt.Errorf("migration %s: name must start with a number: %w", base, err)
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read migration %s: %w", base, err)
		}
		migrations = append(migrations, migration{version: version, name: base, sql: string(data)})
	}

	slices.SortFunc(migrations, func(a, b migration) int { return a.version - b.version })
	for i, m := range migrations {
		if m.version != i+1 {
			return nil, fmt.Errorf("migration %s: want number %d", m.name, i+1)
		}
	}
	return migrations, nil
}

// schemaVersion returns PRAGMA user_version.
func schemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return 0, fmt.Errorf("read user_version: %w", err)
	}
	return version, nil
}
