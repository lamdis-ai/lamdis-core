// cmd/connector-service/main.go
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

	"lamdis/internal/connector"
	"lamdis/pkg/config"
	"lamdis/pkg/connectors"
	"lamdis/pkg/db"
	"lamdis/pkg/logger"
	"lamdis/pkg/middleware"
	"lamdis/pkg/tenants"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.Env)

	var pool = db.MustConnect(cfg, log)

	var prov tenants.Provider
	if pool != nil {
		prov = tenants.NewPostgresProvider(pool, log)
		if err := tenants.EnsureSchema(context.Background(), pool); err != nil {
			log.Fatalw("schema", "err", err)
		}
		if err := tenants.SeedFromEnv(context.Background(), pool, os.Getenv("TENANT_SEED_JSON")); err != nil {
			log.Warnw("seed", "err", err)
		}
	} else {
		prov = tenants.NewMemoryProviderFromEnv(log)
	}

	reg := connectors.NewRegistry(pool)

	r := chi.NewRouter()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recover(log))
	r.Use(middleware.DebugWriteHeader())
	// Public services: allow cross-origin for development/tooling.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" { // echo origin to allow credentials if needed later
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
	r.Use(middleware.Tracing(cfg))
	r.Use(middleware.WithTenant(prov))
	// JWT auth (will noop if issuer/jwks not configured properly -> returns 500/401)
	r.Use(middleware.JWTAuth(cfg, prov, nil))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("pong")) })
	connector.DynamicRouter(r, cfg, prov, reg, pool)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	srv := &http.Server{Addr: cfg.HTTPAddr, Handler: r}
	go func() {
		log.Infow("connector-service listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalw("ListenAndServe", "err", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	fmt.Println("connector-service stopped")
}
