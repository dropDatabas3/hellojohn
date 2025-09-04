package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/config"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/jackc/pgx/v5/pgxpool"
)

func csvEnv(key string, def []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func strEnv(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func urlEncode(s string) string {
	r := strings.NewReplacer(":", "%3A", "/", "%2F")
	return r.Replace(s)
}

func main() {
	cfg, err := config.Load("configs/config.example.yaml")
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.Storage.DSN)
	if err != nil {
		log.Fatalf("pgxpool: %v", err)
	}
	defer pool.Close()

	// ------------------ Defaults (overrideables por ENV) ------------------
	tenantSlug := strEnv("SEED_TENANT_SLUG", "local")
	tenantName := strEnv("SEED_TENANT_NAME", "Local Tenant")

	adminEmail := strEnv("SEED_ADMIN_EMAIL", "admin@local.test")
	adminPass := strEnv("SEED_ADMIN_PASSWORD", "supersecreta")

	clientID := strEnv("SEED_CLIENT_ID", "web-frontend")
	clientName := strEnv("SEED_CLIENT_NAME", "Web Frontend")
	clientType := strEnv("SEED_CLIENT_TYPE", "public") // public|confidential

	allowedOrigins := csvEnv("SEED_ALLOWED_ORIGINS",
		[]string{"http://localhost:3000", "http://127.0.0.1:3000"},
	)
	redirectURIs := csvEnv("SEED_REDIRECT_URIS",
		[]string{
			"http://localhost:3000/callback",
			"http://localhost:8080/v1/auth/social/result",
		},
	)
	providers := csvEnv("SEED_PROVIDERS",
		[]string{"password", "google"},
	)
	scopes := csvEnv("SEED_SCOPES",
		[]string{"openid", "email", "profile"},
	)
	// ---------------------------------------------------------------------

	// 1) Upsert Tenant
	var tenantID string
	err = pool.QueryRow(ctx, `
		INSERT INTO tenant (name, slug, settings)
		VALUES ($1, $2, '{}'::jsonb)
		ON CONFLICT (slug) DO UPDATE
		  SET name = EXCLUDED.name
		RETURNING id
	`, tenantName, tenantSlug).Scan(&tenantID)
	if err != nil {
		log.Fatalf("upsert tenant: %v", err)
	}
	fmt.Println("TENANT_ID:", tenantID)

	// 2) Admin user + identidad password
	phc, _ := password.Hash(password.Default, adminPass)

	_, err = pool.Exec(ctx, `
		INSERT INTO app_user (tenant_id, email, email_verified, status, metadata)
		VALUES ($1, $2, true, 'active', '{}'::jsonb)
		ON CONFLICT (tenant_id, email) DO NOTHING
	`, tenantID, adminEmail)
	if err != nil {
		log.Fatalf("insert user: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO identity (user_id, provider, email, email_verified, password_hash)
		SELECT id, 'password', $1, true, $2
		FROM app_user
		WHERE tenant_id = $3 AND email = $1
		ON CONFLICT DO NOTHING
	`, adminEmail, phc, tenantID)
	if err != nil {
		log.Fatalf("insert identity: %v", err)
	}

	// 3) Upsert Client
	var dbClientID string
	err = pool.QueryRow(ctx, `
		INSERT INTO client (
			tenant_id, name, client_id, client_type,
			redirect_uris, allowed_origins, providers, scopes
		)
		VALUES ($1, $2, $3, $4, $5::text[], $6::text[], $7::text[], $8::text[])
		ON CONFLICT (client_id) DO UPDATE SET
			tenant_id = EXCLUDED.tenant_id,
			name      = EXCLUDED.name,
			client_type = EXCLUDED.client_type,
			redirect_uris = ARRAY(
				SELECT DISTINCT x FROM unnest(client.redirect_uris || EXCLUDED.redirect_uris) AS x
			),
			allowed_origins = ARRAY(
				SELECT DISTINCT x FROM unnest(client.allowed_origins || EXCLUDED.allowed_origins) AS x
			),
			providers = ARRAY(
				SELECT DISTINCT x FROM unnest(client.providers || EXCLUDED.providers) AS x
			),
			scopes = ARRAY(
				SELECT DISTINCT x FROM unnest(client.scopes || EXCLUDED.scopes) AS x
			)
		RETURNING id
	`, tenantID, clientName, clientID, clientType, redirectURIs, allowedOrigins, providers, scopes).Scan(&dbClientID)
	if err != nil {
		log.Fatalf("upsert client: %v", err)
	}

	// 4) Versión activa (sin múltiples sentencias en un Exec)
	var hasActive bool
	if err := pool.QueryRow(ctx, `SELECT active_version_id IS NOT NULL FROM client WHERE id = $1`, dbClientID).Scan(&hasActive); err != nil {
		log.Fatalf("check active version: %v", err)
	}
	if !hasActive {
		var cvID string
		// INSERT (una sola sentencia)
		if err := pool.QueryRow(ctx, `
			INSERT INTO client_version (client_id, version, claim_schema_json, claim_mapping_json, crypto_config_json, status)
			VALUES ($1, 'v1', '{}'::jsonb, '{}'::jsonb, '{"alg":"EdDSA"}'::jsonb, 'active')
			RETURNING id
		`, dbClientID).Scan(&cvID); err != nil {
			log.Fatalf("insert client_version: %v", err)
		}
		// UPDATE (otra sentencia independiente)
		if _, err := pool.Exec(ctx, `
			UPDATE client SET active_version_id = $2 WHERE id = $1
		`, dbClientID, cvID); err != nil {
			log.Fatalf("activate client_version: %v", err)
		}
	}

	fmt.Println()
	fmt.Println("Seed listo ✅")
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Tenant:    %s (slug=%s)\n", tenantID, tenantSlug)
	fmt.Printf("Admin:     %s / %s\n", adminEmail, adminPass)
	fmt.Printf("ClientID:  %s (type=%s)\n", clientID, clientType)
	fmt.Printf("Origins:   %v\n", allowedOrigins)
	fmt.Printf("Redirects: %v\n", redirectURIs)
	fmt.Printf("Providers: %v\n", providers)
	fmt.Printf("Scopes:    %v\n", scopes)
	fmt.Println("--------------------------------------------------")
	fmt.Println("URL de prueba:")
	fmt.Printf(
		"http://localhost:8080/v1/auth/social/google/start?tenant_id=%s&client_id=%s&redirect_uri=%s\n",
		tenantID, clientID, urlEncode("http://localhost:8080/v1/auth/social/result"),
	)
}
