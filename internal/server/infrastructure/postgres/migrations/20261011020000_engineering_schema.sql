-- +goose Up
CREATE SCHEMA engineering;
ALTER TABLE public.organizations SET SCHEMA engineering;
ALTER TABLE public.projects SET SCHEMA engineering;
ALTER TABLE public.tags SET SCHEMA engineering;
ALTER TABLE public.scenes SET SCHEMA engineering;
ALTER TABLE public.widgets SET SCHEMA engineering;
ALTER TABLE public.widget_types SET SCHEMA engineering;
ALTER TABLE public.outbox SET SCHEMA engineering;
ALTER TABLE public.policy_outbox SET SCHEMA engineering;
-- Existing value/quality columns remain here only until the migration runner
-- has copied them through the Runtime port, possibly to a different database.
-- +goose Down
ALTER TABLE engineering.policy_outbox SET SCHEMA public;
ALTER TABLE engineering.outbox SET SCHEMA public;
ALTER TABLE engineering.widget_types SET SCHEMA public;
ALTER TABLE engineering.widgets SET SCHEMA public;
ALTER TABLE engineering.scenes SET SCHEMA public;
ALTER TABLE engineering.tags SET SCHEMA public;
ALTER TABLE engineering.projects SET SCHEMA public;
ALTER TABLE engineering.organizations SET SCHEMA public;
DROP SCHEMA engineering;
