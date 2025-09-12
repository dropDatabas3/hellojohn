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

	// 4) Arrancar server escogiendo .env.dev si existe en la RAÍZ del repo
	envFile := ".env"
	if root, err := findRepoRoot(); err == nil {
		if _, err := os.Stat(filepath.Join(root, ".env.dev")); err == nil {
			envFile = ".env.dev"
			// Cargar las variables de entorno del archivo para el proceso de prueba
			if err := godotenv.Load(filepath.Join(root, ".env.dev")); err != nil {
				panic(err)
			}
		}
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
		srv.stop()
	}
	os.Exit(code)
}
