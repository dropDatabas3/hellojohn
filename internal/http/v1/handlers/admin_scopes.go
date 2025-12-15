/*
admin_scopes.go — Admin Scopes (DB/Store vía ScopesConsents repo) con validación de nombres

Qué hace este handler
---------------------
Este handler expone endpoints administrativos para gestionar el catálogo de "scopes" (OAuth/OIDC)
persistidos en el repositorio ScopesConsents (h.c.ScopesConsents), típicamente basado en DB.

Rutas soportadas:
  - GET    /v1/admin/scopes?tenant_id=...
      -> lista scopes de un tenant

  - POST   /v1/admin/scopes
      -> crea un scope para un tenant (requiere tenant_id y name en el body)

  - PUT    /v1/admin/scopes/{id}
      -> actualiza SOLO la descripción del scope por ID interno (patch-like)
         (no requiere tenant_id; no permite renombrar)

  - DELETE /v1/admin/scopes/{id}
      -> borra un scope por ID interno (con protección “en uso”)

Precondición / driver support
-----------------------------
Requiere h.c.ScopesConsents != nil.
Si ScopesConsents es nil, responde 501 Not Implemented:
  "scopes/consents no soportado por este driver"
Esto aplica cuando el backend corre con un driver/store que no implementa scopes/consents.

Dependencias
------------
- h.c.ScopesConsents: repositorio de scopes/consents:
    - ListScopes(tenantID)
    - CreateScope(tenantID, name, description)
    - UpdateScopeDescriptionByID(id, description)
    - DeleteScopeByID(id)

- validation.ValidScopeName: validador del formato del nombre del scope.
- httpx helpers: ReadJSON / WriteJSON / WriteError.

Flujo detallado por endpoint
----------------------------

1) GET /v1/admin/scopes?tenant_id=...
   - Lee tenant_id desde query param.
   - Si falta -> 400 missing_tenant_id.
   - Llama ScopesConsents.ListScopes(ctx, tenantID).
   - Si error -> 500 server_error.
   - Si ok -> 200 con lista de scopes (JSON).

2) POST /v1/admin/scopes
   Body: { tenant_id, name, description }
   - Lee JSON con httpx.ReadJSON.
   - Trim de tenant_id y name.
   - Valida campos requeridos: tenant_id y name.
   - Validación fuerte del nombre (antes de mutar):
       a) Rechaza mayúsculas: rawName debe ser igual a strings.ToLower(rawName).
       b) Valida formato con validation.ValidScopeName:
          - permitido: [a-z0-9:_-.]
          - longitud 1–64
          - empieza y termina alfanumérico
     Luego normaliza a minúsculas (idempotente).
   - Llama CreateScope(ctx, tenantID, name, description).
     - Si ErrConflict -> 409 scope_exists.
     - Otros -> 400 create_failed.
   - Responde 201 Created con el scope creado (JSON).

   Nota: Esta ruta usa tenant_id desde body. No se deriva del token/tenant context.

3) PUT /v1/admin/scopes/{id}  (patch-like)
   - Extrae {id} desde el path (trim) y valida no vacío.
   - Lee JSON con:
       { name?: *string, description?: *string }
     Usa punteros para distinguir:
       - campo ausente (nil)
       - campo presente vacío ("")
   - Renombrar NO soportado:
       si body.Name != nil y no está vacío => 400 rename_not_supported
   - Si description es nil => no hay nada que actualizar => responde 204 No Content (idempotente).
   - Si description viene:
       - Llama UpdateScopeDescriptionByID(ctx, id, trimmedDescription)
       - Si ErrNotFound -> 404 not_found
       - Otros -> 400 update_failed
       - Si ok -> 204 No Content

   Nota: No requiere tenant_id. Opera por ID interno.

4) DELETE /v1/admin/scopes/{id}
   - Extrae {id} desde el path y valida no vacío.
   - Llama DeleteScopeByID(ctx, id)
     - Si ErrConflict -> 409 scope_in_use (protección: no borrar si está referenciado/usado)
     - Si ErrNotFound -> 404 not_found
     - Otros -> 400 delete_failed
   - Si ok -> 204 No Content

Formato de errores / status codes
---------------------------------
Usa httpx.WriteError con un envelope consistente y códigos internos:
  - 400 por missing_fields/invalid_scope_name/etc.
  - 404 cuando el scope no existe (en PUT/DELETE)
  - 409 cuando existe conflicto (scope_exists / scope_in_use)
  - 500 para errores del repo en list
Responses:
  - List: 200
  - Create: 201
  - Update: 204
  - Delete: 204

Puntos de mejora (deuda técnica / refactor hacia V2)
----------------------------------------------------
1) Separación Controller vs Service:
   El handler contiene reglas de negocio:
     - validación de scope name
     - rename_not_supported
     - idempotencia de update (desc nil => 204)
   En V2: estas reglas deben vivir en un ScopesService, y el controller solo traducir requests/responses.

2) TenantID como input confiable:
   List y Create dependen de tenant_id entregado por query/body.
   En V2: idealmente TenantContext (desde auth/middleware) define el tenant y se elimina tenant_id del request
   (o se permite solo para sysadmin con validación estricta).

3) Contrato REST más consistente:
   Hoy:
     - POST crea por name
     - PUT/DELETE operan por id
   En V2 podrías elegir:
     - por name como identificador (más natural en scopes), o
     - mantener id interno pero agregar GET /scopes/{id}
   También podrías separar:
     - PUT /scopes/{id} (update)
     - PATCH /scopes/{id} (patch)
   (hoy es “PUT patch-like”).

4) Validación de description:
   Se trimea pero no valida longitud / contenido.
   En V2: definir límites (ej 0..256/1024) y sanitización.

5) Concurrencia:
   CRUD de scopes es liviano: no necesita goroutines/channels.
   Lo que sí conviene es:
     - context timeouts (si DB cuelga)
     - rate limiting de admin endpoints

Mapa a arquitectura V2 (qué sería qué)
--------------------------------------
- DTOs:
  - dto.AdminScopeCreateRequest { name, description }
  - dto.AdminScopeResponse { id, tenantId?, name, description, system? }
  - dto.AdminScopeUpdateRequest { description? } (sin name)
  - dto.ErrorResponse estándar

- Controller:
  - AdminScopesController.List
  - AdminScopesController.Create
  - AdminScopesController.UpdateDescription
  - AdminScopesController.Delete

- Service:
  - ScopesService:
      List(ctx, tenantID)
      Create(ctx, tenantID, name, description)  // valida scope name
      UpdateDescription(ctx, scopeID, description)
      Delete(ctx, scopeID)

- Repository:
  - ScopesRepository (implementado por c.ScopesConsents)

Decisiones de compatibilidad (para no romper)
---------------------------------------------
- Create exige tenant_id y name.
- Name debe ser minúsculas y pasar validation.ValidScopeName.
- PUT no permite rename; si no hay description -> 204 idempotente.
- Delete protege conflict "in use" como 409.

Mini tip para V2 (aprovechando que ya validás bien)

Tu regla de scope-name está genial. Yo la movería a services/scopes_service.go y el controller solo hace:
* parse
* svc.CreateScope(ctx, tenantID, dto)
* mapear errores (ErrConflict => 409, etc.)
Así te queda una API limpia, y si mañana agregás scopes “system” (openid/profile/email no borrables),
lo agregás en el service y listo.

*/

package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/dropDatabas3/hellojohn/internal/validation"
)

type AdminScopesHandler struct{ c *app.Container }

func NewAdminScopesHandler(c *app.Container) *AdminScopesHandler { return &AdminScopesHandler{c: c} }

func (h *AdminScopesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.c.ScopesConsents == nil {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "scopes/consents no soportado por este driver", 2400)
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/scopes":
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		if tenantID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_tenant_id", "tenant_id requerido", 2401)
			return
		}
		items, err := h.c.ScopesConsents.ListScopes(r.Context(), tenantID)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 2402)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, items)

	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/scopes":
		var body struct {
			TenantID    string `json:"tenant_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		tenantID := strings.TrimSpace(body.TenantID)
		rawName := strings.TrimSpace(body.Name)
		if tenantID == "" || rawName == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id y name requeridos", 2403)
			return
		}
		// Rechazar mayúsculas antes de mutar
		if rawName != strings.ToLower(rawName) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_scope_name", "el nombre debe estar en minúsculas", 2405)
			return
		}
		if !validation.ValidScopeName(rawName) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_scope_name", "formato inválido: usar [a-z0-9:_-.], 1–64, empieza/termina alfanumérico", 2405)
			return
		}
		name := strings.ToLower(rawName) // persistimos en minúsculas (idempotente)
		res, err := h.c.ScopesConsents.CreateScope(r.Context(), tenantID, name, body.Description)
		if err != nil {
			if errors.Is(err, core.ErrConflict) {
				httpx.WriteError(w, http.StatusConflict, "scope_exists", "ya existe un scope con ese nombre", 2406)
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "create_failed", err.Error(), 2404)
			return
		}
		httpx.WriteJSON(w, http.StatusCreated, res)

	// PUT /v1/admin/scopes/{id} - patch-like (solo description, sin exigir tenant_id)
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/admin/scopes/"):
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/admin/scopes/"))
		if id == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "scope_id requerido", 2410)
			return
		}
		// Usamos punteros para distinguir campo ausente de string vacío.
		var body struct {
			Name        *string `json:"name"`
			Description *string `json:"description"`
		}
		if !httpx.ReadJSON(w, r, &body) { // ReadJSON ya maneja errores 400
			return
		}

		// Renombrar no soportado
		if body.Name != nil && strings.TrimSpace(*body.Name) != "" {
			httpx.WriteError(w, http.StatusBadRequest, "rename_not_supported", "no se puede cambiar el nombre del scope", 2413)
			return
		}

		// Si no viene description (nil) -> no hay nada que actualizar: devolvemos 204 idempotente.
		if body.Description == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		if err := h.c.ScopesConsents.UpdateScopeDescriptionByID(r.Context(), id, strings.TrimSpace(*body.Description)); err != nil {
			if errors.Is(err, core.ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not_found", "scope no encontrado", 2411)
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "update_failed", err.Error(), 2412)
			return
		}
		w.WriteHeader(http.StatusNoContent)
		return

	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/admin/scopes/"):
		id := strings.TrimSpace(strings.TrimPrefix(r.URL.Path, "/v1/admin/scopes/"))
		if id == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "scope_id requerido", 2420)
			return
		}
		if err := h.c.ScopesConsents.DeleteScopeByID(r.Context(), id); err != nil {
			if errors.Is(err, core.ErrConflict) {
				httpx.WriteError(w, http.StatusConflict, "scope_in_use", "no se puede borrar: en uso", 2421)
				return
			}
			if errors.Is(err, core.ErrNotFound) {
				httpx.WriteError(w, http.StatusNotFound, "not_found", "scope no encontrado", 2422)
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "delete_failed", err.Error(), 2423)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.NotFound(w, r)
	}
}
