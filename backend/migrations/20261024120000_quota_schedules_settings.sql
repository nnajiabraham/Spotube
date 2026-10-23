-- +goose Up
-- +goose StatementBegin
-- Track counts per side (tracks_count held the larger of the two, which misled when one side
-- was empty).
ALTER TABLE mappings ADD COLUMN spotify_tracks_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE mappings ADD COLUMN youtube_tracks_count INTEGER NOT NULL DEFAULT 0;
UPDATE mappings SET spotify_tracks_count = tracks_count, youtube_tracks_count = tracks_count;

-- YouTube change probe: the itemCount Spotube expects (last read plus its own inserts). A
-- probe that matches skips the full read.
ALTER TABLE mappings ADD COLUMN youtube_item_count INTEGER;
ALTER TABLE mappings ADD COLUMN last_full_read_at INTEGER;

-- Schedule: every interval_minutes from schedule_start (HH:MM in schedule_tz) on the days in
-- schedule_days (Sunday = 1 ... Saturday = 64; 127 = every day). Daily is interval 1440.
ALTER TABLE mappings ADD COLUMN schedule_start TEXT NOT NULL DEFAULT '00:00';
ALTER TABLE mappings ADD COLUMN schedule_days INTEGER NOT NULL DEFAULT 127;
ALTER TABLE mappings ADD COLUMN schedule_tz TEXT;
-- When the scheduler runs the mapping next; NULL means as soon as possible.
ALTER TABLE mappings ADD COLUMN next_run_at INTEGER;

-- Each playlist syncs with exactly one partner. No duplicates exist today; keep the oldest
-- mapping if one ever does, so the unique indexes can be created.
DELETE FROM mappings WHERE rowid NOT IN (SELECT MIN(rowid) FROM mappings GROUP BY spotify_playlist_id);
DELETE FROM mappings WHERE rowid NOT IN (SELECT MIN(rowid) FROM mappings GROUP BY youtube_playlist_id);
CREATE UNIQUE INDEX idx_mappings_spotify_playlist ON mappings(spotify_playlist_id);
CREATE UNIQUE INDEX idx_mappings_youtube_playlist ON mappings(youtube_playlist_id);

-- The last full read of each synced YouTube playlist, kept so a restart or an unchanged probe
-- does not pay for another read.
CREATE TABLE playlist_cache (
    service TEXT NOT NULL,
    playlist_id TEXT NOT NULL,
    tracks_json TEXT NOT NULL,
    item_count INTEGER NOT NULL,
    read_at INTEGER NOT NULL,
    PRIMARY KEY (service, playlist_id)
);

-- Settings changed on the Settings page (env-pinned settings are never stored here).
CREATE TABLE app_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    updated INTEGER NOT NULL
);

-- Quota errors used to count as failed attempts, so items stopped being retried after three
-- quota-starved days. They were never wrong: queue them again.
UPDATE sync_items
SET status = 'pending', attempt_count = 0, error_message = NULL
WHERE status = 'error'
  AND (error_message LIKE '%quotaExceeded%' OR error_message LIKE '%dailyLimitExceeded%'
       OR error_message LIKE '%exceeded your%quota%');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE app_settings;
DROP TABLE playlist_cache;
DROP INDEX idx_mappings_youtube_playlist;
DROP INDEX idx_mappings_spotify_playlist;
ALTER TABLE mappings DROP COLUMN next_run_at;
ALTER TABLE mappings DROP COLUMN schedule_tz;
ALTER TABLE mappings DROP COLUMN schedule_days;
ALTER TABLE mappings DROP COLUMN schedule_start;
ALTER TABLE mappings DROP COLUMN last_full_read_at;
ALTER TABLE mappings DROP COLUMN youtube_item_count;
ALTER TABLE mappings DROP COLUMN youtube_tracks_count;
ALTER TABLE mappings DROP COLUMN spotify_tracks_count;
-- +goose StatementEnd
