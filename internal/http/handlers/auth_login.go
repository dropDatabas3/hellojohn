package handlers

import (
	"context"
	"encoding/json"
	"io"
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
		ct := strings.ToLower(r.Header.Get("Content-Type"))
		switch {
		case strings.Contains(ct, "application/json"):
			// Leemos el body con límite (igual que ReadJSON) y soportamos claves alternativas
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			defer r.Body.Close()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "json inválido", 1102)
				return
			}

			// Intento 1: snake_case estándar
			_ = json.Unmarshal(body, &req)

			// Fallback: PascalCase (compat con tests que no ponen tags)
			if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
				var alt struct {
					TenantID string `json:"TenantID"`
					ClientID string `json:"ClientID"`
					Email    string `json:"Email"`
					Password string `json:"Password"`
				}
				if err := json.Unmarshal(body, &alt); err == nil {
					if req.TenantID == "" {
						req.TenantID = strings.TrimSpace(alt.TenantID)
					}
					if req.ClientID == "" {
						req.ClientID = strings.TrimSpace(alt.ClientID)
					}
					if req.Email == "" {
						req.Email = strings.TrimSpace(alt.Email)
					}
					if req.Password == "" {
						req.Password = alt.Password
					}
				}
			}

		case strings.Contains(ct, "application/x-www-form-urlencoded"):
			if err := r.ParseForm(); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_form", "form inválido", 1001)
				return
			}
			req.TenantID = strings.TrimSpace(r.FormValue("tenant_id"))
			req.ClientID = strings.TrimSpace(r.FormValue("client_id"))
			req.Email = strings.TrimSpace(strings.ToLower(r.FormValue("email")))
			req.Password = r.FormValue("password")

		default:
			httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Content-Type debe ser application/json", 1102)
			return
		}

		// normalización consistente
		req.Email = strings.TrimSpace(strings.ToLower(req.Email))

		if req.TenantID == "" || req.ClientID == "" || req.Email == "" || req.Password == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id, client_id, email y password son obligatorios", 1002)
			return
		}
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

		// MFA (pre-issue) hook: si el usuario tiene MFA TOTP confirmada y no se detecta trusted device => bifurca flujo.
		// Requiere métodos stub en Store: GetMFATOTP, IsTrustedDevice. Si no existen aún, este bloque no compilará hasta implementarlos.
		type mfaGetter interface {
			GetMFATOTP(ctx context.Context, userID string) (*core.MFATOTP, error)
		}
		type trustedChecker interface {
			IsTrustedDevice(ctx context.Context, userID, deviceHash string, now time.Time) (bool, error)
		}
		if mg, ok := c.Store.(mfaGetter); ok {
			if m, _ := mg.GetMFATOTP(ctx, u.ID); m != nil && m.ConfirmedAt != nil { // usuario tiene MFA configurada
				trusted := false
				if devCookie, err := r.Cookie("mfa_trust"); err == nil && devCookie != nil {
					if tc, ok2 := c.Store.(trustedChecker); ok2 {
						dh := tokens.SHA256Base64URL(devCookie.Value)
						if ok3, _ := tc.IsTrustedDevice(ctx, u.ID, dh, time.Now()); ok3 {
							trusted = true
						}
					}
				}
				if !trusted { // pedir MFA
					ch := mfaChallenge{
						UserID:   u.ID,
						TenantID: req.TenantID,
						ClientID: req.ClientID,
						AMRBase:  []string{"pwd"},
						Scope:    []string{},
					}
					mid, _ := tokens.GenerateOpaqueToken(24)
					key := "mfa:token:" + mid
					buf, _ := json.Marshal(ch)
					c.Cache.Set(key, buf, 5*time.Minute) // TTL 5m

					w.Header().Set("Cache-Control", "no-store")
					w.Header().Set("Pragma", "no-cache")
					httpx.WriteJSON(w, http.StatusOK, map[string]any{
						"mfa_required": true,
						"mfa_token":    mid,
						"amr":          []string{"pwd"},
					})
					return
				}
			}
		}

		// Base claims (normal path)
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
