package handlers

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
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

		ctx := r.Context()

		cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
		if err != nil {
			if err == core.ErrNotFound {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1102)
				return
			}
			httpx.WriteError(w, http.StatusInternalServerError, "store_error", "error interno", 1500)
			return
		}
		if cl.TenantID != req.TenantID {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_tenant", "el client no pertenece al tenant", 1101)
			return
		}
		if !contains(cl.Providers, "password") {
			httpx.WriteError(w, http.StatusForbidden, "password_disabled_for_client", "el client no permite login por password", 1104)
			return
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
		if err := c.Store.CreateUser(ctx, u); err != nil {
			log.Printf("register: create user err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "register_failed", "no se pudo crear el usuario", 1204)
			return
		}

		if err := c.Store.CreatePasswordIdentity(ctx, u.ID, req.Email, false, phc); err != nil {
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
		std := map[string]any{
			"tid": req.TenantID,
			"amr": []string{"pwd"},
			"acr": "urn:hellojohn:loa:1",
		}
		custom := map[string]any{}

		// Hook opcional
		std, custom = applyAccessClaimsHook(ctx, c, req.TenantID, req.ClientID, u.ID, []string{}, []string{"pwd"}, std, custom)

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
		hash := tokens.SHA256Base64URL(rawRT)
		if _, err := c.Store.CreateRefreshToken(ctx, u.ID, cl.ID, hash, time.Now().Add(refreshTTL), nil); err != nil {
			log.Printf("register: create refresh err: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh token", 1203)
			return
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
