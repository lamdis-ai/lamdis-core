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

	"lamdis/internal/policy"
	"lamdis/pkg/config"
	"lamdis/pkg/db"
	"lamdis/pkg/logger"
	"lamdis/pkg/middleware"
	"lamdis/pkg/tenants"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.Env)

	pool := db.MustConnect(cfg, log)

	var prov tenants.Provider
	if pool != nil {
		prov = tenants.NewPostgresProvider(pool, log)
		_ = tenants.EnsureSchema(context.Background(), pool)
		_ = tenants.SeedFromEnv(context.Background(), pool, os.Getenv("TENANT_SEED_JSON"))
	} else {
		prov = tenants.NewMemoryProviderFromEnv(log)
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID())
	r.Use(middleware.Recover(log))
	r.Use(middleware.DebugWriteHeader())
	r.Use(middleware.Tracing(cfg))
	r.Use(middleware.WithTenant(prov))
	r.Use(middleware.JWTAuth(cfg, prov, nil))

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	policy.RegisterHTTP(r, pool)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	addr := cfg.ManifestAddr // reuse manifest addr or introduce POLICY_ADDR via env in future
	srv := &http.Server{Addr: addr, Handler: r}
	go func() {
		log.Infow("policy-service listening", "addr", addr)
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
	fmt.Println("policy-service stopped")
}
