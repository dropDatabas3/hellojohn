package main

import (
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/dropDatabas3/hellojohn/internal/config"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/store"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	pgdriver "github.com/dropDatabas3/hellojohn/internal/store/pg"
)

func main() {
	var (
		flagEnvOnly    = flag.Bool("env", false, "usar SOLO env (y .env si se pasa -env-file)")
		flagEnvFile    = flag.String("env-file", ".env", "ruta a .env")
		flagConfigPath = flag.String("config", "", "ruta a config.yaml (si no se usa -env)")
		cmdRotate      = flag.Bool("rotate", false, "genera nueva clave ACTIVE y pasa la anterior a RETIRING")
		flagNotBefore  = flag.String("not-before", "", "RFC3339 (opcional) para not_before de la nueva clave")
	)
	flag.Parse()

	if *flagEnvFile != "" {
		_ = godotenv.Load(*flagEnvFile)
	}

	// Cargar config (igual que el service)
	var cfg *config.Config
	var err error
	if *flagEnvOnly {
		// reusar loader mínimo vía env del service (o llamar config.Load si lo prefieres)
		cfg = &config.Config{}
		cfg.Storage.Driver = getenv("STORAGE_DRIVER", "postgres")
		cfg.Storage.DSN = getenv("STORAGE_DSN", "postgres://user:password@localhost:5432/login?sslmode=disable")
	} else {
		path := *flagConfigPath
		if path == "" {
			if fileExists("configs/config.yaml") {
				path = "configs/config.yaml"
			} else {
				path = "configs/config.example.yaml"
			}
		}
		cfg, err = config.Load(path)
		if err != nil {
			log.Fatalf("config: %v", err)
		}
	}

	ctx := context.Background()
	repo, err := store.Open(ctx, store.Config{
		Driver: cfg.Storage.Driver,
		DSN:    cfg.Storage.DSN,
		Postgres: struct {
			MaxOpenConns, MaxIdleConns int
			ConnMaxLifetime            string
		}{},
	})
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	pgRepo, ok := repo.(*pgdriver.Store)
	if !ok {
		log.Fatalf("requires postgres store")
	}

	switch {
	case *cmdRotate:
		var nb time.Time
		if *flagNotBefore != "" {
			t, err := time.Parse(time.RFC3339, *flagNotBefore)
			if err != nil {
				log.Fatalf("invalid not-before: %v", err)
			}
			nb = t.UTC()
		} else {
			nb = time.Now().UTC()
		}

		pub, priv, err := jwtx.GenerateEd25519()
		if err != nil {
			log.Fatalf("generate key: %v", err)
		}
		k := core.SigningKey{
			KID:        "kid-" + time.Now().UTC().Format("20060102T150405Z"),
			Alg:        "EdDSA",
			PublicKey:  []byte(ed25519.PublicKey(pub)),
			PrivateKey: []byte(ed25519.PrivateKey(priv)),
			Status:     core.KeyActive,
			NotBefore:  nb,
		}
		prev, err := pgRepo.RotateSigningKeyTx(ctx, k)
		if err != nil {
			log.Fatalf("rotate: %v", err)
		}
		if prev != nil {
			fmt.Printf("Rotated. new_kid=%s previous_active=%s -> retiring\n", k.KID, prev.KID)
		} else {
			fmt.Printf("Inserted first active key. kid=%s\n", k.KID)
		}
	default:
		fmt.Println("usage: keys -rotate [-env | -config configs/config.yaml] [-env-file .env] [-not-before RFC3339]")
	}
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
func getenv(k, d string) string {
	if v := os.Getenv(k); strings.TrimSpace(v) != "" {
		return v
	}
	return d
}
