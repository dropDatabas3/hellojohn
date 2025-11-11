package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	pwd "github.com/dropDatabas3/hellojohn/internal/security/password"
)

// adminFSUser models the JSON entry stored on disk. Kept local to avoid exporting internals.
type adminFSUser struct {
	ID           string         `json:"id"`
	Email        string         `json:"email"`
	PasswordHash string         `json:"password_hash"`
	Roles        []string       `json:"roles,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type adminFSDB struct {
	Users map[string]adminFSUser `json:"users"` // key = lowercased email
}

func fsRoot() string {
	root := strings.TrimSpace(os.Getenv("CONTROL_PLANE_FS_ROOT"))
	if root == "" {
		root = "./data/hellojohn"
	}
	return root
}

func dbPath() string {
	return filepath.Join(fsRoot(), "admin", "admin_users.json")
}

func loadDB() (*adminFSDB, error) {
	p := dbPath()
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return &adminFSDB{Users: map[string]adminFSUser{}}, nil
		}
		return nil, err
	}
	var db adminFSDB
	if err := json.Unmarshal(b, &db); err != nil {
		return nil, err
	}
	if db.Users == nil {
		db.Users = map[string]adminFSUser{}
	}
	return &db, nil
}

func saveDB(db *adminFSDB) error {
	if db == nil {
		return fmt.Errorf("nil db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath()), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	tmp := dbPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, dbPath())
}

func main() {
	var (
		envFile  = flag.String("env-file", ".env", "ruta a .env (opcional)")
		email    = flag.String("email", "", "email del admin FS")
		password = flag.String("password", "", "nueva contraseña en claro")
		ensure   = flag.Bool("ensure", false, "crea el admin si no existe; si existe, no cambia la contraseña")
		setPass  = flag.Bool("set", false, "crea o actualiza la contraseña del admin (idempotente)")
		showPath = flag.Bool("path", false, "muestra la ruta del archivo admin_users.json")
	)
	flag.Parse()

	// Best-effort load .env
	if *envFile != "" {
		_ = loadDotEnv(*envFile)
	}

	if *showPath {
		fmt.Println(dbPath())
		return
	}

	if !*ensure && !*setPass {
		fmt.Println("usage:")
		fmt.Println("  adminfs -set    -email user@admin.local -password 'NuevaClave123!'")
		fmt.Println("  adminfs -ensure -email user@admin.local -password 'Inicial123!'")
		fmt.Println("  adminfs -path   # muestra ruta de admin_users.json")
		os.Exit(2)
	}

	e := strings.ToLower(strings.TrimSpace(*email))
	if e == "" || strings.TrimSpace(*password) == "" {
		log.Fatal("email y password requeridos")
	}

	db, err := loadDB()
	if err != nil {
		log.Fatalf("load: %v", err)
	}

	// ensure: create only if missing, do not change existing password
	if *ensure {
		if _, ok := db.Users[e]; ok {
			fmt.Printf("ok: admin ya existe: %s\n", e)
			return
		}
		phc, err := pwd.Hash(pwd.Default, *password)
		if err != nil {
			log.Fatalf("hash: %v", err)
		}
		u := adminFSUser{
			ID:           uuid.NewString(),
			Email:        e,
			PasswordHash: phc,
			Roles:        []string{"sys:admin"},
			Metadata:     map[string]any{"is_admin": true},
			CreatedAt:    time.Now().UTC(),
		}
		db.Users[e] = u
		if err := saveDB(db); err != nil {
			log.Fatalf("save: %v", err)
		}
		fmt.Printf("ok: admin creado: %s\n", e)
		return
	}

	// set: upsert and always update password
	phc, err := pwd.Hash(pwd.Default, *password)
	if err != nil {
		log.Fatalf("hash: %v", err)
	}
	u := db.Users[e]
	if u.ID == "" {
		u.ID = uuid.NewString()
		u.CreatedAt = time.Now().UTC()
	}
	u.Email = e
	u.PasswordHash = phc
	if len(u.Roles) == 0 {
		u.Roles = []string{"sys:admin"}
	}
	if u.Metadata == nil {
		u.Metadata = map[string]any{"is_admin": true}
	} else {
		u.Metadata["is_admin"] = true
	}
	db.Users[e] = u
	if err := saveDB(db); err != nil {
		log.Fatalf("save: %v", err)
	}
	fmt.Printf("ok: contraseña actualizada para %s (id=%s)\n", e, u.ID)
}

// Minimal dotenv loader to avoid extra deps here.
func loadDotEnv(path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(b), "\n")
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" || strings.HasPrefix(ln, "#") {
			continue
		}
		// key=value
		if i := strings.IndexByte(ln, '='); i > 0 {
			k := strings.TrimSpace(ln[:i])
			v := strings.TrimSpace(ln[i+1:])
			// strip optional quotes
			v = strings.Trim(v, "\"'")
			_ = os.Setenv(k, v)
		}
	}
	return nil
}
