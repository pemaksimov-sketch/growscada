-- +goose Up
CREATE TABLE engineering.libraries (
    id uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES engineering.organizations(id) ON DELETE CASCADE,
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 128),
    is_public boolean NOT NULL DEFAULT false,
    version integer NOT NULL CHECK (version > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (organization_id, name),
    UNIQUE (organization_id, id)
);

INSERT INTO engineering.libraries(id, organization_id, name, is_public, version)
VALUES ('00000000-0000-4000-8000-000000000003',
        '00000000-0000-4000-8000-000000000002', 'GrowSCADA', true, 1);

ALTER TABLE engineering.widget_types ADD COLUMN library_id uuid;
UPDATE engineering.widget_types
SET library_id = '00000000-0000-4000-8000-000000000003';
ALTER TABLE engineering.widget_types ALTER COLUMN library_id
    SET DEFAULT '00000000-0000-4000-8000-000000000003';
ALTER TABLE engineering.widget_types ALTER COLUMN library_id SET NOT NULL;
ALTER TABLE engineering.widget_types ADD CONSTRAINT fk_widget_types_library
    FOREIGN KEY (library_id) REFERENCES engineering.libraries(id) ON DELETE CASCADE;
ALTER TABLE engineering.widget_types ADD CONSTRAINT uq_widget_types_library_name
    UNIQUE (library_id, name);
ALTER TABLE engineering.widget_types ADD CONSTRAINT uq_widget_types_library_id
    UNIQUE (library_id, id);

-- Projects resolve WidgetTypes from their pinned immutable Release, not from
-- the editable Library Draft. Keep Widget rows when a draft type is removed.
ALTER TABLE engineering.widgets DROP CONSTRAINT IF EXISTS fk_widgets_type_id;

CREATE TABLE engineering.library_releases (
    id uuid PRIMARY KEY,
    library_id uuid NOT NULL REFERENCES engineering.libraries(id) ON DELETE CASCADE,
    number integer NOT NULL CHECK (number > 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    content jsonb NOT NULL CHECK (jsonb_typeof(content) = 'object'),
    UNIQUE (library_id, number),
    UNIQUE (library_id, id)
);

-- +goose StatementBegin
CREATE FUNCTION engineering.reject_library_release_mutation() RETURNS trigger AS $$
BEGIN
    RAISE EXCEPTION 'Library Releases are immutable' USING ERRCODE = '55000';
END;
$$ LANGUAGE plpgsql;
CREATE TRIGGER library_releases_immutable
    BEFORE UPDATE OR DELETE ON engineering.library_releases
    FOR EACH ROW EXECUTE FUNCTION engineering.reject_library_release_mutation();
-- +goose StatementEnd

INSERT INTO engineering.library_releases(id, library_id, number, content)
SELECT '00000000-0000-4000-8000-000000000004',
       '00000000-0000-4000-8000-000000000003',
       1,
       jsonb_build_object(
           'widget_types', COALESCE(jsonb_agg(jsonb_build_object(
               'id', wt.id,
               'name', wt.name,
               'html_template', wt.html_template,
               'script', wt.script,
               'script_language', wt.script_language,
               'default_width', wt.default_width,
               'default_height', wt.default_height,
               'input_ports', wt.input_ports,
               'version', wt.version
           ) ORDER BY wt.pk_id), '[]'::jsonb)
       )
FROM engineering.widget_types wt
WHERE wt.library_id = '00000000-0000-4000-8000-000000000003';

CREATE TABLE engineering.project_library_pins (
    project_id uuid NOT NULL REFERENCES engineering.projects(id) ON DELETE CASCADE,
    library_id uuid NOT NULL REFERENCES engineering.libraries(id) ON DELETE CASCADE,
    release_id uuid NOT NULL,
    pinned_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (project_id, library_id),
    FOREIGN KEY (library_id, release_id)
        REFERENCES engineering.library_releases(library_id, id) ON DELETE RESTRICT
);

INSERT INTO engineering.project_library_pins(project_id, library_id, release_id)
VALUES ('00000000-0000-4000-8000-000000000001',
        '00000000-0000-4000-8000-000000000003',
        '00000000-0000-4000-8000-000000000004');

CREATE INDEX idx_libraries_organization ON engineering.libraries(organization_id);
CREATE INDEX idx_library_releases_library ON engineering.library_releases(library_id, number DESC);
CREATE INDEX idx_project_library_pins_release ON engineering.project_library_pins(library_id, release_id);

-- +goose Down
-- Removing Releases would make existing Project Widgets unresolvable.
-- Restore a backup to return to the mutable, organization-wide type model.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Restore the pre-Library Release backup to reverse this migration'; END $$;
-- +goose StatementEnd
