package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/domain/types"
	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/auth"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	store "github.com/dropDatabas3/hellojohn/internal/store"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
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

	// Validación: email y password siempre requeridos
	if in.Email == "" || in.Password == "" {
		return nil, ErrMissingFields
	}

	// Si faltan tenant_id y/o client_id, intentar login como admin global
	if in.TenantID == "" || in.ClientID == "" {
		return s.loginAsAdmin(ctx, in, log)
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
	// Claims Defaults (Hoist to fix scope)
	amr := []string{"pwd"}
	acr := "urn:hellojohn:loa:1"

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

	// Paso 6: MFA gate

	if mfaRepo := tda.MFA(); mfaRepo != nil {
		mfaCfg, err := mfaRepo.GetTOTP(ctx, user.ID)
		if err == nil && mfaCfg != nil && mfaCfg.ConfirmedAt != nil {
			// MFA Enabled - Check trusted device
			// For now, minimal check: if token provided, assume trust (parity TODO: validate against DB hash)
			isTrusted := in.TrustedDeviceToken != ""

			if !isTrusted {
				// Create Challenge
				mfaToken, err := tokens.GenerateOpaqueToken(32)
				if err != nil {
					log.Error("failed to generate mfa token", logger.Err(err))
					return nil, ErrTokenIssueFailed
				}

				// Cache payload
				challenge := map[string]any{
					"uid": user.ID,
					"tid": tenantID,
					"cid": in.ClientID,
					"amr": []string{"pwd"},
					"scp": client.Scopes, // Grant all requestable scopes or just configured?
				}
				challengeJSON, _ := json.Marshal(challenge)

				// Cache: mfa:token:<token>
				cacheKey := "mfa:token:" + mfaToken
				// TTL: 5 min
				if err := tda.Cache().Set(ctx, cacheKey, string(challengeJSON), 5*time.Minute); err != nil {
					log.Error("failed to cache mfa challenge", logger.Err(err))
					// Fail safe
					return nil, ErrTokenIssueFailed
				}

				return &dto.LoginResult{
					MFARequired: true,
					MFAToken:    mfaToken,
					AMR:         []string{"pwd"},
				}, nil
			}

			// Trusted device -> Upgrade trust
			amr = append(amr, "mfa")
			acr = "urn:hellojohn:loa:2"
		}
	}

	// Paso 7: Claims base
	// Paso 7: Claims base
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

// loginAsAdmin maneja el login de administradores globales del sistema.
// Se usa cuando tenant_id y/o client_id están vacíos.
func (s *loginService) loginAsAdmin(ctx context.Context, in dto.LoginRequest, log *zap.Logger) (*dto.LoginResult, error) {
	log = log.With(logger.Op("loginAsAdmin"))

	// Obtener el AdminRepository del Control Plane
	adminRepo := s.deps.DAL.ConfigAccess().Admins()
	if adminRepo == nil {
		log.Debug("admin repository not available")
		return nil, ErrInvalidCredentials
	}

	// Buscar admin por email
	admin, err := adminRepo.GetByEmail(ctx, in.Email)
	if err != nil {
		log.Debug("admin not found", logger.Err(err))
		return nil, ErrInvalidCredentials
	}

	// Verificar que no esté deshabilitado
	if admin.DisabledAt != nil {
		log.Info("admin disabled")
		return nil, ErrUserDisabled
	}

	// Verificar password
	if !adminRepo.CheckPassword(admin.PasswordHash, in.Password) {
		log.Debug("admin password check failed")
		return nil, ErrInvalidCredentials
	}

	// Actualizar last seen (best effort)
	_ = adminRepo.UpdateLastSeen(ctx, admin.ID)

	// Construir claims para admin global
	amr := []string{"pwd"}
	acr := "urn:hellojohn:loa:1"
	grantedScopes := []string{"openid", "profile", "email"}

	std := map[string]any{
		"tid": "global",
		"amr": amr,
		"acr": acr,
		"scp": strings.Join(grantedScopes, " "),
	}
	custom := map[string]any{
		"admin_type": string(admin.Type),
		"roles":      []string{"sys:admin"},
	}

	now := time.Now().UTC()
	exp := now.Add(s.deps.Issuer.AccessTTL)

	// Obtener clave de firma global
	kid, priv, _, kerr := s.deps.Issuer.Keys.Active()
	if kerr != nil {
		log.Error("failed to get signing key", logger.Err(kerr))
		return nil, ErrTokenIssueFailed
	}

	// Construir JWT claims
	claims := jwtv5.MapClaims{
		"iss": s.deps.Issuer.Iss,
		"sub": admin.ID,
		"aud": "admin",
		"iat": now.Unix(),
		"nbf": now.Unix(),
		"exp": exp.Unix(),
	}
	for k, v := range std {
		claims[k] = v
	}
	claims["custom"] = custom

	// Firmar access token
	tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
	tk.Header["kid"] = kid
	tk.Header["typ"] = "JWT"

	accessToken, err := tk.SignedString(priv)
	if err != nil {
		log.Error("failed to sign access token", logger.Err(err))
		return nil, ErrTokenIssueFailed
	}

	// Para admins, emitir refresh token como JWT stateless (no persistido en DB)
	rtClaims := jwtv5.MapClaims{
		"iss":       s.deps.Issuer.Iss,
		"sub":       admin.ID,
		"aud":       "admin",
		"iat":       now.Unix(),
		"nbf":       now.Unix(),
		"exp":       now.Add(s.deps.RefreshTTL).Unix(),
		"token_use": "refresh",
	}
	rtToken := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, rtClaims)
	rtToken.Header["kid"] = kid
	rtToken.Header["typ"] = "JWT"

	refreshToken, err := rtToken.SignedString(priv)
	if err != nil {
		log.Error("failed to sign refresh token", logger.Err(err))
		return nil, ErrTokenIssueFailed
	}

	log.Info("admin login successful")

	return &dto.LoginResult{
		Success:      true,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(time.Until(exp).Seconds()),
	}, nil
}
