// pkg/config/config.go
package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env          string
	HTTPAddr     string // connector-service
	ManifestAddr string // manifest-service

	// Default discovery/base values (tenant-specific override via provider)
	DefaultBasePublicURL string

	// OIDC / JWT (may be tenant-specific via provider)
	Issuer        string
	Audience      string
	JWKSURL       string
	RequireDPoP   bool
	DPoPClockSkew time.Duration

	// Redis & Postgres
	RedisURL    string
	DatabaseURL string
}

func Load() Config {
	_ = godotenv.Load()
	cfg := Config{
		Env:                  env("LAMDIS_ENV", "dev"),
		HTTPAddr:             env("LAMDIS_HTTP_ADDR", ":8080"),
		ManifestAddr:         env("LAMDIS_MANIFEST_ADDR", ":8081"),
		DefaultBasePublicURL: env("BASE_PUBLIC_URL", "http://localhost:8080"),
		Issuer:               env("OIDC_ISSUER", ""),
		Audience:             env("OIDC_AUDIENCE", "lamdis-gateway"),
		JWKSURL:              env("JWKS_URL", ""),
		RequireDPoP:          envBool("REQUIRE_DPOP", false),
		DPoPClockSkew:        envDur("DPOP_CLOCK_SKEW_SEC", 60) * time.Second,
		RedisURL:             env("REDIS_URL", ""),
		DatabaseURL:          env("DATABASE_URL", ""),
	}
	if cfg.DatabaseURL == "" {
		log.Println("[WARN] DATABASE_URL not set — using in-memory tenant provider for dev")
	}
	return cfg
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
func envBool(k string, def bool) bool {
	if v := os.Getenv(k); v != "" {
		b, _ := strconv.ParseBool(v)
		return b
	}
	return def
}
func envDur(k string, def int) time.Duration {
	if v := os.Getenv(k); v != "" {
		i, _ := strconv.Atoi(v)
		return time.Duration(i)
	}
	return time.Duration(def)
}
