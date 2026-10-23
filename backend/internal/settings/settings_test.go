package settings

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func env(vars map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := vars[k]; return v, ok }
}

func view(t *testing.T, s *Store, key string) View {
	t.Helper()
	for _, v := range s.List() {
		if v.Key == key {
			return v
		}
	}
	t.Fatalf("no setting %s", key)
	return View{}
}

func TestPrecedence(t *testing.T) {
	db, err := sqliteconn.OpenWithPragmas(filepath.Join(t.TempDir(), "s.db"))
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))
	defer db.Close()

	s, err := Load(db, env(nil))
	require.NoError(t, err)
	assert.False(t, s.Bool(SyncWorkersEnabled))
	assert.Equal(t, 10000, s.Int(YouTubeDailyQuota))
	assert.Equal(t, SourceDefault, view(t, s, YouTubeDailyQuota).Source)

	// Not pinned: saved to the database and kept across restarts.
	v, err := s.Set(YouTubeDailyQuota, " 20000 ")
	require.NoError(t, err)
	assert.Equal(t, "20000", v.Value)
	assert.Equal(t, SourceDatabase, v.Source)
	s, err = Load(db, env(nil))
	require.NoError(t, err)
	assert.Equal(t, 20000, s.Int(YouTubeDailyQuota))
	assert.Equal(t, SourceDatabase, view(t, s, YouTubeDailyQuota).Source)

	// An env var wins at startup over the database.
	s, err = Load(db, env(map[string]string{"YOUTUBE_DAILY_QUOTA": "50000", "SYNC_WORKERS_ENABLED": "1"}))
	require.NoError(t, err)
	assert.Equal(t, 50000, s.Int(YouTubeDailyQuota))
	assert.True(t, s.Bool(SyncWorkersEnabled))
	pinned := view(t, s, SyncWorkersEnabled)
	assert.True(t, pinned.EnvPinned)
	assert.Equal(t, "true", pinned.EnvValue)
	assert.Equal(t, SourceEnv, pinned.Source)

	// Changing a pinned setting lasts until restart and isn't stored.
	v, err = s.Set(SyncWorkersEnabled, "false")
	require.NoError(t, err)
	assert.Equal(t, SourceRuntime, v.Source)
	assert.False(t, s.Bool(SyncWorkersEnabled))
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key = ?`, SyncWorkersEnabled).Scan(&n))
	assert.Zero(t, n)
	v, err = s.Set(SyncWorkersEnabled, "true")
	require.NoError(t, err)
	assert.Equal(t, SourceEnv, v.Source, "back to the env value")

	s, err = Load(db, env(map[string]string{"SYNC_WORKERS_ENABLED": "true"}))
	require.NoError(t, err)
	assert.True(t, s.Bool(SyncWorkersEnabled), "the env value returns after a restart")

	// An empty env var counts as unset.
	s, err = Load(db, env(map[string]string{"YOUTUBE_DAILY_QUOTA": "  "}))
	require.NoError(t, err)
	assert.Equal(t, 20000, s.Int(YouTubeDailyQuota))
}

func TestValidation(t *testing.T) {
	s := Static(nil)
	_, err := s.Set("nope", "1")
	assert.ErrorIs(t, err, ErrUnknown)
	_, err = s.Set(SyncWorkersEnabled, "maybe")
	assert.ErrorContains(t, err, "true or false")
	_, err = s.Set(DrainItemsPerTick, "0")
	assert.ErrorContains(t, err, "between 1 and 1000")
	_, err = s.Set(DrainItemsPerTick, "1.5")
	assert.ErrorContains(t, err, "whole number")

	assert.NoError(t, s.Validate(map[string]string{DrainItemsPerTick: "5", BackupsEnabled: "false"}))
	assert.Error(t, s.Validate(map[string]string{DrainItemsPerTick: "5", YouTubeDailyQuota: "1"}))
	assert.Equal(t, 100, s.Int(DrainItemsPerTick), "validation applies nothing")

	_, err = Load(nil, env(map[string]string{"YOUTUBE_DAILY_QUOTA": "lots"}))
	assert.ErrorContains(t, err, "YOUTUBE_DAILY_QUOTA")
}

func TestStoredValueOutOfRangeFallsBackToDefault(t *testing.T) {
	db, err := sqliteconn.OpenWithPragmas(filepath.Join(t.TempDir(), "s.db"))
	require.NoError(t, err)
	require.NoError(t, migrate.Up(db))
	defer db.Close()
	_, err = db.Exec(`INSERT INTO app_settings (key, value, updated) VALUES (?, '99999', 0)`, DrainItemsPerTick)
	require.NoError(t, err)
	s, err := Load(db, env(nil))
	require.NoError(t, err)
	assert.Equal(t, 100, s.Int(DrainItemsPerTick))
}

func TestRegistryDefaultsAreValid(t *testing.T) {
	seen := map[string]bool{}
	for _, d := range Registry {
		assert.False(t, seen[d.Key], "duplicate %s", d.Key)
		seen[d.Key] = true
		_, err := normalize(d, d.Default)
		assert.NoError(t, err, d.Key)
		assert.NotEmpty(t, d.Label)
		assert.NotEmpty(t, d.Group)
	}
}
