/*
auth_logout_all.go — Revocar “todas” las refresh tokens de un usuario (opcionalmente filtrado por client)

Qué hace este handler
---------------------
Implementa:
  POST /v1/auth/logout-all   (nombre sugerido por el archivo; la ruta real depende del router)

Objetivo: “cerrar sesión en todos lados”.
Técnicamente: revoca (invalida) refresh tokens persistidas para un user_id, y si viene client_id, puede acotar a ese client.

⚠️ Importante: NO revoca access tokens JWT ya emitidos (son stateless). Lo que logra es que, al expirar el access token,
no puedas refrescar y “se caiga la sesión” en todos los dispositivos.

Entrada / salida
----------------
Request JSON:
  {
    "user_id":   "<uuid o id>",
    "client_id": "<opcional>"
  }

Response:
- 204 No Content si revocó OK
- 400 si falta user_id
- 501 si el store no implementa la interfaz de revocación masiva
- Errores de tenant DB (helpers + httpx helpers)

Flujo paso a paso
-----------------
1) Método
   - Solo POST. Si no => 405.

2) Parse JSON
   - Lee JSON con httpx.ReadJSON (ya maneja límites y errores).

3) Validación
   - target = trim(user_id)
   - si vacío => 400 user_id_required

4) Selección del repo (prefer per-tenant)
   - Requiere c.TenantSQLManager (si no está => error “tenant manager not initialized”)
   - Determina el tenant “slug” usando helpers.ResolveTenantSlug(r)
     (esto usualmente mira header/query/claims, depende del helper)
   - Abre repo del tenant: helpers.OpenTenantRepo(ctx, manager, slug)
     - si ErrNoDBForTenant => httpx.WriteTenantDBMissing
     - si otro error => httpx.WriteTenantDBError

5) Revocación (interface opcional)
   - Usa type assertion a una interfaz local:
       RevokeAllRefreshTokens(ctx, userID, clientID string) error
     (esto evita tocar core.Repository para todos los stores)
   - Si el repo implementa:
       - llama RevokeAllRefreshTokens(target, clientIDTrimmed)
       - si error => 500 revocation_failed
       - si OK => 204 No Content
   - Si NO implementa => 501 not_supported

Qué está bien / qué es medio flojito (sin decidir ahora)
--------------------------------------------------------
- Bien:
  - Interfaz local (type assertion) para no ensuciar core.Repository: práctico.
  - Prioriza repo per-tenant (correcto si las refresh viven en el DB del tenant).
  - Maneja “no DB” con respuesta específica.

- Flojo / raro:
  1) No valida formato de user_id (UUID)
     - En otros handlers se valida con uuid.Parse. Acá no.
     - Si el store espera UUID y le mandás cualquier cosa, vas a tener errores raros.

  2) No usa fallback a global store
     - Si tu arquitectura permite refresh tokens en store global, acá no hay plan B.
     - Hoy directamente error: “tenant manager not initialized”.

  3) Error final poco consistente
     - “WriteTenantDBError(w, "tenant manager not initialized")” devuelve algo tipo 5xx,
       pero semánticamente es 500 internal_error / not_configured.

  4) Nombre: “logout-all”
     - Semánticamente está bien, pero ojo con expectativas: no mata access tokens activos.

Patrones que aplican para refactor (GoF + arquitectura)
-------------------------------------------------------
- Strategy (selección de repositorio)
  ResolverRepo(ctx, r) -> repo
  Así no repetimos lógica en cada handler (resolve tenant slug + OpenTenantRepo + map errores).

- Adapter / Ports & Adapters
  Definir un puerto “RefreshTokenRevoker” que pueda tener implementación per-tenant o global.
  El handler depende del puerto, no del repo concreto.

- Command (acción de revocación)
  Un “RevokeSessionsCommand{UserID, ClientID}” ejecutado por un servicio.
  Útil para log/audit y para testear sin HTTP.

- (Opcional) Chain of Responsibility
  Si querés probar revocación per-tenant y si no existe probar global, etc.

Ideas de eficiencia y reutilización para el repaso global
---------------------------------------------------------
- Extraer helper común:
    ResolveTenantRepoOrFail(w,r,c) (devuelve repo o ya respondió error)
- Estandarizar validación:
    validateUUID("user_id", target)
- Agregar audit (si existe módulo audit) + métricas (cuántos tokens revocados).
- Dejar claro en docstring: “revoca refresh tokens, no access tokens”.

En resumen
----------
Es un handler cortito que hace “logout global” invalidando refresh tokens del usuario (y opcionalmente del client),
operando contra el repo per-tenant. Está bien encaminado, pero le falta consistencia con validaciones y con
la forma de resolver repos/errores que ya aparece repetida en otros handlers.
*/

package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
)

func NewAuthLogoutAllHandler(c *app.Container) http.HandlerFunc {
	// Interface opcional para no cambiar core.Repository
	type revoker interface {
		RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 2500)
			return
		}
		var req struct {
			UserID   string `json:"user_id,omitempty"`
			ClientID string `json:"client_id,omitempty"`
		}
		if !httpx.ReadJSON(w, r, &req) {
			return
		}

		target := strings.TrimSpace(req.UserID)
		if target == "" {
			httpx.WriteError(w, http.StatusBadRequest, "user_id_required", "falta user_id", 2501)
			return
		}

		// Prefer per-tenant repository if available
		if c != nil && c.TenantSQLManager != nil {
			slug := helpers.ResolveTenantSlug(r)
			repo, err := helpers.OpenTenantRepo(r.Context(), c.TenantSQLManager, slug)
			if err != nil {
				if errors.Is(err, tenantsql.ErrNoDBForTenant) {
					httpx.WriteTenantDBMissing(w)
					return
				}
				httpx.WriteTenantDBError(w, err.Error())
				return
			}
			if rv, ok := any(repo).(revoker); ok {
				if err := rv.RevokeAllRefreshTokens(r.Context(), target, strings.TrimSpace(req.ClientID)); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "revocation_failed", "no se pudo revocar", 2503)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
			httpx.WriteError(w, http.StatusNotImplemented, "not_supported", "store no soporta revocación masiva", 2502)
			return
		}
		httpx.WriteTenantDBError(w, "tenant manager not initialized")
	}
}
