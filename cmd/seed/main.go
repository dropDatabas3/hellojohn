package main

import (
	"context"
	"fmt"
	"log"

	"github.com/dropDatabas3/hellojohn/internal/config"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/dropDatabas3/hellojohn/internal/store"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load("../configs/config.example.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()

	// Abrimos repo para usar métodos de Registry (client/version/promote)
	repo, err := store.Open(ctx, store.Config{
		Driver: cfg.Storage.Driver,
		DSN:    cfg.Storage.DSN,
	})
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	// Conexión directa para user/identity (más simple que tocar la interfaz por ahora)
	pool, err := pgxpool.New(ctx, cfg.Storage.DSN)
	if err != nil {
		log.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	// 1) Tenant
	t := &core.Tenant{
		Name:     "REM Gestion",
		Slug:     "rem",
		Settings: map[string]any{},
	}
	if err := repo.CreateTenant(ctx, t); err != nil {
		log.Fatalf("create tenant: %v", err)
	}
	fmt.Println("TENANT_ID:", t.ID)

	// 2) Usuario + identidad (password)
	phc, _ := password.Hash(password.Default, "supersecreta")
	_, err = pool.Exec(ctx, `
INSERT INTO app_user (tenant_id, email, email_verified, status, metadata)
VALUES ($1, $2, true, 'active', '{}'::jsonb)
ON CONFLICT (tenant_id, email) DO NOTHING
`, t.ID, "admin@rem.com")
	if err != nil {
		log.Fatalf("insert user: %v", err)
	}

	_, err = pool.Exec(ctx, `
INSERT INTO identity (user_id, provider, email, email_verified, password_hash)
SELECT id, 'password', $1, true, $2
FROM app_user WHERE tenant_id = $3 AND email = $1
ON CONFLICT DO NOTHING
`, "admin@rem.com", phc, t.ID)
	if err != nil {
		log.Fatalf("insert identity: %v", err)
	}

	// 3) Client + version activa
	c := &core.Client{
		TenantID:       t.ID,
		Name:           "Frontend",
		ClientID:       "web-frontend",
		ClientType:     "public",
		RedirectURIs:   []string{"http://localhost:3000/callback"},
		AllowedOrigins: []string{"http://localhost:3000"},
		Providers:      []string{"password"},
		Scopes:         []string{"openid", "profile", "email"},
	}
	if err := repo.CreateClient(ctx, c); err != nil {
		log.Fatalf("create client: %v", err)
	}

	cv := &core.ClientVersion{
		ClientID:         c.ID,
		Version:          "v1",
		ClaimSchemaJSON:  []byte(`{}`),
		ClaimMappingJSON: []byte(`{}`),
		CryptoConfigJSON: []byte(`{"alg":"EdDSA"}`),
	}
	if err := repo.CreateClientVersion(ctx, cv); err != nil {
		log.Fatalf("create client version: %v", err)
	}
	if err := repo.PromoteClientVersion(ctx, c.ID, cv.ID); err != nil {
		log.Fatalf("promote client version: %v", err)
	}

	fmt.Println("Seed listo. tenant_id arriba. Usuario: admin@rem.com / supersecreta  ClientID: web-frontend")
}
