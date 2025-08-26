package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpserver "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/handlers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/store"
	pgdriver "github.com/dropDatabas3/hellojohn/internal/store/pg"
)

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

	// JWT / JWKS (dev)
	keys, err := jwtx.NewDevEd25519("dev-1")
	if err != nil {
		log.Fatalf("keys: %v", err)
	}
	issuer := jwtx.NewIssuer("hellojohn", keys) // default iss + 15m TTL

	// Container
	container := app.Container{
		Store:  repo,
		Issuer: issuer,
	}

	// Handlers
	jwksHandler := handlers.NewJWKSHandler(keys.JWKSJSON())
	authLoginHandler := handlers.NewAuthLoginHandler(&container)
	authRegisterHandler := handlers.NewAuthRegisterHandler(&container, false) // sin auto-login hasta que metas cfg

	// HTTP
	mux := httpserver.NewMux(jwksHandler, authLoginHandler, authRegisterHandler)

	log.Printf("login-svc up. addr=%s time=%s", cfg.Server.Addr, time.Now().Format(time.RFC3339))
	if err := httpserver.Start(cfg.Server.Addr, mux); err != nil {
		log.Fatalf("http: %v", err)
	}
}
