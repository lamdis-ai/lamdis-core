// pkg/middleware/tenant.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"lamdis/pkg/tenants"
)

type ctxTenantKey struct{}

func WithTenant(prov tenants.Provider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow health/metrics without tenant context
			switch r.URL.Path {
			case "/healthz", "/metrics":
				next.ServeHTTP(w, r)
				return
			}
			host := r.Host
			if i := strings.Index(host, ":"); i > 0 {
				host = host[:i]
			}
			// Fallback mapping: inside Docker different hostnames may be used to reach the same local service.
			// Seed typically uses 'localhost' as host; allow common local synonyms to resolve that tenant.
			tryHosts := []string{host}
			switch host {
			case "127.0.0.1", "host.docker.internal", "manifest", "connector", "policy", "admin-api":
				tryHosts = append(tryHosts, "localhost")
			}
			var t tenants.Tenant
			var err error
			for _, h := range tryHosts {
				t, err = prov.ResolveTenantByHost(r.Context(), h)
				if err == nil {
					break
				}
			}
			if err != nil {
				http.Error(w, "unknown tenant", http.StatusNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), ctxTenantKey{}, t)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func TenantFrom(ctx context.Context) tenants.Tenant {
	if v := ctx.Value(ctxTenantKey{}); v != nil {
		return v.(tenants.Tenant)
	}
	return tenants.Tenant{}
}
