/*
admin_rbac.go — Admin RBAC (roles/perms) con repos opcionales + tenantID desde Bearer

Qué hace este archivo
---------------------
Este archivo implementa endpoints administrativos RBAC (Role-Based Access Control) para:
  1) Gestionar roles asignados a un usuario (listar + asignar/remover roles).
  2) Gestionar permisos asociados a un rol (listar + agregar/remover permisos).

La particularidad: el soporte RBAC depende del driver del Store. Por eso, el handler usa
"type assertions" sobre c.Store para verificar si el backend implementa las interfaces RBAC.

Además, para el endpoint de permisos por rol, el tenant_id se obtiene desde el Bearer token
(leyendo el claim "tid"), asumiendo que un middleware previo (ej. RequireSysAdmin) ya validó acceso.

Dependencias principales
------------------------
- c.Store: store principal. Se usa con type assertions:
    - rbacReadRepo: lectura de roles/permisos por usuario
    - rbacWriteRepo: escritura (assign/remove roles, add/remove perms)
  Si el store no implementa estas interfaces => 501 Not Implemented.

- c.Issuer: issuer JWT (jwtx.Issuer) usado para parsear el Bearer token y extraer "tid".

Helpers internos
----------------
1) rbacReadRepo / rbacWriteRepo:
   Interfaces “opcionales” que el store puede implementar o no.
   Nota técnica: usan un tipo raro de contexto:
     ctxCtx interface{ Done() <-chan struct{} }
   En vez de context.Context. Esto es un olor a legacy y dificulta composición (en V2 debe ser context.Context).

2) parseBearerTenantID(iss, r):
   - Lee Authorization header.
   - Verifica formato "Bearer <token>".
   - Parsea JWT con:
       - Keyfunc del issuer
       - método EdDSA
       - issuer esperado iss.Iss
   - Si token válido, busca el claim "tid" y lo devuelve.
   - Si falta o falla => error.
   Este helper se usa para determinar tenant_id en el endpoint de role perms.

3) dedupTrim([]string):
   - Aplica strings.TrimSpace
   - elimina vacíos
   - deduplica manteniendo orden de aparición
   Se usa para "add" y "remove" en payloads.

Endpoints implementados
-----------------------

A) /v1/admin/rbac/users/{userID}/roles (GET/POST)
-------------------------------------------------
Implementado por AdminRBACUsersRolesHandler(c) que retorna un http.HandlerFunc.

Routing:
- Valida que el path tenga:
    prefijo "/v1/admin/rbac/users/"
    sufijo "/roles"
- Extrae userID del medio y valida que sea UUID.

Dependencias:
- Requiere que c.Store implemente rbacReadRepo (si no: 501 not_supported).

GET:
- rr.GetUserRoles(ctx, userID)
- Responde 200 con:
    { "user_id": "...", "roles": [...] }

POST:
- Requiere además que c.Store implemente rbacWriteRepo (si no: 501 not_supported).
- Lee JSON con decoder + MaxBytesReader 64KB:
    payload: { add: [], remove: [] }
- Normaliza add/remove con dedupTrim.
- Si add no vacío => AssignUserRoles(ctx, userID, add)
- Si remove no vacío => RemoveUserRoles(ctx, userID, remove)
- Luego vuelve a leer roles con GetUserRoles para devolver el estado final.
- Responde 200 con:
    { "user_id": "...", "roles": [...] }

Errores típicos:
- 400 invalid_user_id
- 405 method_not_allowed (si no GET/POST)
- 500 store_error si falla repo

B) /v1/admin/rbac/roles/{role}/perms (GET/POST)
------------------------------------------------
Implementado por AdminRBACRolePermsHandler(c) que retorna un http.HandlerFunc.

Routing:
- Valida path:
    prefijo "/v1/admin/rbac/roles/"
    sufijo "/perms"
- Extrae role del medio y valida no vacío.

Tenant resolution (clave):
- Toma tenant_id parseando el Bearer token y extrayendo claim "tid".
- Si falla => 401 unauthorized.
- Comentario del código: "RequireSysAdmin ya garantizó auth".
  Pero en este handler igual parsea y valida el bearer (redundante, aunque útil para sacar tid).

Dependencias:
- Requiere c.Store implemente rbacWriteRepo (read de role perms también está en esa interfaz).
  Si no => 501.

GET:
- perms := rwr.GetRolePerms(ctx, tenantID, role)
- Responde 200:
    { "tenant_id": "...", "role": "...", "perms": [...] }

POST:
- Lee JSON 64KB:
    payload: { add: [], remove: [] }
- dedupTrim add/remove.
- Si add => AddRolePerms(ctx, tenantID, role, add)
- Si remove => RemoveRolePerms(ctx, tenantID, role, remove)
- Luego re-lee perms para devolver estado final.
- Responde 200:
    { "tenant_id": "...", "role": "...", "perms": [...] }

Puntos de mejora (deuda técnica / refactor hacia V2)
----------------------------------------------------
1) Context type incorrecto en interfaces:
   rbacReadRepo/rbacWriteRepo usan interface{ Done() <-chan struct{} } en vez de context.Context.
   Esto reduce compatibilidad (no tenés Deadline/Value/Err) y complica middlewares y cancelación.
   En V2: usar context.Context en todas las interfaces.

2) Routing manual y repetitivo:
   Valida prefijo/sufijo y extrae substrings a mano.
   En V2: router declarativo con params:
     GET/POST /admin/v2/rbac/users/{userId}/roles
     GET/POST /admin/v2/rbac/roles/{role}/perms

3) TenantID desde token dentro del handler:
   parseBearerTenantID parsea JWT acá mismo.
   En V2: TenantContext debería resolver tenant (tid) en middleware y dejarlo en r.Context().
   El controller solo toma tenantID del contexto (sin volver a parsear JWT).

4) Contratos HTTP sin DTOs explícitos (acoplamiento leve):
   Payloads están como structs anónimos + rbacUserRolesPayload/rbacRolePermsPayload.
   En V2: mover a dtos/:
     dto.RBACUserRolesUpdateRequest {add, remove}
     dto.RBACRolePermsUpdateRequest {add, remove}
   y dto responses.

5) Atomicidad / consistencia:
   POST hace 2 llamadas potenciales (assign y remove) sin transacción.
   Si una falla, el estado queda a medias.
   En V2: service debería aplicar una estrategia:
     - transacción en repo (si DB)
     - o un método único UpdateUserRoles(add, remove)
     - idem para role perms

6) Validaciones de negocio:
   - role vacío se valida, pero no se valida formato (ej. caracteres permitidos).
   - roles/perms no se validan (nombres, prefijos, etc.).
   En V2: definir convenciones (ej. roles: "admin", "viewer"; perms: "admin:tenants:write").

Concurrencia (Golang) — dónde aplica y dónde NO
-----------------------------------------------
- Este módulo es CRUD administrativo chico. No hay beneficio real en goroutines/channels.
- Lo único que podría justificar concurrencia es un endpoint “batch” (ej asignar roles a muchos users),
  donde podrías:
    - usar worker pool con límite (para no matar DB)
    - o transacción/bulk update (mejor que goroutines en DB-bound)

Por defecto en V2: mantenerlo sincrónico y simple.

Mapa a arquitectura V2 (qué sería qué)
--------------------------------------
- DTOs:
  - dto.RBACUserRolesUpdateRequest {add, remove}
  - dto.RBACUserRolesResponse {userId, roles}
  - dto.RBACRolePermsUpdateRequest {add, remove}
  - dto.RBACRolePermsResponse {tenantId, role, perms}

- Controller:
  - RBACUsersController.GetRoles / UpdateRoles
  - RBACRolesController.GetPerms / UpdatePerms

- Service:
  - RBACService:
      GetUserRoles(ctx, userID)
      UpdateUserRoles(ctx, userID, add, remove)
      GetRolePerms(ctx, tenantID, role)
      UpdateRolePerms(ctx, tenantID, role, add, remove)
    + validación de nombres, dedup, atomicidad.

- Repository:
  - RBACRepository (interfaces con context.Context)
    (y el store real implementa esto)

Decisiones de compatibilidad (para no romper)
---------------------------------------------
- GET/POST únicamente.
- userID debe ser UUID.
- role no puede ser vacío.
- tenantID para role perms se extrae del claim "tid".
- dedup/trim se aplica a add/remove.
- Respuesta POST devuelve estado final (re-leyendo roles/perms).

Dos “puntos rojos” específicos para tu refactor V2
--------------------------------------------------
- Ese pseudo-context interface{ Done() <-chan struct{} } es una bomba.
  En V2 clavalo a context.Context y listo.
- parseBearerTenantID en handler: sacalo a middleware (TenantContext) así el controller no parsea tokens.

*/

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

// ---- Repos opcionales (type assertions) ----

type rbacReadRepo interface {
	GetUserRoles(ctxCtx interface{ Done() <-chan struct{} }, userID string) ([]string, error)
	GetUserPermissions(ctxCtx interface{ Done() <-chan struct{} }, userID string) ([]string, error)
}

type rbacWriteRepo interface {
	AssignUserRoles(ctxCtx interface{ Done() <-chan struct{} }, userID string, add []string) error
	RemoveUserRoles(ctxCtx interface{ Done() <-chan struct{} }, userID string, remove []string) error
	GetRolePerms(ctxCtx interface{ Done() <-chan struct{} }, tenantID, role string) ([]string, error)
	AddRolePerms(ctxCtx interface{ Done() <-chan struct{} }, tenantID, role string, add []string) error
	RemoveRolePerms(ctxCtx interface{ Done() <-chan struct{} }, tenantID, role string, remove []string) error
}

// ---- helpers ----

func parseBearerTenantID(iss *jwtx.Issuer, r *http.Request) (string, error) {
	ah := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
		return "", errors.New("missing bearer")
	}
	raw := strings.TrimSpace(ah[len("Bearer "):])
	tk, err := jwt.Parse(raw, iss.Keyfunc(), jwt.WithValidMethods([]string{"EdDSA"}), jwt.WithIssuer(iss.Iss))
	if err != nil || !tk.Valid {
		return "", errors.New("invalid bearer")
	}
	if c, ok := tk.Claims.(jwt.MapClaims); ok {
		if tid, _ := c["tid"].(string); tid != "" {
			return tid, nil
		}
	}
	return "", errors.New("no tid")
}

func dedupTrim(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// ========= /v1/admin/rbac/users/{userID}/roles (GET/POST) =========

type rbacUserRolesPayload struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

func AdminRBACUsersRolesHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Path esperado: /v1/admin/rbac/users/{userID}/roles
		const pfx = "/v1/admin/rbac/users/"
		if !strings.HasPrefix(r.URL.Path, pfx) || !strings.HasSuffix(r.URL.Path, "/roles") {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "ruta no encontrada", 9401)
			return
		}
		tail := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, pfx), "/roles")
		userID := strings.Trim(tail, "/")
		if _, err := uuid.Parse(userID); err != nil || userID == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_user_id", "user_id inválido", 9402)
			return
		}

		rr, okR := c.Store.(rbacReadRepo)
		if !okR {
			httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta RBAC read", 9403)
			return
		}

		switch r.Method {
		case http.MethodGet:
			roles, err := rr.GetUserRoles(r.Context(), userID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9404)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"user_id": userID,
				"roles":   roles,
			})
			return

		case http.MethodPost:
			rwr, okW := c.Store.(rbacWriteRepo)
			if !okW {
				httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta RBAC write", 9405)
				return
			}
			var p rbacUserRolesPayload
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "bad_json", "payload inválido", 9406)
				return
			}
			add := dedupTrim(p.Add)
			rm := dedupTrim(p.Remove)
			if len(add) > 0 {
				if err := rwr.AssignUserRoles(r.Context(), userID, add); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9407)
					return
				}
			}
			if len(rm) > 0 {
				if err := rwr.RemoveUserRoles(r.Context(), userID, rm); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9408)
					return
				}
			}
			roles, err := rr.GetUserRoles(r.Context(), userID)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9409)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"user_id": userID,
				"roles":   roles,
			})
			return

		default:
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 9410)
			return
		}
	}
}

// ========= /v1/admin/rbac/roles/{role}/perms (GET/POST) =========

type rbacRolePermsPayload struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

func AdminRBACRolePermsHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Path esperado: /v1/admin/rbac/roles/{role}/perms
		const pfx = "/v1/admin/rbac/roles/"
		if !strings.HasPrefix(r.URL.Path, pfx) || !strings.HasSuffix(r.URL.Path, "/perms") {
			httpx.WriteError(w, http.StatusNotFound, "not_found", "ruta no encontrada", 9421)
			return
		}
		role := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, pfx), "/perms")
		role = strings.Trim(role, "/")
		if role == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_role", "role vacío", 9422)
			return
		}

		// Tomamos tenant_id del bearer (RequireSysAdmin ya garantizó auth)
		tenantID, err := parseBearerTenantID(c.Issuer, r)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "bearer inválido", 9423)
			return
		}

		rwr, okW := c.Store.(rbacWriteRepo)
		if !okW {
			httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta RBAC write", 9424)
			return
		}

		switch r.Method {
		case http.MethodGet:
			perms, err := rwr.GetRolePerms(r.Context(), tenantID, role)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9425)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"tenant_id": tenantID,
				"role":      role,
				"perms":     perms,
			})
			return

		case http.MethodPost:
			var p rbacRolePermsPayload
			if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&p); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "bad_json", "payload inválido", 9426)
				return
			}
			add := dedupTrim(p.Add)
			rm := dedupTrim(p.Remove)
			if len(add) > 0 {
				if err := rwr.AddRolePerms(r.Context(), tenantID, role, add); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9427)
					return
				}
			}
			if len(rm) > 0 {
				if err := rwr.RemoveRolePerms(r.Context(), tenantID, role, rm); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9428)
					return
				}
			}
			perms, err := rwr.GetRolePerms(r.Context(), tenantID, role)
			if err != nil {
				httpx.WriteError(w, http.StatusInternalServerError, "store_error", err.Error(), 9429)
				return
			}
			httpx.WriteJSON(w, http.StatusOK, map[string]any{
				"tenant_id": tenantID,
				"role":      role,
				"perms":     perms,
			})
			return

		default:
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 9430)
			return
		}
	}
}
