/*
oauth_consent.go — Consent Accept Endpoint (POST) para completar Authorization Code luego de pantalla de consentimiento

Qué es este archivo (la posta)
------------------------------
Este archivo define `NewConsentAcceptHandler(c)` que expone un endpoint HTTP (solo POST) que:
- Consume un `consent_token` “one-shot” guardado en cache (challenge de consentimiento generado por otro handler).
- Permite al usuario aprobar o rechazar el consentimiento.
- Si rechaza: redirige al redirect_uri con `error=access_denied` (RFC).
- Si aprueba: persiste el consentimiento (scopes otorgados) en `c.ScopesConsents` y emite un authorization code
  (reutilizando PKCE/state/nonce/AMR) para redirigir al cliente.

En otras palabras: es el “puente” entre la UI de consentimiento y el retorno a la app (redirect_uri + code).

Rutas soportadas / contrato
---------------------------
- POST (path depende de cómo lo monte el router; el handler no fija ruta)
  Body JSON:
    {
      "consent_token": "....",
      "approve": true|false
    }

Respuestas:
- 405 JSON si no es POST.
- 400 JSON si falta token / token inválido / expirado / payload corrupto.
- 302 Found redirect a redirect_uri:
  - Si approve=false: redirect_uri?error=access_denied[&state=...]
  - Si approve=true: redirect_uri?code=... [&state=...]
- 501 JSON si el store de consents no está habilitado (`c.ScopesConsents == nil`).
- 500 JSON si falla persistir consentimiento o generar code.

Flujo paso a paso (secuencia real)
----------------------------------
1) Validación HTTP:
   - Método: exige POST.
   - Body: `httpx.ReadJSON` -> `consentAcceptReq`.
   - Normaliza `consent_token` con TrimSpace y exige no vacío.
2) Resolver challenge desde cache:
   - key: `consent:token:<token>`
   - `c.Cache.Get(key)`; si no existe => token inválido/expirado.
   - “one-shot”: borra el key inmediatamente con `c.Cache.Delete(key)` (anti-replay).
   - `json.Unmarshal` a `consentChallenge` (tipo definido en otro archivo).
   - Verifica `payload.ExpiresAt` vs `time.Now()`.
3) Si approve=false:
   - arma redirect error RFC:
     - redirect_uri?error=access_denied[&state=...]
   - setea `Cache-Control: no-store` + `Pragma: no-cache`
   - `http.Redirect(302)`
4) Si approve=true:
   A) Persistencia de consent:
      - requiere `c.ScopesConsents != nil` (si no: 501 not_implemented)
      - intenta preferir método “TC” si el store lo implementa:
        `UpsertConsentTC(ctx, tenantID, clientID, userID, scopes)`
      - fallback a `UpsertConsent(ctx, userID, clientID, scopes)` (legacy, sin tenant explícito)
      - si falla => 500
   B) Emisión de authorization code:
      - genera `code` opaco (32)
      - construye `authCode{...}` reutilizando:
        - TenantID / ClientID / UserID / RedirectURI
        - scopes solicitados (join con espacio)
        - nonce, PKCE challenge+method, AMR
        - expira a 5 minutos
      - guarda en cache con key: `oidc:code:<sha256(code)>` ttl 5m
        (nota: acá el code se hashea para storage; el user recibe el code “raw”)
   C) Redirect success:
      - headers no-store/no-cache
      - redirect_uri?code=<raw_code>[&state=...]

Dependencias (reales) y cómo se usan
------------------------------------
- `c.Cache`:
  - Fuente de verdad temporal del consent challenge.
  - Implementa Get/Set/Delete; se usa como one-time token store.
- `c.ScopesConsents`:
  - Persistencia durable del consentimiento (scopes por user+client y opcional tenant).
  - Se soportan 2 APIs:
    - `UpsertConsentTC(...)` (preferida; tenant-aware)
    - `UpsertConsent(...)` (legacy; aparentemente tenant-agnostic)
- `tokens`:
  - `GenerateOpaqueToken` para el code.
  - `SHA256Base64URL` para hashear el code al guardarlo.
- Helpers externos:
  - `httpx.ReadJSON` / `httpx.WriteError` para I/O consistente.
  - `addQS` viene de otro archivo del paquete (se usa para construir redirects con query params).
  - `consentChallenge` y `authCode` también vienen de otros archivos (reuso entre authorize/consent/token).

Seguridad / invariantes (lo crítico)
------------------------------------
- Consent token one-shot:
  - Se borra del cache inmediatamente al leerlo => evita reuso/replay.
- Expiración explícita:
  - Además del TTL del cache, valida `payload.ExpiresAt` (doble cerrojo).
- Redirecciones:
  - Usa `payload.RedirectURI` (ya debería estar validado cuando se creó el challenge).
  - Agrega `Cache-Control: no-store` y `Pragma: no-cache` en redirects (bien para OAuth).
- PKCE/nonce/state:
  - No los recalcula: los “hereda” del challenge para no perder contexto entre UI y backend.
- Hash del authorization code en cache:
  - Reduce impacto si se filtra el storage del cache (no queda el code en claro).
  - OJO: el authorize handler anterior guardaba `code:<code>` sin hashear; acá se guarda hasheado.
    ⇒ inconsistencia peligrosa si /token espera un formato fijo.

Patrones detectados
-------------------
- Token/Challenge pattern:
  - `consent_token` referencia un payload en cache (similar a MFA challenge).
- Capability detection (ports por interface):
  - “Try TC first”: usa type assertion para preferir API tenant-aware.
- Template Method (a mano):
  - “si existe método nuevo -> usarlo; sino fallback legacy”.

Cosas no usadas / legacy / riesgos
----------------------------------
- Inconsistencia de keys de cache para authorization codes:
  - `oauth_authorize.go` guardaba `c.Cache.Set("code:"+code, ...)`
  - acá guarda `c.Cache.Set("oidc:code:"+SHA256(code), ...)`
  Si /token soporta uno solo, vas a tener flows que andan y flows que mueren.
  Esto es EL punto rojo: hay que unificar contrato (prefix + hashing sí/no) entre authorize/consent/token.
- `UpsertConsent(...)` legacy no recibe tenantID:
  - En multi-tenant real, esto puede mezclar consents entre tenants si client_id/user_id no son globalmente únicos
    (o si se reutilizan ids). El método TC es el correcto; el legacy debería quedar deprecado.
- Construcción de URL con `addQS`:
  - Es “naif”: no maneja casos raros (params repetidos, fragments, etc.). Funciona, pero es frágil.

Ideas para V2 (sin implementar, guía de desarme en capas)
---------------------------------------------------------
Meta: sacar del handler la lógica de “consent orchestration” y unificar el contrato de cache de codes.

1) DTO + Controller
- DTO `ConsentAcceptRequest` (token+approve) parseado por un binder común.
- Controller:
  - valida método/body
  - delega al service y traduce a redirect/error

2) Services
- `ConsentChallengeService`:
  - `Consume(token) -> payload` (Get+Delete+ExpiresAt check en un solo lugar)
  - Esto centraliza el “one-shot” y evita duplicación con MFA challenges.
- `ConsentService`:
  - `UpsertConsent(ctx, tenantID, clientID, userID, scopes)` que internamente:
    - usa TC si está
    - si no hay TC, decide política: (a) bloquear, (b) fallback legacy con warning y feature flag
- `AuthCodeService`:
  - `IssueAndStore(ctx, payload, ttl) -> rawCode`
  - Unifica SIEMPRE:
    - prefix (ej: `oidc:code:`)
    - hashing (sí/no, pero uno solo)
    - estructura `authCode`

3) Repos / Ports
- `ConsentRepository` interface con método tenant-aware (sin type assertions en runtime).
- `Cache` interface tipada con helpers:
  - `GetConsentChallenge(token)` / `DeleteConsentChallenge(token)`
  - `SetAuthCode(code, authCodePayload)`

4) Patrones GoF útiles
- Strategy para storage de consents:
  - `TenantAwareConsentRepo` vs `LegacyConsentRepo` (y decidir con wiring, no con type assertion adentro del handler).
- Builder para redirects:
  - construir URL con `net/url` (parse + query.Set) para evitar `addQS` manual.

Concurrencia
------------
No aplica: son operaciones atómicas de cache + 1 write al repo. Meter goroutines acá no suma y sólo agrega riesgos.

Resumen
-------
- Este handler completa el consentimiento: consume `consent_token` one-shot, persiste scopes aprobados y emite auth code + redirect.
- Lo más importante a cuidar es la consistencia del formato de almacenamiento de auth codes (prefix/sha/no-sha) y dejar de depender del repo legacy sin tenant.
*/

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	tokens "github.com/dropDatabas3/hellojohn/internal/security/token"
)

type consentAcceptReq struct {
	Token   string `json:"consent_token"`
	Approve bool   `json:"approve"`
}

func NewConsentAcceptHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 2300)
			return
		}
		var in consentAcceptReq
		if !httpx.ReadJSON(w, r, &in) {
			return
		}
		in.Token = strings.TrimSpace(in.Token)
		if in.Token == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_consent_token", "consent_token requerido", 2301)
			return
		}

		key := "consent:token:" + in.Token
		raw, ok := c.Cache.Get(key)
		if !ok {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_consent_token", "token inválido o expirado", 2302)
			return
		}
		// one-shot
		c.Cache.Delete(key)

		var payload consentChallenge
		if err := json.Unmarshal(raw, &payload); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_consent_token", "payload corrupto", 2303)
			return
		}
		if time.Now().After(payload.ExpiresAt) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_consent_token", "token expirado", 2304)
			return
		}

		if !in.Approve {
			// RFC: access_denied
			loc := addQS(payload.RedirectURI, "error", "access_denied")
			if payload.State != "" {
				loc = addQS(loc, "state", payload.State)
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Pragma", "no-cache")
			http.Redirect(w, r, loc, http.StatusFound)
			return
		}

		// Persistir consentimiento (Upsert con unión en store)
		if c.ScopesConsents == nil {
			httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "scopes/consents no soportado", 2305)
			return
		}

		// Intentar usar método TC primero
		type tcConsent interface {
			UpsertConsentTC(ctx context.Context, tenantID, clientID, userID string, scopes []string) error
		}
		var err error
		if tc, ok := c.ScopesConsents.(tcConsent); ok {
			err = tc.UpsertConsentTC(r.Context(), payload.TenantID, payload.ClientID, payload.UserID, payload.RequestedScopes)
		} else {
			_, err = c.ScopesConsents.UpsertConsent(r.Context(), payload.UserID, payload.ClientID, payload.RequestedScopes)
		}

		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", "no se pudo guardar el consentimiento", 2306)
			return
		}

		// Emitir authorization code reutilizando PKCE/state/nonce
		code, err := tokens.GenerateOpaqueToken(32)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", "no se pudo generar code", 2307)
			return
		}
		ac := authCode{
			UserID:          payload.UserID,
			TenantID:        payload.TenantID,
			ClientID:        payload.ClientID,
			RedirectURI:     payload.RedirectURI,
			Scope:           strings.Join(payload.RequestedScopes, " "),
			Nonce:           payload.Nonce,
			CodeChallenge:   payload.CodeChallenge,
			ChallengeMethod: payload.CodeChallengeMethod,
			AMR:             payload.AMR,
			ExpiresAt:       time.Now().Add(5 * time.Minute),
		}
		buf, _ := json.Marshal(ac)
		c.Cache.Set("oidc:code:"+tokens.SHA256Base64URL(code), buf, 5*time.Minute)

		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		loc := addQS(payload.RedirectURI, "code", code)
		if payload.State != "" {
			loc = addQS(loc, "state", payload.State)
		}
		http.Redirect(w, r, loc, http.StatusFound)
	}
}
