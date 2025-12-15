/*
admin_scopes_fs.go — Admin Scopes (Control Plane / FS Provider) + Cluster Mutations

Qué hace este handler
---------------------
Este handler implementa endpoints administrativos para gestionar "scopes" (OAuth/OIDC scopes)
en el Control Plane (cpctx.Provider), en modo filesystem/config (FS provider) o modo cluster (raft).

Maneja rutas bajo:
  - GET  /v1/admin/scopes              -> lista scopes del tenant
  - POST /v1/admin/scopes              -> crea/actualiza (upsert) un scope (por nombre)
  - PUT  /v1/admin/scopes              -> también hace upsert (alias de POST)
  - DELETE /v1/admin/scopes/{name}     -> elimina un scope por nombre

Este handler NO usa la DB/store core: opera contra cpctx.Provider (controlplane provider).
Si el cluster está presente (h.container.ClusterNode), escribe aplicando una mutation replicada.
Si no, escribe directo al provider.

Cómo resuelve tenant (slug)
---------------------------
Determina el tenant slug de forma simple:
  - Header "X-Tenant-Slug" (prioridad)
  - Query param "tenant"
  - Default "local"

A diferencia de admin_clients_fs.go, acá NO acepta X-Tenant-ID ni tenant_id,
ni convierte UUID->slug. Es más simple pero inconsistente con otros handlers FS.

Routing y métodos
-----------------
El routing es manual:
- base := "/v1/admin/scopes"
- si path == base:
    - GET: listar
    - POST/PUT: upsert
- si strings.HasPrefix(path, base+"/"):
    - toma {name} como strings.TrimPrefix(path, base+"/")
    - DELETE: delete por nombre
- caso contrario: 404

DTOs y modelos usados hoy
-------------------------
- Para upsert, el body se decodifica directamente a cp.Scope (modelo de controlplane).
  Esto acopla el contrato HTTP al modelo interno del controlplane.
- Para cluster, se transforma a cluster.UpsertScopeDTO (Name, Description, System)
  y se manda como payload JSON dentro de una cluster.Mutation.

Nota: el handler responde el mismo `cp.Scope` recibido (no hace read-back),
así que si el provider normaliza/ajusta datos, el response puede no reflejar el estado real persistido.

Flujo detallado por endpoint
----------------------------

1) GET /v1/admin/scopes
   - Llama cpctx.Provider.ListScopes(ctx, slug)
   - Si error: 500 "list scopes failed" (error envelope local {"error": msg})
   - Si ok: 200 + JSON array de scopes

2) POST/PUT /v1/admin/scopes  (Upsert)
   - Decodifica JSON request a cp.Scope (s)
   - Si cluster está activo:
       - Construye payload cluster.UpsertScopeDTO con:
           Name = strings.TrimSpace(s.Name)
           Description = s.Description
           System = s.System
       - Construye cluster.Mutation:
           Type = MutationUpsertScope
           TenantSlug = slug
           TsUnix = now
           Payload = JSON del DTO
       - h.container.ClusterNode.Apply(ctx, mutation)
       - Si falla: usa httpx.WriteError(503, "apply_failed", ...)
       - Si ok: responde 200 con el scope s (no read-back)
   - Si cluster NO está activo:
       - cpctx.Provider.UpsertScope(ctx, slug, s)
       - Si falla: 400 "upsert failed: ..."
       - Si ok: 200 con s

3) DELETE /v1/admin/scopes/{name}
   - Extrae name del path (no valida formato ni trims salvo empty check).
   - Si cluster está activo:
       - Aplica MutationDeleteScope con payload DeleteScopeDTO{Name:name}
       - Si ok: responde 200 {"status":"ok"}
   - Si cluster NO está activo:
       - cpctx.Provider.DeleteScope(ctx, slug, name)
       - Si falla: 400 "delete failed: ..."
       - Si ok: 200 {"status":"ok"}

Formato de errores y consistencia
---------------------------------
Este handler tiene dos estilos de errores mezclados:
- writeErr(...) devuelve {"error": "..."} (simple, sin request_id ni code)
- en errores de cluster aplica httpx.WriteError(...) (envelope más rico con códigos internos)

Esto genera inconsistencia con otros handlers (y dentro del mismo handler).

Puntos de mejora (deuda técnica / refactor hacia V2)
----------------------------------------------------
1) Separación de capas (Controller/Service/Repo):
   ServeHTTP mezcla:
     - tenant resolution
     - routing manual
     - parseo JSON
     - lógica de persistencia (cluster vs directo)
     - respuesta y errores
   En V2:
     - Controller: parsea request/params, llama service, responde con envelope estándar.
     - Service: UpsertScope/DeleteScope/ListScopes con tenantSlug y validaciones.
     - Repo/Client: wrapper ControlPlaneProvider + wrapper ClusterClient.

2) Read-back y consistencia de respuesta:
   En modo cluster, responde el input sin verificar qué quedó persistido.
   En V2: después de Apply, hacer read-back:
     - ListScopes o GetScopeByName (si existe)
   y devolver la entidad real.

3) Validación de scope name:
   No valida:
     - name no vacío en upsert (podría quedar vacío con TrimSpace)
     - caracteres permitidos (ej. RFC/convención)
     - reserved/system scopes (openid/profile/email)
   En V2: centralizar validación de scopes y bloquear borrado de system scopes.

4) Tenant resolution inconsistente:
   Solo X-Tenant-Slug o query tenant.
   En V2: TenantContext único resuelve tenant (id/slug) en middleware y controller no decide.
   Además: eliminar default "local" silencioso o hacerlo explícito para single-tenant.

5) Métodos:
   Acepta POST y PUT para upsert en la misma ruta base.
   En V2: definir un contrato claro:
     - POST /scopes (create)
     - PUT /scopes/{name} (update)
     - DELETE /scopes/{name}
     - GET /scopes y GET /scopes/{name}

6) Concurrencia:
   Este handler es CRUD liviano. No necesita goroutines/channels.
   Si el provider/cluster puede tardar, lo correcto es:
     - timeouts con context
     - y/o rate limiting en /admin
   Worker pool no suma acá salvo que implementes operaciones batch.

Mapa a arquitectura V2 (qué sería qué)
--------------------------------------
- DTOs:
  - dto.ScopeUpsertRequest {name, description, system?}
  - dto.ScopeResponse {name, description, system}
  - dto.StatusOK {status:"ok"}
  (evitar exponer cp.Scope directo como DTO público)

- Controller:
  - AdminScopesController.List
  - AdminScopesController.Upsert (o Create/Update según contrato)
  - AdminScopesController.Delete

- Service:
  - ScopesAdminService:
      List(ctx, tenantSlug)
      Upsert(ctx, tenantSlug, req)
      Delete(ctx, tenantSlug, name)
    Internamente decide:
      - si cluster activo: apply mutation + read-back
      - sino: provider directo

- Repo/Client:
  - ControlPlaneScopesClient (cpctx.Provider.*)
  - ClusterMutationsClient (ClusterNode.Apply)

Decisiones de compatibilidad (para no romper comportamiento actual)
------------------------------------------------------------------
- Tenant slug por header X-Tenant-Slug o query tenant, default "local".
- Upsert responde 200 y devuelve el scope (hoy devuelve el input, no estado persistido).
- Delete responde 200 {"status":"ok"}.
- No existe GET /scopes/{name} en este handler actualmente.


Mini consejo para refactors futuros
Si vas a tener dos mundos (FS control plane vs DB store), está buenísimo que en V2 los nombres lo reflejen:
AdminScopesController (API estable)
y adentro el service decide si usa ControlPlaneScopeRepo o DBScopeRepo, pero el controller ni se entera.
Así evitás terminar con *_fs.go y *_db.go duplicando lógica.
*/

package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/cluster"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
)

type adminScopesFS struct {
	container *app.Container
}

func NewAdminScopesFSHandler(c *app.Container) http.Handler {
	return &adminScopesFS{container: c}
}

func (h *adminScopesFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	base := "/v1/admin/scopes"
	path := r.URL.Path

	slug := "local"
	if v := r.Header.Get("X-Tenant-Slug"); v != "" {
		slug = v
	} else if v := r.URL.Query().Get("tenant"); v != "" {
		slug = v
	}

	write := func(code int, v any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(v)
	}
	writeErr := func(code int, msg string) {
		write(code, map[string]string{"error": msg})
	}

	switch {
	case path == base:
		switch r.Method {
		case http.MethodGet:
			scopes, err := cpctx.Provider.ListScopes(r.Context(), slug)
			if err != nil {
				writeErr(http.StatusInternalServerError, "list scopes failed")
				return
			}
			write(http.StatusOK, scopes)
			return

		case http.MethodPost, http.MethodPut:
			var s cp.Scope
			if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
				writeErr(http.StatusBadRequest, "invalid json")
				return
			}
			if h.container != nil && h.container.ClusterNode != nil {
				payload, _ := json.Marshal(cluster.UpsertScopeDTO{Name: strings.TrimSpace(s.Name), Description: s.Description, System: s.System})
				m := cluster.Mutation{Type: cluster.MutationUpsertScope, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
				if _, err := h.container.ClusterNode.Apply(r.Context(), m); err != nil {
					httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
					return
				}
				write(http.StatusOK, s)
				return
			}
			if err := cpctx.Provider.UpsertScope(r.Context(), slug, s); err != nil {
				writeErr(http.StatusBadRequest, "upsert failed: "+err.Error())
				return
			}
			write(http.StatusOK, s)
			return

		default:
			writeErr(http.StatusMethodNotAllowed, "method not allowed")
			return
		}

	case strings.HasPrefix(path, base+"/"):
		name := strings.TrimPrefix(path, base+"/")
		if name == "" {
			writeErr(http.StatusBadRequest, "missing scope name")
			return
		}
		switch r.Method {
		case http.MethodDelete:
			if h.container != nil && h.container.ClusterNode != nil {
				payload, _ := json.Marshal(cluster.DeleteScopeDTO{Name: name})
				m := cluster.Mutation{Type: cluster.MutationDeleteScope, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
				if _, err := h.container.ClusterNode.Apply(r.Context(), m); err != nil {
					httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
					return
				}
				write(http.StatusOK, map[string]string{"status": "ok"})
				return
			}
			if err := cpctx.Provider.DeleteScope(r.Context(), slug, name); err != nil {
				writeErr(http.StatusBadRequest, "delete failed: "+err.Error())
				return
			}
			write(http.StatusOK, map[string]string{"status": "ok"})
			return
		default:
			writeErr(http.StatusMethodNotAllowed, "method not allowed")
			return
		}

	default:
		writeErr(http.StatusNotFound, "not found")
		return
	}
}
