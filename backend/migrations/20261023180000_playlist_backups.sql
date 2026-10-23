-- +goose Up
-- +goose StatementBegin
-- Playlists the user chose to back up, on either service.
CREATE TABLE playlist_backups (
    id TEXT PRIMARY KEY,
    service TEXT NOT NULL CHECK (service IN ('spotify', 'youtube')),
    playlist_id TEXT NOT NULL,
    playlist_name TEXT,
    enabled INTEGER NOT NULL DEFAULT 1,
    interval_hours INTEGER NOT NULL DEFAULT 24,
    keep_snapshots INTEGER NOT NULL DEFAULT 30,
    last_checked_at INTEGER,
    last_error TEXT,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL
);
CREATE UNIQUE INDEX idx_playlist_backups_playlist ON playlist_backups(service, playlist_id);

-- One row per distinct playlist state: a check that finds no change only moves confirmed_at.
CREATE TABLE playlist_snapshots (
    id TEXT PRIMARY KEY,
    backup_id TEXT NOT NULL REFERENCES playlist_backups(id) ON DELETE CASCADE,
    playlist_name TEXT,
    track_count INTEGER NOT NULL,
    -- Change against the previous snapshot (NULL for the first one).
    added_count INTEGER,
    removed_count INTEGER,
    content_hash TEXT NOT NULL,
    source TEXT NOT NULL CHECK (source IN ('initial', 'scheduled', 'manual', 'sync')),
    created INTEGER NOT NULL,
    confirmed_at INTEGER NOT NULL,
    tracks_json TEXT NOT NULL
);
CREATE INDEX idx_playlist_snapshots_backup ON playlist_snapshots(backup_id, created);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS playlist_snapshots;
DROP TABLE IF EXISTS playlist_backups;
-- +goose StatementEnd
