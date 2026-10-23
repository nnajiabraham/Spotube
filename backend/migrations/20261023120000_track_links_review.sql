-- +goose Up
-- +goose StatementBegin
-- Remembered cross-service pairs. A NULL side means "this song is not on that service",
-- so the sync engine never searches for it again.
CREATE TABLE track_links (
    id TEXT PRIMARY KEY,
    spotify_id TEXT,
    youtube_id TEXT,
    title TEXT,
    artist TEXT,
    origin TEXT NOT NULL CHECK (origin IN ('auto', 'manual')),
    score REAL,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL,
    CHECK (spotify_id IS NOT NULL OR youtube_id IS NOT NULL)
);
CREATE UNIQUE INDEX idx_track_links_spotify ON track_links(spotify_id) WHERE spotify_id IS NOT NULL;
CREATE UNIQUE INDEX idx_track_links_youtube ON track_links(youtube_id) WHERE youtube_id IS NOT NULL;

-- Measured API usage per day (YouTube quota days start at midnight Pacific).
CREATE TABLE api_usage (
    day TEXT NOT NULL,
    service TEXT NOT NULL,
    units INTEGER NOT NULL DEFAULT 0,
    calls INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (day, service)
);

-- Rebuild sync_items to allow status 'review' (no confident match; waits for the user).
-- Dropping the old table fires activity_logs' ON DELETE SET NULL, so keep those links aside.
CREATE TEMP TABLE keep_log_items AS
    SELECT id, sync_item_id FROM activity_logs WHERE sync_item_id IS NOT NULL;

CREATE TABLE sync_items_new (
    id TEXT PRIMARY KEY,
    mapping_id TEXT NOT NULL REFERENCES mappings(id) ON DELETE CASCADE,
    operation TEXT NOT NULL CHECK (operation IN ('add', 'remove', 'rename')),
    service TEXT NOT NULL CHECK (service IN ('spotify', 'youtube')),
    track_id TEXT,
    track_title TEXT,
    track_artist TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'error', 'skipped', 'review')),
    error_message TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at INTEGER,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL,
    analysis_context_json TEXT
);
INSERT INTO sync_items_new
    SELECT id, mapping_id, operation, service, track_id, track_title, track_artist, status,
           error_message, attempt_count, last_attempt_at, created, updated, analysis_context_json
    FROM sync_items;
DROP TABLE sync_items;
ALTER TABLE sync_items_new RENAME TO sync_items;
CREATE INDEX idx_sync_items_mapping_id ON sync_items(mapping_id);
CREATE INDEX idx_sync_items_status ON sync_items(status);
CREATE UNIQUE INDEX idx_sync_items_unique ON sync_items(mapping_id, service, operation, track_id);

UPDATE activity_logs
SET sync_item_id = (SELECT k.sync_item_id FROM keep_log_items k WHERE k.id = activity_logs.id)
WHERE id IN (SELECT id FROM keep_log_items);
DROP TABLE keep_log_items;

-- Searches that found nothing confident now wait for review instead of retrying.
UPDATE sync_items SET status = 'review'
WHERE status = 'error' AND (error_message LIKE 'no confident match%' OR error_message LIKE 'no match on%');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE TEMP TABLE keep_log_items AS
    SELECT id, sync_item_id FROM activity_logs WHERE sync_item_id IS NOT NULL;

CREATE TABLE sync_items_old (
    id TEXT PRIMARY KEY,
    mapping_id TEXT NOT NULL REFERENCES mappings(id) ON DELETE CASCADE,
    operation TEXT NOT NULL CHECK (operation IN ('add', 'remove', 'rename')),
    service TEXT NOT NULL CHECK (service IN ('spotify', 'youtube')),
    track_id TEXT,
    track_title TEXT,
    track_artist TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'done', 'error', 'skipped')),
    error_message TEXT,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at INTEGER,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL,
    analysis_context_json TEXT
);
INSERT INTO sync_items_old
    SELECT id, mapping_id, operation, service, track_id, track_title, track_artist,
           CASE status WHEN 'review' THEN 'error' ELSE status END,
           error_message, attempt_count, last_attempt_at, created, updated, analysis_context_json
    FROM sync_items;
DROP TABLE sync_items;
ALTER TABLE sync_items_old RENAME TO sync_items;
CREATE INDEX idx_sync_items_mapping_id ON sync_items(mapping_id);
CREATE INDEX idx_sync_items_status ON sync_items(status);
CREATE UNIQUE INDEX idx_sync_items_unique ON sync_items(mapping_id, service, operation, track_id);

UPDATE activity_logs
SET sync_item_id = (SELECT k.sync_item_id FROM keep_log_items k WHERE k.id = activity_logs.id)
WHERE id IN (SELECT id FROM keep_log_items);
DROP TABLE keep_log_items;

DROP TABLE IF EXISTS api_usage;
DROP TABLE IF EXISTS track_links;
-- +goose StatementEnd
