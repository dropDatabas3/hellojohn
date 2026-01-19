// repos_extended.go — MFA, Consent, Scope, RBAC repositories
package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
)

// ─── MFARepository ───

type mfaRepo struct{ pool *pgxpool.Pool }

func (r *mfaRepo) UpsertTOTP(ctx context.Context, userID, secretEnc string) error {
	const query = `
		INSERT INTO mfa_totp (user_id, secret_encrypted, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (user_id) DO UPDATE SET secret_encrypted = $2, updated_at = NOW()
	`
	_, err := r.pool.Exec(ctx, query, userID, secretEnc)
	return err
}

func (r *mfaRepo) ConfirmTOTP(ctx context.Context, userID string) error {
	const query = `UPDATE mfa_totp SET confirmed_at = NOW(), updated_at = NOW() WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *mfaRepo) GetTOTP(ctx context.Context, userID string) (*repository.MFATOTP, error) {
	const query = `
		SELECT user_id, secret_encrypted, confirmed_at, last_used_at, created_at, updated_at
		FROM mfa_totp WHERE user_id = $1
	`
	var mfa repository.MFATOTP
	err := r.pool.QueryRow(ctx, query, userID).Scan(
		&mfa.UserID, &mfa.SecretEncrypted, &mfa.ConfirmedAt, &mfa.LastUsedAt, &mfa.CreatedAt, &mfa.UpdatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	return &mfa, err
}

func (r *mfaRepo) UpdateTOTPUsedAt(ctx context.Context, userID string) error {
	const query = `UPDATE mfa_totp SET last_used_at = NOW(), updated_at = NOW() WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *mfaRepo) DisableTOTP(ctx context.Context, userID string) error {
	const query = `DELETE FROM mfa_totp WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *mfaRepo) SetRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Eliminar anteriores
	_, err = tx.Exec(ctx, `DELETE FROM mfa_recovery_code WHERE user_id = $1`, userID)
	if err != nil {
		return err
	}

	// Insertar nuevos
	for _, hash := range hashes {
		_, err = tx.Exec(ctx,
			`INSERT INTO mfa_recovery_code (user_id, code_hash, created_at) VALUES ($1, $2, NOW())`,
			userID, hash)
		if err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func (r *mfaRepo) DeleteRecoveryCodes(ctx context.Context, userID string) error {
	const query = `DELETE FROM mfa_recovery_code WHERE user_id = $1`
	_, err := r.pool.Exec(ctx, query, userID)
	return err
}

func (r *mfaRepo) UseRecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	const query = `
		DELETE FROM mfa_recovery_code WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
	`
	tag, err := r.pool.Exec(ctx, query, userID, hash)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (r *mfaRepo) AddTrustedDevice(ctx context.Context, userID, deviceHash string, expiresAt time.Time) error {
	const query = `
		INSERT INTO mfa_trusted_device (user_id, device_hash, expires_at, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (user_id, device_hash) DO UPDATE SET expires_at = $3
	`
	_, err := r.pool.Exec(ctx, query, userID, deviceHash, expiresAt)
	return err
}

func (r *mfaRepo) IsTrustedDevice(ctx context.Context, userID, deviceHash string) (bool, error) {
	const query = `
		SELECT EXISTS (
			SELECT 1 FROM mfa_trusted_device
			WHERE user_id = $1 AND device_hash = $2 AND expires_at > NOW()
		)
	`
	var exists bool
	err := r.pool.QueryRow(ctx, query, userID, deviceHash).Scan(&exists)
	return exists, err
}

// ─── ConsentRepository ───

type consentRepo struct{ pool *pgxpool.Pool }

func (r *consentRepo) Upsert(ctx context.Context, tenantID, userID, clientID string, scopes []string) (*repository.Consent, error) {
	const query = `
		INSERT INTO user_consent (tenant_id, user_id, client_id, scopes, granted_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (user_id, client_id) DO UPDATE SET scopes = $4, updated_at = NOW(), revoked_at = NULL
		RETURNING id, granted_at, updated_at
	`
	consent := &repository.Consent{
		TenantID: tenantID,
		UserID:   userID,
		ClientID: clientID,
		Scopes:   scopes,
	}
	err := r.pool.QueryRow(ctx, query, tenantID, userID, clientID, scopes).Scan(
		&consent.ID, &consent.GrantedAt, &consent.UpdatedAt,
	)
	return consent, err
}

func (r *consentRepo) Get(ctx context.Context, tenantID, userID, clientID string) (*repository.Consent, error) {
	const query = `
		SELECT id, tenant_id, user_id, client_id, scopes, granted_at, updated_at, revoked_at
		FROM user_consent WHERE user_id = $1 AND client_id = $2
	`
	var consent repository.Consent
	err := r.pool.QueryRow(ctx, query, userID, clientID).Scan(
		&consent.ID, &consent.TenantID, &consent.UserID, &consent.ClientID,
		&consent.Scopes, &consent.GrantedAt, &consent.UpdatedAt, &consent.RevokedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	return &consent, err
}

func (r *consentRepo) ListByUser(ctx context.Context, tenantID, userID string, activeOnly bool) ([]repository.Consent, error) {
	var query string
	if activeOnly {
		query = `SELECT id, tenant_id, user_id, client_id, scopes, granted_at, updated_at, revoked_at
		         FROM user_consent WHERE user_id = $1 AND revoked_at IS NULL ORDER BY granted_at DESC`
	} else {
		query = `SELECT id, tenant_id, user_id, client_id, scopes, granted_at, updated_at, revoked_at
		         FROM user_consent WHERE user_id = $1 ORDER BY granted_at DESC`
	}

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var consents []repository.Consent
	for rows.Next() {
		var c repository.Consent
		if err := rows.Scan(&c.ID, &c.TenantID, &c.UserID, &c.ClientID, &c.Scopes, &c.GrantedAt, &c.UpdatedAt, &c.RevokedAt); err != nil {
			return nil, err
		}
		consents = append(consents, c)
	}
	return consents, rows.Err()
}

func (r *consentRepo) Revoke(ctx context.Context, tenantID, userID, clientID string) error {
	const query = `UPDATE user_consent SET revoked_at = NOW() WHERE user_id = $1 AND client_id = $2`
	_, err := r.pool.Exec(ctx, query, userID, clientID)
	return err
}

// ─── ScopeRepository ───

type scopeRepo struct{ pool *pgxpool.Pool }

func (r *scopeRepo) Create(ctx context.Context, tenantID, name, description string) (*repository.Scope, error) {
	const query = `
		INSERT INTO scope (tenant_id, name, description, created_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, created_at
	`
	scope := &repository.Scope{
		TenantID:    tenantID,
		Name:        name,
		Description: description,
	}
	err := r.pool.QueryRow(ctx, query, tenantID, name, description).Scan(&scope.ID, &scope.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("pg: create scope: %w", err)
	}
	return scope, nil
}

func (r *scopeRepo) GetByName(ctx context.Context, tenantID, name string) (*repository.Scope, error) {
	const query = `
		SELECT id, tenant_id, name, description, system, created_at
		FROM scope WHERE tenant_id = $1 AND name = $2
	`
	var scope repository.Scope
	err := r.pool.QueryRow(ctx, query, tenantID, name).Scan(
		&scope.ID, &scope.TenantID, &scope.Name, &scope.Description, &scope.System, &scope.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, repository.ErrNotFound
	}
	return &scope, err
}

func (r *scopeRepo) List(ctx context.Context, tenantID string) ([]repository.Scope, error) {
	const query = `
		SELECT id, tenant_id, name, description, system, created_at
		FROM scope WHERE tenant_id = $1 ORDER BY name
	`
	rows, err := r.pool.Query(ctx, query, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scopes []repository.Scope
	for rows.Next() {
		var s repository.Scope
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.System, &s.CreatedAt); err != nil {
			return nil, err
		}
		scopes = append(scopes, s)
	}
	return scopes, rows.Err()
}

func (r *scopeRepo) UpdateDescription(ctx context.Context, tenantID, scopeID, description string) error {
	const query = `UPDATE scope SET description = $3 WHERE tenant_id = $1 AND id = $2`
	_, err := r.pool.Exec(ctx, query, tenantID, scopeID, description)
	return err
}

func (r *scopeRepo) Delete(ctx context.Context, tenantID, scopeID string) error {
	const query = `DELETE FROM scope WHERE tenant_id = $1 AND id = $2`
	_, err := r.pool.Exec(ctx, query, tenantID, scopeID)
	return err
}

func (r *scopeRepo) Upsert(ctx context.Context, tenantID, name, description string) (*repository.Scope, error) {
	const query = `
		INSERT INTO scope (tenant_id, name, description, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (tenant_id, name) DO UPDATE SET description = $3
		RETURNING id, created_at
	`
	scope := &repository.Scope{
		TenantID:    tenantID,
		Name:        name,
		Description: description,
	}
	err := r.pool.QueryRow(ctx, query, tenantID, name, description).Scan(&scope.ID, &scope.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("pg: upsert scope: %w", err)
	}
	return scope, nil
}

// ─── RBACRepository ───

type rbacRepo struct{ pool *pgxpool.Pool }

func (r *rbacRepo) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	const query = `SELECT role FROM user_role WHERE user_id = $1`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	return roles, rows.Err()
}

func (r *rbacRepo) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	const query = `
		SELECT DISTINCT rp.permission
		FROM user_role ur
		JOIN role_permission rp ON rp.role = ur.role
		WHERE ur.user_id = $1
	`
	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return perms, rows.Err()
}

func (r *rbacRepo) AssignRole(ctx context.Context, tenantID, userID, role string) error {
	const query = `
		INSERT INTO user_role (tenant_id, user_id, role, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT DO NOTHING
	`
	_, err := r.pool.Exec(ctx, query, tenantID, userID, role)
	return err
}

func (r *rbacRepo) RemoveRole(ctx context.Context, tenantID, userID, role string) error {
	const query = `DELETE FROM user_role WHERE tenant_id = $1 AND user_id = $2 AND role = $3`
	_, err := r.pool.Exec(ctx, query, tenantID, userID, role)
	return err
}

func (r *rbacRepo) GetRolePermissions(ctx context.Context, tenantID, role string) ([]string, error) {
	const query = `SELECT permission FROM role_permission WHERE tenant_id = $1 AND role = $2`
	rows, err := r.pool.Query(ctx, query, tenantID, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var perms []string
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err != nil {
			return nil, err
		}
		perms = append(perms, perm)
	}
	return perms, rows.Err()
}

func (r *rbacRepo) AddPermissionToRole(ctx context.Context, tenantID, role, permission string) error {
	const query = `
		INSERT INTO role_permission (tenant_id, role, permission, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT DO NOTHING
	`
	_, err := r.pool.Exec(ctx, query, tenantID, role, permission)
	return err
}

func (r *rbacRepo) RemovePermissionFromRole(ctx context.Context, tenantID, role, permission string) error {
	const query = `DELETE FROM role_permission WHERE tenant_id = $1 AND role = $2 AND permission = $3`
	_, err := r.pool.Exec(ctx, query, tenantID, role, permission)
	return err
}
