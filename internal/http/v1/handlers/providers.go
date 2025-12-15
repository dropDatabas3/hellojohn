/*
providers.go — Providers Discovery (Auth UI bootstrap): GET /v1/auth/providers (+ start_url opcional)

Qué es este archivo (la posta)
------------------------------
Este archivo implementa el endpoint “discovery” que el frontend/CLI usa para saber:
	- qué métodos de login están disponibles (password, google, ...)
	- si el provider está habilitado y correctamente configurado (Enabled/Ready)
	- si conviene abrir en popup (Popup)
	- y, para algunos providers, devuelve un start_url listo para iniciar el flujo (Google)

Es un endpoint de UX/bootstrapping, no un endpoint de seguridad.
No autentica, no emite tokens, no valida scopes: solo informa.

Dependencias reales
-------------------
- cfg (config.Config): feature flags y credenciales del provider (providers.google.* + jwt.issuer)
- c.Store: se usa indirectamente vía redirectValidatorAdapter para validar redirect_uri contra el client
	(buscar redirectValidatorAdapter en email_flows_wiring.go: se reutiliza en social/providers)

Ojo: redirectValidatorAdapter hace fallback a control-plane via cpctx.Provider y NO chequea nil.
Si corrés en un modo sin control-plane, una validación de redirect podría panic.

Ruta soportada (contrato efectivo)
----------------------------------
- GET /v1/auth/providers?tenant_id=...&client_id=...&redirect_uri=...

Query params:
	- tenant_id: UUID del tenant (opcional; solo necesario para validar redirect)
	- client_id: client_id público OIDC (opcional; solo necesario para validar redirect)
	- redirect_uri: URL final a la que el flujo social debería retornar (opcional)

Response:
	{
		"providers": [
			{
				"name": "password",
				"enabled": true,
				"ready": true,
				"popup": false
			},
			{
				"name": "google",
				"enabled": true|false,
				"ready": true|false,
				"popup": true,
				"start_url": "/v1/auth/social/google/start?..." (opcional)
				"reason": "..." (opcional; solo misconfig)
			}
		]
	}

Headers:
	- Content-Type: application/json
	- Cache-Control: no-store

Flujo interno (cómo decide lo que devuelve)
------------------------------------------
1) Método
	 - solo GET

2) Siempre incluye "password"
	 - Hardcode: Enabled=true, Ready=true
	 - Nota: esto es informativo. El gating real (si un client permite password) se aplica en otros handlers.

3) Provider Google
	 A) Enabled
			- depende de cfg.Providers.Google.Enabled

	 B) Ready (config correcta)
			- requiere ClientID y ClientSecret no vacíos
			- y además: o RedirectURL explícito, o jwt.issuer para poder derivar un callback

			Si NO está ready:
				- setea Reason con detalle “client_id/secret o redirect_url/jwt.issuer faltantes”
				- responde inmediatamente (password + google) y termina.

	 C) start_url (solo si hay suficiente contexto)
			- Si redirect_uri está vacío, intenta default:
					base = trimRight(cfg.JWT.Issuer, "/")
					redirect_uri = base + "/v1/auth/social/result"
				Si jwt.issuer está vacío, no hay redirect default.

			- Para generar start_url exige:
					tenant_id válido (UUID)
					client_id no vacío
					redirect_uri no vacío

			- Si todo está, valida redirect_uri contra el client:
					h.validator.ValidateRedirectURI(tenantUUID, clientID, redirectURI)
				Si pasa, arma:
					/v1/auth/social/google/start?tenant_id=...&client_id=...&redirect_uri=...
				Si no pasa, simplemente omite start_url (sin reason).

Qué NO hace (importante para no asumir de más)
----------------------------------------------
- No verifica que el client tenga providers ["google"] habilitado (provider gating por client).
	Solo valida redirect_uri. El gating del provider por client vive en el flujo social/auth.

- No valida allowlists de cfg.Providers.Google.AllowedTenants/AllowedClients.
	(Eso suele vivir en el handler que ejecuta el start real.)

- No diferencia readiness por tenant/client: Ready es global a la config del server.

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) Posible panic si cpctx.Provider es nil
	 redirectValidatorAdapter hace fallback a cpctx.Provider sin nil-check.
	 Acá se llama solo si hay tenant_id+client_id+redirect_uri, pero igual puede ocurrir.

2) “password” siempre Enabled/Ready
	 En el producto real, el client puede declarar providers y bloquear password.
	 Esta respuesta puede ser “optimista” y confundir al UI si no lo cruza con /v1/auth/config.

3) redirect default basado en cfg.JWT.Issuer
	 Si jwt.issuer no refleja el host público (proxy), el UI podría recibir un redirect_uri inválido.

4) Señales mezcladas (Enabled/Ready vs start_url)
	 Es posible: google Enabled+Ready pero sin start_url por falta de tenant/client/redirect.
	 Eso es correcto, pero conviene que el frontend lo trate como “necesito contexto”.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
FASE 1 — Separar “discovery global” de “availability por client/tenant”
	- /v2/auth/providers (global): Enabled/Ready por provider (sin start_url)
	- /v2/auth/providers/resolve?tenant_id&client_id&redirect_uri: devuelve start_url y gating final

FASE 2 — Validator robusto
	- Hacer que redirectValidatorAdapter no dependa de cpctx.Provider global sin nil-check.
	- Unificar validación con helpers.ValidateRedirectURI + resolver client/tenant consistente.

FASE 3 — Contrato más explícito
	- Campo "needs" o "missing" para indicar qué falta para construir start_url (tenant_id/client_id/redirect_uri)
		sin exponer “reasons” de runtime.

*/

package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/config"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
)

// Respuesta del discovery
type ProvidersResponse struct {
	Providers []ProviderInfo `json:"providers"`
}

type ProviderInfo struct {
	Name     string  `json:"name"`
	Enabled  bool    `json:"enabled"`
	Ready    bool    `json:"ready"`               // nuevo: indica si el provider está correctamente configurado
	Popup    bool    `json:"popup"`               // hint para el front
	StartURL *string `json:"start_url,omitempty"` // cuando aplique (p.ej. Google)
	Reason   string  `json:"reason,omitempty"`    // SOLO para problemas de configuración del provider (no de runtime)
}

type providersHandler struct {
	c   *app.Container
	cfg *config.Config

	validator redirectValidatorAdapter
}

func NewProvidersHandler(c *app.Container, cfg *config.Config) http.Handler {
	return &providersHandler{
		c:         c,
		cfg:       cfg,
		validator: redirectValidatorAdapter{repo: c.Store},
	}
}

// GET /v1/auth/providers?tenant_id=...&client_id=...&redirect_uri=...
//   - Devuelve qué providers están disponibles.
//   - Si Google está habilitado y listo (ready) y redirect_uri es válido para ese tenant/client,
//     incluye start_url listo para usar.
//   - Ya NO muestra "tenant_id/client_id faltantes" en reason. Ese es tema del flujo, no del provider.
func (h *providersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1701)
		return
	}

	q := r.URL.Query()
	tidStr := strings.TrimSpace(q.Get("tenant_id"))
	cid := strings.TrimSpace(q.Get("client_id"))
	redirect := strings.TrimSpace(q.Get("redirect_uri"))

	resp := ProvidersResponse{Providers: make([]ProviderInfo, 0, 3)}

	// Password: informativo (si querés, podés hacerlo dinámico por client).
	resp.Providers = append(resp.Providers, ProviderInfo{
		Name:    "password",
		Enabled: true,
		Ready:   true,
		Popup:   false,
	})

	// GOOGLE
	if h.cfg.Providers.Google.Enabled {
		pi := ProviderInfo{
			Name:    "google",
			Enabled: true,
			Popup:   true,
		}

		// Determinar si el provider está listo (config correcta)
		// Regla: client_id y client_secret deben existir y el redirect interno (callback) debe ser resoluble.
		googleReady := strings.TrimSpace(h.cfg.Providers.Google.ClientID) != "" &&
			strings.TrimSpace(h.cfg.Providers.Google.ClientSecret) != ""
		// Consideramos "resoluble": si RedirectURL está vacío, debe existir jwt.issuer para derivarlo.
		if strings.TrimSpace(h.cfg.Providers.Google.RedirectURL) == "" && strings.TrimSpace(h.cfg.JWT.Issuer) == "" {
			googleReady = false
		}
		pi.Ready = googleReady

		if !googleReady {
			pi.Reason = "google provider no configurado (client_id/secret o redirect_url/jwt.issuer faltantes)"
			resp.Providers = append(resp.Providers, pi)
			writeJSON(w, resp)
			return
		}

		// Si el caller NO mandó tenant/client o mandó inválidos, no mostramos reason;
		// simplemente no generamos start_url.
		var (
			tid uuid.UUID
			err error
		)
		if tidStr != "" {
			tid, err = uuid.Parse(tidStr)
			if err != nil {
				// tid inválido ⇒ omitimos start_url
				tid = uuid.Nil
			}
		}

		// Si no viene redirect, usamos el result por defecto del servicio
		if redirect == "" {
			base := strings.TrimRight(h.cfg.JWT.Issuer, "/")
			if base != "" {
				redirect = base + "/v1/auth/social/result"
			}
		} else {
			redirect = strings.TrimSpace(redirect)
		}

		// Si tenemos tid y cid válidos y un redirect no vacío, validamos contra el client.
		if tid != uuid.Nil && cid != "" && redirect != "" {
			if h.validator.ValidateRedirectURI(tid, cid, redirect) {
				v := url.Values{}
				v.Set("tenant_id", tid.String())
				v.Set("client_id", cid)
				v.Set("redirect_uri", redirect)
				start := "/v1/auth/social/google/start?" + v.Encode()
				pi.StartURL = &start
			} else {
				// redirect_uri no permitido para ese client ⇒ omitimos start_url
				// (sin reason, para no ensuciar el discovery con errores de runtime)
			}
		}

		resp.Providers = append(resp.Providers, pi)
	} else {
		resp.Providers = append(resp.Providers, ProviderInfo{
			Name:    "google",
			Enabled: false,
			Ready:   false,
			Popup:   true,
		})
	}

	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}
