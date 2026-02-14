package store

// nodb_repos.go contiene implementaciones stub de todos los repositorios Data Plane.
// Se usan cuando un tenant tiene DB configurada pero la conexión falló (lazy connection).
// Todos los métodos retornan ErrNoDBForTenant en lugar de causar nil pointer panics.

import (
	"context"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
)

// ─── UserRepository (no-DB) ───

type noDBUserRepo struct{}

func (r *noDBUserRepo) GetByEmail(ctx context.Context, tenantID, email string) (*repository.User, *repository.Identity, error) {
	return nil, nil, ErrNoDBForTenant
}
func (r *noDBUserRepo) GetByID(ctx context.Context, userID string) (*repository.User, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBUserRepo) List(ctx context.Context, tenantID string, filter repository.ListUsersFilter) ([]repository.User, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBUserRepo) Create(ctx context.Context, input repository.CreateUserInput) (*repository.User, *repository.Identity, error) {
	return nil, nil, ErrNoDBForTenant
}
func (r *noDBUserRepo) Update(ctx context.Context, userID string, input repository.UpdateUserInput) error {
	return ErrNoDBForTenant
}
func (r *noDBUserRepo) Delete(ctx context.Context, userID string) error {
	return ErrNoDBForTenant
}
func (r *noDBUserRepo) Disable(ctx context.Context, userID, by, reason string, until *time.Time) error {
	return ErrNoDBForTenant
}
func (r *noDBUserRepo) Enable(ctx context.Context, userID, by string) error {
	return ErrNoDBForTenant
}
func (r *noDBUserRepo) CheckPassword(hash *string, password string) bool {
	return false
}
func (r *noDBUserRepo) SetEmailVerified(ctx context.Context, userID string, verified bool) error {
	return ErrNoDBForTenant
}
func (r *noDBUserRepo) UpdatePasswordHash(ctx context.Context, userID, newHash string) error {
	return ErrNoDBForTenant
}

// ─── TokenRepository (no-DB) ───

type noDBTokenRepo struct{}

func (r *noDBTokenRepo) Create(ctx context.Context, input repository.CreateRefreshTokenInput) (string, error) {
	return "", ErrNoDBForTenant
}
func (r *noDBTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.RefreshToken, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBTokenRepo) GetByID(ctx context.Context, tokenID string) (*repository.RefreshToken, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBTokenRepo) Revoke(ctx context.Context, tokenID string) error {
	return ErrNoDBForTenant
}
func (r *noDBTokenRepo) RevokeAllByUser(ctx context.Context, userID, clientID string) (int, error) {
	return 0, ErrNoDBForTenant
}
func (r *noDBTokenRepo) RevokeAllByClient(ctx context.Context, clientID string) error {
	return ErrNoDBForTenant
}
func (r *noDBTokenRepo) List(ctx context.Context, filter repository.ListTokensFilter) ([]repository.RefreshToken, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBTokenRepo) Count(ctx context.Context, filter repository.ListTokensFilter) (int, error) {
	return 0, ErrNoDBForTenant
}
func (r *noDBTokenRepo) RevokeAll(ctx context.Context) (int, error) {
	return 0, ErrNoDBForTenant
}
func (r *noDBTokenRepo) GetStats(ctx context.Context) (*repository.TokenStats, error) {
	return nil, ErrNoDBForTenant
}

// ─── MFARepository (no-DB) ───

type noDBMFARepo struct{}

func (r *noDBMFARepo) UpsertTOTP(ctx context.Context, userID, secretEnc string) error {
	return ErrNoDBForTenant
}
func (r *noDBMFARepo) ConfirmTOTP(ctx context.Context, userID string) error {
	return ErrNoDBForTenant
}
func (r *noDBMFARepo) GetTOTP(ctx context.Context, userID string) (*repository.MFATOTP, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBMFARepo) UpdateTOTPUsedAt(ctx context.Context, userID string) error {
	return ErrNoDBForTenant
}
func (r *noDBMFARepo) DisableTOTP(ctx context.Context, userID string) error {
	return ErrNoDBForTenant
}
func (r *noDBMFARepo) SetRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	return ErrNoDBForTenant
}
func (r *noDBMFARepo) DeleteRecoveryCodes(ctx context.Context, userID string) error {
	return ErrNoDBForTenant
}
func (r *noDBMFARepo) UseRecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	return false, ErrNoDBForTenant
}
func (r *noDBMFARepo) AddTrustedDevice(ctx context.Context, userID, deviceHash string, expiresAt time.Time) error {
	return ErrNoDBForTenant
}
func (r *noDBMFARepo) IsTrustedDevice(ctx context.Context, userID, deviceHash string) (bool, error) {
	return false, ErrNoDBForTenant
}

// ─── ConsentRepository (no-DB) ───

type noDBConsentRepo struct{}

func (r *noDBConsentRepo) Upsert(ctx context.Context, tenantID, userID, clientID string, scopes []string) (*repository.Consent, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBConsentRepo) Get(ctx context.Context, tenantID, userID, clientID string) (*repository.Consent, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBConsentRepo) ListByUser(ctx context.Context, tenantID, userID string, activeOnly bool) ([]repository.Consent, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBConsentRepo) ListAll(ctx context.Context, tenantID string, limit, offset int, activeOnly bool) ([]repository.Consent, int, error) {
	return nil, 0, ErrNoDBForTenant
}
func (r *noDBConsentRepo) Revoke(ctx context.Context, tenantID, userID, clientID string) error {
	return ErrNoDBForTenant
}

// ─── RBACRepository (no-DB) ───

type noDBRBACRepo struct{}

func (r *noDBRBACRepo) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBRBACRepo) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBRBACRepo) AssignRole(ctx context.Context, tenantID, userID, role string) error {
	return ErrNoDBForTenant
}
func (r *noDBRBACRepo) RemoveRole(ctx context.Context, tenantID, userID, role string) error {
	return ErrNoDBForTenant
}
func (r *noDBRBACRepo) GetRolePermissions(ctx context.Context, tenantID, role string) ([]string, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBRBACRepo) AddPermissionToRole(ctx context.Context, tenantID, role, permission string) error {
	return ErrNoDBForTenant
}
func (r *noDBRBACRepo) RemovePermissionFromRole(ctx context.Context, tenantID, role, permission string) error {
	return ErrNoDBForTenant
}
func (r *noDBRBACRepo) ListRoles(ctx context.Context, tenantID string) ([]repository.Role, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBRBACRepo) GetRole(ctx context.Context, tenantID, name string) (*repository.Role, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBRBACRepo) CreateRole(ctx context.Context, tenantID string, input repository.RoleInput) (*repository.Role, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBRBACRepo) UpdateRole(ctx context.Context, tenantID, name string, input repository.RoleInput) (*repository.Role, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBRBACRepo) DeleteRole(ctx context.Context, tenantID, name string) error {
	return ErrNoDBForTenant
}
func (r *noDBRBACRepo) GetRoleUsersCount(ctx context.Context, tenantID, role string) (int, error) {
	return 0, ErrNoDBForTenant
}

// ─── SchemaRepository (no-DB) ───

type noDBSchemaRepo struct{}

func (r *noDBSchemaRepo) SyncUserFields(ctx context.Context, tenantID string, fields []repository.UserFieldDefinition) error {
	return ErrNoDBForTenant
}
func (r *noDBSchemaRepo) EnsureIndexes(ctx context.Context, tenantID string, schemaDef map[string]any) error {
	return ErrNoDBForTenant
}
func (r *noDBSchemaRepo) IntrospectColumns(ctx context.Context, tenantID, tableName string) ([]repository.ColumnInfo, error) {
	return nil, ErrNoDBForTenant
}

// ─── EmailTokenRepository (no-DB) ───

type noDBEmailTokenRepo struct{}

func (r *noDBEmailTokenRepo) Create(ctx context.Context, input repository.CreateEmailTokenInput) (*repository.EmailToken, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBEmailTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.EmailToken, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBEmailTokenRepo) Use(ctx context.Context, tokenHash string) error {
	return ErrNoDBForTenant
}
func (r *noDBEmailTokenRepo) DeleteExpired(ctx context.Context) (int, error) {
	return 0, ErrNoDBForTenant
}

// ─── IdentityRepository (no-DB) ───

type noDBIdentityRepo struct{}

func (r *noDBIdentityRepo) GetByProvider(ctx context.Context, tenantID, provider, providerUserID string) (*repository.SocialIdentity, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBIdentityRepo) GetByUserID(ctx context.Context, userID string) ([]repository.SocialIdentity, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBIdentityRepo) Upsert(ctx context.Context, input repository.UpsertSocialIdentityInput) (string, bool, error) {
	return "", false, ErrNoDBForTenant
}
func (r *noDBIdentityRepo) Link(ctx context.Context, userID string, input repository.UpsertSocialIdentityInput) (*repository.SocialIdentity, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBIdentityRepo) Unlink(ctx context.Context, userID, provider string) error {
	return ErrNoDBForTenant
}
func (r *noDBIdentityRepo) UpdateClaims(ctx context.Context, identityID string, claims map[string]any) error {
	return ErrNoDBForTenant
}

// ─── SessionRepository (no-DB) ───

type noDBSessionRepo struct{}

func (r *noDBSessionRepo) Create(ctx context.Context, input repository.CreateSessionInput) (*repository.Session, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBSessionRepo) Get(ctx context.Context, sessionIDHash string) (*repository.Session, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBSessionRepo) GetByIDHash(ctx context.Context, sessionIDHash string) (*repository.Session, error) {
	return nil, ErrNoDBForTenant
}
func (r *noDBSessionRepo) UpdateActivity(ctx context.Context, sessionIDHash string, lastActivity time.Time) error {
	return ErrNoDBForTenant
}
func (r *noDBSessionRepo) List(ctx context.Context, filter repository.ListSessionsFilter) ([]repository.Session, int, error) {
	return nil, 0, ErrNoDBForTenant
}
func (r *noDBSessionRepo) Revoke(ctx context.Context, sessionIDHash, revokedBy, reason string) error {
	return ErrNoDBForTenant
}
func (r *noDBSessionRepo) RevokeAllByUser(ctx context.Context, userID, revokedBy, reason string) (int, error) {
	return 0, ErrNoDBForTenant
}
func (r *noDBSessionRepo) RevokeAll(ctx context.Context, revokedBy, reason string) (int, error) {
	return 0, ErrNoDBForTenant
}
func (r *noDBSessionRepo) DeleteExpired(ctx context.Context) (int, error) {
	return 0, ErrNoDBForTenant
}
func (r *noDBSessionRepo) GetStats(ctx context.Context) (*repository.SessionStats, error) {
	return nil, ErrNoDBForTenant
}

// ─── Singleton instances (no allocation per request) ───

var (
	noDBUsers      repository.UserRepository       = &noDBUserRepo{}
	noDBTokens     repository.TokenRepository      = &noDBTokenRepo{}
	noDBMFA        repository.MFARepository        = &noDBMFARepo{}
	noDBConsents   repository.ConsentRepository     = &noDBConsentRepo{}
	noDBRBAC       repository.RBACRepository        = &noDBRBACRepo{}
	noDBSchema     repository.SchemaRepository      = &noDBSchemaRepo{}
	noDBEmailTkns  repository.EmailTokenRepository  = &noDBEmailTokenRepo{}
	noDBIdentities repository.IdentityRepository    = &noDBIdentityRepo{}
	noDBSessions   repository.SessionRepository     = &noDBSessionRepo{}
)
