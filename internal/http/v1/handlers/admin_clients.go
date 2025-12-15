/*
admin_clients.go — Admin Clients (DB/Store-backed)

Qué hace este handler
---------------------
Este handler implementa la API administrativa de CRUD de OAuth/OIDC clients (aplicaciones cliente),
pero en este caso trabajando contra la capa de datos (h.c.Store), no contra el control plane provider.

Expone endpoints bajo:
  - POST   /v1/admin/clients             -> crea un client en la DB (store.CreateClient)
  - GET    /v1/admin/clients             -> lista clients por tenant_id (store.ListClients)
  - GET    /v1/admin/clients/{id}        -> obtiene detalle por UUID interno y su active version
  - PUT    /v1/admin/clients/{id}        -> actualiza un client por UUID interno
  - DELETE /v1/admin/clients/{id}        -> borra un client (hard) o “soft” revocando tokens
  - POST   /v1/admin/clients/{id}/revoke -> revoca todos los refresh tokens del client

Es un handler "todo en uno": parsea ruta/método a mano, valida inputs básicos, llama al Store,
y arma respuestas JSON o errores estándar usando httpx.WriteError.

Precondición / dependencia principal
------------------------------------
Requiere h.c.Store != nil.
Si Store es nil, responde 501 Not Implemented con error "store requerido".
Esto evita operar cuando el backend está configurado sin DB o en un modo que no soporta persistencia.

Cómo enruta (routing)
---------------------
Usa un switch con condiciones sobre método y r.URL.Path.
No hay un router declarativo: usa strings.HasPrefix/HasSuffix y comparación exacta de paths.
Además usa un helper local pathID(...) para extraer el primer segmento tras el prefijo.

Nota: El orden de casos importa. En particular:
- Los casos con "/v1/admin/clients/" capturan varias variantes (GET/PUT/DELETE).
- El caso del revoke es específico: HasPrefix(...) + HasSuffix(..., "/revoke") y luego parse manual.

DTOs y formatos usados hoy
--------------------------
Entrada/salida usa directamente core.Client (modelo del store/core) como body.
Esto mezcla contrato HTTP con modelo de persistencia (acoplamiento fuerte).

Respuestas:
- POST create -> 201 Created + JSON del mismo core.Client recibido (con posibles campos completados por store)
- GET list    -> 200 OK + JSON array de clients
- GET by id   -> 200 OK + {"client": <client>, "active_version": <version>}
- PUT update  -> 204 No Content
- DELETE      -> 204 No Content (en soft y hard, si todo ok)
- POST revoke -> 204 No Content

Errores:
Usa httpx.WriteError(...) para devolver un envelope con:
  - error string (ej "missing_fields", "invalid_client_id", etc.)
  - error_description (mensaje)
  - code numérico interno (ej 3001, 3021, etc.)

Validaciones y reglas de negocio aplicadas acá
----------------------------------------------
1) Create (POST /v1/admin/clients):
   - Lee JSON en core.Client usando httpx.ReadJSON.
   - Trim de: TenantID, ClientID, Name, ClientType.
   - Valida obligatorios: tenant_id, client_id, name, client_type.
   - Llama store.CreateClient(ctx, &body).
     - Si ErrConflict -> 409 Conflict
     - Otros -> 400 Bad Request
   - Devuelve 201 y el body JSON.

2) List (GET /v1/admin/clients):
   - Requiere query tenant_id.
   - Param opcional q para filtro/búsqueda.
   - Llama store.ListClients(ctx, tenantID, q).
   - Devuelve lista JSON.

3) Get by ID (GET /v1/admin/clients/{id}):
   - Extrae id del path y valida UUID.
   - Llama store.GetClientByID(ctx, id) que devuelve:
     - client (core.Client)
     - active version (core.ClientVersion u otro tipo)
   - Si ErrNotFound -> 404
   - Otros -> 500
   - Devuelve {"client": c, "active_version": v}.

4) Update (PUT /v1/admin/clients/{id}):
   - Valida UUID.
   - Lee JSON en core.Client, setea body.ID = id.
   - Llama store.UpdateClient(ctx, &body).
   - Si falla -> 400
   - Si ok -> 204.

5) Delete (DELETE /v1/admin/clients/{id}):
   - Valida UUID.
   - Lee query soft=true (case-insensitive).
   - Siempre intenta revocar refresh tokens del client (Store.RevokeAllRefreshTokensByClient).
     *IMPORTANTE*: el error de esta revocación se ignora en delete (se asigna a _), lo cual puede ocultar fallas.
   - Si soft=true -> solo revoca tokens y retorna 204 (no borra DB).
   - Si soft=false -> revoca tokens y luego store.DeleteClient(ctx, id). Si falla -> 400. Si ok -> 204.

6) Revoke (POST /v1/admin/clients/{id}/revoke):
   - Extrae id robustamente (defensivo) y valida UUID.
   - Llama store.RevokeAllRefreshTokensByClient(ctx, id).
   - Si falla -> 500
   - Si ok -> 204.

Puntos de mejora (deuda técnica / refactor hacia V2)
----------------------------------------------------
1) Separación de capas:
   Hoy el handler:
   - enruta (routing)
   - parsea/valida JSON
   - ejecuta reglas (revoke tokens antes de delete, soft delete, etc.)
   - llama directo a Store
   - forma respuestas
   En V2 conviene dividir:
   - Controller (HTTP): parse, validar formato, mapear request DTO -> service
   - Service: lógica de negocio (create/update/delete/revoke, y orden de operaciones)
   - Repository/Client: store.Repository

2) DTOs HTTP vs modelos internos:
   Usar core.Client como input/output acopla la API al storage model.
   En V2: dto.AdminClientCreateRequest / dto.AdminClientUpdateRequest / dto.AdminClientResponse.
   Y mapear desde/hacia core.Client en el service (o mapper).

3) Consistencia de status codes y errores:
   - Create: errores “no conflicto” hoy devuelven 400 (podría ser 500 si es falla interna del store).
   - Update/Delete: devuelve 400 para casi todo (podría distinguir NotFound -> 404, etc.).
   - Delete ignora error de revocar tokens (silencioso).
   En V2: error mapping consistente (400 invalid input, 404 not found, 409 conflict, 500 internal).

4) Routing manual:
   Esta lógica a mano con prefix/suffix es frágil y repetitiva.
   En V2: router declarativo (chi o mux con patrones) con handlers por ruta:
     POST   /admin/v2/clients
     GET    /admin/v2/clients
     GET    /admin/v2/clients/{id}
     PUT    /admin/v2/clients/{id}
     DELETE /admin/v2/clients/{id}
     POST   /admin/v2/clients/{id}/revoke

5) Soft delete:
   Hoy “soft delete” significa únicamente revocar tokens y no borrar el registro.
   No hay campo active=false o deleted_at. O sea que el client sigue existiendo idéntico.
   En V2 podrías:
   - implementar “deactivate” (active=false) y opcionalmente keep in DB
   - y dejar “delete hard” para casos raros

6) Seguridad/multi-tenant:
   Este handler toma tenant_id en query para listar, y en body para crear.
   No valida que el admin token pertenezca a ese tenant (eso debería estar garantizado por middleware/claims).
   En V2: idealmente el tenant se resuelve desde el token (TenantContext) y no se confía en un tenant_id input.

Mapa a arquitectura V2 (qué sería qué)
--------------------------------------
- DTOs:
  - core.Client (body) -> dto.AdminClientCreateRequest / dto.AdminClientUpdateRequest
  - Response list -> []dto.AdminClientItem
  - GetByID -> dto.AdminClientDetailResponse {client, activeVersion}

- Controller:
  - AdminClientsController.Create
  - AdminClientsController.List
  - AdminClientsController.Get
  - AdminClientsController.Update
  - AdminClientsController.Delete
  - AdminClientsController.RevokeSessions

- Service:
  - AdminClientsService:
      Create(ctx, tenantID, req)
      List(ctx, tenantID, q)
      Get(ctx, id)
      Update(ctx, id, req)
      Delete(ctx, id, soft)
      Revoke(ctx, id)

  La regla “revocar refresh antes de borrar” vive acá (no en controller).

- Repository/Client:
  - core.Repository (Store):
      CreateClient, ListClients, GetClientByID, UpdateClient, DeleteClient, RevokeAllRefreshTokensByClient

Decisiones de compatibilidad (para no romper comportamiento actual)
------------------------------------------------------------------
- Create devuelve 201 con el objeto.
- Update devuelve 204.
- Delete (soft o hard) devuelve 204.
- Revoke devuelve 204.
- GetByID devuelve {"client":..., "active_version":...}.
- Validación de id como UUID para rutas con {id}.
- Soft delete hoy solo revoca tokens (no desactiva en DB).

Bonus:
Este handler es el ejemplo perfecto de “Controller gordo”. En V2 quedaría re prolijo así:
- controllers/admin_clients_controller.go
- services/admin_clients_service.go
- dtos/admin_clients_dto.go
- repos/client_repo.go (wrapper del store si querés)
Y listo: el controller no sabe si revocás antes o después, solo llama service.Delete(...).

*/

package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type AdminClientsHandler struct{ c *app.Container }

func NewAdminClientsHandler(c *app.Container) *AdminClientsHandler { return &AdminClientsHandler{c: c} }

// helper: extrae el primer segmento después del prefijo
func pathID(path, prefix string) string {
	s := strings.TrimPrefix(path, prefix)
	if i := strings.IndexByte(s, '/'); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

func (h *AdminClientsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.c.Store == nil {
		httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "store requerido", 3000)
		return
	}

	switch {
	// POST /v1/admin/clients  (create)
	case r.Method == http.MethodPost && r.URL.Path == "/v1/admin/clients":
		var body core.Client
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		body.TenantID = strings.TrimSpace(body.TenantID)
		body.ClientID = strings.TrimSpace(body.ClientID)
		body.Name = strings.TrimSpace(body.Name)
		body.ClientType = strings.TrimSpace(body.ClientType)

		if body.TenantID == "" || body.ClientID == "" || body.Name == "" || body.ClientType == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "tenant_id, client_id, name, client_type obligatorios", 3001)
			return
		}
		if err := h.c.Store.CreateClient(r.Context(), &body); err != nil {
			code := http.StatusBadRequest
			if err == core.ErrConflict {
				code = http.StatusConflict
			}
			httpx.WriteError(w, code, "create_failed", err.Error(), 3002)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(body)

	// GET /v1/admin/clients  (list)
	case r.Method == http.MethodGet && r.URL.Path == "/v1/admin/clients":
		tenantID := strings.TrimSpace(r.URL.Query().Get("tenant_id"))
		if tenantID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "missing_tenant_id", "tenant_id requerido", 3011)
			return
		}
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		items, err := h.c.Store.ListClients(r.Context(), tenantID, q)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "list_failed", err.Error(), 3012)
			return
		}
		_ = json.NewEncoder(w).Encode(items)

	// GET /v1/admin/clients/{id}
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/admin/clients/"):
		id := pathID(r.URL.Path, "/v1/admin/clients/")
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "id debe ser UUID", 3021)
			return
		}
		c, v, err := h.c.Store.GetClientByID(r.Context(), id)
		if err != nil {
			status := http.StatusInternalServerError
			if err == core.ErrNotFound {
				status = http.StatusNotFound
			}
			httpx.WriteError(w, status, "get_failed", err.Error(), 3022)
			return
		}
		resp := map[string]any{"client": c, "active_version": v}
		_ = json.NewEncoder(w).Encode(resp)

	// PUT /v1/admin/clients/{id}
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/v1/admin/clients/"):
		id := pathID(r.URL.Path, "/v1/admin/clients/")
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "id debe ser UUID", 3031)
			return
		}
		var body core.Client
		if !httpx.ReadJSON(w, r, &body) {
			return
		}
		body.ID = id
		if err := h.c.Store.UpdateClient(r.Context(), &body); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "update_failed", err.Error(), 3032)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	// DELETE /v1/admin/clients/{id}
	case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/admin/clients/"):
		id := pathID(r.URL.Path, "/v1/admin/clients/")
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "id debe ser UUID", 3041)
			return
		}
		soft := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("soft")), "true")
		if soft {
			_ = h.c.Store.RevokeAllRefreshTokensByClient(r.Context(), id)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		_ = h.c.Store.RevokeAllRefreshTokensByClient(r.Context(), id)
		if err := h.c.Store.DeleteClient(r.Context(), id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "delete_failed", err.Error(), 3042)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	// POST /v1/admin/clients/{id}/revoke
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/v1/admin/clients/") && strings.HasSuffix(r.URL.Path, "/revoke"):
		raw := strings.TrimPrefix(r.URL.Path, "/v1/admin/clients/")
		// por si aparece algo más después de /revoke (defensivo)
		if i := strings.Index(raw, "/revoke"); i >= 0 {
			raw = raw[:i]
		}
		id := pathID("/v1/admin/clients/"+raw, "/v1/admin/clients/")
		if _, err := uuid.Parse(id); err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_client_id", "id debe ser UUID", 3051)
			return
		}
		if err := h.c.Store.RevokeAllRefreshTokensByClient(r.Context(), id); err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "revoke_failed", err.Error(), 3052)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.NotFound(w, r)
	}
}
