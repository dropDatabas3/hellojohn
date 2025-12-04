package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/oauth/google"
	"github.com/dropDatabas3/hellojohn/internal/rate"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/dropDatabas3/hellojohn/internal/util"
)

type googleHandler struct {
	cfg    *config.Config
	c      *app.Container
	pool   *pgxpool.Pool // Changed from *pgx.Conn
	oidc   *google.OIDC
	issuer *jwtx.Issuer // firma/verifica "state" (JWT EdDSA)

	// utils/adapters (ya existen en el proyecto)
	validator redirectValidatorAdapter
	issuerTok tokenIssuerAdapter
}

// issueSocialTokens centraliza la emisión de access/refresh y el flujo opcional de login_code.
// Siempre escribe la respuesta (redirect o JSON). Devuelve inmediatamente true para permitir return rápido.
func (h *googleHandler) issueSocialTokens(w http.ResponseWriter, r *http.Request, uid uuid.UUID, tid uuid.UUID, cid string, clientRedirect string, amr []string) bool {
	// ACR según AMR (elevar a LoA2 si incluye "mfa")
	acr := "urn:hellojohn:loa:1"
	for _, v := range amr {
		if v == "mfa" {
			acr = "urn:hellojohn:loa:2"
			break
		}
	}

	// Scopes placeholder: use client default scopes for scp
	scopes := []string{}
	if cl, _, e2 := h.c.Store.GetClientByClientID(r.Context(), cid); e2 == nil && cl != nil && strings.EqualFold(cl.TenantID, tid.String()) {
		scopes = append(scopes, cl.Scopes...)
	}
	std := map[string]any{"tid": tid.String(), "amr": amr, "acr": acr, "scp": strings.Join(scopes, " ")}
	custom := map[string]any{}

	// Hook + SYS_NS + roles/perms (best-effort)
	std, custom = applyAccessClaimsHook(r.Context(), h.c, tid.String(), cid, uid.String(), scopes, amr, std, custom)
	if u, err := h.c.Store.GetUserByID(r.Context(), uid.String()); err == nil && u != nil {
		type rbacReader interface {
			GetUserRoles(ctx context.Context, userID string) ([]string, error)
			GetUserPermissions(ctx context.Context, userID string) ([]string, error)
		}
		var roles, perms []string
		if rr, ok := h.c.Store.(rbacReader); ok {
			roles, _ = rr.GetUserRoles(r.Context(), uid.String())
			perms, _ = rr.GetUserPermissions(r.Context(), uid.String())
		}
		custom = helpers.PutSystemClaimsV2(custom, h.c.Issuer.Iss, u.Metadata, roles, perms)
	} else {
		custom = helpers.PutSystemClaimsV2(custom, h.c.Issuer.Iss, nil, nil, nil)
	}

	access, exp, err := h.c.Issuer.IssueAccess(uid.String(), cid, std, custom)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "issue_failed", "no se pudo emitir access", 1621)
		return true
	}

	cl, _, e2 := h.c.Store.GetClientByClientID(r.Context(), cid)
	if e2 != nil || cl == nil || !strings.EqualFold(cl.TenantID, tid.String()) {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1623)
		return true
	}

	rawRT, err := tokens.GenerateOpaqueToken(32)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1622)
		return true
	}
	// Usar CreateRefreshTokenTC para social login
	if tcStore, ok := h.c.Store.(interface {
		CreateRefreshTokenTC(context.Context, string, string, string, time.Time, *string) (string, error)
	}); ok {
		hash := tokens.SHA256Hex(rawRT)
		if _, err := tcStore.CreateRefreshTokenTC(r.Context(), tid.String(), cl.ClientID, hash, time.Now().Add(h.issuerTok.refreshTTL), nil); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh TC", 1624)
			return true
		}
	} else {
		// Fallback legacy
		hash := tokens.SHA256Base64URL(rawRT)
		if _, err := h.c.Store.CreateRefreshToken(r.Context(), uid.String(), cl.ID, hash, time.Now().Add(h.issuerTok.refreshTTL), nil); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1624)
			return true
		}
	}

	respAuth := AuthLoginResponse{AccessToken: access, TokenType: "Bearer", ExpiresIn: int64(time.Until(exp).Seconds()), RefreshToken: rawRT}

	if clientRedirect != "" { // login_code flow
		loginCode := randB64(32)
		cacheKey := "social:code:" + loginCode
		payload, _ := json.Marshal(struct {
			ClientID string            `json:"client_id"`
			TenantID string            `json:"tenant_id"`
			Response AuthLoginResponse `json:"response"`
		}{ClientID: cid, TenantID: tid.String(), Response: respAuth})
		ttl := h.cfg.Providers.LoginCodeTTL
		if ttl <= 0 {
			ttl = 60 * time.Second
		}
		h.c.Cache.Set(cacheKey, payload, ttl)
		if os.Getenv("SOCIAL_DEBUG_LOG") == "1" {
			log.Printf(`{"level":"debug","msg":"social_login_code_store","code":"%s","client_id":"%s","tenant_id":"%s","ttl_sec":%d}`, loginCode, cid, tid.String(), int(ttl.Seconds()))
		}
		target := clientRedirect
		if u, err := url.Parse(clientRedirect); err == nil {
			q := u.Query()
			q.Set("code", loginCode)
			u.RawQuery = q.Encode()
			target = u.String()
		} else {
			sep := "?"
			if strings.Contains(clientRedirect, "?") {
				sep = "&"
			}
			target = clientRedirect + sep + "code=" + loginCode
		}
		http.Redirect(w, r, target, http.StatusFound)
		return true
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(respAuth)
	return true
}

func BuildGoogleSocialHandlers(
	ctx context.Context,
	cfg *config.Config,
	c *app.Container,
	refreshTTL time.Duration,
) (start http.Handler, callback http.Handler, cleanup func(), err error) {

	if !cfg.Providers.Google.Enabled {
		return nil, nil, func() {}, nil
	}

	// sanity
	if cfg.Providers.Google.ClientID == "" || cfg.Providers.Google.ClientSecret == "" {
		return nil, nil, func() {}, errors.New("google: missing client_id/client_secret")
	}

	// RedirectURL: si no viene en YAML, derivarlo de jwt.issuer (mínima config)
	redirectURL := strings.TrimSpace(cfg.Providers.Google.RedirectURL)
	if redirectURL == "" && strings.TrimSpace(cfg.JWT.Issuer) != "" {
		base := strings.TrimRight(cfg.JWT.Issuer, "/")
		redirectURL = base + "/v1/auth/social/google/callback"
	}
	if redirectURL == "" {
		return nil, nil, func() {}, errors.New("google: missing redirect_url (set providers.google.redirect_url or jwt.issuer)")
	}

	// Use the existing pool from container if available, or connect new one?
	// The container has `Store`. We should cast it to get the pool.
	// But `Store` is an interface `core.Repository`.
	// Let's assume we can get it via type assertion to `*pg.Store` or similar.
	// For now, let's keep the local connection logic but use pgxpool.

	pgxPool, err := pgxpool.New(ctx, cfg.Storage.DSN)
	if err != nil {
		return nil, nil, func() {}, err
	}
	cleanup = func() { pgxPool.Close() }

	oidc := google.New(
		cfg.Providers.Google.ClientID,
		cfg.Providers.Google.ClientSecret,
		redirectURL,
		cfg.Providers.Google.Scopes,
	)

	h := &googleHandler{
		cfg:       cfg,
		c:         c,
		pool:      pgxPool,
		oidc:      oidc,
		issuer:    c.Issuer,
		validator: redirectValidatorAdapter{repo: c.Store},
		issuerTok: tokenIssuerAdapter{c: c, refreshTTL: refreshTTL},
	}

	start = http.HandlerFunc(h.start)
	callback = http.HandlerFunc(h.callback)
	log.Printf(`{"level":"info","msg":"google_wiring_ready","redirect":"%s","scopes":"%s"}`, redirectURL, strings.Join(cfg.Providers.Google.Scopes, " "))

	return start, callback, cleanup, nil
}

// -------- Helpers --------

func randB64(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func (h *googleHandler) isAllowed(tid uuid.UUID, cid string) bool {
	ats := h.cfg.Providers.Google.AllowedTenants
	acs := h.cfg.Providers.Google.AllowedClients
	okT := len(ats) == 0
	okC := len(acs) == 0
	if !okT {
		for _, t := range ats {
			if strings.EqualFold(t, tid.String()) {
				okT = true
				break
			}
		}
	}
	if !okC {
		for _, c := range acs {
			if strings.EqualFold(c, cid) {
				okC = true
				break
			}
		}
	}
	return okT && okC
}

// socialEnforce: rate limit simple por IP para endpoints sociales.
// keyPrefix distingue start / callback. Devuelve true si se permite continuar.
func socialEnforce(w http.ResponseWriter, r *http.Request, lim interface{}, limit int, window time.Duration, keyPrefix string) bool {
	type multi interface {
		AllowWithLimits(ctx context.Context, key string, limit int, window time.Duration) (rate.Result, error)
	}
	m, ok := lim.(multi)
	if !ok || limit <= 0 || window <= 0 {
		return true
	}
	ip := r.RemoteAddr
	if hf := r.Header.Get("X-Forwarded-For"); hf != "" {
		ip = strings.TrimSpace(strings.Split(hf, ",")[0])
	}
	key := keyPrefix + ip
	res, err := m.AllowWithLimits(r.Context(), key, limit, window)
	if err != nil {
		return true
	}
	now := time.Now().UTC()
	windowStart := now.Truncate(window)
	resetAt := windowStart.Add(window)
	if res.Allowed {
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(int(res.Remaining)))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		return true
	}
	retryAfter := time.Until(resetAt)
	if retryAfter < 0 {
		retryAfter = window
	}
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
	w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
	httpx.WriteError(w, http.StatusTooManyRequests, "rate_limited", "demasiadas solicitudes", 1401)
	return false
}

// state JWT firmado con EdDSA usando Issuer.SignRaw
func (h *googleHandler) signState(tid uuid.UUID, cid, clientRedirect, nonce string) (string, error) {
	now := time.Now().UTC()
	claims := jwtv5.MapClaims{
		"iss":   h.issuer.Iss,
		"aud":   "google-state",
		"iat":   now.Unix(),
		"nbf":   now.Unix(),
		"exp":   now.Add(5 * time.Minute).Unix(),
		"tid":   tid.String(),
		"cid":   cid,
		"redir": clientRedirect,
		"nonce": nonce,
	}
	signed, _, err := h.issuer.SignRaw(claims)
	return signed, err
}

func (h *googleHandler) parseState(s string) (map[string]any, error) {
	tk, err := jwtv5.Parse(s, h.issuer.Keyfunc(), jwtv5.WithValidMethods([]string{"EdDSA"}))
	if err != nil || !tk.Valid {
		return nil, errors.New("invalid_state_token")
	}
	claims, ok := tk.Claims.(jwtv5.MapClaims)
	if !ok {
		return nil, errors.New("bad_state_claims")
	}
	if iss, _ := claims["iss"].(string); iss != h.issuer.Iss {
		return nil, errors.New("state_iss_mismatch")
	}
	if aud, _ := claims["aud"].(string); aud != "google-state" {
		return nil, errors.New("state_aud_mismatch")
	}
	if expf, ok := claims["exp"].(float64); ok {
		if time.Unix(int64(expf), 0).Before(time.Now().Add(-30 * time.Second)) {
			return nil, errors.New("state_expired")
		}
	}
	return map[string]any(claims), nil
}

// -------- Start --------
// GET /v1/auth/social/google/start?tenant_id=...&client_id=...&redirect_uri=...
func (h *googleHandler) start(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1601)
		return
	}
	// Rate limit básico por IP para start (ej: 15 req / 1 min) si MultiLimiter disponible
	if h.c.MultiLimiter != nil {
		// usamos enforceWithKey directamente mediante wrapper ligero
		ipKey := r.RemoteAddr
		_ = ipKey
		// Reutilizamos función interna vía helpers.enforceWithKey: no exportada, así que implementamos local
		// Simplificado: key = social:start:<ip>
		if ok := socialEnforce(w, r, h.c.MultiLimiter, 15, time.Minute, "social:start:"); !ok {
			return
		}
	}
	q := r.URL.Query()
	tidStr := strings.TrimSpace(q.Get("tenant_id"))
	cid := strings.TrimSpace(q.Get("client_id"))
	clientRedirect := strings.TrimSpace(q.Get("redirect_uri"))

	tid, err := uuid.Parse(tidStr)
	if err != nil || cid == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "tenant_id/client_id requeridos", 1602)
		return
	}

	// redirect de la app debe estar permitido por el client? (opcional; distinto del redirect de Google)
	if clientRedirect != "" {
		// Validamos solo scheme+host+path (ignoramos query/fragment)
		baseToValidate := clientRedirect
		if u, err := url.Parse(clientRedirect); err == nil {
			u.RawQuery = ""
			u.Fragment = ""
			baseToValidate = u.String()
		}
		if !h.validator.ValidateRedirectURI(tid, cid, baseToValidate) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uri no permitido", 1603)
			return
		}
	}

	// restricciones opcionales
	if !h.isAllowed(tid, cid) {
		httpx.WriteError(w, http.StatusForbidden, "access_denied", "no permitido para este tenant/cliente", 1604)
		return
	}

	nonce := randB64(16)
	state, err := h.signState(tid, cid, clientRedirect, nonce)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "server_error", "no se pudo firmar state", 1605)
		return
	}

	authURL, err := h.oidc.AuthURL(r.Context(), state, nonce)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "server_error", "no se pudo construir auth url", 1606)
		return
	}

	http.Redirect(w, r, authURL, http.StatusFound)
}

// -------- Callback --------
// GET /v1/auth/social/google/callback?state=...&code=... (o error=...)
func (h *googleHandler) callback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1611)
		return
	}
	if h.c.MultiLimiter != nil {
		if ok := socialEnforce(w, r, h.c.MultiLimiter, 30, time.Minute, "social:cb:"); !ok {
			return
		}
	}
	q := r.URL.Query()
	if e := strings.TrimSpace(q.Get("error")); e != "" {
		ed := strings.TrimSpace(q.Get("error_description"))
		httpx.WriteError(w, http.StatusBadRequest, "idp_error", e+" "+ed, 1612)
		return
	}
	state := strings.TrimSpace(q.Get("state"))
	code := strings.TrimSpace(q.Get("code"))
	if state == "" || code == "" {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "state/code requeridos", 1613)
		return
	}

	// ───────── DEBUG SHORTCUT (solo dev/testing) ─────────
	// Activado sólo si SOCIAL_DEBUG_HEADERS=true en el entorno.
	if os.Getenv("SOCIAL_DEBUG_HEADERS") == "true" {
		log.Printf("DEBUG social_google: debug mode active code=%s hdr_email=%s", code, r.Header.Get("X-Debug-Google-Email"))
		// Fallback prioritario: si code tiene prefijo debug- simulamos directamente.
		if strings.HasPrefix(code, "debug-") {
			st, err := h.parseState(state)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "state inválido", 1614)
				return
			}
			get := func(k string) string {
				if s, _ := st[k].(string); s != "" {
					return s
				}
				return ""
			}
			idTid := get("tid")
			tid, err := uuid.Parse(idTid)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "tid inválido", 1615)
				return
			}
			cid := get("cid")
			clientRedirect := get("redir")
			if !h.isAllowed(tid, cid) {
				httpx.WriteError(w, http.StatusForbidden, "access_denied", "no permitido para este tenant/cliente", 1616)
				return
			}
			idc := &google.IDClaims{Email: "debug+" + code + "@example.test", EmailVerified: true, Sub: "sub-" + code}
			uid, err := h.ensureUserAndIdentity(r.Context(), tid, idc)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "provision_failed", "no se pudo crear/ligar usuario", 1620)
				return
			}
			_ = h.issueSocialTokens(w, r, uid, tid, cid, clientRedirect, []string{"google"})
			return
		}
		if dbgEmail := strings.TrimSpace(r.Header.Get("X-Debug-Google-Email")); dbgEmail != "" {
			dbgSub := strings.TrimSpace(r.Header.Get("X-Debug-Google-Sub"))
			dbgNonce := strings.TrimSpace(r.Header.Get("X-Debug-Google-Nonce"))
			st, err := h.parseState(state)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "state inválido", 1614)
				return
			}
			get := func(k string) string {
				if s, _ := st[k].(string); s != "" {
					return s
				}
				return ""
			}
			tid, err := uuid.Parse(get("tid"))
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "tid inválido", 1615)
				return
			}
			cid := get("cid")
			clientRedirect := get("redir")
			if hn := get("nonce"); hn != "" && dbgNonce != "" && hn != dbgNonce { /* ignore mismatch in debug */
			}
			if !h.isAllowed(tid, cid) {
				httpx.WriteError(w, http.StatusForbidden, "access_denied", "no permitido para este tenant/cliente", 1616)
				return
			}
			idc := &google.IDClaims{Email: dbgEmail, EmailVerified: true, Sub: dbgSub}
			uid, err := h.ensureUserAndIdentity(r.Context(), tid, idc)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "provision_failed", "no se pudo crear/ligar usuario", 1620)
				return
			}
			type mfaGetter interface {
				GetMFATOTP(context.Context, string) (*core.MFATOTP, error)
			}
			type trustedChecker interface {
				IsTrustedDevice(context.Context, string, string, time.Time) (bool, error)
			}
			if mg, ok := h.c.Store.(mfaGetter); ok {
				if m, _ := mg.GetMFATOTP(r.Context(), uid.String()); m != nil && m.ConfirmedAt != nil {
					trusted := false
					if devCookie, err := r.Cookie("mfa_trust"); err == nil && devCookie != nil {
						if tc, ok2 := h.c.Store.(trustedChecker); ok2 {
							dh := tokens.SHA256Base64URL(devCookie.Value)
							if ok3, _ := tc.IsTrustedDevice(r.Context(), uid.String(), dh, time.Now()); ok3 {
								trusted = true
							}
						}
					}
					if !trusted {
						ch := mfaChallenge{UserID: uid.String(), TenantID: tid.String(), ClientID: cid, AMRBase: []string{"google"}, Scope: []string{}}
						mid, _ := tokens.GenerateOpaqueToken(24)
						key := "mfa:token:" + mid
						buf, _ := json.Marshal(ch)
						h.c.Cache.Set(key, buf, 5*time.Minute)
						w.Header().Set("Cache-Control", "no-store")
						w.Header().Set("Pragma", "no-cache")
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						_ = json.NewEncoder(w).Encode(map[string]any{"mfa_required": true, "mfa_token": mid, "amr": []string{"google"}})
						return
					}
				}
			}
			_ = h.issueSocialTokens(w, r, uid, tid, cid, clientRedirect, []string{"google"})
			return
		}
	}

	// validar state
	st, err := h.parseState(state)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "state inválido", 1614)
		return
	}
	get := func(k string) string {
		if s, _ := st[k].(string); s != "" {
			return s
		}
		return ""
	}
	tid, err := uuid.Parse(get("tid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "tid inválido", 1615)
		return
	}
	cid := get("cid")
	clientRedirect := get("redir")
	nonce := get("nonce")

	// restricciones opcionales
	if !h.isAllowed(tid, cid) {
		httpx.WriteError(w, http.StatusForbidden, "access_denied", "no permitido para este tenant/cliente", 1616)
		return
	}

	// Intercambio code -> tokens + verify id_token
	tok, err := h.oidc.ExchangeCode(r.Context(), code)
	if err != nil {
		httpx.WriteError(w, http.StatusBadGateway, "exchange_failed", "no se pudo intercambiar el code", 1617)
		return
	}
	idc, err := h.oidc.VerifyIDToken(r.Context(), tok.IDToken, nonce)
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "id_token_invalid", "id_token no válido", 1618)
		return
	}
	if idc.Email == "" {
		httpx.WriteError(w, http.StatusUnauthorized, "email_missing", "no se obtuvo email", 1619)
		return
	}

	// Provisioning / linking (SQL directo)
	uid, err := h.ensureUserAndIdentity(r.Context(), tid, idc)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "provision_failed", "no se pudo crear/ligar usuario", 1620)
		return
	}

	// MFA hook (social): si usuario tiene MFA confirmada y no hay trusted device, bifurcamos antes de emitir tokens
	// Interfaces opcionales para MFA (evita romper compilación si aún no están implementadas en Store)
	type mfaGetter interface {
		GetMFATOTP(ctx context.Context, userID string) (*core.MFATOTP, error)
	}
	type trustedChecker interface {
		IsTrustedDevice(ctx context.Context, userID, deviceHash string, now time.Time) (bool, error)
	}
	if mg, ok := h.c.Store.(mfaGetter); ok {
		if m, _ := mg.GetMFATOTP(r.Context(), uid.String()); m != nil && m.ConfirmedAt != nil {
			trusted := false
			if devCookie, err := r.Cookie("mfa_trust"); err == nil && devCookie != nil {
				if tc, ok2 := h.c.Store.(trustedChecker); ok2 {
					dh := tokens.SHA256Base64URL(devCookie.Value)
					if ok3, _ := tc.IsTrustedDevice(r.Context(), uid.String(), dh, time.Now()); ok3 {
						trusted = true
					}
				}
			}
			if !trusted {
				ch := mfaChallenge{
					UserID:   uid.String(),
					TenantID: tid.String(),
					ClientID: cid,
					AMRBase:  []string{"google"},
					Scope:    []string{},
				}
				mid, _ := tokens.GenerateOpaqueToken(24)
				key := "mfa:token:" + mid
				buf, _ := json.Marshal(ch)
				h.c.Cache.Set(key, buf, 5*time.Minute)

				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Pragma", "no-cache")
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"mfa_required": true,
					"mfa_token":    mid,
					"amr":          []string{"google"},
				})
				return
			}
		}
	}

	_ = h.issueSocialTokens(w, r, uid, tid, cid, clientRedirect, []string{"google"})

	// Log útil
	rid := w.Header().Get("X-Request-ID")
	log.Printf(`{"level":"info","msg":"google_callback_ok","request_id":"%s","email":"%s","tenant":"%s","client_id":"%s","redir":"%s"}`, rid, util.MaskEmail(idc.Email), tid, cid, clientRedirect)
}

// ensureUserAndIdentity: upsert app_user + identity(provider='google')
func (h *googleHandler) ensureUserAndIdentity(ctx context.Context, tid uuid.UUID, idc *google.IDClaims) (uuid.UUID, error) {
	// 1) buscar app_user por (tenant_id,email)
	var userID uuid.UUID
	var emailVerified bool
	q1 := `
SELECT id, email_verified
FROM app_user
WHERE tenant_id=$1 AND email=$2
LIMIT 1`
	err := h.pool.QueryRow(ctx, q1, tid, idc.Email).Scan(&userID, &emailVerified)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, err
		}
		// crear
		qIns := `
INSERT INTO app_user (tenant_id, email, email_verified, status, metadata)
VALUES ($1,$2,$3,'active','{}'::jsonb)
RETURNING id`
		ev := idc.EmailVerified
		if err := h.pool.QueryRow(ctx, qIns, tid, idc.Email, ev).Scan(&userID); err != nil {
			return uuid.Nil, err
		}
	} else {
		// actualizar verificación si ahora viene true
		if idc.EmailVerified && !emailVerified {
			_, _ = h.pool.Exec(ctx, `UPDATE app_user SET email_verified=true WHERE id=$1`, userID)
		}
	}

	// 2) identity(provider='google', provider_user_id=sub)
	var idExists bool
	err = h.pool.QueryRow(ctx, `
SELECT EXISTS(SELECT 1 FROM identity WHERE provider='google' AND provider_user_id=$1 AND user_id=$2)
`, idc.Sub, userID).Scan(&idExists)
	if err != nil {
		return uuid.Nil, err
	}
	if !idExists {
		_, err = h.pool.Exec(ctx, `
INSERT INTO identity (user_id, provider, provider_user_id, email, email_verified)
VALUES ($1,'google',$2,$3,$4)
`, userID, idc.Sub, idc.Email, idc.EmailVerified)
		if err != nil {
			// caso raro: ya existe ligada a otro user ⇒ no tomamos control.
			// (Se puede resolver con "link account" autenticado en el futuro).
			return uuid.Nil, err
		}
	}
	return userID, nil
}
