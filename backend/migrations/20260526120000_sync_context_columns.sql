-- +goose Up
-- +goose StatementBegin
ALTER TABLE activity_logs ADD COLUMN sync_item_id TEXT REFERENCES sync_items(id) ON DELETE SET NULL;
ALTER TABLE activity_logs ADD COLUMN details_json TEXT;

ALTER TABLE sync_items ADD COLUMN analysis_context_json TEXT;

CREATE INDEX idx_activity_logs_sync_item_id ON activity_logs(sync_item_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_activity_logs_sync_item_id;

ALTER TABLE sync_items DROP COLUMN analysis_context_json;
ALTER TABLE activity_logs DROP COLUMN details_json;
ALTER TABLE activity_logs DROP COLUMN sync_item_id;
-- +goose StatementEnd
