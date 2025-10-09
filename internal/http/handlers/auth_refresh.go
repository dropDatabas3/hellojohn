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
	ClientID     string `json:"client_id"`
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
		req.ClientID = strings.TrimSpace(req.ClientID)
		req.RefreshToken = strings.TrimSpace(req.RefreshToken)
		if req.ClientID == "" || req.RefreshToken == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "client_id y refresh_token son obligatorios", 1002)
			return
		}

		// Guard: verificar que el store esté inicializado
		if c.Store == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "store not initialized", 1003)
			return
		}

		ctx := r.Context()

		// Primero obtenemos el client para tener el tenant_id
		cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			httpx.WriteError(w, status, "invalid_client", "client inválido", 1403)
			return
		}

		// Obtener y validar refresh token (usar método TC si está disponible)
		var rt *core.RefreshToken
		tcStore, ok := c.Store.(interface {
			GetRefreshTokenTC(ctx context.Context, tenantID, clientID, token string) (*core.RefreshToken, error)
		})
		if ok {
			// Usar método TC
			rt, err = tcStore.GetRefreshTokenTC(ctx, cl.TenantID, req.ClientID, req.RefreshToken)
		} else {
			// Fallback al método viejo
			hash := tokens.SHA256Base64URL(req.RefreshToken)
			rt, err = c.Store.GetRefreshTokenByHash(ctx, hash)
		}

		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			httpx.WriteError(w, status, "invalid_refresh", "refresh inválido", 1401)
			return
		}

		now := time.Now()
		if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_refresh", "refresh revocado o expirado", 1402)
			return
		}

		// Validar que el refresh pertenece al client (solo necesario para método viejo)
		if !ok && rt.ClientIDText != cl.ClientID {
			httpx.WriteError(w, http.StatusUnauthorized, "mismatched_client", "refresh no pertenece al client", 1404)
			return
		}

		// Rechazar refresh si el usuario está deshabilitado
		if u, err := c.Store.GetUserByID(ctx, rt.UserID); err == nil && u != nil {
			if u.DisabledAt != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "user_disabled", "usuario deshabilitado", 1410)
				return
			}
		}

		std := map[string]any{
			"tid": cl.TenantID,
			"amr": []string{"refresh"},
		}
		// Hook + SYS namespace
		custom := map[string]any{}
		std, custom = applyAccessClaimsHook(r.Context(), c, cl.TenantID, req.ClientID, rt.UserID, []string{}, []string{"refresh"}, std, custom)
		// derivar is_admin + RBAC (Fase 2)
		if u, err := c.Store.GetUserByID(r.Context(), rt.UserID); err == nil && u != nil {
			type rbacReader interface {
				GetUserRoles(ctx context.Context, userID string) ([]string, error)
				GetUserPermissions(ctx context.Context, userID string) ([]string, error)
			}
			var roles, perms []string
			if rr, ok := c.Store.(rbacReader); ok {
				roles, _ = rr.GetUserRoles(r.Context(), rt.UserID)
				perms, _ = rr.GetUserPermissions(r.Context(), rt.UserID)
			}
			custom = helpers.PutSystemClaimsV2(custom, c.Issuer.Iss, u.Metadata, roles, perms)
		}

		token, exp, err := c.Issuer.IssueAccess(rt.UserID, req.ClientID, std, custom)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1405)
			return
		}

		// Crear nuevo refresh token usando método TC si está disponible
		var rawRT string
		tcCreateStore, tcOk := c.Store.(interface {
			CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
		})
		if tcOk {
			// Usar método TC para crear el nuevo
			rawRT, err = tcCreateStore.CreateRefreshTokenTC(ctx, cl.TenantID, req.ClientID, rt.UserID, refreshTTL)
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
			if tcStore, ok := c.Store.(interface {
				CreateRefreshTokenTC(context.Context, string, string, string, time.Time, *string) (string, error)
			}); ok {
				if _, err := tcStore.CreateRefreshTokenTC(ctx, rt.TenantID, cl.ClientID, newHash, expiresAt, &rt.ID); err != nil {
					log.Printf("refresh: create new rt TC err: %v", err)
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
					return
				}
			} else {
				// Fallback legacy
				if _, err := c.Store.CreateRefreshToken(ctx, rt.UserID, cl.ID, newHash, expiresAt, &rt.ID); err != nil {
					log.Printf("refresh: create new rt err: %v", err)
					httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
					return
				}
			}
		}

		// Revocar el token viejo
		if err := c.Store.RevokeRefreshToken(ctx, rt.ID); err != nil {
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
	TenantID     string `json:"tenant_id"`
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

func NewAuthLogoutHandler(c *app.Container) http.HandlerFunc {
	type revoker interface {
		RevokeRefreshByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (int64, error)
	}
	rv, _ := c.Store.(revoker)

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
		if req.RefreshToken == "" || req.ClientID == "" || req.TenantID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id, client_id y refresh_token son obligatorios", 1002)
			return
		}

		ctx := r.Context()
		// Usar el mismo hashing que los métodos TC (hex en lugar de base64)
		sum := sha256.Sum256([]byte(req.RefreshToken))
		hash := hex.EncodeToString(sum[:])

		if rv != nil {
			// Usar método TC para revocar por (tenant, client_id_text, token_hash)
			_, _ = rv.RevokeRefreshByHashTC(ctx, req.TenantID, req.ClientID, hash)
		} else {
			// Fallback al método viejo si no está disponible
			if rt, err := c.Store.GetRefreshTokenByHash(ctx, hash); err == nil && rt != nil {
				_ = c.Store.RevokeRefreshToken(ctx, rt.ID)
			}
		}

		w.WriteHeader(http.StatusNoContent) // 204
	}
}
