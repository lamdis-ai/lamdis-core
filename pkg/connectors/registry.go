package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectorRecord represents a connector definition stored in the DB.
// Two categories:
// - builtin: implemented in code (referenced by builtin_kind)
// - custom: dynamic HTTP proxy / template (future extension)
// Auth configuration kept generic for now.
type ConnectorRecord struct {
	ID          string
	TenantID    string
	Kind        string // logical unique name per tenant
	BuiltinKind string // if non-empty, name of builtin implementation (e.g. "shopify")
	ConfigJSON  map[string]any
	SecretJSON  map[string]any
	Auth        map[string]any // placeholder for auth strategy (oauth2, api_key etc.)
}

type OperationMeta struct {
	Method  string
	Path    string
	Summary string
	Scopes  []string
	Params  []map[string]any // parameter schema for OS-level agents
}

type Builtin interface {
	Name() string
	Operations() []OperationMeta
}

// Factory returns a builtin connector implementation given config+secrets (as maps) if needed.
type Factory func(cfg, secret map[string]any) (Builtin, error)

type operationRow struct {
	Method  string
	Path    string
	Summary string
	Scopes  []string
	Params  []map[string]any
	BaseURL *string // optional upstream base URL for passthrough
	AuthRef *string // optional reference to tenant_auth_configs.id for auth injection
	Kind    *string // connector kind namespace
}

type cachedTenant struct {
	loadedAt   time.Time
	operations []operationRow
}

// Registry holds builtin factories and performs DB lookups for tenant connectors.
type Registry struct {
	pool      *pgxpool.Pool
	factories map[string]Factory
	mu        sync.RWMutex
	byTenant  map[string]cachedTenant
	ttl       time.Duration
}

func NewRegistry(pool *pgxpool.Pool) *Registry {
	return &Registry{pool: pool, factories: map[string]Factory{}, byTenant: map[string]cachedTenant{}, ttl: 30 * time.Second}
}

func (r *Registry) RegisterFactory(kind string, f Factory) { r.factories[kind] = f }

// ListTenantConnectors loads connector records for a tenant (both builtin and custom).
func (r *Registry) ListTenantConnectors(ctx context.Context, tenantID string) ([]ConnectorRecord, error) {
	rows, err := r.pool.Query(ctx, `SELECT kind, config, secret FROM tenant_connectors WHERE tenant_id=$1`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConnectorRecord
	for rows.Next() {
		var kind string
		var cfgRaw, secRaw []byte
		_ = rows.Scan(&kind, &cfgRaw, &secRaw)
		var cfgMap, secMap map[string]any
		_ = json.Unmarshal(cfgRaw, &cfgMap)
		_ = json.Unmarshal(secRaw, &secMap)
		out = append(out, ConnectorRecord{TenantID: tenantID, Kind: kind, BuiltinKind: kind, ConfigJSON: cfgMap, SecretJSON: secMap})
	}
	return out, nil
}

// InstantiateBuiltin turns a record referencing a builtin into an implementation.
func (r *Registry) InstantiateBuiltin(rec ConnectorRecord) (Builtin, error) {
	if rec.BuiltinKind == "" {
		return nil, errors.New("not a builtin")
	}
	f, ok := r.factories[rec.BuiltinKind]
	if !ok {
		return nil, fmt.Errorf("no factory for builtin %s", rec.BuiltinKind)
	}
	return f(rec.ConfigJSON, rec.SecretJSON)
}

// LoadOperations returns dynamic operations for a tenant (cached).
func (r *Registry) LoadOperations(ctx context.Context, tenantID string) ([]operationRow, error) {
	// Dev fallback: if no DB configured, surface static sample operations so OpenAPI/manifest work.
	if r.pool == nil {
		// simple in-memory cache per tenant
		r.mu.RLock()
		c, ok := r.byTenant[tenantID]
		if ok && time.Since(c.loadedAt) < r.ttl {
			r.mu.RUnlock()
			return c.operations, nil
		}
		r.mu.RUnlock()
		kind := "sample"
		ops := []operationRow{
			{Method: "GET", Path: "/v1/dev/ping", Summary: "Ping test endpoint", Scopes: []string{"dev:read"}, Kind: &kind},
			{Method: "POST", Path: "/v1/dev/echo", Summary: "Echo posted payload", Scopes: []string{"dev:write"}, Kind: &kind},
			{Method: "GET", Path: "/v1/dev/orders/{id}", Summary: "Fetch mock order by id", Scopes: []string{"order:read"}, Kind: &kind},
		}
		r.mu.Lock()
		r.byTenant[tenantID] = cachedTenant{loadedAt: time.Now(), operations: ops}
		r.mu.Unlock()
		return ops, nil
	}
	r.mu.RLock()
	c, ok := r.byTenant[tenantID]
	if ok && time.Since(c.loadedAt) < r.ttl {
		r.mu.RUnlock()
		return c.operations, nil
	}
	r.mu.RUnlock()
	rows, err := r.pool.Query(ctx, `
		SELECT o.method, o.path, o.summary, COALESCE(o.scopes, ARRAY[]::text[]), COALESCE(o.params,'[]'::jsonb), d.base_url, d.auth_ref, d.kind
		FROM connector_operations o
		JOIN connector_definitions d ON o.connector_id=d.id
		JOIN tenant_connectors tc ON tc.connector_id=d.id::text AND tc.tenant_id=$1 AND COALESCE(tc.enabled,false)=true
		WHERE d.tenant_id=$1
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ops []operationRow
	for rows.Next() {
		var or operationRow
		var paramsRaw []byte
		_ = rows.Scan(&or.Method, &or.Path, &or.Summary, &or.Scopes, &paramsRaw, &or.BaseURL, &or.AuthRef, &or.Kind)
		if len(paramsRaw) > 0 {
			_ = json.Unmarshal(paramsRaw, &or.Params)
		}
		ops = append(ops, or)
	}
	r.mu.Lock()
	r.byTenant[tenantID] = cachedTenant{loadedAt: time.Now(), operations: ops}
	r.mu.Unlock()
	return ops, nil
}
