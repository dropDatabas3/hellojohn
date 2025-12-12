package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	texttpl "text/template"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/dropDatabas3/hellojohn/internal/email"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
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
	Tokens *store.TokenStore
	Users  *store.UserStore
	// Mailer        email.Sender // Removed in favor of provider
	SenderProvider email.SenderProvider
	Tmpl           *email.Templates
	Policy         password.Policy
	Redirect       RedirectValidator
	Issuer         TokenIssuer
	Auth           CurrentUserProvider
	Limiter        RateLimiter
	BlacklistPath  string
	Provider       controlplane.ControlPlane

	// Phase 4.1: per-tenant DB gating for email flows
	TenantMgr *tenantsql.Manager

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
	TenantID string `json:"tenant_id"`
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Redirect string `json:"redirect_uri,omitempty"`
}

func (h *EmailFlowsHandler) verifyEmailStart(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in verifyStartIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"verify_start_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}

	// Resolve tenant ID by UUID or slug
	var tenantID uuid.UUID
	if parsed, err := uuid.Parse(in.TenantID); err == nil {
		tenantID = parsed
	} else if h.Provider != nil {
		if tenant, err := h.Provider.GetTenantBySlug(r.Context(), in.TenantID); err == nil && tenant != nil {
			if parsed, err := uuid.Parse(tenant.ID); err == nil {
				tenantID = parsed
			}
		}
	}

	log.Printf(`{"level":"info","msg":"verify_start_begin","request_id":"%s","tenant_id":"%s","client_id":"%s","redirect":"%s"}`, rid, tenantID, in.ClientID, in.Redirect)

	if tenantID == uuid.Nil || in.ClientID == "" {
		log.Printf(`{"level":"warn","msg":"verify_start_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id and client_id required", http.StatusBadRequest)
		return
	}

	// Gate by tenant DB availability (Phase 4.1). If TenantMgr is present, try opening repo.
	if h.TenantMgr != nil {
		if _, err := helpers.OpenTenantRepo(r.Context(), h.TenantMgr, tenantID.String()); err != nil {
			// Mirror 501/500 semantics
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
	}

	if h.rlOr429(w, "verify_start:"+clientIPOrEmpty(r)) {
		log.Printf(`{"level":"warn","msg":"verify_start_rate_limited","request_id":"%s","remote":"%s"}`, rid, clientIPOrEmpty(r))
		return
	}

	userID, err := h.Auth.CurrentUserID(r)
	var emailStr string

	// Resolve TokenStore (prefer Tenant DB) explicitly for this request
	var userStore = h.Users // fallback default

	if h.TenantMgr != nil {
		if tenantDB, err := h.TenantMgr.GetPG(r.Context(), tenantID.String()); err == nil {
			userStore = &store.UserStore{DB: tenantDB.Pool()}
		} else {
			log.Printf(`{"level":"warn","msg":"verify_start_tenant_db_err","err":"%v"}`, err)
		}
	}

	if err == nil {
		// Authenticated request
		emailStr, _ = h.Auth.CurrentUserEmail(r)
		if emailStr == "" {
			if e, err := h.Users.GetEmailByID(r.Context(), userID); err == nil && e != "" {
				emailStr = e
			}
		}
	} else {
		// Unauthenticated request (resend flow)
		if in.Email == "" {
			writeErr(w, "login_required", "Bearer or email required", http.StatusUnauthorized)
			return
		}

		// Rate limit by IP for public endpoint
		if h.rlOr429(w, "verify_resend:"+clientIPOrEmpty(r)) {
			return
		}

		uid, ok, err := userStore.LookupUserIDByEmail(r.Context(), tenantID, in.Email)
		if err != nil {
			log.Printf(`{"level":"error","msg":"verify_lookup_error","request_id":"%s","err":"%v"}`, rid, err)
			writeErr(w, "server_error", "lookup failed", http.StatusInternalServerError)
			return
		}
		if !ok {
			// User not found. Return 204 to avoid enumeration.
			log.Printf(`{"level":"info","msg":"verify_user_not_found","request_id":"%s","email":"%s"}`, rid, in.Email)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		userID = uid
		emailStr = in.Email
	}

	if emailStr == "" {
		writeErr(w, "server_error", "user email not found", http.StatusInternalServerError)
		return
	}

	if in.Redirect != "" && !h.Redirect.ValidateRedirectURI(tenantID, in.ClientID, in.Redirect) {
		log.Printf(`{"level":"warn","msg":"verify_start_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, in.Redirect)
		writeErr(w, "invalid_redirect_uri", "redirect_uri not allowed", http.StatusBadRequest)
		return
	}

	// Delegate to shared method
	if err := h.SendVerificationEmail(r.Context(), rid, tenantID, userID, emailStr, in.Redirect, in.ClientID); err != nil {
		// SendVerificationEmail logs errors. We just map to HTTP status if needed or return 500.
		// Since validation errors (like redirect) happen inside, we might want to move validation OUT.
		// For now, assume global error is 500.
		writeErr(w, "server_error", "could not send verification email", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SendVerificationEmail sends the validation email. Exposed for Register handler.
func (h *EmailFlowsHandler) SendVerificationEmail(ctx context.Context, rid string, tenantID, userID uuid.UUID, emailStr, redirect, clientID string) error {
	// IP/UA handling
	var ipPtr, uaPtr *string
	// extracting from context is hard without request, so we pass defaults or nil if internal

	// Resolve TokenStore (prefer Tenant DB) explicitly
	var tokenStore = h.Tokens // fallback default
	if h.TenantMgr != nil {
		if tenantDB, err := h.TenantMgr.GetPG(ctx, tenantID.String()); err == nil {
			tokenStore = store.NewTokenStore(tenantDB.Pool())
		} else {
			log.Printf(`{"level":"warn","msg":"verify_send_tenant_db_err","err":"%v"}`, err)
		}
	}

	log.Printf(`{"level":"info","msg":"verify_send_create_token_try","request_id":"%s","tenant_id":"%s","user_id":"%s","email":"%s","ttl_sec":%d}`,
		rid, tenantID, userID, emailStr, int(h.VerifyTTL.Seconds()))

	// Use local tokenStore
	pt, err := tokenStore.CreateEmailVerification(ctx, tenantID, userID, emailStr, h.VerifyTTL, ipPtr, uaPtr)
	if err != nil {
		log.Printf(`{"level":"error","msg":"verify_send_create_token_err","request_id":"%s","err":"%v"}`, rid, err)
		return err
	}
	log.Printf(`{"level":"info","msg":"verify_send_token_ok","request_id":"%s"}`, rid)

	link := h.buildLink("/v1/auth/verify-email", pt, redirect, clientID, tenantID.String())
	if h.DebugEchoLinks {
		log.Printf(`{"level":"debug","msg":"verify_send_link","request_id":"%s","link":"%s"}`, rid, link)
	}

	// Fetch tenant settings for templates
	tenant, err := h.Provider.GetTenantByID(ctx, tenantID.String())
	if err != nil {
		log.Printf(`{"level":"warn","msg":"verify_send_tenant_fetch_err","err":"%v"}`, err)
	}

	tenantName := tenantID.String()
	if tenant != nil {
		tenantName = tenant.Name
	}

	htmlBody, textBody, err := renderVerify(h.Tmpl, tenant, email.VerifyVars{
		UserEmail: emailStr, Tenant: tenantName, Link: link, TTL: h.VerifyTTL.String(),
	})
	if err != nil {
		log.Printf(`{"level":"error","msg":"verify_send_template_err","request_id":"%s","err":"%v"}`, rid, err)
		return err
	}

	// Resolve sender
	sender, err := h.SenderProvider.GetSender(ctx, tenantID)
	if err != nil {
		log.Printf(`{"level":"error","msg":"verify_send_sender_err","request_id":"%s","err":"%v"}`, rid, err)
		return err
	}

	if err := sender.Send(emailStr, "Verificá tu email", htmlBody, textBody); err != nil {
		diag := email.DiagnoseSMTP(err)
		lvl := "error"
		if diag.Temporary {
			lvl = "warn"
		}
		log.Printf(`{"level":"%s","msg":"verify_send_mail_err","request_id":"%s","to":"%s","code":"%s","temporary":%t,"retry_after_sec":%d,"err":"%v"}`,
			lvl, rid, emailStr, diag.Code, diag.Temporary, int(diag.RetryAfter.Seconds()), err)
		// Return error? VerifyStart consumed it without error.
		// Register might want to know? Unlikely to rollback register.
		// Just log and return nil (soft failure) or error?
		// Register: "registro exitoso... debe ejecutar flujo". If mail fails, user created but not verified.
		// We return nil to avoid breaking flow?
		return nil
	}
	log.Printf(`{"level":"info","msg":"verify_send_mail_ok","request_id":"%s","to":"%s"}`, rid, emailStr)
	return nil
}

func (h *EmailFlowsHandler) verifyEmailConfirm(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	q := r.URL.Query()
	token := q.Get("token")
	redirect := q.Get("redirect_uri")
	clientID := q.Get("client_id")
	tenantIDParam := q.Get("tenant_id")
	log.Printf(`{"level":"info","msg":"verify_confirm_begin","request_id":"%s","redirect":"%s","client_id":"%s","tenant_id":"%s"}`, rid, redirect, clientID, tenantIDParam)

	if token == "" {
		log.Printf(`{"level":"warn","msg":"verify_confirm_missing_token","request_id":"%s"}`, rid)
		writeErr(w, "invalid_token", "token required", http.StatusBadRequest)
		return
	}

	// Determine which TokenStore and UserStore to use based on tenant_id
	var tokenStore *store.TokenStore = h.Tokens
	var userStore *store.UserStore = h.Users
	var tenantID uuid.UUID
	if tenantIDParam != "" {
		if parsed, err := uuid.Parse(tenantIDParam); err == nil {
			tenantID = parsed
		}
	}

	if tenantIDParam != "" && h.TenantMgr != nil {
		// Use tenant-specific stores
		tenantDB, err := h.TenantMgr.GetPG(r.Context(), tenantIDParam)
		if err == nil {
			tokenStore = store.NewTokenStore(tenantDB.Pool())
			userStore = &store.UserStore{DB: tenantDB.Pool()}
			log.Printf(`{"level":"debug","msg":"verify_using_tenant_db","request_id":"%s","tenant_id":"%s"}`, rid, tenantIDParam)
		} else {
			log.Printf(`{"level":"warn","msg":"verify_tenant_db_err","request_id":"%s","err":"%v"}`, rid, err)
		}
	}

	_, userID, err := tokenStore.UseEmailVerification(r.Context(), token)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"verify_confirm_token_bad","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_token", "token invalid/expired/used", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"verify_confirm_token_ok","request_id":"%s","tenant_id":"%s","user_id":"%s"}`, rid, tenantID, userID)

	// If redirect is empty, try to resolve a default from client config
	if redirect == "" && clientID != "" && h.Provider != nil {
		// We need tenant slug for GetClient. We only have tenantID (UUID).
		// Try to resolve slug from ID or assume we can look it up.
		// Since we already might have fetched tenant for store, reuse or fetch.
		var tenantSlug string
		if t, err := h.Provider.GetTenantByID(r.Context(), tenantID.String()); err == nil && t != nil {
			tenantSlug = t.Slug
		}

		if tenantSlug != "" {
			if c, err := h.Provider.GetClient(r.Context(), tenantSlug, clientID); err == nil && c != nil {
				// Priority: User defined RedirectURIs > VerifyEmailURL
				if len(c.RedirectURIs) > 0 {
					redirect = c.RedirectURIs[0]
				} else if c.VerifyEmailURL != "" {
					redirect = c.VerifyEmailURL
				}
				log.Printf(`{"level":"info","msg":"verify_confirm_resolved_default_redirect","request_id":"%s","redirect":"%s"}`, rid, redirect)
			}
		}
	}

	if redirect != "" && !h.Redirect.ValidateRedirectURI(tenantID, clientID, redirect) {
		log.Printf(`{"level":"warn","msg":"verify_confirm_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, redirect)
		writeErr(w, "invalid_redirect_uri", "redirect_uri not allowed", http.StatusBadRequest)
		return
	}
	if err := userStore.SetEmailVerified(r.Context(), userID); err != nil {
		log.Printf(`{"level":"error","msg":"verify_confirm_mark_verified_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "could not mark verified", http.StatusInternalServerError)
		return
	}
	log.Printf(`{"level":"info","msg":"verify_confirm_mark_verified_ok","request_id":"%s"}`, rid)

	if redirect != "" {
		u, _ := url.Parse(redirect)
		qs := u.Query()
		qs.Set("status", "verified")
		// Also add token/email hints if needed? No, just status usually enough.
		u.RawQuery = qs.Encode()
		http.Redirect(w, r, u.String(), http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "verified"})
}

func (h *EmailFlowsHandler) buildLink(path, token, redirect, clientID, tenantID string) string {
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
	if tenantID != "" {
		q.Set("tenant_id", tenantID)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// --- Forgot / Reset ---

type forgotIn struct {
	TenantID string `json:"tenant_id"` // Can be UUID or slug
	ClientID string `json:"client_id"`
	Email    string `json:"email"`
	Redirect string `json:"redirect_uri,omitempty"`
}

func (h *EmailFlowsHandler) forgot(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in forgotIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"forgot_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}

	// Resolve tenant ID (can be UUID or slug)
	var tenantID uuid.UUID
	if parsed, err := uuid.Parse(in.TenantID); err == nil {
		tenantID = parsed
	} else if h.Provider != nil {
		// Try to lookup by slug
		if tenant, err := h.Provider.GetTenantBySlug(r.Context(), in.TenantID); err == nil && tenant != nil {
			if parsed, err := uuid.Parse(tenant.ID); err == nil {
				tenantID = parsed
			}
		}
	}

	if h.TenantMgr != nil && tenantID != uuid.Nil {
		if _, err := helpers.OpenTenantRepo(r.Context(), h.TenantMgr, tenantID.String()); err != nil {
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
	}
	log.Printf(`{"level":"info","msg":"forgot_begin","request_id":"%s","tenant_id":"%s","client_id":"%s","email":"%s","redirect":"%s"}`, rid, tenantID, in.ClientID, in.Email, in.Redirect)

	if tenantID == uuid.Nil || in.ClientID == "" || in.Email == "" {
		log.Printf(`{"level":"warn","msg":"forgot_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id, client_id, email required", http.StatusBadRequest)
		return
	}

	// Get tenant DB for user lookup AND token creation
	var userStore *store.UserStore
	var tokenStore = h.Tokens // fallback
	if h.TenantMgr != nil {
		tenantDB, err := h.TenantMgr.GetPG(r.Context(), in.TenantID) // Use slug for lookup
		if err != nil {
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
		userStore = &store.UserStore{DB: tenantDB.Pool()}
		tokenStore = store.NewTokenStore(tenantDB.Pool())
	} else {
		userStore = h.Users
	}

	if h.rlOr429(w, "forgot:"+tenantID.String()+":"+strings.ToLower(in.Email)) {
		log.Printf(`{"level":"warn","msg":"forgot_rate_limited","request_id":"%s"}`, rid)
		return
	}

	if uid, ok, err := userStore.LookupUserIDByEmail(r.Context(), tenantID, in.Email); err == nil && ok {
		if in.Redirect != "" && !h.Redirect.ValidateRedirectURI(tenantID, in.ClientID, in.Redirect) {
			log.Printf(`{"level":"warn","msg":"forgot_invalid_redirect","request_id":"%s","redirect":"%s"}`, rid, in.Redirect)
			in.Redirect = ""
		}

		ipStr := clientIPOrEmpty(r)
		uaStr := r.UserAgent()
		ipPtr := strPtrOrNil(ipStr)
		uaPtr := strPtrOrNil(uaStr)

		log.Printf(`{"level":"info","msg":"forgot_create_token_try","request_id":"%s","tenant_id":"%s","user_id":"%s","email":"%s","ttl_sec":%d,"ip":"%s","ua":"%s"}`,
			rid, tenantID, uid, in.Email, int(h.ResetTTL.Seconds()), ipStr, sanitizeUA(uaStr))

		if pt, err := tokenStore.CreatePasswordReset(r.Context(), tenantID, uid, in.Email, h.ResetTTL, ipPtr, uaPtr); err == nil {
			// Fetch tenant for templates and client config
			tenant, _ := h.Provider.GetTenantBySlug(r.Context(), in.TenantID)

			// Lookup client to check for custom reset URL
			var link string
			if h.Provider != nil {
				if client, err := h.Provider.GetClient(r.Context(), tenant.Slug, in.ClientID); err == nil && client != nil && client.ResetPasswordURL != "" {
					// Use custom reset URL from client config
					u, _ := url.Parse(client.ResetPasswordURL)
					q := u.Query()
					q.Set("token", pt)
					u.RawQuery = q.Encode()
					link = u.String()
					log.Printf(`{"level":"info","msg":"forgot_using_custom_reset_url","request_id":"%s","url":"%s"}`, rid, client.ResetPasswordURL)
				}
			}
			if link == "" {
				// Fallback to backend reset endpoint
				link = h.buildLink("/v1/auth/reset", pt, in.Redirect, in.ClientID, tenantID.String())
			}

			if h.DebugEchoLinks {
				log.Printf(`{"level":"debug","msg":"forgot_link","request_id":"%s","link":"%s"}`, rid, link)
			}

			// Get tenant display name for email template
			tenantDisplayName := in.TenantID
			if tenant != nil && tenant.DisplayName != "" {
				tenantDisplayName = tenant.DisplayName
			} else if tenant != nil && tenant.Name != "" {
				tenantDisplayName = tenant.Name
			}

			htmlBody, textBody, _ := renderReset(h.Tmpl, tenant, email.ResetVars{
				UserEmail: in.Email, Tenant: tenantDisplayName, Link: link, TTL: h.ResetTTL.String(),
			})
			// Resolve sender
			sender, err := h.SenderProvider.GetSender(r.Context(), tenantID)
			if err != nil {
				log.Printf(`{"level":"error","msg":"forgot_sender_err","request_id":"%s","err":"%v"}`, rid, err)
				writeErr(w, "email_config_error", "Error de configuración de email", http.StatusInternalServerError)
				return
			}

			if err := sender.Send(in.Email, "Restablecé tu contraseña", htmlBody, textBody); err != nil {
				diag := email.DiagnoseSMTP(err)
				lvl := "error"
				if diag.Temporary {
					lvl = "warn"
				}
				log.Printf(`{"level":"%s","msg":"forgot_mail_send_err","request_id":"%s","to":"%s","code":"%s","temporary":%t,"retry_after_sec":%d,"err":"%v"}`,
					lvl, rid, in.Email, diag.Code, diag.Temporary, int(diag.RetryAfter.Seconds()), err)
				writeErr(w, "email_send_failed", "Error al enviar el email de recuperación", http.StatusServiceUnavailable)
				return
			}
			log.Printf(`{"level":"info","msg":"forgot_mail_send_ok","request_id":"%s","to":"%s"}`, rid, in.Email)

			if h.DebugEchoLinks {
				w.Header().Set("X-Debug-Reset-Link", link)
			}
		} else {
			log.Printf(`{"level":"error","msg":"forgot_create_token_err","request_id":"%s","err":"%v"}`, rid, err)
			writeErr(w, "token_error", "Error interno al procesar la solicitud", http.StatusInternalServerError)
			return
		}
	} else if err != nil {
		log.Printf(`{"level":"error","msg":"forgot_lookup_user_err","request_id":"%s","err":"%v"}`, rid, err)
		// For security, still return OK to not reveal if email exists
	}
	// Note: We return OK even if user not found (security: don't reveal if email exists)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type resetIn struct {
	TenantID    string `json:"tenant_id"` // Can be UUID or slug
	ClientID    string `json:"client_id"`
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

func (h *EmailFlowsHandler) reset(w http.ResponseWriter, r *http.Request) {
	rid := w.Header().Get("X-Request-ID")

	var in resetIn
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		log.Printf(`{"level":"warn","msg":"reset_decode","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_json", "Malformed body", http.StatusBadRequest)
		return
	}

	// Resolve tenant ID (can be UUID or slug)
	var tenantID uuid.UUID
	if parsed, err := uuid.Parse(in.TenantID); err == nil {
		tenantID = parsed
	} else if h.Provider != nil {
		// Try to lookup by slug
		if tenant, err := h.Provider.GetTenantBySlug(r.Context(), in.TenantID); err == nil && tenant != nil {
			if parsed, err := uuid.Parse(tenant.ID); err == nil {
				tenantID = parsed
			}
		}
	}

	log.Printf(`{"level":"info","msg":"reset_begin","request_id":"%s","tenant_id":"%s","client_id":"%s"}`, rid, tenantID, in.ClientID)

	if tenantID == uuid.Nil || in.ClientID == "" || in.Token == "" || in.NewPassword == "" {
		log.Printf(`{"level":"warn","msg":"reset_missing_fields","request_id":"%s"}`, rid)
		writeErr(w, "missing_fields", "tenant_id, client_id, token, new_password required", http.StatusBadRequest)
		return
	}

	// Get tenant DB and create per-tenant UserStore AND TokenStore
	var userStore *store.UserStore
	var tokenStore = h.Tokens // fallback
	if h.TenantMgr != nil {
		tenantDB, err := h.TenantMgr.GetPG(r.Context(), in.TenantID)
		if err != nil {
			if helpers.IsNoDBForTenant(err) {
				httpx.WriteTenantDBMissing(w)
				return
			}
			httpx.WriteTenantDBError(w, err.Error())
			return
		}
		userStore = &store.UserStore{DB: tenantDB.Pool()}
		tokenStore = store.NewTokenStore(tenantDB.Pool())
	} else {
		userStore = h.Users
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

	// Use tenant-specific token store. tenantID returned is nil/ignored as check is implicit by using Tenant DB
	_, userID, err := tokenStore.UsePasswordReset(r.Context(), in.Token)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"reset_token_bad","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "invalid_token", "token invalid/expired/used", http.StatusBadRequest)
		return
	}
	log.Printf(`{"level":"info","msg":"reset_token_ok","request_id":"%s","tenant_id":"%s","user_id":"%s"}`, rid, tenantID, userID)

	// Blacklist opcional (config/ENV)
	if p := strings.TrimSpace(h.BlacklistPath); p != "" {
		if bl, err := password.GetCachedBlacklist(p); err == nil && bl.Contains(in.NewPassword) {
			writeErr(w, "policy_violation", "password no permitido por política", http.StatusBadRequest)
			return
		}
	}

	phash, err := password.Hash(password.Default, in.NewPassword)
	if err != nil {
		log.Printf(`{"level":"error","msg":"reset_hash_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "hash error", http.StatusInternalServerError)
		return
	}
	if err := userStore.UpdatePasswordHash(r.Context(), userID, phash); err != nil {
		log.Printf(`{"level":"error","msg":"reset_update_pwd_err","request_id":"%s","err":"%v"}`, rid, err)
		writeErr(w, "server_error", "update password error", http.StatusInternalServerError)
		return
	}
	_ = userStore.RevokeAllRefreshTokens(r.Context(), userID)
	log.Printf(`{"level":"info","msg":"reset_password_updated","request_id":"%s"}`, rid)

	if h.AutoLoginReset && h.Issuer != nil {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		if err := h.Issuer.IssueTokens(w, r, tenantID, in.ClientID, userID); err != nil {
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

func renderVerify(t *email.Templates, tenant *controlplane.Tenant, v email.VerifyVars) (html, text string, err error) {
	// Check overrides
	if tenant != nil && tenant.Settings.Mailing != nil && tenant.Settings.Mailing.Templates != nil {
		if tpl, ok := tenant.Settings.Mailing.Templates[email.TemplateVerify]; ok {
			return renderOverride(tpl, v)
		}
	}

	var hb, tb strings.Builder
	if err = t.VerifyHTML.Execute(&hb, v); err != nil {
		return
	}
	if err = t.VerifyTXT.Execute(&tb, v); err != nil {
		return
	}
	return hb.String(), tb.String(), nil
}

func renderReset(t *email.Templates, tenant *controlplane.Tenant, v email.ResetVars) (html, text string, err error) {
	// Check overrides
	if tenant != nil && tenant.Settings.Mailing != nil && tenant.Settings.Mailing.Templates != nil {
		if tpl, ok := tenant.Settings.Mailing.Templates[email.TemplateReset]; ok {
			return renderOverride(tpl, v)
		}
	}

	var hb, tb strings.Builder
	if err = t.ResetHTML.Execute(&hb, v); err != nil {
		return
	}
	if err = t.ResetTXT.Execute(&tb, v); err != nil {
		return
	}
	return hb.String(), tb.String(), nil
}

func renderOverride(tpl controlplane.EmailTemplate, v any) (html, text string, err error) {
	if tpl.Body == "" {
		return "", "", nil
	}
	t, err := texttpl.New("override").Parse(tpl.Body)
	if err != nil {
		return "", "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, v); err != nil {
		return "", "", err
	}
	// For now return same content for HTML and Text
	return buf.String(), buf.String(), nil
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
