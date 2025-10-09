package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type RefreshRequest struct {
	TenantID     string `json:"tenant_id,omitempty"` // aceptado por contrato; no usado para lógica
	ClientID     string `json:"client_id,omitempty"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func NewAuthRefreshHandler(c *app.Container, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req RefreshRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.RefreshToken = strings.TrimSpace(req.RefreshToken)
		req.ClientID = strings.TrimSpace(req.ClientID)
		if req.RefreshToken == "" || req.ClientID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "client_id y refresh_token son obligatorios", 1002)
			return
		}

		ctx := r.Context()

		// 0) Resolver tenant slug en este orden: body.tenant_id -> cpctx.ResolveTenant(r) -> helpers.ResolveTenantSlug(r)
		tenantSlug := strings.TrimSpace(req.TenantID)
		if tenantSlug == "" {
			tenantSlug = helpers.ResolveTenantSlug(r)
		}
		if strings.TrimSpace(tenantSlug) == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id o contexto de tenant requerido", 1002)
			return
		}

		// 1) Cargar RT como fuente de verdad (por hash). No usar c.Store en runtime.
		var (
			rt  *core.RefreshToken
			err error
		)
		// hash en HEX (alineado con store PG)
		sum := sha256.Sum256([]byte(req.RefreshToken))
		hashHex := hex.EncodeToString(sum[:])

		// Intentar en el repo del tenant resuelto
		if c.TenantSQLManager == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "tenant manager not initialized", 1003)
			return
		}
		repo, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
		if err != nil {
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
		}
		if rtx, e2 := repo.GetRefreshTokenByHash(ctx, hashHex); e2 == nil {
			rt = rtx
		}
		if rt == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_grant", "refresh inválido", 1401)
			return
		}

		now := time.Now()
		if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_grant", "refresh revocado o expirado", 1402)
			return
		}

		// 2) ClientID desde RT si no vino en request; si vino y no coincide, invalid_client
		clientID := rt.ClientIDText
		if req.ClientID != "" && !strings.EqualFold(req.ClientID, clientID) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "cliente inválido", 1403)
			return
		}

		// 3) Si el RT pertenece a otro tenant, reabrir repo por rt.TenantID (RT define el tenant)
		if !strings.EqualFold(rt.TenantID, tenantSlug) {
			repo2, e2 := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, rt.TenantID)
			if e2 != nil {
				if helpers.IsNoDBForTenant(e2) {
					httpx.WriteTenantDBMissing(w)
					return
				}
				httpx.WriteTenantDBError(w, e2.Error())
				return
			}
			repo = repo2
		}

		// Rechazar refresh si el usuario está deshabilitado
		if u, err := repo.GetUserByID(ctx, rt.UserID); err == nil && u != nil {
			if u.DisabledAt != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "user_disabled", "usuario deshabilitado", 1410)
				return
			}
		}

		// Scopes: intentar desde FS (hint). Si no, fallback a openid.
		var grantedScopes []string
		if fsc2, err2 := helpers.ResolveClientFSBySlug(ctx, rt.TenantID, clientID); err2 == nil {
			grantedScopes = append([]string{}, fsc2.Scopes...)
		} else {
			grantedScopes = []string{"openid"}
		}
		std := map[string]any{
			"tid": rt.TenantID,
			"amr": []string{"refresh"},
			"scp": strings.Join(grantedScopes, " "),
		}
		// Hook + SYS namespace
		custom := map[string]any{}
		std, custom = applyAccessClaimsHook(r.Context(), c, rt.TenantID, clientID, rt.UserID, grantedScopes, []string{"refresh"}, std, custom)
		// derivar is_admin + RBAC (Fase 2)
		if u, err := repo.GetUserByID(r.Context(), rt.UserID); err == nil && u != nil {
			type rbacReader interface {
				GetUserRoles(ctx context.Context, userID string) ([]string, error)
				GetUserPermissions(ctx context.Context, userID string) ([]string, error)
			}
			var roles, perms []string
			if rr, ok := any(repo).(rbacReader); ok {
				roles, _ = rr.GetUserRoles(r.Context(), rt.UserID)
				perms, _ = rr.GetUserPermissions(r.Context(), rt.UserID)
			}
			custom = helpers.PutSystemClaimsV2(custom, c.Issuer.Iss, u.Metadata, roles, perms)
		}

		token, exp, err := c.Issuer.IssueAccess(rt.UserID, clientID, std, custom)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1405)
			return
		}

		// Crear nuevo refresh token usando método TC si está disponible
		var rawRT string
		tcCreateStore, tcOk := any(repo).(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
		})
		if tcOk {
			// Usar método TC para crear el nuevo sobre el mismo tenant del RT
			rawRT, err = tcCreateStore.CreateRefreshTokenTC(ctx, rt.TenantID, clientID, rt.UserID, refreshTTL)
			if err != nil {
				log.Printf("refresh: create new rt TC err: %v", err)
				httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
				return
			}
		} else {
			// Fallback al método viejo
			rawRT, err = tokens.GenerateOpaqueToken(32)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1406)
				return
			}
			newHash := tokens.SHA256Hex(rawRT)
			expiresAt := now.Add(refreshTTL)

			// Usar CreateRefreshTokenTC para rotación
			if tcStore, ok := any(repo).(interface {
				CreateRefreshTokenTC(context.Context, string, string, string, time.Time, *string) (string, error)
			}); ok {
				if _, err := tcStore.CreateRefreshTokenTC(ctx, rt.TenantID, clientID, newHash, expiresAt, &rt.ID); err != nil {
					log.Printf("refresh: create new rt TC err: %v", err)
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
					return
				}
			} else {
				// Fallback legacy
				if _, err := repo.CreateRefreshToken(ctx, rt.UserID, clientID, newHash, expiresAt, &rt.ID); err != nil {
					log.Printf("refresh: create new rt err: %v", err)
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
					return
				}
			}
		}

		// Revocar el token viejo
		if err := repo.RevokeRefreshToken(ctx, rt.ID); err != nil {
			log.Printf("refresh: revoke old rt err: %v", err)
		}

		// evitar cache en respuestas que incluyen tokens
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		httpx.WriteJSON(w, http.StatusOK, RefreshResponse{
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		})
	}
}

// ------- LOGOUT -------

type LogoutRequest struct {
	TenantID     string `json:"tenant_id,omitempty"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

func NewAuthLogoutHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		var req LogoutRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.RefreshToken = strings.TrimSpace(req.RefreshToken)
		req.ClientID = strings.TrimSpace(req.ClientID)
		req.TenantID = strings.TrimSpace(req.TenantID)
		if req.RefreshToken == "" || req.ClientID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "client_id y refresh_token son obligatorios", 1002)
			return
		}

		ctx := r.Context()
		// Usar el mismo hashing que los métodos TC (hex en lugar de base64)
		sum := sha256.Sum256([]byte(req.RefreshToken))
		hash := hex.EncodeToString(sum[:])

		// Resolver tenant: body -> contexto -> fallback
		tenantSlug := req.TenantID
		if tenantSlug == "" {
			tenantSlug = helpers.ResolveTenantSlug(r)
		}
		if tenantSlug == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id o contexto de tenant requerido", 1002)
			return
		}

		// Resolver repo y buscar RT para validar client y potencial cruce de tenant
		if c.TenantSQLManager == nil {
			httpx.WriteTenantDBError(w, "tenant manager not initialized")
			return
		}
		repo, err := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
		if err != nil {
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
		}

		// Intentar obtener RT por hash
		rt, _ := repo.GetRefreshTokenByHash(ctx, hash)
		if rt == nil {
			// Idempotente: no filtrar existencia
			w.WriteHeader(http.StatusNoContent)
			return
		}
		// Validar client id coincida
		if !strings.EqualFold(req.ClientID, rt.ClientIDText) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client_id no coincide", 2101)
			return
		}
		// Reabrir repo si el RT pertenece a otro tenant
		if !strings.EqualFold(rt.TenantID, tenantSlug) {
			repo2, e2 := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, rt.TenantID)
			if e2 != nil {
				if helpers.IsNoDBForTenant(e2) {
					httpx.WriteTenantDBMissing(w)
					return
				}
				httpx.WriteTenantDBError(w, e2.Error())
				return
			}
			repo = repo2
		}
		type revoker interface {
			RevokeRefreshByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (int64, error)
		}
		if rv, ok := any(repo).(revoker); ok {
			_, _ = rv.RevokeRefreshByHashTC(ctx, rt.TenantID, rt.ClientIDText, hash)
		}

		w.WriteHeader(http.StatusNoContent) // 204
	}
}
