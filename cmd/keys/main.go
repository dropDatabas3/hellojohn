package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"github.com/dropDatabas3/hellojohn/internal/config"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/store/v1"
	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
	pgdriver "github.com/dropDatabas3/hellojohn/internal/store/v1/pg"
)

func main() {
	var (
		flagEnvOnly     = flag.Bool("env", false, "usar SOLO env (y .env si se pasa -env-file)")
		flagEnvFile     = flag.String("env-file", ".env", "ruta a .env")
		flagConfigPath  = flag.String("config", "", "ruta a config.yaml (si no se usa -env)")
		cmdRotate       = flag.Bool("rotate", false, "genera nueva clave ACTIVE y pasa la anterior a RETIRING")
		cmdRetire       = flag.Bool("retire", false, "marca claves RETIRING antiguas como RETIRED (limpia JWKS)")
		cmdList         = flag.Bool("list", false, "lista todas las claves con sus estados")
		cmdGenSecretbox = flag.Bool("gen-secretbox", false, "genera nueva clave para SECRETBOX_MASTER_KEY")
		flagNotBefore   = flag.String("not-before", "", "RFC3339 (opcional) para not_before de la nueva clave")
		flagAge         = flag.String("age", "24h", "duración mínima para marcar retiring->retired")
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
	case *cmdGenSecretbox:
		generateSecretboxKey()
		return
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

		// Opcional: cifrar private_key si SIGNING_MASTER_KEY está presente
		if masterKey := os.Getenv("SIGNING_MASTER_KEY"); masterKey != "" {
			encrypted, err := jwtx.EncryptPrivateKey(k.PrivateKey, masterKey)
			if err != nil {
				log.Fatalf("encrypt private key: %v", err)
			}
			k.PrivateKey = encrypted
			fmt.Printf("Private key encrypted with SIGNING_MASTER_KEY\n")
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
	case *cmdRetire:
		minAge, err := time.ParseDuration(*flagAge)
		if err != nil {
			log.Fatalf("invalid age: %v", err)
		}
		cutoff := time.Now().UTC().Add(-minAge)

		count, err := pgRepo.RetireOldKeys(ctx, cutoff)
		if err != nil {
			log.Fatalf("retire: %v", err)
		}
		fmt.Printf("Marked %d retiring keys as retired (older than %s)\n", count, *flagAge)
	case *cmdList:
		keys, err := pgRepo.ListAllSigningKeys(ctx)
		if err != nil {
			log.Fatalf("list: %v", err)
		}
		fmt.Printf("KID\t\t\t\tSTATUS\t\tNOT_BEFORE\t\t\tCREATED_AT\t\t\tROTATED_AT\n")
		for _, k := range keys {
			rotated := ""
			if k.RotatedAt != nil {
				rotated = k.RotatedAt.Format("2006-01-02 15:04:05")
			}
			fmt.Printf("%s\t%s\t\t%s\t%s\t%s\n",
				k.KID, k.Status,
				k.NotBefore.Format("2006-01-02 15:04:05"),
				k.CreatedAt.Format("2006-01-02 15:04:05"),
				rotated)
		}
	default:
		fmt.Println("usage:")
		fmt.Println("  keys -rotate [-env | -config configs/config.yaml] [-env-file .env] [-not-before RFC3339]")
		fmt.Println("  keys -retire [-age 24h]")
		fmt.Println("  keys -list")
		fmt.Println("  keys -gen-secretbox")
	}
}

func generateSecretboxKey() {
	fmt.Println("🔐 HelloJohn - Secret Key Generator")
	fmt.Println("Generating 32-byte base64 key for SECRETBOX_MASTER_KEY...")

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		fmt.Printf("❌ Error generating key: %v\n", err)
		os.Exit(1)
	}

	encoded := base64.StdEncoding.EncodeToString(key)
	fmt.Printf("✅ Generated key: %s\n", encoded)
	fmt.Println("\n💡 Add this to your .env file:")
	fmt.Printf("SECRETBOX_MASTER_KEY=%s\n", encoded)
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
