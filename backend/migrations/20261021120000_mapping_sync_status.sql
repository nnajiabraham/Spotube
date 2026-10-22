-- +goose Up
-- +goose StatementBegin
-- Outcome of the most recent sync run per mapping, shown in the UI so failures are visible
-- without digging through activity logs.
ALTER TABLE mappings ADD COLUMN last_sync_status TEXT;
ALTER TABLE mappings ADD COLUMN last_sync_error TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE mappings DROP COLUMN last_sync_error;
ALTER TABLE mappings DROP COLUMN last_sync_status;
-- +goose StatementEnd
