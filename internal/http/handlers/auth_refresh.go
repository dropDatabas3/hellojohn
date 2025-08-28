package handlers

import (
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

		ctx := r.Context()

		hash := tokens.SHA256Base64URL(req.RefreshToken)
		rt, err := c.Store.GetRefreshTokenByHash(ctx, hash)
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

		cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			httpx.WriteError(w, status, "invalid_client", "client inválido", 1403)
			return
		}
		if rt.ClientID != cl.ID {
			httpx.WriteError(w, http.StatusUnauthorized, "mismatched_client", "refresh no pertenece al client", 1404)
			return
		}

		std := map[string]any{
			"tid": cl.TenantID,
			"amr": []string{"refresh"},
		}
		token, exp, err := c.Issuer.IssueAccess(rt.UserID, req.ClientID, std, map[string]any{})
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1405)
			return
		}

		rawRT, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1406)
			return
		}
		newHash := tokens.SHA256Base64URL(rawRT)
		expiresAt := now.Add(refreshTTL)

		if _, err := c.Store.CreateRefreshToken(ctx, rt.UserID, cl.ID, newHash, expiresAt, &rt.ID); err != nil {
			log.Printf("refresh: create new rt err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir nuevo refresh", 1407)
			return
		}
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
		if req.RefreshToken == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "refresh_token es obligatorio", 1002)
			return
		}
		ctx := r.Context()
		hash := tokens.SHA256Base64URL(req.RefreshToken)
		if rt, err := c.Store.GetRefreshTokenByHash(ctx, hash); err == nil && rt != nil {
			_ = c.Store.RevokeRefreshToken(ctx, rt.ID)
		}
		w.WriteHeader(http.StatusNoContent) // 204
	}
}
