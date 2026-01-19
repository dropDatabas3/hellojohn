/*
admin_clients_fs.go — Admin Clients (Control Plane / FS Provider)

Qué hace este handler
---------------------
Este handler implementa endpoints administrativos para gestionar OIDC/OAuth clients (aplicaciones
cliente) en un “provider” de control plane (cpctx.Provider) que persiste configuración por tenant
(en modo FS o en modo cluster/raft).

En particular, maneja CRUD básico de clients bajo:
  - GET  /v1/admin/clients                 -> lista clients del tenant
  - POST /v1/admin/clients                 -> crea/actualiza (upsert) un client
  - PUT/PATCH /v1/admin/clients/{clientId} -> crea/actualiza (upsert) un client con clientId fijo
  - DELETE /v1/admin/clients/{clientId}    -> borra un client

A diferencia del handler “admin_clients.go” (que suele trabajar contra DB/Store),
este archivo está orientado al Control Plane y su provider (FS/Raft), usando cpctx.Provider
para leer/escribir configuración de clients.

Cómo resuelve el tenant (multi-tenant routing)
----------------------------------------------
El handler necesita saber “para qué tenant” operar. Para eso, determina un "slug" del tenant
siguiendo un orden de prioridad:

1) Header "X-Tenant-Slug"
2) Header "X-Tenant-ID" (si es UUID, intenta traducirlo a slug consultando el FS Provider)
3) Query param "tenant"
4) Query param "tenant_id" (si es UUID, intenta traducirlo a slug)

Si no llega ninguno, usa default: slug = "local".

La función resolveTenantSlug:
- Si el valor no parece UUID, asume que ya es un slug.
- Si parece UUID, intenta convertirlo a slug consultando cp.AsFSProvider(cpctx.Provider)
  y fsp.GetTenantByID(...). Si falla, hace fallback y devuelve lo original.

IMPORTANTE: este fallback puede esconder inconsistencias (ej: llega un UUID que no existe
en FS provider, y el handler termina usando el UUID como slug, lo cual puede romper o
crear un “tenant fantasma” dependiendo del provider). Esto es un punto a limpiar en V2.

Modo cluster (raft) vs fallback directo
---------------------------------------
Este handler tiene dos caminos de escritura al crear/actualizar/borrar clients:

A) Si existe cluster node (h.container.ClusterNode != nil):
   - Convierte el input cp.ClientInput en un DTO de cluster (cluster.UpsertClientDTO)
   - Serializa a JSON
   - Crea una "Mutation" con:
       Type = MutationUpsertClient o MutationDeleteClient
       TenantSlug = slug
       TsUnix = timestamp
       Payload = JSON del DTO
   - Aplica la mutación al cluster via ClusterNode.Apply(ctx, mutation)
   - Luego hace un "read back" usando cpctx.Provider.GetClient(...) para devolver el estado final.

   Este enfoque garantiza que el cambio quede replicado y ordenado por el cluster.

B) Si NO hay cluster node:
   - Escribe directo llamando cpctx.Provider.UpsertClient(...) o cpctx.Provider.DeleteClient(...)

Este doble camino existe para soportar despliegues “sin cluster” y despliegues “con cluster”.
En V2 conviene encapsular esto en un service (ej: ClientAdminService) y que el controller no
sepa cómo se persiste (cluster vs directo).

Formato de requests/responses
-----------------------------
- Requests:
  - POST y PUT/PATCH esperan JSON con estructura cp.ClientInput.
  - En PUT/PATCH, el clientId del path se fuerza al input (in.ClientID = clientID), pisando el body.

- Responses:
  - Usa un helper local "write" que responde siempre application/json.
  - Los errores locales usan writeErr con {"error": "..."}.
  - En algunos errores de cluster usa httpx.WriteError(...) que retorna un objeto de error más rico
    (con code e info). Esto produce inconsistencia de formato entre errores “locales” y errores “httpx”.

Puntos de mejora (deuda técnica / refactor hacia V2)
----------------------------------------------------
1) Separar responsabilidades:
   Hoy ServeHTTP mezcla:
   - resolución de tenant
   - parseo de path y routing manual
   - decode/validate JSON
   - lógica de negocio (upsert/delete + cluster mutation)
   - serialización y manejo de errores

   En V2 debería quedar:
   - Controller: parsea request, arma DTO de entrada, llama a service, responde
   - Service: decide cluster vs provider directo, aplica reglas, hace read-back si aplica
   - Client/Repo: cpctx.Provider (y/o cluster apply) como dependencias.

2) Normalizar el contrato de errores:
   Ahora hay dos estilos distintos (writeErr vs httpx.WriteError).
   En V2: un error envelope estándar:
     { "error": "...", "error_description": "...", "request_id": "..." }

3) Validaciones:
   Este handler no valida mucho (ej. clientID vacío, json inválido).
   Validaciones de redirect URIs, scopes, etc. quedan delegadas al provider.
   En V2: validar consistentemente en service (y compartir validadores).

4) Tenant resolution:
   Hoy se aceptan múltiples formas (header/query) y se hace fallback “peligroso”.
   En V2: TenantContext debe resolver tenant en un middleware único y fallar si no existe.
   Esto saca de los handlers la lógica de “tenant = local / fallback”.

5) Ruteo manual:
   Usa strings.HasPrefix para detectar /v1/admin/clients/{clientId}.
   En V2: router declarativo (chi, std mux con patterns, etc.) con handlers por ruta.

Mapa a arquitectura V2 (qué sería qué)
--------------------------------------
- DTOs (entrada/salida):
  - cp.ClientInput (hoy) -> V2: dto.AdminUpsertClientRequest
  - cluster.UpsertClientDTO / cluster.DeleteClientDTO -> V2: internos del service/repo cluster.
  - Response: client object del provider -> V2: dto.AdminClientResponse

- Controller:
  - adminClientsFS.ServeHTTP se divide en:
    - AdminClientsController.ListClients
    - AdminClientsController.UpsertClient (POST)
    - AdminClientsController.UpsertClientByID (PUT/PATCH)
    - AdminClientsController.DeleteClient

- Service:
  - ClientAdminService:
      List(tenantSlug)
      Upsert(tenantSlug, input)
      Delete(tenantSlug, clientID)
    Internamente decide:
      if cluster: apply mutation + readback
      else: provider direct

- Client/Repository:
  - ControlPlaneProviderClient (wrapper sobre cpctx.Provider)
  - ClusterClient (wrapper sobre container.ClusterNode.Apply)

Notas de comportamiento (para no romper compatibilidad)
------------------------------------------------------
- Fuerza clientID del path en PUT/PATCH.
- Si cluster está activo, siempre hace read-back antes de responder.
- Default tenant slug: "local" si no viene nada.

*/

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	clusterv1 "github.com/dropDatabas3/hellojohn/internal/cluster/v1"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/google/uuid"
)

type adminClientsFS struct {
	container *app.Container // reservado para logs/metrics si querés
}

func NewAdminClientsFSHandler(c *app.Container) http.Handler {
	return &adminClientsFS{container: c}
}

// isUUID checks if a string is a valid UUID
func isUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// resolveTenantSlug converts a tenant ID (UUID) to its slug
func resolveTenantSlug(ctx context.Context, idOrSlug string) string {
	if !isUUID(idOrSlug) {
		return idOrSlug
	}
	if fsp, ok := cp.AsFSProvider(cpctx.Provider); ok {
		if t, err := fsp.GetTenantByID(ctx, idOrSlug); err == nil && t != nil {
			return t.Slug
		}
	}
	return idOrSlug // fallback
}

func (h *adminClientsFS) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Paths esperados:
	//   /v1/admin/clients               (GET=list, POST=create)
	//   /v1/admin/clients/{clientId}    (PUT=update, DELETE=delete)
	path := r.URL.Path
	base := "/v1/admin/clients"

	// Tenant slug: header > query. Accept both slug and id param names. Default "local".
	slug := "local"
	if v := r.Header.Get("X-Tenant-Slug"); v != "" {
		slug = v
	} else if v := r.Header.Get("X-Tenant-ID"); v != "" {
		slug = resolveTenantSlug(r.Context(), v)
	} else if v := r.URL.Query().Get("tenant"); v != "" {
		slug = v
	} else if v := r.URL.Query().Get("tenant_id"); v != "" {
		slug = resolveTenantSlug(r.Context(), v)
	}

	// helper JSON
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
			clients, err := cpctx.Provider.ListClients(r.Context(), slug)
			if err != nil {
				writeErr(http.StatusInternalServerError, "list clients failed")
				return
			}
			write(http.StatusOK, clients)
			return

		case http.MethodPost:
			var in cp.ClientInput
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeErr(http.StatusBadRequest, "invalid json")
				return
			}
			if h.container != nil && h.container.ClusterNode != nil {
				dto := clusterv1.UpsertClientDTO{
					Name:                     in.Name,
					ClientID:                 in.ClientID,
					Type:                     in.Type,
					RedirectURIs:             in.RedirectURIs,
					AllowedOrigins:           in.AllowedOrigins,
					Providers:                in.Providers,
					Scopes:                   in.Scopes,
					Secret:                   in.Secret,
					RequireEmailVerification: in.RequireEmailVerification,
					ResetPasswordURL:         in.ResetPasswordURL,
					VerifyEmailURL:           in.VerifyEmailURL,
				}
				payload, _ := json.Marshal(dto)
				m := clusterv1.Mutation{
					Type:       clusterv1.MutationUpsertClient,
					TenantSlug: slug,
					TsUnix:     time.Now().Unix(),
					Payload:    payload,
				}
				if _, err := h.container.ClusterNode.Apply(r.Context(), m); err != nil {
					httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
					return
				}
				// Read back and return client
				cobj, err := cpctx.Provider.GetClient(r.Context(), slug, in.ClientID)
				if err != nil {
					writeErr(http.StatusInternalServerError, "readback failed")
					return
				}
				write(http.StatusOK, cobj)
				return
			}
			// Fallback (cluster off): direct write
			cobj, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
			if err != nil {
				writeErr(http.StatusBadRequest, "create/update failed: "+err.Error())
				return
			}
			write(http.StatusOK, cobj)
			return

		default:
			writeErr(http.StatusMethodNotAllowed, "method not allowed")
			return
		}

	case strings.HasPrefix(path, base+"/"):
		clientID := strings.TrimPrefix(path, base+"/")
		if clientID == "" {
			writeErr(http.StatusBadRequest, "missing client id")
			return
		}

		switch r.Method {
		case http.MethodPut, http.MethodPatch:
			var in cp.ClientInput
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeErr(http.StatusBadRequest, "invalid json")
				return
			}
			// forzamos el clientId del path si no vino en body (o lo pisamos)
			in.ClientID = clientID
			if h.container != nil && h.container.ClusterNode != nil {
				dto := clusterv1.UpsertClientDTO{
					Name:                     in.Name,
					ClientID:                 in.ClientID,
					Type:                     in.Type,
					RedirectURIs:             in.RedirectURIs,
					AllowedOrigins:           in.AllowedOrigins,
					Providers:                in.Providers,
					Scopes:                   in.Scopes,
					Secret:                   in.Secret,
					RequireEmailVerification: in.RequireEmailVerification,
					ResetPasswordURL:         in.ResetPasswordURL,
					VerifyEmailURL:           in.VerifyEmailURL,
				}
				payload, _ := json.Marshal(dto)
				m := clusterv1.Mutation{
					Type:       clusterv1.MutationUpsertClient,
					TenantSlug: slug,
					TsUnix:     time.Now().Unix(),
					Payload:    payload,
				}
				if _, err := h.container.ClusterNode.Apply(r.Context(), m); err != nil {
					httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
					return
				}
				// read back
				cobj, err := cpctx.Provider.GetClient(r.Context(), slug, in.ClientID)
				if err != nil {
					writeErr(http.StatusInternalServerError, "readback failed")
					return
				}
				write(http.StatusOK, cobj)
				return
			}
			// fallback
			cobj, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
			if err != nil {
				writeErr(http.StatusBadRequest, "update failed: "+err.Error())
				return
			}
			write(http.StatusOK, cobj)
			return

		case http.MethodDelete:
			// If cluster is present, apply mutation; otherwise direct delete
			if h.container != nil && h.container.ClusterNode != nil {
				payload, _ := json.Marshal(clusterv1.DeleteClientDTO{ClientID: clientID})
				m := clusterv1.Mutation{Type: clusterv1.MutationDeleteClient, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
				if _, err := h.container.ClusterNode.Apply(r.Context(), m); err != nil {
					httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
					return
				}
				write(http.StatusOK, map[string]string{"status": "ok"})
				return
			}
			if err := cpctx.Provider.DeleteClient(r.Context(), slug, clientID); err != nil {
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
