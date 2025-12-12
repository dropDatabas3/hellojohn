package handlers

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type AuthRegisterRequest struct {
	TenantID     string         `json:"tenant_id"`
	ClientID     string         `json:"client_id"`
	Email        string         `json:"email"`
	Password     string         `json:"password"`
	CustomFields map[string]any `json:"custom_fields,omitempty"`
}

type AuthRegisterResponse struct {
	UserID       string `json:"user_id,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	TokenType    string `json:"token_type,omitempty"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func NewAuthRegisterHandler(c *app.Container, emailHandler *EmailFlowsHandler, autoLogin bool, refreshTTL time.Duration, blacklistPath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req AuthRegisterRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.TenantID = strings.TrimSpace(req.TenantID)
		req.ClientID = strings.TrimSpace(req.ClientID)
		// Require email and password. Tenant and client optional to allow global FS-admin register.
		if req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "email y password son obligatorios", 1002)
			return
		}
		if req.TenantID == "" || req.ClientID == "" {
			if helpers.FSAdminEnabled() {
				// Register as FS admin directly
				ufs, ferr := helpers.FSAdminRegister(req.Email, req.Password)
				if ferr != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "register_failed", ferr.Error(), 1204)
					return
				}
				grantedScopes := []string{"openid", "profile", "email"}
				std := map[string]any{
					"tid": "global",
					"amr": []string{"pwd"},
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma", 1201)
					return
				}
				claims := jwtv5.MapClaims{
					"iss": effIss,
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Pragma", "no-cache")
				httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{
					UserID:      ufs.ID,
					AccessToken: token,
					TokenType:   "Bearer",
					ExpiresIn:   int64(time.Until(exp).Seconds()),
				})
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id y client_id son obligatorios", 1002)
			return
		}

		// Contexto
		ctx := r.Context()

		// Resolver slug + UUID del tenant
		tenantSlug, tenantUUID := helpers.ResolveTenantSlugAndID(ctx, req.TenantID)

		// Resolver client desde FS si existe
		var (
			fsClient        helpers.FSClient
			haveFSClient    bool
			clientProviders []string
			clientScopes    []string
		)
		if fsc, err := helpers.ResolveClientFSBySlug(ctx, tenantSlug, req.ClientID); err == nil {
			fsClient = fsc
			haveFSClient = true
			clientProviders = append([]string{}, fsClient.Providers...)
			clientScopes = append([]string{}, fsClient.Scopes...)
		}

		// Abrir repo por tenant (gating por DSN)
		if c == nil || c.TenantSQLManager == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "tenant manager not initialized", 1003)
			return
		}
		var repo core.Repository
		rc, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
		if err != nil {
			if helpers.IsTenantNotFound(err) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "tenant inválido", 2100)
				return
			}
			// Fallback FS-admin para cualquier error de apertura (excepto tenant inexistente)
			if helpers.FSAdminEnabled() {
				ufs, ferr := helpers.FSAdminRegister(req.Email, req.Password)
				if ferr != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "register_failed", ferr.Error(), 1204)
					return
				}
				grantedScopes := []string{"openid", "profile", "email"}
				std := map[string]any{
					"tid": "global",
					"amr": []string{"pwd"},
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma", 1201)
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
					httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
					return
				}
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Pragma", "no-cache")
				httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{
					UserID:      ufs.ID,
					AccessToken: token,
					TokenType:   "Bearer",
					ExpiresIn:   int64(time.Until(exp).Seconds()),
				})
				return
			}
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
		repo = rc

		// Si no hay client en FS, intentar obtenerlo desde DB (modo compat)
		if !haveFSClient {
			type clientGetter interface {
				GetClientByClientID(ctx context.Context, clientID string) (*core.Client, *core.ClientVersion, error)
			}
			if cg, ok := any(repo).(clientGetter); ok {
				if cdb, _, e2 := cg.GetClientByClientID(ctx, req.ClientID); e2 == nil && cdb != nil {
					clientProviders = append([]string{}, cdb.Providers...)
					clientScopes = append([]string{}, cdb.Scopes...)
					fsClient = helpers.FSClient{TenantSlug: tenantSlug, ClientID: cdb.ClientID}
				} else {
					httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1102)
					return
				}
			} else {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1102)
				return
			}
		}

		// Provider gating: si existen providers y no incluye password => bloquear
		if len(clientProviders) > 0 {
			allowed := false
			for _, p := range clientProviders {
				if strings.EqualFold(p, "password") {
					allowed = true
					break
				}
			}
			if !allowed {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "password login deshabilitado para este client", 1104)
				return
			}
		}

		// Blacklist opcional
		p := strings.TrimSpace(blacklistPath)
		if p == "" {
			p = strings.TrimSpace(os.Getenv("SECURITY_PASSWORD_BLACKLIST_PATH"))
		}
		if p != "" {
			if bl, err := password.GetCachedBlacklist(p); err == nil && bl.Contains(req.Password) {
				httpx.WriteError(w, http.StatusBadRequest, "policy_violation", "password no permitido por política", 2401)
				return
			}
		}

		phc, err := password.Hash(password.Default, req.Password)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "hash_failed", "no se pudo hashear el password", 1200)
			return
		}

		u := &core.User{
			TenantID:       tenantUUID,
			Email:          req.Email,
			EmailVerified:  false,
			Metadata:       map[string]any{},
			CustomFields:   req.CustomFields,
			SourceClientID: &req.ClientID,
		}
		if err := repo.CreateUser(ctx, u); err != nil {
			log.Printf("register: create user err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "register_failed", "no se pudo crear el usuario", 1204)
			return
		}

		if err := repo.CreatePasswordIdentity(ctx, u.ID, req.Email, false, phc); err != nil {
			if err == core.ErrConflict {
				httpx.WriteError(w, http.StatusConflict, "email_taken", "ya existe un usuario con ese email", 1409)
				return
			}
			log.Printf("register: create identity err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "register_failed", "no se pudo crear la identidad", 1204)
			return
		}

		// Si no hay auto-login, devolver sólo user_id
		if !autoLogin {
			httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{UserID: u.ID})
			return
		}

		// Auto-login + refresh inicial
		grantedScopes := append([]string{}, clientScopes...)
		std := map[string]any{
			"tid": tenantUUID,
			"amr": []string{"pwd"},
			"acr": "urn:hellojohn:loa:1",
			"scp": strings.Join(grantedScopes, " "),
		}
		custom := map[string]any{}

		// Hook opcional
		std, custom = applyAccessClaimsHook(ctx, c, tenantUUID, req.ClientID, u.ID, grantedScopes, []string{"pwd"}, std, custom)

		// Per-tenant issuer + signing key
		effIss := c.Issuer.Iss
		if cpctx.Provider != nil {
			if ten, errTen := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); errTen == nil && ten != nil {
				effIss = jwtx.ResolveIssuer(c.Issuer.Iss, ten.Settings.IssuerMode, ten.Slug, ten.Settings.IssuerOverride)
			}
		}
		now := time.Now().UTC()
		exp := now.Add(c.Issuer.AccessTTL)
		var (
			kid  string
			priv any
			kerr error
		)
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
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo obtener clave de firma", 1201)
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
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
			return
		}

		rawRT, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh token", 1202)
			return
		}
		if tcs, ok := any(repo).(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
		}); ok {
			rawRT, err = tcs.CreateRefreshTokenTC(ctx, tenantUUID, req.ClientID, u.ID, refreshTTL)
			if err != nil {
				log.Printf("register: create refresh TC err: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh token", 1203)
				return
			}
		} else {
			hash := tokens.SHA256Base64URL(rawRT)
			if _, err := repo.CreateRefreshToken(ctx, u.ID, req.ClientID, hash, time.Now().Add(refreshTTL), nil); err != nil {
				log.Printf("register: create refresh err: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh token", 1203)
				return
			}
		}

		// evitar cache en respuestas que incluyen tokens
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		// Check for email verification requirement (FS Client)
		var verificationRequired bool
		if cpctx.Provider != nil {
			if ten, tErr := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug); tErr == nil && ten != nil {
				if fsc, cErr := cpctx.Provider.GetClient(ctx, tenantSlug, req.ClientID); cErr == nil && fsc != nil {
					verificationRequired = fsc.RequireEmailVerification
				}
			}
		}

		// Trigger verification email if required
		if verificationRequired && emailHandler != nil {
			// We use a background context or the request context? Request context is fine, but we should not fail register if mail fails (soft fail).
			rid := w.Header().Get("X-Request-ID")
			// Create uuid from string for tenantID/userID
			tidUUID, _ := uuid.Parse(tenantUUID) // should be valid
			uidUUID, _ := uuid.Parse(u.ID)

			// We pass empty redirect (will use default) or we can try to guess/construct it.
			// The email link will point to /v1/auth/verify-email?token=... which redirects to Valid Redirect URI.
			// Since we don't have a specific redirect URI in Register Request, we pass empty string.
			_ = emailHandler.SendVerificationEmail(ctx, rid, tidUUID, uidUUID, req.Email, "", req.ClientID)
		}

		httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{
			UserID:       u.ID,
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		})
	}
}
