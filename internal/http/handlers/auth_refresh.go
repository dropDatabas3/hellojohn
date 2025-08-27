package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type RefreshRequest struct {
	ClientID     string `json:"client_id"`
	RefreshToken string `json:"refresh_token"`
}

type RefreshResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// NewAuthRefreshHandler implementa rotación de refresh:
// - valida hash en DB, expiración y revocación
// - valida que pertenezca al client solicitado
// - emite access nuevo y refresh nuevo (rotado) y revoca el anterior
func NewAuthRefreshHandler(c *app.Container, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req RefreshRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "payload JSON inválido", 1001)
			return
		}
		req.ClientID = strings.TrimSpace(req.ClientID)
		req.RefreshToken = strings.TrimSpace(req.RefreshToken)
		if req.ClientID == "" || req.RefreshToken == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "client_id y refresh_token son obligatorios", 1002)
			return
		}

		ctx := r.Context()

		// 1) hash y búsqueda
		hash := tokens.SHA256Base64URL(req.RefreshToken)
		rt, err := c.Store.GetRefreshTokenByHash(ctx, hash)
		if err != nil {
			if err == core.ErrNotFound {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token inválido", 1301)
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "store_error", "error interno", 1500)
			return
		}
		now := time.Now()
		if rt.RevokedAt != nil || !now.Before(rt.ExpiresAt) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_refresh", "refresh token revocado o expirado", 1301)
			return
		}

		// 2) validar client
		cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
		if err != nil {
			if err == core.ErrNotFound {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1102)
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "store_error", "error interno", 1500)
			return
		}
		if rt.ClientID != cl.ID {
			httpx.WriteError(w, http.StatusUnauthorized, "mismatched_client", "refresh token no pertenece al client", 1103)
			return
		}

		// 3) emitir nuevo access (tenant desde el client)
		std := map[string]any{
			"tid": cl.TenantID,
			"amr": []string{"refresh"},
		}
		token, exp, err := c.Issuer.IssueAccess(rt.UserID, req.ClientID, std, map[string]any{})
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
			return
		}

		// 4) generar y guardar nuevo refresh, revocar el anterior
		rawRT, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh token", 1202)
			return
		}
		newHash := tokens.SHA256Base64URL(rawRT)
		expiresAt := now.Add(refreshTTL)

		if _, err := c.Store.CreateRefreshToken(ctx, rt.UserID, cl.ID, newHash, expiresAt, &rt.ID); err != nil {
			log.Printf("refresh: create new rt err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh token", 1203)
			return
		}
		if err := c.Store.RevokeRefreshToken(ctx, rt.ID); err != nil {
			// No abortamos la respuesta; log y continuamos.
			log.Printf("refresh: revoke old rt err: %v", err)
		}

		_ = json.NewEncoder(w).Encode(RefreshResponse{
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		})
	}
}

// ------- LOGOUT -------

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// NewAuthLogoutHandler revoca el refresh recibido (idempotente).
func NewAuthLogoutHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req LogoutRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "payload JSON inválido", 1001)
			return
		}
		req.RefreshToken = strings.TrimSpace(req.RefreshToken)
		if req.RefreshToken == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "refresh_token es obligatorio", 1002)
			return
		}

		ctx := r.Context()
		hash := tokens.SHA256Base64URL(req.RefreshToken)
		if rt, err := c.Store.GetRefreshTokenByHash(ctx, hash); err == nil && rt != nil {
			_ = c.Store.RevokeRefreshToken(ctx, rt.ID) // idempotente; ignoramos error aquí
		}

		w.WriteHeader(http.StatusNoContent) // 204
	}
}
