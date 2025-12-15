/*
oauth_authorize.go — OAuth 2.1 / OIDC Authorization Endpoint (GET /authorize) con PKCE + sesión por cookie + fallback Bearer + step-up MFA (TOTP)

Qué es este archivo (la posta)
------------------------------
Este archivo define `NewOAuthAuthorizeHandler(c, cookieName, allowBearer)` que devuelve un `http.HandlerFunc`
para implementar el endpoint de autorización (Authorization Endpoint) del flow Authorization Code:

- Valida request OAuth/OIDC básica: response_type=code, client_id, redirect_uri, scope con openid, state/nonce opcionales.
- Obliga PKCE S256 (code_challenge + code_challenge_method=S256).
- Resuelve client + tenant (FS/DB) vía `helpers.LookupClient`.
- Autentica usuario por:
  1) Cookie de sesión (SID) contra cache (sid:<sha256(cookie)>)
  2) (Opcional) Bearer token como fallback (si allowBearer=true)
- Hace step-up de MFA: si el usuario tiene TOTP confirmada y sólo viene "pwd" en AMR, corta y devuelve JSON `mfa_required`
  (o auto-eleva AMR si hay trusted device cookie).
- Si todo OK: genera Authorization Code opaco, lo guarda en cache (`code:<code>`), y redirige a redirect_uri con `code` (+ state).

Esto NO emite tokens directamente: sólo genera el “code” para que después /token lo canjee.

Fuente de verdad / multi-tenant (FS vs DB)
------------------------------------------
- El “client lookup” y tenantSlug se resuelven por `helpers.LookupClient(ctx, r, clientID)`.
  Esto suele esconder el gating: DB primero, fallback FS provider (según cómo esté implementado helpers).
- Para features extra (MFA / trusted devices), el handler intenta elegir `activeStore` con precedencia:
  - tenantDB (si `c.TenantSQLManager != nil` y `cpctx.ResolveTenant(r)` engancha un slug válido)
  - fallback a `c.Store` global
- OJO: `tenantSlug` que devuelve LookupClient se usa para “legacy mapping” y match de tenant con la sesión.
  `activeStore` en cambio se resuelve con `cpctx.ResolveTenant(r)` (otro mecanismo). Esto puede desalinearse si:
  - resolveTenant() no coincide con el tenant del clientID
  - el request no trae suficiente info para resolver tenant (depende del middleware / host / path issuer mode)

Rutas / contrato (lo que expone realmente)
------------------------------------------
- Handler pensado para: GET (probablemente montado como /v1/oauth/authorize o similar; acá no se ve el router).
- Entradas por query string:
  - response_type=code (obligatorio)
  - client_id (obligatorio)
  - redirect_uri (obligatorio)
  - scope (obligatorio; debe incluir "openid")
  - state (opcional)
  - nonce (opcional OIDC)
  - code_challenge (obligatorio PKCE)
  - code_challenge_method=S256 (obligatorio PKCE)
  - prompt (opcional; si incluye "none" y no hay sesión, devuelve error al redirect)
- Respuestas:
  - 302 Found redirect a redirect_uri?code=...&state=...
  - 302 Found redirect con error=... (si prompt=none y falta login)
  - 200 OK JSON `{status:"mfa_required", mfa_token:"..."}` (cuando requiere step-up)
  - 4xx JSON (httpx.WriteError) para errores “antes” de validar redirect (invalid_request, invalid_client, invalid_scope, invalid_redirect_uri)
  - 405 si no es GET

Flujo paso a paso (secuencia real)
----------------------------------
1) Método: solo GET, setea `Vary: Cookie` y si allowBearer `Vary: Authorization`.
2) Elegir `activeStore`:
   - intenta tenant store via `c.TenantSQLManager.GetPG(ctx, cpctx.ResolveTenant(r))`
   - si falla o no hay manager, usa `c.Store` global
   (ScopesConsents repo aparece comentado/unusado)
3) Leer y validar query params:
   - response_type == "code"
   - client_id, redirect_uri, scope no vacíos
   - scope debe incluir "openid"
   - PKCE: code_challenge_method=S256 y code_challenge no vacío
4) Resolver client + tenantSlug:
   - `helpers.LookupClient(ctx, r, clientID)` -> (client, tenantSlug, err)
   - Validar redirect_uri: `helpers.ValidateRedirectURI(client, redirectURI)`
   - Validar scopes: `controlplane.DefaultIsScopeAllowed(client, s)` por cada scope
5) Mapear a estructura legacy `core.Client` para match de tenant y redirectURIs:
   - cl.ID = client.ClientID
   - cl.TenantID = tenantSlug (OJO: acá tenantID “legacy” es slug, no UUID)
6) Autenticación:
   A) Cookie SID:
      - busca cookie `cookieName`
      - key cache: `sid:` + SHA256Base64URL(cookieValue)
      - parsea `SessionPayload` (no está en este archivo)
      - valida: now < sp.Expires y sp.TenantID == cl.TenantID
      - setea: sub=sp.UserID, tid=sp.TenantID, amr=["pwd"]
   B) Fallback Bearer (si allowBearer && sub vacío):
      - parse JWT EdDSA con issuer `c.Issuer.Iss` y `c.Issuer.Keyfunc()`
      - saca sub, tid, y amr desde claims
7) Si no hay sesión válida o tenant mismatch:
   - si prompt contiene "none": redirect error login_required a redirect_uri (no UI)
   - si no: redirige a UI login:
     - UI_BASE_URL (ENV, default localhost:3000)
     - return_to = URL absoluta del authorize request
8) Step-up MFA (solo si amr == ["pwd"] y hay activeStore):
   - si store implementa `GetMFATOTP(userID)` y está confirmado:
     - intenta trusted device cookie `mfa_trust`
     - si store implementa `IsTrustedDevice(userID, deviceHash, now)` y da true -> trustedByCookie=true
     - si NO trusted: arma challenge `mfaChallenge{UserID,TenantID,ClientID,AMRBase:["pwd"],Scope:[]}`
       - genera mid opaco (24)
       - guarda en cache: `mfa_req:<mid>` ttl 5 min
       - responde 200 JSON {status:"mfa_required", mfa_token: mid}
     - si trusted: eleva AMR agregando "mfa","totp"
9) Generar authorization code:
   - code opaco (32)
   - payload `authCode{UserID,TenantID,ClientID,RedirectURI,Scope,Nonce,CodeChallenge,ChallengeMethod,AMR,ExpiresAt}`
   - guarda en cache: `code:<code>` ttl 10 min
10) Redirect success:
    - redirect_uri?code=<code>[&state=<state>]

Seguridad / invariantes (las importantes)
-----------------------------------------
- PKCE S256 obligatorio (bien, 2025 vibes): evita code interception.
- Validación estricta de redirect_uri contra el client (bien, corta open-redirect).
- Validación de scopes solicitados contra scopes del client (DefaultIsScopeAllowed).
- Session binding por tenant:
  - cookie session sólo vale si tenant match con el client del authorize.
  - Bearer también se chequea contra tenant del client antes de emitir code.
- Cache-Control/Pragma no-store en redirectError, pero en success redirect NO se setea explícitamente (deuda chica).
- AMR se propaga dentro del authCode para que /token pueda emitir tokens con AMR correcto.
- `prompt=none` respeta OIDC: si no hay login, devuelve login_required (sin UI).

Patrones detectados (GoF / arquitectura)
----------------------------------------
- Adapter/Ports (implícito):
  - Usa `activeStore` con type assertions para capacidades (GetMFATOTP, IsTrustedDevice). Eso es “feature detection”
    estilo plugin: el store es un “port” y se extiende por interfaces chicas.
- Strategy:
  - Autenticación por cookie vs bearer.
  - MFA step-up según estado + trusted device.
- Facade:
  - helpers.LookupClient + helpers.ValidateRedirectURI ocultan la complejidad FS/DB y normalización.

Cosas no usadas / legacy / riesgos (marcar sin piedad)
------------------------------------------------------
- `authCode` incluye `TenantID` como string y acá se setea `tid` que viene de sesión/bearer.
  En cookie flow tid = sp.TenantID, que parece ser slug (por el match con cl.TenantID). En bearer flow tid suele ser UUID (depende del issuer).
  ⇒ Riesgo: mixed “tenant slug vs tenant UUID” en el mismo campo. Después /token puede romper o permitir cosas raras si no valida.
- `trustedByCookie` depende de un cookie `mfa_trust` que NO valida binding con tenant/client acá (sólo hash y store).
  Está ok si store lo ata bien, pero conviene remarcarlo.
- `activeStore` se resuelve por `cpctx.ResolveTenant(r)` pero el tenant del client viene de `helpers.LookupClient`.
  Si esos dos divergen, el step-up MFA podría consultar el store equivocado o fallar silencioso.
- Logging “DEBUG:” a stdout con datos de sesión/tenant (ojo con PII y logs en prod).
- `redirectError` agrega query params con `addQS` (a mano). Funciona, pero no controla si ya existe error=... o si redirect_uri tiene fragment.
  OIDC recomienda no meter params en fragment acá (igual está ok para code flow), pero mejor builder robusto.

Ideas de refactor a V2 (sin decidir nada, solo guía accionable)
--------------------------------------------------------------
Objetivo: separar “OAuth/OIDC controller” de autenticación de sesión, MFA y cache persistence.

1) DTO / parsing
- Crear un `AuthorizeRequestDTO` con parse/validate:
  - required params, PKCE, openid en scope
  - normalizar scope a []string y también mantener raw
- Centralizar errores:
  - si falla antes de validar redirect -> JSON httpx.WriteError
  - si ya hay redirect válido -> redirectError (siempre)

2) Controller (HTTP)
- `AuthorizeController.Handle(w,r)` que sólo:
  - parse request
  - resolve client
  - auth session
  - decide “need_login / need_mfa / issue_code”
  - mapear a response (redirect o JSON)

3) Services
- `ClientResolverService`:
  - ResolveClient(ctx, req) -> (client, tenantCtx) y validar redirect/scopes
- `SessionAuthService`:
  - FromCookie(cookieName) -> (user, tenant, amr)
  - FromBearer(issuer) -> (user, tenant, amr)
  - En ambos: output normalizado con tenantID canonical (ideal: UUID) + tenantSlug si hace falta.
- `MFAService`:
  - RequiresStepUp(ctx, userID) -> bool, details
  - TrustedDevice(ctx, userID, deviceCookie) -> bool
  - CreateChallenge(ctx, payload) -> mfa_token
- `AuthCodeService`:
  - Create(ctx, payload) -> code
  - Persist(ctx, code, payload, ttl)
  (y que /token consuma desde el mismo service)

4) Repo / Cache
- Abstraer c.Cache con una interfaz:
  - Get/Set con prefijos tipados (sid, code, mfa_req) para no mezclar strings sueltas.
- Meter TTLs como constantes/config (10m code, 5m mfa) en un solo lugar.

5) Patrones GoF que sí sumarían
- Chain of Responsibility (middleware-ish) dentro del handler:
  - ValidateRequest -> ResolveClient -> Authenticate -> StepUpMFA -> IssueCode
  Te deja testeable por etapas sin reventar el handler.
- Strategy para AuthMethod (CookieAuthStrategy vs BearerAuthStrategy).
- Builder para redirect URL (evitar addQS manual y manejar query/fragment bien).

6) Concurrencia
- Acá NO hace falta: todo es lectura + writes chicos en cache.
  La única “paralelización” útil sería en futuras versiones si querés buscar MFA + session en paralelo,
  pero es overkill y te complica (y además cache+DB no siempre conviene).

Resumen
-------
- Este handler es el “/authorize” de tu OIDC: valida PKCE+openid, autentica por cookie/bearer, hace step-up MFA y emite auth code en cache.
- Lo más jugoso para V2 es normalizar tenant identifiers (slug vs UUID), alinear ResolveTenant (cpctx) con LookupClient (helpers),
  y separar responsabilidades (auth/mfa/code/cache) para que no sea un god handler a medida que crece.
*/

package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

type authCode struct {
	UserID          string    `json:"user_id"`
	TenantID        string    `json:"tenant_id"`
	ClientID        string    `json:"client_id"`
	RedirectURI     string    `json:"redirect_uri"`
	Scope           string    `json:"scope"`
	Nonce           string    `json:"nonce"`
	CodeChallenge   string    `json:"code_challenge"`
	ChallengeMethod string    `json:"code_challenge_method"`
	AMR             []string  `json:"amr"`
	ExpiresAt       time.Time `json:"expires_at"`
}

func addQS(u, k, v string) string {
	sep := "?"
	if strings.Contains(u, "?") {
		sep = "&"
	}
	return u + sep + url.QueryEscape(k) + "=" + url.QueryEscape(v)
}

func redirectError(w http.ResponseWriter, r *http.Request, redirectURI, state, code, desc string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	loc := addQS(redirectURI, "error", code)
	if desc != "" {
		loc = addQS(loc, "error_description", desc)
	}
	if state != "" {
		loc = addQS(loc, "state", state)
	}
	http.Redirect(w, r, loc, http.StatusFound)
}

// soporta cookie de sesión y (opcional) Bearer como fallback
func NewOAuthAuthorizeHandler(c *app.Container, cookieName string, allowBearer bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1000)
			return
		}
		w.Header().Add("Vary", "Cookie")
		if allowBearer {
			w.Header().Add("Vary", "Authorization")
		}

		// Resolver store con precedencia: tenantDB > globalDB (para consent y magic link)
		// var activeScopesConsents core.ScopesConsentsRepository // Unused for now
		var activeStore core.Repository

		if c.TenantSQLManager != nil {
			// Intentar obtener store del tenant actual
			tenantSlug := cpctx.ResolveTenant(r)
			if tenantStore, err := c.TenantSQLManager.GetPG(r.Context(), tenantSlug); err == nil && tenantStore != nil {
				activeStore = tenantStore
				// Para tenant stores, por ahora usar fallback a global consent (TODO: implementar per-tenant consent)
			}
		}
		// Fallback a global consents
		/* if c.ScopesConsents != nil {
			activeScopesConsents = c.ScopesConsents
		} */
		if activeStore == nil && c.Store != nil {
			activeStore = c.Store
		}

		q := r.URL.Query()
		responseType := strings.TrimSpace(q.Get("response_type"))
		clientID := strings.TrimSpace(q.Get("client_id"))
		redirectURI := strings.TrimSpace(q.Get("redirect_uri"))
		scope := strings.TrimSpace(q.Get("scope"))
		state := strings.TrimSpace(q.Get("state"))
		nonce := strings.TrimSpace(q.Get("nonce"))
		codeChallenge := strings.TrimSpace(q.Get("code_challenge"))
		codeMethod := strings.TrimSpace(q.Get("code_challenge_method"))

		if responseType != "code" || clientID == "" || redirectURI == "" || scope == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "faltan parámetros obligatorios", 2101)
			return
		}
		if !strings.Contains(scope, "openid") {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_scope", "scope debe incluir openid", 2102)
			return
		}
		if !strings.EqualFold(codeMethod, "S256") || codeChallenge == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "PKCE S256 requerido", 2103)
			return
		}

		ctx := r.Context()

		client, tenantSlug, err := helpers.LookupClient(ctx, r, clientID)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client", "client not found", 2104)
			return
		}
		if err := helpers.ValidateRedirectURI(client, redirectURI); err != nil {
			// If redirect_uri is present but doesn't match, RFC says invalid_redirect_uri
			httpx.WriteError(w, http.StatusBadRequest, "invalid_redirect_uri", "redirect_uri not allowed for this client", 2105)
			return
		}

		// Validar scopes solicitados
		if scope := strings.TrimSpace(scope); scope != "" {
			for _, s := range strings.Fields(scope) {
				if !controlplane.DefaultIsScopeAllowed(client, s) {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_scope", "scope not allowed", 2106)
					return
				}
			}
		}

		// Continuar con lógica existente
		_ = tenantSlug

		// Mapear client FS a estructura legacy para compatibilidad
		cl := &core.Client{
			ID:           client.ClientID, // Usar clientID como ID temporal
			TenantID:     tenantSlug,      // Usar tenantSlug como TenantID
			RedirectURIs: client.RedirectURIs,
			Scopes:       client.Scopes,
		}
		// reqScopes removed as it was unused

		var (
			sub             string
			tid             string
			amr             []string
			trustedByCookie bool
		)

		// 1) Sesión COOKIE
		if ck, err := r.Cookie(cookieName); err == nil && ck != nil && strings.TrimSpace(ck.Value) != "" {
			key := "sid:" + tokens.SHA256Base64URL(ck.Value)
			log.Printf("DEBUG: authorize cookie found: %s", ck.Name)
			if b, ok := c.Cache.Get(key); ok {
				var sp SessionPayload
				_ = json.Unmarshal(b, &sp)
				log.Printf("DEBUG: authorize session payload: %+v | expected tenant: %s", sp, cl.TenantID)

				// Compare carefully
				if time.Now().Before(sp.Expires) && strings.EqualFold(sp.TenantID, cl.TenantID) {
					sub = sp.UserID
					tid = sp.TenantID
					amr = []string{"pwd"}
					log.Printf("DEBUG: authorize session valid! sub=%s tid=%s", sub, tid)
				} else {
					log.Printf("DEBUG: authorize session invalid (expired=%v, tenant_match=%v, sp_tenant='%s', cl_tenant='%s')",
						!time.Now().Before(sp.Expires), strings.EqualFold(sp.TenantID, cl.TenantID), sp.TenantID, cl.TenantID)
				}
			} else {
				log.Printf("DEBUG: authorize session cache miss for key: %s (truncated)", key[:10])
			}
		} else {
			log.Printf("DEBUG: authorize NO cookie found (err=%v)", err)
		}

		// 2) Fallback Bearer
		if sub == "" && allowBearer {
			ah := strings.TrimSpace(r.Header.Get("Authorization"))
			if strings.HasPrefix(strings.ToLower(ah), "bearer ") {
				raw := strings.TrimSpace(ah[len("Bearer "):])
				tk, err := jwtv5.Parse(raw, c.Issuer.Keyfunc(),
					jwtv5.WithValidMethods([]string{"EdDSA"}), jwtv5.WithIssuer(c.Issuer.Iss))
				if err == nil && tk.Valid {
					if claims, ok := tk.Claims.(jwtv5.MapClaims); ok {
						sub, _ = claims["sub"].(string)
						tid, _ = claims["tid"].(string)
						if v, ok := claims["amr"].([]any); ok {
							for _, i := range v {
								if s, ok := i.(string); ok {
									amr = append(amr, s)
								}
							}
						}
					}
				}
			}
		}

		if sub == "" || tid == "" || !strings.EqualFold(tid, cl.TenantID) {
			// Si prompt=none, retornamos error (interacción no permitida)
			if strings.Contains(r.URL.Query().Get("prompt"), "none") {
				redirectError(w, r, redirectURI, state, "login_required", "requiere login")
				return
			}

			// Redirigir al Login UI
			uiBase := os.Getenv("UI_BASE_URL")
			if uiBase == "" {
				uiBase = "http://localhost:3000"
			}

			// Construir return_to con la URL actual de authorize
			returnTo := r.URL.String() // path + query
			// Asegurar que returnTo tenga el host si falta (r.URL.String() puede ser relativo)
			if !r.URL.IsAbs() {
				scheme := "http"
				if r.TLS != nil {
					scheme = "https"
				}
				host := r.Host
				if host == "" {
					host = "localhost:8080"
				}
				returnTo = fmt.Sprintf("%s://%s%s", scheme, host, r.URL.RequestURI())
			}

			loginURL := fmt.Sprintf("%s/login?return_to=%s", uiBase, url.QueryEscape(returnTo))
			http.Redirect(w, r, loginURL, http.StatusFound)
			return
		}

		// Step-up MFA: si el usuario tiene MFA TOTP confirmada y la AMR actual solo contiene pwd, devolver JSON mfa_required
		if len(amr) == 1 && amr[0] == "pwd" && activeStore != nil {
			// intentamos detectar secreto MFA
			if mg, ok := activeStore.(interface {
				GetMFATOTP(ctx context.Context, userID string) (*core.MFATOTP, error)
			}); ok {
				if m, _ := mg.GetMFATOTP(r.Context(), sub); m != nil && m.ConfirmedAt != nil {
					// Revisar trusted device cookie (si existe) para posiblemente elevar amr automáticamente
					if ck, err := r.Cookie("mfa_trust"); err == nil && ck != nil {
						if tc, ok2 := activeStore.(interface {
							IsTrustedDevice(ctx context.Context, userID, deviceHash string, now time.Time) (bool, error)
						}); ok2 {
							if ok3, _ := tc.IsTrustedDevice(r.Context(), sub, tokens.SHA256Base64URL(ck.Value), time.Now()); ok3 {
								trustedByCookie = true
							}
						}
					}
					if !trustedByCookie { // requerir desafío MFA antes de emitir code
						ch := mfaChallenge{
							UserID:   sub,
							TenantID: tid,
							ClientID: clientID,
							AMRBase:  []string{"pwd"},
							Scope:    strings.Fields(scope),
						}
						mid, _ := tokens.GenerateOpaqueToken(24)

						// Store challenge in cache
						b, _ := json.Marshal(ch)
						key := "mfa_req:" + mid
						c.Cache.Set(key, b, 5*time.Minute)

						// Return json instructing UI to show MFA entry
						resp := map[string]string{
							"status":    "mfa_required",
							"mfa_token": mid,
						}
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(http.StatusOK) // 200 OK so UI handles it
						json.NewEncoder(w).Encode(resp)
						return
					}
					// if trustedByCookie, we upgrade AMR implicitly
					amr = append(amr, "mfa", "totp")
				}
			}
		}

		// Generar Authorization Code
		code, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", "code gen failed", 2107)
			return
		}

		ac := authCode{
			UserID:          sub,
			TenantID:        tid,
			ClientID:        clientID,
			RedirectURI:     redirectURI,
			Scope:           scope,
			Nonce:           nonce,
			CodeChallenge:   codeChallenge,
			ChallengeMethod: codeMethod,
			AMR:             amr,
			ExpiresAt:       time.Now().Add(10 * time.Minute),
		}
		b, _ := json.Marshal(ac)
		// Guardar code en cache (usamos prefijo code:)
		c.Cache.Set("code:"+code, b, 10*time.Minute)

		log.Printf("DEBUG: authorize success, redirecting to %s", redirectURI)

		// Success Redirect
		loc := addQS(redirectURI, "code", code)
		if state != "" {
			loc = addQS(loc, "state", state)
		}

		http.Redirect(w, r, loc, http.StatusFound)
	}
}
