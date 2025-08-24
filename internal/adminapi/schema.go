package adminapi

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ensureTenantMarketplaceSchema creates tenant-scoped auth table and augments connector tables for custom connectors.
func ensureTenantMarketplaceSchema(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
-- Ensure tenants has columns used by admin-api OIDC config
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS client_id_user TEXT;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS client_id_machine TEXT;
CREATE TABLE IF NOT EXISTS tenant_auth_configs (
	id UUID PRIMARY KEY,
	tenant_id UUID NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	type TEXT NOT NULL, -- api_key | bearer | oauth2_client
	config JSONB NOT NULL DEFAULT '{}'::JSONB,
	secrets_encrypted BYTEA,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE(tenant_id, name)
);
ALTER TABLE connector_definitions ADD COLUMN IF NOT EXISTS base_url TEXT;
ALTER TABLE connector_definitions ADD COLUMN IF NOT EXISTS auth_ref UUID;
ALTER TABLE connector_definitions ADD COLUMN IF NOT EXISTS title TEXT;
ALTER TABLE connector_definitions ADD COLUMN IF NOT EXISTS summary TEXT;
ALTER TABLE connector_operations ADD COLUMN IF NOT EXISTS request_tmpl JSONB;
ALTER TABLE connector_operations ADD COLUMN IF NOT EXISTS params JSONB;
ALTER TABLE connector_operations ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;
-- Align tenant_connectors for marketplace usage
ALTER TABLE tenant_connectors ADD COLUMN IF NOT EXISTS connector_id TEXT;
ALTER TABLE tenant_connectors ADD COLUMN IF NOT EXISTS enabled BOOLEAN DEFAULT false;
ALTER TABLE tenant_connectors ADD COLUMN IF NOT EXISTS secrets_encrypted BYTEA;
ALTER TABLE tenant_connectors ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();
-- Ensure tenants timestamp columns exist for triggers defined in base migrations
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
-- Backfill connector_id from legacy 'kind' where needed
DO $$
DECLARE in_pk BOOLEAN;
BEGIN
	IF EXISTS (
		SELECT 1 FROM information_schema.columns WHERE table_name='tenant_connectors' AND column_name='kind'
	) THEN
		SELECT EXISTS (
			SELECT 1
			FROM pg_index i
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			WHERE i.indrelid = 'tenant_connectors'::regclass AND i.indisprimary AND a.attname = 'kind'
		) INTO in_pk;
		IF NOT in_pk THEN
			EXECUTE 'ALTER TABLE tenant_connectors ALTER COLUMN kind DROP NOT NULL';
		END IF;
		-- Keep ids in sync either way
		EXECUTE 'UPDATE tenant_connectors SET connector_id = COALESCE(connector_id, kind) WHERE connector_id IS NULL';
		EXECUTE 'UPDATE tenant_connectors SET kind = connector_id WHERE kind IS NULL AND connector_id IS NOT NULL';
	END IF;
END $$;
CREATE UNIQUE INDEX IF NOT EXISTS tenant_connectors_tenant_connector_idx ON tenant_connectors(tenant_id, connector_id);
`)
	return err
}

// ensureConnectorSchema creates the connectors table if it doesn't exist.
func ensureConnectorSchema(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
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
				ALTER TABLE connectors ADD COLUMN IF NOT EXISTS category TEXT;
				ALTER TABLE connectors ADD COLUMN IF NOT EXISTS tags TEXT[] DEFAULT '{}';
		`)
	return err
}
