/*
social_google.go — comentario/diagnóstico (bien completo, con “caminos”, capas y dónde partirlo)

Qué es este archivo
-------------------
Este handler implementa **Login Social con Google** para un tenant (modo “tenant DB”) con dos endpoints:

1) START
   GET /v1/auth/social/google/start?tenant_id=...&client_id=...&redirect_uri=...
   - Arma `state` (JWT EdDSA) + `nonce`
   - Valida redirect_uri (del cliente) contra el client (tu validator)
   - Redirige a Google con AuthURL(state, nonce)

2) CALLBACK
   GET /v1/auth/social/google/callback?state=...&code=...
   - Valida `state` (firma/iss/aud/exp)
   - Intercambia code con Google, verifica ID Token contra nonce
   - Provisiona/“linkea” user e identity en DB del tenant (h.pool)
   - Aplica “hook” MFA (si corresponde)
   - Emite tokens propios (access JWT + refresh opaco) y:
     - o devuelve JSON (AuthLoginResponse)
     - o si venía redirect_uri del cliente en el state, crea login_code, guarda en cache y redirige al cliente

Así que maneja 3 problemas mezclados: (a) OIDC con Google, (b) provisioning SQL, (c) emisión de tokens / login_code.

Mapa de caminos (flow)
----------------------

A) /start (GET)
   1. Rate limit (si MultiLimiter)
   2. Lee query: tenant_id, client_id, redirect_uri (opcional)
   3. Valida tenant_id es UUID y client_id no vacío
   4. Si redirect_uri viene:
        - normaliza (sin query/fragment)
        - valida con redirectValidatorAdapter contra el client
   5. (Opcional) allowlist tenant/client (cfg.Providers.Google.Allowed*)
   6. Genera nonce + firma state (JWT) con:
        iss = issuer.Iss
        aud = "google-state"
        exp = now + 5m
        tid, cid, redir, nonce
   7. oidc.AuthURL(ctx, state, nonce)
   8. Redirect 302 a Google

B) /callback (GET) camino normal
   1. Rate limit
   2. Si query error=..., devuelve idp_error
   3. Valida que existan state y code
   4. parseState(state):
        - jwt parse EdDSA
        - tk.Valid
        - iss coincide
        - aud coincide
        - exp no vencido (con -30s skew)
   5. Extrae tid,cid,redir,nonce
   6. allowlist (isAllowed)
   7. tok := oidc.ExchangeCode(code)
   8. idc := oidc.VerifyIDToken(tok.IDToken, nonce)
      - exige email
   9. uid := ensureUserAndIdentity(tid, idc) usando h.pool
  10. MFA check:
        - si store implementa GetMFATOTP + IsTrustedDevice
        - si tiene MFA confirmada y no trusted device:
             guarda challenge en cache "mfa:token:<mid>"
             responde JSON {mfa_required:true, mfa_token:..., amr:["google"]}
             return
  11. issueSocialTokens(..., amr:["google"]) -> emite access/refresh
  12. log info final

C) /callback debug (solo si SOCIAL_DEBUG_HEADERS=true)
   - Permite simular sin Google real:
     - code=debug-... o headers X-Debug-Google-Email/Sub/Nonce
   OJO: esto es una puerta *muy* peligrosa si se te filtra a prod.

Qué está bien (posta)
----------------------
- `state` firmado con EdDSA: buen anti-CSRF + integridad.
- `nonce` y verificación de ID token con nonce: bien OIDC.
- Rate limit por IP (start/callback): suma un montón contra abuso.
- “login_code flow” para apps SPA/popups: práctico y prolijo (y ya tenés exchange/result).
- Provisioning en tenant DB con pool: consistente con multi-tenant por DB.
- MFA hook “antes de emitir tokens”: correcto (no entregás tokens si falta 2FA).

Red flags / bugs / deuda técnica (lo importante)
------------------------------------------------

1) Emisión de tokens usa h.c.Store para user/roles + client scopes (BUG multi-tenant)
   --------------------------------------------------------------------------------
   En issueSocialTokens hacés:
     h.c.Store.GetClientByClientID(...)
     h.c.Store.GetUserByID(...)
     y roles/perms desde h.c.Store.(rbacReader)

   Pero el provisioning y refresh_token insert lo hacés con h.pool (tenant DB).
   Si `h.c.Store` es global o de otro tenant, te trae:
   - scopes de otro lado
   - metadata del usuario no encontrada o del tenant equivocado
   - roles/perms incorrectos

   FIX recomendado:
   - Para social, TODO lo “tenant data” debe salir de tenant store/pool.
   - O bien, pasale a googleHandler un `repo core.Repository` ya resuelto por tenant,
     y usalo para GetUser/GetRoles/GetClient.
   - Si no tenés repo por tenant, al menos hacé queries SQL por h.pool para:
       - scopes del client (si están en FS, tomalos del FS, no de DB)
       - metadata/roles/perms del user (tenant DB)

2) Refresh token insert: `NOW() + $4::interval` con string “72h0m0s” (posible crash)
   ------------------------------------------------------------------------------
   Estás pasando `h.issuerTok.refreshTTL.String()` como intervalo.
   En Postgres, interval acepta cosas tipo '72 hours' o '5 minutes'.
   `"72h0m0s"` NO siempre parsea (de hecho, suele fallar).

   FIX:
   - Pasar segundos y usar `NOW() + ($4 * interval '1 second')`
     y mandar int64(refreshTTL.Seconds()).
   - O formatear `'72 hours'` vos.

3) El token access lo emitís con IssueAccess(uid, cid, ...) sin issuer por tenant
   -----------------------------------------------------------------------------
   En tu OAuth token endpoint ya resolviste issuer efectivo por tenant (issuer mode path/custom).
   Acá no: usás `h.c.Issuer.Iss` fijo, y `IssueAccess` (no IssueAccessForTenant).
   Eso te rompe consistencia con:
   - JWKS per-tenant
   - iss por tenant (path mode)
   - introspection que valida issuer/slug

   FIX:
   - igual que en oauth_token.go: resolver `effIss` y firmar “for tenant”
     con `IssueAccessForTenant(tenantSlug, effIss, ...)` o equivalente.
   - y el SYS namespace también debería usar effIss.

4) `helpers.ResolveClientFSByTenantID(tid.String(), cid)` dice “ONLY FS”
   --------------------------------------------------------------------
   Perfecto, pero entonces dejá de usar c.Store para client scopes.
   Tenés fsCl ahí: usalo para scopes (fsCl.Scopes).
   Hoy lo resolvés y después no lo usás.

5) ensureUserAndIdentity ignora “tid” (ok) pero entonces email debe ser único por tenant DB
   --------------------------------------------------------------------------------------
   Como cada tenant es su DB, está bien que `app_user` no tenga tenant_id.
   Pero ojo: si a futuro cambiás a shared DB, esto explota.

6) DEBUG SHORTCUT es una bomba si se habilita en prod
   --------------------------------------------------
   `SOCIAL_DEBUG_HEADERS=true` permite loguear con headers sin pasar por Google.
   Eso tiene que estar:
   - hardcodeado a “solo dev”
   - o protegido por allowlist de IP + secret adicional
   - o directamente eliminado y reemplazado por tests de integración.

7) “state_expired” usa skew de -30s raro
   -------------------------------------
   `Before(time.Now().Add(-30s))` => o sea, tolerás 30s *después* de exp.
   Está bien como clock skew, pero dejalo explícito como `skew := 30s` para claridad.

8) Falta MaxBytesReader en callbacks (no grave porque es GET, pero start/callback no consumen body)
   ------------------------------------------------------------------------------------------------
   No aplica mucho acá. Donde sí: exchange POST.

Cómo lo separaría (capas y archivos)
------------------------------------

Hoy `social_google.go` tiene 4 responsabilidades. Te lo partiría así:

1) handlers/social/google_handler.go (HTTP puro)
   - start(w,r)
   - callback(w,r)
   - parse query, headers, rate limit, http errors, redirects
   - NO debería hablar SQL directo salvo a través de un service

2) internal/auth/social/google/state.go
   - SignState(issuer, tid, cid, redir, nonce, ttl) (y parse/validate)
   - Tipos claims tipados (en vez de MapClaims) para evitar casts

3) internal/auth/social/google/oidc.go
   - wrapper: ExchangeCode + VerifyIDToken
   - aislás dependencias de google.OIDC

4) internal/auth/social/provisioning/service.go
   - EnsureUserAndIdentity(ctx, db, provider, idClaims) (usa h.pool)
   - maneja upsert con transacción (ideal):
       BEGIN
         select user
         insert/update
         ensure identity
       COMMIT
     (ahora está bien, pero sin tx puede haber carreras raras)

5) internal/auth/token/service.go
   - IssueTokensForSocial(ctx, tenant, client, user, amr, scopes, clientRedirect)
   - decide:
       - MFA required?
       - emitir access/refresh
       - login_code o JSON

6) internal/auth/social/logincode/store.go
   - SaveLoginCode(code -> payload, ttl)
   - ConsumeLoginCode(code)

Objetivo: el handler orquesta y el service decide lógica.

Quick wins para que quede sólido ya
-----------------------------------

- En issueSocialTokens:
  1) usar scopes desde FS:
       scopes := fsCl.Scopes (o default)
  2) user/roles/perms: leer desde tenant DB, no c.Store global.
     Si no querés implementar repo por tenant ahora:
       - por lo menos no intentes roles/perms (o dejalo “best-effort” pero con interfaz sobre tenant store)
  3) emitir access con issuer efectivo por tenant (como oauth_token.go)

- Refresh insert:
  cambiar a segundos:
    const q = `... expires_at = NOW() + ($4 * interval '1 second') ...`
    h.pool.QueryRow(ctx, q, cid, uid, hash, int64(refreshTTL.Seconds()))

- DEBUG:
  meter un guardia duro:
    if cfg.Env != "dev" { ignorar SOCIAL_DEBUG_HEADERS aunque esté seteado }

Cierre
------
El flow está copado y bastante completo (state/nonce/rate-limit/mfa/login_code), pero hoy tenés
inconsistencias multi-tenant fuertes: *emitís tokens mirando c.Store*, pero persistís usuario/refresh en tenant DB.
Si arreglás eso + el interval del refreshTTL + issuer efectivo por tenant, te queda una base muy sólida.
*/

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

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
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

	// Use ONLY FS control plane for client lookup (no global DB)
	fsCl, fsErr := helpers.ResolveClientFSByTenantID(r.Context(), tid.String(), cid)
	if fsErr != nil || fsCl.ClientID == "" {
		log.Printf(`{"level":"error","msg":"issueSocialTokens_client_not_found","client_id":"%s","tenant_id":"%s","err":"%v"}`, cid, tid, fsErr)
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_client", "client inválido", 1623)
		return true
	}
	// fsCl contains client info from FS control plane

	// Create refresh token using tenant-specific pool (h.pool, NOT h.c.Store)
	rawToken, err := tokens.GenerateOpaqueToken(32)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "token_gen_failed", "no se pudo generar refresh", 1622)
		return true
	}
	tokenHash := tokens.SHA256Base64URL(rawToken)

	// INSERT directly into tenant DB using h.pool
	const qInsert = `
		INSERT INTO refresh_token (client_id, user_id, token_hash, issued_at, expires_at, metadata)
		VALUES ($1, $2, $3, NOW(), NOW() + $4::interval, '{}')
		RETURNING id`
	var tokenID string
	err = h.pool.QueryRow(r.Context(), qInsert, cid, uid.String(), tokenHash, h.issuerTok.refreshTTL.String()).Scan(&tokenID)
	if err != nil {
		log.Printf(`{"level":"error","msg":"issueSocialTokens_refresh_create_err","pool":"tenant","err":"%v"}`, err)
		httpx.WriteError(w, http.StatusInternalServerError, "persist_failed", "no se pudo persistir refresh", 1624)
		return true
	}
	rawRT := rawToken

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
// Uses ONLY tenant-specific DB (no tenant_id column in queries)
func (h *googleHandler) ensureUserAndIdentity(ctx context.Context, tid uuid.UUID, idc *google.IDClaims) (uuid.UUID, error) {
	var userID uuid.UUID
	var emailVerified bool

	// 1) Try to find existing user by email (tenant DB - no tenant_id column)
	qSelect := `SELECT id, email_verified FROM app_user WHERE email=$1 LIMIT 1`
	err := h.pool.QueryRow(ctx, qSelect, idc.Email).Scan(&userID, &emailVerified)
	log.Printf(`{"level":"debug","msg":"ensureUser_select","email":"%s","err":"%v"}`, idc.Email, err)

	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			log.Printf(`{"level":"error","msg":"ensureUser_select_err","err":"%v"}`, err)
			return uuid.Nil, err
		}
		// User doesn't exist - create them (tenant DB - no tenant_id/status columns)
		qInsert := `INSERT INTO app_user (email, email_verified, name, given_name, family_name, picture, locale, metadata)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'{}'::jsonb) RETURNING id`

		ev := idc.EmailVerified
		err = h.pool.QueryRow(ctx, qInsert, idc.Email, ev, idc.Name, idc.GivenName, idc.FamilyName, idc.Picture, idc.Locale).Scan(&userID)
		log.Printf(`{"level":"debug","msg":"ensureUser_insert","email":"%s","err":"%v"}`, idc.Email, err)
		if err != nil {
			log.Printf(`{"level":"error","msg":"ensureUser_insert_err","err":"%v"}`, err)
			return uuid.Nil, err
		}
	} else {
		// User exists - update verification if needed
		if idc.EmailVerified && !emailVerified {
			_, _ = h.pool.Exec(ctx, `UPDATE app_user SET email_verified=true WHERE id=$1`, userID)
		}
	}

	log.Printf(`{"level":"debug","msg":"ensureUser_user_ready","user_id":"%s"}`, userID)

	// 2) Ensure identity(provider='google', provider_user_id=sub) exists
	var idExists bool
	err = h.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM identity WHERE provider='google' AND provider_user_id=$1 AND user_id=$2)
	`, idc.Sub, userID).Scan(&idExists)
	log.Printf(`{"level":"debug","msg":"ensureUser_identity_check","sub":"%s","exists":%v,"err":"%v"}`, idc.Sub, idExists, err)

	if err != nil {
		log.Printf(`{"level":"error","msg":"ensureUser_identity_check_err","err":"%v"}`, err)
		return uuid.Nil, err
	}
	if !idExists {
		_, err = h.pool.Exec(ctx, `
			INSERT INTO identity (user_id, provider, provider_user_id, email, email_verified)
			VALUES ($1,'google',$2,$3,$4)
		`, userID, idc.Sub, idc.Email, idc.EmailVerified)
		log.Printf(`{"level":"debug","msg":"ensureUser_identity_insert","err":"%v"}`, err)
		if err != nil {
			log.Printf(`{"level":"error","msg":"ensureUser_identity_insert_err","err":"%v"}`, err)
			return uuid.Nil, err
		}
	}
	return userID, nil
}
