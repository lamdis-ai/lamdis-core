package tenants

import (
	"context"
)

type Provider interface {
	// Resolve tenant from incoming host (or header).
	ResolveTenantByHost(ctx context.Context, host string) (Tenant, error)
	// Optional: resolve from slug/id
	ResolveTenantByID(ctx context.Context, id string) (Tenant, error)
	// Return connector credentials per tenant/kind
	GetConnectorCreds(ctx context.Context, tenantID, kind string) (ConnectorCreds, error)
	// List connector kinds configured for the tenant
	ListTenantConnectorKinds(ctx context.Context, tenantID string) ([]string, error)
	// (Future) maybe: UpdateTenant / ListTenants for admin APIs
}
