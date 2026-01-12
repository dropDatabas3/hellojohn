package auth

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/domain/types"
	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/auth"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// RegisterService defines operations for user registration.
type RegisterService interface {
	Register(ctx context.Context, in dto.RegisterRequest) (*dto.RegisterResult, error)
}

// RegisterDeps contains dependencies for the register service.
type RegisterDeps struct {
	DAL           store.DataAccessLayer
	Issuer        *jwtx.Issuer
	RefreshTTL    time.Duration
	ClaimsHook    ClaimsHook
	BlacklistPath string
	AutoLogin     bool
	// FSAdminEnabled allows registration without tenant/client (FS-admin mode).
	FSAdminEnabled bool
}

type registerService struct {
	deps RegisterDeps
}

// NewRegisterService creates a new register service.
func NewRegisterService(deps RegisterDeps) RegisterService {
	if deps.ClaimsHook == nil {
		deps.ClaimsHook = NoOpClaimsHook{}
	}
	return &registerService{deps: deps}
}

// Register errors
var (
	ErrRegisterMissingFields      = fmt.Errorf("missing required fields")
	ErrRegisterInvalidClient      = fmt.Errorf("invalid client")
	ErrRegisterPasswordNotAllowed = fmt.Errorf("password registration not allowed for this client")
	ErrRegisterEmailTaken         = fmt.Errorf("email already registered")
	ErrRegisterPolicyViolation    = fmt.Errorf("password policy violation")
	ErrRegisterHashFailed         = fmt.Errorf("failed to hash password")
	ErrRegisterCreateFailed       = fmt.Errorf("failed to create user")
	ErrRegisterTokenFailed        = fmt.Errorf("failed to issue tokens")
)

func (s *registerService) Register(ctx context.Context, in dto.RegisterRequest) (*dto.RegisterResult, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("auth.register"),
		logger.Op("Register"),
	)

	// Normalize inputs
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.TenantID = strings.TrimSpace(in.TenantID)
	in.ClientID = strings.TrimSpace(in.ClientID)

	// Validate required fields
	if in.Email == "" || in.Password == "" {
		return nil, ErrRegisterMissingFields
	}

	// Check for FS-admin mode (no tenant/client)
	if in.TenantID == "" || in.ClientID == "" {
		if s.deps.FSAdminEnabled {
			return s.registerFSAdmin(ctx, in, log)
		}
		return nil, ErrRegisterMissingFields
	}

	// Standard tenant-based registration
	return s.registerTenantUser(ctx, in, log)
}

// registerFSAdmin handles FS-admin registration (global, no tenant).
// TODO: Migrate fs_admin.go helpers from V1 to V2 before enabling this.
func (s *registerService) registerFSAdmin(ctx context.Context, in dto.RegisterRequest, log *zap.Logger) (*dto.RegisterResult, error) {
	log.Debug("FS-admin registration mode - not yet implemented in V2")
	// FS-admin mode requires helpers.FSAdminRegister from V1 to be migrated.
	// For now, return error indicating this mode is not available in V2.
	return nil, fmt.Errorf("FS-admin registration not available in V2 yet")
}

// registerTenantUser handles standard tenant-based user registration.
func (s *registerService) registerTenantUser(ctx context.Context, in dto.RegisterRequest, log *zap.Logger) (*dto.RegisterResult, error) {
	// Resolve tenant
	tda, err := s.deps.DAL.ForTenant(ctx, in.TenantID)
	if err != nil {
		log.Debug("tenant resolution failed", logger.Err(err))
		return nil, ErrRegisterInvalidClient
	}

	tenantSlug := tda.Slug()
	tenantID := tda.ID()
	log = log.With(logger.TenantSlug(tenantSlug))

	// Require DB for user creation
	if err := tda.RequireDB(); err != nil {
		log.Debug("tenant DB not available", logger.Err(err))
		return nil, ErrNoDatabase
	}

	// Resolve client
	client, err := tda.Clients().Get(ctx, tenantID, in.ClientID)
	if err != nil {
		log.Debug("client not found", logger.Err(err))
		return nil, ErrRegisterInvalidClient
	}

	// Provider gating: check if password registration is allowed
	if !helpers.IsPasswordProviderAllowed(client.Providers) {
		log.Debug("password provider not allowed for client")
		return nil, ErrRegisterPasswordNotAllowed
	}

	// Password policy: blacklist check
	if err := s.checkPasswordPolicy(in.Password); err != nil {
		log.Debug("password policy violation", logger.Err(err))
		return nil, ErrRegisterPolicyViolation
	}

	// Hash password
	phc, err := password.Hash(password.Default, in.Password)
	if err != nil {
		log.Error("password hash failed", logger.Err(err))
		return nil, ErrRegisterHashFailed
	}

	// Create user (UserRepository.Create creates user + password identity together)
	userInput := repository.CreateUserInput{
		TenantID:       tenantID,
		Email:          in.Email,
		PasswordHash:   phc,
		CustomFields:   in.CustomFields,
		SourceClientID: in.ClientID,
	}

	user, _, err := tda.Users().Create(ctx, userInput)
	if err != nil {
		if err == repository.ErrConflict {
			log.Debug("email already exists")
			return nil, ErrRegisterEmailTaken
		}
		log.Error("user creation failed", logger.Err(err))
		return nil, ErrRegisterCreateFailed
	}

	log = log.With(logger.UserID(user.ID))

	// If no auto-login, return just user_id
	if !s.deps.AutoLogin {
		log.Info("user registered (no auto-login)")
		return &dto.RegisterResult{UserID: user.ID}, nil
	}

	// Auto-login: issue tokens
	return s.issueTokens(ctx, tda, user.ID, in.ClientID, client.Scopes, log)
}

// issueTokens issues access and refresh tokens after registration.
func (s *registerService) issueTokens(ctx context.Context, tda store.TenantDataAccess, userID, clientID string, scopes []string, log *zap.Logger) (*dto.RegisterResult, error) {
	tenantID := tda.ID()
	tenantSlug := tda.Slug()

	// Build claims
	amr := []string{"pwd"}
	std := map[string]any{
		"tid": tenantID,
		"amr": amr,
		"acr": "urn:hellojohn:loa:1",
		"scp": strings.Join(scopes, " "),
	}
	custom := map[string]any{}

	// Apply claims hook
	std, custom = s.deps.ClaimsHook.ApplyAccess(ctx, tenantID, clientID, userID, scopes, amr, std, custom)

	// Resolve effective issuer
	effIss := jwtx.ResolveIssuer(
		s.deps.Issuer.Iss,
		string(tda.Settings().IssuerMode),
		tenantSlug,
		tda.Settings().IssuerOverride,
	)

	// Add system claims
	custom = helpers.PutSystemClaimsV2(custom, effIss, nil, nil, nil)

	// Select signing key
	kid, priv, _, err := s.selectSigningKey(tda)
	if err != nil {
		log.Error("failed to get signing key", logger.Err(err))
		return nil, ErrRegisterTokenFailed
	}

	now := time.Now().UTC()
	exp := now.Add(s.deps.Issuer.AccessTTL)

	claims := jwtv5.MapClaims{
		"iss": effIss,
		"sub": userID,
		"aud": clientID,
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}
	for k, v := range std {
		claims[k] = v
	}
	if len(custom) > 0 {
		claims["custom"] = custom
	}

	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = kid
	tk.Header["typ"] = "JWT"

	accessToken, err := tk.SignedString(priv)
	if err != nil {
		log.Error("failed to sign access token", logger.Err(err))
		return nil, ErrRegisterTokenFailed
	}

	// Generate refresh token
	rawRefresh, err := tokens.GenerateOpaqueToken(32)
	if err != nil {
		log.Error("failed to generate refresh token", logger.Err(err))
		return nil, ErrRegisterTokenFailed
	}

	refreshHash := tokens.SHA256Hex(rawRefresh)
	ttlSeconds := int(s.deps.RefreshTTL.Seconds())

	tokenInput := repository.CreateRefreshTokenInput{
		TenantID:   tenantID,
		ClientID:   clientID,
		UserID:     userID,
		TokenHash:  refreshHash,
		TTLSeconds: ttlSeconds,
	}

	if _, err := tda.Tokens().Create(ctx, tokenInput); err != nil {
		log.Error("failed to persist refresh token", logger.Err(err))
		return nil, ErrRegisterTokenFailed
	}

	log.Info("user registered with auto-login")

	return &dto.RegisterResult{
		UserID:       userID,
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(time.Until(exp).Seconds()),
	}, nil
}

// checkPasswordPolicy validates the password against configured policies.
func (s *registerService) checkPasswordPolicy(pwd string) error {
	path := strings.TrimSpace(s.deps.BlacklistPath)
	if path == "" {
		path = strings.TrimSpace(os.Getenv("SECURITY_PASSWORD_BLACKLIST_PATH"))
	}
	if path == "" {
		return nil // No policy configured
	}

	bl, err := password.GetCachedBlacklist(path)
	if err != nil {
		return nil // Ignore blacklist errors
	}

	if bl.Contains(pwd) {
		return ErrRegisterPolicyViolation
	}

	return nil
}

func (s *registerService) selectSigningKey(tda store.TenantDataAccess) (kid string, priv any, pub any, err error) {
	settings := tda.Settings()
	if types.IssuerMode(settings.IssuerMode) == types.IssuerModePath {
		return s.deps.Issuer.Keys.ActiveForTenant(tda.Slug())
	}
	return s.deps.Issuer.Keys.Active()
}
