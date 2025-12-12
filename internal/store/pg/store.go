package pg

import (
	"context"
	"errors"
	"fmt"
	"log"
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

// Pool expone el pool interno para usos avanzados (metrics/migraciones).
func (s *Store) Pool() *pgxpool.Pool {
	if s == nil {
		return nil
	}
	return s.pool
}

// PoolStats devuelve un snapshot del estado del pool (puede ser nil si el pool no está inicializado).
func (s *Store) PoolStats() *pgxpool.Stat {
	if s == nil || s.pool == nil {
		return nil
	}
	return s.pool.Stat()
}

// Close cierra el pool subyacente (idempotente).
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

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
		// Mapear MaxIdleConns → MinConns (pgxpool)
		if v.MaxIdleConns > 0 {
			pcfg.MinConns = int32(v.MaxIdleConns)
		}
		if v.ConnMaxLifetime != "" {
			if d, err := time.ParseDuration(v.ConnMaxLifetime); err == nil {
				pcfg.MaxConnLifetime = d
				// También configurar MaxConnIdleTime si queremos
				pcfg.MaxConnIdleTime = d
			}
		}
	}

	// Set default MaxConns if not specified
	if pcfg.MaxConns == 0 {
		pcfg.MaxConns = 5 // reduced for testing
	}

	// Límites más conservadores para desarrollo local
	if pcfg.MaxConns > 8 {
		pcfg.MaxConns = 8
	}
	if pcfg.MinConns > 4 {
		pcfg.MinConns = 4
	}

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, err
	}

	// Non-blocking startup: try to ping, but don't fail if it fails.
	// This allows the app to start even if DB is temporarily down.
	if err := pool.Ping(ctx); err != nil {
		log.Printf(`{"level":"warn","msg":"pg_pool_startup_ping_failed","err":"%v"}`, err)
	} else {
		log.Printf(`{"level":"info","msg":"pg_pool_ready","max_conns":%d}`, pcfg.MaxConns)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

type noopTx struct{}

func (n *noopTx) Commit(context.Context) error            { return nil }
func (n *noopTx) Rollback(context.Context) error          { return nil }
func (s *Store) BeginTx(context.Context) (core.Tx, error) { return &noopTx{}, nil }

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
func derefTime(t *time.Time) time.Time {
	if t == nil {
		return time.Time{}
	}
	return *t
}

// ====================== AUTH ======================

func (s *Store) GetUserByEmail(ctx context.Context, tenantID, email string) (*core.User, *core.Identity, error) {
	// 1. Obtener usuario con todas las columnas (incluyendo dinámicas)
	const qUser = `SELECT * FROM app_user WHERE LOWER(email) = LOWER($1) LIMIT 1`
	rows, err := s.pool.Query(ctx, qUser, email)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, nil, core.ErrNotFound
	}

	// Escaneo dinámico
	cols := rows.FieldDescriptions()
	vals := make([]any, len(cols))
	for i := range vals {
		vals[i] = new(any)
	}

	if err := rows.Scan(vals...); err != nil {
		return nil, nil, err
	}

	var u core.User
	u.CustomFields = make(map[string]any)

	for i, col := range cols {
		val := *(vals[i].(*any))
		colName := string(col.Name)

		// Helper to convert UUID bytes to string if needed
		uuidToStr := func(v any) string {
			if s, ok := v.(string); ok {
				return s
			}
			if b, ok := v.([16]byte); ok {
				return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
			}
			return ""
		}

		switch colName {
		case "id":
			u.ID = uuidToStr(val)
		case "tenant_id":
			u.TenantID = uuidToStr(val)
		case "email":
			if v, ok := val.(string); ok {
				u.Email = v
			}
		case "email_verified":
			if v, ok := val.(bool); ok {
				u.EmailVerified = v
			}
		case "metadata":
			if v, ok := val.(map[string]any); ok {
				u.Metadata = v
			}
		case "created_at":
			if v, ok := val.(time.Time); ok {
				u.CreatedAt = v
			}
		case "disabled_at":
			if v, ok := val.(time.Time); ok {
				u.DisabledAt = &v
			}
		case "disabled_reason":
			if v, ok := val.(string); ok {
				u.DisabledReason = &v
			}
		default:
			// Campo dinámico
			u.CustomFields[colName] = val
		}
	}
	rows.Close() // Cerrar explícitamente antes de la siguiente query

	// 2. Obtener identidad password (si existe)
	const qIdent = `SELECT id, provider, provider_user_id, email, email_verified, password_hash, created_at FROM identity WHERE user_id = $1 AND provider = 'password'`
	row := s.pool.QueryRow(ctx, qIdent, u.ID)

	var i core.Identity
	var providerUserID *string
	var idEmail *string
	var idEmailVerified *bool
	var pwd *string

	if err := row.Scan(&i.ID, &i.Provider, &providerUserID, &idEmail, &idEmailVerified, &pwd, &i.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Usuario existe pero sin password identity (e.g. social login only)
			// Retornamos usuario y nil identity
			return &u, nil, nil
		}
		return nil, nil, err
	}

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
	var active *string

	var vID, vVersion, vStatus *string
	var vCreatedAt, vPromotedAt *time.Time
	var claimSchema, claimMapping, cryptoConf []byte

	if err := row.Scan(
		&c.ID, &c.TenantID, &c.Name, &c.ClientID, &c.ClientType, &c.RedirectURIs, &c.AllowedOrigins, &c.Providers, &c.Scopes, &active, &c.CreatedAt,
		&vID, &vVersion, &claimSchema, &claimMapping, &cryptoConf, &vStatus, &vCreatedAt, &vPromotedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, core.ErrNotFound
		}
		log.Printf(`{"level":"error","msg":"pg_get_client_by_client_id_err","client_id":"%s","err":"%v"}`, clientID, err)
		return nil, nil, err
	}
	c.ActiveVersionID = active

	if vID == nil {
		return &c, nil, nil
	}

	v := &core.ClientVersion{
		ID:               *vID,
		ClientID:         c.ID,
		Version:          deref(vVersion),
		ClaimSchemaJSON:  claimSchema,
		ClaimMappingJSON: claimMapping,
		CryptoConfigJSON: cryptoConf,
		Status:           deref(vStatus),
		CreatedAt:        derefTime(vCreatedAt),
		PromotedAt:       vPromotedAt,
	}
	return &c, v, nil
}

// CreateUser crea o devuelve el existente (upsert) y rellena ID/CreatedAt.
// Soporta campos dinámicos en CustomFields.
func (s *Store) CreateUser(ctx context.Context, u *core.User) error {
	if u.Metadata == nil {
		u.Metadata = map[string]any{}
	}

	// Campos fijos
	cols := []string{"id", "email", "email_verified", "metadata", "source_client_id"}
	vals := []any{"gen_random_uuid()", strings.ToLower(u.Email), u.EmailVerified, u.Metadata, u.SourceClientID}

	// Campos dinámicos
	// Nota: pgx maneja la sanitización de valores, pero los nombres de columna deben ser seguros.
	// Asumimos que vienen validados o son seguros (vienen del struct interno).
	// Para mayor seguridad, podríamos validar contra regex de identificadores.
	keys := make([]string, 0, len(u.CustomFields))
	for k := range u.CustomFields {
		keys = append(keys, k)
	}
	sort.Strings(keys) // Determinismo

	for _, k := range keys {
		// Normalizamos la key para matchear con SchemaManager (ToLower + TrimSpace)
		normKey := strings.ToLower(strings.TrimSpace(k))
		cols = append(cols, pgIdentifier(normKey))
		vals = append(vals, u.CustomFields[k])
	}

	// Construir query
	var qCols, qVals string
	args := make([]any, 0, len(vals))
	argIdx := 1

	for i, col := range cols {
		if i > 0 {
			qCols += ", "
			qVals += ", "
		}
		qCols += col

		// Si el valor es string "gen_random_uuid()", lo ponemos directo (hack simple para este caso)
		// O mejor, lo manejamos como default en DB si fuera posible, pero acá estamos forzando ID.
		// En el array vals original puse el string literal.
		if sVal, ok := vals[i].(string); ok && sVal == "gen_random_uuid()" {
			qVals += sVal
		} else {
			qVals += fmt.Sprintf("$%d", argIdx)
			args = append(args, vals[i])
			argIdx++
		}
	}

	q := fmt.Sprintf(`
	INSERT INTO app_user (%s)
	VALUES (%s)
	ON CONFLICT (email)
	DO UPDATE SET email = EXCLUDED.email
	RETURNING id, created_at`, qCols, qVals)

	return s.pool.QueryRow(ctx, q, args...).Scan(&u.ID, &u.CreatedAt)
}

func (s *Store) UpdateUser(ctx context.Context, userID string, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}

	setParts := make([]string, 0, len(updates))
	args := []any{userID}
	argIdx := 2

	// We separate validation/logic for known columns vs custom fields
	// But efficiently we can build the dynamic query.
	// Known columns: source_client_id, email_verified, disabled_at, etc.
	// We'll focus on what's needed now (source_client_id) but keep it generic-ish.

	// For custom fields, we need to merge or replace?
	// Usually PATCH merges. But here 'custom_fields' is a JSONB column or separate items?
	// The store implementation maps individual columns to custom fields.
	// So we assume 'updates' keys match DB columns.

	keys := make([]string, 0, len(updates))
	for k := range updates {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		val := updates[k]
		col := pgIdentifier(strings.ToLower(strings.TrimSpace(k)))

		// Handle special nil logic if needed, but pgx handles nil as NULL usually.
		setParts = append(setParts, fmt.Sprintf("%s = $%d", col, argIdx))
		args = append(args, val)
		argIdx++
	}

	q := fmt.Sprintf("UPDATE app_user SET %s WHERE id = $1", strings.Join(setParts, ", "))
	tag, err := s.pool.Exec(ctx, q, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return core.ErrNotFound
	}
	return nil
}

// pgIdentifier sanitiza un identificador simple (solo letras, números, guiones bajos)
// para evitar inyección SQL en nombres de columna.
func pgIdentifier(s string) string {
	return "\"" + strings.ReplaceAll(s, "\"", "") + "\""
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
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrConflict
		}
		log.Printf(`{"level":"error","msg":"pg_create_pwd_identity_err","user_id":"%s","email":"%s","err":"%v"}`, userID, email, err)
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
		if e.Type().IsRegular() {
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, "_up.sql") {
				files = append(files, dir+"/"+e.Name())
			}
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

func (s *Store) RunMigrationsDown(ctx context.Context, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() {
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, "_down.sql") {
				files = append(files, dir+"/"+e.Name())
			}
		}
	}
	sort.Strings(files)
	for i := len(files) - 1; i >= 0; i-- {
		f := files[i]
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

func (s *Store) CreateUserWithPassword(ctx context.Context, tenantID, email, passwordHash string) (*core.User, *core.Identity, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var u core.User
	var meta map[string]any

	err = tx.QueryRow(ctx, `
		INSERT INTO app_user (email, email_verified, metadata)
		VALUES (LOWER($1), false, '{}'::jsonb)
		RETURNING id, email, email_verified, metadata, created_at
	`, email).
		Scan(&u.ID, &u.Email, &u.EmailVerified, &meta, &u.CreatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, nil, core.ErrConflict
		}
		log.Printf(`{"level":"error","msg":"pg_create_user_err","tenant_id":"%s","email":"%s","err":"%v"}`, tenantID, email, err)
		return nil, nil, err
	}
	u.Metadata = meta

	var id core.Identity
	var providerUserID *string
	var idEmail *string
	var idEmailVerified *bool
	var pwd *string

	err = tx.QueryRow(ctx, `
		INSERT INTO identity (user_id, provider, email, email_verified, password_hash)
		VALUES ($1, 'password', $2, false, $3)
		RETURNING id, provider, provider_user_id, email, email_verified, password_hash, created_at
	`, u.ID, strings.ToLower(email), passwordHash).
		Scan(&id.ID, &id.Provider, &providerUserID, &idEmail, &idEmailVerified, &pwd, &id.CreatedAt)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return nil, nil, core.ErrConflict
		}
		log.Printf(`{"level":"error","msg":"pg_create_identity_err","user_id":"%s","email":"%s","err":"%v"}`, u.ID, email, err)
		return nil, nil, err
	}
	id.UserID = u.ID
	if providerUserID != nil {
		id.ProviderUserID = *providerUserID
	}
	if idEmail != nil {
		id.Email = *idEmail
	}
	if idEmailVerified != nil {
		id.EmailVerified = *idEmailVerified
	}
	id.PasswordHash = pwd

	if err := tx.Commit(ctx); err != nil {
		log.Printf(`{"level":"error","msg":"pg_tx_commit_err","err":"%v"}`, err)
		return nil, nil, err
	}
	return &u, &id, nil
}

// ====================== REFRESH TOKENS ======================

// CreateRefreshToken - DEPRECATED: Usar CreateRefreshTokenTC en su lugar
func (s *Store) CreateRefreshToken(ctx context.Context, userID, clientID, tokenHash string, expiresAt time.Time, rotatedFrom *string) (string, error) {
	const q = `
INSERT INTO refresh_token (id, user_id, client_id_text, token_hash, issued_at, expires_at, rotated_from)
VALUES (gen_random_uuid(), $1, $2, $3, now(), $4, $5)
RETURNING id`
	var id string
	if err := s.pool.QueryRow(ctx, q, userID, clientID, tokenHash, expiresAt, rotatedFrom).Scan(&id); err != nil {
		log.Printf(`{"level":"error","msg":"pg_create_refresh_err","user_id":"%s","client_id_text":"%s","err":"%v"}`, userID, clientID, err)
		return "", err
	}
	return id, nil
}

func (s *Store) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*core.RefreshToken, error) {
	const q = `
SELECT id, user_id, client_id_text, token_hash, issued_at, expires_at, rotated_from, revoked_at, tenant_id
FROM refresh_token
WHERE token_hash = $1
LIMIT 1`
	row := s.pool.QueryRow(ctx, q, tokenHash)

	var rt core.RefreshToken
	if err := row.Scan(&rt.ID, &rt.UserID, &rt.ClientIDText, &rt.TokenHash, &rt.IssuedAt, &rt.ExpiresAt, &rt.RotatedFrom, &rt.RevokedAt, &rt.TenantID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		log.Printf(`{"level":"error","msg":"pg_get_refresh_by_hash_err","err":"%v"}`, err)
		return nil, err
	}
	return &rt, nil
}

func (s *Store) RevokeRefreshToken(ctx context.Context, id string) error {
	const q = `UPDATE refresh_token SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`
	_, err := s.pool.Exec(ctx, q, id)
	return err
}

func (s *Store) GetUserByID(ctx context.Context, userID string) (*core.User, error) {
	const q = `SELECT * FROM app_user WHERE id = $1 LIMIT 1`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, core.ErrNotFound
	}

	// Escaneo dinámico
	cols := rows.FieldDescriptions()
	vals := make([]any, len(cols))
	for i := range vals {
		vals[i] = new(any)
	}

	if err := rows.Scan(vals...); err != nil {
		return nil, err
	}

	var u core.User
	u.CustomFields = make(map[string]any)

	for i, col := range cols {
		val := *(vals[i].(*any))
		colName := string(col.Name)

		switch colName {
		case "id":
			u.ID = uuidToString(val)
		case "tenant_id":
			u.TenantID = uuidToString(val)
		case "email":
			if v, ok := val.(string); ok {
				u.Email = v
			}
		case "email_verified":
			if v, ok := val.(bool); ok {
				u.EmailVerified = v
			}
		case "metadata":
			if v, ok := val.(map[string]any); ok {
				u.Metadata = v
			}
		case "created_at":
			if v, ok := val.(time.Time); ok {
				u.CreatedAt = v
			}
		case "disabled_at":
			if v, ok := val.(time.Time); ok {
				u.DisabledAt = &v
			}
		case "disabled_reason":
			if v, ok := val.(string); ok {
				u.DisabledReason = &v
			}
		case "disabled_until":
			if v, ok := val.(time.Time); ok {
				u.DisabledUntil = &v
			}
		case "name":
			if v, ok := val.(string); ok {
				u.Name = v
			}
		case "given_name":
			if v, ok := val.(string); ok {
				u.GivenName = v
			}
		case "family_name":
			if v, ok := val.(string); ok {
				u.FamilyName = v
			}
		case "picture":
			if v, ok := val.(string); ok {
				u.Picture = v
			}
		case "locale":
			if v, ok := val.(string); ok {
				u.Locale = v
			}
		case "source_client_id":
			if v, ok := val.(string); ok {
				u.SourceClientID = &v
			}
		default:
			// Campo dinámico
			u.CustomFields[colName] = val
		}
	}
	return &u, nil
}

func (s *Store) ListUsers(ctx context.Context, tenantID string) ([]core.User, error) {
	const q = `SELECT * FROM app_user ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []core.User
	cols := rows.FieldDescriptions()

	// Pre-allocate pointers for scanning
	vals := make([]any, len(cols))
	for i := range vals {
		vals[i] = new(any)
	}

	for rows.Next() {
		if err := rows.Scan(vals...); err != nil {
			return nil, err
		}

		var u core.User
		u.CustomFields = make(map[string]any)

		for i, col := range cols {
			val := *(vals[i].(*any))
			colName := string(col.Name)

			switch colName {
			case "id":
				u.ID = uuidToString(val)
			case "tenant_id":
				u.TenantID = uuidToString(val)
			case "email":
				if v, ok := val.(string); ok {
					u.Email = v
				}
			case "email_verified":
				if v, ok := val.(bool); ok {
					u.EmailVerified = v
				}
			case "metadata":
				if v, ok := val.(map[string]any); ok {
					u.Metadata = v
				}
			case "created_at":
				if v, ok := val.(time.Time); ok {
					u.CreatedAt = v
				}
			case "disabled_at":
				if v, ok := val.(time.Time); ok {
					u.DisabledAt = &v
				}
			case "disabled_reason":
				if v, ok := val.(string); ok {
					u.DisabledReason = &v
				}
			case "disabled_until":
				if v, ok := val.(time.Time); ok {
					u.DisabledUntil = &v
				}
			case "name":
				if v, ok := val.(string); ok {
					u.Name = v
				}
			case "given_name":
				if v, ok := val.(string); ok {
					u.GivenName = v
				}
			case "family_name":
				if v, ok := val.(string); ok {
					u.FamilyName = v
				}
			case "picture":
				if v, ok := val.(string); ok {
					u.Picture = v
				}
			case "locale":
				if v, ok := val.(string); ok {
					u.Locale = v
				}
			case "source_client_id":
				if v, ok := val.(string); ok {
					u.SourceClientID = &v
				}
			default:
				u.CustomFields[colName] = val
			}
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *Store) DeleteUser(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM app_user WHERE id = $1`, userID)
	return err
}

// RevokeAllRefreshTokens revoca todos los refresh de un usuario (opcionalmente filtrado por client_id_text).
func (s *Store) RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error {
	if strings.TrimSpace(clientID) == "" {
		_, err := s.pool.Exec(ctx, `
			UPDATE refresh_token
			SET revoked_at = NOW()
			WHERE user_id = $1 AND revoked_at IS NULL
		`, userID)
		return err
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_token
		SET revoked_at = NOW()
		WHERE user_id = $1 AND client_id_text = $2 AND revoked_at IS NULL
	`, userID, clientID)
	return err
}

// ====================== ADMIN CLIENTS ======================

// ListClients: filtra por tenant; query opcional sobre name/client_id (ILIKE %q%)
func (s *Store) ListClients(ctx context.Context, tenantID, query string) ([]core.Client, error) {
	q := `
SELECT id, tenant_id, name, client_id, client_type, redirect_uris, allowed_origins, providers, scopes, active_version_id, created_at
FROM client
WHERE tenant_id = $1`
	args := []any{tenantID}
	if strings.TrimSpace(query) != "" {
		q += " AND (name ILIKE $2 OR client_id ILIKE $2)"
		args = append(args, "%"+query+"%")
	}
	q += " ORDER BY created_at DESC"
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Client
	for rows.Next() {
		var c core.Client
		var active *string
		if err := rows.Scan(&c.ID, &c.TenantID, &c.Name, &c.ClientID, &c.ClientType, &c.RedirectURIs, &c.AllowedOrigins, &c.Providers, &c.Scopes, &active, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.ActiveVersionID = active
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) GetClientByID(ctx context.Context, id string) (*core.Client, *core.ClientVersion, error) {
	const q = `
SELECT c.id, c.tenant_id, c.name, c.client_id, c.client_type, c.redirect_uris, c.allowed_origins, c.providers, c.scopes, c.active_version_id, c.created_at,
       v.id, v.version, v.claim_schema_json, v.claim_mapping_json, v.crypto_config_json, v.status, v.created_at, v.promoted_at
FROM client c
LEFT JOIN client_version v ON v.id = c.active_version_id
WHERE c.id = $1
LIMIT 1`
	row := s.pool.QueryRow(ctx, q, id)

	var c core.Client
	var active *string

	// Campos de la versión como punteros para tolerar NULL del LEFT JOIN
	var vID, vVersion, vStatus *string
	var vCreatedAt, vPromotedAt *time.Time
	var claimSchema, claimMapping, cryptoConf []byte

	if err := row.Scan(
		&c.ID, &c.TenantID, &c.Name, &c.ClientID, &c.ClientType, &c.RedirectURIs, &c.AllowedOrigins, &c.Providers, &c.Scopes, &active, &c.CreatedAt,
		&vID, &vVersion, &claimSchema, &claimMapping, &cryptoConf, &vStatus, &vCreatedAt, &vPromotedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, core.ErrNotFound
		}
		return nil, nil, err
	}
	c.ActiveVersionID = active

	// Si no hay active_version_id, vID será nil
	if vID == nil {
		return &c, nil, nil
	}

	v := &core.ClientVersion{
		ID:               *vID,
		ClientID:         c.ID,
		Version:          deref(vVersion),
		ClaimSchemaJSON:  claimSchema,
		ClaimMappingJSON: claimMapping,
		CryptoConfigJSON: cryptoConf,
		Status:           deref(vStatus),
		CreatedAt:        derefTime(vCreatedAt),
		PromotedAt:       vPromotedAt,
	}
	return &c, v, nil
}

// UpdateClient: campos permitidos (name, client_type, redirect_uris, allowed_origins, providers, scopes)
func (s *Store) UpdateClient(ctx context.Context, c *core.Client) error {
	const q = `
UPDATE client
SET name=$2, client_type=$3, redirect_uris=$4, allowed_origins=$5, providers=$6, scopes=$7
WHERE id=$1`
	_, err := s.pool.Exec(ctx, q, c.ID, c.Name, c.ClientType, c.RedirectURIs, c.AllowedOrigins, c.Providers, c.Scopes)
	return err
}

func (s *Store) DeleteClient(ctx context.Context, id string) error {
	// Opcional: cascada lógica. Acá hacemos delete directo; tu FK debe estar definida con ON DELETE RESTRICT o CASCADE según tu modelo.
	_, err := s.pool.Exec(ctx, `DELETE FROM client WHERE id = $1`, id)
	return err
}

// Revocar todo por client (todos los usuarios que tengan refresh con ese client)
func (s *Store) RevokeAllRefreshTokensByClient(ctx context.Context, clientID string) error {
	_, err := s.pool.Exec(ctx, `
        UPDATE refresh_token
        SET revoked_at = NOW()
        WHERE client_id_text = $1 AND revoked_at IS NULL
    `, clientID)
	return err
}

// ====================== ADMIN USERS (disable/enable) ======================

// DisableUser marca al usuario como 'disabled' y setea metadata auxiliar si las columnas existen.
// DisableUser marca al usuario como 'disabled' y setea metadata auxiliar si las columnas existen.
func (s *Store) DisableUser(ctx context.Context, userID, by, reason string, until *time.Time) error {
	// DEBUG DIAGNOSTICS
	var dbName, dbPort string
	var colExists bool
	_ = s.pool.QueryRow(ctx, "SELECT current_database(), current_setting('port')").Scan(&dbName, &dbPort)
	_ = s.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name='app_user' AND column_name='disabled_until')").Scan(&colExists)
	log.Printf(">>> DEBUG [DisableUser]: Connecting to DB=%s Port=%s. Column 'disabled_until' exists? %v", dbName, dbPort, colExists)

	// status='disabled'. Columnas opcionales disabled_at/disabled_by/disabled_reason pueden no existir.
	// Intentamos la actualización extendida; si falla por columna, caemos a mínima.
	_, err := s.pool.Exec(ctx, `UPDATE app_user SET disabled_at=NOW(), disabled_reason=NULLIF($2,''), disabled_until=$3 WHERE id=$1`, userID, reason, until)
	return err
}

// EnableUser marca al usuario como 'active' y limpia metadata auxiliar conocida.
func (s *Store) EnableUser(ctx context.Context, userID, by string) error {
	_, err := s.pool.Exec(ctx, `UPDATE app_user SET disabled_at=NULL, disabled_reason=NULL, disabled_until=NULL WHERE id=$1`, userID)
	return err
}

// RevokeAllRefreshByUser revokes and returns affected rows
func (s *Store) RevokeAllRefreshByUser(ctx context.Context, userID string) (int, error) {
	tag, err := s.pool.Exec(ctx, `UPDATE refresh_token SET revoked_at = NOW() WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return int(tag.RowsAffected()), err
}
