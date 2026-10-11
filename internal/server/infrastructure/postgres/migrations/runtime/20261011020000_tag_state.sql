-- +goose Up
CREATE TABLE runtime.tags (
    id uuid PRIMARY KEY,
    project_id uuid NOT NULL,
    name text NOT NULL,
    type text NOT NULL CHECK(type IN ('integer','string','boolean')),
    origin text NOT NULL CHECK(origin IN ('Project','Runtime')),
    value text NOT NULL,
    quality text NOT NULL CHECK(quality IN ('good','bad','uncertain')),
    sequence bigint NOT NULL CHECK(sequence >= 0),
    UNIQUE(project_id,name)
);
CREATE INDEX idx_runtime_tags_project ON runtime.tags(project_id);
-- +goose Down
DROP TABLE runtime.tags;
