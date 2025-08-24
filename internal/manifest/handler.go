// internal/manifest/handler.go
package manifest

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"lamdis/pkg/config"
	"lamdis/pkg/connectors"
	"lamdis/pkg/middleware"
	"lamdis/pkg/tenants"
)

func RegisterRoutes(r chi.Router, cfg config.Config, log *zap.SugaredLogger, prov tenants.Provider, reg *connectors.Registry) {
	// CORS preflight for public well-known endpoint
	r.Options("/.well-known/ai-actions", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusNoContent)
	})
	r.Get("/.well-known/ai-actions", func(w http.ResponseWriter, req *http.Request) {
		t := middleware.TenantFrom(req.Context())
		m := BuildManifest(req.Context(), cfg, t, reg)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		_ = json.NewEncoder(w).Encode(m)
	})
}
