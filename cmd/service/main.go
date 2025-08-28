package main

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpserver "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/handlers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/rate"
	"github.com/dropDatabas3/hellojohn/internal/store"
	pgdriver "github.com/dropDatabas3/hellojohn/internal/store/pg"
	rdb "github.com/redis/go-redis/v9"
)

// Adapter para que rate.Limiter cumpla con http.RateLimiter
type redisLimiterAdapter struct {
	inner rate.Limiter
}

func (a redisLimiterAdapter) Allow(ctx context.Context, key string) (struct {
	Allowed     bool
	Remaining   int64
	RetryAfter  time.Duration
	WindowTTL   time.Duration
	CurrentHits int64
}, error) {
	res, err := a.inner.Allow(ctx, key)
	if err != nil {
		return struct {
			Allowed     bool
			Remaining   int64
			RetryAfter  time.Duration
			WindowTTL   time.Duration
			CurrentHits int64
		}{}, err
	}
	return struct {
		Allowed     bool
		Remaining   int64
		RetryAfter  time.Duration
		WindowTTL   time.Duration
		CurrentHits int64
	}{
		Allowed:     res.Allowed,
		Remaining:   res.Remaining,
		RetryAfter:  res.RetryAfter,
		WindowTTL:   res.WindowTTL,
		CurrentHits: res.CurrentHits,
	}, nil
}

func main() {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "configs/config.example.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()

	// Store (multi-DB)
	repo, err := store.Open(ctx, store.Config{
		Driver: cfg.Storage.Driver,
		DSN:    cfg.Storage.DSN,
		Postgres: struct {
			MaxOpenConns, MaxIdleConns int
			ConnMaxLifetime            string
		}{
			MaxOpenConns:    cfg.Storage.Postgres.MaxOpenConns,
			MaxIdleConns:    cfg.Storage.Postgres.MaxIdleConns,
			ConnMaxLifetime: cfg.Storage.Postgres.ConnMaxLifetime,
		},
		MySQL: struct{ DSN string }{DSN: cfg.Storage.MySQL.DSN},
		Mongo: struct{ URI, Database string }{URI: cfg.Storage.Mongo.URI, Database: cfg.Storage.Mongo.Database},
	})
	if err != nil {
		log.Fatalf("store open: %v", err)
	}

	// Migraciones
	if cfg.Flags.Migrate {
		if pg, ok := repo.(interface {
			RunMigrations(context.Context, string) error
		}); ok {
			if err := pg.RunMigrations(ctx, "migrations/postgres"); err != nil {
				log.Fatalf("migrations: %v", err)
			}
		} else if _, ok := repo.(*pgdriver.Store); ok {
			if err := repo.(*pgdriver.Store).RunMigrations(ctx, "migrations/postgres"); err != nil {
				log.Fatalf("migrations: %v", err)
			}
		}
	}

	// JWT / JWKS
	keys, err := jwtx.NewDevEd25519("dev-1")
	if err != nil {
		log.Fatalf("keys: %v", err)
	}
	iss := cfg.JWT.Issuer
	if iss == "" {
		iss = "http://localhost:8080"
	}
	issuer := jwtx.NewIssuer(iss, keys)
	if cfg.JWT.AccessTTL != "" {
		if d, err := time.ParseDuration(cfg.JWT.AccessTTL); err == nil {
			issuer.AccessTTL = d
		}
	}
	// Refresh TTL (desde config si viene)
	refreshTTL := 30 * 24 * time.Hour
	if cfg.JWT.RefreshTTL != "" {
		if d, err := time.ParseDuration(cfg.JWT.RefreshTTL); err == nil {
			refreshTTL = d
		}
	}

	// Container DI
	container := app.Container{
		Store:  repo,
		Issuer: issuer,
	}

	// Handlers base
	jwksHandler := handlers.NewJWKSHandler(keys.JWKSJSON())
	authLoginHandler := handlers.NewAuthLoginHandler(&container, refreshTTL)
	authRegisterHandler := handlers.NewAuthRegisterHandler(&container, cfg.Register.AutoLogin, refreshTTL)
	authRefreshHandler := handlers.NewAuthRefreshHandler(&container, refreshTTL)
	authLogoutHandler := handlers.NewAuthLogoutHandler(&container)
	meHandler := handlers.NewMeHandler(&container)

	// Rate limiter (Redis) y ping opcional para /readyz
	var limiter httpserver.RateLimiter
	var redisPing func(context.Context) error
	if cfg.Rate.Enabled && strings.EqualFold(cfg.Cache.Kind, "redis") {
		rc := rdb.NewClient(&rdb.Options{
			Addr: cfg.Cache.Redis.Addr,
			DB:   cfg.Cache.Redis.DB,
		})
		if win, err := time.ParseDuration(cfg.Rate.Window); err == nil {
			rl := rate.NewRedisLimiter(rc, cfg.Cache.Redis.Prefix+"rl:", cfg.Rate.MaxRequests, win)
			limiter = redisLimiterAdapter{inner: rl}
		}
		redisPing = func(ctx context.Context) error { return rc.Ping(ctx).Err() }
	}

	// /readyz con chequeo de Redis si está disponible
	readyzHandler := handlers.NewReadyzHandler(&container, redisPing)

	// HTTP mux
	mux := httpserver.NewMux(
		jwksHandler,
		authLoginHandler,
		authRegisterHandler,
		authRefreshHandler,
		authLogoutHandler,
		meHandler,
		readyzHandler,
	)

	// Middlewares: CORS -> RateLimit -> RequestID -> Recover -> Logging
	handler := httpserver.WithLogging(
		httpserver.WithRecover(
			httpserver.WithRequestID(
				httpserver.WithRateLimit(
					httpserver.WithCORS(mux, cfg.Server.CORSAllowedOrigins),
					limiter,
				),
			),
		),
	)

	log.Printf("service up. addr=%s time=%s", cfg.Server.Addr, time.Now().Format(time.RFC3339))
	if err := httpserver.Start(cfg.Server.Addr, handler); err != nil {
		log.Fatalf("http: %v", err)
	}
}
