package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
)

type SessionLoginRequest struct {
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type SessionPayload struct {
	UserID   string    `json:"user_id"`
	TenantID string    `json:"tenant_id"`
	Expires  time.Time `json:"expires"`
}

func NewSessionLoginHandler(c *app.Container, cookieName, cookieDomain, sameSite string, secure bool, ttl time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		var req SessionLoginRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id, client_id, email y password son obligatorios", 1002)
			return
		}

		// Guard: verificar que el store esté inicializado
		if c.Store == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "store not initialized", 1003)
			return
		}

		ctx := r.Context()
		u, id, err := c.Store.GetUserByEmail(ctx, req.TenantID, req.Email)
		if err != nil || id == nil || id.PasswordHash == nil || !c.Store.CheckPassword(id.PasswordHash, req.Password) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "usuario o password inválidos", 1202)
			return
		}
		cl, _, err := c.Store.GetClientByClientID(ctx, req.ClientID)
		if err != nil || cl == nil || cl.TenantID != req.TenantID {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
			return
		}

		rawSID, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar sid", 1301)
			return
		}
		exp := time.Now().Add(ttl)
		payload := SessionPayload{UserID: u.ID, TenantID: req.TenantID, Expires: exp}
		b, _ := json.Marshal(payload)
		c.Cache.Set("sid:"+tokens.SHA256Base64URL(rawSID), b, ttl)

		// Seteamos cookie usando el helper centralizado
		http.SetCookie(w, BuildSessionCookie(cookieName, rawSID, cookieDomain, sameSite, secure, ttl))

		// Evitar cacheo de respuestas que tocan sesión
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")

		w.WriteHeader(http.StatusNoContent) // 204
	}
}
