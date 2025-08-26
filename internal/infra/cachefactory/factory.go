package cachefactory

import (
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/cache"
	cmem "github.com/dropDatabas3/hellojohn/internal/cache/memory"
	credis "github.com/dropDatabas3/hellojohn/internal/cache/redis"
)

type Config struct {
	Kind  string
	Redis struct {
		Addr string
		DB   int
	}
	Memory struct{ DefaultTTL string }
}

func Open(cfg Config) (cache.Cache, error) {
	switch strings.ToLower(cfg.Kind) {
	case "redis":
		return credis.New(cfg.Redis.Addr, cfg.Redis.DB), nil
	default:
		d, _ := time.ParseDuration(cfg.Memory.DefaultTTL)
		if d == 0 {
			d = 2 * time.Minute
		}
		return cmem.New(d), nil
	}
}
