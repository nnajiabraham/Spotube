-- +goose Up
-- +goose StatementBegin
CREATE TABLE settings (
    id TEXT PRIMARY KEY DEFAULT '1',
    spotify_client_id TEXT,
    spotify_client_secret TEXT,
    google_client_id TEXT,
    google_client_secret TEXT,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL
);

CREATE TABLE oauth_tokens (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL CHECK (provider IN ('spotify', 'google')),
    access_token TEXT,
    refresh_token TEXT,
    expiry INTEGER,
    scopes TEXT,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL
);

CREATE UNIQUE INDEX idx_oauth_tokens_provider ON oauth_tokens(provider);

CREATE TABLE mappings (
    id TEXT PRIMARY KEY,
    spotify_playlist_id TEXT NOT NULL,
    youtube_playlist_id TEXT NOT NULL,
    spotify_playlist_name TEXT,
    youtube_playlist_name TEXT,
    sync_name INTEGER NOT NULL DEFAULT 1,
    sync_tracks INTEGER NOT NULL DEFAULT 1,
    interval_minutes INTEGER NOT NULL DEFAULT 60,
    last_analysis_at INTEGER,
    tracks_count INTEGER NOT NULL DEFAULT 0,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL
);

CREATE TABLE sync_items (
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
    updated INTEGER NOT NULL
);

CREATE INDEX idx_sync_items_mapping_id ON sync_items(mapping_id);
CREATE INDEX idx_sync_items_status ON sync_items(status);
CREATE UNIQUE INDEX idx_sync_items_unique ON sync_items(mapping_id, service, operation, track_id);

CREATE TABLE blacklist (
    id TEXT PRIMARY KEY,
    mapping_id TEXT NOT NULL REFERENCES mappings(id) ON DELETE CASCADE,
    service TEXT NOT NULL CHECK (service IN ('spotify', 'youtube')),
    track_id TEXT NOT NULL,
    reason TEXT,
    skip_counter INTEGER NOT NULL DEFAULT 0,
    last_skipped_at INTEGER,
    created INTEGER NOT NULL,
    updated INTEGER NOT NULL
);

CREATE INDEX idx_blacklist_mapping_id ON blacklist(mapping_id);
CREATE UNIQUE INDEX idx_blacklist_unique ON blacklist(mapping_id, service, track_id);

CREATE TABLE activity_logs (
    id TEXT PRIMARY KEY,
    level TEXT NOT NULL CHECK (level IN ('info', 'warn', 'error')),
    message TEXT NOT NULL,
    mapping_id TEXT,
    job_type TEXT NOT NULL CHECK (job_type IN ('analysis', 'executor', 'system')),
    created INTEGER NOT NULL
);

CREATE INDEX idx_activity_logs_job_type ON activity_logs(job_type);
CREATE INDEX idx_activity_logs_created ON activity_logs(created);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS activity_logs;
DROP TABLE IF EXISTS blacklist;
DROP TABLE IF EXISTS sync_items;
DROP TABLE IF EXISTS mappings;
DROP TABLE IF EXISTS oauth_tokens;
DROP TABLE IF EXISTS settings;
-- +goose StatementEnd 