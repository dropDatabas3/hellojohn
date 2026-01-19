/*
social_dynamic.go — comentario/diagnóstico (solo este archivo)

Qué carajo hace este handler
----------------------------
`DynamicSocialHandler` es un “router” para social login multi-tenant, donde el tenant se decide *en runtime*
y las credenciales (client_id / client_secret del proveedor) se cargan desde el Control Plane (cpctx.Provider).

Ruta esperada:
- /v1/auth/social/{provider}/{action}
  - provider: hoy solo "google"
  - action: "start" o "callback"

Flujo alto nivel:
1) Parsear provider/action desde la URL.
2) Resolver tenantSlug:
   - start: viene por query param `tenant` (fallback `tenant_id`)
   - callback: viene adentro de `state` (JWT firmado) => parseStateGeneric => claims["tenant_slug"]
3) Con tenantSlug, pedir el tenant al control plane: cpctx.Provider.GetTenantBySlug(...)
4) Leer settings.SocialProviders y verificar si el proveedor está habilitado.
5) Decrypt secret (secretbox) o fallback a plain text si parece dev.
6) Crear handler específico (googleHandler) con OIDC + pool del tenant + adapters.
7) Delegar:
   - start: generar state “compatible” con googleHandler (incluye tid/cid/redir/nonce + tenant_slug extra)
          y redirigir a Google.
   - callback: delegar a gh.callback(w,r) (que hace exchange, verify id_token, provisioning, MFA, tokens, etc.)

Puntos buenos (bien ahí)
------------------------
- Multi-tenant de verdad: credenciales por tenant desde Control Plane.
- No mezcla config global con tenant config.
- Usa pool específico del tenant cuando TenantSQLManager está disponible.
- El state es JWT EdDSA firmado por tu Issuer, no un random string (bien para integridad).
- Fallback dev para secretos no cifrados (útil, pero ojo en prod).

Riesgos / bugs / cosas flojas
-----------------------------

1) Construcción de redirectURL (scheme) es medio “fantasiosa”
   ----------------------------------------------------------
   `redirectURL := fmt.Sprintf("%s://%s/...", r.URL.Scheme, r.Host)`
   En Go server-side, r.URL.Scheme suele venir vacío porque el server no sabe si el cliente vino por http/https
   (a menos que tu reverse proxy lo setee y vos lo traduzcas).
   Luego hacés fallback:
     - https por defecto
     - http si host arranca con localhost/127.0.0.1
   Problema real: detrás de un proxy (nginx/traefik/alb) puede ser HTTPS afuera, HTTP adentro.
   Si no usás X-Forwarded-Proto, te podés clavar con redirect_uri incorrecto y Google te lo rechaza.

   Qué haría:
   - Si hay `X-Forwarded-Proto`, usar ese.
   - Si tenés en config/tenant settings un “public_base_url”, usar eso siempre.

2) Callback depende de `tenant_slug` en state (y si no, muere)
   -----------------------------------------------------------
   En callback:
   - parseStateGeneric(state) y saca claims["tenant_slug"].
   Si no está, devolvés error 1695.
   Esto es correcto para TU diseño, pero dejás comentado que googleHandler “espera tid UUID”.
   O sea: estás metiendo un claim extra para poder resolver el tenant antes de instanciar OIDC.

   Bien: necesitás tenantSlug para saber qué clientSecret usar antes de verificar id_token.
   Pero: si mañana alguien te pega a /callback de un flow viejo que no incluía tenant_slug, se rompe.

   Mitigación:
   - Guardar también `tenant_slug` en un cache “state jti -> tenant_slug” en start (one-shot / ttl),
     y en callback si no está el claim, usar jti o hash del state para buscarlo.
   - O permitir fallback a tid:
       - si cpctx.Provider soporta GetTenantByID, usarlo
       - sino, indexar tenants por ID en memoria al boot (map[uuid]slug)

3) Aud hardcodeado “google-state”
   -------------------------------
   generateState usa aud "google-state" y parseStateGeneric lo valida.
   Está ok, pero ojo con reutilización si en el futuro agregás GitHub/Microsoft/etc:
   - si todos usan "google-state", no es grave, pero semánticamente raro.
   Mejor: aud = "social-state" y además claim "p" = provider, o aud = "google-state" pero entonces
   `DynamicSocialHandler` debería validar provider también dentro del state.

4) Confusión de nombres: tenantSlug vs tenant_id
   ----------------------------------------------
   En start aceptás `tenant` y fallback a `tenant_id`, pero ambos los tratás como slug.
   Eso te puede generar quilombo porque “tenant_id” suele ser UUID.
   Si alguien te manda UUID en tenant_id, vos lo pasás a GetTenantBySlug y da not_found.

   Arreglo práctico:
   - renombrar query param a `tenant` o `tenant_slug` y chau.
   - si querés soportar UUID, detectá si parsea como uuid, y resolvé por ID.

5) `validator: redirectValidatorAdapter{repo: h.c.Store}` podría validar contra DB incorrecta
   -----------------------------------------------------------------------------------------
   Vos en dynamic instanciás gh con pool del tenant, pero el validator queda apuntando al repo global (`h.c.Store`).
   Si el redirect validator depende de “clientes” en DB global, ok.
   Pero si tus clients viven en control plane FS o DB por tenant, se te desincroniza.

   Según social_google.go, en issueSocialTokens usás ResolveClientFSByTenantID (FS), no DB.
   Entonces lo más consistente sería que el validator también valide contra FS/control plane, no contra `h.c.Store`.

6) Decrypt secret: fallback a plaintext por “no contiene |”
   --------------------------------------------------------
   Está bien para dev, pero en prod es un footgun:
   - si por algún motivo un secreto cifrado cambia formato y no trae "|", lo aceptás igual como plaintext
     y lo mandás a Google: falla y encima te hace perder tiempo.
   Mejor:
   - un flag `AllowPlainSecrets` por env (solo dev)
   - si prod: si no se puede decrypt => error y listo.

7) Seguridad de state: issuer y keyfunc
   ------------------------------------
   parseStateGeneric valida:
   - firma EdDSA con Keyfunc()
   - iss == h.c.Issuer.Iss
   - aud == google-state
   - exp (con un “grace” raro de -30s)

   Ojo:
   - El grace que hacés es “exp < now-30s => expired”; eso da 30s extra después de exp.
     Si querés tolerancia de reloj, suele ser al revés (aceptar tokens apenas antes de nbf/iat),
     pero exp normalmente se respeta estricto o con pequeña tolerancia (5-30s) está ok.
   - No validás `nbf` ni `iat` explícitamente (jwt lib puede no hacerlo si es MapClaims sin options).
     Si te importa, validalo.

8) `pgxpool.Pool` y store del tenant
   ----------------------------------
   Bien que pedís pool del tenantStore.Pool().
   Pero estás asumiendo que GetPG devuelve un tipo con Pool() (comentario dice *pg.Store).
   Si mañana cambiás store, esto rompe.

   Mejor:
   - Definir interfaz chica:
     type PoolProvider interface { Pool() *pgxpool.Pool }
     y listo (ya lo hacés para fallback, pero no para tenantStore).

Cómo lo separaría (sin reescribir todo el sistema)
--------------------------------------------------
Este archivo hoy mezcla 3 responsabilidades:
1) Router de path (/provider/action)
2) Tenant resolution (start por query, callback por state)
3) Factory de provider handler (googleHandler) + wiring (pool/oidc/adapters)

Yo lo partiría en 3 piezas (sin cambiar behavior):
- social_router.go:
  - parse provider/action
  - dispatch a SocialService
- social_tenant_resolver.go:
  - ResolveTenantSlug(r, action) -> slug
  - (start: query param; callback: state; fallback por tid si se banca)
- social_provider_factory.go:
  - BuildGoogleHandler(tenantSlug, tenantSettings, requestContext) -> handler

Eso te deja el ServeHTTP limpito y testeable.

Notas puntuales sobre números de error
--------------------------------------
Los códigos 1690..1708 están bien “secuenciados”, pero ojo que:
- 1693 missing_state en callback no considera tu debug mode (comentaste “assume standard flow first”).
  Si debug mode de verdad admite callback sin state, acá nunca llega.

Qué ajustaría YA (quick wins)
-----------------------------
- redirectURL: usar X-Forwarded-Proto si existe (y quizá X-Forwarded-Host).
- query param: matar `tenant_id` o soportar UUID de verdad.
- validator: que valide redirect_uri contra FS (control plane) como hace el resto del flow social.
- secrets: plaintext fallback solo con env flag (dev).

En resumen
----------
`social_dynamic.go` está bien encaminado como “puente” entre:
- Control plane (tenant settings)
- Proveedor (Google)
- DB por tenant (pool)
pero ahora mismo tiene dos bombas: el scheme/redirect y el validator apuntando al repo global.
Si arreglás eso, te queda bastante sólido.
*/

package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/oauth/google"
	sec "github.com/dropDatabas3/hellojohn/internal/security/secretbox"
)

// DynamicSocialHandler handles social login requests by loading tenant configuration dynamically.
type DynamicSocialHandler struct {
	cfg        *config.Config
	c          *app.Container
	refreshTTL time.Duration
}

func NewDynamicSocialHandler(cfg *config.Config, c *app.Container, refreshTTL time.Duration) *DynamicSocialHandler {
	return &DynamicSocialHandler{
		cfg:        cfg,
		c:          c,
		refreshTTL: refreshTTL,
	}
}

// ServeHTTP dispatches to the appropriate provider handler based on path and tenant config.
// Path expected: /v1/auth/social/{provider}/{action}
// e.g. /v1/auth/social/google/start?tenant=acme
func (h *DynamicSocialHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/auth/social/"), "/")
	if len(parts) < 2 {
		httpx.WriteError(w, http.StatusNotFound, "not_found", "ruta incompleta", 1690)
		return
	}
	provider := parts[0]
	action := parts[1]

	// Extract Tenant Slug from query param (for start) or state (for callback)
	var tenantSlug string
	var err error

	if action == "start" {
		tenantSlug = r.URL.Query().Get("tenant")
		// Fallback to tenant_id if tenant slug is missing, but prefer slug.
		if tenantSlug == "" {
			tenantSlug = r.URL.Query().Get("tenant_id")
		}
		if tenantSlug == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_tenant", "tenant slug requerido", 1691)
			return
		}
	} else if action == "callback" {
		state := r.URL.Query().Get("state")
		if state == "" {
			// Try to fallback to debug headers if enabled, otherwise error
			if r.Header.Get("X-Debug-Google-Email") != "" {
				// Debug mode might not have state, but let's assume standard flow first
			}
			httpx.WriteError(w, http.StatusBadRequest, "missing_state", "state requerido", 1693)
			return
		}
		// We need to parse state to get Tenant Slug.
		claims, err := h.parseStateGeneric(state)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_state", "state inválido: "+err.Error(), 1694)
			return
		}
		// We stored slug in "tenant_slug" or we can look up by "tid" (UUID) if we have a reverse lookup.
		// But better to store slug in state if we want to look it up by slug.
		// However, googleHandler expects "tid" (UUID).
		// So we should have stored both or look up by UUID.
		// Since we need to load tenant config to get credentials to verify token, we need to know which tenant!
		// If we stored "tenant_slug" in state, we can use it.
		tenantSlug, _ = claims["tenant_slug"].(string)
		if tenantSlug == "" {
			// Fallback: try to find tenant by TID UUID if possible?
			// cpctx.Provider (FS) might not support GetTenantByID efficiently or at all if ID is inside YAML.
			// But we can iterate.
			// For now, let's assume we stored tenant_slug.
			httpx.WriteError(w, http.StatusBadRequest, "invalid_state_tenant", "tenant_slug en state inválido", 1695)
			return
		}
	} else {
		httpx.WriteError(w, http.StatusNotFound, "unknown_action", "acción desconocida", 1696)
		return
	}

	// Load Tenant Config from Control Plane (FS)
	if cpctx.Provider == nil {
		httpx.WriteError(w, http.StatusInternalServerError, "cp_missing", "control plane no inicializado", 1697)
		return
	}

	tenant, err := cpctx.Provider.GetTenantBySlug(r.Context(), tenantSlug)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 1698)
		return
	}

	// Check Provider Config in Tenant Settings
	settings := tenant.Settings
	if settings.SocialProviders == nil {
		httpx.WriteError(w, http.StatusForbidden, "provider_disabled", "providers no configurados", 1699)
		return
	}

	var clientID, clientSecret string
	var enabled bool

	switch provider {
	case "google":
		enabled = settings.SocialLoginEnabled || settings.SocialProviders.GoogleEnabled
		clientID = settings.SocialProviders.GoogleClient
		secretEnc := settings.SocialProviders.GoogleSecret
		if secretEnc != "" {
			clientSecret, err = sec.Decrypt(secretEnc)
			if err != nil {
				// Fallback: if it doesn't look encrypted (no pipe separator), use as plain text (dev mode)
				if !strings.Contains(secretEnc, "|") {
					clientSecret = secretEnc
				} else {
					httpx.WriteError(w, http.StatusInternalServerError, "decrypt_error", "error descifrando secreto", 1705)
					return
				}
			}
		}
	default:
		httpx.WriteError(w, http.StatusBadRequest, "unknown_provider", "proveedor no soportado: "+provider, 1703)
		return
	}

	if !enabled {
		httpx.WriteError(w, http.StatusForbidden, "provider_disabled", provider+" deshabilitado", 1701)
		return
	}

	if clientID == "" || clientSecret == "" {
		httpx.WriteError(w, http.StatusInternalServerError, "provider_misconfigured", "credenciales faltantes", 1702)
		return
	}

	// Instantiate Provider Handler
	switch provider {
	case "google":
		// Redirect URI construction
		redirectURL := fmt.Sprintf("%s://%s/v1/auth/social/google/callback", r.URL.Scheme, r.Host)
		if r.URL.Scheme == "" {
			scheme := "https"
			if strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1") {
				scheme = "http"
			}
			redirectURL = fmt.Sprintf("%s://%s/v1/auth/social/google/callback", scheme, r.Host)
		}

		oidc := google.New(clientID, clientSecret, redirectURL, []string{"openid", "profile", "email"})

		// Get tenant-specific store from TenantSQLManager
		var pool *pgxpool.Pool
		if h.c.TenantSQLManager != nil {
			tenantStore, err := h.c.TenantSQLManager.GetPG(r.Context(), tenantSlug)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "tenant_db_error", "no se pudo obtener db del tenant: "+err.Error(), 1708)
				return
			}
			// tenantStore is *pg.Store which has Pool() method
			pool = tenantStore.Pool()
		} else {
			// Fallback to global store (legacy/dev mode)
			type poolGetter interface {
				Pool() *pgxpool.Pool
			}
			if pg, ok := h.c.Store.(poolGetter); ok {
				pool = pg.Pool()
			} else {
				httpx.WriteError(w, http.StatusInternalServerError, "store_incompatible", "store no es compatible con social login (no pool)", 1704)
				return
			}
		}

		gh := &googleHandler{
			cfg:       h.cfg,
			c:         h.c,
			pool:      pool,
			oidc:      oidc,
			issuer:    h.c.Issuer,
			validator: redirectValidatorAdapter{repo: h.c.Store},
			issuerTok: tokenIssuerAdapter{c: h.c, refreshTTL: h.refreshTTL},
		}

		if action == "start" {
			// Extract params
			cid := r.URL.Query().Get("client_id")
			redir := r.URL.Query().Get("redirect_uri")
			if cid == "" {
				httpx.WriteError(w, http.StatusBadRequest, "missing_client_id", "client_id requerido", 1602)
				return
			}

			// Parse Tenant UUID
			tid, err := uuid.Parse(tenant.ID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "invalid_tenant_id", "tenant ID inválido en configuración", 1707)
				return
			}

			// Generate state compatible with googleHandler
			nonce := randB64(16)
			// We use h.generateState to include tenant_slug AND tid/cid/redir/nonce
			state, err := h.generateState(tid, tenantSlug, cid, redir, nonce)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "state_error", "error generando state", 1706)
				return
			}

			// Redirect to Google
			authURL, err := oidc.AuthURL(r.Context(), state, nonce)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "auth_url_error", "error generando auth url", 1606)
				return
			}
			http.Redirect(w, r, authURL, http.StatusFound)

		} else {
			// Callback
			// Delegate to googleHandler.callback
			// It will parse state, verify signature, extract tid/cid, exchange code, etc.
			// Since we initialized gh with the correct OIDC (tenant creds), it should work.
			gh.callback(w, r)
		}

	default:
		httpx.WriteError(w, http.StatusBadRequest, "unknown_provider", "proveedor no soportado: "+provider, 1703)
	}
}

func (h *DynamicSocialHandler) generateState(tid uuid.UUID, tenantSlug, cid, redir, nonce string) (string, error) {
	now := time.Now().UTC()
	claims := jwtv5.MapClaims{
		"iss":         h.c.Issuer.Iss,
		"aud":         "google-state", // Must match what callback expects
		"exp":         now.Add(h.cfg.Providers.LoginCodeTTL).Unix(),
		"iat":         now.Unix(),
		"nbf":         now.Unix(),
		"jti":         fmt.Sprintf("%d", time.Now().UnixNano()),
		"tid":         tid.String(),
		"tenant_slug": tenantSlug, // Extra claim for us to resolve tenant in callback
		"cid":         cid,
		"redir":       redir,
		"nonce":       nonce,
	}
	signed, _, err := h.c.Issuer.SignRaw(claims)
	return signed, err
}

func (h *DynamicSocialHandler) parseStateGeneric(s string) (map[string]any, error) {
	tk, err := jwtv5.Parse(s, h.c.Issuer.Keyfunc(), jwtv5.WithValidMethods([]string{"EdDSA"}))
	if err != nil || !tk.Valid {
		return nil, errors.New("invalid_state_token")
	}
	claims, ok := tk.Claims.(jwtv5.MapClaims)
	if !ok {
		return nil, errors.New("bad_state_claims")
	}
	if iss, _ := claims["iss"].(string); iss != h.c.Issuer.Iss {
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
