package helpers

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/google/uuid"
)

// FS-backed minimal admin users registry for FS-only mode.
// Guard usage behind FS_ADMIN_ENABLE=1 to avoid impacting normal DB-backed flows/tests.

type fsAdminUser struct {
	ID           string         `json:"id"`
	Email        string         `json:"email"`
	PasswordHash string         `json:"password_hash"`
	Roles        []string       `json:"roles,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type fsAdminDB struct {
	Users map[string]fsAdminUser `json:"users"` // key = lowercased email
}

func FSAdminEnabled() bool {
	return strings.TrimSpace(os.Getenv("FS_ADMIN_ENABLE")) == "1"
}

func fsAdminRoot() string {
	root := strings.TrimSpace(os.Getenv("CONTROL_PLANE_FS_ROOT"))
	if root == "" {
		root = "./data/hellojohn"
	}
	return filepath.Join(root, "admin")
}

func fsAdminPath() string {
	return filepath.Join(fsAdminRoot(), "admin_users.json")
}

func loadFSAdmin() (*fsAdminDB, error) {
	p := fsAdminPath()
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &fsAdminDB{Users: map[string]fsAdminUser{}}, nil
		}
		return nil, err
	}
	var db fsAdminDB
	if err := json.Unmarshal(b, &db); err != nil {
		return nil, err
	}
	if db.Users == nil {
		db.Users = map[string]fsAdminUser{}
	}
	return &db, nil
}

func saveFSAdmin(db *fsAdminDB) error {
	if db == nil {
		return errors.New("nil db")
	}
	if err := os.MkdirAll(fsAdminRoot(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return err
	}
	tmp := fsAdminPath() + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, fsAdminPath())
}

// FSAdminRegister creates or idempotently returns an admin user stored in FS.
// It does NOT enforce password blacklist or strong policy; this is for bootstrapping only.
func FSAdminRegister(email, plainPassword string) (fsAdminUser, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(plainPassword) == "" {
		return fsAdminUser{}, errors.New("missing email/password")
	}
	db, err := loadFSAdmin()
	if err != nil {
		return fsAdminUser{}, err
	}
	if u, ok := db.Users[email]; ok {
		// idempotent: if exists, keep existing and optionally update password if changed
		if u.PasswordHash == "" {
			if phc, err := password.Hash(password.Default, plainPassword); err == nil {
				u.PasswordHash = phc
				db.Users[email] = u
				_ = saveFSAdmin(db)
			}
		}
		return u, nil
	}
	phc, err := password.Hash(password.Default, plainPassword)
	if err != nil {
		return fsAdminUser{}, err
	}
	id := uuid.NewString()
	u := fsAdminUser{
		ID:           id,
		Email:        email,
		PasswordHash: phc,
		Roles:        []string{"sys:admin"},
		Metadata:     map[string]any{"is_admin": true},
		CreatedAt:    time.Now().UTC(),
	}
	db.Users[email] = u
	if err := saveFSAdmin(db); err != nil {
		return fsAdminUser{}, err
	}
	return u, nil
}

// FSAdminVerify verifies email/password against the FS admin DB.
func FSAdminVerify(email, plainPassword string) (fsAdminUser, bool) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(plainPassword) == "" {
		return fsAdminUser{}, false
	}
	db, err := loadFSAdmin()
	if err != nil {
		return fsAdminUser{}, false
	}
	u, ok := db.Users[email]
	if !ok || u.PasswordHash == "" {
		return fsAdminUser{}, false
	}
	if password.Verify(plainPassword, u.PasswordHash) {
		return u, true
	}
	return fsAdminUser{}, false
}
