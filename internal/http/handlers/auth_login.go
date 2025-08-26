package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AuthLoginRequest struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthLoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"` // "Bearer"
	ExpiresIn   int64  `json:"expires_in"` // segundos
}

func NewAuthLoginHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req AuthLoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid_json"}`, http.StatusBadRequest)
			return
		}
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
			http.Error(w, `{"error":"missing_fields"}`, http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		u, id, err := c.Store.GetUserByEmail(ctx, req.TenantID, req.Email)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			log.Printf("auth login: user not found or err: %v (tenant=%s email=%s)", err, req.TenantID, req.Email)
			http.Error(w, `{"error":"invalid_credentials"}`, status)
			return
		}

		if id == nil || id.PasswordHash == nil || *id.PasswordHash == "" {
			log.Printf("auth login: no password_hash (tenant=%s email=%s provider=%s)", req.TenantID, req.Email, id.Provider)
			http.Error(w, `{"error":"invalid_credentials"}`, http.StatusUnauthorized)
			return
		}

		if ok := c.Store.CheckPassword(id.PasswordHash, req.Password); !ok {
			log.Printf("auth login: verify=false (tenant=%s email=%s)", req.TenantID, req.Email)
			http.Error(w, `{"error":"invalid_credentials"}`, http.StatusUnauthorized)
			return
		}

		std := map[string]any{
			"tid": req.TenantID,
			"amr": []string{"pwd"},
		}
		custom := map[string]any{}

		token, exp, err := c.Issuer.IssueAccess(u.ID, req.ClientID, std, custom)
		if err != nil {
			http.Error(w, `{"error":"issue_failed"}`, http.StatusInternalServerError)
			return
		}

		resp := AuthLoginResponse{
			AccessToken: token,
			TokenType:   "Bearer",
			ExpiresIn:   int64(time.Until(exp).Seconds()),
		}
		_ = json.NewEncoder(w).Encode(resp)
	}
}
