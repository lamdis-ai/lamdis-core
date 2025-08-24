-- Policy/facts/decisions schema with RLS by tenant

DO $$ BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app') THEN
    CREATE ROLE app;
  END IF;
END $$;

-- Actions optional registry (keyed)
CREATE TABLE IF NOT EXISTS actions (
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key TEXT NOT NULL,
  display_name TEXT,
  inputs_schema JSONB DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, key)
);

-- Fact resolvers and mappings
CREATE TABLE IF NOT EXISTS fact_resolvers (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key TEXT NOT NULL,
  name TEXT NOT NULL,
  connector_key TEXT,
  request_template JSONB DEFAULT '{}'::jsonb,
  response_sample JSONB DEFAULT '{}'::jsonb,
  needs JSONB DEFAULT '[]'::jsonb,
  enabled BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(tenant_id, action_key, name)
);

CREATE TABLE IF NOT EXISTS fact_mappings (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key TEXT NOT NULL,
  name TEXT NOT NULL,
  jmespath TEXT NOT NULL,
  fact_key TEXT NOT NULL,
  transform TEXT,
  transform_args JSONB DEFAULT '[]'::jsonb,
  required BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(tenant_id, action_key, name)
);

-- Policies: versions
CREATE TABLE IF NOT EXISTS policy_versions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  version INT NOT NULL,
  compiled_rego TEXT,
  status TEXT NOT NULL DEFAULT 'draft', -- draft|published|archived
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(tenant_id, version)
);

-- Decisions and executions
CREATE TABLE IF NOT EXISTS decisions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key TEXT NOT NULL,
  inputs JSONB NOT NULL DEFAULT '{}'::jsonb,
  facts JSONB NOT NULL DEFAULT '{}'::jsonb,
  policy_version INT NOT NULL DEFAULT 0,
  status TEXT NOT NULL,
  reasons JSONB,
  needs JSONB,
  alternatives JSONB,
  hash TEXT,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS executions (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key TEXT NOT NULL,
  decision_id UUID REFERENCES decisions(id) ON DELETE SET NULL,
  idempotency_key TEXT,
  steps JSONB,
  result JSONB,
  status TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- RLS
ALTER TABLE actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE fact_resolvers ENABLE ROW LEVEL SECURITY;
ALTER TABLE fact_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE decisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE executions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenants_rls_actions ON actions;
CREATE POLICY tenants_rls_actions ON actions USING (tenant_id = current_setting('app.tenant_id')::uuid);

DROP POLICY IF EXISTS tenants_rls_resolvers ON fact_resolvers;
CREATE POLICY tenants_rls_resolvers ON fact_resolvers USING (tenant_id = current_setting('app.tenant_id')::uuid);

DROP POLICY IF EXISTS tenants_rls_mappings ON fact_mappings;
CREATE POLICY tenants_rls_mappings ON fact_mappings USING (tenant_id = current_setting('app.tenant_id')::uuid);

DROP POLICY IF EXISTS tenants_rls_policy_versions ON policy_versions;
CREATE POLICY tenants_rls_policy_versions ON policy_versions USING (tenant_id = current_setting('app.tenant_id')::uuid);

DROP POLICY IF EXISTS tenants_rls_decisions ON decisions;
CREATE POLICY tenants_rls_decisions ON decisions USING (tenant_id = current_setting('app.tenant_id')::uuid);

DROP POLICY IF EXISTS tenants_rls_executions ON executions;
CREATE POLICY tenants_rls_executions ON executions USING (tenant_id = current_setting('app.tenant_id')::uuid);

-- Indices
CREATE INDEX IF NOT EXISTS idx_decisions_hash ON decisions(hash);
CREATE INDEX IF NOT EXISTS idx_decisions_status ON decisions(status);
CREATE INDEX IF NOT EXISTS idx_executions_action ON executions(action_key);

-- Triggers
DROP TRIGGER IF EXISTS actions_touch ON actions;
CREATE TRIGGER actions_touch BEFORE UPDATE ON actions FOR EACH ROW EXECUTE PROCEDURE touch_updated_at();
DROP TRIGGER IF EXISTS fact_resolvers_touch ON fact_resolvers;
CREATE TRIGGER fact_resolvers_touch BEFORE UPDATE ON fact_resolvers FOR EACH ROW EXECUTE PROCEDURE touch_updated_at();
DROP TRIGGER IF EXISTS fact_mappings_touch ON fact_mappings;
CREATE TRIGGER fact_mappings_touch BEFORE UPDATE ON fact_mappings FOR EACH ROW EXECUTE PROCEDURE touch_updated_at();
DROP TRIGGER IF EXISTS policy_versions_touch ON policy_versions;
CREATE TRIGGER policy_versions_touch BEFORE UPDATE ON policy_versions FOR EACH ROW EXECUTE PROCEDURE touch_updated_at();
-- 0002_policies.sql
-- Schema for policies, facts engine, orchestrator, and secrets with strict RLS.

-- Enable required extension for JSON indexing if not already present
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- All tables include tenant_id UUID NOT NULL and RLS is enabled to restrict to current_setting('app.tenant_id').

-- Actions catalog (per-tenant logical actions)
CREATE TABLE IF NOT EXISTS actions (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  key text NOT NULL,
  display_name text NOT NULL,
  requires_preflight boolean NOT NULL DEFAULT true,
  flow jsonb NOT NULL DEFAULT '{}'::jsonb,
  enabled boolean NOT NULL DEFAULT true,
  version int NOT NULL DEFAULT 1,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, key)
);

-- Policy sets and versions
CREATE TABLE IF NOT EXISTS policy_sets (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name text NOT NULL,
  environment text NOT NULL CHECK (environment IN ('dev','staging','prod')),
  UNIQUE(tenant_id, name, environment)
);

CREATE TABLE IF NOT EXISTS policy_versions (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  policy_set_id uuid NOT NULL REFERENCES policy_sets(id) ON DELETE CASCADE,
  version int NOT NULL,
  status text NOT NULL CHECK (status IN ('draft','published','archived')),
  dsl jsonb NOT NULL,
  compiled_rego text,
  created_by text,
  created_at timestamptz NOT NULL DEFAULT now(),
  published_at timestamptz,
  UNIQUE(tenant_id, policy_set_id, version)
);

-- Fact resolvers & mappings
CREATE TABLE IF NOT EXISTS fact_resolvers (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key text NOT NULL,
  name text NOT NULL,
  connector_key text NOT NULL,
  request_template jsonb NOT NULL DEFAULT '{}'::jsonb,
  response_sample jsonb,
  enabled boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS fact_mappings (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key text NOT NULL,
  name text NOT NULL,
  jmespath text NOT NULL,
  fact_key text NOT NULL,
  transform text,
  required boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Alternatives (suggestions when blocked or conditional)
CREATE TABLE IF NOT EXISTS alternatives (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key text NOT NULL,
  "when" text NOT NULL CHECK ("when" IN ('blocked','conditions')),
  alt_action_key text NOT NULL,
  params_template jsonb NOT NULL DEFAULT '{}'::jsonb,
  consent_template jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Decisions & Executions
CREATE TABLE IF NOT EXISTS decisions (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key text NOT NULL,
  inputs jsonb NOT NULL DEFAULT '{}'::jsonb,
  facts jsonb NOT NULL DEFAULT '{}'::jsonb,
  policy_version int NOT NULL,
  status text NOT NULL CHECK (status IN ('ALLOW','ALLOW_WITH_CONDITIONS','BLOCKED','NEEDS_INPUT')),
  reasons jsonb,
  needs jsonb,
  alternatives jsonb,
  hash text,
  expires_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS executions (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_key text NOT NULL,
  decision_id uuid NOT NULL REFERENCES decisions(id) ON DELETE CASCADE,
  idempotency_key text,
  steps jsonb NOT NULL DEFAULT '[]'::jsonb,
  result jsonb,
  status text NOT NULL CHECK (status IN ('SUCCEEDED','FAILED','PARTIAL')),
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Secrets (envelope-encrypted at application layer)
CREATE TABLE IF NOT EXISTS secrets (
  id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
  tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  name text NOT NULL,
  ciphertext bytea NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, name)
);

-- Indexes for performance & debugging
CREATE INDEX IF NOT EXISTS idx_actions_tenant_key ON actions(tenant_id, key);
CREATE INDEX IF NOT EXISTS idx_fact_resolvers_tenant_action ON fact_resolvers(tenant_id, action_key);
CREATE INDEX IF NOT EXISTS idx_fact_mappings_tenant_action ON fact_mappings(tenant_id, action_key);
CREATE INDEX IF NOT EXISTS idx_alternatives_tenant_action ON alternatives(tenant_id, action_key);
CREATE INDEX IF NOT EXISTS idx_decisions_tenant_action ON decisions(tenant_id, action_key);
CREATE INDEX IF NOT EXISTS idx_decisions_hash ON decisions(hash);
CREATE INDEX IF NOT EXISTS idx_executions_tenant_action ON executions(tenant_id, action_key);
CREATE INDEX IF NOT EXISTS idx_decisions_facts_gin ON decisions USING GIN (facts);
CREATE INDEX IF NOT EXISTS idx_decisions_inputs_gin ON decisions USING GIN (inputs);

-- RLS: enable and set select/update/insert/delete policies per tenant_id
ALTER TABLE actions ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_sets ENABLE ROW LEVEL SECURITY;
ALTER TABLE policy_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE fact_resolvers ENABLE ROW LEVEL SECURITY;
ALTER TABLE fact_mappings ENABLE ROW LEVEL SECURITY;
ALTER TABLE alternatives ENABLE ROW LEVEL SECURITY;
ALTER TABLE decisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE secrets ENABLE ROW LEVEL SECURITY;

DO $$ BEGIN
  -- actions
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'actions' AND policyname = 'tenant_isolation_actions') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_actions ON actions USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
  -- policy_sets
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'policy_sets' AND policyname = 'tenant_isolation_policy_sets') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_policy_sets ON policy_sets USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
  -- policy_versions
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'policy_versions' AND policyname = 'tenant_isolation_policy_versions') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_policy_versions ON policy_versions USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
  -- fact_resolvers
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'fact_resolvers' AND policyname = 'tenant_isolation_fact_resolvers') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_fact_resolvers ON fact_resolvers USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
  -- fact_mappings
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'fact_mappings' AND policyname = 'tenant_isolation_fact_mappings') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_fact_mappings ON fact_mappings USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
  -- alternatives
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'alternatives' AND policyname = 'tenant_isolation_alternatives') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_alternatives ON alternatives USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
  -- decisions
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'decisions' AND policyname = 'tenant_isolation_decisions') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_decisions ON decisions USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
  -- executions
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'executions' AND policyname = 'tenant_isolation_executions') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_executions ON executions USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
  -- secrets
  IF NOT EXISTS (SELECT 1 FROM pg_policies WHERE schemaname = current_schema() AND tablename = 'secrets' AND policyname = 'tenant_isolation_secrets') THEN
    EXECUTE 'CREATE POLICY tenant_isolation_secrets ON secrets USING (tenant_id = current_setting(''app.tenant_id'')::uuid) WITH CHECK (tenant_id = current_setting(''app.tenant_id'')::uuid)';
  END IF;
END $$;
