/*
admin_tenants_fs.go — “Monohandler” Admin Tenants (FS Control Plane) + Infra ops + Users + Keys + Settings

Qué es este archivo (la posta)
------------------------------
Este archivo define NewAdminTenantsFSHandler(c) que devuelve UN solo http.HandlerFunc que actúa como:
  - Router manual para muchas rutas bajo /v1/admin/tenants
  - Controller de tenants (CRUD)
  - Controller de tenant settings con ETag / If-Match
  - Servicio de “infra ops” (test DB, migrate, schema apply, infra stats, cache test)
  - Servicio de users por tenant (list/create/update/delete)
  - Servicio de mailing (test email) por tenant
  - Servicio de rotación de keys por tenant (JWKS/Issuer keys) con soporte cluster
  - CRUD parcial de clients y scopes bajo un tenant (delegando a cpctx.Provider)
  - Encriptación de secretos (en varios formatos distintos!) + guardado de logo en FS

Es, literalmente, un “god handler”: mezcla routing + validación + negocio + persistencia + infraestructura + crypto.
Por eso la complejidad cognitiva y el riesgo de bugs/duplicación.

Fuente de verdad / modo FS
--------------------------
Este handler SOLO funciona si cpctx.Provider es un FS provider (controlplane/fs):
  fsProvider, ok := controlplane.AsFSProvider(cpctx.Provider)
Si no, responde 501 fs_provider_required.

En modo cluster:
- Si c.ClusterNode != nil, muchas operaciones “write” se hacen aplicando cluster.Mutation (raft replication).
- Si no hay cluster, escribe directo sobre fsProvider.

Helpers locales dentro del handler
----------------------------------
- etag(data []byte) -> genera ETag SHA256 truncado (8 bytes) como string `"abcd..."`.
- validSlug(slug) -> valida 1..32 y regex ^[a-z0-9\-]+$.

Problema: varios helpers están duplicados (saveLogo se define 3 veces), y hay inconsistencias de crypto.

Rutas soportadas (lo que expone realmente)
------------------------------------------
(Es un resumen “por módulos” del switch gigante)

A) Tenants base
---------------
- GET  /v1/admin/tenants
    -> fsProvider.ListTenants()

- POST /v1/admin/tenants
    -> crea tenant + settings, procesa logo, encripta secretos,
       persiste (cluster mutation o fsProvider.UpsertTenant),
       luego intenta:
          * migración automática DB (TenantSQLManager.MigrateTenant)
          * generación de keys iniciales (Issuer.Keys.RotateFor)

- GET  /v1/admin/tenants/{slug}
    -> fsProvider.GetTenantRaw() y devuelve tenant + ETag

- PUT  /v1/admin/tenants/{slug}
    -> upsert “completo” de tenant: valida slug, issuerMode, procesa logo, encripta secretos,
       aplica cluster mutation o fsProvider.UpdateTenantSettings
       luego si UserFields != nil sincroniza schema (tenantsql.SchemaManager.SyncUserFields) bajo lock

- DELETE /v1/admin/tenants/{slug}
    -> borra tenant (cluster mutation o fsProvider.DeleteTenant)

B) Settings con control de concurrencia optimista (ETag)
--------------------------------------------------------
- GET /v1/admin/tenants/{slug}/settings
    -> fsProvider.GetTenantSettingsRaw(), devuelve settings + ETag

- PUT /v1/admin/tenants/{slug}/settings
    -> requiere If-Match, verifica ETag contra settings actuales,
       parsea newSettings y:
         * encripta secretos (OJO: acá usa jwtx + SIGNING_MASTER_KEY y base64url)
         * valida issuerMode
         * procesa logo
       luego aplica:
         - cluster mutation UpdateTenantSettings (si cluster)
         - o fsProvider.UpdateTenantSettings (si no cluster)
       luego si UserFields != nil sincroniza schema bajo lock
       devuelve {"updated": true} + ETag nuevo

C) Keys rotation por tenant (JWKS)
----------------------------------
- POST /v1/admin/tenants/{slug}/keys/rotate?graceSeconds=
    -> valida tenant existe
    -> graceSeconds se lee de query o ENV KEY_ROTATION_GRACE_SECONDS (default 60)
    -> si cluster:
         - rota keys en líder: Issuer.Keys.RotateFor(slug, grace)
         - lee active.json/retiring.json del FS
         - manda mutation RotateTenantKey con el material exacto (active/retiring JSON)
         - invalida JWKSCache
         - responde {"kid": ...} con Cache-Control no-store
       si no cluster:
         - rota local, invalida cache y responde {"kid": ...}

D) User store infra ops (DB por tenant) y schema
------------------------------------------------
- POST /v1/admin/tenants/test-connection
    -> test DSN arbitrario (transient pgxpool, ping con timeout 5s)

- POST /v1/admin/tenants/{slug}/user-store/test-connection
    -> verifica tenant existe
    -> TenantSQLManager.GetPG(slug) y Ping bajo WithTenantMigrationLock

- POST /v1/admin/tenants/{slug}/user-store/migrate
    -> verifica tenant existe
    -> TenantSQLManager.MigrateTenant(slug)
    -> si DeadlineExceeded => 409 conflict + Retry-After

- POST /v1/admin/tenants/{slug}/migrate
    -> también llama MigrateTenant(slug) (endpoint duplicado con el de user-store/migrate)

- POST /v1/admin/tenants/{slug}/schema/apply
    -> parse schemaDef (map[string]any)
    -> SchemaManager.EnsureIndexes(ctx, slug, schemaDef)

- GET /v1/admin/tenants/{slug}/infra-stats
    -> junta stats db (TenantSQLManager.GetStats) + cache (client.Stats) y los devuelve

E) Users CRUD por tenant (mezclado en este handler)
---------------------------------------------------
- GET  /v1/admin/tenants/{slug}/users
    -> obtiene store del tenant (TenantSQLManager.GetPG)
    -> obtiene tenant para su ID real
    -> usa type assertion: ListUsers(ctx, tenantID) ([]core.User, error)

- POST /v1/admin/tenants/{slug}/users
    -> parse {email, password, email_verified, custom_fields, source_client_id?}
    -> normaliza email
    -> hash password (password.Hash)
    -> store.CreateUser + store.CreatePasswordIdentity
    -> devuelve user creado

- PATCH /v1/admin/tenants/{slug}/users/{id}
    -> solo soporta update de source_client_id con reglas especiales ("", "_none" => NULL)
    -> usa type assertion UpdateUser(ctx, userID, updates)
    -> intenta devolver el user (GetUserByID) si existe interface

- DELETE /v1/admin/tenants/{slug}/users/{id}
    -> usa type assertion DeleteUser(ctx, userID)

F) Cache y mailing por tenant
-----------------------------
- POST /v1/admin/tenants/{slug}/cache/test-connection
    -> TenantCacheManager.Get(slug) -> client.Ping()

- POST /v1/admin/tenants/{slug}/mailing/test
    -> fsProvider.GetTenantBySlug(slug) y delega a SendTestEmail(w,r,t)

G) Clients y scopes debajo del tenant (FS control plane)
--------------------------------------------------------
- PUT /v1/admin/tenants/{slug}/clients/{clientID}
    -> Upsert client via cluster mutation o cpctx.Provider.UpsertClient
    -> en cluster: read-back cpctx.Provider.GetClient

- PUT/POST /v1/admin/tenants/{slug}/scopes  (bulk)
    -> body: { scopes: [] }
    -> loop: Provider.UpsertScope por cada scope
    -> responde {"updated": true}
   Nota: naive y sin transacción, sin validación, ignora errores (los descarta).

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) Mezcla total de responsabilidades:
   Este handler hace routing, valida, encripta, maneja fs, cluster, db, cache, keys, users.
   Es inmantenible y hace que cualquier cambio rompa otra ruta.

2) Duplicación de lógica:
   - saveLogo aparece varias veces con el mismo código.
   - Encriptación de secretos aparece 2+ veces, pero con dos mecanismos distintos:
       * secretbox.Encrypt(...) (en POST /tenants y PUT /tenants/{slug})
       * jwtx.EncryptPrivateKey + SIGNING_MASTER_KEY + base64url (en PUT /settings)
     Esto es peligrosísimo: terminás con settings en formatos distintos según endpoint usado.

3) Inconsistencias de respuesta/error:
   - Algunos endpoints devuelven JSON + 200, otros 204, otros 201.
   - Algunos usan httpx.WriteError con codes 50xx, otros print a stdout.
   - Algunos hacen read-back, otros devuelven input.

4) Operaciones pesadas en request thread:
   - POST /tenants hace migración automática y rotación de keys en el mismo request.
     Si la DB tarda, esa request queda colgada y saturás workers.
   - SyncUserFields puede ser pesado y también corre inline.

5) Type assertions para features (store “a medias”):
   Users CRUD depende de que el store tenant implemente interfaces ad-hoc.
   Esto produce: endpoints que existen pero se caen en runtime con “not_implemented”.

6) Rutas duplicadas/solapadas:
   /user-store/migrate y /migrate hacen casi lo mismo.
   /test-connection global DSN vs per-tenant test-connection.
   Esto confunde UI/CLI y aumenta superficie de bugs.

7) Seguridad / secretos / logs:
   - Usa SIGNING_MASTER_KEY para descifrar/encriptar cosas que no son “signing”.
   - A veces encripta, a veces no, a veces ignora error (en /settings).
   - Guarda logos desde data URIs: puede permitir payload grande (no hay MaxBytesReader).
   - Falta Cache-Control no-store en settings/secret flows (salvo keys rotate).
   - Falta rate-limit específico en endpoints pesados (migrate/schema).

8) Concurrencia y locks:
   - Bien: usa WithTenantMigrationLock para migraciones/ping/sync schema.
   - Mal: bulk upsert scopes no tiene lock ni control.
   - Falta consistencia: algunos flows usan lock, otros no.

Cómo lo refactorizaría a V2 (plan concreto por módulos)
-------------------------------------------------------
Objetivo: romper este “monohandler” en controllers + services + clients, con interfaces claras
y un TenantContext que resuelva tenant una sola vez (slug/id) y lo deje en context.

FASE 0 — Congelar contrato y medir
- Documentar endpoints actuales (tabla ruta/método/request/response).
- Agregar tests e2e básicos para las rutas principales (tenants CRUD, settings ETag, rotate keys, migrate, users).
- Agregar límites de tamaño de body en rutas que reciben blobs (logo, schema).

FASE 1 — Router declarativo + Controllers separados
- Crear router /admin/v2/tenants con subrouters:
    /tenants
    /tenants/{tenant}/settings
    /tenants/{tenant}/keys
    /tenants/{tenant}/users
    /tenants/{tenant}/user-store
    /tenants/{tenant}/cache
    /tenants/{tenant}/mailing
    /tenants/{tenant}/infra-stats
    /tenants/{tenant}/schema
    /tenants/{tenant}/clients
    /tenants/{tenant}/scopes

- Cada controller en su archivo:
    controllers/tenants_controller.go
    controllers/tenant_settings_controller.go
    controllers/tenant_keys_controller.go
    controllers/tenant_users_controller.go
    controllers/tenant_infra_controller.go
    controllers/tenant_clients_controller.go
    controllers/tenant_scopes_controller.go
    controllers/tenant_mailing_controller.go
    controllers/tenant_cache_controller.go

FASE 2 — Services (negocio) por dominio
- services/tenants_service.go:
    CreateTenant(ctx, req)
    UpdateTenant(ctx, tenant, req)
    DeleteTenant(ctx, tenant)
    GetTenant(ctx, tenant)
    ListTenants(ctx)

- services/tenant_settings_service.go:
    GetSettings(ctx, tenant) -> settings + raw/etag
    UpdateSettings(ctx, tenant, ifMatch, req) -> newEtag
    (acá vive la lógica ETag y validación issuerMode)

- services/tenant_keys_service.go:
    RotateKeys(ctx, tenant, grace) -> kid
    (maneja cluster vs local, invalida JWKS cache)

- services/tenant_infra_service.go:
    TestDSN(ctx, dsn)
    TestTenantDB(ctx, tenant)
    MigrateTenantDB(ctx, tenant)
    ApplySchema(ctx, tenant, schemaDef)
    InfraStats(ctx, tenant)

- services/tenant_users_service.go:
    ListUsers(ctx, tenant)
    CreateUser(ctx, tenant, req)  (incluye hashing y create identity)
    PatchUser(ctx, tenant, userID, req)
    DeleteUser(ctx, tenant, userID)

- services/tenant_clients_service.go y tenant_scopes_service.go:
    UpsertClient(ctx, tenant, clientID, input) (cluster vs provider + readback)
    BulkUpsertScopes(ctx, tenant, scopes) (con validación + manejo de errores + lock opcional)

FASE 3 — Clients/Repos: separar FS Provider, Cluster, SQL, Cache, Email, Secrets
- clients/controlplane_client.go:
    interface ControlPlaneClient { ListTenants, GetTenantBySlug, UpdateSettings, UpsertClient, UpsertScope... }

- clients/cluster_client.go:
    interface ClusterClient { Apply(ctx, Mutation) error }

- clients/tenant_sql.go:
    TenantSQLManager wrapper + Store interfaces concretas (sin type assertions sueltas).
    Ej: TenantUserRepository con métodos List/Create/Update/Delete.

- clients/tenant_cache.go, clients/email.go

- security/secret_manager.go (clave):
    Unificar en UN solo mecanismo para secretos:
      - EncryptString / DecryptString
      - (internamente usa secretbox, o KMS, pero consistente)
    Esto reemplaza el mix secretbox vs jwtx.EncryptPrivateKey.

- infra/logo_store.go:
    SaveDataURI(tenantSlug, dataURI) -> publicURL
    Validar mime, size límite, decode base64 con MaxBytesReader upstream, etc.

FASE 4 — Concurrencia “bien usada” (donde suma)
- No meter goroutines por deporte. Solo en operaciones pesadas/batch o IO externo:
  1) POST CreateTenant:
     - Persistir tenant rápido (fs/cluster).
     - En vez de migrar DB inline, ofrecer:
         a) modo sincrónico con timeout y opción ?wait=true
         b) modo async default: encolar job “migrate” + “rotate initial keys”
     - Implementar worker pool “tenant-jobs” con semáforo (ej 2-4 concurrente) para no saturar DB/FS.
  2) BulkUpsertScopes:
     - Si son muchos scopes, en vez de loop sin control:
         - validar primero
         - luego upsert secuencial (barato) o con concurrencia limitada (worker pool pequeño)
       OJO: si el provider escribe a FS, mucha concurrencia puede empeorar (lock de FS).
       Mejor: secuencial + batch en provider si existe.
  3) InfraStats:
     - Acá sí: podés ejecutar DB stats y cache stats en paralelo con goroutines + WaitGroup,
       porque son lecturas independientes.
     - Con context timeout corto (ej 1s-2s) para que no cuelgue UI.

FASE 5 — Contrato limpio y seguridad
- Unificar errores: {error, error_description, request_id} (como el resto de /admin).
- Agregar Cache-Control: no-store para settings y endpoints con secretos.
- MaxBytesReader en:
    - CreateTenant/UpdateSettings (logo data-uri)
    - Schema apply
- Rate-limit especial para:
    - migrate
    - schema apply
    - keys rotate
- Eliminar endpoints duplicados o dejarlos como alias con deprecación.

Refactor mapping (DTO/Controller/Service/Client) — para que lo puedas implementar
--------------------------------------------------------------------------------
DTOs recomendados:
- dtos/tenants.go:
    CreateTenantRequest {name, slug, settings}
    UpdateTenantRequest {name?, displayName?, settings?}
    TenantResponse {...}

- dtos/tenant_settings.go:
    UpdateSettingsRequest {settings}
    UpdateSettingsResponse {updated: true, etag: "..."}
    SettingsResponse {settings, etag}

- dtos/infra.go:
    TestConnectionRequest {dsn}
    InfraStatsResponse {db?, cache?, db_error?, cache_error?}

- dtos/users.go:
    CreateUserRequest {email, password, emailVerified, customFields, sourceClientId?}
    PatchUserRequest {sourceClientId?}
    UserResponse

Clients/Repos:
- TenantsRepository (FS provider)
- TenantsMutations (cluster)
- TenantUsersRepository (DB)
- TenantSchemaRepository (schema manager)
- SecretsManager (encrypt/decrypt)
- LogoStore (save + validate)
- JWKSCache (invalidate)
- TenantCacheClient (ping/stats)
- EmailSender (send test)

validateIssuerMode
------------------
validateIssuerMode(m) valida enum IssuerMode aceptando "" como global.
Esto debe moverse a:
  - validation/tenant.go o services/tenant_settings_service.go
para que sea una regla de negocio centralizada.



Ahora, “cómo lo refactorizaría” con metas bien claras (ultra accionable)
1) Partilo por rutas, no por “archivos”
	+ Este handler hoy tiene 7-8 subdominios. La división correcta es:
	+ Tenants CRUD: list/get/create/update/delete
	+ Tenant Settings: get/put con ETag
	+ Tenant Keys: rotate
	+ Tenant Users: list/create/patch/delete
	+ Tenant Infra: test-connection, migrate, schema/apply, infra-stats
	+ Tenant Integrations: mailing test, cache test
	+ Tenant CP Subresources: clients upsert, scopes bulk
  Cada uno en su controller+service.

2) Unificá el quilombo de secretos (esto es prioridad alta)
Hoy tenés:
 - secretbox.Encrypt() (en create/update tenant)
 - jwtx.EncryptPrivateKey() + base64url + SIGNING_MASTER_KEY (en update settings)

Eso te deja settings inconsistentes según endpoint.
En V2: un SecretsManager único:
 - EncryptString(plain) -> encString
 - DecryptString(encString) -> plain

Y que todo lo demás use eso (SMTP, DSN, Cache password).
Después, en DTOs, no aceptes PasswordEnc desde UI, solo password plano (y lo encriptás).

3) Concurrencia donde suma (sin cagar el FS)

	+ InfraStats: paralelizable (DB stats y cache stats en goroutines).
	+ CreateTenant: migración + rotate keys puede ser job async con worker pool y semáforo.
	+ Bulk scopes: yo lo dejo secuencial (FS writes), pero con validación y errores. Si querés paralelo, limitadísimo (2-4) y solo si el provider no hace lock global.

4) Sacá “users” de acá cuanto antes
-----------------------------------
/tenants/{slug}/users no pertenece a “control plane FS”. Pertenece a “User Store admin”.
Yo haría:
	+ AdminTenantUsersController que usa TenantSQLManager y un TenantUsersRepo fijo (sin type assertions).

5) Cerrá duplicados

Elegí uno:

	+ o /user-store/migrate
	+ o /{slug}/migrate
y el otro queda como alias que llama al mismo service (marcado deprecated).

*/

package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	clusterv1 "github.com/dropDatabas3/hellojohn/internal/cluster/v1"
	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
	cpfs "github.com/dropDatabas3/hellojohn/internal/controlplane/v1/fs"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/infra/v1/tenantcache"
	"github.com/dropDatabas3/hellojohn/internal/infra/v1/tenantsql"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	"github.com/dropDatabas3/hellojohn/internal/security/secretbox"
	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	slugRegex = regexp.MustCompile(`^[a-z0-9\-]+$`)
)

// NewAdminTenantsFSHandler expone endpoints básicos para tenants:
//
//	GET    /v1/admin/tenants
//	POST   /v1/admin/tenants
//	GET    /v1/admin/tenants/{slug}
//	GET    /v1/admin/tenants/{slug}/settings
//	PUT    /v1/admin/tenants/{slug}/settings
func NewAdminTenantsFSHandler(c *app.Container) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		base := "/v1/admin/tenants"

		// Helper para generar ETag desde bytes
		etag := func(data []byte) string {
			h := sha256.Sum256(data)
			return `"` + hex.EncodeToString(h[:8]) + `"`
		}

		// Validar slug
		validSlug := func(slug string) bool {
			return len(slug) > 0 && len(slug) <= 32 && slugRegex.MatchString(slug)
		}

		// Obtener FSProvider
		fsProvider, ok := controlplane.AsFSProvider(cpctx.Provider)
		if !ok {
			httpx.WriteError(w, http.StatusNotImplemented, "fs_provider_required", "solo FS provider soportado", 5001)
			return
		}

		switch {
		// POST /v1/admin/tenants/test-connection
		case path == base+"/test-connection" && r.Method == http.MethodPost:
			var req struct {
				DSN string `json:"dsn"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5090)
				return
			}
			if strings.TrimSpace(req.DSN) == "" {
				httpx.WriteError(w, http.StatusBadRequest, "missing_dsn", "DSN requerido", 5091)
				return
			}

			// Create transient connection with short timeout
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			pool, err := pgxpool.New(ctx, req.DSN)
			if err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "connect_error", fmt.Sprintf("Error parseando DSN o conectando: %v", err), 5092)
				return
			}
			defer pool.Close()

			if err := pool.Ping(ctx); err != nil {
				httpx.WriteError(w, http.StatusBadRequest, "ping_error", fmt.Sprintf("No se pudo establecer conexión: %v", err), 5093)
				return
			}

			httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return

		case path == base:
			switch r.Method {
			case http.MethodGet:
				// Listar todos los tenants
				tenants, err := fsProvider.ListTenants(r.Context())
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "list_failed", err.Error(), 5002)
					return
				}
				httpx.WriteJSON(w, http.StatusOK, tenants)
				return

			case http.MethodPost:
				// Crear nuevo tenant
				var req struct {
					Name     string                      `json:"name"`
					Slug     string                      `json:"slug"`
					Settings controlplane.TenantSettings `json:"settings"`
				}
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					fmt.Printf("JSON Decode Error: %v\n", err)
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5003)
					return
				}

				// Leader enforcement moved to middleware

				req.Name = strings.TrimSpace(req.Name)
				req.Slug = strings.TrimSpace(req.Slug)

				if req.Name == "" || req.Slug == "" {
					httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "name y slug son requeridos", 5004)
					return
				}

				if !validSlug(req.Slug) {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_slug", "slug debe ser [a-z0-9\\-] y <=32 chars", 5005)
					return
				}

				// Helper to save logo
				saveLogo := func(slug string, logoData string) (string, error) {
					if !strings.HasPrefix(logoData, "data:image/") {
						return logoData, nil
					}
					parts := strings.Split(logoData, ",")
					if len(parts) != 2 {
						return "", fmt.Errorf("invalid data URI format")
					}
					mimeType := strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")
					ext := ""
					switch mimeType {
					case "image/png":
						ext = ".png"
					case "image/jpeg":
						ext = ".jpg"
					case "image/svg+xml":
						ext = ".svg"
					case "image/gif":
						ext = ".gif"
					case "image/webp":
						ext = ".webp"
					default:
						return "", fmt.Errorf("unsupported image type: %s", mimeType)
					}

					data, err := base64.StdEncoding.DecodeString(parts[1])
					if err != nil {
						return "", fmt.Errorf("invalid base64: %w", err)
					}

					// Ensure directory exists
					targetDir := filepath.Join(fsProvider.FSRoot(), "tenants", slug)
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						return "", err
					}

					fileName := "logo" + ext
					filePath := filepath.Join(targetDir, fileName)
					if err := os.WriteFile(filePath, data, 0644); err != nil {
						return "", err
					}

					// Return relative URL pattern (assumed to be served by some asset handler)
					// If using standard static file serving, this path needs to match that router.
					// For now, we return a predictable path.
					return fmt.Sprintf("/v1/assets/tenants/%s/%s", slug, fileName), nil
				}

				// Encrypt sensitive fields using secretbox (matching manager.go expectations)
				// SMTP Password
				if req.Settings.SMTP != nil && req.Settings.SMTP.Password != "" {
					if enc, err := secretbox.Encrypt(req.Settings.SMTP.Password); err == nil {
						req.Settings.SMTP.PasswordEnc = enc
						req.Settings.SMTP.Password = "" // Clear plain
					} else {
						httpx.WriteError(w, http.StatusInternalServerError, "encrypt_error", "failed to encrypt smtp password", 5008)
						return
					}
				}
				// UserDB DSN
				if req.Settings.UserDB != nil && req.Settings.UserDB.DSN != "" {
					if enc, err := secretbox.Encrypt(req.Settings.UserDB.DSN); err == nil {
						req.Settings.UserDB.DSNEnc = enc
						req.Settings.UserDB.DSN = "" // Clear plain
					} else {
						httpx.WriteError(w, http.StatusInternalServerError, "encrypt_error", "failed to encrypt dsn", 5008)
						return
					}
				}
				// Cache Password
				if req.Settings.Cache != nil && req.Settings.Cache.Password != "" {
					if enc, err := secretbox.Encrypt(req.Settings.Cache.Password); err == nil {
						req.Settings.Cache.PassEnc = enc
						req.Settings.Cache.Password = "" // Clear plain
					} else {
						httpx.WriteError(w, http.StatusInternalServerError, "encrypt_error", "failed to encrypt cache password", 5008)
						return
					}
				}

				// Process Logo
				if req.Settings.LogoURL != "" {
					if url, err := saveLogo(req.Slug, req.Settings.LogoURL); err == nil {
						req.Settings.LogoURL = url
					} else {
						// Log error but continue with empty logo or original data?
						// For now, let's fail to warn user
						httpx.WriteError(w, http.StatusBadRequest, "invalid_logo", err.Error(), 5007)
						return
					}
				}

				// Crear tenant via Raft si existe cluster, sino directo
				if c != nil && c.ClusterNode != nil {
					payload, _ := json.Marshal(clusterv1.UpsertTenantDTO{ID: req.Slug, Name: req.Name, Slug: req.Slug, Settings: req.Settings})
					m := clusterv1.Mutation{Type: clusterv1.MutationUpsertTenant, TenantSlug: req.Slug, TsUnix: time.Now().Unix(), Payload: payload}
					if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
						httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
						return
					}
					// read back
					if t, err := fsProvider.GetTenantBySlug(r.Context(), req.Slug); err == nil {
						httpx.WriteJSON(w, http.StatusCreated, t)
						return
					}
				}
				now := time.Now().UTC()
				tenant := &controlplane.Tenant{ID: req.Slug, Name: req.Name, Slug: req.Slug, CreatedAt: now, UpdatedAt: now, Settings: req.Settings}
				if err := fsProvider.UpsertTenant(r.Context(), tenant); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "create_failed", err.Error(), 5006)
					return
				}

				// Automatic Migration
				if c.TenantSQLManager != nil && req.Settings.UserDB != nil {
					// Only attempt if we have a DSN (encrypted or plain)
					if req.Settings.UserDB.DSNEnc != "" || req.Settings.UserDB.DSN != "" {
						_, err := c.TenantSQLManager.MigrateTenant(r.Context(), req.Slug)
						if err != nil {
							// Rollback: Delete tenant
							_ = fsProvider.DeleteTenant(r.Context(), req.Slug)
							// Return friendly error
							msg := fmt.Sprintf("Migration failed: %v", err)
							if tenantsql.IsNoDBForTenant(err) {
								msg = "Database connection failed (check DSN)"
							}
							httpx.WriteError(w, http.StatusBadRequest, "migration_failed", msg, 5009)
							return
						}
					}
				}

				// Generate initial keys for the new tenant
				if c != nil && c.Issuer != nil {
					if _, err := c.Issuer.Keys.RotateFor(req.Slug, 0); err != nil {
						// Log warning but don't fail the request as tenant is created
						// Keys will be generated on first use or next rotation
						fmt.Printf("WARN: failed to generate initial keys for tenant %s: %v\n", req.Slug, err)
					}
				}

				httpx.WriteJSON(w, http.StatusCreated, tenant)
				return

			default:
				httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 5000)
				return
			}

		case strings.HasPrefix(path, base+"/"):
			// /v1/admin/tenants/{slug}[/settings]
			rest := strings.TrimPrefix(path, base+"/")
			parts := strings.Split(strings.Trim(rest, "/"), "/")
			if len(parts) == 0 || parts[0] == "" {
				httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5010)
				return
			}

			slug := parts[0]

			// Support lookup by ID: if slug looks like UUID, try to resolve it
			if _, err := uuid.Parse(slug); err == nil {
				if t, err := fsProvider.GetTenantByID(r.Context(), slug); err == nil {
					slug = t.Slug
				}
				// If not found by ID, we continue assuming it might be a weird slug (unlikely but safe)
				// or it will fail later with "tenant not found"
			}

			// Helper to save logo (SAME AS ABOVE - duplicated for now to avoid large refactor of handler struct)
			// Ideally this should be a shared function, but for this edit we inline it in the closure or extract it.
			// Given I just added it in the POST handle, I can't access it here easily without moving it up.
			// I will move it up to the handler scope in the next edit or repeat it.
			// To be clean, I will define `saveLogo` at the top of the handler function.
			// Since I am already restricted in the `ReplaceFileContent` to a block, I will assume I can't move it easily without replacing the whole func.
			// I will duplicate it for now in the PUT block or I should have defined it earlier.
			// I made a mistake in the previous tool call by defining it inside the POST block.
			// I will correct this in the instruction by replacing the whole function body or defining it twice.
			// Defining it twice is safer for now to avoid context limit issues with large replaces, though it's dry-violation.

			// POST /v1/admin/tenants/{slug}/keys/rotate
			if len(parts) == 3 && parts[1] == "keys" && parts[2] == "rotate" {
				if r.Method != http.MethodPost {
					httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5060)
					return
				}
				// Verificar tenant existe (404 si no)
				if _, err := fsProvider.GetTenantBySlug(r.Context(), slug); err != nil {
					if err == cpfs.ErrTenantNotFound {
						httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
						return
					}
					httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
					return
				}

				// Leer graceSeconds: query param tiene prioridad, luego ENV KEY_ROTATION_GRACE_SECONDS, default 60
				var grace int64 = 60
				if env := strings.TrimSpace(os.Getenv("KEY_ROTATION_GRACE_SECONDS")); env != "" {
					if v, err := strconv.ParseInt(env, 10, 64); err == nil && v >= 0 {
						grace = v
					}
				}
				if qs := r.URL.Query().Get("graceSeconds"); qs != "" {
					if v, err := strconv.ParseInt(qs, 10, 64); err == nil && v >= 0 {
						grace = v
					}
				}

				// Rotación replicada: en líder, rotar e incluir material exacto para seguidores
				if c != nil && c.ClusterNode != nil {
					// Perform rotation on leader using keystore, then read active/retiring JSON files to include in mutation
					sk, err := c.Issuer.Keys.RotateFor(slug, grace)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 5061)
						return
					}
					// Resolve FS root via provider
					fsProv, ok := controlplane.AsFSProvider(cpctx.Provider)
					if !ok {
						httpx.WriteError(w, http.StatusInternalServerError, "server_error", "fs provider required for rotation", 5062)
						return
					}
					keysDir := filepath.Join(fsProv.FSRoot(), "keys", slug)
					// Read file contents as-is
					actBytes, aerr := os.ReadFile(filepath.Join(keysDir, "active.json"))
					if aerr != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "server_error", aerr.Error(), 5063)
						return
					}
					var retBytes []byte
					if b, rerr := os.ReadFile(filepath.Join(keysDir, "retiring.json")); rerr == nil {
						retBytes = b
					}
					dto := clusterv1.RotateTenantKeyDTO{
						ActiveJSON:   string(actBytes),
						RetiringJSON: string(retBytes),
						GraceSeconds: grace,
					}
					payload, _ := json.Marshal(dto)
					m := clusterv1.Mutation{Type: clusterv1.MutationRotateTenantKey, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
					if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
						httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
						return
					}
					// Invalidate JWKS locally (followers will do so during Apply)
					if c.JWKSCache != nil {
						c.JWKSCache.Invalidate(slug)
					}
					setNoStore(w)
					httpx.WriteJSON(w, http.StatusOK, map[string]any{"kid": sk.ID})
					return
				}
				// Fallback (no cluster): local rotate only
				if sk, err := c.Issuer.Keys.RotateFor(slug, grace); err == nil {
					if c.JWKSCache != nil {
						c.JWKSCache.Invalidate(slug)
					}
					setNoStore(w)
					httpx.WriteJSON(w, http.StatusOK, map[string]any{"kid": sk.ID})
					return
				} else {
					httpx.WriteError(w, http.StatusInternalServerError, "server_error", err.Error(), 5061)
					return
				}
			}

			// PUT /v1/admin/tenants/{slug} -> Upsert tenant (id, slug, settings)
			if len(parts) == 1 && r.Method == http.MethodPut {
				var in struct {
					ID          string                      `json:"id"`
					Slug        string                      `json:"slug"`
					Name        string                      `json:"name"`
					DisplayName string                      `json:"display_name"`
					Settings    controlplane.TenantSettings `json:"settings"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5030)
					return
				}
				// Leader enforcement moved to middleware
				if strings.TrimSpace(in.Slug) == "" {
					in.Slug = slug
				}
				if !validSlug(in.Slug) || in.Slug != slug {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_slug", "slug inválido o mismatch con path", 5031)
					return
				}
				// Validar issuerMode (permite vacío como "global").
				if err := validateIssuerMode(in.Settings.IssuerMode); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_issuer_mode", err.Error(), 5033)
					return
				}

				// Handle Logo Update
				// We need the saveLogo func again.
				saveLogo := func(slug string, logoData string) (string, error) {
					if !strings.HasPrefix(logoData, "data:image/") {
						return logoData, nil
					}
					parts := strings.Split(logoData, ",")
					if len(parts) != 2 {
						return "", fmt.Errorf("invalid data URI format")
					}
					mimeType := strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")
					ext := ""
					switch mimeType {
					case "image/png":
						ext = ".png"
					case "image/jpeg":
						ext = ".jpg"
					case "image/svg+xml":
						ext = ".svg"
					case "image/gif":
						ext = ".gif"
					case "image/webp":
						ext = ".webp"
					default:
						return "", fmt.Errorf("unsupported image type: %s", mimeType)
					}

					data, err := base64.StdEncoding.DecodeString(parts[1])
					if err != nil {
						return "", fmt.Errorf("invalid base64: %w", err)
					}

					targetDir := filepath.Join(fsProvider.FSRoot(), "tenants", slug)
					if err := os.MkdirAll(targetDir, 0755); err != nil {
						return "", err
					}

					fileName := "logo" + ext
					filePath := filepath.Join(targetDir, fileName)
					if err := os.WriteFile(filePath, data, 0644); err != nil {
						return "", err
					}
					return fmt.Sprintf("/v1/assets/tenants/%s/%s", slug, fileName), nil
				}

				if in.Settings.LogoURL != "" {
					if url, err := saveLogo(in.Slug, in.Settings.LogoURL); err == nil {
						in.Settings.LogoURL = url
					} else {
						httpx.WriteError(w, http.StatusBadRequest, "invalid_logo", err.Error(), 5007)
						return
					}
				}

				// Encrypt sensitive fields using secretbox (Update flow)
				// SMTP Password
				if in.Settings.SMTP != nil && in.Settings.SMTP.Password != "" {
					if enc, err := secretbox.Encrypt(in.Settings.SMTP.Password); err == nil {
						in.Settings.SMTP.PasswordEnc = enc
						in.Settings.SMTP.Password = "" // Clear plain
					} else {
						httpx.WriteError(w, http.StatusInternalServerError, "encrypt_error", "failed to encrypt smtp password", 5008)
						return
					}
				}
				// UserDB DSN
				if in.Settings.UserDB != nil && in.Settings.UserDB.DSN != "" {
					if enc, err := secretbox.Encrypt(in.Settings.UserDB.DSN); err == nil {
						in.Settings.UserDB.DSNEnc = enc
						in.Settings.UserDB.DSN = "" // Clear plain
					} else {
						httpx.WriteError(w, http.StatusInternalServerError, "encrypt_error", "failed to encrypt dsn", 5008)
						return
					}
				}
				// Cache Password
				if in.Settings.Cache != nil && in.Settings.Cache.Password != "" {
					if enc, err := secretbox.Encrypt(in.Settings.Cache.Password); err == nil {
						in.Settings.Cache.PassEnc = enc
						in.Settings.Cache.Password = "" // Clear plain
					} else {
						httpx.WriteError(w, http.StatusInternalServerError, "encrypt_error", "failed to encrypt cache password", 5008)
						return
					}
				}

				if c != nil && c.ClusterNode != nil {
					payload, _ := json.Marshal(clusterv1.UpsertTenantDTO{ID: strings.TrimSpace(in.ID), Name: strings.TrimSpace(in.Name), Slug: in.Slug, Settings: in.Settings})
					m := clusterv1.Mutation{Type: clusterv1.MutationUpsertTenant, TenantSlug: in.Slug, TsUnix: time.Now().Unix(), Payload: payload}
					if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
						httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
						return
					}
					if t, err := fsProvider.GetTenantBySlug(r.Context(), in.Slug); err == nil {
						httpx.WriteJSON(w, http.StatusOK, t)
						return
					}
				}
				now := time.Now().UTC()
				t := &controlplane.Tenant{
					ID:          strings.TrimSpace(in.ID),
					Name:        strings.TrimSpace(in.Name),
					DisplayName: strings.TrimSpace(in.DisplayName),
					Slug:        in.Slug,
					CreatedAt:   now,
					UpdatedAt:   now,
					Settings:    in.Settings,
				}
				if strings.TrimSpace(t.ID) == "" {
					// Try to preserve existing ID
					if existing, err := fsProvider.GetTenantBySlug(r.Context(), in.Slug); err == nil {
						t.ID = existing.ID
						t.CreatedAt = existing.CreatedAt // Preserve creation time too
					} else {
						// If new, leave empty to let ensureTenantID generate UUID
						// t.ID = in.Slug // REMOVED: Do not default to slug as it triggers regeneration if not UUID
					}
				}
				// Update settings via provider
				if err := fsProvider.UpdateTenantSettings(r.Context(), slug, &in.Settings); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "update_failed", err.Error(), 5032)
					return
				}

				// FIX: La comparación anterior fallaba por slices.
				// Asumimos que si llegamos aquí es un upsert completo, así que settings se actualizan.
				// Además, si hay UserFields (incluso vacío si se borraron todos), sincronizamos el esquema.
				if in.Settings.UserFields != nil {
					// Obtener store para sincronizar
					if c.TenantSQLManager != nil {
						if store, err := c.TenantSQLManager.GetPG(r.Context(), slug); err == nil {
							// Usar schema manager
							sm := tenantsql.NewSchemaManager(store.Pool())
							// Obtener tenantID real si es posible, sino slug
							tid := t.ID
							if tid == "" {
								tid = slug
							}

							// Ejecutar bajo lock de migración para evitar conflictos
							_ = tenantsql.WithTenantMigrationLock(r.Context(), store.Pool(), tid, 10*time.Second, func(ctx context.Context) error {
								return sm.SyncUserFields(ctx, tid, in.Settings.UserFields)
							})
						}
					}
				}

				httpx.WriteJSON(w, http.StatusOK, t)
				return
			}

			// DELETE /v1/admin/tenants/{slug} -> Delete tenant
			if len(parts) == 1 && r.Method == http.MethodDelete {
				// Leader enforcement moved to middleware
				if c != nil && c.ClusterNode != nil {
					m := clusterv1.Mutation{Type: clusterv1.MutationDeleteTenant, TenantSlug: slug, TsUnix: time.Now().Unix()}
					if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
						httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
						return
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
				if err := fsProvider.DeleteTenant(r.Context(), slug); err != nil {
					if err == cpfs.ErrTenantNotFound {
						httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
						return
					}
					httpx.WriteError(w, http.StatusInternalServerError, "delete_failed", err.Error(), 5035)
					return
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}

			// ─── Admin: per-tenant users ───
			// GET  /v1/admin/tenants/{slug}/users
			// POST /v1/admin/tenants/{slug}/users
			if len(parts) == 2 && parts[1] == "users" {
				if c.TenantSQLManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "sql_manager_required", "SQL Manager no inicializado", 5080)
					return
				}
				// Obtener store del tenant
				store, err := c.TenantSQLManager.GetPG(r.Context(), slug)
				if err != nil {
					if tenantsql.IsNoDBForTenant(err) {
						httpx.WriteTenantDBMissing(w)
						return
					}
					httpx.WriteTenantDBError(w, err.Error())
					return
				}

				// GET: Listar usuarios
				if r.Method == http.MethodGet {
					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "get_tenant_failed", err.Error(), 5081)
						return
					}

					// Usar interface casting para acceder a ListUsers
					type userLister interface {
						ListUsers(ctx context.Context, tenantID string) ([]core.User, error)
					}

					if lister, ok := any(store).(userLister); ok {
						users, err := lister.ListUsers(r.Context(), t.ID)
						if err != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "list_users_failed", err.Error(), 5082)
							return
						}
						httpx.WriteJSON(w, http.StatusOK, users)
						return
					}
					httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "store no soporta listar usuarios", 5083)
					return
				}

				// POST: Crear usuario
				if r.Method == http.MethodPost {
					var req struct {
						Email          string         `json:"email"`
						Password       string         `json:"password"`
						EmailVerified  bool           `json:"email_verified"`
						CustomFields   map[string]any `json:"custom_fields"`
						SourceClientID *string        `json:"source_client_id"`
					}
					if !httpx.ReadJSON(w, r, &req) {
						return
					}
					req.Email = strings.TrimSpace(strings.ToLower(req.Email))
					if req.Email == "" || req.Password == "" {
						httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "email y password requeridos", 5084)
						return
					}

					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "get_tenant_failed", err.Error(), 5081)
						return
					}

					phc, err := password.Hash(password.Default, req.Password)
					if err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "hash_failed", "no se pudo hashear el password", 5085)
						return
					}

					u := &core.User{
						TenantID:       t.ID,
						Email:          req.Email,
						EmailVerified:  req.EmailVerified,
						Metadata:       map[string]any{},
						CustomFields:   req.CustomFields,
						SourceClientID: req.SourceClientID,
					}

					if err := store.CreateUser(r.Context(), u); err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "create_user_failed", err.Error(), 5086)
						return
					}

					// Crear identidad password
					if err := store.CreatePasswordIdentity(r.Context(), u.ID, req.Email, req.EmailVerified, phc); err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "create_identity_failed", err.Error(), 5087)
						return
					}

					httpx.WriteJSON(w, http.StatusCreated, u)
					return
				}

				httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/POST", 5088)
				return
			}

			// DELETE /v1/admin/tenants/{slug}/users/{id}
			// PATCH  /v1/admin/tenants/{slug}/users/{id}
			if len(parts) == 3 && parts[1] == "users" {
				userID := parts[2]

				// PATCH: Update user
				if r.Method == http.MethodPatch {
					if c.TenantSQLManager == nil {
						httpx.WriteError(w, http.StatusNotImplemented, "sql_manager_required", "SQL Manager no inicializado", 5080)
						return
					}
					store, err := c.TenantSQLManager.GetPG(r.Context(), slug)
					if err != nil {
						if tenantsql.IsNoDBForTenant(err) {
							httpx.WriteTenantDBMissing(w)
							return
						}
						httpx.WriteTenantDBError(w, err.Error())
						return
					}

					var req map[string]any
					if !httpx.ReadJSON(w, r, &req) {
						return
					}

					// Validate allowed fields for update
					updates := make(map[string]any)
					// Helper to treat null/empty
					if v, ok := req["source_client_id"]; ok {
						if s, ok := v.(string); ok && (s == "" || s == "_none") {
							updates["source_client_id"] = nil // Set to NULL in DB
						} else {
							updates["source_client_id"] = v
						}
					}

					if len(updates) == 0 {
						httpx.WriteError(w, http.StatusBadRequest, "no_updates", "no se enviaron campos válidos para actualizar", 5090)
						return
					}

					type userUpdater interface {
						UpdateUser(ctx context.Context, userID string, updates map[string]any) error
					}

					if updater, ok := any(store).(userUpdater); ok {
						if err := updater.UpdateUser(r.Context(), userID, updates); err != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "update_user_failed", err.Error(), 5091)
							return
						}

						// Return updated user?? Or just NoContent. Let's return the user for UI convenience if possible,
						// but simpler to return NoContent or minimal confirmation.
						// Fetching user again to return it is polite.
						if lister, ok := any(store).(interface {
							GetUserByID(ctx context.Context, id string) (*core.User, error)
						}); ok {
							u, _ := lister.GetUserByID(r.Context(), userID)
							if u != nil {
								httpx.WriteJSON(w, http.StatusOK, u)
								return
							}
						}

						w.WriteHeader(http.StatusOK)
						return
					}
					httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "store no soporta actualizar usuarios", 5083)
					return
				}

				if r.Method == http.MethodDelete {
					if c.TenantSQLManager == nil {
						httpx.WriteError(w, http.StatusNotImplemented, "sql_manager_required", "SQL Manager no inicializado", 5080)
						return
					}
					// Obtener store del tenant (necesario inicializarlo aqui porque el bloque anterior era if len==2)
					store, err := c.TenantSQLManager.GetPG(r.Context(), slug)
					if err != nil {
						if tenantsql.IsNoDBForTenant(err) {
							httpx.WriteTenantDBMissing(w)
							return
						}
						httpx.WriteTenantDBError(w, err.Error())
						return
					}

					// Usar interface casting para DeleteUser
					type userDeleter interface {
						DeleteUser(ctx context.Context, userID string) error
					}
					if deleter, ok := any(store).(userDeleter); ok {
						if err := deleter.DeleteUser(r.Context(), userID); err != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "delete_user_failed", err.Error(), 5089)
							return
						}
						w.WriteHeader(http.StatusNoContent)
						return
					}
					httpx.WriteError(w, http.StatusNotImplemented, "not_implemented", "store no soporta eliminar usuarios", 5083)
					return
				}
			}

			// ─── Admin: per-tenant user-store ───
			// POST /v1/admin/tenants/{slug}/user-store/test-connection
			// POST /v1/admin/tenants/{slug}/user-store/migrate
			if len(parts) >= 2 && parts[1] == "user-store" {
				if c.TenantSQLManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "tenant_sql_manager_required", "TenantSQLManager no inicializado", 5025)
					return
				}

				if len(parts) == 3 && parts[2] == "test-connection" {
					if r.Method != http.MethodPost {
						httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5026)
						return
					}
					// First, verify tenant exists in FS; admin over nonexistent tenant -> 404
					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
						return
					}
					store, err := c.TenantSQLManager.GetPG(r.Context(), slug)
					if err != nil {
						if tenantsql.IsNoDBForTenant(err) {
							httpx.WriteTenantDBMissing(w)
							return
						}
						httpx.WriteTenantDBError(w, err.Error())
						return
					}
					// Ejecutar ping bajo el lock (misma conexión)
					if err := tenantsql.WithTenantMigrationLock(r.Context(), store.Pool(), t.ID, 5*time.Second, func(ctx context.Context) error {
						return store.Ping(ctx)
					}); err != nil {
						httpx.WriteTenantDBError(w, err.Error())
						return
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}

				if len(parts) == 3 && parts[2] == "migrate" {
					if r.Method != http.MethodPost {
						httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5027)
						return
					}
					// Verify tenant exists (404 if not)
					_, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
						return
					}

					// Usar MigrateTenant del manager que ya tiene configurado el path correcto y maneja el lock
					count, err := c.TenantSQLManager.MigrateTenant(r.Context(), slug)
					if err != nil {
						if tenantsql.IsNoDBForTenant(err) {
							httpx.WriteTenantDBMissing(w)
							return
						}
						// Si fue timeout/contención, devolver 409 con Retry-After
						if errors.Is(err, context.DeadlineExceeded) {
							w.Header().Set("Retry-After", "5")
							httpx.WriteError(w, http.StatusConflict, "conflict", "migration lock busy, retry later", 2605)
							return
						}
						httpx.WriteTenantDBError(w, err.Error())
						return
					}

					// Log success (optional, manager already logs)
					_ = count
					w.WriteHeader(http.StatusNoContent)
					return
				}

				httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5028)
				return
			}

			// ─── Admin: per-tenant mailing ───
			// POST /v1/admin/tenants/{slug}/mailing/test
			if len(parts) == 3 && parts[1] == "mailing" && parts[2] == "test" {
				// Verify tenant exists
				t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
				if err != nil {
					if err == cpfs.ErrTenantNotFound {
						httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
						return
					}
					httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
					return
				}
				SendTestEmail(w, r, t)
				return
			}

			// ─── Admin: per-tenant cache ───
			// POST /v1/admin/tenants/{slug}/cache/test-connection
			if len(parts) >= 2 && parts[1] == "cache" {
				if c.TenantCacheManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "tenant_cache_manager_required", "TenantCacheManager no inicializado", 5070)
					return
				}

				if len(parts) == 3 && parts[2] == "test-connection" {
					if r.Method != http.MethodPost {
						httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5071)
						return
					}
					// Verify tenant exists
					t, err := fsProvider.GetTenantBySlug(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
						return
					}

					// Get cache client (creates connection if needed)
					client, err := c.TenantCacheManager.Get(r.Context(), t.Slug)
					if err != nil {
						if err == tenantcache.ErrNoCacheForTenant {
							// If no cache configured, we can consider it "success" if using default memory,
							// or error if explicit test requested. For now, let's return 404-like or specific error.
							// Better: return 400 saying no cache config.
							httpx.WriteError(w, http.StatusBadRequest, "no_cache_config", "no cache configured for tenant", 5072)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5073)
						return
					}
					// Ping
					if err := client.Ping(r.Context()); err != nil {
						httpx.WriteError(w, http.StatusBadGateway, "ping_failed", "no se pudo conectar a redis: "+err.Error(), 5074)
						return
					}
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}

			// GET /v1/admin/tenants/{slug}/infra-stats
			if len(parts) == 2 && parts[1] == "infra-stats" && r.Method == http.MethodGet {
				stats := make(map[string]any)

				// DB Stats
				if c.TenantSQLManager != nil {
					if dbStats, err := c.TenantSQLManager.GetStats(r.Context(), slug); err == nil {
						stats["db"] = dbStats
					} else {
						stats["db_error"] = err.Error()
					}
				}

				// Cache Stats
				if c.TenantCacheManager != nil {
					if client, err := c.TenantCacheManager.Get(r.Context(), slug); err == nil {
						if cacheStats, err := client.Stats(r.Context()); err == nil {
							stats["cache"] = cacheStats
						} else {
							stats["cache_error"] = err.Error()
						}
					} else {
						// Ignore error if just not configured, but report if other error
						if err != tenantcache.ErrNoCacheForTenant {
							stats["cache_error"] = err.Error()
						}
					}
				}

				httpx.WriteJSON(w, http.StatusOK, stats)
				return
			}

			// POST /v1/admin/tenants/{slug}/migrate
			if len(parts) == 2 && parts[1] == "migrate" && r.Method == http.MethodPost {
				if c.TenantSQLManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "sql_manager_required", "SQL Manager no inicializado", 5075)
					return
				}

				count, err := c.TenantSQLManager.MigrateTenant(r.Context(), slug)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "migrate_failed", err.Error(), 5076)
					return
				}

				httpx.WriteJSON(w, http.StatusOK, map[string]any{
					"status":  "success",
					"applied": count,
				})
				return
			}

			// POST /v1/admin/tenants/{slug}/schema/apply
			if len(parts) == 3 && parts[1] == "schema" && parts[2] == "apply" && r.Method == http.MethodPost {
				if c.TenantSQLManager == nil {
					httpx.WriteError(w, http.StatusNotImplemented, "sql_manager_required", "SQL Manager no inicializado", 5077)
					return
				}

				// Parse schema definition from body
				var schemaDef map[string]any
				if err := json.NewDecoder(r.Body).Decode(&schemaDef); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON body", 5078)
					return
				}

				// Get store to reuse connection pool
				store, err := c.TenantSQLManager.GetPG(r.Context(), slug)
				if err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "db_connection_failed", err.Error(), 5079)
					return
				}

				// Initialize Schema Manager (Postgres implementation)
				// In a real multi-driver scenario, we would get the correct manager from a factory.
				schemaMgr := tenantsql.NewSchemaManager(store.Pool())

				// Apply Indexes
				if err := schemaMgr.EnsureIndexes(r.Context(), slug, schemaDef); err != nil {
					httpx.WriteError(w, http.StatusInternalServerError, "schema_apply_failed", err.Error(), 5080)
					return
				}

				httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "applied"})
				return
			}

			if len(parts) == 1 && r.Method == http.MethodGet {
				// GET /v1/admin/tenants/{slug}
				tenant, raw, err := fsProvider.GetTenantRaw(r.Context(), slug)
				if err != nil {
					if err == cpfs.ErrTenantNotFound {
						httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5011)
						return
					}
					httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5012)
					return
				}

				w.Header().Set("ETag", etag(raw))
				httpx.WriteJSON(w, http.StatusOK, tenant)
				return
			}

			if len(parts) == 2 && parts[1] == "settings" {
				switch r.Method {
				case http.MethodGet:
					// GET /v1/admin/tenants/{slug}/settings
					settings, raw, err := fsProvider.GetTenantSettingsRaw(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5013)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5014)
						return
					}

					w.Header().Set("ETag", etag(raw))
					httpx.WriteJSON(w, http.StatusOK, settings)
					return

				case http.MethodPut:
					// PUT /v1/admin/tenants/{slug}/settings (con If-Match)
					// Leader enforcement moved to middleware
					ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
					if ifMatch == "" {
						httpx.WriteError(w, http.StatusPreconditionRequired, "if_match_required", "If-Match header requerido", 5015)
						return
					}

					// Verificar ETag actual
					_, raw, err := fsProvider.GetTenantSettingsRaw(r.Context(), slug)
					if err != nil {
						if err == cpfs.ErrTenantNotFound {
							httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 5016)
							return
						}
						httpx.WriteError(w, http.StatusInternalServerError, "get_failed", err.Error(), 5017)
						return
					}

					currentETag := etag(raw)
					if currentETag != ifMatch {
						httpx.WriteError(w, http.StatusPreconditionFailed, "etag_mismatch", "la versión cambió, recarga y reintenta", 5018)
						return
					}

					var newSettings controlplane.TenantSettings
					if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5019)
						return
					}

					// Encrypt sensitive fields
					masterKey := os.Getenv("SIGNING_MASTER_KEY")
					if masterKey != "" {
						// SMTP Password
						if newSettings.SMTP != nil && newSettings.SMTP.Password != "" {
							if enc, err := jwtx.EncryptPrivateKey([]byte(newSettings.SMTP.Password), masterKey); err == nil {
								newSettings.SMTP.PasswordEnc = jwtx.EncodeBase64URL(enc)
								newSettings.SMTP.Password = "" // Clear plain
							}
						}
						// UserDB DSN
						if newSettings.UserDB != nil && newSettings.UserDB.DSN != "" {
							if enc, err := jwtx.EncryptPrivateKey([]byte(newSettings.UserDB.DSN), masterKey); err == nil {
								newSettings.UserDB.DSNEnc = jwtx.EncodeBase64URL(enc)
								newSettings.UserDB.DSN = "" // Clear plain
							}
						}
						// Cache Password
						if newSettings.Cache != nil && newSettings.Cache.Password != "" {
							if enc, err := jwtx.EncryptPrivateKey([]byte(newSettings.Cache.Password), masterKey); err == nil {
								newSettings.Cache.PassEnc = jwtx.EncodeBase64URL(enc)
								newSettings.Cache.Password = "" // Clear plain
							}
						}
					}

					if err := validateIssuerMode(newSettings.IssuerMode); err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "invalid_issuer_mode", err.Error(), 5033)
						return
					}

					// Helper to save logo (local scope)
					saveLogo := func(slug string, logoData string) (string, error) {
						if !strings.HasPrefix(logoData, "data:image/") {
							return logoData, nil
						}
						parts := strings.Split(logoData, ",")
						if len(parts) != 2 {
							return "", fmt.Errorf("invalid data URI format")
						}
						mimeType := strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")
						ext := ""
						switch mimeType {
						case "image/png":
							ext = ".png"
						case "image/jpeg":
							ext = ".jpg"
						case "image/svg+xml":
							ext = ".svg"
						case "image/gif":
							ext = ".gif"
						case "image/webp":
							ext = ".webp"
						default:
							return "", fmt.Errorf("unsupported image type: %s", mimeType)
						}

						data, err := base64.StdEncoding.DecodeString(parts[1])
						if err != nil {
							return "", fmt.Errorf("invalid base64: %w", err)
						}

						targetDir := filepath.Join(fsProvider.FSRoot(), "tenants", slug)
						if err := os.MkdirAll(targetDir, 0755); err != nil {
							return "", err
						}

						fileName := "logo" + ext
						filePath := filepath.Join(targetDir, fileName)
						if err := os.WriteFile(filePath, data, 0644); err != nil {
							return "", err
						}
						return fmt.Sprintf("/v1/assets/tenants/%s/%s", slug, fileName), nil
					}

					if newSettings.LogoURL != "" {
						if url, err := saveLogo(slug, newSettings.LogoURL); err == nil {
							newSettings.LogoURL = url
						} else {
							httpx.WriteError(w, http.StatusBadRequest, "invalid_logo", err.Error(), 5007)
							return
						}
					}

					// Apply via Raft if cluster
					if c != nil && c.ClusterNode != nil {
						payload, _ := json.Marshal(clusterv1.UpdateTenantSettingsDTO{Settings: newSettings})
						m := clusterv1.Mutation{Type: clusterv1.MutationUpdateTenantSettings, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
						if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
							httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
							return
						}
						_, newRaw, _ := fsProvider.GetTenantSettingsRaw(r.Context(), slug)
						w.Header().Set("ETag", etag(newRaw))
						httpx.WriteJSON(w, http.StatusOK, map[string]any{"updated": true})
						return
					}

					if err := fsProvider.UpdateTenantSettings(r.Context(), slug, &newSettings); err != nil {
						httpx.WriteError(w, http.StatusInternalServerError, "update_failed", err.Error(), 5032)
						return
					}

					// Sync User Fields if present
					if newSettings.UserFields != nil {
						if c.TenantSQLManager != nil {
							if store, err := c.TenantSQLManager.GetPG(r.Context(), slug); err == nil {
								sm := tenantsql.NewSchemaManager(store.Pool())
								// Need tenant ID for lock, try to get it from context or slug
								// For now use slug as ID for lock if ID not handy, but better to fetch tenant.
								// We already fetched raw settings, maybe we can fetch tenant quickly or just use slug.
								// Using slug is safe enough for lock ID usually.
								_ = tenantsql.WithTenantMigrationLock(r.Context(), store.Pool(), slug, 10*time.Second, func(ctx context.Context) error {
									return sm.SyncUserFields(ctx, slug, newSettings.UserFields)
								})
							}
						}
					}

					_, newRaw, _ := fsProvider.GetTenantSettingsRaw(r.Context(), slug)
					w.Header().Set("ETag", etag(newRaw))
					httpx.WriteJSON(w, http.StatusOK, map[string]any{"updated": true})
					return

				default:
					httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET/PUT", 5021)
					return
				}
			}

			// Under tenant: clients CRUD via FS
			if len(parts) >= 2 && parts[1] == "clients" {
				// PUT /v1/admin/tenants/{slug}/clients/{clientID}
				if len(parts) == 3 && r.Method == http.MethodPut {
					var in controlplane.ClientInput
					if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5040)
						return
					}
					in.ClientID = parts[2]
					// Leader enforcement moved to middleware
					if c != nil && c.ClusterNode != nil {
						dto := clusterv1.UpsertClientDTO{Name: in.Name, ClientID: in.ClientID, Type: in.Type, RedirectURIs: in.RedirectURIs, AllowedOrigins: in.AllowedOrigins, Providers: in.Providers, Scopes: in.Scopes, Secret: in.Secret}
						payload, _ := json.Marshal(dto)
						m := clusterv1.Mutation{Type: clusterv1.MutationUpsertClient, TenantSlug: slug, TsUnix: time.Now().Unix(), Payload: payload}
						if _, err := c.ClusterNode.Apply(r.Context(), m); err != nil {
							httpx.WriteError(w, http.StatusServiceUnavailable, "apply_failed", err.Error(), 4002)
							return
						}
						co, err := cpctx.Provider.GetClient(r.Context(), slug, in.ClientID)
						if err != nil {
							httpx.WriteError(w, http.StatusInternalServerError, "readback_failed", err.Error(), 5042)
							return
						}
						httpx.WriteJSON(w, http.StatusOK, co)
						return
					}
					cobj, err := cpctx.Provider.UpsertClient(r.Context(), slug, in)
					if err != nil {
						httpx.WriteError(w, http.StatusBadRequest, "upsert_failed", err.Error(), 5041)
						return
					}
					httpx.WriteJSON(w, http.StatusOK, cobj)
					return
				}
			}

			// Under tenant: scopes bulk upsert (simple array)
			if len(parts) == 2 && parts[1] == "scopes" && (r.Method == http.MethodPut || r.Method == http.MethodPost) {
				var in struct {
					Scopes []controlplane.Scope `json:"scopes"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "JSON inválido", 5050)
					return
				}
				// naive: replace one by one using UpsertScope
				for _, s := range in.Scopes {
					_ = cpctx.Provider.UpsertScope(r.Context(), slug, controlplane.Scope{Name: strings.TrimSpace(s.Name), Description: s.Description})
				}
				httpx.WriteJSON(w, http.StatusOK, map[string]any{"updated": true})
				return
			}

			httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5010)
			return

		default:
			httpx.WriteError(w, http.StatusNotFound, "not_found", "", 5010)
			return
		}
	})
}

// validateIssuerMode valida el enum aceptando vacío como "global".
func validateIssuerMode(m controlplane.IssuerMode) error {
	switch m {
	case "", controlplane.IssuerModeGlobal, controlplane.IssuerModePath, controlplane.IssuerModeDomain:
		return nil
	default:
		return fmt.Errorf("invalid issuerMode: %q", m)
	}
}
