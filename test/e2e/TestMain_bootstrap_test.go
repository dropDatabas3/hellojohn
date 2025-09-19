package e2e

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestMain(m *testing.M) {
	// 1) Envs mínimos (SIGNING_MASTER_KEY >= 32 bytes = 64 hex chars)
	if len(os.Getenv("SIGNING_MASTER_KEY")) < 64 {
		_ = os.Setenv("SIGNING_MASTER_KEY", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	}

	// 2) Migrar
	{
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := runCmd(ctx, ".", "run", "./cmd/migrate"); err != nil {
			panic(err)
		}
	}

	// 2.5) Generar llaves JWT
	{
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := runCmd(ctx, ".", "run", "./cmd/keys", "-rotate"); err != nil {
			panic(err)
		}
	}

	// 3) Seed
	{
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := runCmd(ctx, ".", "run", "./cmd/seed"); err != nil {
			panic(err)
		}
		var err error
		seed, err = mustLoadSeedYAML()
		if err != nil {
			panic(err)
		}
	}

	// 4) Forzar uso de .env (o archivo indicado por E2E_ENV_FILE). Ignoramos .env.dev para evitar overrides en E2E.
	envFile := os.Getenv("E2E_ENV_FILE")
	if envFile == "" {
		envFile = ".env"
	}
	if root, err := findRepoRoot(); err == nil {
		candidate := filepath.Join(root, envFile)
		if _, err := os.Stat(candidate); err == nil {
			if err := godotenv.Load(candidate); err != nil {
				panic(err)
			}
			// Recalcular baseURL según variables cargadas
			if issuer := os.Getenv("JWT_ISSUER"); issuer != "" {
				baseURL = issuer
			} else if e := os.Getenv("EMAIL_BASE_URL"); e != "" {
				baseURL = e
			}
		} else {
			// Si no existe el .env requerido, fallamos temprano para que sea visible.
			panic("env file requerido no encontrado: " + candidate)
		}
	}

	// 4.5) Credenciales básicas para /oauth2/introspect (si no vienen dadas)
	if os.Getenv("INTROSPECT_BASIC_USER") == "" {
		_ = os.Setenv("INTROSPECT_BASIC_USER", "introspect-user")
	}
	if os.Getenv("INTROSPECT_BASIC_PASS") == "" {
		_ = os.Setenv("INTROSPECT_BASIC_PASS", "introspect-pass")
	}

	// 4.6) Password blacklist path (ensure we test rejection)
	if os.Getenv("SECURITY_PASSWORD_BLACKLIST_PATH") == "" {
		if root, err := findRepoRoot(); err == nil {
			_ = os.Setenv("SECURITY_PASSWORD_BLACKLIST_PATH", root+"/security_password_blacklist.txt")
		}
	}

	// 4.7) Enable social debug headers so social login_code test can simulate Google callback without real provider.
	if os.Getenv("SOCIAL_DEBUG_HEADERS") == "" {
		_ = os.Setenv("SOCIAL_DEBUG_HEADERS", "true")
	}

	// 4.8) Enable Google provider in tests with dummy credentials so discovery lists it and debug path works.
	if os.Getenv("GOOGLE_ENABLED") == "" {
		_ = os.Setenv("GOOGLE_ENABLED", "true")
	}
	// Provide safe dummy values (they won't be used because debug shortcut short-circuits real exchange).
	if os.Getenv("GOOGLE_CLIENT_ID") == "" {
		_ = os.Setenv("GOOGLE_CLIENT_ID", "dummy-google-client-id.apps.googleusercontent.com")
	}
	if os.Getenv("GOOGLE_CLIENT_SECRET") == "" {
		_ = os.Setenv("GOOGLE_CLIENT_SECRET", "dummy-google-secret")
	}

	var err error
	srv, err = startServer(context.Background(), envFile)
	if err != nil {
		panic(err)
	}
	if err := waitReady(baseURL, 20*time.Second); err != nil {
		if srv != nil && srv.out != nil {
			println(srv.out.String())
		}
		panic(err)
	}

	code := m.Run()

	if srv != nil {
		// Dump server logs for debugging social debug shortcut
		if srv.out != nil {
			println("--- SERVER LOGS START ---")
			println(srv.out.String())
			println("--- SERVER LOGS END ---")
		}
		srv.stop()
	}
	os.Exit(code)
}
