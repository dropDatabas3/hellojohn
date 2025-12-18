// Package pg implementa el adapter PostgreSQL para store/v2.
// Usa pgxpool directamente, sin dependencias de store/v1.
package pg

import (
	"context"
	"fmt"
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
func (c *pgConnection) Tenants() repository.TenantRepository { return nil }
func (c *pgConnection) Clients() repository.ClientRepository { return nil }
func (c *pgConnection) Keys() repository.KeyRepository       { return nil } // Keys viven en FS

// ─── UserRepository ───

type userRepo struct{ pool *pgxpool.Pool }

func (r *userRepo) GetByEmail(ctx context.Context, tenantID, email string) (*repository.User, *repository.Identity, error) {
	const query = `
		SELECT u.id, u.tenant_id, u.email, u.email_verified, u.name, u.given_name, u.family_name,
		       u.picture, u.locale, u.created_at, u.metadata, u.custom_fields,
		       u.disabled_at, u.disabled_until, u.disabled_reason,
		       i.id, i.provider, i.provider_user_id, i.email, i.email_verified, i.password_hash, i.created_at
		FROM app_user u
		LEFT JOIN identity i ON i.user_id = u.id AND i.provider = 'password'
		WHERE u.tenant_id = $1 AND u.email = $2
		LIMIT 1
	`

	var user repository.User
	var identity repository.Identity
	var pwdHash *string
	var metadata, customFields []byte

	err := r.pool.QueryRow(ctx, query, tenantID, email).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.EmailVerified,
		&user.Name, &user.GivenName, &user.FamilyName, &user.Picture, &user.Locale,
		&user.CreatedAt, &metadata, &customFields,
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
	const query = `
		SELECT id, tenant_id, email, email_verified, name, given_name, family_name,
		       picture, locale, created_at, disabled_at, disabled_until, disabled_reason
		FROM app_user WHERE id = $1
	`

	var user repository.User
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.EmailVerified,
		&user.Name, &user.GivenName, &user.FamilyName, &user.Picture, &user.Locale,
		&user.CreatedAt, &user.DisabledAt, &user.DisabledUntil, &user.DisabledReason,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("pg: get user by id: %w", err)
	}

	return &user, nil
}

func (r *userRepo) Create(ctx context.Context, input repository.CreateUserInput) (*repository.User, *repository.Identity, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("pg: begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Crear usuario
	user := &repository.User{
		TenantID:  input.TenantID,
		Email:     input.Email,
		CreatedAt: time.Now(),
	}

	const insertUser = `
		INSERT INTO app_user (tenant_id, email, email_verified, created_at)
		VALUES ($1, $2, false, $3)
		RETURNING id
	`
	err = tx.QueryRow(ctx, insertUser, user.TenantID, user.Email, user.CreatedAt).Scan(&user.ID)
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
	const query = `
		UPDATE app_user SET
			name = COALESCE($2, name),
			given_name = COALESCE($3, given_name),
			family_name = COALESCE($4, family_name),
			picture = COALESCE($5, picture),
			locale = COALESCE($6, locale)
		WHERE id = $1
	`
	_, err := r.pool.Exec(ctx, query, userID,
		input.Name, input.GivenName, input.FamilyName, input.Picture, input.Locale)
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

	// Query base con orden estable
	baseQuery := `
		SELECT id, tenant_id, email, email_verified, name, given_name, family_name,
		       picture, locale, created_at, disabled_at, disabled_until, disabled_reason
		FROM app_user
		WHERE tenant_id = $1
	`

	var args []any
	args = append(args, tenantID)

	// Búsqueda opcional por email o nombre
	if filter.Search != "" {
		baseQuery += ` AND (email ILIKE $2 OR name ILIKE $2)`
		args = append(args, "%"+filter.Search+"%")
		baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $3 OFFSET $4")
		args = append(args, limit, offset)
	} else {
		baseQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $2 OFFSET $3")
		args = append(args, limit, offset)
	}

	rows, err := r.pool.Query(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("pg: list users: %w", err)
	}
	defer rows.Close()

	var users []repository.User
	for rows.Next() {
		var u repository.User
		if err := rows.Scan(
			&u.ID, &u.TenantID, &u.Email, &u.EmailVerified,
			&u.Name, &u.GivenName, &u.FamilyName, &u.Picture, &u.Locale,
			&u.CreatedAt, &u.DisabledAt, &u.DisabledUntil, &u.DisabledReason,
		); err != nil {
			return nil, fmt.Errorf("pg: scan user: %w", err)
		}
		users = append(users, u)
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

	// Borrar identities, tokens, consents
	_, _ = tx.Exec(ctx, `DELETE FROM identity WHERE user_id = $1`, userID)
	_, _ = tx.Exec(ctx, `DELETE FROM refresh_token WHERE user_id = $1`, userID)
	_, _ = tx.Exec(ctx, `DELETE FROM user_consent WHERE user_id = $1`, userID)
	_, _ = tx.Exec(ctx, `DELETE FROM mfa_totp WHERE user_id = $1`, userID)
	_, _ = tx.Exec(ctx, `DELETE FROM mfa_recovery_code WHERE user_id = $1`, userID)
	_, _ = tx.Exec(ctx, `DELETE FROM mfa_trusted_device WHERE user_id = $1`, userID)
	_, _ = tx.Exec(ctx, `DELETE FROM user_role WHERE user_id = $1`, userID)

	// Borrar usuario
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
	const query = `
		INSERT INTO refresh_token (user_id, client_id_text, tenant_id, token_hash, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW() + $5::interval)
		RETURNING id
	`
	ttl := fmt.Sprintf("%d seconds", input.TTLSeconds)
	var id string
	err := r.pool.QueryRow(ctx, query,
		input.UserID, input.ClientID, input.TenantID, input.TokenHash, ttl,
	).Scan(&id)
	return id, err
}

func (r *tokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.RefreshToken, error) {
	const query = `
		SELECT id, user_id, client_id_text, tenant_id, token_hash, issued_at, expires_at, rotated_from, revoked_at
		FROM refresh_token WHERE token_hash = $1
	`
	var token repository.RefreshToken
	err := r.pool.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID, &token.UserID, &token.ClientID, &token.TenantID,
		&token.TokenHash, &token.IssuedAt, &token.ExpiresAt, &token.RotatedFrom, &token.RevokedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
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
