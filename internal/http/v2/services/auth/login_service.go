package auth

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/domain/types"
	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/auth"
	"github.com/dropDatabas3/hellojohn/internal/http/v2/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// LoginDeps contiene las dependencias para el login service.
type LoginDeps struct {
	DAL        store.DataAccessLayer
	Issuer     *jwtx.Issuer
	RefreshTTL time.Duration
	ClaimsHook ClaimsHook // nil = NoOp
}

type loginService struct {
	deps LoginDeps
}

// NewLoginService crea un nuevo servicio de login.
func NewLoginService(deps LoginDeps) LoginService {
	if deps.ClaimsHook == nil {
		deps.ClaimsHook = NoOpClaimsHook{}
	}
	return &loginService{deps: deps}
}

// Errores de login
var (
	ErrMissingFields      = fmt.Errorf("missing required fields")
	ErrInvalidClient      = fmt.Errorf("invalid client")
	ErrPasswordNotAllowed = fmt.Errorf("password login not allowed for this client")
	ErrInvalidCredentials = fmt.Errorf("invalid credentials")
	ErrUserDisabled       = fmt.Errorf("user disabled")
	ErrEmailNotVerified   = fmt.Errorf("email not verified")
	ErrNoDatabase         = fmt.Errorf("no database for tenant")
	ErrTokenIssueFailed   = fmt.Errorf("failed to issue token")
)

func (s *loginService) LoginPassword(ctx context.Context, in dto.LoginRequest) (*dto.LoginResult, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("auth.login"),
		logger.Op("LoginPassword"),
	)

	// Paso 0: Normalización
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.TenantID = strings.TrimSpace(in.TenantID)
	in.ClientID = strings.TrimSpace(in.ClientID)

	// Validación mínima
	if in.Email == "" || in.Password == "" || in.TenantID == "" || in.ClientID == "" {
		return nil, ErrMissingFields
	}

	// Paso 1: Resolver tenant (sin abrir DB todavía)
	tda, err := s.deps.DAL.ForTenant(ctx, in.TenantID)
	if err != nil {
		log.Debug("tenant resolution failed", logger.Err(err))
		return nil, ErrInvalidClient
	}
	tenantSlug := tda.Slug()
	tenantID := tda.ID()

	log = log.With(logger.TenantSlug(tenantSlug))

	// Paso 2: Resolver client por FS y aplicar provider gating
	client, err := tda.Clients().Get(ctx, tenantID, in.ClientID)
	if err != nil {
		log.Debug("client not found", logger.Err(err))
		return nil, ErrInvalidClient
	}

	// Provider gating: verificar que "password" esté permitido
	if !helpers.IsPasswordProviderAllowed(client.Providers) {
		log.Debug("password provider not allowed")
		return nil, ErrPasswordNotAllowed
	}

	// Paso 3: Ahora sí requerir DB
	if err := tda.RequireDB(); err != nil {
		log.Debug("tenant DB not available", logger.Err(err))
		return nil, ErrNoDatabase
	}

	// Paso 4: Buscar usuario y verificar password
	user, identity, err := tda.Users().GetByEmail(ctx, tenantID, in.Email)
	if err != nil {
		log.Debug("user not found")
		return nil, ErrInvalidCredentials
	}

	log = log.With(logger.UserID(user.ID))

	// Verificar estado del usuario
	if helpers.IsUserDisabled(user) {
		log.Info("user disabled")
		return nil, ErrUserDisabled
	}

	// Verificar password
	if identity == nil || identity.PasswordHash == nil || *identity.PasswordHash == "" {
		log.Debug("no password identity")
		return nil, ErrInvalidCredentials
	}

	if !tda.Users().CheckPassword(identity.PasswordHash, in.Password) {
		log.Debug("password check failed")
		return nil, ErrInvalidCredentials
	}

	// Paso 5: Email verification gating
	if client.RequireEmailVerification && !user.EmailVerified {
		log.Info("email not verified")
		return nil, ErrEmailNotVerified
	}

	// Paso 6: MFA gate (TODO en iteración 3)

	// Paso 7: Claims base
	amr := []string{"pwd"}
	acr := "urn:hellojohn:loa:1"
	grantedScopes := client.Scopes

	std := map[string]any{
		"tid": tenantID,
		"amr": amr,
		"acr": acr,
		"scp": strings.Join(grantedScopes, " "),
	}
	custom := map[string]any{}

	// RBAC (TODO en iteración 2): roles/perms si disponibles

	// Claims hook (extensible)
	std, custom = s.deps.ClaimsHook.ApplyAccess(ctx, tenantID, in.ClientID, user.ID, grantedScopes, amr, std, custom)

	// Paso 8: Resolver issuer efectivo y emitir Access Token
	effIss := jwtx.ResolveIssuer(
		s.deps.Issuer.Iss,
		string(tda.Settings().IssuerMode),
		tenantSlug,
		tda.Settings().IssuerOverride,
	)

	now := time.Now().UTC()
	exp := now.Add(s.deps.Issuer.AccessTTL)

	// Seleccionar key según modo
	kid, priv, _, err := s.selectSigningKey(tda)
	if err != nil {
		log.Error("failed to get signing key", logger.Err(err))
		return nil, ErrTokenIssueFailed
	}

	claims := jwtv5.MapClaims{
		"iss": effIss,
		"sub": user.ID,
		"aud": in.ClientID,
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
		return nil, ErrTokenIssueFailed
	}

	// Paso 9: Refresh token persistente
	rawRefresh, err := tokens.GenerateOpaqueToken(32)
	if err != nil {
		log.Error("failed to generate refresh token", logger.Err(err))
		return nil, ErrTokenIssueFailed
	}

	// Guardar refresh token hash en DB
	refreshHash := tokens.SHA256Base64URL(rawRefresh)
	ttlSeconds := int(s.deps.RefreshTTL.Seconds())

	tokenInput := repository.CreateRefreshTokenInput{
		TenantID:   tenantID,
		ClientID:   in.ClientID,
		UserID:     user.ID,
		TokenHash:  refreshHash,
		TTLSeconds: ttlSeconds,
	}

	if _, err := tda.Tokens().Create(ctx, tokenInput); err != nil {
		log.Error("failed to persist refresh token", logger.Err(err))
		return nil, ErrTokenIssueFailed
	}

	log.Info("login successful")

	return &dto.LoginResult{
		Success:      true,
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresIn:    int64(time.Until(exp).Seconds()),
	}, nil
}

// ─── Internal Helpers ───
// Nota: helpers comunes están en internal/http/v2/helpers/

func (s *loginService) selectSigningKey(tda store.TenantDataAccess) (kid string, priv any, pub any, err error) {
	settings := tda.Settings()
	if types.IssuerMode(settings.IssuerMode) == types.IssuerModePath {
		return s.deps.Issuer.Keys.ActiveForTenant(tda.Slug())
	}
	return s.deps.Issuer.Keys.Active()
}
