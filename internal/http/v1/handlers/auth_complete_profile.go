/*
auth_complete_profile.go — Completar perfil (custom fields) para usuario autenticado

Qué hace este handler
---------------------
Implementa el endpoint:
  POST /v1/auth/complete-profile

La idea es permitir que un usuario autenticado (vía Bearer JWT) actualice “custom fields” de su perfil.
Pero OJO: además de guardar metadata, intenta mapear dinámicamente algunos campos a columnas reales
de la tabla `app_user` (si existen), para soportar el modelo “user_fields” que crea columnas reales.

Flujo completo (paso a paso)
----------------------------
1) Valida método HTTP
   - Solo permite POST. Si no: 405 method_not_allowed.

2) Valida Bearer token manualmente
   - Lee header Authorization, exige “Bearer ...”.
   - Parsea el JWT con c.Issuer.Keyfunc() y método EdDSA.
   - Extrae claims:
       sub = user_id
       tid = tenant_id (en tu sistema parece ser UUID del tenant)
   - Si falta sub/tid: 400 invalid_token.
   *Nota:* esto está duplicando responsabilidades porque en el resto del sistema
   normalmente un middleware ya valida token y deja claims en context (httpx.GetClaims).

3) Lee body JSON
   Body esperado:
     { "custom_fields": { "campo1": "valor", "campo2": "valor" } }
   - Si viene vacío: 400 invalid_request.

4) Intenta resolver el tenant "slug" a partir del tid (UUID)
   - tenantSlug := tid
   - Si cpctx.Provider existe, llama ListTenants() y recorre buscando t.ID == tid => usa t.Slug.
   Esto es una búsqueda O(N) por request (y además depende de FS/controlplane).
   Si no lo encuentra, se queda con tid como slug (lo cual puede romper GetPG si espera slug).

5) Resuelve el store del tenant
   - Por default usa c.Store (global).
   - Si c.TenantSQLManager existe: intenta GetPG(ctx, tenantSlug) y si ok usa ese store.
   Esto asume que el “user store” es por-tenant.

6) Carga el usuario actual para fusionar metadata
   - userStore.GetUserByID(ctx, sub)
   - Si no existe: 404 user_not_found.
   - Asegura user.Metadata map.
   - Mezcla req.CustomFields dentro de user.Metadata["custom_fields"] (map[string]any).

7) Actualiza la DB con SQL dinámico e introspección de columnas (lo más heavy del handler)
   - Necesita que userStore también exponga Pool() *pgxpool.Pool (type assertion).
   - Hace introspección de schema:
       SELECT column_name FROM information_schema.columns WHERE table_name = 'app_user'
     y arma un set realColumns[colName]=true.
   - Por cada custom field recibido:
       - Si coincide con una columna real (k o strings.ToLower(k)):
           agrega SET "col" = $n con el valor (y lo comilla para soportar nombres raros).
       - Si NO coincide:
           lo deja para metadata (metaUpdates).
   - Si hay metaUpdates:
       agrega SET metadata = $n con el mapa completo ya mergeado.
   - Ejecuta:
       UPDATE app_user SET ... WHERE id = $n

8) Devuelve OK
   - Responde 200 con {"success": true, ...} o "no changes".

Problemas / cuellos de botella / riesgos reales
-----------------------------------------------
A) Duplicación de auth + riesgo de inconsistencias
   - Parsea JWT a mano adentro del handler.
   - Si mañana cambias validación, issuer, aud, etc. tenés que tocar esto también.
   - Mejor: middleware de auth que deja claims en contexto y listo.

B) Resolver tenantSlug listando TODOS los tenants por request (O(N))
   - Es re caro y encima depende del controlplane.
   - Además es frágil: tid es UUID, pero GetPG probablemente quiera slug (o al revés).

C) Introspección de columnas en cada request
   - `information_schema.columns` por request es un ancla.
   - En producción te mata latencia y carga de DB.
   - Esto debería estar cacheado por tenant (y con TTL / invalidación cuando cambia schema).

D) SQL dinámico con identificadores de usuario
   - Aunque comillás columnas, igual estás armando query string.
   - La protección depende de que solo uses nombres que existan en realColumns.
   - Si mañana realColumns se llena con algo inesperado (por bug), podrías abrirte a inyección de identifiers.
   - Además permitir columnas con espacios (“Pais de origen”) es mala idea de base.

E) Mezcla total de responsabilidades
   - Controller hace: auth, resolver tenant, resolver store, leer usuario, merge metadata,
     introspección SQL, construcción query, ejecución DB.
   - Esto te deja imposible testear bien.

F) Tipos / contrato flojo
   - CustomFields map[string]string limita a string, pero después metadata es map[string]any
     y la DB puede querer JSONB con tipos distintos. En tu mundo “user_fields” capaz querés numbers/bools.

Qué patrones usaría para refactorizar (V2)
------------------------------------------
Objetivo: controller finito + service + repo; cero introspección por request; y soporte multi-tenant correcto.

1) DTOs claros (controllers/dtos)
   - dto CompleteProfileRequestV2:
       CustomFields map[string]any `json:"customFields"`
     (camelCase en v2 si estás normalizando)
   - Validador: tamaño máximo de mapa, keys permitidas, etc.

2) Auth: sacar parsing JWT del handler
   - Middleware AuthRequired que:
       - valida Bearer, firma, exp, iss/aud, etc.
       - mete claims en context
   - El controller solo hace:
       claims := httpx.GetClaims(ctx)
       sub := claims["sub"]
       tid := claims["tid"]
   Esto baja complejidad fuerte.

3) TenantResolver (crítico)
   - service TenantResolver:
       ResolveTenant(ctx, tid string) -> (tenantUUID, tenantSlug)
     y cachearlo (LRU/TTL). No list Tenants por request.
   - Mejor aún: que el token ya lleve slug directamente (o ambos tid y tslug).

4) Repositorio de “UserProfileUpdater”
   En vez de hacer introspección acá:
   - repo ProfileRepo.UpdateProfile(ctx, tenantSlug, userID, updates)
   Donde updates ya viene separado:
     Updates.RealColumns map[string]any
     Updates.Metadata map[string]any

5) Cache de columnas reales (por tenant)
   - service ColumnRegistry:
       GetUserColumns(ctx, tenantSlug) -> map[string]bool
     Implementación:
       - cache in-memory con TTL (ej. 5-15 min) + invalidación cuando SyncUserFields corre.
       - primer fetch consulta information_schema, pero no en cada request.
   Incluso podés “precargar” en startup para tenants activos.

6) “Schema-aware” update seguro (sin query string peligrosa)
   - La forma más segura es construir el UPDATE usando pgx + pgx.Identifier para columnas,
     o generar una capa que whitelistée columnas con nombre normalizado.
   - Si realmente soportás nombres con espacios, te complicás al pedo. Mi recomendación:
     normalizar keys (snake_case, [a-z0-9_]) y mapear displayName afuera (UI).

7) Concurrencia (dónde sí, dónde no)
   - Acá NO conviene meter goroutines para DB updates.
   - Lo que sí podés hacer concurrente es:
       - obtener columnas (cache) y obtener usuario (si lo necesitás) en paralelo,
         PERO: en la práctica no vale la pena si está bien cacheado.
   Concurrencia en auth/profile no suele ser el bottleneck, el bottleneck era la introspección.

Plan de refactor concreto (cómo lo partiría)
--------------------------------------------
A) Controller: controllers/auth_complete_profile_controller.go
   - Check method + ReadJSON + validar input
   - sub/tid de context claims
   - llama: profileSvc.CompleteProfile(ctx, tid, sub, req.CustomFields)
   - responde 200

B) Service: services/profile_service.go
   - resuelve tenantSlug con TenantResolver
   - valida keys (tamaño, formato)
   - columns := columnRegistry.UserColumns(tenantSlug)  (cacheado)
   - separa updates:
       for k,v in fields:
         if columns[k] => realUpdates[k]=v
         else metaUpdates[k]=v
   - llama repo.UpdateUserProfile(ctx, tenantSlug, userID, realUpdates, metaUpdates)
   - retorna

C) Repo: repos/user_profile_repo_pg.go
   - ejecuta UPDATE en una transacción si hace falta
   - actualiza metadata JSONB con merge (ideal: jsonb_set / || ) para no tener que leer user antes
   - actualiza columnas reales con SET col = $n
   - evita el SELECT previo del user salvo que sea necesario.

D) Infra: infra/column_registry.go
   - cache TTL por tenant
   - método Invalidate(tenantSlug) para cuando cambian UserFields (migración/sync)

Mejoras puntuales que te dejan “pro” sin drama
----------------------------------------------
- Límite de body: http.MaxBytesReader (ej. 64KB).
- Limitar cantidad de campos y tamaño de cada value.
- Validar keys: nada de espacios; preferible snake_case.
- Guardar custom_fields en un JSONB dedicado (columna custom_fields jsonb) en vez de mezclar todo en metadata.
  (metadata termina siendo un tacho de basura si no lo acotás).

Resumen del veredicto
---------------------
Este handler funciona, pero es un “monolito” que:
- hace auth a mano,
- resuelve tenant de forma cara,
- introspecta schema por request,
- arma SQL dinámico,
- mezcla metadata con columnas reales.

En V2 lo ideal es:
controller fino + ProfileService + ColumnRegistry cacheado + repo Postgres que haga update seguro y eficiente.
*/

package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CompleteProfileRequest is the request body for completing user profile
type CompleteProfileRequest struct {
	CustomFields map[string]string `json:"custom_fields"`
}

// NewCompleteProfileHandler creates a handler for POST /v1/auth/complete-profile
// This endpoint allows authenticated users to update their custom fields.
func NewCompleteProfileHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 1000)
			return
		}

		// 1. Validate Bearer token
		ah := strings.TrimSpace(r.Header.Get("Authorization"))
		if ah == "" || !strings.HasPrefix(strings.ToLower(ah), "bearer ") {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "Authorization header requerido", 1850)
			return
		}
		raw := strings.TrimSpace(ah[len("Bearer "):])

		// Parse token to extract claims
		tk, err := jwtv5.Parse(raw, c.Issuer.Keyfunc(), jwtv5.WithValidMethods([]string{"EdDSA"}))
		if err != nil || !tk.Valid {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "token inválido o expirado", 1851)
			return
		}
		claims, ok := tk.Claims.(jwtv5.MapClaims)
		if !ok {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "claims inválidos", 1852)
			return
		}

		sub, _ := claims["sub"].(string)
		tid, _ := claims["tid"].(string)
		if sub == "" || tid == "" {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_token", "sub/tid faltante en token", 1853)
			return
		}

		// 2. Read request body
		var req CompleteProfileRequest
		if !httpx.ReadJSON(w, r, &req) {
			return
		}
		if len(req.CustomFields) == 0 {
			httpx.WriteError(w, http.StatusBadRequest, "invalid_request", "custom_fields vacíos", 1854)
			return
		}

		// 3. Resolve tenant slug from UUID
		tenantSlug := tid
		if cpctx.Provider != nil {
			if tenants, err := cpctx.Provider.ListTenants(r.Context()); err == nil {
				for _, t := range tenants {
					if t.ID == tid {
						tenantSlug = t.Slug
						break
					}
				}
			}
		}

		// 4. Get tenant store
		var userStore = c.Store // Default
		if c.TenantSQLManager != nil {
			if tStore, err := c.TenantSQLManager.GetPG(r.Context(), tenantSlug); err == nil && tStore != nil {
				userStore = tStore
			}
		}

		// 5. Update user custom fields
		// First get current user to merge custom fields
		user, err := userStore.GetUserByID(r.Context(), sub)
		if err != nil || user == nil {
			httpx.WriteError(w, http.StatusNotFound, "user_not_found", "usuario no encontrado", 1855)
			return
		}

		// Merge new fields into existing metadata/custom_fields
		if user.Metadata == nil {
			user.Metadata = map[string]any{}
		}
		customFields, ok := user.Metadata["custom_fields"].(map[string]any)
		if !ok {
			customFields = map[string]any{}
		}
		for k, v := range req.CustomFields {
			customFields[k] = v
		}
		user.Metadata["custom_fields"] = customFields

		// 6. Save updated user using dynamic SQL to support real columns
		type poolGetter interface {
			Pool() *pgxpool.Pool
		}
		pg, ok := userStore.(poolGetter)
		if !ok {
			httpx.WriteError(w, http.StatusInternalServerError, "store_incompatible", "el store no soporta actualizaciones directas", 1857)
			return
		}
		pool := pg.Pool()

		// Introspect columns to separate real columns from metadata
		rows, err := pool.Query(r.Context(), `SELECT column_name FROM information_schema.columns WHERE table_name = 'app_user'`)
		if err != nil {
			httpx.WriteError(w, http.StatusInternalServerError, "introspect_failed", "error leyendo esquema", 1858)
			return
		}
		defer rows.Close()

		realColumns := make(map[string]bool)
		for rows.Next() {
			var colName string
			if err := rows.Scan(&colName); err == nil {
				realColumns[colName] = true
			}
		}

		// Prepare Update Query
		var setParts []string
		var args []any
		argIdx := 1

		// Fields to go into metadata
		metaUpdates := make(map[string]any)

		// Check each request field
		for k, v := range req.CustomFields {
			// Normalize key to lower case for column matching check? Postgres columns usually lowercase.
			// Let's try exact match or lower match.
			kLower := strings.ToLower(k)
			if realColumns[k] || realColumns[kLower] {
				// It's a real column
				col := k
				if realColumns[kLower] && !realColumns[k] {
					col = kLower
				}
				// FIX: Quote column name to handle spaces like "Pais de origen"
				setParts = append(setParts, fmt.Sprintf("\"%s\" = $%d", col, argIdx))
				args = append(args, v)
				argIdx++
			} else {
				// It goes to metadata
				metaUpdates[k] = v
			}
		}

		// Always merge metadata updates
		if len(metaUpdates) > 0 {
			// Fetch current metadata to merge
			currentMeta := user.Metadata
			if currentMeta == nil {
				currentMeta = make(map[string]any)
			}
			customFields, _ := currentMeta["custom_fields"].(map[string]any)
			if customFields == nil {
				customFields = make(map[string]any)
			}

			for k, v := range metaUpdates {
				customFields[k] = v
			}
			currentMeta["custom_fields"] = customFields

			setParts = append(setParts, fmt.Sprintf("metadata = $%d", argIdx))
			args = append(args, currentMeta)
			argIdx++
		}

		if len(setParts) == 0 {
			// Nothing to update?
			httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "no changes"})
			return
		}

		// Add WHERE clause
		q := fmt.Sprintf("UPDATE app_user SET %s WHERE id = $%d", strings.Join(setParts, ", "), argIdx)
		args = append(args, sub)

		_, err = pool.Exec(r.Context(), q, args...)
		if err != nil {
			log.Printf("CompleteProfile Update Error: %v | Query: %s", err, q)
			httpx.WriteError(w, http.StatusInternalServerError, "update_failed", "error actualizando perfil: "+err.Error(), 1856)
			return
		}

		// 7. Return success
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "Perfil actualizado correctamente",
		})
	}
}
