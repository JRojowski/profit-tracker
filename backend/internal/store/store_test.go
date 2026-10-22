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
