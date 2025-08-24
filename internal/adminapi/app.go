package adminapi

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"go.uber.org/zap"
)

// Config holds admin-api specific configuration.
type Config struct {
	HTTPAddr      string
	OIDCIssuer    string
	OIDCAudience  string
	JWKSURL       string
	RegistryDir   string
	EncryptionKey string
}

// App is the admin-api application container.
// Handlers and middleware have methods on this type.
//
// Keep it lean: shared deps and config only.
// Request-scoped work should use context.
//
// Log is a sugared zap logger; db is a pgx pool.
// OIDC fields configure admin bearer validation.
// encrypterKey enables optional symmetric secrets encryption.
//
// Note: additional deps (redis, http clients) can be added later.
type App struct {
	log          *zap.SugaredLogger
	db           *pgxpool.Pool
	adminJWKS    jwk.Set
	adminIssuer  string
	adminAud     string
	encrypterKey []byte
}

// New constructs App and performs one-time startup tasks (schema, seeds, registry import).
func New(log *zap.SugaredLogger, db *pgxpool.Pool, cfg Config) *App {
	app := &App{
		log:         log,
		db:          db,
		adminIssuer: cfg.OIDCIssuer,
		adminAud:    cfg.OIDCAudience,
	}
	if k := cfg.EncryptionKey; k != "" {
		app.encrypterKey = []byte(k)
	}
	if cfg.JWKSURL != "" {
		app.adminJWKS = mustJWKS(cfg.JWKSURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ensure DB schema and seed/import registry
	if err := ensureConnectorSchema(ctx, app.db); err != nil {
		log.Fatalf("ensureConnectorSchema: %v", err)
	}
	if err := ensureTenantMarketplaceSchema(ctx, app.db); err != nil {
		log.Fatalf("ensure marketplace schema: %v", err)
	}
	if dir := cfg.RegistryDir; dir != "" {
		if err := importConnectorsFromDir(ctx, app.db, log, dir); err != nil {
			log.Warnf("registry import failed: %v", err)
		}
	}
	var cnt int
	_ = app.db.QueryRow(ctx, `SELECT COUNT(*) FROM connectors`).Scan(&cnt)
	if cnt == 0 {
		_ = seedDefaultConnector(ctx, app.db, log)
	}
	return app
}
