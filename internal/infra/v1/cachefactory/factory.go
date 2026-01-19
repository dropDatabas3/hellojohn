// Package cachefactory creates cache clients from configuration.
//
// Deprecated: This package is superseded by internal/cache/v2.
// For V2 refactoring, use cache.New() directly or rely on TenantDataAccess.Cache().
//
// Migration guide:
//
//	// BEFORE (V1):
//	import "github.com/dropDatabas3/hellojohn/internal/infra/v1/cachefactory"
//	cc, err := cachefactory.Open(cachefactory.Config{
//	    Kind: "redis",
//	    Redis: struct{ Addr string; DB int; Prefix string }{...},
//	})
//
//	// AFTER (V2 - direct):
//	import cache "github.com/dropDatabas3/hellojohn/internal/cache/v2"
//	cc, err := cache.New(cache.Config{
//	    Driver: "redis",
//	    Host:   "localhost",
//	    Port:   6379,
//	    Prefix: "myapp:",
//	})
//
//	// AFTER (V2 - via TenantDataAccess):
//	tda, _ := dal.ForTenant(ctx, slug)
//	tda.Cache().Set(ctx, "key", "value", 5*time.Minute)
package cachefactory

import (
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/cache/v1"
	cmem "github.com/dropDatabas3/hellojohn/internal/cache/v1/memory"
	credis "github.com/dropDatabas3/hellojohn/internal/cache/v1/redis"
)

type Config struct {
	Kind  string
	Redis struct {
		Addr   string
		DB     int
		Prefix string
	}
	Memory struct{ DefaultTTL string }
}

func Open(cfg Config) (cache.Cache, error) {
	switch strings.ToLower(cfg.Kind) {
	case "redis":
		if c, err := credis.New(cfg.Redis.Addr, cfg.Redis.DB, cfg.Redis.Prefix); err == nil {
			return c, nil
		}
		// Fallback silencioso a memoria si Redis no está disponible
		fallthrough
	default:
		d, _ := time.ParseDuration(cfg.Memory.DefaultTTL)
		if d == 0 {
			d = 2 * time.Minute
		}
		return cmem.New(d), nil
	}
}
