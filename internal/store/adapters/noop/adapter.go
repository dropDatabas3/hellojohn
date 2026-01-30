// Package noop implementa el adapter no-op para modo sin DB.
package noop

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

// El adapter noop NO se auto-registra porque es un fallback especial.
// Se usa explícitamente cuando no hay DB configurada.

type noopAdapter struct{}

// New retorna el adapter noop.
func New() store.Adapter {
	return &noopAdapter{}
}

func (a *noopAdapter) Name() string { return "noop" }

func (a *noopAdapter) Connect(ctx context.Context, cfg store.AdapterConfig) (store.AdapterConnection, error) {
	return &noopConnection{}, nil
}

type noopConnection struct{}

func (c *noopConnection) Name() string                   { return "noop" }
func (c *noopConnection) Ping(ctx context.Context) error { return nil }
func (c *noopConnection) Close() error                   { return nil }

// Todos los repos retornan ErrNoDatabase
func (c *noopConnection) Users() repository.UserRepository                           { return &noopUserRepo{} }
func (c *noopConnection) Tokens() repository.TokenRepository                         { return &noopTokenRepo{} }
func (c *noopConnection) MFA() repository.MFARepository                              { return &noopMFARepo{} }
func (c *noopConnection) Consents() repository.ConsentRepository                     { return &noopConsentRepo{} }
func (c *noopConnection) Scopes() repository.ScopeRepository                         { return &noopScopeRepo{} }
func (c *noopConnection) RBAC() repository.RBACRepository                            { return &noopRBACRepo{} }
func (c *noopConnection) Schema() repository.SchemaRepository                        { return &noopSchemaRepo{} }
func (c *noopConnection) Keys() repository.KeyRepository                             { return &noopKeyRepo{} }
func (c *noopConnection) Tenants() repository.TenantRepository                       { return nil }
func (c *noopConnection) Clients() repository.ClientRepository                       { return nil }
func (c *noopConnection) Admins() repository.AdminRepository                         { return nil }
func (c *noopConnection) AdminRefreshTokens() repository.AdminRefreshTokenRepository { return nil }
func (c *noopConnection) Claims() repository.ClaimRepository                         { return nil }
func (c *noopConnection) EmailTokens() repository.EmailTokenRepository               { return &noopEmailTokenRepo{} }
func (c *noopConnection) Identities() repository.IdentityRepository                  { return &noopIdentityRepo{} }
func (c *noopConnection) Sessions() repository.SessionRepository                     { return &noopSessionRepo{} }

// ─── Repos que retornan ErrNoDatabase ───

type noopUserRepo struct{}
type noopTokenRepo struct{}
type noopMFARepo struct{}
type noopConsentRepo struct{}
type noopScopeRepo struct{}
type noopRBACRepo struct{}
type noopSessionRepo struct{}

func (r *noopUserRepo) GetByEmail(ctx context.Context, tenantID, email string) (*repository.User, *repository.Identity, error) {
	return nil, nil, repository.ErrNoDatabase
}
func (r *noopUserRepo) GetByID(ctx context.Context, userID string) (*repository.User, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopUserRepo) Create(ctx context.Context, input repository.CreateUserInput) (*repository.User, *repository.Identity, error) {
	return nil, nil, repository.ErrNoDatabase
}
func (r *noopUserRepo) Update(ctx context.Context, userID string, input repository.UpdateUserInput) error {
	return repository.ErrNoDatabase
}
func (r *noopUserRepo) Disable(ctx context.Context, userID, by, reason string, until *time.Time) error {
	return repository.ErrNoDatabase
}
func (r *noopUserRepo) Enable(ctx context.Context, userID, by string) error {
	return repository.ErrNoDatabase
}
func (r *noopUserRepo) CheckPassword(hash *string, password string) bool { return false }
func (r *noopUserRepo) SetEmailVerified(ctx context.Context, userID string, verified bool) error {
	return repository.ErrNoDatabase
}
func (r *noopUserRepo) UpdatePasswordHash(ctx context.Context, userID, newHash string) error {
	return repository.ErrNoDatabase
}
func (r *noopUserRepo) List(ctx context.Context, tenantID string, filter repository.ListUsersFilter) ([]repository.User, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopUserRepo) Delete(ctx context.Context, userID string) error {
	return repository.ErrNoDatabase
}

func (r *noopTokenRepo) Create(ctx context.Context, input repository.CreateRefreshTokenInput) (string, error) {
	return "", repository.ErrNoDatabase
}
func (r *noopTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.RefreshToken, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopTokenRepo) Revoke(ctx context.Context, tokenID string) error {
	return repository.ErrNoDatabase
}
func (r *noopTokenRepo) RevokeAllByUser(ctx context.Context, userID, clientID string) (int, error) {
	return 0, repository.ErrNoDatabase
}
func (r *noopTokenRepo) RevokeAllByClient(ctx context.Context, clientID string) error {
	return repository.ErrNoDatabase
}
func (r *noopTokenRepo) GetByID(ctx context.Context, tokenID string) (*repository.RefreshToken, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopTokenRepo) List(ctx context.Context, filter repository.ListTokensFilter) ([]repository.RefreshToken, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopTokenRepo) Count(ctx context.Context, filter repository.ListTokensFilter) (int, error) {
	return 0, repository.ErrNoDatabase
}
func (r *noopTokenRepo) RevokeAll(ctx context.Context) (int, error) {
	return 0, repository.ErrNoDatabase
}
func (r *noopTokenRepo) GetStats(ctx context.Context) (*repository.TokenStats, error) {
	return nil, repository.ErrNoDatabase
}

func (r *noopMFARepo) UpsertTOTP(ctx context.Context, userID, secretEnc string) error {
	return repository.ErrNoDatabase
}
func (r *noopMFARepo) ConfirmTOTP(ctx context.Context, userID string) error {
	return repository.ErrNoDatabase
}
func (r *noopMFARepo) GetTOTP(ctx context.Context, userID string) (*repository.MFATOTP, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopMFARepo) UpdateTOTPUsedAt(ctx context.Context, userID string) error {
	return repository.ErrNoDatabase
}
func (r *noopMFARepo) DisableTOTP(ctx context.Context, userID string) error {
	return repository.ErrNoDatabase
}
func (r *noopMFARepo) SetRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	return repository.ErrNoDatabase
}
func (r *noopMFARepo) DeleteRecoveryCodes(ctx context.Context, userID string) error {
	return repository.ErrNoDatabase
}
func (r *noopMFARepo) UseRecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	return false, repository.ErrNoDatabase
}
func (r *noopMFARepo) AddTrustedDevice(ctx context.Context, userID, deviceHash string, expiresAt time.Time) error {
	return repository.ErrNoDatabase
}
func (r *noopMFARepo) IsTrustedDevice(ctx context.Context, userID, deviceHash string) (bool, error) {
	return false, repository.ErrNoDatabase
}

func (r *noopConsentRepo) Upsert(ctx context.Context, tenantID, userID, clientID string, scopes []string) (*repository.Consent, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopConsentRepo) Get(ctx context.Context, tenantID, userID, clientID string) (*repository.Consent, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopConsentRepo) ListByUser(ctx context.Context, tenantID, userID string, activeOnly bool) ([]repository.Consent, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopConsentRepo) ListAll(ctx context.Context, tenantID string, limit, offset int, activeOnly bool) ([]repository.Consent, int, error) {
	return nil, 0, repository.ErrNoDatabase
}
func (r *noopConsentRepo) Revoke(ctx context.Context, tenantID, userID, clientID string) error {
	return repository.ErrNoDatabase
}

func (r *noopScopeRepo) Create(ctx context.Context, tenantID string, input repository.ScopeInput) (*repository.Scope, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopScopeRepo) GetByName(ctx context.Context, tenantID, name string) (*repository.Scope, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopScopeRepo) List(ctx context.Context, tenantID string) ([]repository.Scope, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopScopeRepo) Update(ctx context.Context, tenantID string, input repository.ScopeInput) (*repository.Scope, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopScopeRepo) Delete(ctx context.Context, tenantID, scopeID string) error {
	return repository.ErrNoDatabase
}
func (r *noopScopeRepo) Upsert(ctx context.Context, tenantID string, input repository.ScopeInput) (*repository.Scope, error) {
	return nil, repository.ErrNoDatabase
}

func (r *noopRBACRepo) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopRBACRepo) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopRBACRepo) AssignRole(ctx context.Context, tenantID, userID, role string) error {
	return repository.ErrNoDatabase
}
func (r *noopRBACRepo) RemoveRole(ctx context.Context, tenantID, userID, role string) error {
	return repository.ErrNoDatabase
}
func (r *noopRBACRepo) GetRolePermissions(ctx context.Context, tenantID, role string) ([]string, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopRBACRepo) AddPermissionToRole(ctx context.Context, tenantID, role, permission string) error {
	return repository.ErrNoDatabase
}
func (r *noopRBACRepo) RemovePermissionFromRole(ctx context.Context, tenantID, role, permission string) error {
	return repository.ErrNoDatabase
}
func (r *noopRBACRepo) ListRoles(ctx context.Context, tenantID string) ([]repository.Role, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopRBACRepo) GetRole(ctx context.Context, tenantID, name string) (*repository.Role, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopRBACRepo) CreateRole(ctx context.Context, tenantID string, input repository.RoleInput) (*repository.Role, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopRBACRepo) UpdateRole(ctx context.Context, tenantID, name string, input repository.RoleInput) (*repository.Role, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopRBACRepo) DeleteRole(ctx context.Context, tenantID, name string) error {
	return repository.ErrNoDatabase
}
func (r *noopRBACRepo) GetRoleUsersCount(ctx context.Context, tenantID, role string) (int, error) {
	return 0, repository.ErrNoDatabase
}

// ─── EmailToken noop repo ───

type noopEmailTokenRepo struct{}

func (r *noopEmailTokenRepo) Create(ctx context.Context, input repository.CreateEmailTokenInput) (*repository.EmailToken, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopEmailTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.EmailToken, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopEmailTokenRepo) Use(ctx context.Context, tokenHash string) error {
	return repository.ErrNoDatabase
}
func (r *noopEmailTokenRepo) DeleteExpired(ctx context.Context) (int, error) {
	return 0, repository.ErrNoDatabase
}

// ─── Identity noop repo ───

type noopIdentityRepo struct{}

func (r *noopIdentityRepo) GetByProvider(ctx context.Context, tenantID, provider, providerUserID string) (*repository.SocialIdentity, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopIdentityRepo) GetByUserID(ctx context.Context, userID string) ([]repository.SocialIdentity, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopIdentityRepo) Upsert(ctx context.Context, input repository.UpsertSocialIdentityInput) (string, bool, error) {
	return "", false, repository.ErrNoDatabase
}
func (r *noopIdentityRepo) Link(ctx context.Context, userID string, input repository.UpsertSocialIdentityInput) (*repository.SocialIdentity, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopIdentityRepo) Unlink(ctx context.Context, userID, provider string) error {
	return repository.ErrNoDatabase
}
func (r *noopIdentityRepo) UpdateClaims(ctx context.Context, identityID string, claims map[string]any) error {
	return repository.ErrNoDatabase
}

// ─── Schema noop repo ───

type noopSchemaRepo struct{}

func (r *noopSchemaRepo) SyncUserFields(ctx context.Context, tenantID string, fields []repository.UserFieldDefinition) error {
	return nil
}
func (r *noopSchemaRepo) EnsureIndexes(ctx context.Context, tenantID string, schemaDef map[string]any) error {
	return nil
}
func (r *noopSchemaRepo) IntrospectColumns(ctx context.Context, tenantID, tableName string) ([]repository.ColumnInfo, error) {
	return nil, repository.ErrNoDatabase
}

// ─── Key noop repo ───

type noopKeyRepo struct{}

func (r *noopKeyRepo) GetActive(ctx context.Context, tenantID string) (*repository.SigningKey, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopKeyRepo) GetByKID(ctx context.Context, kid string) (*repository.SigningKey, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopKeyRepo) GetJWKS(ctx context.Context, tenantID string) (*repository.JWKS, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopKeyRepo) Generate(ctx context.Context, tenantID, algorithm string) (*repository.SigningKey, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopKeyRepo) Rotate(ctx context.Context, tenantID string, gracePeriod time.Duration) (*repository.SigningKey, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopKeyRepo) Revoke(ctx context.Context, kid string) error {
	return repository.ErrNoDatabase
}
func (r *noopKeyRepo) ListAll(ctx context.Context, tenantID string) ([]*repository.SigningKey, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopKeyRepo) ToEdDSA(key *repository.SigningKey) (ed25519.PrivateKey, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopKeyRepo) ToECDSA(key *repository.SigningKey) (*ecdsa.PrivateKey, error) {
	return nil, repository.ErrNoDatabase
}

// ─── Session noop repo ───

func (r *noopSessionRepo) Create(ctx context.Context, input repository.CreateSessionInput) (*repository.Session, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopSessionRepo) Get(ctx context.Context, sessionIDHash string) (*repository.Session, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopSessionRepo) UpdateActivity(ctx context.Context, sessionIDHash string, lastActivity time.Time) error {
	return repository.ErrNoDatabase
}
func (r *noopSessionRepo) List(ctx context.Context, filter repository.ListSessionsFilter) ([]repository.Session, int, error) {
	return nil, 0, repository.ErrNoDatabase
}
func (r *noopSessionRepo) Revoke(ctx context.Context, sessionIDHash, revokedBy, reason string) error {
	return repository.ErrNoDatabase
}
func (r *noopSessionRepo) RevokeAllByUser(ctx context.Context, userID, revokedBy, reason string) (int, error) {
	return 0, repository.ErrNoDatabase
}
func (r *noopSessionRepo) RevokeAll(ctx context.Context, revokedBy, reason string) (int, error) {
	return 0, repository.ErrNoDatabase
}
func (r *noopSessionRepo) DeleteExpired(ctx context.Context) (int, error) {
	return 0, repository.ErrNoDatabase
}
func (r *noopSessionRepo) GetByIDHash(ctx context.Context, sessionIDHash string) (*repository.Session, error) {
	return nil, repository.ErrNoDatabase
}
func (r *noopSessionRepo) GetStats(ctx context.Context) (*repository.SessionStats, error) {
	return nil, repository.ErrNoDatabase
}
