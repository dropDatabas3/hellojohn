package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
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

	// Convert config to store config
	storeCfg := store.Config{
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
	}

	// Store (multi‑DB)
	repo, err := store.Open(ctx, storeCfg)
	if err != nil {
		log.Fatalf("store open: %v", err)
	}

	// Migraciones (cuando aplique)
	if cfg.Flags.Migrate {
		if pg, ok := repo.(interface {
			RunMigrations(context.Context, string) error
		}); ok {
			if err := pg.RunMigrations(ctx, "migrations/postgres"); err != nil {
				log.Fatalf("migrations: %v", err)
			}
		} else if _, ok := repo.(*pgdriver.Store); ok {
			// fallback para PG concreto
			if err := repo.(*pgdriver.Store).RunMigrations(ctx, "migrations/postgres"); err != nil {
				log.Fatalf("migrations: %v", err)
			}
		}
	}

	// App container (por ahora solo guarda el repo; luego cache, jwt, etc.)
	container := app.Container{Store: repo}
	_ = container // avoid unused variable error for now

	log.Printf("login-svc up. addr=%s time=%s", cfg.Server.Addr, time.Now().Format(time.RFC3339))
	select {} // todavía sin HTTP server; seguimos bottom-up
}
