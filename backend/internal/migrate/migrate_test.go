package migrate

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

const opTimeout = 5 * time.Second

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), opTimeout)
}

func openTestDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}

	return db, dbPath
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	ctx, cancel := withTimeout()
	defer cancel()

	row := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", name)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("check table exists: %v", err)
	}
	return count > 0
}

func TestMigrationsUpDownIdempotent(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	if err := Up(db); err != nil {
		t.Fatalf("up failed: %v", err)
	}

	if err := Up(db); err != nil {
		t.Fatalf("second up failed: %v", err)
	}

	if !tableExists(t, db, "settings") {
		t.Fatalf("expected settings table after up")
	}

	for tableExists(t, db, "settings") {
		if err := Down(db); err != nil {
			t.Fatalf("down failed: %v", err)
		}
	}

	if err := Up(db); err != nil {
		t.Fatalf("final up failed: %v", err)
	}
}

func TestMigrationConstraintsAndCascade(t *testing.T) {
	db, _ := openTestDB(t)
	defer db.Close()

	if err := Up(db); err != nil {
		t.Fatalf("up failed: %v", err)
	}
	defer Down(db)

	now := time.Now().Unix()

	mustExec(t, db, "INSERT INTO settings (id, created, updated) VALUES ('1', ?, ?) ON CONFLICT(id) DO UPDATE SET created=excluded.created, updated=excluded.updated", now, now)

	ctx, cancel := withTimeout()
	if _, err := db.QueryContext(ctx, "SELECT id FROM settings LIMIT 1"); err != nil && !errors.Is(err, sql.ErrNoRows) {
		cancel()
		t.Fatalf("settings query failed: %v", err)
	}
	cancel()

	mustExec(t, db, "INSERT INTO oauth_tokens (id, provider, created, updated) VALUES (?, 'spotify', ?, ?)", "token_spotify", now, now)
	if _, err := execContext(t, db, "INSERT INTO oauth_tokens (id, provider, created, updated) VALUES (?, 'spotify', ?, ?)", "token_spotify_dup", now, now); err == nil {
		t.Fatalf("expected unique provider constraint violation")
	}

	if _, err := execContext(t, db, "INSERT INTO oauth_tokens (id, provider, created, updated) VALUES (?, 'invalid', ?, ?)", "token_invalid", now, now); err == nil {
		t.Fatalf("expected provider CHECK constraint violation")
	}

	mappingID := "mapping-1"
	mustExec(t, db, "INSERT INTO mappings (id, spotify_playlist_id, youtube_playlist_id, created, updated) VALUES (?, ?, ?, ?, ?)", mappingID, "spot", "yt", now, now)

	syncID := "sync-1"
	mustExec(t, db, "INSERT INTO sync_items (id, mapping_id, operation, service, track_id, status, created, updated) VALUES (?, ?, 'add', 'spotify', 'track', 'pending', ?, ?)", syncID, mappingID, now, now)

	if _, err := execContext(t, db, "INSERT INTO sync_items (id, mapping_id, operation, service, track_id, status, created, updated) VALUES (?, ?, 'add', 'spotify', 'track', 'pending', ?, ?)", "sync-dup", mappingID, now, now); err == nil {
		t.Fatalf("expected unique composite constraint violation on sync_items")
	}

	mustExec(t, db, "INSERT INTO blacklist (id, mapping_id, service, track_id, created, updated) VALUES (?, ?, 'spotify', 'track', ?, ?)", "blacklist-1", mappingID, now, now)

	mustExec(t, db, "DELETE FROM mappings WHERE id=?", mappingID)

	if countRows(t, db, "SELECT COUNT(*) FROM sync_items") != 0 {
		t.Fatalf("expected sync_items to cascade delete with mapping")
	}

	if countRows(t, db, "SELECT COUNT(*) FROM blacklist") != 0 {
		t.Fatalf("expected blacklist to cascade delete with mapping")
	}
}

func execContext(t *testing.T, db *sql.DB, query string, args ...any) (sql.Result, error) {
	t.Helper()
	ctx, cancel := withTimeout()
	defer cancel()
	return db.ExecContext(ctx, query, args...)
}

func mustExec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := execContext(t, db, query, args...); err != nil {
		t.Fatalf("exec failed: %v", err)
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	ctx, cancel := withTimeout()
	defer cancel()

	row := db.QueryRowContext(ctx, query, args...)
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("count rows failed: %v", err)
	}
	return count
}
