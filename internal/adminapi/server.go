package adminapi

import (
	"net/http"
	"os"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

// Handler builds the HTTP handler with routes and middleware.
func (a *App) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Logger, chimw.Recoverer)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	allowed := []string{"http://localhost:3001"}
	if v := strings.TrimSpace(os.Getenv("ADMIN_CORS_ORIGINS")); v != "" {
		parts := strings.Split(v, ",")
		tmp := make([]string, 0, len(parts))
		for _, p := range parts {
			if s := strings.TrimSpace(p); s != "" {
				tmp = append(tmp, s)
			}
		}
		if len(tmp) > 0 {
			allowed = tmp
		}
	}

	r.Route("/admin", func(ar chi.Router) {
		ar.Use(cors(allowed))
		ar.Use(a.adminAuth)
		ar.Get("/tenant/self", a.getTenantSelf)
		ar.Put("/tenant/oidc", a.putTenantOIDC)
		ar.Get("/registry/connectors", a.getRegistry)
		ar.Put("/registry/connectors/{id}", a.upsertConnector)
		ar.Put("/tenant/connectors/{connectorId}", a.putTenantConnector)
		ar.Put("/tenant/policies", a.putPolicies)
		ar.Get("/audit", a.getAudit)
		ar.Get("/decisions", a.listDecisions)
		// Marketplace endpoints
		ar.Get("/auth", a.listAuth)
		ar.Post("/auth", a.createAuth)
		ar.Put("/auth/{id}", a.updateAuth)
		ar.Delete("/auth/{id}", a.deleteAuth)
		ar.Get("/tenant/connectors", a.listTenantConnectors)
		ar.Get("/tenant/configured-connectors", a.listConfiguredConnectors)
		ar.Post("/tenant/connectors", a.createCustomConnector)
		ar.Get("/tenant/custom-connectors/{id}", a.getCustomConnector)
		ar.Put("/tenant/custom-connectors/{id}", a.updateCustomConnector)
		// Enable/disable a specific operation (action) on a custom connector
		ar.Put("/tenant/custom-connectors/{id}/actions/{opId}", a.putConnectorAction)
		// List enabled actions for a connector (optimized UI)
		ar.Get("/tenant/custom-connectors/{id}/actions", a.listConnectorActions)
		ar.Get("/usage/summary", a.getUsageSummary)
		// Policies admin
		ar.Get("/policies/versions", a.listPolicyVersions)
		ar.Get("/policies/versions/{version}", a.getPolicyVersion)
		ar.Get("/policies/active", a.getActivePolicy)
		ar.Post("/policies/versions", a.createPolicyVersion)
		ar.Post("/policies/versions/{version}/publish", a.publishPolicyVersion)
		ar.Post("/policies/compile", a.compilePolicy)
		ar.Post("/policies/dry-run", a.dryRunPolicy)
		ar.Post("/policies/test-execute", a.testExecutePolicy)
		// Actions/resolvers/mappings admin
		ar.Get("/actions/coverage", a.getActionsCoverage)
		ar.Get("/actions/summary", a.getActionsSummary)
		ar.Get("/actions", a.listActions)
		ar.Put("/actions/{key}", a.upsertAction)
		ar.Get("/actions/{key}/resolvers", a.listResolvers)
		ar.Put("/actions/{key}/resolvers/{name}", a.upsertResolver)
		ar.Delete("/actions/{key}/resolvers/{name}", a.deleteResolver)
		ar.Get("/actions/{key}/mappings", a.listMappings)
		ar.Put("/actions/{key}/mappings/{name}", a.upsertMapping)
		ar.Delete("/actions/{key}/mappings/{name}", a.deleteMapping)
	})

	return r
}
