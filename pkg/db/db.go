// pkg/db/db.go
package db

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"lamdis/pkg/config"
)

func MustConnect(cfg config.Config, log *zap.SugaredLogger) *pgxpool.Pool {
	if cfg.DatabaseURL == "" {
		return nil
	}
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalw("pg connect", "err", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalw("pg ping", "err", err)
	}
	log.Infow("postgres ready", "host", redactDSN(cfg.DatabaseURL))
	return pool
}

func MustRedis(cfg config.Config, log *zap.SugaredLogger) *redis.Client {
	if cfg.RedisURL == "" {
		return nil
	}
	opts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalw("redis parse", "err", err)
	}
	cli := redis.NewClient(opts)
	if err := cli.Ping(context.Background()).Err(); err != nil {
		log.Fatalw("redis ping", "err", err)
	}
	log.Infow("redis ready", "addr", opts.Addr)
	return cli
}

func redactDSN(dsn string) string {
	if i := strings.Index(dsn, "@"); i > 0 {
		return "***@" + dsn[i+1:]
	}
	return dsn
}
