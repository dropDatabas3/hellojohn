// Package pg implementa el adapter PostgreSQL para store/v2.
// Usa pgxpool directamente, sin dependencias de store/v1.
package pg

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

func init() {
	store.RegisterAdapter(&postgresAdapter{})
}

// nullIfEmpty returns nil if the string is empty, otherwise returns the string pointer.
// Useful for inserting optional string fields into PostgreSQL.
func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// pgIdentifier sanitizes a string to be used as a PostgreSQL identifier.
// Only allows alphanumeric characters and underscores.
var validIdentifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// pgIdentifier normalizes and validates a string to be used as a PostgreSQL identifier.
// Converts spaces to underscores, removes accents, and validates the result.
func pgIdentifier(name string) string {
	// Normalize: lowercase, trim, replace spaces with underscores
	name = strings.ToLower(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, " ", "_")

	// Remove accents and special characters (keep only a-z, 0-9, _)
	var normalized strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z':
			normalized.WriteRune(r)
		case r >= '0' && r <= '9':
			normalized.WriteRune(r)
		case r == '_':
			normalized.WriteRune(r)
		// Common accented characters -> base letter
		case r == 'á' || r == 'à' || r == 'ä' || r == 'â' || r == 'ã':
			normalized.WriteRune('a')
		case r == 'é' || r == 'è' || r == 'ë' || r == 'ê':
			normalized.WriteRune('e')
		case r == 'í' || r == 'ì' || r == 'ï' || r == 'î':
			normalized.WriteRune('i')
		case r == 'ó' || r == 'ò' || r == 'ö' || r == 'ô' || r == 'õ':
			normalized.WriteRune('o')
		case r == 'ú' || r == 'ù' || r == 'ü' || r == 'û':
			normalized.WriteRune('u')
		case r == 'ñ':
			normalized.WriteRune('n')
		case r == 'ç':
			normalized.WriteRune('c')
			// Skip other characters
		}
	}
	name = normalized.String()

	// Ensure it doesn't start with a number
	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		name = "_" + name
	}

	// Final validation
	if !validIdentifier.MatchString(name) || name == "" {
		return ""
	}
	return name
}

// isSystemColumn returns true if the column is a system column (not a custom field).
func isSystemColumn(name string) bool {
	switch name {
	case "id", "email", "email_verified", "status", "profile", "metadata",
		"disabled_at", "disabled_reason", "disabled_until",
		"created_at", "updated_at", "password_hash",
		"name", "given_name", "family_name", "picture", "locale", "language", "source_client_id":
		return true
	}
	return false
}

// postgresAdapter implementa store.Adapter para PostgreSQL.
type postgresAdapter struct{}

func (a *postgresAdapter) Name() string { return "postgres" }

func (a *postgresAdapter) Connect(ctx context.Context, cfg store.AdapterConfig) (store.AdapterConnection, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("pg: parse DSN: %w", err)
	}

	// Configurar pool
	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	} else {
		poolCfg.MaxConns = 10
	}
	if cfg.MaxIdleConns > 0 {
		poolCfg.MinConns = int32(cfg.MaxIdleConns)
	} else {
		poolCfg.MinConns = 2
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("pg: create pool: %w", err)
	}

	// Verificar conexión
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pg: ping failed: %w", err)
	}

	return &pgConnection{pool: pool, schema: cfg.Schema}, nil
}

// pgConnection representa una conexión activa a PostgreSQL.
type pgConnection struct {
	pool   *pgxpool.Pool
	schema string
}

func (c *pgConnection) Name() string { return "postgres" }

func (c *pgConnection) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

func (c *pgConnection) Close() error {
	c.pool.Close()
	return nil
}

// ─── Repositorios ───

func (c *pgConnection) Users() repository.UserRepository       { return &userRepo{pool: c.pool} }
func (c *pgConnection) Tokens() repository.TokenRepository     { return &tokenRepo{pool: c.pool} }
func (c *pgConnection) MFA() repository.MFARepository          { return &mfaRepo{pool: c.pool} }
func (c *pgConnection) Consents() repository.ConsentRepository { return &consentRepo{pool: c.pool} }
func (c *pgConnection) Scopes() repository.ScopeRepository     { return &scopeRepo{pool: c.pool} }
func (c *pgConnection) RBAC() repository.RBACRepository        { return &rbacRepo{pool: c.pool} }
func (c *pgConnection) Schema() repository.SchemaRepository    { return &pgSchemaRepo{conn: c} }
func (c *pgConnection) EmailTokens() repository.EmailTokenRepository {
	return newEmailTokenRepo(c.pool)
}
func (c *pgConnection) Identities() repository.IdentityRepository { return newIdentityRepo(c.pool) }

// Control plane (no soportado por PG, viene de FS)
func (c *pgConnection) Tenants() repository.TenantRepository                       { return nil }
func (c *pgConnection) Clients() repository.ClientRepository                       { return nil }
func (c *pgConnection) Admins() repository.AdminRepository                         { return nil }
func (c *pgConnection) AdminRefreshTokens() repository.AdminRefreshTokenRepository { return nil }
func (c *pgConnection) Keys() repository.KeyRepository                             { return nil } // Keys viven en FS

// GetMigrationExecutor implementa store.MigratableConnection.
// Retorna un wrapper del pool para migraciones.
func (c *pgConnection) GetMigrationExecutor() store.PgxPoolExecutor {
	return &pgxPoolWrapper{pool: c.pool}
}

// pgxPoolWrapper adapta pgxpool.Pool a store.PgxPoolExecutor.
type pgxPoolWrapper struct {
	pool *pgxpool.Pool
}

func (w *pgxPoolWrapper) Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error) {
	return w.pool.Exec(ctx, sql, args...)
}

func (w *pgxPoolWrapper) QueryRow(ctx context.Context, sql string, args ...any) interface{ Scan(dest ...any) error } {
	return w.pool.QueryRow(ctx, sql, args...)
}

// ─── UserRepository ───

type userRepo struct{ pool *pgxpool.Pool }

func (r *userRepo) GetByEmail(ctx context.Context, tenantID, email string) (*repository.User, *repository.Identity, error) {
	// Nota: tenant_id no existe en la tabla porque cada tenant tiene su propia DB aislada
	// Custom fields are stored as dynamic columns, not as a JSON field
	const query = `
		SELECT u.id, u.email, u.email_verified, COALESCE(u.name, ''), COALESCE(u.given_name, ''), COALESCE(u.family_name, ''),
		       COALESCE(u.picture, ''), COALESCE(u.locale, ''), COALESCE(u.language, ''), u.source_client_id, u.created_at, u.metadata,
		       u.disabled_at, u.disabled_until, u.disabled_reason,
		       i.id, i.provider, i.provider_user_id, i.email, i.email_verified, i.password_hash, i.created_at
		FROM app_user u
		LEFT JOIN identity i ON i.user_id = u.id AND i.provider = 'password'
		WHERE u.email = $1
		LIMIT 1
	`

	var user repository.User
	var identity repository.Identity
	var pwdHash *string
	var metadata []byte

	// tenantID se usa para poblar el struct, no para filtrar (cada DB es de un tenant)
	user.TenantID = tenantID

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.EmailVerified,
		&user.Name, &user.GivenName, &user.FamilyName, &user.Picture, &user.Locale, &user.Language, &user.SourceClientID,
		&user.CreatedAt, &metadata,
		&user.DisabledAt, &user.DisabledUntil, &user.DisabledReason,
		&identity.ID, &identity.Provider, &identity.ProviderUserID,
		&identity.Email, &identity.EmailVerified, &pwdHash, &identity.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, nil, fmt.Errorf("pg: get user by email: %w", err)
	}

	identity.UserID = user.ID
	identity.PasswordHash = pwdHash

	return &user, &identity, nil
}

func (r *userRepo) GetByID(ctx context.Context, userID string) (*repository.User, error) {
	// Use SELECT * to get all columns including dynamic custom fields
	const query = `SELECT * FROM app_user WHERE id = $1`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("pg: get user by id: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, repository.ErrNotFound
	}

	user, err := r.scanUserRow(rows)
	if err != nil {
		return nil, fmt.Errorf("pg: scan user: %w", err)
	}

	return user, nil
}

// scanUserRow scans a user row with dynamic columns into a repository.User struct.
func (r *userRepo) scanUserRow(rows pgx.Rows) (*repository.User, error) {
	cols := rows.FieldDescriptions()
	vals := make([]any, len(cols))
	for i := range vals {
		vals[i] = new(any)
	}

	if err := rows.Scan(vals...); err != nil {
		return nil, err
	}

	var user repository.User
	user.CustomFields = make(map[string]any)

	for i, col := range cols {
		val := *(vals[i].(*any))
		if val == nil {
			continue
		}
		colName := string(col.Name)

		// Helper to convert UUID bytes to string
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
			user.ID = uuidToStr(val)
		case "email":
			if v, ok := val.(string); ok {
				user.Email = v
			}
		case "email_verified":
			if v, ok := val.(bool); ok {
				user.EmailVerified = v
			}
		case "name":
			if v, ok := val.(string); ok {
				user.Name = v
			}
		case "given_name":
			if v, ok := val.(string); ok {
				user.GivenName = v
			}
		case "family_name":
			if v, ok := val.(string); ok {
				user.FamilyName = v
			}
		case "picture":
			if v, ok := val.(string); ok {
				user.Picture = v
			}
		case "locale":
			if v, ok := val.(string); ok {
				user.Locale = v
			}
		case "language":
			if v, ok := val.(string); ok {
				user.Language = v
			}
		case "source_client_id":
			if v, ok := val.(string); ok {
				user.SourceClientID = &v
			}
		case "created_at":
			if v, ok := val.(time.Time); ok {
				user.CreatedAt = v
			}
		case "disabled_at":
			if v, ok := val.(time.Time); ok {
				user.DisabledAt = &v
			}
		case "disabled_until":
			if v, ok := val.(time.Time); ok {
				user.DisabledUntil = &v
			}
		case "disabled_reason":
			if v, ok := val.(string); ok {
				user.DisabledReason = &v
			}
		case "metadata":
			// Skip metadata column for now (used internally)
		default:
			// Dynamic custom field column
			user.CustomFields[colName] = val
		}
	}

	return &user, nil
}

func (r *userRepo) Create(ctx context.Context, input repository.CreateUserInput) (*repository.User, *repository.Identity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("pg: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Crear usuario (tenant_id se guarda en el struct pero no en la DB - cada DB es aislada por tenant)
	user := &repository.User{
		TenantID:     input.TenantID,
		Email:        input.Email,
		Name:         input.Name,
		GivenName:    input.GivenName,
		FamilyName:   input.FamilyName,
		Picture:      input.Picture,
		Locale:       input.Locale,
		CustomFields: input.CustomFields,
		CreatedAt:    time.Now(),
	}
	if input.SourceClientID != "" {
		user.SourceClientID = &input.SourceClientID
	}

	// Build dynamic INSERT query with custom fields
	cols := []string{"email", "email_verified", "name", "given_name", "family_name", "picture", "locale", "source_client_id", "created_at"}
	args := []any{user.Email, false, nullIfEmpty(input.Name), nullIfEmpty(input.GivenName), nullIfEmpty(input.FamilyName), nullIfEmpty(input.Picture), nullIfEmpty(input.Locale), user.SourceClientID, user.CreatedAt}

	// Add custom fields as dynamic columns (sorted for determinism)
	if len(input.CustomFields) > 0 {
		keys := make([]string, 0, len(input.CustomFields))
		for k := range input.CustomFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			colName := pgIdentifier(k)
			if colName == "" || isSystemColumn(colName) {
				continue // Skip invalid or system columns
			}
			cols = append(cols, colName)
			args = append(args, input.CustomFields[k])
		}
	}

	// Build parameterized query
	placeholders := make([]string, len(args))
	for i := range args {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}

	insertQuery := fmt.Sprintf(
		"INSERT INTO app_user (%s) VALUES (%s) RETURNING id",
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)

	err = tx.QueryRow(ctx, insertQuery, args...).Scan(&user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("pg: insert user: %w", err)
	}

	// Crear identity con password
	identity := &repository.Identity{
		UserID:       user.ID,
		Provider:     "password",
		Email:        input.Email,
		PasswordHash: &input.PasswordHash,
		CreatedAt:    time.Now(),
	}

	const insertIdentity = `
		INSERT INTO identity (user_id, provider, email, email_verified, password_hash, created_at)
		VALUES ($1, $2, $3, false, $4, $5)
		RETURNING id
	`
	err = tx.QueryRow(ctx, insertIdentity,
		identity.UserID, identity.Provider, identity.Email, input.PasswordHash, identity.CreatedAt,
	).Scan(&identity.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("pg: insert identity: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, nil, fmt.Errorf("pg: commit tx: %w", err)
	}

	return user, identity, nil
}

func (r *userRepo) Update(ctx context.Context, userID string, input repository.UpdateUserInput) error {
	// Build dynamic UPDATE query
	setClauses := []string{}
	args := []any{userID}
	argIdx := 2

	// System fields with COALESCE (only update if provided)
	if input.Name != nil {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", argIdx))
		args = append(args, *input.Name)
		argIdx++
	}
	if input.GivenName != nil {
		setClauses = append(setClauses, fmt.Sprintf("given_name = $%d", argIdx))
		args = append(args, *input.GivenName)
		argIdx++
	}
	if input.FamilyName != nil {
		setClauses = append(setClauses, fmt.Sprintf("family_name = $%d", argIdx))
		args = append(args, *input.FamilyName)
		argIdx++
	}
	if input.Picture != nil {
		setClauses = append(setClauses, fmt.Sprintf("picture = $%d", argIdx))
		args = append(args, *input.Picture)
		argIdx++
	}
	if input.Locale != nil {
		setClauses = append(setClauses, fmt.Sprintf("locale = $%d", argIdx))
		args = append(args, *input.Locale)
		argIdx++
	}
	if input.SourceClientID != nil {
		setClauses = append(setClauses, fmt.Sprintf("source_client_id = $%d", argIdx))
		// Handle empty string as NULL
		if *input.SourceClientID == "" {
			args = append(args, nil)
		} else {
			args = append(args, *input.SourceClientID)
		}
		argIdx++
	}

	// Custom fields as dynamic columns (sorted for determinism)
	if len(input.CustomFields) > 0 {
		keys := make([]string, 0, len(input.CustomFields))
		for k := range input.CustomFields {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			colName := pgIdentifier(k)
			if colName == "" || isSystemColumn(colName) {
				continue // Skip invalid or system columns
			}
			setClauses = append(setClauses, fmt.Sprintf("%s = $%d", colName, argIdx))
			args = append(args, input.CustomFields[k])
			argIdx++
		}
	}

	// If nothing to update, return early
	if len(setClauses) == 0 {
		return nil
	}

	query := fmt.Sprintf("UPDATE app_user SET %s WHERE id = $1", strings.Join(setClauses, ", "))
	_, err := r.pool.Exec(ctx, query, args...)
	return err
}

func (r *userRepo) Disable(ctx context.Context, userID, by, reason string, until *time.Time) error {
	const query = `
		UPDATE app_user SET
			disabled_at = NOW(),
			disabled_until = $2,
			disabled_reason = $3
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID, until, reason)
	return err
}

func (r *userRepo) Enable(ctx context.Context, userID, by string) error {
	const query = `
		UPDATE app_user SET
			disabled_at = NULL,
			disabled_until = NULL,
			disabled_reason = NULL
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *userRepo) CheckPassword(hash *string, password string) bool {
	if hash == nil {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(*hash), []byte(password)) == nil
}

func (r *userRepo) SetEmailVerified(ctx context.Context, userID string, verified bool) error {
	const query = `UPDATE app_user SET email_verified = $2 WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, userID, verified)
	return err
}

func (r *userRepo) UpdatePasswordHash(ctx context.Context, userID, newHash string) error {
	// Actualiza el password_hash en la identity "password" del usuario
	const query = `UPDATE identity SET password_hash = $2, updated_at = NOW() WHERE user_id = $1 AND provider = 'password'`
	tag, err := r.pool.Exec(ctx, query, userID, newHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}
	return nil
}

func (r *userRepo) List(ctx context.Context, tenantID string, filter repository.ListUsersFilter) ([]repository.User, error) {
	// Defaults y clamp
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	// Build query with SELECT * to include custom fields
	var baseQuery string
	var args []any

	if filter.Search != "" {
		baseQuery = `SELECT * FROM app_user WHERE (email ILIKE $1 OR name ILIKE $1) ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = []any{"%" + filter.Search + "%", limit, offset}
	} else {
		baseQuery = `SELECT * FROM app_user ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		args = []any{limit, offset}
	}

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("pg: list users: %w", err)
	}
	defer rows.Close()

	var users []repository.User
	for rows.Next() {
		user, err := r.scanUserRow(rows)
		if err != nil {
			return nil, fmt.Errorf("pg: scan user: %w", err)
		}
		user.TenantID = tenantID // Asignar desde parámetro ya que no está en la DB
		users = append(users, *user)
	}

	return users, rows.Err()
}

func (r *userRepo) Delete(ctx context.Context, userID string) error {
	// TX para borrar dependencias y usuario
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("pg: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Helper para ejecutar DELETE de forma segura (ignora errores de tabla inexistente)
	safeDelete := func(table string) {
		// Usamos DO block para manejar tabla inexistente sin abortar la transacción
		query := fmt.Sprintf(`
			DO $$
			BEGIN
				DELETE FROM %s WHERE user_id = '%s';
			EXCEPTION
				WHEN undefined_table THEN
					-- Tabla no existe, ignorar
					NULL;
			END $$;
		`, table, userID)
		_, _ = tx.Exec(ctx, query)
	}

	// Borrar dependencias de forma segura
	safeDelete("identity")
	safeDelete("refresh_token")
	safeDelete("user_consent")
	safeDelete("mfa_totp")
	safeDelete("mfa_recovery_code")
	safeDelete("mfa_trusted_device")
	safeDelete("user_role")

	// Borrar usuario (esta tabla debe existir)
	tag, err := tx.Exec(ctx, `DELETE FROM app_user WHERE id = $1`, userID)
	if err != nil {
		return fmt.Errorf("pg: delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return repository.ErrNotFound
	}

	return tx.Commit(ctx)
}

// ─── TokenRepository ───

type tokenRepo struct{ pool *pgxpool.Pool }

func (r *tokenRepo) Create(ctx context.Context, input repository.CreateRefreshTokenInput) (string, error) {
	// Note: tenant_id is not stored in DB since each tenant has isolated DB
	const query = `
		INSERT INTO refresh_token (user_id, client_id_text, token_hash, issued_at, expires_at)
		VALUES ($1, $2, $3, NOW(), NOW() + $4::interval)
		RETURNING id
	`
	ttl := fmt.Sprintf("%d seconds", input.TTLSeconds)
	var id string
	err := r.pool.QueryRow(ctx, query,
		input.UserID, input.ClientID, input.TokenHash, ttl,
	).Scan(&id)
	return id, err
}

func (r *tokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.RefreshToken, error) {
	const query = `
		SELECT id, user_id, client_id_text, token_hash, issued_at, expires_at, rotated_from, revoked_at
		FROM refresh_token WHERE token_hash = $1
	`
	var token repository.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID, &token.UserID, &token.ClientID,
		&token.TokenHash, &token.IssuedAt, &token.ExpiresAt, &token.RotatedFrom, &token.RevokedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	// TenantID should be set by the caller from the context/TDA
	return &token, err
}

func (r *tokenRepo) Revoke(ctx context.Context, tokenID string) error {
	const query = `UPDATE refresh_token SET revoked_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, tokenID)
	return err
}

func (r *tokenRepo) RevokeAllByUser(ctx context.Context, userID, clientID string) (int, error) {
	var query string
	var args []any
	if clientID != "" {
		query = `UPDATE refresh_token SET revoked_at = NOW() WHERE user_id = $1 AND client_id_text = $2 AND revoked_at IS NULL`
		args = []any{userID, clientID}
	} else {
		query = `UPDATE refresh_token SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`
		args = []any{userID}
	}
	tag, err := r.pool.Exec(ctx, query, args...)
	return int(tag.RowsAffected()), err
}

func (r *tokenRepo) RevokeAllByClient(ctx context.Context, clientID string) error {
	const query = `UPDATE refresh_token SET revoked_at = NOW() WHERE client_id_text = $1 AND revoked_at IS NULL`
	_, err := r.pool.Exec(ctx, query, clientID)
	return err
}
