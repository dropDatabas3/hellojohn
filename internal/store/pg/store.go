package pg

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct{ pool *pgxpool.Pool }

type pgCfg struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime string
}

func New(ctx context.Context, dsn string, cfg any) (*Store, error) {
	pcfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}

	// Parse optional tuning from cfg
	if v, ok := cfg.(struct {
		MaxOpenConns, MaxIdleConns int
		ConnMaxLifetime            string
	}); ok {
		if v.MaxOpenConns > 0 {
			pcfg.MaxConns = int32(v.MaxOpenConns)
		}
		// pgxpool no tiene "idle conns" explícito; maneja el total con MaxConns
		if v.ConnMaxLifetime != "" {
			if d, err := time.ParseDuration(v.ConnMaxLifetime); err == nil {
				pcfg.MaxConnLifetime = d
			}
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

type noopTx struct{}

func (n *noopTx) Commit(context.Context) error            { return nil }
func (n *noopTx) Rollback(context.Context) error          { return nil }
func (s *Store) BeginTx(context.Context) (core.Tx, error) { return &noopTx{}, nil }

// ====================== AUTH ======================

func (s *Store) GetUserByEmail(ctx context.Context, tenantID, email string) (*core.User, *core.Identity, error) {
	const q = `
SELECT u.id, u.tenant_id, u.email, u.email_verified, u.status, u.metadata, u.created_at,
       i.id, i.provider, i.provider_user_id, i.email, i.email_verified, i.password_hash, i.created_at
FROM app_user u
JOIN identity i ON i.user_id = u.id AND i.provider = 'password'
WHERE u.tenant_id = $1 AND LOWER(u.email) = LOWER($2)
LIMIT 1`
	row := s.pool.QueryRow(ctx, q, tenantID, email)

	var u core.User
	var i core.Identity
	var meta map[string]any
	// campos NULLables
	var providerUserID *string
	var idEmail *string
	var idEmailVerified *bool
	var pwd *string

	if err := row.Scan(
		&u.ID, &u.TenantID, &u.Email, &u.EmailVerified, &u.Status, &meta, &u.CreatedAt,
		&i.ID, &i.Provider, &providerUserID, &idEmail, &idEmailVerified, &pwd, &i.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, core.ErrNotFound
		}
		return nil, nil, err
	}
	u.Metadata = meta
	i.UserID = u.ID
	if providerUserID != nil {
		i.ProviderUserID = *providerUserID
	}
	if idEmail != nil {
		i.Email = *idEmail
	}
	if idEmailVerified != nil {
		i.EmailVerified = *idEmailVerified
	}
	i.PasswordHash = pwd
	return &u, &i, nil
}

func (s *Store) CheckPassword(hash *string, plain string) bool {
	if hash == nil || *hash == "" || plain == "" {
		return false
	}
	return password.Verify(plain, *hash)
}

// ====================== REGISTRY ======================

func (s *Store) CreateTenant(ctx context.Context, t *core.Tenant) error {
	const q = `
INSERT INTO tenant (id, name, slug, settings)
VALUES (gen_random_uuid(), $1, $2, $3)
RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q, t.Name, t.Slug, t.Settings).Scan(&t.ID, &t.CreatedAt)
}

func (s *Store) CreateClient(ctx context.Context, c *core.Client) error {
	const q = `
INSERT INTO client
(id, tenant_id, name, client_id, client_type, redirect_uris, allowed_origins, providers, scopes)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q,
		c.TenantID, c.Name, c.ClientID, c.ClientType,
		c.RedirectURIs, c.AllowedOrigins, c.Providers, c.Scopes).
		Scan(&c.ID, &c.CreatedAt)
}

func (s *Store) CreateClientVersion(ctx context.Context, v *core.ClientVersion) error {
	const q = `
INSERT INTO client_version
(id, client_id, version, claim_schema_json, claim_mapping_json, crypto_config_json, status)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, 'draft')
RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q,
		v.ClientID, v.Version, v.ClaimSchemaJSON, v.ClaimMappingJSON, v.CryptoConfigJSON).
		Scan(&v.ID, &v.CreatedAt)
}

func (s *Store) PromoteClientVersion(ctx context.Context, clientID, versionID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `UPDATE client SET active_version_id = $1 WHERE id = $2`, versionID, clientID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE client_version SET status = 'active', promoted_at = now() WHERE id = $1`, versionID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE client_version SET status = 'retired' WHERE client_id = $1 AND id <> $2 AND status = 'active'`, clientID, versionID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) GetClientByClientID(ctx context.Context, clientID string) (*core.Client, *core.ClientVersion, error) {
	const q = `
SELECT c.id, c.tenant_id, c.name, c.client_id, c.client_type, c.redirect_uris, c.allowed_origins, c.providers, c.scopes, c.active_version_id, c.created_at,
       v.id, v.version, v.claim_schema_json, v.claim_mapping_json, v.crypto_config_json, v.status, v.created_at, v.promoted_at
FROM client c
LEFT JOIN client_version v ON v.id = c.active_version_id
WHERE c.client_id = $1
LIMIT 1`
	row := s.pool.QueryRow(ctx, q, clientID)

	var c core.Client
	var v core.ClientVersion
	var active *string
	if err := row.Scan(&c.ID, &c.TenantID, &c.Name, &c.ClientID, &c.ClientType, &c.RedirectURIs, &c.AllowedOrigins, &c.Providers, &c.Scopes, &active, &c.CreatedAt,
		&v.ID, &v.Version, &v.ClaimSchemaJSON, &v.ClaimMappingJSON, &v.CryptoConfigJSON, &v.Status, &v.CreatedAt, &v.PromotedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, core.ErrNotFound
		}
		return nil, nil, err
	}
	c.ActiveVersionID = active
	if v.ID == "" {
		return &c, nil, nil
	}
	v.ClientID = c.ID
	return &c, &v, nil
}

// CreateUser crea o devuelve el existente (upsert) y rellena ID/CreatedAt.
func (s *Store) CreateUser(ctx context.Context, u *core.User) error {
	if u.Metadata == nil {
		u.Metadata = map[string]any{}
	}
	// guardamos email en minúscula para consistencia
	const q = `
INSERT INTO app_user (id, tenant_id, email, email_verified, status, metadata)
VALUES (gen_random_uuid(), $1, LOWER($2), $3, $4, $5)
ON CONFLICT (tenant_id, email)
DO UPDATE SET email = EXCLUDED.email
RETURNING id, created_at`
	return s.pool.QueryRow(ctx, q,
		u.TenantID, u.Email, u.EmailVerified, u.Status, u.Metadata,
	).Scan(&u.ID, &u.CreatedAt)
}

func (s *Store) CreatePasswordIdentity(ctx context.Context, userID, email string, emailVerified bool, passwordHash string) error {
	const q = `
INSERT INTO identity (id, user_id, provider, email, email_verified, password_hash)
VALUES (gen_random_uuid(), $1, 'password', LOWER($2), $3, $4)
ON CONFLICT (user_id, provider) DO NOTHING
RETURNING id`
	var id string
	err := s.pool.QueryRow(ctx, q, userID, email, emailVerified, passwordHash).Scan(&id)
	if err != nil {
		// si no hay row (por ON CONFLICT DO NOTHING) pgx da ErrNoRows
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrConflict // ya existía identidad password
		}
		return err
	}
	return nil
}

// ====================== MIGRACIONES ======================

func (s *Store) RunMigrations(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(strings.ToLower(e.Name()), ".sql") {
			files = append(files, dir+"/"+e.Name())
		}
	}
	sort.Strings(files)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		if _, err := s.pool.Exec(ctx, string(b)); err != nil {
			return fmt.Errorf("exec %s: %w", f, err)
		}
	}
	return nil
}
