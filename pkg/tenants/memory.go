// pkg/tenants/memory.go
package tenants

import (
	"context"
	"encoding/json"
	"errors"
	"os"

	"go.uber.org/zap"
)

type memProvider struct {
	log    *zap.SugaredLogger
	byHost map[string]Tenant
	creds  map[string]ConnectorCreds // key: tenantID+":"+kind
}

func NewMemoryProviderFromEnv(log *zap.SugaredLogger) Provider {
	p := &memProvider{log: log, byHost: map[string]Tenant{}, creds: map[string]ConnectorCreds{}}
	seed := os.Getenv("TENANT_SEED_JSON")
	if seed != "" {
		var entries []struct {
			ID, Slug, Host, OAuthIssuer, JWKSURL, BasePublicURL string
			ShopifyDomain, ShopifyToken                         string
		}
		_ = json.Unmarshal([]byte(seed), &entries)
		for _, e := range entries {
			p.byHost[e.Host] = Tenant{
				ID: e.ID, Slug: e.Slug, Host: e.Host,
				OAuthIssuer: e.OAuthIssuer, JWKSURL: e.JWKSURL, BasePublicURL: e.BasePublicURL,
				AuthMode: "byoidc", AccountClaim: "sub", AcceptedAudiences: nil,
				MachineAllowedScopes: []string{}, RequiredACRByAction: map[string]string{},
			}
			p.creds[e.ID+":shopify"] = ConnectorCreds{ShopifyDomain: e.ShopifyDomain, ShopifyToken: e.ShopifyToken}
		}
	} else {
		// sensible localhost defaults for both services and common variants
		dev := Tenant{
			ID: "00000000-0000-0000-0000-000000000001", Slug: "dev",
			OAuthIssuer: os.Getenv("OIDC_ISSUER"), JWKSURL: os.Getenv("JWKS_URL"), BasePublicURL: os.Getenv("BASE_PUBLIC_URL"),
			AuthMode: "byoidc", AccountClaim: "sub", MachineAllowedScopes: []string{}, RequiredACRByAction: map[string]string{},
		}
		for _, h := range []string{
			"localhost:8081", "127.0.0.1:8081", "host.docker.internal:8081", "manifest:8081",
			"localhost:8080", "127.0.0.1:8080", "host.docker.internal:8080", "connector:8080",
		} {
			dd := dev
			dd.Host = h
			p.byHost[h] = dd
		}
	}
	return p
}

func (m *memProvider) ResolveTenantByHost(ctx context.Context, host string) (Tenant, error) {
	if t, ok := m.byHost[host]; ok {
		return t, nil
	}
	return Tenant{}, errors.New("tenant not found")
}
func (m *memProvider) ResolveTenantByID(ctx context.Context, id string) (Tenant, error) {
	for _, t := range m.byHost {
		if t.ID == id {
			return t, nil
		}
	}
	return Tenant{}, errors.New("tenant not found")
}
func (m *memProvider) GetConnectorCreds(ctx context.Context, tenantID, kind string) (ConnectorCreds, error) {
	if c, ok := m.creds[tenantID+":"+kind]; ok {
		return c, nil
	}
	return ConnectorCreds{}, errors.New("connector creds not found")
}
func (m *memProvider) ListTenantConnectorKinds(ctx context.Context, tenantID string) ([]string, error) {
	var kinds []string
	for k := range m.creds {
		// key pattern tenantID:kind
		if len(k) > len(tenantID)+1 && k[:len(tenantID)] == tenantID {
			parts := k[len(tenantID)+1:]
			kinds = append(kinds, parts)
		}
	}
	return kinds, nil
}
