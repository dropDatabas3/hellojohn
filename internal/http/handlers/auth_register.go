package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AuthRegisterRequest struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type AuthRegisterResponse struct {
	UserID      string `json:"user_id,omitempty"`
	AccessToken string `json:"access_token,omitempty"`
	TokenType   string `json:"token_type,omitempty"`
	ExpiresIn   int64  `json:"expires_in,omitempty"`
}

func contains(ss []string, v string) bool {
	for _, s := range ss {
		if strings.EqualFold(s, v) {
			return true
		}
	}
	return false
}

func NewAuthRegisterHandler(c *app.Container, autoLogin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req AuthRegisterRequest
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

		// 1) Validar client y que soporte password
		cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusUnauthorized
			}
			log.Printf("register: client not found or err: %v (client_id=%s)", err, req.ClientID)
			http.Error(w, `{"error":"invalid_client"}`, status)
			return
		}
		if cl.TenantID != req.TenantID {
			http.Error(w, `{"error":"invalid_tenant"}`, http.StatusUnauthorized)
			return
		}
		if !contains(cl.Providers, "password") {
			http.Error(w, `{"error":"password_disabled_for_client"}`, http.StatusForbidden)
			return
		}

		// 2) Hashear password
		phc, err := password.Hash(password.Default, req.Password)
		if err != nil {
			http.Error(w, `{"error":"hash_failed"}`, http.StatusInternalServerError)
			return
		}

		// 3) Crear/obtener user y crear identidad password
		u := &core.User{
			TenantID:      req.TenantID,
			Email:         req.Email,
			EmailVerified: false,
			Status:        "active",
			Metadata:      map[string]any{},
		}
		if err := c.Store.CreateUser(ctx, u); err != nil {
			log.Printf("register: create user err: %v", err)
			http.Error(w, `{"error":"register_failed"}`, http.StatusInternalServerError)
			return
		}

		if err := c.Store.CreatePasswordIdentity(ctx, u.ID, req.Email, false, phc); err != nil {
			if err == core.ErrConflict {
				http.Error(w, `{"error":"email_taken"}`, http.StatusConflict)
				return
			}
			log.Printf("register: create identity err: %v", err)
			http.Error(w, `{"error":"register_failed"}`, http.StatusInternalServerError)
			return
		}

		// 4) Responder
		if !autoLogin {
			_ = json.NewEncoder(w).Encode(AuthRegisterResponse{UserID: u.ID})
			return
		}

		// auto-login
		std := map[string]any{
			"tid": req.TenantID,
			"amr": []string{"pwd"},
		}
		token, exp, err := c.Issuer.IssueAccess(u.ID, req.ClientID, std, map[string]any{})
		if err != nil {
			http.Error(w, `{"error":"issue_failed"}`, http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(AuthRegisterResponse{
			UserID:      u.ID,
			AccessToken: token,
			TokenType:   "Bearer",
			ExpiresIn:   int64(time.Until(exp).Seconds()),
		})
	}
}
