-- Add extra columns to support transforms and inputs schema
ALTER TABLE fact_mappings ADD COLUMN IF NOT EXISTS transform_args JSONB DEFAULT '[]'::jsonb;
ALTER TABLE fact_resolvers ADD COLUMN IF NOT EXISTS needs JSONB DEFAULT '[]'::jsonb;
ALTER TABLE actions ADD COLUMN IF NOT EXISTS inputs_schema JSONB DEFAULT '{}'::jsonb;
-- 0003_facts_manifest.sql
-- Extend facts mappings/resolvers and actions for manifest inputs schema.

ALTER TABLE fact_mappings ADD COLUMN IF NOT EXISTS transform_args jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE fact_resolvers ADD COLUMN IF NOT EXISTS needs jsonb NOT NULL DEFAULT '[]'::jsonb;
ALTER TABLE actions ADD COLUMN IF NOT EXISTS inputs_schema jsonb NOT NULL DEFAULT '{}'::jsonb;
