package server

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	appv2 "github.com/dropDatabas3/hellojohn/internal/app/v2"
	cache "github.com/dropDatabas3/hellojohn/internal/cache/v2"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane/v2"
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	emailv2 "github.com/dropDatabas3/hellojohn/internal/email/v2"
	oauth "github.com/dropDatabas3/hellojohn/internal/http/v2/services/oauth"
	socialsvc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/social"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
	migrations "github.com/dropDatabas3/hellojohn/migrations/postgres"
)

// BuildV2Handler builds the HTTP V2 handler with all dependencies wired.
// This function acts as the main entry point for the HTTP server to get the V2 handler.
// It instantiates dependencies (DAL, etc.) if they are not provided, or uses stubs for now.
func BuildV2Handler() (http.Handler, func() error, error) {
	handler, cleanup, _, err := buildV2HandlerInternal()
	return handler, cleanup, err
}

// BuildV2HandlerWithDeps builds the HTTP V2 handler and returns DAL for bootstrap
func BuildV2HandlerWithDeps() (http.Handler, func() error, store.DataAccessLayer, error) {
	return buildV2HandlerInternal()
}

// buildV2HandlerInternal is the internal implementation that returns all dependencies
func buildV2HandlerInternal() (http.Handler, func() error, store.DataAccessLayer, error) {
	ctx := context.Background()

	// 1. Config (Environment)
	// TODO: Use a proper config loader if available
	fsRoot := os.Getenv("FS_ROOT")
	if fsRoot == "" {
		fsRoot = "data"
	}
	masterKey := os.Getenv("SIGNING_MASTER_KEY")
	if len(masterKey) < 32 {
		return nil, nil, nil, fmt.Errorf("SIGNING_MASTER_KEY must be at least 32 bytes")
	}
	v2BaseURL := os.Getenv("V2_BASE_URL")
	if v2BaseURL == "" {
		v2BaseURL = "http://localhost:8082"
	}

	// 2. Data Store (DAL + Manager)
	manager, err := store.NewManager(ctx, store.ManagerConfig{
		FSRoot:           fsRoot,
		SigningMasterKey: masterKey,
		Logger:           log.Default(), // Use standard logger for store debug
		// Migraciones per-tenant embebidas
		MigrationsFS:  migrations.TenantFS,
		MigrationsDir: migrations.TenantDir,
		// DB configs could be loaded here if needed for Hybrid mode
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to init store manager: %w", err)
	}

	// Cleanup for store
	cleanup := func() error {
		return manager.Close()
	}

	// 3. Keys & Issuer
	// Access keys via ConfigAccess (FS adapter)
	keyRepo := manager.ConfigAccess().Keys()

	// Keystore wrapper
	persistentKS := jwtx.NewPersistentKeystore(keyRepo)

	// Ensure minimal bootstrap (creates global keys if missing)
	if err := persistentKS.EnsureBootstrap(ctx); err != nil {
		_ = cleanup()
		return nil, nil, nil, fmt.Errorf("keystore bootstrap failed: %w", err)
	}

	issuer := jwtx.NewIssuer(v2BaseURL, persistentKS)

	// 3.5. JWKS Cache (15s TTL)
	// El loader recibe slug/ID del tenant y resuelve las keys
	jwksCache := jwtx.NewJWKSCache(15*time.Second, func(slugOrID string) (json.RawMessage, error) {
		// Global JWKS
		if strings.TrimSpace(slugOrID) == "" || strings.EqualFold(slugOrID, "global") {
			b, err := persistentKS.JWKSJSON()
			if err != nil {
				return nil, err
			}
			return json.RawMessage(b), nil
		}

		// Tenant JWKS - necesitamos resolver el tenant primero
		// porque JWKSJSONForTenant espera el slug exacto
		tda, err := manager.ForTenant(ctx, slugOrID)
		if err != nil {
			// Si el tenant no existe, retornar error
			return nil, fmt.Errorf("tenant not found: %w", err)
		}

		// Obtener JWKS usando el slug resuelto
		b, err := persistentKS.JWKSJSONForTenant(tda.Slug())
		if err != nil {
			return nil, err
		}
		return json.RawMessage(b), nil
	})

	// 4. Control Plane Service
	cpService := cp.NewService(manager)

	// 5. Email Service (V2)
	// Use separate key for encryption/decryption (not signing key)
	emailKey := os.Getenv("SECRETBOX_MASTER_KEY")
	if emailKey == "" {
		emailKey = os.Getenv("EMAIL_MASTER_KEY")
	}

	if _, err := validateSecretBoxKey(emailKey, "SECRETBOX_MASTER_KEY"); err != nil {
		_ = cleanup()
		return nil, nil, nil, err
	}

	emailService, err := emailv2.NewService(emailv2.ServiceConfig{
		DAL:       manager,
		MasterKey: emailKey,
		BaseURL:   v2BaseURL,
		VerifyTTL: 48 * time.Hour,
		ResetTTL:  1 * time.Hour,
	})
	if err != nil {
		_ = cleanup()
		return nil, nil, nil, fmt.Errorf("email v2 init failed: %w", err)
	}

	// 6. Social Cache (Stub/Real?)
	// Usually dependent on Redis. V2 Store Manager has Cache(), but SocialCache interface might differ.
	// Keeping NoOp for safety.
	socialCache := &NoOpSocialCache{}

	// 7. Dependencies Struct
	deps := appv2.Deps{
		DAL:          manager,
		ControlPlane: cpService,
		Email:        emailService,
		Issuer:       issuer,
		JWKSCache:    jwksCache,
		BaseIssuer:   v2BaseURL,
		RefreshTTL:   30 * 24 * time.Hour, // 30 days default
		SocialCache:  socialCache,
		MasterKey:    masterKey,
		RateLimiter:  nil, // Optional, can be added if needed
		// Auth Config
		AutoLogin:      getenvBool("REGISTER_AUTO_LOGIN", true),
		FSAdminEnabled: getenvBool("FS_ADMIN_ENABLE", false),
		Social: socialsvc.NewServices(socialsvc.Deps{
			DAL:            manager,
			Cache:          socialsvc.NewCacheAdapter(cache.NewMemory("social")),
			DebugPeek:      getenvBool("SOCIAL_DEBUG_PEEK", false),
			Issuer:         issuer,
			RefreshTTL:     24 * time.Hour * 30, // Default 30 days
			LoginCodeTTL:   60 * time.Second,
			TenantProvider: cpService,
			OIDCFactory:    socialsvc.NewOIDCFactory(cpService),
			// ConfiguredProviders: Load from config/env
		}),
		// OAuth
		OAuthCache:       oauth.NewCacheAdapter(cache.NewMemory("oauth")),
		OAuthCookieName:  "sid", // Default
		OAuthAllowBearer: true,  // Default V1 behavior
	}

	// 8. Build App (Router, Controllers)
	app, err := appv2.New(appv2.Config{}, deps)
	if err != nil {
		_ = cleanup()
		return nil, nil, nil, fmt.Errorf("failed to build v2 app: %w", err)
	}

	return app.Handler, cleanup, manager, nil
}

func getenvBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func validateSecretBoxKey(val, name string) (string, error) {
	val = strings.TrimSpace(val)
	if val == "" {
		return "", fmt.Errorf("%s required", name)
	}
	// Try Base64 (Std)
	if b, err := base64.StdEncoding.DecodeString(val); err == nil && len(b) == 32 {
		return val, nil
	}
	// Try Base64 (Raw)
	if b, err := base64.RawStdEncoding.DecodeString(val); err == nil && len(b) == 32 {
		return val, nil
	}
	// Try Hex
	if len(val) == 64 {
		if b, err := hex.DecodeString(val); err == nil && len(b) == 32 {
			return val, nil
		}
	}
	// Try Raw
	if len(val) == 32 {
		return val, nil
	}
	return "", fmt.Errorf("%s must be 32 bytes (base64 std/raw, hex 64 chars, or raw 32 chars)", name)
}

// ─── Stubs for Compilation (mocks) ───

type NoOpEmailService struct{}

func (s *NoOpEmailService) GetSender(ctx context.Context, tenantSlugOrID string) (emailv2.Sender, error) {
	return nil, nil
}
func (s *NoOpEmailService) SendVerificationEmail(ctx context.Context, req emailv2.SendVerificationRequest) error {
	return nil
}
func (s *NoOpEmailService) SendPasswordResetEmail(ctx context.Context, req emailv2.SendPasswordResetRequest) error {
	return nil
}
func (s *NoOpEmailService) SendNotificationEmail(ctx context.Context, req emailv2.SendNotificationRequest) error {
	return nil
}
func (s *NoOpEmailService) TestSMTP(ctx context.Context, tenantSlugOrID, recipientEmail string, override *emailv2.SMTPConfig) error {
	return nil
}

// NoOpSocialCache implements social.CacheWriter
type NoOpSocialCache struct{}

func (c *NoOpSocialCache) Get(key string) ([]byte, bool) {
	return nil, false
}
func (c *NoOpSocialCache) Set(key string, value []byte, ttl time.Duration) {
	// No-op
}
func (c *NoOpSocialCache) Delete(key string) error {
	return nil
}

// MockKeyRepository implements repository.KeyRepository
type MockKeyRepository struct{}

func (m *MockKeyRepository) GetActive(ctx context.Context, tenantID string) (*repository.SigningKey, error) {
	// Return a static key for testing
	pub, priv, _ := ed25519.GenerateKey(nil)
	return &repository.SigningKey{
		ID:         "mock-kid",
		Algorithm:  "EdDSA",
		PrivateKey: priv,
		PublicKey:  pub,
		Status:     repository.KeyStatusActive,
		CreatedAt:  time.Now(),
	}, nil
}
func (m *MockKeyRepository) GetByKID(ctx context.Context, kid string) (*repository.SigningKey, error) {
	return m.GetActive(ctx, "")
}
func (m *MockKeyRepository) GetJWKS(ctx context.Context, tenantID string) (*repository.JWKS, error) {
	return &repository.JWKS{Keys: []repository.JWK{}}, nil
}
func (m *MockKeyRepository) Generate(ctx context.Context, tenantID, algorithm string) (*repository.SigningKey, error) {
	return m.GetActive(ctx, "")
}
func (m *MockKeyRepository) Rotate(ctx context.Context, tenantID string, gracePeriod time.Duration) (*repository.SigningKey, error) {
	return m.GetActive(ctx, "")
}
func (m *MockKeyRepository) Revoke(ctx context.Context, kid string) error { return nil }

func (m *MockKeyRepository) ToEdDSA(key *repository.SigningKey) (ed25519.PrivateKey, error) {
	if k, ok := key.PrivateKey.(ed25519.PrivateKey); ok {
		return k, nil
	}
	return nil, fmt.Errorf("invalid key type")
}
func (m *MockKeyRepository) ToECDSA(key *repository.SigningKey) (*ecdsa.PrivateKey, error) {
	return nil, fmt.Errorf("not implemented")
}

// MockControlPlane implements cp.Service
type MockControlPlane struct{}

func (m *MockControlPlane) ListTenants(ctx context.Context) ([]repository.Tenant, error) {
	return nil, nil
}
func (m *MockControlPlane) GetTenant(ctx context.Context, slug string) (*repository.Tenant, error) {
	return nil, nil
}
func (m *MockControlPlane) GetTenantByID(ctx context.Context, id string) (*repository.Tenant, error) {
	return nil, nil
}
func (m *MockControlPlane) CreateTenant(ctx context.Context, name, slug, language string) (*repository.Tenant, error) {
	return nil, nil
}
func (m *MockControlPlane) UpdateTenant(ctx context.Context, tenant *repository.Tenant) error {
	return nil
}
func (m *MockControlPlane) DeleteTenant(ctx context.Context, slug string) error { return nil }
func (m *MockControlPlane) UpdateTenantSettings(ctx context.Context, slug string, settings *repository.TenantSettings) error {
	return nil
}
func (m *MockControlPlane) ListClients(ctx context.Context, slug string) ([]repository.Client, error) {
	return nil, nil
}
func (m *MockControlPlane) GetClient(ctx context.Context, slug, clientID string) (*repository.Client, error) {
	return nil, nil
}
func (m *MockControlPlane) CreateClient(ctx context.Context, slug string, input cp.ClientInput) (*repository.Client, error) {
	return nil, nil
}
func (m *MockControlPlane) UpdateClient(ctx context.Context, slug string, input cp.ClientInput) (*repository.Client, error) {
	return nil, nil
}
func (m *MockControlPlane) DeleteClient(ctx context.Context, slug, clientID string) error { return nil }
func (m *MockControlPlane) DecryptClientSecret(ctx context.Context, slug, clientID string) (string, error) {
	return "", nil
}
func (m *MockControlPlane) ListScopes(ctx context.Context, slug string) ([]repository.Scope, error) {
	return nil, nil
}
func (m *MockControlPlane) CreateScope(ctx context.Context, slug, name, description string) (*repository.Scope, error) {
	return nil, nil
}
func (m *MockControlPlane) DeleteScope(ctx context.Context, slug, name string) error { return nil }
func (m *MockControlPlane) ValidateClientID(id string) bool                          { return true }
func (m *MockControlPlane) ValidateRedirectURI(uri string) bool                      { return true }
func (m *MockControlPlane) IsScopeAllowed(client *repository.Client, scope string) bool {
	return true
}

// ─── Additional Stubs for Login Service ───

// ─── Additional Stubs for Login Service ───

// MockDAL implements store.DataAccessLayer
type MockDAL struct{}

func (m *MockDAL) ForTenant(ctx context.Context, slugOrID string) (store.TenantDataAccess, error) {
	return &MockTDA{slug: slugOrID}, nil
}
func (m *MockDAL) ConfigAccess() store.ConfigAccess { return nil }
func (m *MockDAL) Mode() store.OperationalMode      { return store.ModeFSTenantDB }
func (m *MockDAL) Capabilities() store.ModeCapabilities {
	return store.GetCapabilities(store.ModeFSTenantDB)
}
func (m *MockDAL) Stats() store.FactoryStats { return store.FactoryStats{} }
func (m *MockDAL) MigrateTenant(ctx context.Context, slugOrID string) (*store.MigrationResult, error) {
	return nil, nil
}
func (m *MockDAL) Close() error { return nil }

// MockTDA implements store.TenantDataAccess
type MockTDA struct {
	slug string
}

func (m *MockTDA) Slug() string { return m.slug }
func (m *MockTDA) ID() string   { return m.slug }
func (m *MockTDA) Settings() *repository.TenantSettings {
	return &repository.TenantSettings{IssuerMode: "path"}
}
func (m *MockTDA) Driver() string { return "mock" }

func (m *MockTDA) Users() repository.UserRepository                                { return &MockUserRepo{} }
func (m *MockTDA) Tokens() repository.TokenRepository                              { return &MockTokenRepo{} }
func (m *MockTDA) Clients() repository.ClientRepository                            { return &MockClientRepo{} }
func (m *MockTDA) MFA() repository.MFARepository                                   { return &MockMFARepo{} }
func (m *MockTDA) Consents() repository.ConsentRepository                          { return nil }
func (m *MockTDA) RBAC() repository.RBACRepository                                 { return nil }
func (m *MockTDA) Schema() repository.SchemaRepository                             { return nil }
func (m *MockTDA) EmailTokens() repository.EmailTokenRepository                    { return nil }
func (m *MockTDA) Identities() repository.IdentityRepository                       { return nil }
func (m *MockTDA) Scopes() repository.ScopeRepository                              { return nil }
func (m *MockTDA) Cache() cache.Client                                             { return nil }
func (m *MockTDA) CacheRepo() repository.CacheRepository                           { return nil }
func (m *MockTDA) Mailer() store.MailSender                                        { return nil }
func (m *MockTDA) InfraStats(ctx context.Context) (*store.TenantInfraStats, error) { return nil, nil }
func (m *MockTDA) HasDB() bool                                                     { return true }
func (m *MockTDA) RequireDB() error                                                { return nil }

// MockUserRepo
type MockUserRepo struct{}

func (m *MockUserRepo) GetByEmail(ctx context.Context, tenantID, email string) (*repository.User, *repository.Identity, error) {
	return nil, nil, repository.ErrNotFound
}
func (m *MockUserRepo) GetByID(ctx context.Context, userID string) (*repository.User, error) {
	return nil, repository.ErrNotFound
}
func (m *MockUserRepo) CheckPassword(hash *string, password string) bool { return false }
func (m *MockUserRepo) Create(ctx context.Context, input repository.CreateUserInput) (*repository.User, *repository.Identity, error) {
	return nil, nil, repository.ErrNotImplemented
}
func (m *MockUserRepo) Update(ctx context.Context, userID string, input repository.UpdateUserInput) error {
	return repository.ErrNotImplemented
}
func (m *MockUserRepo) Disable(ctx context.Context, userID, by, reason string, until *time.Time) error {
	return repository.ErrNotImplemented
}
func (m *MockUserRepo) Enable(ctx context.Context, userID, by string) error {
	return repository.ErrNotImplemented
}
func (m *MockUserRepo) SetEmailVerified(ctx context.Context, userID string, verified bool) error {
	return repository.ErrNotImplemented
}
func (m *MockUserRepo) UpdatePasswordHash(ctx context.Context, userID, newHash string) error {
	return repository.ErrNotImplemented
}
func (m *MockUserRepo) List(ctx context.Context, tenantID string, filter repository.ListUsersFilter) ([]repository.User, error) {
	return nil, repository.ErrNotImplemented
}
func (m *MockUserRepo) Delete(ctx context.Context, userID string) error {
	return repository.ErrNotImplemented
}

// MockTokenRepo
type MockTokenRepo struct{}

func (m *MockTokenRepo) Create(ctx context.Context, input repository.CreateRefreshTokenInput) (string, error) {
	return "mock-refresh-token", nil
}
func (m *MockTokenRepo) GetByHash(ctx context.Context, tokenHash string) (*repository.RefreshToken, error) {
	return nil, repository.ErrNotFound
}
func (m *MockTokenRepo) Revoke(ctx context.Context, tokenID string) error { return nil }
func (m *MockTokenRepo) RevokeAllByUser(ctx context.Context, userID, clientID string) (int, error) {
	return 0, nil
}
func (m *MockTokenRepo) RevokeAllByClient(ctx context.Context, clientID string) error { return nil }

// MockClientRepo
type MockClientRepo struct{}

func (m *MockClientRepo) Get(ctx context.Context, tenantID, clientID string) (*repository.Client, error) {
	return nil, repository.ErrNotFound
}
func (m *MockClientRepo) GetByUUID(ctx context.Context, uuid string) (*repository.Client, *repository.ClientVersion, error) {
	return nil, nil, repository.ErrNotFound
}
func (m *MockClientRepo) List(ctx context.Context, tenantID string, query string) ([]repository.Client, error) {
	return nil, nil
}
func (m *MockClientRepo) Create(ctx context.Context, tenantID string, input repository.ClientInput) (*repository.Client, error) {
	return nil, nil
}
func (m *MockClientRepo) Update(ctx context.Context, tenantID string, input repository.ClientInput) (*repository.Client, error) {
	return nil, nil
}
func (m *MockClientRepo) Delete(ctx context.Context, tenantID, clientID string) error { return nil }
func (m *MockClientRepo) DecryptSecret(ctx context.Context, tenantID, clientID string) (string, error) {
	return "", nil
}
func (m *MockClientRepo) ValidateClientID(id string) bool     { return true }
func (m *MockClientRepo) ValidateRedirectURI(uri string) bool { return true }
func (m *MockClientRepo) IsScopeAllowed(client *repository.Client, scope string) bool {
	return true
}

// MockMFARepo
type MockMFARepo struct{}

func (m *MockMFARepo) UpsertTOTP(ctx context.Context, userID, secretEnc string) error { return nil }
func (m *MockMFARepo) ConfirmTOTP(ctx context.Context, userID string) error           { return nil }
func (m *MockMFARepo) GetTOTP(ctx context.Context, userID string) (*repository.MFATOTP, error) {
	return nil, repository.ErrNotFound
}
func (m *MockMFARepo) UpdateTOTPUsedAt(ctx context.Context, userID string) error { return nil }
func (m *MockMFARepo) DisableTOTP(ctx context.Context, userID string) error      { return nil }
func (m *MockMFARepo) SetRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	return nil
}
func (m *MockMFARepo) DeleteRecoveryCodes(ctx context.Context, userID string) error { return nil }
func (m *MockMFARepo) UseRecoveryCode(ctx context.Context, userID, hash string) (bool, error) {
	return false, nil
}
func (m *MockMFARepo) AddTrustedDevice(ctx context.Context, userID, deviceHash string, expiresAt time.Time) error {
	return nil
}
func (m *MockMFARepo) IsTrustedDevice(ctx context.Context, userID, deviceHash string) (bool, error) {
	return false, nil
}
