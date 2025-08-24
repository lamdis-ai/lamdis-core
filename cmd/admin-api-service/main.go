package main

import (
	"net/http"
	"os"
	"strings"

	"lamdis/internal/adminapi"
	"lamdis/pkg/config"
	pdb "lamdis/pkg/db"
	"lamdis/pkg/logger"
)

func main() {
	cfg := config.Load()
	log := logger.New(cfg.Env)
	defer log.Sync()

	bind := os.Getenv("ADMIN_HTTP_ADDR")
	if strings.TrimSpace(bind) == "" {
		bind = ":8082"
	}

	app := adminapi.New(
		log,
		pdb.MustConnect(cfg, log),
		adminapi.Config{
			HTTPAddr:      bind,
			OIDCIssuer:    os.Getenv("ADMIN_OIDC_ISSUER"),
			OIDCAudience:  os.Getenv("ADMIN_OIDC_AUDIENCE"),
			JWKSURL:       os.Getenv("ADMIN_JWKS_URL"),
			RegistryDir:   os.Getenv("REGISTRY_DIR"),
			EncryptionKey: os.Getenv("ENCRYPTION_KEY"),
		},
	)

	log.Infof("admin-api listening at %s", bind)
	if err := http.ListenAndServe(bind, app.Handler()); err != nil {
		log.Fatalf("listen: %v", err)
	}
}
