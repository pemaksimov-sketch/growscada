-- +goose Up
CREATE TABLE organizations (
    id uuid PRIMARY KEY,
    name text NOT NULL
);
INSERT INTO organizations (id, name) VALUES ('00000000-0000-4000-8000-000000000002', 'Default');
CREATE TABLE projects (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organizations(id),
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 128),
    version integer NOT NULL,
    CONSTRAINT uq_projects_name UNIQUE (organization_id, name)
);
INSERT INTO projects (id, organization_id, name, version)
VALUES ('00000000-0000-4000-8000-000000000001', '00000000-0000-4000-8000-000000000002', 'Main', 1);

ALTER TABLE tags ADD COLUMN project_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000001'
    REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE tags DROP CONSTRAINT uq_tags_name;
ALTER TABLE tags ADD CONSTRAINT uq_tags_name UNIQUE (project_id, name);
ALTER TABLE scenes ADD COLUMN project_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000001'
    REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE scenes ADD CONSTRAINT uq_scenes_project UNIQUE (id, project_id);
ALTER TABLE widgets ADD COLUMN project_id uuid NOT NULL DEFAULT '00000000-0000-4000-8000-000000000001'
    REFERENCES projects(id) ON DELETE CASCADE;
UPDATE widgets w SET project_id=s.project_id FROM scenes s WHERE s.id=w.scene_id;
ALTER TABLE widgets ADD CONSTRAINT fk_widgets_scene_project FOREIGN KEY (scene_id, project_id)
    REFERENCES scenes(id, project_id) ON DELETE CASCADE;
CREATE INDEX idx_scenes_project ON scenes(project_id);
CREATE INDEX idx_widgets_project ON widgets(project_id);

-- Events survive deletion of their owner, so outboxes deliberately have no
-- project FK. NULL identifies organization-wide library/system events.
ALTER TABLE outbox ADD COLUMN project_id uuid;
UPDATE outbox SET project_id='00000000-0000-4000-8000-000000000001'
WHERE type LIKE 'tag_%' OR type LIKE 'scene_%' OR type IN ('widget_created','widget_updated','widget_deleted');
ALTER TABLE policy_outbox ADD COLUMN project_id uuid;
UPDATE policy_outbox SET project_id='00000000-0000-4000-8000-000000000001'
WHERE type LIKE 'tag_%' OR type LIKE 'scene_%' OR type IN ('widget_created','widget_updated','widget_deleted');

-- +goose Down
-- +goose StatementBegin
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM projects WHERE id <> '00000000-0000-4000-8000-000000000001') THEN
        RAISE EXCEPTION 'Cannot remove project isolation while additional projects exist';
    END IF;
END $$;
-- +goose StatementEnd
ALTER TABLE policy_outbox DROP COLUMN project_id;
ALTER TABLE outbox DROP COLUMN project_id;
ALTER TABLE widgets DROP CONSTRAINT fk_widgets_scene_project;
DROP INDEX idx_widgets_project;
ALTER TABLE widgets DROP COLUMN project_id;
ALTER TABLE scenes DROP CONSTRAINT uq_scenes_project;
DROP INDEX idx_scenes_project;
ALTER TABLE scenes DROP COLUMN project_id;
ALTER TABLE tags DROP CONSTRAINT uq_tags_name;
ALTER TABLE tags DROP COLUMN project_id;
ALTER TABLE tags ADD CONSTRAINT uq_tags_name UNIQUE(name);
DROP TABLE projects;
DROP TABLE organizations;
