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

func NewAuthLoginHandler(c *app.Container, refreshTTL time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req AuthLoginRequest
		dec := json.NewDecoder(r.Body)
		if err := dec.Decode(&req); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "payload JSON inválido", 1001)
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id, client_id, email y password son obligatorios", 1002)
			return
		}

		ctx := r.Context()

		// Usuario + password
		u, id, err := c.Store.GetUserByEmail(ctx, req.TenantID, req.Email)
		if err != nil {
			if err == core.ErrNotFound {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "credenciales inválidas", 1101)
				return
			}
			log.Printf("auth login: GetUserByEmail error: %v (tenant=%s email=%s)", err, req.TenantID, req.Email)
			httpx.WriteError(w, http.StatusInternalServerError, "store_error", "error interno", 1500)
			return
		}
		if id == nil || id.PasswordHash == nil || *id.PasswordHash == "" || !c.Store.CheckPassword(id.PasswordHash, req.Password) {
			log.Printf("auth login: verify=false (tenant=%s email=%s)", req.TenantID, req.Email)
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "credenciales inválidas", 1101)
			return
		}

		// Validar client y tenant
		cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
		if err != nil || cl == nil || cl.TenantID != req.TenantID {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido para el tenant", 1102)
			return
		}

		// Access token
		std := map[string]any{
			"tid": req.TenantID,
			"amr": []string{"pwd"},
		}
		custom := map[string]any{}

		token, exp, err := c.Issuer.IssueAccess(u.ID, req.ClientID, std, custom)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1201)
			return
		}

		// Refresh token (opaco) inicial
		rawRT, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh token", 1202)
			return
		}
		hash := tokens.SHA256Base64URL(rawRT)
		if _, err := c.Store.CreateRefreshToken(ctx, u.ID, cl.ID, hash, time.Now().Add(refreshTTL), nil); err != nil {
			log.Printf("auth login: CreateRefreshToken error: %v", err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh token", 1203)
			return
		}

		resp := AuthLoginResponse{
			AccessToken:  token,
			TokenType:    "Bearer",
			ExpiresIn:    int64(time.Until(exp).Seconds()),
			RefreshToken: rawRT,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
