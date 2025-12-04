package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	rdb "github.com/redis/go-redis/v9"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
	"github.com/dropDatabas3/hellojohn/internal/email"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/rate"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// ───────────────────── Adaptadores para Email Flows ─────────────────────

type flowsLimiterAdapter struct{ inner rate.Limiter }

func (a flowsLimiterAdapter) Allow(key string) (bool, time.Duration) {
	if a.inner == nil {
		return true, 0
	}
	res, err := a.inner.Allow(context.Background(), "mailflows:"+key)
	if err != nil {
		log.Printf(`{"level":"warn","msg":"flows_rl_error","key":"%s","err":"%v"}`, key, err)
		return true, 0
	}
	return res.Allowed, res.RetryAfter
}

type redirectValidatorAdapter struct{ repo core.Repository }

func (v redirectValidatorAdapter) ValidateRedirectURI(tenantID uuid.UUID, clientID, redirectURI string) bool {
	if clientID == "" || redirectURI == "" {
		return false
	}
	cl, _, err := v.repo.GetClientByClientID(context.Background(), clientID)
	if err != nil || cl == nil {
		log.Printf(`{"level":"warn","msg":"redirect_validate_no_client","client_id":"%s","err":"%v"}`, clientID, err)
		return false
	}
	if !strings.EqualFold(cl.TenantID, tenantID.String()) {
		log.Printf(`{"level":"warn","msg":"redirect_validate_bad_tenant","client_id":"%s","expected_tid":"%s","client_tid":"%s"}`, clientID, tenantID, cl.TenantID)
		return false
	}
	for _, ru := range cl.RedirectURIs {
		if ru == redirectURI {
			return true
		}
	}
	log.Printf(`{"level":"warn","msg":"redirect_validate_not_allowed","client_id":"%s","redirect":"%s"}`, clientID, redirectURI)
	return false
}

type tokenIssuerAdapter struct {
	c          *app.Container
	refreshTTL time.Duration
}

func (ti tokenIssuerAdapter) IssueTokens(w http.ResponseWriter, r *http.Request, tenantID uuid.UUID, clientID string, userID uuid.UUID) error {
	rid := w.Header().Get("X-Request-ID")

	cl, _, err := ti.c.Store.GetClientByClientID(r.Context(), clientID)
	if err != nil || cl == nil || !strings.EqualFold(cl.TenantID, tenantID.String()) {
		log.Printf(`{"level":"warn","msg":"issuer_invalid_client","request_id":"%s","client_id":"%s","tenant_id":"%s","err":"%v"}`, rid, clientID, tenantID, err)
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1203)
		return nil
	}

	std := map[string]any{"tid": tenantID.String(), "amr": []string{"reset"}}
	custom := map[string]any{}
	// Hook + SYS namespace (Fase 2)
	std, custom = applyAccessClaimsHook(r.Context(), ti.c, tenantID.String(), clientID, userID.String(), []string{}, []string{"reset"}, std, custom)
	if u, err := ti.c.Store.GetUserByID(r.Context(), userID.String()); err == nil && u != nil {
		type rbacReader interface {
			GetUserRoles(ctx context.Context, userID string) ([]string, error)
			GetUserPermissions(ctx context.Context, userID string) ([]string, error)
		}
		var roles, perms []string
		if rr, ok := ti.c.Store.(rbacReader); ok {
			roles, _ = rr.GetUserRoles(r.Context(), userID.String())
			perms, _ = rr.GetUserPermissions(r.Context(), userID.String())
		}
		custom = helpers.PutSystemClaimsV2(custom, ti.c.Issuer.Iss, u.Metadata, roles, perms)
	}

	access, exp, err := ti.c.Issuer.IssueAccess(userID.String(), clientID, std, custom)
	if err != nil {
		log.Printf(`{"level":"error","msg":"issuer_issue_access_err","request_id":"%s","err":"%v"}`, rid, err)
		httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir el access token", 1204)
		return nil
	}
	log.Printf(`{"level":"info","msg":"issuer_issue_access_ok","request_id":"%s","expires_in_sec":%d}`, rid, int(time.Until(exp).Seconds()))

	var rawRT string
	// IMPORTANTE: usamos CreateRefreshTokenTC (tenant + client_id_text) que hashea con SHA256+hex.
	// No volver a CreateRefreshToken (legacy, Base64URL) salvo en fallback cuando el store no implemente TC.
	if tcs, ok := ti.c.Store.(interface {
		CreateRefreshTokenTC(ctx context.Context, tenantID, clientIDText, userID string, ttl time.Duration) (string, error)
	}); ok {
		// Preferir TC: (tenant + client_id_text) y hash SHA256+hex interno
		rawRT, err = tcs.CreateRefreshTokenTC(r.Context(), tenantID.String(), clientID, userID.String(), ti.refreshTTL)
		if err != nil {
			log.Printf(`{"level":"error","msg":"issuer_persist_refresh_tc_err","request_id":"%s","err":"%v"}`, rid, err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1206)
			return nil
		}
	} else {
		// Fallback legacy (solo si el store no soporta TC)
		rawRT, err = tokens.GenerateOpaqueToken(32)
		if err != nil {
			log.Printf(`{"level":"error","msg":"issuer_gen_refresh_err","request_id":"%s","err":"%v"}`, rid, err)
			httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1205)
			return nil
		}
		hash := tokens.SHA256Base64URL(rawRT)
		if _, err := ti.c.Store.CreateRefreshToken(r.Context(), userID.String(), cl.ID, hash, time.Now().Add(ti.refreshTTL), nil); err != nil {
			log.Printf(`{"level":"error","msg":"issuer_persist_refresh_legacy_err","request_id":"%s","err":"%v"}`, rid, err)
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1206)
			return nil
		}
	}
	log.Printf(`{"level":"info","msg":"issuer_refresh_ok","request_id":"%s"}`, rid)

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(AuthLoginResponse{
		AccessToken:  access,
		TokenType:    "Bearer",
		ExpiresIn:    int64(time.Until(exp).Seconds()),
		RefreshToken: rawRT,
	})
}

type currentUserProviderAdapter struct{ issuer *jwtx.Issuer }

func (a currentUserProviderAdapter) parse(r *http.Request) (jwtv5.MapClaims, error) {
	ah := strings.TrimSpace(r.Header.Get("Authorization"))
	if ah == "" || !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
		return nil, http.ErrNoCookie
	}
	raw := strings.TrimSpace(ah[len("Bearer "):])
	tk, err := jwtv5.Parse(raw,
		a.issuer.Keyfunc(), // ← usa keystore/JWKS (resuelve según 'kid')
		jwtv5.WithValidMethods([]string{"EdDSA"}),
		jwtv5.WithIssuer(a.issuer.Iss),
	)
	if err != nil || !tk.Valid {
		return nil, http.ErrNoCookie
	}
	if c, ok := tk.Claims.(jwtv5.MapClaims); ok {
		return c, nil
	}
	return nil, http.ErrNoCookie
}
func (a currentUserProviderAdapter) CurrentUserID(r *http.Request) (uuid.UUID, error) {
	c, err := a.parse(r)
	if err != nil {
		return uuid.Nil, err
	}
	if s, _ := c["sub"].(string); s != "" {
		return uuid.Parse(s)
	}
	return uuid.Nil, http.ErrNoCookie
}
func (a currentUserProviderAdapter) CurrentTenantID(r *http.Request) (uuid.UUID, error) {
	c, err := a.parse(r)
	if err != nil {
		return uuid.Nil, err
	}
	if s, _ := c["tid"].(string); s != "" {
		return uuid.Parse(s)
	}
	return uuid.Nil, http.ErrNoCookie
}
func (a currentUserProviderAdapter) CurrentUserEmail(r *http.Request) (string, error) {
	c, err := a.parse(r)
	if err != nil {
		return "", err
	}
	if s, _ := c["email"].(string); s != "" {
		return s, nil
	}
	if m, ok := c["custom"].(map[string]any); ok {
		if s, _ := m["email"].(string); s != "" {
			return s, nil
		}
	}
	return "", nil
}

// ───────────────────── Builder público ─────────────────────

func BuildEmailFlowHandlers(
	ctx context.Context,
	cfg *config.Config,
	c *app.Container,
	refreshTTL time.Duration,
) (verifyStart http.Handler, verifyConfirm http.Handler, forgot http.Handler, reset http.Handler, cleanup func(), err error) {

	// Mailer initialization removed in favor of SenderProvider
	// mailer := email.NewSMTPSender(cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From, cfg.SMTP.Username, cfg.SMTP.Password)
	// mailer.TLSMode = cfg.SMTP.TLS
	// mailer.InsecureSkipVerify = cfg.SMTP.InsecureSkipVerify
	// log.Printf(`{"level":"info","msg":"email_wiring_mailer","host":"%s","port":%d,"from":"%s","tls_mode":"%s"}`, cfg.SMTP.Host, cfg.SMTP.Port, cfg.SMTP.From, cfg.SMTP.TLS)

	// Templates
	tmpls, err := email.LoadTemplates(cfg.Email.TemplatesDir)
	if err != nil {
		log.Printf(`{"level":"error","msg":"email_wiring_templates_err","dir":"%s","err":"%v"}`, cfg.Email.TemplatesDir, err)
		return nil, nil, nil, nil, func() {}, err
	}
	log.Printf(`{"level":"info","msg":"email_wiring_templates_ok","dir":"%s"}`, cfg.Email.TemplatesDir)

	// Password policy
	pol := password.Policy{
		MinLength:     cfg.Security.PasswordPolicy.MinLength,
		RequireUpper:  cfg.Security.PasswordPolicy.RequireUpper,
		RequireLower:  cfg.Security.PasswordPolicy.RequireLower,
		RequireDigit:  cfg.Security.PasswordPolicy.RequireDigit,
		RequireSymbol: cfg.Security.PasswordPolicy.RequireSymbol,
	}
	log.Printf(`{"level":"info","msg":"email_wiring_policy","min_len":%d,"upper":%t,"lower":%t,"digit":%t,"symbol":%t}`,
		pol.MinLength, pol.RequireUpper, pol.RequireLower, pol.RequireDigit, pol.RequireSymbol)

	// Limiter (opcional, Redis)
	var flowsLimiter RateLimiter
	if cfg.Rate.Enabled && strings.EqualFold(cfg.Cache.Kind, "redis") {
		rc := rdb.NewClient(&rdb.Options{
			Addr: cfg.Cache.Redis.Addr,
			DB:   cfg.Cache.Redis.DB,
		})
		win, _ := time.ParseDuration(cfg.Rate.Window)
		if win == 0 {
			win = time.Minute
		}
		rl := rate.NewRedisLimiter(rc, cfg.Cache.Redis.Prefix+"rl:", cfg.Rate.MaxRequests, win)
		flowsLimiter = flowsLimiterAdapter{inner: rl}
		log.Printf(`{"level":"info","msg":"email_wiring_rate","enabled":true,"window":"%s","max":%d}`, win, cfg.Rate.MaxRequests)
	} else {
		log.Printf(`{"level":"info","msg":"email_wiring_rate","enabled":false}`)
	}

	// Stores: reusar pool existente si es posible para evitar "too many connections"
	var dbOps store.DBOps
	if pgStore, ok := c.Store.(interface{ Pool() *pgxpool.Pool }); ok {
		dbOps = pgStore.Pool()
		log.Printf(`{"level":"info","msg":"email_wiring_reuse_pool"}`)
		cleanup = func() {} // Pool managed by main store
	} else {
		// Fallback: connect manually (should not happen if hasGlobalDB is true and driver is PG)
		log.Printf(`{"level":"warn","msg":"email_wiring_new_conn","reason":"store_not_pool_compatible"}`)
		pgxConn, err := pgx.Connect(ctx, cfg.Storage.DSN)
		if err != nil {
			log.Printf(`{"level":"error","msg":"email_wiring_pgx_connect_err","err":"%v"}`, err)
			return nil, nil, nil, nil, func() {}, err
		}
		dbOps = pgxConn
		cleanup = func() { _ = pgxConn.Close(ctx) }
	}
	log.Printf(`{"level":"info","msg":"email_wiring_pgx_ok"}`)

	ts := store.NewTokenStore(dbOps)
	us := &store.UserStore{DB: dbOps} // sin constructor

	// Adapters
	rvalidator := redirectValidatorAdapter{repo: c.Store}
	tissuer := tokenIssuerAdapter{c: c, refreshTTL: refreshTTL}
	authProv := currentUserProviderAdapter{issuer: c.Issuer}

	// Handler principal
	ef := &EmailFlowsHandler{
		Tokens:         ts,
		Users:          us,
		SenderProvider: c.SenderProvider,
		Tmpl:           tmpls,
		Policy:         pol,
		Redirect:       rvalidator,
		Issuer:         tissuer,
		Auth:           authProv,
		Limiter:        flowsLimiter,
		BaseURL:        cfg.Email.BaseURL,
		VerifyTTL:      cfg.Auth.Verify.TTL,
		ResetTTL:       cfg.Auth.Reset.TTL,
		AutoLoginReset: cfg.Auth.Reset.AutoLogin,
		DebugEchoLinks: cfg.Email.DebugEchoLinks,
		BlacklistPath:  cfg.Security.PasswordBlacklistPath,
		TenantMgr:      c.TenantSQLManager,
	}
	if strings.TrimSpace(cfg.Security.PasswordBlacklistPath) != "" {
		log.Printf(`{"level":"info","msg":"email_wiring_blacklist","path":"%s"}`, cfg.Security.PasswordBlacklistPath)
	} else {
		log.Printf(`{"level":"info","msg":"email_wiring_blacklist","path":"(empty)"}`)
	}
	log.Printf(`{"level":"info","msg":"email_wiring_ready","base_url":"%s","verify_ttl":"%s","reset_ttl":"%s","autologin":%t,"debug_links":%t}`,
		ef.BaseURL, ef.VerifyTTL, ef.ResetTTL, ef.AutoLoginReset, ef.DebugEchoLinks)

	verifyStart = http.HandlerFunc(ef.verifyEmailStart)
	verifyConfirm = http.HandlerFunc(ef.verifyEmailConfirm)
	forgot = http.HandlerFunc(ef.forgot)
	reset = http.HandlerFunc(ef.reset)

	return verifyStart, verifyConfirm, forgot, reset, cleanup, nil
}
