// cmd/manifest-service/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"lamdis/internal/manifest"
	"lamdis/pkg/config"
	"lamdis/pkg/connectors"
	"lamdis/pkg/db"
	"lamdis/pkg/logger"
	"lamdis/pkg/middleware"
	"lamdis/pkg/tenants"
)

func main() {
	// 1. Load configuration & initialize structured logger.
	cfg := config.Load()
	appLog := logger.New(cfg.Env)

	// 2. Attempt database connection (optional depending on config).
	var dbPool = db.MustConnect(cfg, appLog)

	// 3. Initialize tenant provider (DB-backed if pool present, otherwise in‑memory).
	var tenantProvider tenants.Provider
	if dbPool != nil {
		tenantProvider = tenants.NewPostgresProvider(dbPool, appLog)
		_ = tenants.EnsureSchema(context.Background(), dbPool)
		_ = tenants.SeedFromEnv(context.Background(), dbPool, os.Getenv("TENANT_SEED_JSON"))
	} else {
		tenantProvider = tenants.NewMemoryProviderFromEnv(appLog)
	}

	// Always create registry (dbPool may be nil -> dev fallback operations)
	reg := connectors.NewRegistry(dbPool)

	// 4. Build HTTP router and register middlewares.
	router := chi.NewRouter()
	router.Use(middleware.RequestID())
	router.Use(middleware.Recover(appLog))
	// Optional diagnostic middleware for double WriteHeader.
	router.Use(middleware.DebugWriteHeader())
	// Permissive CORS for well-known + all endpoints (public discovery).
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Tenant-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	router.Use(middleware.Tracing(cfg))
	router.Use(middleware.WithTenant(tenantProvider))
	// JWT middleware (mostly no-op for public manifest but future-proof)
	router.Use(middleware.JWTAuth(cfg, tenantProvider, nil))

	// 5. Basic operational endpoints.
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	router.Get("/ping", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("pong")) })
	router.Get("/metrics", promhttp.Handler().ServeHTTP)

	// 6. Domain (manifest) routes.
	manifest.RegisterRoutes(router, cfg, appLog, tenantProvider, reg)

	// 7. Configure and start HTTP server asynchronously.
	httpServer := &http.Server{Addr: cfg.ManifestAddr, Handler: router}
	go func() {
		appLog.Infow("manifest-service listening", "addr", cfg.ManifestAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			appLog.Fatalw("ListenAndServe", "err", err)
		}
	}()

	// 8. Wait for termination signal (SIGINT/SIGTERM) to begin graceful shutdown.
	stopCh := make(chan os.Signal, 1)
	signal.Notify(stopCh, os.Interrupt, syscall.SIGTERM)
	<-stopCh

	// 9. Graceful shutdown with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(ctx)
	fmt.Println("manifest-service stopped")
}
