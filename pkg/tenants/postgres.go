// pkg/tenants/postgres.go
package tenants

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// pgProvider implements Provider backed by PostgreSQL.
type pgProvider struct {
	dbPool *pgxpool.Pool      // Connection pool to PostgreSQL
	log    *zap.SugaredLogger // Logger for diagnostic output
}

// NewPostgresProvider constructs a PostgreSQL-backed tenant provider.
func NewPostgresProvider(dbPool *pgxpool.Pool, log *zap.SugaredLogger) Provider {
	return &pgProvider{dbPool: dbPool, log: log}
}

// EnsureSchema creates required tables if they do not already exist plus new auth columns.
// Safe to call repeatedly (idempotent).
func EnsureSchema(ctx context.Context, dbPool *pgxpool.Pool) error {
	_, err := dbPool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS tenants (
  id uuid PRIMARY KEY,
  slug text UNIQUE,
  host text UNIQUE,
  oauth_issuer text,
  jwks_url text,
  base_public_url text,
  auth_mode text DEFAULT 'byoidc',
  discovery_url text,
  accepted_audiences text[] DEFAULT '{}',
  account_claim text DEFAULT 'sub',
  machine_allowed_scopes text[] DEFAULT '{}',
  required_acr_by_action jsonb DEFAULT '{}'::jsonb,
  dpop_required boolean DEFAULT false
);
CREATE TABLE IF NOT EXISTS tenant_connectors (
	tenant_id uuid REFERENCES tenants(id) ON DELETE CASCADE,
	connector_id text,
	enabled boolean NOT NULL DEFAULT false,
	secrets_encrypted bytea,
	updated_at timestamptz NOT NULL DEFAULT NOW(),
	PRIMARY KEY (tenant_id, connector_id)
);
CREATE TABLE IF NOT EXISTS connector_definitions (
  id uuid PRIMARY KEY,
  tenant_id uuid REFERENCES tenants(id) ON DELETE CASCADE,
  kind text,
  builtin_kind text,
  auth jsonb,
  config jsonb,
  secret jsonb,
  UNIQUE(tenant_id, kind)
);
CREATE TABLE IF NOT EXISTS connector_operations (
  id uuid PRIMARY KEY,
  connector_id uuid REFERENCES connector_definitions(id) ON DELETE CASCADE,
  method text,
  path text,
  summary text,
  scopes text[] DEFAULT '{}',
  request_schema jsonb,
  response_schema jsonb
);
CREATE TABLE IF NOT EXISTS usage_events (
	id BIGSERIAL PRIMARY KEY,
	tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
	action_id text,
	method text,
	path text,
	mode text,
	rail text,
	actor_sub text,
	request_id text,
	status_code int,
	duration_ms int,
	started_at timestamptz NOT NULL DEFAULT NOW(),
	finished_at timestamptz
);
-- Backfill / ensure new columns exist (for upgrades)
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS auth_mode text DEFAULT 'byoidc';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS discovery_url text;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS accepted_audiences text[] DEFAULT '{}';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS account_claim text DEFAULT 'sub';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS machine_allowed_scopes text[] DEFAULT '{}';
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS required_acr_by_action jsonb DEFAULT '{}'::jsonb;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS dpop_required boolean DEFAULT false;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS jwks_url text;
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS base_public_url text;
-- Ensure timestamp columns exist to satisfy update triggers
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT NOW();
ALTER TABLE tenants ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT NOW();
-- Marketplace runtime columns (may be added after initial versions)
ALTER TABLE connector_definitions ADD COLUMN IF NOT EXISTS base_url text;
ALTER TABLE connector_definitions ADD COLUMN IF NOT EXISTS auth_ref uuid;
ALTER TABLE connector_definitions ADD COLUMN IF NOT EXISTS title text;
ALTER TABLE connector_definitions ADD COLUMN IF NOT EXISTS summary text;
ALTER TABLE connector_operations ADD COLUMN IF NOT EXISTS request_tmpl jsonb;
ALTER TABLE connector_operations ADD COLUMN IF NOT EXISTS params jsonb;
-- Align tenant_connectors for new marketplace usage
ALTER TABLE tenant_connectors ADD COLUMN IF NOT EXISTS connector_id text;
ALTER TABLE tenant_connectors ADD COLUMN IF NOT EXISTS enabled boolean DEFAULT false;
ALTER TABLE tenant_connectors ADD COLUMN IF NOT EXISTS secrets_encrypted bytea;
ALTER TABLE tenant_connectors ADD COLUMN IF NOT EXISTS updated_at timestamptz DEFAULT NOW();
-- Backfill connector_id from legacy 'kind' if present
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
		EXECUTE 'UPDATE tenant_connectors SET connector_id = COALESCE(connector_id, kind) WHERE connector_id IS NULL';
		EXECUTE 'UPDATE tenant_connectors SET kind = connector_id WHERE kind IS NULL AND connector_id IS NOT NULL';
	END IF;
END $$;
-- Ensure a unique index for (tenant_id, connector_id) to support ON CONFLICT
CREATE UNIQUE INDEX IF NOT EXISTS tenant_connectors_tenant_connector_idx ON tenant_connectors(tenant_id, connector_id);
`)
	return err
}

// SeedFromEnv ingests initial tenant + connector data.
// jsonSeed format (TENANT_SEED_JSON):
// [
//
//	{
//	  "id":"...","slug":"...","host":"...","oauth_issuer":"...","jwks_url":"...","base_public_url":"...",
//	  "shopify_domain":"...","shopify_token":"..."
//	}
//
// ]
func SeedFromEnv(ctx context.Context, dbPool *pgxpool.Pool, jsonSeed string) error {
	if jsonSeed == "" {
		return nil
	}
	var entries []struct {
		ID, Slug, Host, OAuthIssuer, JWKSURL, BasePublicURL string
		ShopifyDomain, ShopifyToken                         string
	}
	if err := json.Unmarshal([]byte(jsonSeed), &entries); err != nil {
		return err
	}
	for _, entry := range entries {
		_, _ = dbPool.Exec(ctx, `INSERT INTO tenants(id,slug,host,oauth_issuer,jwks_url,base_public_url)
		  VALUES ($1,$2,$3,$4,$5,$6)
		  ON CONFLICT (id) DO UPDATE SET slug=EXCLUDED.slug,host=EXCLUDED.host,oauth_issuer=EXCLUDED.oauth_issuer,jwks_url=EXCLUDED.jwks_url,base_public_url=EXCLUDED.base_public_url`,
			entry.ID, entry.Slug, entry.Host, entry.OAuthIssuer, entry.JWKSURL, entry.BasePublicURL)
		// Write dynamic definition (replaces legacy tenant_connectors)
		// Dynamic definition (upsert)
		var defID uuid.UUID = uuid.New()
		row := dbPool.QueryRow(ctx, `SELECT id FROM connector_definitions WHERE tenant_id=$1 AND kind='shopify'`, entry.ID)
		var existing uuid.UUID
		if err := row.Scan(&existing); err == nil {
			defID = existing
		} else {
			_, _ = dbPool.Exec(ctx, `INSERT INTO connector_definitions(id,tenant_id,kind,builtin_kind,auth,config,secret)
			 VALUES ($1,$2,'shopify','shopify',$3,$4,$5)`, defID, entry.ID, map[string]any{"type": "none"}, map[string]any{"domain": entry.ShopifyDomain}, map[string]any{"token": entry.ShopifyToken})
		}
		// Ensure operations for shopify
		// checkout-link
		_, _ = dbPool.Exec(ctx, `INSERT INTO connector_operations(id,connector_id,method,path,summary,scopes)
		 VALUES ($1,$2,'POST','/v1/checkout-link','Create checkout link',ARRAY['order:write']) ON CONFLICT DO NOTHING`, uuid.New(), defID)
		// refund
		_, _ = dbPool.Exec(ctx, `INSERT INTO connector_operations(id,connector_id,method,path,summary,scopes)
		 VALUES ($1,$2,'POST','/v1/orders/{id}/refund','Refund order',ARRAY['refund:write']) ON CONFLICT DO NOTHING`, uuid.New(), defID)
	}
	return nil
}

// ResolveTenantByHost fetches a tenant using its host value.
func (p *pgProvider) ResolveTenantByHost(ctx context.Context, host string) (Tenant, error) {
	row := p.dbPool.QueryRow(ctx, `SELECT id,slug,host,oauth_issuer,COALESCE(jwks_url,''),COALESCE(base_public_url,''),auth_mode,COALESCE(discovery_url,''),accepted_audiences,COALESCE(account_claim,'sub'),machine_allowed_scopes,required_acr_by_action,dpop_required FROM tenants WHERE host=$1`, host)
	var t Tenant
	var accepted, machine []string
	var requiredJSON []byte
	if err := row.Scan(&t.ID, &t.Slug, &t.Host, &t.OAuthIssuer, &t.JWKSURL, &t.BasePublicURL, &t.AuthMode, &t.DiscoveryURL, &accepted, &t.AccountClaim, &machine, &requiredJSON, &t.DPoPRequired); err != nil {
		return Tenant{}, errors.New("tenant not found")
	}
	t.AcceptedAudiences = accepted
	t.MachineAllowedScopes = machine
	if len(requiredJSON) > 0 {
		_ = json.Unmarshal(requiredJSON, &t.RequiredACRByAction)
	}
	if t.RequiredACRByAction == nil {
		t.RequiredACRByAction = map[string]string{}
	}
	return t, nil
}

// ResolveTenantByID fetches a tenant by its UUID.
func (p *pgProvider) ResolveTenantByID(ctx context.Context, id string) (Tenant, error) {
	row := p.dbPool.QueryRow(ctx, `SELECT id,slug,host,oauth_issuer,COALESCE(jwks_url,''),COALESCE(base_public_url,''),auth_mode,COALESCE(discovery_url,''),accepted_audiences,COALESCE(account_claim,'sub'),machine_allowed_scopes,required_acr_by_action,dpop_required FROM tenants WHERE id=$1`, id)
	var t Tenant
	var accepted, machine []string
	var requiredJSON []byte
	if err := row.Scan(&t.ID, &t.Slug, &t.Host, &t.OAuthIssuer, &t.JWKSURL, &t.BasePublicURL, &t.AuthMode, &t.DiscoveryURL, &accepted, &t.AccountClaim, &machine, &requiredJSON, &t.DPoPRequired); err != nil {
		return Tenant{}, errors.New("tenant not found")
	}
	t.AcceptedAudiences = accepted
	t.MachineAllowedScopes = machine
	if len(requiredJSON) > 0 {
		_ = json.Unmarshal(requiredJSON, &t.RequiredACRByAction)
	}
	if t.RequiredACRByAction == nil {
		t.RequiredACRByAction = map[string]string{}
	}
	return t, nil
}

// GetConnectorCreds returns connector-specific credentials (currently Shopify).
func (p *pgProvider) GetConnectorCreds(ctx context.Context, tenantID, kind string) (ConnectorCreds, error) {
	row := p.dbPool.QueryRow(ctx, `SELECT COALESCE(config->>'domain',''), COALESCE(secret->>'token','') FROM connector_definitions WHERE tenant_id=$1 AND kind=$2`, tenantID, kind)
	var creds ConnectorCreds
	if err := row.Scan(&creds.ShopifyDomain, &creds.ShopifyToken); err != nil {
		return ConnectorCreds{}, errors.New("connector creds not found")
	}
	return creds, nil
}

// ListTenantConnectorKinds returns the kinds of connectors available to a tenant.
func (p *pgProvider) ListTenantConnectorKinds(ctx context.Context, tenantID string) ([]string, error) {
	rows, err := p.dbPool.Query(ctx, `SELECT kind FROM connector_definitions WHERE tenant_id=$1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var kinds []string
	for rows.Next() {
		var k string
		_ = rows.Scan(&k)
		kinds = append(kinds, k)
	}
	return kinds, nil
}
