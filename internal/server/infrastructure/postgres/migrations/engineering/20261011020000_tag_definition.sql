-- +goose Up
ALTER TABLE engineering.tags RENAME TO tag_definitions;
ALTER TABLE engineering.tag_definitions ADD COLUMN origin text NOT NULL DEFAULT 'Project'
    CHECK(origin IN ('Project','Runtime'));
ALTER TABLE engineering.tag_definitions DROP CONSTRAINT chk_tags_quality;
DROP INDEX engineering.idx_tags_quality;
ALTER TABLE engineering.tag_definitions DROP COLUMN value;
ALTER TABLE engineering.tag_definitions DROP COLUMN quality;
-- A process sequence is not an edit Version. Existing definitions start at 1;
-- their previous counters were preserved as Runtime sequences by the runner.
UPDATE engineering.tag_definitions SET version=1;
ALTER TABLE engineering.widgets ADD COLUMN entity_origin text NOT NULL DEFAULT 'Project'
    CHECK(entity_origin IN ('Project','Runtime'));
-- +goose Down
-- Lossless reversal would require fetching process state through Runtime.
-- Refuse SQL-only rollback rather than silently invent values and Quality.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Restore the pre-migration backup to reverse the node split'; END $$;
-- +goose StatementEnd
