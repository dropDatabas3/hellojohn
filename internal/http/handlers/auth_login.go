package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/dropDatabas3/hellojohn/internal/util"
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
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req AuthLoginRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id, client_id, email y password son obligatorios", 1002)
			return
		}

		// Rate limiting específico para login (endpoint semántico)
		if c.MultiLimiter != nil {
			// Parseamos la configuración específica para login
			loginWindow, err := time.ParseDuration(cfg.Rate.Login.Window)
			if err != nil {
				loginWindow = time.Minute // fallback
			}

			loginCfg := helpers.LoginRateConfig{
				Limit:  cfg.Rate.Login.Limit,
				Window: loginWindow,
			}

			if !helpers.EnforceLoginLimit(w, r, c.MultiLimiter, loginCfg, req.TenantID, req.Email) {
				// Rate limited - la función ya escribió la respuesta 429
				return
			}
		}

		ctx := r.Context()

		u, id, err := c.Store.GetUserByEmail(ctx, req.TenantID, req.Email)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			log.Printf("auth login: user not found or err: %v (tenant=%s email=%s)", err, req.TenantID, util.MaskEmail(req.Email))
			httpx.WriteError(w, status, "invalid_credentials", "usuario o password inválidos", 1201)
			return
		}
		if id == nil || id.PasswordHash == nil || *id.PasswordHash == "" || !c.Store.CheckPassword(id.PasswordHash, req.Password) {
			log.Printf("auth login: verify=false (tenant=%s email=%s)", req.TenantID, util.MaskEmail(req.Email))
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
			return
		}

		cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
		if err != nil || cl == nil || cl.TenantID != req.TenantID {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
			return
		}

		// Base claims
		std := map[string]any{
			"tid": req.TenantID,
			"amr": []string{"pwd"},
		}
		custom := map[string]any{}

		// Hook opcional (CEL/webhook/etc.)
		std, custom = applyAccessClaimsHook(ctx, c, req.TenantID, req.ClientID, u.ID, []string{}, []string{"pwd"}, std, custom)

		token, exp, err := c.Issuer.IssueAccess(u.ID, req.ClientID, std, custom)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
			return
		}

		rawRT, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1205)
			return
		}
		hash := tokens.SHA256Base64URL(rawRT)
		if _, err := c.Store.CreateRefreshToken(ctx, u.ID, cl.ID, hash, time.Now().Add(refreshTTL), nil); err != nil {
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
