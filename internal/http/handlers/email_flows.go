package handlers

import (
	"encoding/json"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/email"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/dropDatabas3/hellojohn/internal/store"
)

// --- Interfaces de integración con tu core ---

type RedirectValidator interface {
	ValidateRedirectURI(tenantID uuid.UUID, clientID string, redirectURI string) bool
}

type TokenIssuer interface {
	IssueTokens(w http.ResponseWriter, r *http.Request, tenantID uuid.UUID, clientID string, userID uuid.UUID) error
}

type CurrentUserProvider interface {
	CurrentUserID(r *http.Request) (uuid.UUID, error)
	CurrentTenantID(r *http.Request) (uuid.UUID, error)
	CurrentUserEmail(r *http.Request) (string, error)
}

type RateLimiter interface {
	Allow(key string) (allowed bool, retryAfter time.Duration)
}

// --- Handler ---

type EmailFlowsHandler struct {
	Tokens   *store.TokenStore
	Users    *store.UserStore
	Mailer   email.Sender
	Tmpl     *email.Templates
	Policy   password.Policy
	Redirect RedirectValidator
	Issuer   TokenIssuer
	Auth     CurrentUserProvider
	Limiter  RateLimiter

	BaseURL        string
	VerifyTTL      time.Duration
	ResetTTL       time.Duration
	AutoLoginReset bool

	DebugEchoLinks bool // expone links en headers para DX/testing
}

func (h *EmailFlowsHandler) Register(r chi.Router) {
	r.Group(func(r chi.Router) {
		r.Post("/v1/auth/verify-email/start", h.verifyEmailStart)
		r.Get("/v1/auth/verify-email", h.verifyEmailConfirm)

		r.Post("/v1/auth/forgot", h.forgot)
		r.Post("/v1/auth/reset", h.reset)
	})
}

func writeErr(w http.ResponseWriter, code, desc string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": code, "error_description": desc,
	})
}

func (h *EmailFlowsHandler) rlOr429(w http.ResponseWriter, key string) bool {
	if h.Limiter == nil {
		return false
	}
	allowed, retry := h.Limiter.Allow(key)
	if allowed {
		return false
	}
	if retry > 0 {
		secs := int(math.Ceil(retry.Seconds()))
		if secs < 1 {
			secs = 1
		}
		w.Header().Set("Retry-After", strconv.Itoa(secs))
	}
	writeErr(w, "rate_limited", "Too many requests", http.StatusTooManyRequests)
	return true
}

// --- Verify Email ---

type verifyStartIn struct {
	TenantID uuid.UUID `json:"tenant_id"`
	ClientID string    `json:"client_id"`
	Redirect string    `json:"redirect_uri,omitempty"`
}

func (h *EmailFlowsHandler) verifyEmailStart(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in verifyStartIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"verify_start_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"verify_start_begin","request_id":"%s","tenant_id":"%s","client_id":"%s","redirect":"%s"}`, rid, in.TenantID, in.ClientID, in.Redirect)

	if in.TenantID == uuid.Nil || in.ClientID == "" {
		log.Printf(`{"level":"warn","msg":"verify_start_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id and client_id required", http.StatusBadRequest)
		return
	}

	if h.rlOr429(w, "verify_start:"+clientIPOrEmpty(r)) {
		log.Printf(`{"level":"warn","msg":"verify_start_rate_limited","request_id":"%s","remote":"%s"}`, rid, clientIPOrEmpty(r))
		return
	}

	userID, err := h.Auth.CurrentUserID(r)
	if err != nil {
		writeErr(w, "login_required", "Bearer required", http.StatusUnauthorized)
		return
	}
	emailStr, _ := h.Auth.CurrentUserEmail(r)
	if emailStr == "" {
		// Fallback a DB si el token no trae claim 'email'
		if e, err := h.Users.GetEmailByID(r.Context(), userID); err == nil && e != "" {
			emailStr = e
		}
	}
	if emailStr == "" {
		// No podemos enviar sin destinatario: log explícito y 500
		writeErr(w, "server_error", "user email not found", http.StatusInternalServerError)
		return
	}

	if in.Redirect != "" && !h.Redirect.ValidateRedirectURI(in.TenantID, in.ClientID, in.Redirect) {
		log.Printf(`{"level":"warn","msg":"verify_start_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, in.Redirect)
		writeErr(w, "invalid_redirect_uri", "redirect_uri not allowed", http.StatusBadRequest)
		return
	}

	// IP/UA -> NULL si están vacíos (evita "" en INET)
	ipStr := clientIPOrEmpty(r)
	uaStr := r.UserAgent()
	ipPtr := strPtrOrNil(ipStr)
	uaPtr := strPtrOrNil(uaStr)

	log.Printf(`{"level":"info","msg":"verify_start_create_token_try","request_id":"%s","tenant_id":"%s","user_id":"%s","email":"%s","ttl_sec":%d,"ip":"%s","ua":"%s"}`,
		rid, in.TenantID, userID, emailStr, int(h.VerifyTTL.Seconds()), ipStr, sanitizeUA(uaStr))

	pt, err := h.Tokens.CreateEmailVerification(r.Context(), in.TenantID, userID, emailStr, h.VerifyTTL, ipPtr, uaPtr)
	if err != nil {
		log.Printf(`{"level":"error","msg":"verify_start_create_token_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "could not create token", http.StatusInternalServerError)
		return
	}
	log.Printf(`{"level":"info","msg":"verify_start_token_ok","request_id":"%s"}`, rid)

	link := h.buildLink("/v1/auth/verify-email", pt, in.Redirect, in.ClientID)
	if h.DebugEchoLinks {
		log.Printf(`{"level":"debug","msg":"verify_start_link","request_id":"%s","link":"%s"}`, rid, link)
	}
	htmlBody, textBody, err := renderVerify(h.Tmpl, email.VerifyVars{
		UserEmail: emailStr, Tenant: in.TenantID.String(), Link: link, TTL: h.VerifyTTL.String(),
	})
	if err != nil {
		log.Printf(`{"level":"error","msg":"verify_start_template_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "template error", http.StatusInternalServerError)
		return
	}

	if err := h.Mailer.Send(emailStr, "Verificá tu email", htmlBody, textBody); err != nil {
		diag := email.DiagnoseSMTP(err)
		lvl := "error"
		if diag.Temporary {
			lvl = "warn"
		}
		log.Printf(`{"level":"%s","msg":"verify_start_mail_send_err","request_id":"%s","to":"%s","code":"%s","temporary":%t,"retry_after_sec":%d,"err":"%v"}`,
			lvl, rid, emailStr, diag.Code, diag.Temporary, int(diag.RetryAfter.Seconds()), err)
		// Respuesta sigue siendo 204 para no filtrar info al usuario.
	} else {
		log.Printf(`{"level":"info","msg":"verify_start_mail_send_ok","request_id":"%s","to":"%s"}`, rid, emailStr)
	}

	if h.DebugEchoLinks {
		w.Header().Set("X-Debug-Verify-Link", link)
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *EmailFlowsHandler) verifyEmailConfirm(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	q := r.URL.Query()
	token := q.Get("token")
	redirect := q.Get("redirect_uri")
	clientID := q.Get("client_id")
	log.Printf(`{"level":"info","msg":"verify_confirm_begin","request_id":"%s","redirect":"%s","client_id":"%s"}`, rid, redirect, clientID)

	if token == "" {
		log.Printf(`{"level":"warn","msg":"verify_confirm_missing_token","request_id":"%s"}`, rid)
		writeErr(w, "invalid_token", "token required", http.StatusBadRequest)
		return
	}

	tenantID, userID, err := h.Tokens.UseEmailVerification(r.Context(), token)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"verify_confirm_token_bad","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_token", "token invalid/expired/used", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"verify_confirm_token_ok","request_id":"%s","tenant_id":"%s","user_id":"%s"}`, rid, tenantID, userID)

	if redirect != "" && !h.Redirect.ValidateRedirectURI(tenantID, clientID, redirect) {
		log.Printf(`{"level":"warn","msg":"verify_confirm_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, redirect)
		writeErr(w, "invalid_redirect_uri", "redirect_uri not allowed", http.StatusBadRequest)
		return
	}
	if err := h.Users.SetEmailVerified(r.Context(), userID); err != nil {
		log.Printf(`{"level":"error","msg":"verify_confirm_mark_verified_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "could not mark verified", http.StatusInternalServerError)
		return
	}
	log.Printf(`{"level":"info","msg":"verify_confirm_mark_verified_ok","request_id":"%s"}`, rid)

	if redirect != "" {
		u, _ := url.Parse(redirect)
		qs := u.Query()
		qs.Set("status", "verified")
		u.RawQuery = qs.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "verified"})
}

func (h *EmailFlowsHandler) buildLink(path, token, redirect, clientID string) string {
	u, _ := url.Parse(h.BaseURL)
	u.Path = path
	q := u.Query()
	q.Set("token", token)
	if redirect != "" {
		q.Set("redirect_uri", redirect)
	}
	if clientID != "" {
		q.Set("client_id", clientID)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// --- Forgot / Reset ---

type forgotIn struct {
	TenantID uuid.UUID `json:"tenant_id"`
	ClientID string    `json:"client_id"`
	Email    string    `json:"email"`
	Redirect string    `json:"redirect_uri,omitempty"`
}

func (h *EmailFlowsHandler) forgot(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in forgotIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"forgot_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"forgot_begin","request_id":"%s","tenant_id":"%s","client_id":"%s","email":"%s","redirect":"%s"}`, rid, in.TenantID, in.ClientID, in.Email, in.Redirect)

	if in.TenantID == uuid.Nil || in.ClientID == "" || in.Email == "" {
		log.Printf(`{"level":"warn","msg":"forgot_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id, client_id, email required", http.StatusBadRequest)
		return
	}

	if h.rlOr429(w, "forgot:"+in.TenantID.String()+":"+strings.ToLower(in.Email)) {
		log.Printf(`{"level":"warn","msg":"forgot_rate_limited","request_id":"%s"}`, rid)
		return
	}

	if uid, ok, err := h.Users.LookupUserIDByEmail(r.Context(), in.TenantID, in.Email); err == nil && ok {
		if in.Redirect != "" && !h.Redirect.ValidateRedirectURI(in.TenantID, in.ClientID, in.Redirect) {
			log.Printf(`{"level":"warn","msg":"forgot_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, in.Redirect)
			in.Redirect = ""
		}

		ipStr := clientIPOrEmpty(r)
		uaStr := r.UserAgent()
		ipPtr := strPtrOrNil(ipStr)
		uaPtr := strPtrOrNil(uaStr)

		log.Printf(`{"level":"info","msg":"forgot_create_token_try","request_id":"%s","tenant_id":"%s","user_id":"%s","email":"%s","ttl_sec":%d,"ip":"%s","ua":"%s"}`,
			rid, in.TenantID, uid, in.Email, int(h.ResetTTL.Seconds()), ipStr, sanitizeUA(uaStr))

		if pt, err := h.Tokens.CreatePasswordReset(r.Context(), in.TenantID, uid, in.Email, h.ResetTTL, ipPtr, uaPtr); err == nil {
			link := h.buildLink("/v1/auth/reset", pt, in.Redirect, in.ClientID)
			if h.DebugEchoLinks {
				log.Printf(`{"level":"debug","msg":"forgot_link","request_id":"%s","link":"%s"}`, rid, link)
			}
			htmlBody, textBody, _ := renderReset(h.Tmpl, email.ResetVars{
				UserEmail: in.Email, Tenant: in.TenantID.String(), Link: link, TTL: h.ResetTTL.String(),
			})
			if err := h.Mailer.Send(in.Email, "Restablecé tu contraseña", htmlBody, textBody); err != nil {
				diag := email.DiagnoseSMTP(err)
				lvl := "error"
				if diag.Temporary {
					lvl = "warn"
				}
				log.Printf(`{"level":"%s","msg":"forgot_mail_send_err","request_id":"%s","to":"%s","code":"%s","temporary":%t,"retry_after_sec":%d,"err":"%v"}`,
					lvl, rid, in.Email, diag.Code, diag.Temporary, int(diag.RetryAfter.Seconds()), err)
			} else {
				log.Printf(`{"level":"info","msg":"forgot_mail_send_ok","request_id":"%s","to":"%s"}`, rid, in.Email)
			}
			if h.DebugEchoLinks {
				w.Header().Set("X-Debug-Reset-Link", link)
			}
		} else {
			log.Printf(`{"level":"error","msg":"forgot_create_token_err","request_id":"%s","err":"%v"}`, rid, err)
		}
	} else if err != nil {
		log.Printf(`{"level":"error","msg":"forgot_lookup_user_err","request_id":"%s","err":"%v"}`, rid, err)
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type resetIn struct {
	TenantID    uuid.UUID `json:"tenant_id"`
	ClientID    string    `json:"client_id"`
	Token       string    `json:"token"`
	NewPassword string    `json:"new_password"`
}

func (h *EmailFlowsHandler) reset(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in resetIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"reset_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"reset_begin","request_id":"%s","tenant_id":"%s","client_id":"%s"}`, rid, in.TenantID, in.ClientID)

	if in.TenantID == uuid.Nil || in.ClientID == "" || in.Token == "" || in.NewPassword == "" {
		log.Printf(`{"level":"warn","msg":"reset_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id, client_id, token, new_password required", http.StatusBadRequest)
		return
	}

	if h.rlOr429(w, "reset:"+clientIPOrEmpty(r)) {
		log.Printf(`{"level":"warn","msg":"reset_rate_limited","request_id":"%s"}`, rid)
		return
	}

	ok, reasons := h.Policy.Validate(in.NewPassword)
	if !ok {
		log.Printf(`{"level":"warn","msg":"reset_weak_password","request_id":"%s","reasons":"%s"}`, rid, strings.Join(reasons, ","))
		writeErr(w, "weak_password", strings.Join(reasons, ","), http.StatusBadRequest)
		return
	}

	tenantID, userID, err := h.Tokens.UsePasswordReset(r.Context(), in.Token)
	if err != nil || tenantID != in.TenantID {
		log.Printf(`{"level":"warn","msg":"reset_token_bad","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_token", "token invalid/expired/used", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"reset_token_ok","request_id":"%s","tenant_id":"%s","user_id":"%s"}`, rid, tenantID, userID)

	phash, err := password.Hash(password.Default, in.NewPassword)
	if err != nil {
		log.Printf(`{"level":"error","msg":"reset_hash_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "hash error", http.StatusInternalServerError)
		return
	}
	if err := h.Users.UpdatePasswordHash(r.Context(), userID, phash); err != nil {
		log.Printf(`{"level":"error","msg":"reset_update_pwd_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "update password error", http.StatusInternalServerError)
		return
	}
	_ = h.Users.RevokeAllRefreshTokens(r.Context(), userID)
	log.Printf(`{"level":"info","msg":"reset_password_updated","request_id":"%s"}`, rid)

	if h.AutoLoginReset && h.Issuer != nil {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		if err := h.Issuer.IssueTokens(w, r, in.TenantID, in.ClientID, userID); err != nil {
			log.Printf(`{"level":"error","msg":"reset_issue_tokens_err","request_id":"%s","err":"%v"}`, rid, err)
			writeErr(w, "server_error", "issue tokens error", http.StatusInternalServerError)
		} else {
			log.Printf(`{"level":"info","msg":"reset_issue_tokens_ok","request_id":"%s"}`, rid)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// templating shortcuts

func renderVerify(t *email.Templates, v email.VerifyVars) (html, text string, err error) {
	var hb, tb strings.Builder
	if err = t.VerifyHTML.Execute(&hb, v); err != nil {
		return
	}
	if err = t.VerifyTXT.Execute(&tb, v); err != nil {
		return
	}
	return hb.String(), tb.String(), nil
}

func renderReset(t *email.Templates, v email.ResetVars) (html, text string, err error) {
	var hb, tb strings.Builder
	if err = t.ResetHTML.Execute(&hb, v); err != nil {
		return
	}
	if err = t.ResetTXT.Execute(&tb, v); err != nil {
		return
	}
	return hb.String(), tb.String(), nil
}

// sanitiza user-agent para no romper logs
func sanitizeUA(ua string) string {
	return strings.ReplaceAll(ua, `"`, `'`)
}

// IP desde XFF o RemoteAddr; si falla, string vacío
func clientIPOrEmpty(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		parts := strings.Split(xf, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return ""
}

// string -> *string (nil si vacío)
func strPtrOrNil(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	v := s
	return &v
}
