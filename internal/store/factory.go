package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/dropDatabas3/hellojohn/internal/store/mongo"
	"github.com/dropDatabas3/hellojohn/internal/store/mysql"
	"github.com/dropDatabas3/hellojohn/internal/store/pg"
)

type Config struct {
	Driver   string
	DSN      string
	Postgres struct {
		MaxOpenConns, MaxIdleConns int
		ConnMaxLifetime            string
	}
	MySQL struct{ DSN string }
	Mongo struct{ URI, Database string }
}

func Open(ctx context.Context, cfg Config) (core.Repository, error) {
	d := strings.ToLower(cfg.Driver)
	switch d {
	case "postgres", "pg", "postgresql":
		return pg.New(ctx, cfg.DSN, cfg.Postgres)
	case "mysql":
		dsn := cfg.DSN
		if cfg.MySQL.DSN != "" {
			dsn = cfg.MySQL.DSN
		}
		return mysql.New(ctx, dsn)
	case "mongo", "mongodb":
		return mongo.New(ctx, cfg.Mongo.URI, cfg.Mongo.Database)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
}
