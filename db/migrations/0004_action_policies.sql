-- Add action-scoped policies support
-- Adds action_key to policy_versions and adjusts uniqueness to (tenant_id, action_key, version)

ALTER TABLE policy_versions ADD COLUMN IF NOT EXISTS action_key TEXT NOT NULL DEFAULT '';

DO $$
BEGIN
  -- Drop old unique constraint on (tenant_id, version) if it exists
  IF EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'policy_versions'::regclass
      AND conname = 'policy_versions_tenant_id_version_key'
  ) THEN
    ALTER TABLE policy_versions DROP CONSTRAINT policy_versions_tenant_id_version_key;
  END IF;
EXCEPTION WHEN undefined_table THEN
  -- table might not exist in certain partial deployments
  NULL;
END $$;

-- Create new unique constraint including action_key
DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conrelid = 'policy_versions'::regclass
      AND conname = 'policy_versions_tenant_action_version_key'
  ) THEN
    ALTER TABLE policy_versions ADD CONSTRAINT policy_versions_tenant_action_version_key UNIQUE (tenant_id, action_key, version);
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_policy_versions_tenant_action_status ON policy_versions(tenant_id, action_key, status, version DESC);
