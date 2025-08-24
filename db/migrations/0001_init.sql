CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS tenants (
  id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
  slug TEXT UNIQUE NOT NULL,
  host TEXT UNIQUE NOT NULL,
  auth_mode TEXT NOT NULL DEFAULT 'byoidc',
  oauth_issuer TEXT NOT NULL,
  accepted_audiences TEXT[] NOT NULL DEFAULT ARRAY['https://lamdis.ai'],
  client_id_user TEXT,
  client_id_machine TEXT,
  account_claim TEXT NOT NULL DEFAULT 'sub',
  dpop_required BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS tenant_connectors (
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  connector_id TEXT NOT NULL,
  enabled BOOLEAN NOT NULL DEFAULT FALSE,
  secrets_encrypted BYTEA,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (tenant_id, connector_id)
);

-- Global registry of connectors
CREATE TABLE IF NOT EXISTS connectors (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  display_name TEXT NOT NULL,
  capabilities JSONB NOT NULL DEFAULT '[]'::JSONB,
  requirements JSONB NOT NULL DEFAULT '{"secrets":[],"webhooks":[]}'::JSONB,
  audit_mode TEXT NOT NULL DEFAULT 'none',
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS policies (
  tenant_id UUID PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  machine_allowed_scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
  step_up JSONB NOT NULL DEFAULT '{}'::JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS audit_log (
  id BIGSERIAL PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  rail TEXT,
  order_id TEXT,
  actor_sub TEXT,
  mode TEXT,
  result_code INT,
  request_id TEXT,
  ts TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Usage events per tenant for action executions (canonical and dynamic)
CREATE TABLE IF NOT EXISTS usage_events (
  id BIGSERIAL PRIMARY KEY,
  tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  action_id TEXT,
  method TEXT,
  path TEXT,
  mode TEXT,
  rail TEXT,
  actor_sub TEXT,
  request_id TEXT,
  status_code INT,
  duration_ms INT,
  started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  finished_at TIMESTAMPTZ
);

CREATE OR REPLACE FUNCTION touch_updated_at() RETURNS TRIGGER AS $$
BEGIN NEW.updated_at = NOW(); RETURN NEW; END; $$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS tenants_touch ON tenants;
CREATE TRIGGER tenants_touch BEFORE UPDATE ON tenants FOR EACH ROW EXECUTE PROCEDURE touch_updated_at();

DROP TRIGGER IF EXISTS tenant_connectors_touch ON tenant_connectors;
CREATE TRIGGER tenant_connectors_touch BEFORE UPDATE ON tenant_connectors FOR EACH ROW EXECUTE PROCEDURE touch_updated_at();

DROP TRIGGER IF EXISTS connectors_touch ON connectors;
CREATE TRIGGER connectors_touch BEFORE UPDATE ON connectors FOR EACH ROW EXECUTE PROCEDURE touch_updated_at();

DROP TRIGGER IF EXISTS policies_touch ON policies;
CREATE TRIGGER policies_touch BEFORE UPDATE ON policies FOR EACH ROW EXECUTE PROCEDURE touch_updated_at();
