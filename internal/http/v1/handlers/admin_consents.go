/*
admin_consents.go — Admin Consents (Scopes/Consents + best-effort session revoke)

Qué hace este handler
---------------------
Este handler implementa endpoints administrativos para administrar "consentimientos" (consents)
de OAuth/OIDC: el registro de qué scopes otorgó un usuario a un cliente (app).

Expone rutas principales:
  - POST   /v1/admin/consents/upsert
      -> crea o actualiza el consent de (user_id, client_id) con una lista de scopes

  - GET    /v1/admin/consents/by-user/{userID}?active_only=true
    y GET  /v1/admin/consents/by-user?user_id={userID}&active_only=true
      -> lista consents de un usuario (con opción de filtrar solo activos)

  - POST   /v1/admin/consents/revoke
      -> revoca (soft) un consent (user_id, client_id) en un timestamp dado (o ahora)
      -> además intenta revocar refresh tokens del usuario para ese cliente (best-effort)

  - GET    /v1/admin/consents?user_id=&client_id=&active_only=true
      -> si viene user_id + client_id: devuelve 0..1 elemento (wrap en array)
      -> si viene solo user_id: lista consents del usuario
      -> si no viene nada: 400

  - DELETE /v1/admin/consents/{user_id}/{client_id}
      -> revoca el consent en time.Now() (equivalente a revoke) y best-effort revoca refresh tokens

Este handler trabaja con dos dependencias:
  1) h.c.ScopesConsents (core.ScopesConsentsRepository): fuente de verdad para consents/scopes.
  2) h.c.Store (core.Repository / store principal): se usa solo para resolver client_id público a UUID
     y para revocar refresh tokens como “medida efectiva” luego de revocar un consent.

Precondición / driver support
-----------------------------
Requiere que h.c.ScopesConsents != nil.
Si ScopesConsents es nil, responde 501 Not Implemented:
  "scopes/consents no soportado por este driver"
Esto sucede en drivers que no implementan la parte de OAuth scopes/consents.

Resolución de client_id (UUID interno vs client_id público)
-----------------------------------------------------------
Muchas rutas aceptan "client_id" como:
  - UUID interno (core.Client.ID)   -> se usa directo
  - client_id público (OAuth client_id) -> se resuelve a UUID interno

Esto se implementa en resolveClientID():
  1) Trim + validate no vacío
  2) Si parsea como UUID -> return tal cual
  3) Si no es UUID -> llama Store.GetClientByClientID(ctx, in) para buscar por client_id público
     - si not found -> ErrNotFound
     - si ok -> devuelve cl.ID (UUID interno)

NOTA: resolveClientID depende de h.c.Store. En este archivo no se valida Store != nil, así que
si por configuración Store fuera nil, resolver client_id público podría panic o fallar.
En V2 conviene:
  - exigir Store en endpoints que acepten client_id público
  - o mover el mapping client_id público -> UUID al ScopesConsents repo (si tiene esa info)
  - o forzar que admin API use siempre UUID interno (más estricto).

Endpoints y lógica detallada
----------------------------

1) POST /v1/admin/consents/upsert
   Body: { user_id, client_id, scopes: [] }
   - Lee JSON con httpx.ReadJSON.
   - Valida:
       user_id requerido y UUID válido
       client_id requerido
       scopes requerido y no vacío
   - Resuelve client_id a UUID interno mediante resolveClientID().
     - si client no existe -> 404
     - si invalido -> 400
   - Llama ScopesConsents.UpsertConsent(ctx, userID, clientUUID, scopes)
     => crea/actualiza el consent (reemplaza scopes otorgados).
   - Responde 200 OK con el objeto core.UserConsent resultante.

2) GET /v1/admin/consents/by-user/{userID}?active_only=true
   GET /v1/admin/consents/by-user?user_id={userID}&active_only=true
   - Prioriza query param user_id.
   - Si no viene, intenta tomarlo del path (by-user/{id}).
   - Valida UUID user_id.
   - active_only=true opcional: si true, el repo filtra los revocados.
   - Llama ScopesConsents.ListConsentsByUser(ctx, userID, activeOnly)
   - Responde 200 OK con []core.UserConsent.

3) POST /v1/admin/consents/revoke
   Body: { user_id, client_id, at? }
   - Lee JSON.
   - Valida user_id UUID, client_id requerido.
   - Resuelve client_id -> UUID interno.
   - Determina timestamp "at":
       - si body.At viene -> parse RFC3339
       - si no -> time.Now()
   - Llama ScopesConsents.RevokeConsent(ctx, userID, clientUUID, at)
     => marca revoked_at del consent (soft revoke).
   - Luego intenta revocar refresh tokens del usuario para ese client (best-effort):
       - hace type assertion a una interfaz opcional:
           RevokeAllRefreshTokens(ctx, userID, clientID) error
       - si el store la implementa, la llama ignorando el error.
   - Responde 204 No Content.

   IMPORTANTE:
   - La revocación efectiva de sesiones es “best-effort”: el consentimiento se revoca siempre
     si el repo lo permite, pero la invalidación de refresh tokens puede fallar silenciosamente.

4) GET /v1/admin/consents (filtros)
   Query:
     user_id (opcional)
     client_id (opcional, UUID o público)
     active_only=true (opcional)
   Reglas:
     - Si client_id viene: lo normaliza a UUID (resolviendo si es público).
       * Si el client no existe: devuelve lista vacía [] (decisión explícita).
     - Si user_id + client_id:
         => ScopesConsents.GetConsent(ctx, userID, clientUUID)
         - si not found: [] (vacía)
         - si active_only && RevokedAt != nil: [] (vacía)
         - si ok: devuelve []core.UserConsent{uc} (wrap en array)
     - Si solo user_id:
         => ListConsentsByUser(...)
     - Si ninguno:
         => 400 missing_filters

   NOTA: que GetConsent devuelva 0..1 se adapta a “list UI” devolviendo siempre un array.

5) DELETE /v1/admin/consents/{user_id}/{client_id}
   - Extrae ambos segmentos del path y valida que haya exactamente 2.
   - Valida user_id UUID.
   - Resuelve client_id -> UUID interno.
   - Llama RevokeConsent(ctx, userID, clientUUID, time.Now()).
   - Luego best-effort revoca refresh tokens igual que en /revoke.
   - Responde 204 No Content.

Formato de errores / status codes
---------------------------------
Usa httpx.WriteError con códigos internos (2001, 2011, 2021, 2100x...).
En general:
  - 400 para input inválido o faltante
  - 404 si el client público no existe al resolver client_id
  - 500 en algunos list/get si falla el repo
  - 204 en revoke/delete exitoso

Puntos de mejora (deuda técnica / refactor hacia V2)
----------------------------------------------------
1) Separación de capas (Controller vs Service):
   El handler hoy hace:
     - routing y parseo de path
     - parse/validación de JSON
     - resolve client_id público -> UUID (usando Store)
     - lógica de negocio: upsert/list/revoke y además invalidar sesiones best-effort
   En V2:
     - Controller: parse request, DTOs, llamar al Service
     - Service: resolver client_id, ejecutar repo consents, y coordinar la revocación de sesiones
     - Repo: ScopesConsents y Store wrapper

2) Unificar rutas / simplificar:
   Hay dos formas de listar por usuario:
     - /by-user/{id} y /by-user?user_id=
   Podrías quedarte con una sola convención (ideal: path param).
   Lo mismo con GET /consents que hace "multi-modo" (get o list). Se puede separar:
     - GET /consents?user_id=...
     - GET /consents/{user_id}/{client_id}

3) Robustecer dependencia Store:
   resolveClientID requiere h.c.Store para client_id público.
   Si Store es nil o si el driver no soporta ese lookup, se rompe.
   En V2: hacer esto explícito:
     - o exigir UUID interno en admin API (más estricto y simple)
     - o mover la resolución al repositorio de consents (si tiene acceso a clients)
     - o inyectar un ClientLookupService dedicado.

4) Revocación de sesiones:
   Hoy se hace best-effort y se ignoran errores.
   En V2: decidir política:
     - Opción A (segura): si falla revocar tokens => devolver 500 (porque no fue “efectivo”)
     - Opción B (pragmática): mantener best-effort pero loguear/auditar el error
     - Opción C: encolar revocación a un job async (worker pool) y responder 204 rápido.

5) Validación de scopes:
   Upsert exige len(scopes)>0 pero no valida formato/reservados.
   En V2: validar scopes contra catálogo (scope existe / permitido / system) si aplica.

Mapa a arquitectura V2 (qué sería qué)
--------------------------------------
- DTOs:
  - dto.AdminConsentUpsertRequest {userId, clientId, scopes}
  - dto.AdminConsentRevokeRequest {userId, clientId, at?}
  - dto.AdminConsentListResponse []dto.ConsentItem (o directo []core.UserConsent si querés)
  - Normalización clientId: aceptar público o UUID (decidir en V2).

- Controller:
  - AdminConsentsController.Upsert
  - AdminConsentsController.ListByUser
  - AdminConsentsController.Revoke
  - AdminConsentsController.List (o Get)
  - AdminConsentsController.Delete (revoke por path)

- Service:
  - AdminConsentsService:
      ResolveClientUUID(ctx, clientIDOrPublic) -> uuid
      UpsertConsent(ctx, userID, clientID, scopes)
      ListConsentsByUser(ctx, userID, activeOnly)
      GetConsent(ctx, userID, clientID)
      RevokeConsent(ctx, userID, clientID, at)
    + coordinación de invalidación de sesiones:
      RevokeSessionsForUserClient(ctx, userID, clientUUID)

- Repos/Clients:
  - ConsentsRepo (ScopesConsentsRepository)
  - ClientsRepo/Lookup (Store.GetClientByClientID)
  - SessionsRepo (Store optional interface RevokeAllRefreshTokens)

Decisiones de compatibilidad (para no romper comportamiento actual)
------------------------------------------------------------------
- Acepta client_id como UUID o como client_id público.
- GET /consents con user_id+client_id devuelve array (0..1) (no un objeto).
- active_only filtra revocados.
- Revoke/Delete responden 204 y hacen best-effort revoke refresh tokens.
- Si client_id público no existe en GET /consents, retorna [] (lista vacía).

Dos observaciones rápidas (esto te va a servir para V2)

Este handler es de los que más se beneficia de un Service porque coordina dos mundos:
------------------------------------------------------------------------------------
- “consent en DB” (ScopesConsentsRepo)
- “sesiones/tokens” (Store opcional)
Y también es un candidato ideal para usar worker pool si querés que la revocación de
sesiones sea async (sobre todo si revocar tokens implica queries grandes).

*/

package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AdminConsentsHandler struct{ c *app.Container }

func NewAdminConsentsHandler(c *app.Container) *AdminConsentsHandler {
	return &AdminConsentsHandler{c: c}
}

// resolveClientID acepta UUID interno o client_id público.
// Si viene client_id público, lo resuelve a UUID (core.Client.ID).
func (h *AdminConsentsHandler) resolveClientID(r *http.Request, in string) (string, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", core.ErrInvalid
	}
	// ¿Parece UUID?
	if _, err := uuid.Parse(in); err == nil {
		return in, nil
	}
	// Resolver por client_id público
	cl, _, err := h.c.Store.GetClientByClientID(r.Context(), in)
	if err != nil {
		if err == core.ErrNotFound {
			return "", core.ErrNotFound
		}
		return "", err
	}
	return cl.ID, nil
}

func (h *AdminConsentsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.c.ScopesConsents == nil {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "scopes/consents no soportado por este driver", 1900)
		return
	}

	switch {
	// ───────────────────────────────────────────
	// POST /v1/admin/consents/upsert
	// body: { user_id, client_id, scopes: [] }
	// client_id: acepta UUID o client_id público
	// ───────────────────────────────────────────
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/consents/upsert":
		var body struct {
			UserID   string   `json:"user_id"`
			ClientID string   `json:"client_id"`
			Scopes   []string `json:"scopes"`
		}
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		body.UserID = strings.TrimSpace(body.UserID)
		body.ClientID = strings.TrimSpace(body.ClientID)

		if body.UserID == "" || body.ClientID == "" || len(body.Scopes) == 0 {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "user_id, client_id y scopes requeridos", 2001)
			return
		}
		if _, err := uuid.Parse(body.UserID); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 2002)
			return
		}

		cid, err := h.resolveClientID(r, body.ClientID)
		if err != nil {
			status := http.StatusBadRequest
			code := "invalid_client_id"
			desc := "client_id invalido"
			if err == core.ErrNotFound {
				status = http.StatusNotFound
				desc = "client no encontrado"
			}
			httpx.WriteError(w, status, code, desc, 2003)
			return
		}

		uc, err := h.c.ScopesConsents.UpsertConsent(r.Context(), body.UserID, cid, body.Scopes)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "upsert_failed", err.Error(), 2004)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, uc)

	// ───────────────────────────────────────────
	// GET /v1/admin/consents/by-user/{userID}?active_only=true
	// GET /v1/admin/consents/by-user?user_id={userID}&active_only=true
	// ───────────────────────────────────────────
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/admin/consents/by-user"):
		// preferir query param
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		if userID == "" && strings.HasPrefix(r.URL.Path, "/v1/admin/consents/by-user/") {
			// fallback: path param exacto
			raw := strings.TrimPrefix(r.URL.Path, "/v1/admin/consents/by-user/")
			// r.URL.Path NO incluye querystring, así que raw ya es solo el id
			userID = strings.TrimSpace(raw)
		}
		if userID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_user_id", "user_id requerido", 2011)
			return
		}
		if _, err := uuid.Parse(userID); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 2012)
			return
		}
		activeOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active_only")), "true")
		list, err := h.c.ScopesConsents.ListConsentsByUser(r.Context(), userID, activeOnly)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 2013)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, list)

	// ───────────────────────────────────────────
	// POST /v1/admin/consents/revoke
	// body: { user_id, client_id, at? (RFC3339) }
	// client_id: acepta UUID o client_id público
	// ───────────────────────────────────────────
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/consents/revoke":
		var body struct {
			UserID   string `json:"user_id"`
			ClientID string `json:"client_id"`
			At       string `json:"at,omitempty"` // opcional RFC3339
		}
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		body.UserID = strings.TrimSpace(body.UserID)
		body.ClientID = strings.TrimSpace(body.ClientID)

		if body.UserID == "" || body.ClientID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "user_id y client_id requeridos", 2021)
			return
		}
		if _, err := uuid.Parse(body.UserID); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 2022)
			return
		}

		cid, err := h.resolveClientID(r, body.ClientID)
		if err != nil {
			status := http.StatusBadRequest
			code := "invalid_client_id"
			desc := "client_id invalido"
			if err == core.ErrNotFound {
				status = http.StatusNotFound
				desc = "client no encontrado"
			}
			httpx.WriteError(w, status, code, desc, 2023)
			return
		}

		var at time.Time
		if strings.TrimSpace(body.At) != "" {
			t, err := time.Parse(time.RFC3339, body.At)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_at", "at debe ser RFC3339", 2024)
				return
			}
			at = t
		} else {
			at = time.Now()
		}

		if err := h.c.ScopesConsents.RevokeConsent(r.Context(), body.UserID, cid, at); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "revoke_failed", err.Error(), 2025)
			return
		}
		// Revocación efectiva de refresh tokens (best-effort)
		type revAll interface {
			RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error
		}
		if rAll, ok := h.c.Store.(revAll); ok {
			_ = rAll.RevokeAllRefreshTokens(r.Context(), body.UserID, cid)
		}
		w.WriteHeader(http.StatusNoContent)

	// ───────────────────────────────────────────
	// GET /v1/admin/consents?user_id=&client_id=&active_only=true
	// Si user_id && client_id => 0..1 (Get + wrap)
	// Si solo user_id => ListConsentsByUser
	// Si ninguno => 400
	// ───────────────────────────────────────────
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/consents":
		userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
		clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
		activeOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("active_only")), "true")

		// Normalizar clientID (acepta UUID o client_id público)
		if clientID != "" {
			if cid, err := h.resolveClientID(r, clientID); err == nil {
				clientID = cid
			} else if err == core.ErrNotFound {
				// pidieron un par inexistente => lista vacía
				httpx.WriteJSON(w, http.StatusOK, []core.UserConsent{})
				return
			} else {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "client_id invalido", 21001)
				return
			}
		}

		switch {
		case userID != "" && clientID != "":
			if _, err := uuid.Parse(userID); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 21002)
				return
			}
			uc, err := h.c.ScopesConsents.GetConsent(r.Context(), userID, clientID)
			if err != nil {
				if err == core.ErrNotFound {
					httpx.WriteJSON(w, http.StatusOK, []core.UserConsent{})
					return
				}
				httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 21003)
				return
			}
			if activeOnly && uc.RevokedAt != nil {
				httpx.WriteJSON(w, http.StatusOK, []core.UserConsent{})
				return
			}
			httpx.WriteJSON(w, http.StatusOK, []core.UserConsent{uc})
			return

		case userID != "":
			if _, err := uuid.Parse(userID); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 21004)
				return
			}
			list, err := h.c.ScopesConsents.ListConsentsByUser(r.Context(), userID, activeOnly)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 21005)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, list)
			return

		default:
			httpx.WriteError(w, http.StatusBadRequest, "missing_filters", "requerido al menos user_id o user_id+client_id", 21006)
			return
		}

	// ───────────────────────────────────────────
	// DELETE /v1/admin/consents/{user_id}/{client_id}
	// Implementado como revocación (soft) en 'now()'.
	// client_id acepta UUID o client_id público.
	// ───────────────────────────────────────────
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/admin/consents/"):
		tail := strings.TrimPrefix(r.URL.Path, "/v1/admin/consents/")
		parts := strings.Split(strings.Trim(tail, "/"), "/")
		if len(parts) != 2 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_path", "se espera /v1/admin/consents/{user_id}/{client_id}", 21011)
			return
		}
		userID := strings.TrimSpace(parts[0])
		clientID := strings.TrimSpace(parts[1])
		if _, err := uuid.Parse(userID); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id debe ser UUID", 21012)
			return
		}
		cid, err := h.resolveClientID(r, clientID)
		if err != nil {
			code := http.StatusBadRequest
			desc := "client_id invalido"
			if err == core.ErrNotFound {
				code = http.StatusNotFound
				desc = "client no encontrado"
			}
			httpx.WriteError(w, code, "invalid_client_id", desc, 21013)
			return
		}
		if err := h.c.ScopesConsents.RevokeConsent(r.Context(), userID, cid, time.Now()); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "revoke_failed", err.Error(), 21014)
			return
		}
		// Revocación efectiva de refresh tokens (best-effort)
		type revAll interface {
			RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error
		}
		if rAll, ok := h.c.Store.(revAll); ok {
			_ = rAll.RevokeAllRefreshTokens(r.Context(), userID, cid)
		}
		w.WriteHeader(http.StatusNoContent)
		return

	default:
		http.NotFound(w, r)
	}
}
