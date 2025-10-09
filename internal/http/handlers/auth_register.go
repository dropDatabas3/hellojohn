package handlers

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AuthRegisterRequest struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
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

func NewAuthRegisterHandler(c *app.Container, autoLogin bool, refreshTTL time.Duration, blacklistPath string) http.HandlerFunc {
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
		if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id, client_id, email y password son obligatorios", 1002)
			return
		}

		// Primero: resolver client desde FS; si no existe, intentaremos fallback a catálogo DB más adelante.
		ctx := r.Context()
		var (
			fsClient        helpers.FSClient
			haveFSClient    bool
			clientProviders []string
			clientScopes    []string
		)
		if fsc, err := helpers.ResolveClientFSBySlug(ctx, req.TenantID, req.ClientID); err == nil {
			fsClient = fsc
			haveFSClient = true
			clientProviders = append([]string{}, fsClient.Providers...)
			clientScopes = append([]string{}, fsClient.Scopes...)
		}

		// Abrir repo por tenant (gating por DSN) con fallback a global store si está presente.
		if c == nil || c.TenantSQLManager == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "tenant manager not initialized", 1003)
			return
		}
		var repo core.Repository
		if rc, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, req.TenantID); err != nil {
			// Phase 4: gate by tenant DB. Do not fallback to global store when tenant DB is missing.
			if helpers.IsTenantNotFound(err) {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "tenant inválido", 2100)
				return
			}
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		} else {
			repo = rc
		}
		ctx = helpers.WithTenantRepo(ctx, repo)

		// Si no hay client en FS, intentar obtenerlo desde DB (modo compat)
		if !haveFSClient {
			type clientGetter interface {
				GetClientByClientID(ctx context.Context, clientID string) (*core.Client, *core.ClientVersion, error)
			}
			if cg, ok := any(repo).(clientGetter); ok {
				if cdb, _, e2 := cg.GetClientByClientID(ctx, req.ClientID); e2 == nil && cdb != nil {
					clientProviders = append([]string{}, cdb.Providers...)
					clientScopes = append([]string{}, cdb.Scopes...)
					fsClient = helpers.FSClient{TenantSlug: req.TenantID, ClientID: cdb.ClientID}
				} else {
					httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1102)
					return
				}
			} else {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1102)
				return
			}
		}
		// Provider gating: if Providers exists and doesn't include password, block
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
		if p == "" { // modo env-only
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
			TenantID:      req.TenantID,
			Email:         req.Email,
			EmailVerified: false,
			Metadata:      map[string]any{},
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

		// Si no hay auto-login, devolvés sólo el user_id (no hay tokens)
		if !autoLogin {
			httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{UserID: u.ID})
			return
		}

		// Auto-login + refresh inicial
		// Scopes placeholder: client default scopes
		grantedScopes := append([]string{}, clientScopes...)
		std := map[string]any{
			"tid": req.TenantID,
			"amr": []string{"pwd"},
			"acr": "urn:hellojohn:loa:1",
			"scp": strings.Join(grantedScopes, " "),
		}
		custom := map[string]any{}

		// Hook opcional
		std, custom = applyAccessClaimsHook(ctx, c, req.TenantID, req.ClientID, u.ID, grantedScopes, []string{"pwd"}, std, custom)

		token, exp, err := c.Issuer.IssueAccess(u.ID, req.ClientID, std, custom)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
			return
		}

		rawRT, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh token", 1202)
			return
		}
		// Usar método TC (Tenant+Client) para Phase 3
		if tcs, ok := any(repo).(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
		}); ok {
			rawRT, err = tcs.CreateRefreshTokenTC(ctx, req.TenantID, req.ClientID, u.ID, refreshTTL)
			if err != nil {
				log.Printf("register: create refresh TC err: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh token", 1203)
				return
			}
		} else {
			// Fallback a método viejo (no debería llegar aquí en Phase 3)
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

		httpx.WriteJSON(w, http.StatusOK, AuthRegisterResponse{
			UserID:       u.ID,
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		})
	}
}
