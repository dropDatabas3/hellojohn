package handlers

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/config"
	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/dropDatabas3/hellojohn/internal/util"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type AuthLoginRequest struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthLoginResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"` // "Bearer"
	ExpiresIn    int64  `json:"expires_in"` // segundos
	RefreshToken string `json:"refresh_token"`
}

func NewAuthLoginHandler(c *app.Container, cfg *config.Config, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Debug: log entry
		log.Printf("DEBUG: auth_login handler entry")

		// Usar context del request directamente por ahora
		ctx := r.Context()

		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req AuthLoginRequest
		ct := strings.ToLower(r.Header.Get("Content-Type"))
		switch {
		case strings.Contains(ct, "application/json"):
			// Leemos el body con límite (igual que ReadJSON) y soportamos claves alternativas
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			defer r.Body.Close()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "json inválido", 1102)
				return
			}

			// Intento 1: snake_case estándar
			_ = json.Unmarshal(body, &req)

			// Fallback: PascalCase (compat con tests que no ponen tags)
			if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
				var alt struct {
					TenantID string `json:"TenantID"`
					ClientID string `json:"ClientID"`
					Email    string `json:"Email"`
					Password string `json:"Password"`
				}
				if err := json.Unmarshal(body, &alt); err == nil {
					if req.TenantID == "" {
						req.TenantID = strings.TrimSpace(alt.TenantID)
					}
					if req.ClientID == "" {
						req.ClientID = strings.TrimSpace(alt.ClientID)
					}
					if req.Email == "" {
						req.Email = strings.TrimSpace(alt.Email)
					}
					if req.Password == "" {
						req.Password = alt.Password
					}
				}
			}

		case strings.Contains(ct, "application/x-www-form-urlencoded"):
			if err := r.ParseForm(); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_form", "form inválido", 1001)
				return
			}
			req.TenantID = strings.TrimSpace(r.FormValue("tenant_id"))
			req.ClientID = strings.TrimSpace(r.FormValue("client_id"))
			req.Email = strings.TrimSpace(strings.ToLower(r.FormValue("email")))
			req.Password = r.FormValue("password")

		default:
			httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Content-Type debe ser application/json", 1102)
			return
		}

		// normalización consistente
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		log.Printf("DEBUG: after email normalization, validating fields")

		// Require email and password. Tenant and client are optional to support
		// global FS-admins that do not belong to any tenant or client.
		if req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "email y password son obligatorios", 1002)
			return
		}

		// If tenant or client is missing, attempt FS-admin login when enabled.
		if req.TenantID == "" || req.ClientID == "" {
			if helpers.FSAdminEnabled() {
				// Try to verify FS admin directly. If valid, issue an admin token and return.
				if ufs, ok := helpers.FSAdminVerify(req.Email, req.Password); ok {
					// Provider gating: if clientProviders declared and password not allowed, block.
					// Since no client provided, we skip provider gating here for global admins.
					amrSlice := []string{"pwd"}
					grantedScopes := []string{"openid", "profile", "email"}
					std := map[string]any{
						"tid": "global",
						"amr": amrSlice,
						"acr": "urn:hellojohn:loa:1",
						"scp": strings.Join(grantedScopes, " "),
					}
					custom := helpers.PutSystemClaimsV2(map[string]any{}, c.Issuer.Iss, ufs.Metadata, []string{"sys:admin"}, nil)

					now := time.Now().UTC()
					exp := now.Add(c.Issuer.AccessTTL)
					kid, priv, _, kerr := c.Issuer.Keys.Active()
					if kerr != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma 1", 1204)
						return
					}
					claims := jwtv5.MapClaims{
						"iss": c.Issuer.Iss,
						"sub": ufs.ID,
						"aud": "admin",
						"iat": now.Unix(),
						"nbf": now.Unix(),
						"exp": exp.Unix(),
					}
					for k, v := range std {
						claims[k] = v
					}
					if custom != nil {
						claims["custom"] = custom
					}
					tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
					tk.Header["kid"] = kid
					tk.Header["typ"] = "JWT"
					token, err := tk.SignedString(priv)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
						return
					}
					// Issue refresh token as JWT for stateless admin session
					rtClaims := jwtv5.MapClaims{
						"iss":       c.Issuer.Iss,
						"sub":       ufs.ID,
						"aud":       "admin",
						"iat":       now.Unix(),
						"nbf":       now.Unix(),
						"exp":       now.Add(refreshTTL).Unix(),
						"token_use": "refresh",
					}
					rtToken := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, rtClaims)
					rtToken.Header["kid"] = kid
					rtToken.Header["typ"] = "JWT"
					rtString, err := rtToken.SignedString(priv)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el refresh token", 1204)
						return
					}

					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("Pragma", "no-cache")
					httpx.WriteJSON(w, http.StatusOK, AuthLoginResponse{
						AccessToken:  token,
						TokenType:    "Bearer",
						ExpiresIn:    int64(time.Until(exp).Seconds()),
						RefreshToken: rtString,
					})
					return
				}
				// If FS admin verification failed, return invalid credentials.
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
				return
			}

			// If FS admin not enabled, require tenant and client.
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id y client_id son obligatorios", 1002)
			return
		}

		// Debug: check container
		log.Printf("DEBUG: container=%v, tenantSQLMgr=%v", c != nil, c != nil && c.TenantSQLManager != nil)

		// Resolver slug + UUID del tenant
		tenantSlug, tenantUUID := helpers.ResolveTenantSlugAndID(ctx, req.TenantID)

		// Primero: resolver client desde FS. Si falla, devolver 401 invalid_client y no abrir DB.
		// Resolve client. Prefer FS control-plane; if unavailable, we'll try DB client catalog later only if repo opens.
		var (
			fsClient        helpers.FSClient
			clientScopes    []string
			clientProviders []string
			haveFSClient    bool
		)
		if fsc, err := helpers.ResolveClientFSBySlug(ctx, tenantSlug, req.ClientID); err == nil {
			fsClient = fsc
			clientScopes = append([]string{}, fsClient.Scopes...)
			clientProviders = append([]string{}, fsClient.Providers...)
			haveFSClient = true
		}

		// Abrir repo por tenant solo después de tener un client válido (o si no hay en FS, intentaremos fallback DB más abajo)
		// Compatibility: if per-tenant DB is not configured, fall back to global store (if available).
		var repoCore core.Repository
		if c == nil || c.TenantSQLManager == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "tenant manager not initialized", 1004)
			return
		}
		if rc, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug); err != nil {
			// Phase 4: gate by tenant DB. No fallback to global store in FS-only mode.
			if helpers.IsTenantNotFound(err) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "tenant inválido", 2100)
				return
			}
			if helpers.FSAdminEnabled() {
				// Optional FS-admin fallback: allow admin login when FS_ADMIN_ENABLE=1
				// Triggered on any tenant repo open error when explicitly enabled.
				ufs, ok := helpers.FSAdminVerify(req.Email, req.Password)
				if !ok {
					httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
					return
				}
				// Provider gating remains: if FS had clientProviders and did not include password, block
				if len(clientProviders) > 0 {
					allowed := false
					for _, p := range clientProviders {
						if strings.EqualFold(p, "password") {
							allowed = true
							break
						}
					}
					if !allowed {
						httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "password login deshabilitado para este client", 1207)
						return
					}
				}
				// Issue admin token (no refresh persistence in FS mode)
				amrSlice := []string{"pwd"}
				grantedScopes := append([]string{}, clientScopes...)
				if len(grantedScopes) == 0 {
					grantedScopes = []string{"openid", "profile", "email"}
				}
				std := map[string]any{
					"tid": "global",
					"amr": amrSlice,
					"acr": "urn:hellojohn:loa:1",
					"scp": strings.Join(grantedScopes, " "),
				}
				custom := map[string]any{}
				effIss := c.Issuer.Iss
				custom = helpers.PutSystemClaimsV2(custom, effIss, ufs.Metadata, []string{"sys:admin"}, nil)

				now := time.Now().UTC()
				exp := now.Add(c.Issuer.AccessTTL)
				kid, priv, _, kerr := c.Issuer.Keys.Active()
				if kerr != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma 2", 1204)
					return
				}
				claims := jwtv5.MapClaims{
					"iss": effIss,
					"sub": ufs.ID,
					"aud": req.ClientID,
					"iat": now.Unix(),
					"nbf": now.Unix(),
					"exp": exp.Unix(),
				}
				for k, v := range std {
					claims[k] = v
				}
				if custom != nil {
					claims["custom"] = custom
				}
				tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
				tk.Header["kid"] = kid
				tk.Header["typ"] = "JWT"
				token, err := tk.SignedString(priv)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
					return
				}
				// avoid cache
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Pragma", "no-cache")
				httpx.WriteJSON(w, http.StatusOK, AuthLoginResponse{
					AccessToken: token,
					TokenType:   "Bearer",
					ExpiresIn:   int64(time.Until(exp).Seconds()),
					// No refresh in FS admin mode
				})
				return
			}
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		} else {
			repoCore = rc
		}
		// Optional: cache repo in request context for downstream calls
		ctx = helpers.WithTenantRepo(ctx, repoCore)

		log.Printf("DEBUG: passed guards, parsing request")

		// Rate limiting específico para login (endpoint semántico)
		log.Printf("DEBUG: checking rate limiting, MultiLimiter=%v", c.MultiLimiter != nil)
		if c.MultiLimiter != nil {
			// Parseamos la configuración específica para login
			loginWindow, err := time.ParseDuration(cfg.Rate.Login.Window)
			if err != nil {
				log.Printf("DEBUG: rate limit window parse error: %v, using fallback", err)
				loginWindow = time.Minute // fallback
			}

			loginCfg := helpers.LoginRateConfig{
				Limit:  cfg.Rate.Login.Limit,
				Window: loginWindow,
			}

			log.Printf("DEBUG: calling EnforceLoginLimit with limit=%d, window=%s", loginCfg.Limit, loginCfg.Window)

			// Rate limiting
			rateLimited := !helpers.EnforceLoginLimit(w, r, c.MultiLimiter, loginCfg, req.TenantID, req.Email)

			if rateLimited {
				// Rate limited - la función ya escribió la respuesta 429
				return
			}
		}

		log.Printf("DEBUG: passed rate limiting")

		// Si no teníamos client en FS, intentar lookup en DB solo ahora que repo está abierto
		if !haveFSClient {
			// Fallback: try DB client lookup (works with global repo)
			type clientGetter interface {
				GetClientByClientID(ctx context.Context, clientID string) (*core.Client, *core.ClientVersion, error)
			}
			if cg, ok := any(repoCore).(clientGetter); ok {
				if cdb, _, e2 := cg.GetClientByClientID(ctx, req.ClientID); e2 == nil && cdb != nil {
					clientScopes = append([]string{}, cdb.Scopes...)
					clientProviders = append([]string{}, cdb.Providers...)
					// synthesize minimal fsClient for downstream fields if needed
					fsClient = helpers.FSClient{TenantSlug: tenantSlug, ClientID: cdb.ClientID}
				} else {
					httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
					return
				}
			} else {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
				return
			}
		}

		// Provider gating: ensure password login is allowed for this client.
		// If providers are defined (FS or DB), require "password".
		if len(clientProviders) > 0 {
			allowed := false
			for _, p := range clientProviders {
				if strings.EqualFold(p, "password") {
					allowed = true
					break
				}
			}
			if !allowed {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "password login deshabilitado para este client", 1207)
				return
			}
		}

		// Debug: before repo call
		log.Printf("DEBUG: calling GetUserByEmail with tenant_id=%s, email=%s", tenantUUID, util.MaskEmail(req.Email))

		// ctx ya está definido con timeout arriba
		repo, _ := helpers.GetTenantRepo(ctx)
		u, id, err := repo.GetUserByEmail(ctx, tenantUUID, req.Email)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			log.Printf("auth login: user not found or err: %v (tenant=%s email=%s)", err, req.TenantID, util.MaskEmail(req.Email))
			httpx.WriteError(w, status, "invalid_credentials", "usuario o password inválidos", 1201)
			return
		}

		// Bloqueo por usuario deshabilitado
		isBlocked := false
		if u.DisabledUntil != nil {
			if time.Now().Before(*u.DisabledUntil) {
				isBlocked = true
			}
		} else if u.DisabledAt != nil {
			isBlocked = true
		}

		if isBlocked {
			// prefer 423 Locked for login when disabled
			httpx.WriteError(w, http.StatusLocked, "user_disabled", "usuario deshabilitado", 1210)
			return
		}
		if id == nil || id.PasswordHash == nil || *id.PasswordHash == "" || !repo.CheckPassword(id.PasswordHash, req.Password) {
			log.Printf("auth login: verify=false (tenant=%s email=%s)", req.TenantID, util.MaskEmail(req.Email))
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
			return
		}
		// Client already validated by FS; nothing else to load from DB here

		// MFA (pre-issue) hook: si el usuario tiene MFA TOTP confirmada y no se detecta trusted device => bifurca flujo.
		// Requiere métodos stub en Store: GetMFATOTP, IsTrustedDevice. Si no existen aún, este bloque no compilará hasta implementarlos.
		type mfaGetter interface {
			GetMFATOTP(ctx context.Context, userID string) (*core.MFATOTP, error)
		}
		type trustedChecker interface {
			IsTrustedDevice(ctx context.Context, userID, deviceHash string, now time.Time) (bool, error)
		}
		trustedByCookie := false
		if mg, ok := any(repo).(mfaGetter); ok {
			if m, _ := mg.GetMFATOTP(ctx, u.ID); m != nil && m.ConfirmedAt != nil { // usuario tiene MFA configurada
				if devCookie, err := r.Cookie("mfa_trust"); err == nil && devCookie != nil {
					if tc, ok2 := any(repo).(trustedChecker); ok2 {
						dh := tokens.SHA256Base64URL(devCookie.Value)
						if ok3, _ := tc.IsTrustedDevice(ctx, u.ID, dh, time.Now()); ok3 {
							trustedByCookie = true
						}
					}
				}
				if !trustedByCookie { // pedir MFA interactiva
					ch := mfaChallenge{
						UserID:   u.ID,
						TenantID: req.TenantID,
						ClientID: req.ClientID,
						AMRBase:  []string{"pwd"},
						Scope:    []string{},
					}
					mid, _ := tokens.GenerateOpaqueToken(24)
					key := "mfa:token:" + mid
					buf, _ := json.Marshal(ch)
					c.Cache.Set(key, buf, 5*time.Minute) // TTL 5m

					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("Pragma", "no-cache")
					httpx.WriteJSON(w, http.StatusOK, map[string]any{
						"mfa_required": true,
						"mfa_token":    mid,
						"amr":          []string{"pwd"},
					})
					return
				}
			}
		}

		// Base claims (normal path)
		amrSlice := []string{"pwd"}
		acrVal := "urn:hellojohn:loa:1"
		if trustedByCookie { // Dispositivo previamente validado por MFA
			amrSlice = []string{"pwd", "mfa"}
			acrVal = "urn:hellojohn:loa:2"
		}
		// Scopes placeholder: grant client default scopes for now (Phase 4 minimal)
		grantedScopes := append([]string{}, clientScopes...)
		std := map[string]any{
			"tid": tenantUUID,
			"amr": amrSlice,
			"acr": acrVal,
			"scp": strings.Join(grantedScopes, " "),
		}
		custom := map[string]any{}

		// Hook opcional (CEL/webhook/etc.)
		std, custom = applyAccessClaimsHook(ctx, c, tenantUUID, req.ClientID, u.ID, grantedScopes, amrSlice, std, custom)

		// ── RBAC (Fase 2): roles/perms opcionales si el repo per-tenant los implementa
		type rbacReader interface {
			GetUserRoles(ctx context.Context, userID string) ([]string, error)
			GetUserPermissions(ctx context.Context, userID string) ([]string, error)
		}
		var roles, perms []string
		if repoRR, ok := any(repo).(rbacReader); ok {
			roles, _ = repoRR.GetUserRoles(ctx, u.ID)
			perms, _ = repoRR.GetUserPermissions(ctx, u.ID)
		}
		// issuer se ajusta más abajo según tenant

		// Resolver issuer efectivo por tenant y firmar con clave del tenant si existe
		effIss := c.Issuer.Iss
		if cpctx.Provider != nil {
			if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil {
				effIss = jwtx.ResolveIssuer(c.Issuer.Iss, ten.Settings.IssuerMode, ten.Slug, ten.Settings.IssuerOverride)
			}
		}
		// Actualizar system claims con el issuer efectivo
		custom = helpers.PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)

		now := time.Now().UTC()
		exp := now.Add(c.Issuer.AccessTTL)
		var (
			kid  string
			priv any
			kerr error
		)
		// Elegir clave según modo del issuer: Path => por tenant; Global/Default => global
		if cpctx.Provider != nil {
			if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil && ten.Settings.IssuerMode == controlplane.IssuerModePath {
				kid, priv, _, kerr = c.Issuer.Keys.ActiveForTenant(tenantSlug)
			} else {
				kid, priv, _, kerr = c.Issuer.Keys.Active()
			}
		} else {
			kid, priv, _, kerr = c.Issuer.Keys.Active()
		}
		if kerr != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma 3", 1204)
			return
		}
		claims := jwtv5.MapClaims{
			"iss": effIss,
			"sub": u.ID,
			"aud": req.ClientID,
			"iat": now.Unix(),
			"nbf": now.Unix(),
			"exp": exp.Unix(),
		}
		for k, v := range std {
			claims[k] = v
		}
		if custom != nil {
			claims["custom"] = custom
		}
		tk := jwtv5.NewWithClaims(jwtv5.SigningMethodEdDSA, claims)
		tk.Header["kid"] = kid
		tk.Header["typ"] = "JWT"
		token, err := tk.SignedString(priv)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
			return
		}

		// Crear refresh token usando método TC
		tcStore, ok := any(repo).(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
		})
		if !ok {
			httpx.WriteError(w, http.StatusInternalServerError, "store_not_supported", "store no soporta métodos TC", 1205)
			return
		}

		rawRT, err := tcStore.CreateRefreshTokenTC(ctx, tenantUUID, req.ClientID, u.ID, refreshTTL)
		if err != nil {
			log.Printf("login: create refresh err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1206)
			return
		}

		// evitar cache en respuestas que incluyen tokens
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		httpx.WriteJSON(w, http.StatusOK, AuthLoginResponse{
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		})
	}
}
