package store_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
	_ "modernc.org/sqlite"
)

func TestOpen_MakesFolderAndAppliesMigrationsOnce(t *testing.T) {
	t.Parallel()
	// The folder does not exist yet. Open must make it.
	path := filepath.Join(t.TempDir(), "new-folder", "profit-tracker.db")

	first, err := store.Open(t.Context(), path)
	require.NoError(t, err)
	firstVersion, err := first.SchemaVersion(t.Context())
	require.NoError(t, err)
	require.NoError(t, first.Close())

	second, err := store.Open(t.Context(), path)
	require.NoError(t, err, "a second open must not apply the migrations again")
	t.Cleanup(func() { require.NoError(t, second.Close()) })
	secondVersion, err := second.SchemaVersion(t.Context())
	require.NoError(t, err)

	assert.Positive(t, firstVersion)
	assert.Equal(t, firstVersion, secondVersion)
}

func TestOpen_DatabaseNewerThanBinary_ReturnsError(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "profit-tracker.db")
	// A newer binary applied migration 999. This binary does not know it.
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = raw.ExecContext(t.Context(), "PRAGMA user_version = 999")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	_, err = store.Open(t.Context(), path)

	require.ErrorIs(t, err, store.ErrDatabaseNewer)
	assert.Contains(t, err.Error(), "999")
}

func TestOpen_PathWithURISpecialCharacters_OpensThatFile(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "a?b#c%20d")
	path := filepath.Join(dir, "profit?tracker#1%41.db")

	db, err := store.Open(t.Context(), path)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Contains(t, names, "profit?tracker#1%41.db", "the database must be at the exact path")
}

func TestOpen_AccountFromBeforeProvider_GetsProviderXTB(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "profit-tracker.db")
	// A database of an older binary: the migrations before the Provider, with one Account.
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	migrations, err := filepath.Glob(filepath.Join("migrations", "*.sql"))
	require.NoError(t, err)
	for _, name := range migrations {
		if filepath.Base(name) >= "0013" {
			break
		}
		data, err := os.ReadFile(name)
		require.NoError(t, err)
		_, err = raw.ExecContext(t.Context(), string(data))
		require.NoError(t, err, "migration %s", name)
	}
	_, err = raw.ExecContext(t.Context(), "PRAGMA user_version = 12")
	require.NoError(t, err)
	_, err = raw.ExecContext(t.Context(),
		"INSERT INTO accounts (account_number, name, type, currency) VALUES ('56000001', 'XTB USD', 'regular', 'USD')")
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	db, err := store.Open(t.Context(), path)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	accounts, err := db.Accounts(t.Context())
	require.NoError(t, err)

	require.Len(t, accounts, 1)
	assert.Equal(t, "xtb", accounts[0].Provider)
}
