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


admin_mailing.go — Admin “Send Test Email” (SMTP config resolution + decryption + send)

Qué hace esta pieza
-------------------
Este archivo implementa un endpoint administrativo para enviar un “email de prueba” (test email)
para validar que la configuración SMTP de un tenant funciona.

El flujo principal:
  - Recibe un request POST con JSON { to, smtp? }.
  - Determina la configuración SMTP efectiva:
      A) si el request trae smtp (override), usa esa (ideal para probar valores de un formulario).
      B) si no trae smtp, usa la configuración SMTP guardada en el tenant (t.Settings.SMTP).
  - Si la config guardada trae PasswordEnc y Password está vacío, intenta descifrar PasswordEnc
    usando una master key desde environment.
  - Construye un SMTP sender (email.NewSMTPSender) y envía un mail con subject/body de prueba.
  - Devuelve 200 con {status:"ok", sent_to:"..."} o un error con httpx.WriteError.

No maneja persistencia (no guarda SMTP). Solo “prueba envío”.

Endpoints / contrato esperado
-----------------------------
Se asume que esta función se usa en una ruta administrativa (enrutada desde otro handler/router).
Solo acepta:
  - POST (si no, 405)

Request JSON:
  - to: string requerido (destinatario)
  - smtp: objeto opcional controlplane.SMTPSettings (override para probar credenciales)

Response JSON:
  - 200 OK: {"status":"ok","sent_to":"..."}
Errores:
  - 405 si no es POST
  - 400 si falta 'to' o no hay SMTP disponible
  - 502 si falla el envío SMTP (BadGateway) con diagnóstico

DTOs actuales (acoplamiento)
----------------------------
- TestEmailReq: DTO HTTP de entrada {To, SMTP?}.
- SMTPSettings: viene de controlplane (modelo interno), usado directamente como DTO de entrada.
  Esto acopla el contrato HTTP al modelo interno de controlplane, y además permite que el request
  inyecte campos que quizá no querés exponer (ej: PasswordEnc, etc.). En V2 conviene DTO propio.

Cómo determina la configuración SMTP efectiva
---------------------------------------------
1) Valida request:
   - requiere r.Method POST
   - parsea JSON con httpx.ReadJSON
   - req.To no puede estar vacío

2) Selección de configuración:
   - Si req.SMTP != nil => smtpCfg = req.SMTP (override)
   - Sino:
       - Si t.Settings.SMTP != nil => smtpCfg = t.Settings.SMTP

3) Desencriptado de password (solo en fallback stored settings):
   - Si smtpCfg.PasswordEnc != "" y smtpCfg.Password == "":
       - lee masterKey desde env var "SIGNING_MASTER_KEY"
       - intenta:
          DecodeBase64URL(PasswordEnc) -> bytes
          DecryptPrivateKey(bytes, masterKey) -> plaintext
          smtpCfg.Password = string(plaintext)

   Nota: el nombre "SIGNING_MASTER_KEY" es confuso: se usa para descifrar password SMTP, no para firmar JWT.
   También muta smtpCfg.Password en memoria (puede impactar si smtpCfg apunta a un struct compartido).

4) Validaciones mínimas:
   - Si smtpCfg nil o Host vacío => 400 smtp_missing
   - Si Port=0 => default 587

5) Construye sender y envía:
   - sender := email.NewSMTPSender(host, port, fromEmail, username, password)
   - Si UseTLS true => ajusta sender.TLSMode="starttls" (hay comentario de ambigüedad STARTTLS vs SSL)
   - Genera subject/body HTML/Text con info del tenant y config usada.
   - sender.Send(to, subject, html, text)
   - Si error => email.DiagnoseSMTP(err) y devuelve 502 con detalle

Puntos de mejora (deuda técnica / refactor hacia V2)
----------------------------------------------------
1) Separación de capas (Controller/Service/Client):
   Actualmente:
     - valida método + parsea JSON (controller)
     - resuelve config efectiva y descifra secret (service)
     - instancia SMTP sender y envía (client)
   Está todo mezclado en una función.

   En V2:
     - Controller: valida POST, parsea dto.TestEmailRequest, obtiene tenant desde TenantContext,
       llama a service.SendTestEmail(...)
     - Service: decide config efectiva, valida campos, (opcional) descifra password,
       arma contenido, llama a EmailClient.
     - Client: interfaz EmailSender con implementación SMTP.

2) Evitar mutar settings compartidas:
   Cuando usa t.Settings.SMTP, smtpCfg apunta al mismo struct del tenant. Al descifrar, se asigna smtpCfg.Password,
   lo cual puede quedar “cacheado” en memoria del tenant y se presta a leaks (por logs o dumps).
   Mejor:
     - clonar smtpCfg antes de completar Password
     - y mantener el password en una variable local solo para el envío.

3) Gestión segura de secretos:
   - No debería depender de una env var con nombre ambiguo (“SIGNING_MASTER_KEY”) para descifrar SMTP.
   - Si hay un “master key” del sistema, debería vivir en un componente SecretManager (TenantResources),
     y/o usar una env var específica (ej: SMTP_MASTER_KEY o SECRETBOX_MASTER_KEY) consistente.
   - No devolver nunca el password en responses, ni loguearlo.

4) TLS mode ambiguo:
   El código fuerza TLSMode="starttls" si UseTLS es true, pero el comentario indica ambigüedad.
   En V2: definir contrato claro:
     - tlsMode: "auto" | "starttls" | "ssl"
     - y mapearlo sin suposiciones. O bien: UseTLS + Port 465 => ssl, Port 587 => starttls.

5) Timeouts y experiencia de usuario:
   Send SMTP puede tardar o colgar si no hay timeout (depende de email package).
   En V2 conviene:
     - usar context con timeout (ej 10s) si el cliente SMTP lo soporta
     - devolver error claro si se excede el timeout

6) Concurrencia (Golang) — dónde aplica
   Un “send test email” normalmente es una acción manual, así que:
     - se puede hacer sincrónico (responder cuando termina).
   Pero para no bloquear el server ante SMTP lento, hay 2 enfoques:
     A) Sincrónico con timeout (recomendado para “test”)
     B) Asincrónico: encolar un job en un worker pool y responder 202 Accepted con un “job id”.
        Esto es útil si querés UI que muestre estado. Es más complejo, pero prolijo.

   Si adoptás worker pool:
     - tené un pool “emails” con concurrencia limitada (semáforo/pool) para no saturar salida
     - y siempre con context/cancelación en shutdown.

Mapa a arquitectura V2 (qué sería qué)
--------------------------------------
- DTOs:
  - dto.AdminTestEmailRequest { to, smtpOverride? }
  - dto.SMTPOverride { host, port, fromEmail, username, password, tlsMode }
  (evitar usar controlplane.SMTPSettings directamente como DTO público)

- Controller:
  - AdminMailingController.SendTestEmail(ctx, tenantCtx, req)

- Service:
  - MailingService.SendTestEmail(ctx, tenantID, to, smtpOverride?)
    - resuelve config: override vs tenant settings
    - valida
    - obtiene credenciales (descifra via SecretManager)
    - llama email client
    - traduce errores a códigos (smtp auth fail, timeout, dns fail, etc.)

- Client:
  - EmailClient interface:
      Send(ctx, to, subject, html, text) error
    Implementación SMTP (wrapper de email.NewSMTPSender).

Decisiones de compatibilidad (para no romper)
---------------------------------------------
- Solo POST.
- Si no se envía smtp override, usa SMTP del tenant.
- Si falta SMTP o Host => 400 smtp_missing.
- Port default 587 si viene 0.
- Ante error SMTP: 502 smtp_error con diagnóstico.

Concurrencia recomendada acá (para tu V2)

Para “test email” yo haría sincrónico con timeout (simple y UX clara).
---------------------------------------------------------------------
- Si igual querés aprovechar Go:
  + ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
  +  y tu cliente SMTP respeta ese contexto (si tu lib no lo respeta, le agregás timeouts al dial).
- Si querés “pro” con UI:
  +  Worker pool emails + queue size chico (backpressure)
  +  POST /admin/v2/mailing/test devuelve 202 Accepted + jobId
  +  GET /admin/v2/jobs/{jobId} para ver estado (esto ya es otro módulo).




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



admin_users.go — Admin Actions sobre usuarios (disable/enable/resend verification) + revoke tokens + audit + emails

Qué hace este handler
---------------------
Este handler implementa acciones administrativas sobre usuarios. No es “CRUD de users”, sino acciones puntuales:
  1) Bloquear (disable) un usuario, opcionalmente por un tiempo (duration) y con motivo (reason).
     - Revoca refresh tokens del usuario (best-effort).
     - Loguea auditoría.
     - (Opcional) Envía email de notificación usando templates del tenant.

  2) Desbloquear (enable) un usuario.
     - Loguea auditoría.
     - (Opcional) Envía email de notificación usando templates.

  3) Reenviar email de verificación (resend-verification).
     - Verifica que el usuario exista y NO esté verificado.
     - Genera token de verificación en la DB del tenant (TokenStore).
     - Construye link de verificación (BASE_URL + endpoint verify-email) incluyendo tenant_id y opcional client_id.
     - Renderiza template de mail del tenant (o fallback).
     - Envía email.
     - Loguea auditoría.

Rutas que maneja (todas POST)
-----------------------------
- POST /v1/admin/users/disable
- POST /v1/admin/users/enable
- POST /v1/admin/users/resend-verification

Todas las rutas comparten el mismo body:
  {
    "user_id": "uuid",
    "tenant_id": "opcional (uuid o slug según caso)",
    "reason": "opcional",
    "duration": "opcional (e.g. 24h, 30m, 2h30m)"
  }

Precondiciones / dependencias
-----------------------------
- Requiere h.c.Store != nil (si no, 501 not_implemented).
- Algunas rutas usan h.c.TenantSQLManager para resolver la DB del tenant.
- Para emails usa:
    - h.c.SenderProvider.GetSender(ctx, tenantUUID)
    - templates desde controlplane tenant settings (cpctx.Provider...)
    - renderOverride(tpl, vars) definido en otro archivo del mismo package.

- Para auditoría usa audit.Log(ctx, event, fields).

Cómo resuelve el store (global vs tenant)
-----------------------------------------
El handler define una interface local `userManager` con lo mínimo que necesita:
  DisableUser(ctx, userID, by, reason string, until *time.Time) error
  EnableUser(ctx, userID, by string) error
  GetUserByID(ctx, id string) (*core.User, error)

Por defecto:
  store := h.c.Store  (global store)

Si body.TenantID viene:
  - intenta h.c.TenantSQLManager.GetPG(ctx, body.TenantID) y usa ese store tenant-specific
  - también asigna `revoker = ts` (si implementa RevokeAllRefreshByUser)

Si NO viene TenantID:
  - revoker se intenta obtener del global store por type assertion:
      RevokeAllRefreshByUser(ctx, userID) (int, error)
  - si no existe, fallback a método legacy:
      h.c.Store.RevokeAllRefreshTokens(ctx, userID, "")

Ojo conceptual:
- body.TenantID se usa tanto para "buscar tenant store" como para “resolver templates”, pero puede ser slug o UUID.
- En varios lugares asume que TenantID es UUID (uuid.MustParse), lo que puede romper si te pasan slug.

Cómo obtiene "by" (quién ejecuta la acción)
--------------------------------------------
Lee claims del contexto (httpx.GetClaims(ctx)) y toma `sub` como actor (by).
Esto se guarda en auditoría y se pasa al store para registrar “by”/razón/hasta cuándo.

Endpoint: POST /v1/admin/users/disable
--------------------------------------
Flujo:
1) Parse body, valida user_id requerido.
2) Si duration viene:
     - time.ParseDuration(duration)
     - until = now + duration
3) Llama store.DisableUser(ctx, user_id, by, reason, until).
4) Revoca tokens:
     - si revoker != nil => revoker.RevokeAllRefreshByUser(ctx, user_id) (best-effort)
     - else fallback legacy RevokeAllRefreshTokens(user_id, "")
5) Audit:
     event: "admin_user_disabled"
     fields: by, user_id, reason, tenant_id, until?
6) Email (best-effort):
     - store.GetUserByID() para obtener email y tenantID real del usuario
     - determina tid = body.TenantID si vino, si no u.TenantID
     - busca tenant settings con cpctx.Provider.GetTenantBySlug(ctx, tid) (ojo: asume slug)
     - si hay template email.TemplateUserBlocked:
         vars {UserEmail, Reason, Until, Tenant}
         renderOverride(tpl, vars)
         sender := h.c.SenderProvider.GetSender(ctx, uuid.MustParse(tid)) (ojo: asume UUID)
         sender.Send(...)
7) Responde 204 No Content

Riesgos/bugs actuales:
- Si tid es slug, uuid.MustParse(tid) va a panic.
- cpctx.Provider.GetTenantBySlug con tid UUID probablemente falle.
- El email se intenta con dos supuestos contradictorios (slug y UUID).

Endpoint: POST /v1/admin/users/enable
-------------------------------------
Similar a disable pero:
- store.EnableUser(ctx, user_id, by)
- audit: "admin_user_enabled"
- email con template email.TemplateUserUnblocked (vars {UserEmail, Tenant})
- Responde 204.

Endpoint: POST /v1/admin/users/resend-verification
--------------------------------------------------
Flujo:
1) Requiere tenant_id explícito (400 si falta).
2) store.GetUserByID(ctx, user_id)
   - si no existe => 404 user_not_found
   - si u.EmailVerified => 400 already_verified
3) Resuelve tenantUUID:
   - si tenant_id parsea UUID -> ok
   - si no, intenta cpctx.Provider.GetTenantBySlug y parsea t.ID como UUID
   - si no puede => 400 invalid_tenant
4) TokenStore:
   - requiere h.c.TenantSQLManager
   - tenantDB := GetPG(ctx, body.TenantID) (ojo: si body.TenantID es slug, esto puede fallar si GetPG espera slug o id según implementación)
   - tokenStore := storelib.NewTokenStore(tenantDB.Pool())
5) Crea token verify:
   verifyTTL := 48h
   pt := tokenStore.CreateEmailVerification(ctx, tenantUUID, userUUID, email, ttl, nil, nil)
6) Construye link:
   baseURL = ENV BASE_URL (default http://localhost:8080)
   link = baseURL + "/v1/auth/verify-email?token=" + pt + "&tenant_id=" + body.TenantID
   si u.SourceClientID != nil => &client_id=...
   (No agrega redirect_uri intencionalmente)
7) Renderiza template:
   - intenta obtener tenant con cpctx.Provider.GetTenantByID(ctx, tenantUUID.String())
   - vars {UserEmail, Tenant, Link, TTL}
   - si existe template email.TemplateVerify => renderOverride
   - fallback a email simple si no hay template
8) Envío:
   sender := h.c.SenderProvider.GetSender(ctx, tenantUUID)
   sender.Send(u.Email, subj, htmlBody, textBody)
9) Audit: "admin_resend_verification"
10) Responde 204.

Problemas importantes / deuda técnica
-------------------------------------
1) Inconsistencia tenant identifier (slug vs UUID):
   - Disable/Enable mezclan GetTenantBySlug(tid) con uuid.MustParse(tid).
   - Resend-verification intenta soportar ambos, pero después vuelve a llamar GetPG con body.TenantID (que podría ser slug/uuid).
   Esto es fuente de bugs y panics.

2) Mezcla de responsabilidades (God controller):
   Controller hace:
     - parse/validación
     - resolver store (global/tenant)
     - lógica de disable/enable
     - revocación de tokens
     - auditoría
     - armado y envío de emails + templates
     - generación de tokens y links
   En V2 esto debe separarse en services.

3) Side effects pesados inline:
   - enviar mails y generar tokens está inline en la request.
   - si el SMTP cuelga, este endpoint se vuelve lento.
   Mejor: best-effort con timeout o job async.

4) Falta de límites:
   - no usa MaxBytesReader para body.
   - no valida UUID de user_id antes de usar uuid.MustParse en resend-verification.

5) Revoke tokens “best-effort” pero inconsistente:
   - si existe revoker usa RevokeAllRefreshByUser
   - si no, usa RevokeAllRefreshTokens(user,"")
   Esto sugiere APIs duplicadas en store.

Cómo lo refactorizaría (V2) — arquitectura y patrones
-----------------------------------------------------
Objetivo: controller fino + services + repos + clients.

1) TenantResolver (clave)
   Crear un componente único:
     ResolveTenant(ctx, tenantIdOrSlug string) -> (tenantUUID, tenantSlug, *Tenant, error)
   y otro:
     ResolveUserTenant(ctx, user *core.User, providedTenant string) -> tenantUUID/slug
   Así eliminás los panics y la mezcla slug/uuid.

2) Servicios por dominio
   - services/admin_user_service.go:
       DisableUser(ctx, req, actor) -> error
       EnableUser(ctx, req, actor) -> error
       ResendVerification(ctx, req, actor) -> error

     Este service encapsula:
       - resolver store correcto (global/tenant)
       - parse duration
       - disable/enable
       - revocar refresh tokens
       - auditoría
       - “disparar” notificación email (sync con timeout o async)

   - services/verification_service.go:
       CreateVerificationToken(ctx, tenantUUID, userUUID, email, ttl) -> token

   - services/email_notification_service.go:
       SendUserBlocked(ctx, tenantUUID, email, vars)
       SendUserUnblocked(ctx, tenantUUID, email, vars)
       SendVerifyEmail(ctx, tenantUUID, email, link, vars)

3) Repos/clients
   - UserAdminRepository (global y tenant):
       DisableUser(...)
       EnableUser(...)
       GetUserByID(...)
       RevokeAllRefreshByUser(...)  // unificar en una sola API

   - TokenStoreFactory:
       ForTenant(ctx, tenantUUID/slug) -> TokenStore
     (para no crear TokenStore directo en controller)

   - LinkBuilder:
       BuildVerifyEmailLink(baseURL, token, tenantKey, clientID?) -> string

4) Concurrencia (donde conviene)
   - En disable/enable:
       - hacer disable/enable + revoke tokens sincrónico (rápido)
       - email: goroutine con timeout corto (ej 2-3s) y log warning si falla
         (o encolar job si querés full robust)
   Importante: NO hacer goroutines sin control masivo; acá son 1-2 emails por request.

5) DTOs
   - dtos/admin_users.go:
       DisableUserRequest { userId, tenantId?, reason?, duration? }
       EnableUserRequest  { userId, tenantId? }
       ResendVerificationRequest { userId, tenantId }
   - respuestas: 204 o {status:"ok"} si querés UI-friendly.

6) Seguridad y consistencia
   - Validar user_id UUID siempre antes de usarlo.
   - Nunca usar uuid.MustParse con datos externos.
   - Cache-Control: no-store (admin endpoints + tokens).
   - Body size limit: MaxBytesReader(w, r.Body, 32<<10).

Refactor mapping (para tu V2)
-----------------------------
- Controller:
    controllers/admin_users_controller.go
      -> switch por path (disable/enable/resend)
      -> parse DTO + call service

- Service:
    services/admin_user_service.go
    services/tenant_resolver.go
    services/email_notifications.go
    services/verification_service.go
    services/link_builder.go

- Clients/Repos:
    clients/controlplane_client.go (para traer tenant templates)
    repos/user_admin_repo.go
    repos/token_repo.go



Dos “bombitas” que te conviene arreglar YA (aunque sea en v1)
-------------------------------------------------------------
	+ uuid.MustParse(tid) te puede tumbar el proceso si tid es slug.
	Cambialo por parse seguro y resolver tenant UUID bien.

	+ Unificá “tenantIdOrSlug” en una sola convención de entrada
	(o siempre slug, o siempre UUID) y resolvelo con un TenantResolver.




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



auth_config.go — Endpoint “config” para el frontend de auth (branding + providers + features + custom fields)

Qué hace este handler
---------------------
Implementa:
  GET /v1/auth/config?client_id=...

Su objetivo es devolverle al frontend (login/register UI, SDK, etc.) la “config pública” necesaria para:
- Branding del tenant (nombre, slug, logo, color)
- Datos del client (nombre, providers habilitados)
- Flags de features (smtp/social/mfa/require_email_verification)
- Definición de custom fields (derivada de tenant.Settings.UserFields)
- URLs de flujos email (reset password / verify email) si vienen configuradas en el client (FS)

Si NO viene client_id:
- devuelve un config genérico “HelloJohn Admin” con password_enabled=true (modo fallback).

Flujo real (paso a paso)
------------------------
1) Lee query param `client_id`.
   - Si viene vacío => responde “Admin config” genérico y chau.

2) Busca el client en SQL (Store)
   - c.Store.GetClientByClientID(ctx, clientID)
   - Si falla o nil, hace un fallback pesado a FS:
       - cpctx.Provider.ListTenants()
       - por cada tenant: cpctx.Provider.GetClient(ctx, tenantSlug, clientID)
       - si lo encuentra, “fabrica” un core.Client (medio trucho) con:
           - ID = cFS.ClientID (ojo)
           - TenantID = t.ID (o si vacío, usa t.Slug)
           - Name + Providers del FS
       - además guarda `clientFS` para extraer config extra (RequireEmailVerification, ResetPasswordURL, VerifyEmailURL)

3) Si no encontró client => 404 client_not_found

4) Busca el tenant para branding
   - exige cpctx.Provider != nil
   - intenta cpctx.Provider.GetTenantByID(ctx, cl.TenantID)
   - si falla => vuelve a listar tenants y matchea por ID o por Slug (otro O(N))

5) Construye la respuesta
   - TenantName / TenantSlug / ClientName / SocialProviders / PasswordEnabled
   - si `clientFS` existe:
       - setea RequireEmailVerification + ResetPasswordURL + VerifyEmailURL
   - logo:
       - si t.Settings.LogoURL existe, lo usa
       - si no existe o no empieza con http => intenta leer “logo.png” del FS (DATA_ROOT/tenants/{slug}/logo.png)
         y lo embebe como data URL base64 (data:image/png;base64,...)
   - color:
       - PrimaryColor = t.Settings.BrandColor (si está)
   - passwordEnabled:
       - por default true
       - si cl.Providers tiene items, revisa si incluye "password" (case-insensitive)
   - Features map:
       smtp_enabled, social_login_enabled, mfa_enabled, require_email_verification
   - CustomFields:
       recorre t.Settings.UserFields y los transforma a CustomFieldSchema (Label = Name)

Cuellos de botella / cosas “viejas y rotas” probables
-----------------------------------------------------
A) O(N) en caliente por request (dos veces)
   - Fallback de client: ListTenants + GetClient por tenant (potencialmente carísimo)
   - Fallback de tenant: ListTenants y match ID/slug
   Esto escala horrible con muchos tenants.

B) Mezcla de fuentes (SQL vs FS) sin una abstracción clara
   - El handler hace “dual read” y arma structs fake.
   - cl.ID = clientID cuando no hay UUID real => te va a romper invariantes en otros lugares.

C) Logo embebido como base64 en el JSON
   - Puede inflar respuestas y cache/cdn se vuelve un quilombo.
   - Además lee disco por request (otra vez, caro).

D) Logging “DEBUG” con log.Printf en handler
   - Ruido en prod y puede filtrar info operativa.
   - Si mañana metés secrets en settings, te podés mandar una cagada.

E) Responsabilidades mezcladas
   - Controller hace: lookup, fallback FS, resolver tenant, leer logo del FS, construir response.
   - Difícil de testear, y más difícil de evolucionar.

F) Esquema de custom fields: hoy Label=Name porque “no hay label”
   - Si mañana agregás label, este handler debería estar listo para mapearlo.
   - Y Type “text/number/boolean”: hoy se confía en uf.Type sin validación.

Cómo lo refactorizaría en V2 (bien prolijo)
-------------------------------------------
Meta: controller finito + service + repos + caches. Y que “de dónde sale la data” esté encapsulado.

1) Introducir un “ConfigService” (Service Layer)
   - Patrón GoF: Facade (fachada) hacia varias fuentes y caches.
   - Firma:
       ConfigService.GetAuthConfig(ctx, clientID string) (AuthConfigResponse, error)

2) Introducir un “ClientResolver” (Strategy + Chain of Responsibility)
   - GoF: Strategy para “resolver client” según backend activo.
   - GoF: Chain of Responsibility para fallback ordenado:
       a) SQLClientRepo (rápido)
       b) FSClientRepo (si está habilitado)
   - Esto evita que el handler tenga loops de tenants.

3) Introducir “TenantResolver” (cacheado)
   - ResolveTenantByIDOrSlug(ctx, idOrSlug) -> Tenant
   - Cache TTL/LRU (por ej 1-5 min) y/o invalidación cuando cambia el controlplane.

4) Introducir “LogoProvider” (Strategy)
   - GoF: Strategy para resolver logo:
       - URLLogoProvider (si LogoURL es http(s))
       - FSLogoProvider (lee del FS)
   - Pero importante: NO embebas base64 en este endpoint si podés evitarlo.
     Mejor:
       - devolver siempre una URL de assets (ej /v1/assets/tenants/{slug}/logo.png)
       - y que ese endpoint sirva el archivo con cache headers.

5) DTOs + contratos estables
   - Este endpoint es “público” => mantenelo versionado.
   - CamelCase consistente si tu v2 lo pide.
   - Validaciones: providers permitidos, tipos de custom fields, etc.

6) Cache multi-nivel (mejora de infra sin goroutines falopa)
   - Cachear:
       - client config por client_id (TTL corto)
       - tenant branding por tenant_slug/id (TTL corto)
       - logo bytes o “logo URL” (TTL más largo)
   - Esto te baja muchísimo CPU/IO.

Dónde meter concurrencia (si querés aprovechar Go)
--------------------------------------------------
Acá sí tiene sentido un poco, pero con criterio:
- Una vez que resolviste el tenant y el client:
  podés construir features + custom fields en paralelo? meh, no vale.
- Donde sí: si tu LogoProvider implica IO (leer archivo, o pedir a storage),
  y también necesitás otra cosa, podés hacer:
    - goroutine para resolver logo
    - goroutine para resolver tenant settings
  y esperar con errgroup + context.
Pero si cacheás bien, no lo necesitás.

Plan concreto de refactor por capas
-----------------------------------
A) Controller: controllers/auth_config_controller.go
   - parsea client_id
   - llama configSvc.GetAuthConfig(ctx, clientID)
   - WriteJSON

B) Service: services/auth_config_service.go
   - si clientID vacío => config admin default
   - cl := clientResolver.ResolveByClientID(ctx, clientID)
   - tenant := tenantResolver.Resolve(ctx, cl.TenantRef) (id o slug)
   - logo := logoProvider.Resolve(ctx, tenant)
   - arma response (features + providers + custom fields)
   - devuelve

C) Repos:
   - repos/client_repo_sql.go
   - repos/client_repo_fs.go
   - repos/tenant_repo_cp.go (o tenant resolver)

D) Infra:
   - infra/cache (TTL/LRU)
   - infra/assets (servir logo estático con cache-control)

Checklist de mejoras inmediatas sin romper todo
-----------------------------------------------
- Sacar loops de ListTenants() del handler (ponerlo en resolver con cache).
- Dejar de embebir logo base64 en config (devolver URL).
- No “fabrices” core.Client con ID=clientID (creá un DTO propio de config).
- Limitar logs y mover a logger estructurado.
- Test unitarios: casos SQL ok, FS fallback ok, tenant mismatch, client not found.

Resumen del veredicto
---------------------
Este handler es útil, pero hoy es un “mezcladito” de SQL+FS con búsquedas O(N) y lectura de FS por request.
En V2 hay que convertirlo en un Facade (ConfigService) apoyado en Resolvers (Strategy/Chain) + caches,
y separar “branding/assets” del JSON para que sea eficiente y mantenible.




auth_login.go — Login “password” + emisión de tokens (access + refresh) + gating por client + rate limit + MFA + RBAC (opcional)

Qué hace este handler
---------------------
Implementa (en la práctica):
  POST /v1/auth/login

Recibe credenciales (email + password) y, si valida:
- Verifica que el client exista y permita provider "password"
- Busca usuario por email en el user-store del tenant
- Chequea password hash
- Bloquea si el usuario está deshabilitado
- Opcional: exige email verificado si el client lo pide (FS controlplane)
- Opcional: bifurca a MFA (TOTP) si está habilitada y el device no es trusted
- Emite Access Token (JWT EdDSA) con issuer efectivo por tenant
- Persiste/crea Refresh Token via CreateRefreshTokenTC (store method)
- Devuelve JSON con access_token + refresh_token + expiración

Además:
- Soporta login “FS Admin” (sin tenant/client) si FSAdminEnabled()
- Soporta body JSON o form-urlencoded
- Aplica rate limiting específico de login (si MultiLimiter configurado)

Entrada / salida
----------------
Request (JSON o form):
  {
    tenant_id, client_id, email, password
  }
Notas:
- tenant_id y client_id son opcionales SOLO para el modo “FS admin”
- email y password siempre obligatorios

Response (OK):
  { access_token, token_type="Bearer", expires_in, refresh_token }

Errores relevantes:
- 400 missing_fields (si falta email/pass o tenant/client cuando no hay FS admin)
- 401 invalid_credentials (usuario o password)
- 401 invalid_client (tenant/client inválidos o password no permitido)
- 423 user_disabled
- 403 email_not_verified (si client exige verificación)
- 429 rate limit (lo escribe EnforceLoginLimit)
- 5xx varios (tenant manager, store, emisión tokens, etc.)

Flujo paso a paso (normal, NO FS-admin)
---------------------------------------
1) Validación HTTP + parse body
   - Solo POST.
   - Content-Type:
       a) application/json:
          - lee body (MaxBytes 1MB)
          - json.Unmarshal a AuthLoginRequest (snake_case)
          - fallback extra: intenta PascalCase (TenantID/ClientID/Email/Password)
             * Esto existe por compat de tests/clients viejos.
       b) application/x-www-form-urlencoded:
          - ParseForm() y lee tenant_id/client_id/email/password
       c) otro CT => 400

   Normaliza email => trim + strings.ToLower

2) Validación mínima
   - email y password obligatorios
   - si falta tenant_id o client_id:
       - si helpers.FSAdminEnabled(): intenta FSAdminVerify(email, password)
           - si OK => emite token “admin” (JWT access + refresh JWT stateless)
           - si FAIL => 401 invalid_credentials
       - si FSAdmin NO habilitado => 400 (tenant_id y client_id obligatorios)

3) Resolve tenant slug + tenant UUID
   - helpers.ResolveTenantSlugAndID(ctx, req.TenantID)
   - Devuelve tenantSlug y tenantUUID (string con UUID? O slug? según helper)

4) Resolver client (prefer FS) ANTES de abrir DB
   - helpers.ResolveClientFSBySlug(ctx, tenantSlug, req.ClientID)
   - si existe:
       - guarda scopes/providers desde FSClient
       - haveFSClient = true
   - si no:
       - se deja para fallback DB más adelante (cuando el repo ya esté abierto)

   Objetivo: “si client es inválido, no abras DB al pedo” (aunque hoy igual la abre antes del fallback DB; el comentario dice una cosa y el código hace otra en algunos caminos).

5) Abrir repo del tenant (TenantSQLManager)
   - helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
   - Si error:
       - si tenant inválido => 401 invalid_client (tenant inválido)
       - si FSAdminEnabled() => fallback FS admin (pero ahora con aud=req.ClientID)
           * Provider gating: si el client tenía providers y no incluye "password" => bloquea
           * Emite access token (sin refresh)
       - si no DB configurada => httpx.WriteTenantDBMissing
       - otros => httpx.WriteTenantDBError
   - Si OK: guarda repoCore y lo mete en context (helpers.WithTenantRepo)

6) Rate limiting (si c.MultiLimiter != nil)
   - Lee cfg.Rate.Login.Window (parse duration)
   - EnforceLoginLimit(w, r, limiter, loginCfg, req.TenantID, req.Email)
   - Si rate limited => ya respondió y corta

7) Si no había FS client, lookup en DB ahora que repo está abierto
   - interface clientGetter { GetClientByClientID(...) }
   - si el repo lo implementa, trae scopes/providers
   - si no existe client => 401 invalid_client
   - si repo no implementa => 401 invalid_client

8) Provider gating (clientProviders)
   - si providers no vacío => debe contener "password"
   - si no => 401 invalid_client (“password login deshabilitado para este client”)

9) Buscar usuario + identidad por email
   - repo.GetUserByEmail(ctx, tenantUUID, email)
   - si ErrNotFound => 401 invalid_credentials
   - otros => 500 invalid_credentials (hoy usa InternalServerError para err != NotFound)

10) Bloqueo de usuario
   - Si DisabledUntil != nil y now < DisabledUntil => locked
   - Si DisabledAt != nil => locked
   - Responde 423 Locked (bien)

11) Verificar password
   - Si identity/passwordHash nil o vacío => 401
   - repo.CheckPassword(hash, req.Password) => 401 si no coincide

12) Email verification (opcional por client FS)
   - cpctx.Provider.GetClient(ctx, tenantSlug, req.ClientID)
   - si RequireEmailVerification && !u.EmailVerified => 403 email_not_verified

13) MFA pre-issue (opcional)
   - Si repo implementa:
       - GetMFATOTP(userID) -> si ConfirmedAt != nil => MFA configurada
       - Si cookie "mfa_trust" existe y repo implementa IsTrustedDevice:
           - calcula hash tokens.SHA256Base64URL(cookie.Value)
           - si trusted => trustedByCookie=true
       - Si NO trusted => bifurca:
           - crea mfaChallenge (struct no visible acá)
           - genera opaque token mid
           - guarda en c.Cache (TTL 5m) bajo key mfa:token:<mid>
           - responde 200 con {mfa_required:true, mfa_token:mid, amr:["pwd"]}

14) Claims base + hooks + RBAC opcional
   - amr/acr:
       - default amr=["pwd"], acr=loa:1
       - si trustedByCookie => amr=["pwd","mfa"], acr=loa:2
   - scopes: grantedScopes = clientScopes
   - std claims: tid, amr, acr, scp
   - applyAccessClaimsHook(...) (hook opcional, puede mutar std/custom)
   - RBAC opcional:
       - si repo implementa GetUserRoles/GetUserPermissions => los agrega al “custom system claims”

15) Resolver issuer efectivo por tenant (y key para firmar)
   - effIss := c.Issuer.Iss
   - cpctx.Provider.GetTenantBySlug(ctx, tenantSlug)
   - effIss = jwtx.ResolveIssuer(globalIss, issuerMode, slug, override)
   - PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)
   - Selección de key:
       - si IssuerModePath => c.Issuer.Keys.ActiveForTenant(tenantSlug)
       - else => c.Issuer.Keys.Active()

16) Emitir access token (JWT EdDSA)
   - claims: iss, sub, aud=client_id, iat/nbf/exp + std + custom
   - SignedString(priv)

17) Crear refresh token persistente (TC)
   - repo debe implementar CreateRefreshTokenTC(ctx, tenantID, clientID, userID, ttl)
   - si no => 500 store_not_supported
   - si error => 500 persist_failed

18) Responder
   - Cache-Control no-store, Pragma no-cache
   - JSON: AuthLoginResponse (access + refresh + expires)

Cosas que están “raras” o para marcar (sin decidir aún)
-------------------------------------------------------
1) Debug logs a lo pavote
   - log.Printf("DEBUG: ...") por todos lados.
   - Esto en prod te llena logs y te puede afectar performance.
   - No está usando logger estructurado, ni levels reales.

2) Import potencialmente no usado
   - En ESTE archivo, revisá: `github.com/dropDatabas3/hellojohn/internal/controlplane`
     Solo se usa para comparar `ten.Settings.IssuerMode == controlplane.IssuerModePath`.
     O sea: sí se usa.
   - `tokens` y `jwtx` y `util` también se usan.
   - `cpctx` se usa.
   (Igual, el compilador te lo canta si alguno sobra.)

3) Doble modo “FS admin”
   - Hay dos caminos:
       a) cuando faltan tenant/client (stateless refresh JWT para admin)
       b) cuando falla abrir repo tenant y FSAdminEnabled() (sin refresh)
   - Inconsistente: en un caso emite refresh JWT, en otro no.
   - Aud distinto: "admin" vs req.ClientID.
   - Tid fijo "global" (ok) pero mezcla claims/flows.

4) Mezcla de fuentes para client gating
   - Primero intenta FS, después DB.
   - Después vuelve a consultar cpctx.Provider.GetClient() solo para email verification.
   - Es decir: el “client config” se obtiene 2-3 veces por distintos caminos.

5) Cache usage para MFA
   - c.Cache.Set(...) asume que Cache existe y está inicializada.
   - No hay nil-check (si c.Cache puede ser nil, esto explota).
   - El tipo `mfaChallenge` no está definido acá: dependencia implícita del paquete.

6) Comentarios vs comportamiento
   - “Primero resolver client desde FS… y no abrir DB”,
     pero después abre repo igual, y si no había FS client recién ahí intenta DB.
     El objetivo se cumple parcialmente, pero no siempre.

7) Token issuance duplicado
   - La lógica de construir JWT (claims + headers + SignedString) está repetida
     en varios handlers (y dentro de este mismo para FS admin vs normal).
   - Refactor claro hacia un “TokenIssuer” (Builder / Factory) cuando hagamos visión global.

Patrones que encajan para la futura refactor (sin implementarla todavía)
-----------------------------------------------------------------------
- GoF: Facade / Service Layer
  AuthService.LoginPassword(...) que devuelva un “resultado” (tokens o mfa_required).

- GoF: Strategy + Chain of Responsibility
  ClientResolver (FS -> DB), TenantResolver (ID/slug), AdminAuthStrategy.

- GoF: Builder
  Para armar JWT claims + headers de forma consistente (evitar duplicación).

- GoF: Template Method
  “emitAccessToken()” con pasos fijos (issuer -> key -> claims -> sign),
  y variaciones por tipo de sesión (user/admin).

- Concurrencia (solo donde aporte)
  No hace falta en login; el bottleneck es DB + hashing.
  Si se agrega, sería para “resolver client+tenant config” en paralelo con cache,
  pero cuidado: no sumar latencia por goroutines al pedo.

Ideas de eficiencia/reutilización para el repaso global
-------------------------------------------------------
- Unificar parse de request (JSON/form + fallback PascalCase) en helper común.
- Unificar resolución de client/tenant (y cachearlo).
- Unificar issuer/key selection en un “IssuerResolver/KeySelector”.
- Unificar emisión de tokens en un componente reutilizable.
- Limpiar FS admin flows (definir 1 sola política coherente).
- Mover rate limiting a middleware semántico (o helper menos invasivo).
- Evitar múltiples hits a cpctx.Provider (traer client config 1 vez y reusar).

En resumen
----------
Este handler es el “centro neurálgico” del login password: parsea, rate-limitea, valida client, busca user, chequea password,
aplica políticas (email verification, user disabled), opcionalmente MFA, arma claims (scopes + RBAC + hook),
resuelve issuer/key por tenant y emite tokens (access + refresh persistido).
Está funcional, pero hoy tiene duplicación fuerte (token issuance), caminos FS admin inconsistentes,
y mucha lógica de orquestación que pide a gritos separarse en servicios/resolvers reutilizables.




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




auth_refresh.go — Refresh + Logout (rotación de refresh tokens) + refresh “admin stateless” por JWT

Este archivo en realidad contiene DOS handlers:
  1) NewAuthRefreshHandler  -> POST /v1/auth/refresh (renueva access + rota refresh)
  2) NewAuthLogoutHandler   -> POST /v1/auth/logout  (revoca un refresh puntual; idempotente)

Además, soporta un caso especial:
  - “Refresh token JWT” (stateless) para admins FS/globales: si el refresh parece JWT, lo valida y re-emite access+refresh JWT.

================================================================================
1) POST /v1/auth/refresh — NewAuthRefreshHandler
================================================================================

Qué hace (objetivo funcional)
-----------------------------
- Recibe (client_id, refresh_token) y un “tenant context” (tenant_id opcional o derivado del request).
- Valida el refresh token contra el storage (DB per-tenant) usando hash SHA-256 (hex).
- Si está OK:
    - Emite un NUEVO access token (JWT EdDSA)
    - Emite un NUEVO refresh token (rotación)
    - Revoca el refresh viejo
- En paralelo, arma claims:
    - tid, amr=["refresh"], scp=scopes del client
    - custom SYS claims (roles/perms/is_admin/etc) si el repo lo soporta
    - issuer efectivo según tenant (IssuerMode) y firma con key global o per-tenant (si IssuerModePath)

Entrada / salida
----------------
Request JSON:
  {
    "tenant_id": "<opcional>",   // ACEPTADO pero (en teoría) no debería ser fuente de verdad.
    "client_id": "<obligatorio>",
    "refresh_token": "<obligatorio>"
  }

Response JSON 200:
  {
    "access_token": "...",
    "token_type": "Bearer",
    "expires_in": <segundos>,
    "refresh_token": "..."
  }

Errores típicos:
- 400 missing_fields si falta client_id o refresh_token
- 401 invalid_grant si el refresh no existe / revocado / expirado
- 401 invalid_client si el client_id no matchea con el del refresh
- 500 si no hay TenantSQLManager / no se pueden obtener keys / etc
- “tenant db missing/error” con helpers/httpx wrappers

Flujo paso a paso (detallado)
-----------------------------
A) Validaciones HTTP básicas
   - Solo POST.
   - Content-Type set a JSON.
   - ReadJSON en RefreshRequest.
   - Trim: refresh_token y client_id.
   - Requiere ambos.

B) Resolver tenant (para ubicar el repo inicial)
   - tenantSlug se obtiene en orden:
       1) body.tenant_id
       2) helpers.ResolveTenantSlug(r)
   - Si sigue vacío => 400.
   - Llama a helpers.ResolveTenantSlugAndID(ctx, tenantSlug) pero ignora el resultado (solo “warmup”).
     ⚠️ Nota: ese resultado no se usa, y además el comentario dice “source of truth remains RT”.

C) Caso especial: refresh token con formato JWT (admin stateless)
   - Heurística: strings.Count(token, ".") == 2
   - jwt.Parse con keyfunc custom:
       - extrae kid del header
       - busca public key por KID en c.Issuer.Keys.PublicKeyByKID
   - Verifica claim token_use == "refresh"
   - Si es válido:
       - emite nuevo ACCESS JWT (aud="admin", tid="global", amr=["pwd","refresh"], scopes hardcode)
       - emite nuevo REFRESH JWT (token_use="refresh", aud="admin") con TTL refreshTTL
       - responde 200 con ambos
   - Si falla parse/valid => log y cae al flujo “DB refresh”.
   📌 Patrón: “Dual-mode token strategy” (stateless vs stateful). Está mezclado en el handler.

D) Flujo principal (refresh stateful via DB) — “RT como fuente de verdad”
   1) Hashear refresh_token:
        sha256(refresh_token) -> hex string
      Comentario: “alineado con store PG”.

   2) Abrir repo per-tenant (por tenantSlug resuelto “del contexto”):
        repo := helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
      Maneja:
        - tenant inválido -> 401 invalid_client
        - sin DB -> httpx.WriteTenantDBMissing
        - otros -> httpx.WriteTenantDBError

   3) Buscar refresh token por hash:
        rt := repo.GetRefreshTokenByHash(ctx, hashHex)
      Si no existe => 401 invalid_grant (“refresh inválido”).

   4) Validar estado del refresh:
        - si rt.RevokedAt != nil o now >= rt.ExpiresAt => 401 invalid_grant

   5) Validar client:
        - clientID := rt.ClientIDText
        - si request.ClientID no coincide => 401 invalid_client

   6) “RT define el tenant” (re-abrir repo si corresponde)
      - Si rt.TenantID (texto/uuid) no coincide con tenantSlug actual:
          slug2 := helpers.ResolveTenantSlugAndID(ctx, rt.TenantID)
          repo2 := OpenTenantRepo(ctx, slug2)
        y se pasa a usar repo2.
      ⚠️ Esto mezcla “slug” vs “uuid” de forma peligrosa:
         rt.TenantID se comenta como UUID, pero tenantSlug es slug. Compararlos con EqualFold puede fallar siempre.
         Igual la idea es correcta: “si el token pertenece a otro tenant, usar ese tenant”.

   7) Rechazar refresh si el usuario está deshabilitado
      - repo.GetUserByID(rt.UserID)
      - si DisabledAt != nil => 401 user_disabled
      ⚠️ Solo mira DisabledAt, no DisabledUntil (en login sí se mira DisabledUntil).

   8) Scopes
      - Intenta obtener scopes desde FS:
          helpers.ResolveClientFSBySlug(ctx, rt.TenantID, clientID)  (ojo: le pasa rt.TenantID)
        si falla => scopes=["openid"]
      - std claims: tid=rt.TenantID, amr=["refresh"], scp="..."

   9) Hook de claims + SYS namespace
      - applyAccessClaimsHook(...) modifica std/custom (hook tipo “policy engine”)
      - Luego calcula issuer efectivo:
          effIss = jwtx.ResolveIssuer(base, issuerMode, slug, override)
      - Agrega system claims (roles/perms) si el repo implementa RBAC:
          GetUserRoles/GetUserPermissions
        y luego:
          helpers.PutSystemClaimsV2(custom, effIss, u.Metadata, roles, perms)

   10) Selección de key para firmar (global vs per-tenant)
      - Si issuerMode del tenant == Path => usa ActiveForTenant(slugForKeys)
      - Si no => Active() global
      - Emite access token JWT (aud=clientID, sub=userID, iss=effIss)

   11) Rotación de refresh token (nuevo refresh + revocar viejo)
      - Camino “nuevo” preferido:
          CreateRefreshTokenTC(ctx, tenantID, clientID, userID, ttl) (devuelve raw token)
      - Si no existe:
          - genera raw opaque token
          - hashea a hex
          - intenta un método TC alternativo con firma rara:
              CreateRefreshTokenTC(ctx, tenantID, clientID, newHash, expiresAt, &oldID)
            ⚠️ Esto parece OTRA interfaz con mismo nombre pero distinta firma: peligro de confusión.
          - si no, usa legacy repo.CreateRefreshToken(...)
      - Finalmente revoca el viejo:
          repo.RevokeRefreshToken(ctx, rt.ID) (si falla, log y sigue)

   12) Respuesta
      - Cache-Control no-store + Pragma no-cache
      - 200 con access_token + refresh_token nuevo

Qué NO se usa / cosas raras (marcadas, sin decidir todavía)
-----------------------------------------------------------
- RefreshRequest.TenantID: comentario dice “aceptado por contrato; no usado para lógica”.
  En realidad SÍ se usa como primer intento para tenantSlug. Lo que “no se usa” es como fuente de verdad final:
  el refresh token encontrado define el tenant real.
- Se llama helpers.ResolveTenantSlugAndID(ctx, tenantSlug) y se descarta => es “dead-ish” (side effects?).
- Comparación rt.TenantID vs tenantSlug es dudosa (UUID vs slug). Riesgo de reabrir repo mal.
- Doble sistema de refresh “TC” con interfaces distintas y mismo nombre => deuda técnica fuerte.
- Inconsistencia de bloqueo de usuario (solo DisabledAt, no DisabledUntil).
- Mezcla de “admin refresh JWT” y “user refresh DB” en el mismo handler => alto acoplamiento.

Patrones / refactor propuesto (con ganas, para V2)
--------------------------------------------------
A) Separar responsabilidades (Single Responsibility + GoF Strategy)
   - Strategy: RefreshModeStrategy
       1) JWTStatelessRefreshStrategy (admin/global)
       2) DBRefreshStrategy (tenant/user)
     El handler solo decide cuál aplica y delega.

B) Service Layer (Application Service / Use Case)
   - RefreshService.Refresh(ctx, RefreshCommand) -> RefreshResult
   - LogoutService.Logout(ctx, LogoutCommand) -> error
   Esto te permite testear sin HTTP y reutilizar lógica desde otros flows (ej: device sessions).

C) Repository Port + Adapter
   - Definir una interfaz clara:
       RefreshTokenRepo {
         FindByHash(ctx, tenantSlug, hash) (*RefreshToken, error)
         Rotate(ctx, tokenID, tenantID, clientID, userID, ttl) (newRaw string, error)
         Revoke(ctx, tokenID) error
       }
     Luego adapters:
       - PostgresTenantRepoAdapter
       - (Opcional) LegacyAdapter
     Evitás los type assertions repetidos y las firmas “TC” duplicadas.

D) Factory para “issuer + signing key” (Factory Method / Abstract Factory)
   - IssuerResolver.Resolve(tenantSlug) -> effIss, mode
   - KeySelector.Select(mode, tenantSlug) -> (kid, priv)
   Sacás el if/else repetido.

E) Template Method para construir claims
   - buildBaseClaims(...)
   - enrichWithHook(...)
   - enrichWithRBACIfSupported(...)
   Así el flujo refresh/login comparten construcción de claims.

F) Seguridad / consistencia
   - Normalizar “tenant identity”:
       TenantRef {Slug, UUID}
     Y dejar UNA sola comparación (no slug vs uuid).
   - Asegurar que scope lookup usa slug correcto (no rt.TenantID si es UUID).
   - Hacer revocación/rotación transaccional si el store lo banca (ideal):
       rotate => create new + revoke old en una transacción.

G) Concurrencia (si aplica, sin inventar)
   - Acá no hace falta worker pool: es request/response puro.
   - Lo único concurrente útil sería:
       - paralelizar (con errgroup) lookup de tenant config + rbac roles/perms,
         pero solo si esos accesos son independientes y no agregan carga innecesaria.
     Ojo: primero claridad, después micro-optimización.

================================================================================
2) POST /v1/auth/logout — NewAuthLogoutHandler
================================================================================

Qué hace
--------
- Recibe refresh_token + client_id (+ tenant context)
- Busca el refresh por hash en el repo del tenant “contextual”
- Si no existe: devuelve 204 (idempotente, no filtra existencia)
- Si existe:
    - valida que client_id matchee
    - si el token pertenece a otro tenant, reabre repo para ese tenant
    - intenta revocar por hash con método TC (si existe)
    - devuelve 204

Notas importantes
-----------------
- Logout es idempotente: si el refresh no existe, igual 204.
- Acá NO se usa repo.RevokeRefreshToken(tokenID) de forma directa; usa un método TC opcional
  (Revoker por hash) y si no existe, no hace nada más (igual responde 204).
  Eso puede dejar tokens sin revocar si el store no implementa revoker TC.

Patrones/refactor para logout
-----------------------------
- Compartir el mismo “RefreshTokenResolver” del refresh:
    resolveByRawToken -> (repo, rt)
- Command + Service:
    LogoutService.RevokeRefresh(ctx, tenantRef, clientID, rawRefresh) -> error
- Strategy:
    - Revocar por ID (si ya encontraste rt.ID)
    - Revocar por hash (si el store lo prefiere)
  Elegís la estrategia por capacidades del repo.

Resumen corto
-------------
- auth_refresh.go mezcla: refresh stateless (admin JWT) + refresh stateful (DB) + logout puntual.
- La idea central es correcta (RT como fuente de verdad, rotación + revocación),
  pero está todo muy pegado con type assertions, comparaciones slug/uuid confusas y dos APIs “TC” distintas.
- En V2 lo más rentable es separar en servicios + strategies + factories para issuer/keys,
  y estandarizar TenantRef para no volver a sufrir el quilombo slug/uuid.




auth_register.go — Registro de usuario (tenant/client) + opción FS-admin + (opcional) auto-login + (opcional) email de verificación

Este handler implementa el endpoint de registro “password-based” y, dependiendo de flags/config, también:
  - Permite registrar “FS admins” globales (sin tenant/client) cuando FS_ADMIN_ENABLE=1
  - Puede hacer auto-login (emitir access + refresh) tras registrar (autoLogin=true)
  - Puede disparar email de verificación si el client lo requiere y hay emailHandler

================================================================================
Qué hace (objetivo funcional)
================================================================================
1) Valida request y normaliza email/ids.
2) Determina si es un registro normal (tenant/client) o un registro FS-admin global.
3) En registro normal:
   - Resuelve tenantSlug + tenantUUID
   - Resuelve client (prefer FS control-plane; fallback a DB si no está en FS)
   - Aplica “provider gating”: si el client declara providers y NO incluye "password", bloquea.
   - Aplica política de password blacklist (opcional).
   - Hashea password.
   - Crea usuario en el repo del tenant + crea identidad password (username/password).
   - Si autoLogin:
        - Emite access JWT (issuer efectivo según tenant)
        - Crea refresh token (prefer método TC si existe; fallback legacy)
        - (opcional) envía email de verificación si el client lo exige
        - Responde con tokens + user_id
     Si NO autoLogin:
        - Responde solo user_id

================================================================================
Entrada / salida
================================================================================
Request JSON:
  {
    "tenant_id": "<requerido salvo modo FS-admin>",
    "client_id": "<requerido salvo modo FS-admin>",
    "email": "<requerido>",
    "password": "<requerido>",
    "custom_fields": { ... }  // opcional
  }

Response JSON (según modo):
- FS-admin register: { user_id, access_token, token_type, expires_in }   (sin refresh)
- Normal sin autoLogin: { user_id }
- Normal con autoLogin: { user_id, access_token, token_type, expires_in, refresh_token }

Errores típicos:
- 400 missing_fields si falta email/password; o falta tenant/client cuando no está FS-admin enabled
- 401 invalid_client si tenant/client inválido, o password provider deshabilitado para ese client
- 409 email_taken si CreatePasswordIdentity devuelve core.ErrConflict
- 400 policy_violation si password está en blacklist
- 500/4xx tenant db missing/error via httpx helpers

================================================================================
Flujo paso a paso (detallado)
================================================================================

A) Validaciones HTTP + parseo
- Solo POST.
- ReadJSON sobre AuthRegisterRequest.
- Normaliza:
    email => trim + lower
    tenant_id/client_id => trim
- Requiere email + password siempre.

B) Rama “sin tenant_id/client_id”: modo FS-admin (si está habilitado)
- Si tenant_id=="" || client_id=="":
    - Si helpers.FSAdminEnabled():
        - helpers.FSAdminRegister(email, password)
        - Emite ACCESS token JWT (aud="admin", tid="global", amr=["pwd"], scopes fijos openid/profile/email)
        - Responde 200 con user_id + access_token (sin refresh)
    - Si NO FSAdminEnabled => 400 (tenant_id y client_id obligatorios)
📌 Patrón: Strategy/Branching por “tipo de sujeto” (tenant user vs global admin)
⚠️ Observación: esta rama repite bastante lógica de emisión JWT que aparece en login/refresh.

C) Registro normal: resolver tenant + abrir repo
- ctx := r.Context()
- tenantSlug, tenantUUID := helpers.ResolveTenantSlugAndID(ctx, req.TenantID)

- Resolver client:
    - Primero intenta FS: helpers.ResolveClientFSBySlug(ctx, tenantSlug, req.ClientID)
      Si OK => clientProviders/clientScopes vienen de FS.
    - Después abre repo del tenant (gating por DSN):
        helpers.OpenTenantRepo(ctx, c.TenantSQLManager, tenantSlug)
      Maneja:
        - tenant inválido => 401 invalid_client
        - (cualquier otro error) si FS admin enabled => fallback registra FS-admin (!!)
        - si IsNoDBForTenant => WriteTenantDBMissing
        - else WriteTenantDBError
  ⚠️ Ojo importante: el “fallback FS-admin” se activa ante CUALQUIER error de apertura de repo
     (excepto tenant inexistente). Eso incluye caídas temporales de DB: podés terminar creando admins
     por un problema infra. No digo que esté mal ahora, pero es una decisión heavy para revisar después.

- Si no había client en FS, intenta lookup en DB:
    repo.(clientGetter).GetClientByClientID(...)
  y toma providers/scopes desde ahí.
  Si el repo no implementa esa interfaz => invalid_client.
📌 Patrón: Adapter/Capability-based programming via type assertion (ok para compat, pero ensucia handler).

D) Provider gating (password login habilitado)
- Si clientProviders no está vacío:
    requiere que incluya "password"
  Si no => 401 invalid_client (“password login deshabilitado para este client”)
📌 Patrón: Policy Gate / Guard Clause.

E) Password policy (blacklist opcional)
- Obtiene path:
    - param blacklistPath
    - o env SECURITY_PASSWORD_BLACKLIST_PATH
- Si hay path:
    - password.GetCachedBlacklist(path)
    - si Contains(password) => 400 policy_violation
📌 Patrón: Cache (memoization) vía GetCachedBlacklist.
⚠️ Esto mete política en el handler; en V2 convendría mover a un PasswordPolicyService.

F) Crear usuario + identidad password
- phc := password.Hash(password.Default, req.Password)
- Construye core.User:
    TenantID=tenantUUID
    Email, EmailVerified=false
    Metadata={}
    CustomFields=req.CustomFields
    SourceClientID=&req.ClientID
- repo.CreateUser(ctx, u)
- repo.CreatePasswordIdentity(ctx, u.ID, email, verified=false, phc)
    - Si ErrConflict => 409 email_taken
📌 Patrón: Transaction Script (todo en un handler). En V2 ideal “RegisterUserUseCase”.

G) Si autoLogin == false
- Responde 200 con {user_id} y listo.

H) Si autoLogin == true: emitir tokens
- grantedScopes := clientScopes (copiado)
- std claims:
    tid=tenantUUID
    amr=["pwd"]
    acr="urn:hellojohn:loa:1"
    scp="..."
- custom := {}
- applyAccessClaimsHook(...) puede mutar std/custom

- Resolver issuer efectivo por tenant:
    effIss = jwtx.ResolveIssuer(base, issuerMode, slug, override)
- Selección de key:
    - si issuerMode == Path => Keys.ActiveForTenant(tenantSlug)
    - else => Keys.Active()
- Emite access token EdDSA.

- Refresh token:
    - genera rawRT (pero si existe CreateRefreshTokenTC, lo reemplaza con el retornado por TC)
    - Si repo implementa CreateRefreshTokenTC(ctx, tenantID, clientID, userID, ttl) => usa eso
    - else fallback legacy:
        hash := tokens.SHA256Base64URL(rawRT)   (nota: en refresh.go se usa hex; inconsistencia)
        repo.CreateRefreshToken(...)

- Headers no-store/no-cache.

I) Email verification (opcional, “soft”)
- Determina si el client requiere verificación:
    - cpctx.Provider.GetTenantBySlug(...)
    - cpctx.Provider.GetClient(...).RequireEmailVerification
- Si verificationRequired && emailHandler != nil:
    - construye tidUUID, uidUUID (Parse)
    - emailHandler.SendVerificationEmail(ctx, rid, tidUUID, uidUUID, email, redirect="", clientID)
  ⚠️ Está “soft fail”: ignora error (por diseño).
  ⚠️ El rid se saca de w.Header().Get("X-Request-ID") (probablemente vacío si nadie lo setea antes).

J) Responde 200 con AuthRegisterResponse completo.

================================================================================
Qué NO se usa / cosas raras (marcadas)
================================================================================
- Func contains(ss, v) está DECLARADA pero NO se usa en este archivo.
- Import "context" se usa (para clientGetter interface, etc) OK.
- Import "jwtx" se usa.
- Variable fsClient se guarda pero prácticamente no se usa luego (más que para “haveFSClient” y scopes/providers).
- tokens.GenerateOpaqueToken(32) se llama aunque luego se pisa con CreateRefreshTokenTC si existe.
  (micro-ineficiente; no rompe nada).
- Inconsistencia de hashing de refresh en fallback:
    - acá: SHA256Base64URL(rawRT)
    - en refresh.go/logout: SHA256 hex
  Esto es re importante a revisar globalmente después (vos ya venís viendo esa mezcla TC/legacy).
- Fallback FS-admin cuando falla abrir repo (por cualquier error) puede ser riesgoso.

================================================================================
Patrones (GoF / arquitectura) y cómo lo refactorizaría en V2
================================================================================

1) Strategy (GoF) para “modo de registro”
- RegisterStrategy:
    - FSAdminRegisterStrategy
    - TenantUserRegisterStrategy
  El handler decide y delega. Te limpia el “if tenant/client missing” + el fallback raro.

2) Application Service / Use Case (Service Layer)
- RegisterUserService.Register(ctx, cmd) -> result
  cmd incluye: tenantRef, clientID, email, password, customFields, autoLogin flag.
  Eso te separa:
    - Validación/políticas
    - Persistencia
    - Emisión de tokens
    - Side-effects (email)

3) Template Method para “emitir access token”
- Hoy la emisión JWT se repite en login/refresh/register (y admin variants).
- Crear:
    TokenIssuer.IssueAccess(ctx, params) -> token string, exp time.Time
  internamente:
    - resolve issuer
    - select key
    - build claims std/custom
    - sign
  Así dejás un solo camino.

4) Policy objects (GoF-ish) / Chain of Responsibility para validaciones
- PasswordPolicyChain:
    - BlacklistPolicy
    - MinLengthPolicy (si existe)
    - StrengthPolicy (si existe)
- ClientAuthPolicy:
    - ProviderGatingPolicy (password enabled)
    - EmailVerificationPolicy (si aplica al login/register)
  Cada policy retorna error tipado => el handler traduce a HTTP.

5) Ports & Adapters (Clean-ish)
- Definir interfaces “claras”:
    ClientCatalog (FS/DB)
    UserRepository
    IdentityRepository
    RefreshTokenRepository
  En vez de type assertions “si implementa X”.
  En V1 podés mantener adapters que envuelvan repo y expongan las capacidades.

6) Observer / Domain Events (GoF: Observer)
- En vez de que el handler “mande email”:
    - Emitís evento UserRegistered{tenant, user, client, verificationRequired}
    - Subscriber: EmailVerificationSender
  Así, si mañana querés colgar auditoría, métricas, welcome email, etc, no tocás el handler.

7) Consistencia de tenantRef (slug + uuid)
- Formalizar un TenantRef:
    type TenantRef struct { Slug string; ID uuid.UUID }
  y listo. Deja de haber comparaciones raras y parseos sueltos.

================================================================================
Resumen
================================================================================
- auth_register.go hace: register tenant-user con password (+ blacklist + provider gating), crea identidad,
  y opcionalmente auto-login + refresh + email verification; y también permite registrar FS-admin global.
- Está cargado de responsabilidades mezcladas (token issuance, policies, repo opening, email side-effect),
  con repetición respecto a login/refresh y varias “compat branches” (FS vs DB).
- Para V2, lo más rentable es separar por Strategy (FS-admin vs tenant user), extraer TokenIssuer compartido,
  y mover políticas + side effects (email) a services/events para limpiar el handler.




claims_hook.go — Hook de claims (Access/ID) + merge “seguro” para no romper invariantes del token

Este archivo NO es un handler HTTP en sí, pero es una pieza clave que usan varios handlers
(login/refresh/register/oauth token) para “dejar enchufar” lógica externa (policies / reglas / scripts)
que agregue claims a los tokens sin permitir que pisen campos críticos.

================================================================================
Qué hace (objetivo funcional)
================================================================================
1) Define un set de claims “reservadas” (reservedTopLevel) que NO se pueden sobreescribir desde policies/hooks,
   porque romperían invariantes de seguridad y compat OIDC:
   - iss/sub/aud/exp/iat/nbf/jti/typ
   - at_hash/azp/nonce/tid
   - scp/scope/amr (se gestionan internamente por el servidor)

2) Implementa mergeSafe(dst, src, prevent):
   - Copia claves desde src a dst
   - NO copia si:
       a) la clave está en la lista prevent (comparación case-insensitive)
       b) dst ya tenía esa clave (no sobreescribe)
   - O sea: es “merge no destructivo” + “denylist” de claves.

3) Expone dos helpers:
   - applyAccessClaimsHook(...): para Access Token
   - applyIDClaimsHook(...): para ID Token
   Ambos:
     - Ejecutan c.ClaimsHook si está seteado
     - Fusionan addStd/addExtra en std/custom (o std/extra) usando mergeSafe
     - Bloquean explícitamente la inyección del namespace “del sistema” (sysNS) dentro de custom/top-level

================================================================================
Cómo se usa (en el resto del código)
================================================================================
- Handlers construyen mapas base:
    std   => claims estándar que el server controla (tid, amr, acr, scp, etc.)
    custom/extra => claims adicionales
- Luego llaman:
    std, custom = applyAccessClaimsHook(...)
  para que un hook externo agregue cosas sin romper lo importante.

Esto permite personalización por tenant/client/user (ej: “meter plan=pro”, “feature_flags”, “region”, etc.)
sin tocar el core del emisor de tokens.

================================================================================
Detalles importantes / invariantes de seguridad
================================================================================
A) reservedTopLevel
- Protege claims críticas y las que el server quiere controlar:
  - “amr” (métodos de autenticación), “scp/scope” (scopes) son sensibles.
  - “tid” también: cambiarlo rompe multi-tenancy.

B) Bloqueo del System Namespace (sysNS)
- sysNS = claims.SystemNamespace(c.Issuer.Iss)
  Esto parece ser el “namespace” donde el server mete claims internas (roles/perms/is_admin, etc.)
- applyAccessClaimsHook hace 2 defensas:
  1) En top-level: agrega sysNS a prevent (para que no lo inyecten “arriba”)
  2) En custom: delete(addExtra, sysNS) antes de mergear
  => evita que un hook se haga pasar por el sistema y fuerce roles/permisos, etc.

C) mergeSafe NO sobreescribe
- Esto define una regla clara: el core gana.
- Si el hook quiere cambiar algo ya seteado, no puede. (bien para seguridad, pero a veces limita casos de uso)

================================================================================
Patrones (GoF / arquitectura) que aparecen acá
================================================================================
1) Hook / Plugin (arquitectura)
- c.ClaimsHook funciona como punto de extensión (plugin).
- Muy en la línea de “Inversion of Control”: el core llama al hook, no al revés.

2) Chain of Responsibility (potencial)
- Hoy hay un solo hook (ClaimsHook). Pero la idea ya calza perfecto con CoR:
    - multiples policies/hooklets ejecutándose en orden
    - cada una agrega claims y el mergeSafe aplica reglas
  En V2, podrías permitir []ClaimsPolicy y componerlas.

3) Decorator (potencial)
- Podés pensar applyAccessClaimsHook como decorador del “claims builder”.
- Envuelve los mapas base y añade comportamiento (merge + bloqueos).

4) Guard Clauses / Fail-closed
- Si hook tira error => se ignora y se devuelve std/custom tal cual.
  (Ojo: esto es fail-open respecto a “enriquecimiento”, pero fail-closed respecto a seguridad: no agrega nada.)

================================================================================
Cosas a revisar / mejoras para V2 (sin decidir todavía)
================================================================================
1) Eficiencia micro
- slices.ContainsFunc dentro del loop por cada clave => O(n*m) (n=claves, m=prevent).
  Para pocos claims no importa, pero se puede:
    - precomputar un map[string]struct{} con prevent en lowercase
    - lookup O(1)
  (No es urgente, pero queda prolijo.)

2) Normalización de claves
- mergeSafe compara prevent con lk := strings.ToLower(k) pero después usa dst[k] “tal cual”.
  Esto puede permitir duplicados raros por case:
    dst["Foo"]=1 y src["foo"]=2 => no lo pisa porque “exists” mira k exacto, no lower.
  En JWT claims suele ser lowercase, pero no siempre.
  En V2, conviene:
    - o normalizar siempre a lower (excepto si querés mantener formato)
    - o chequear existencias case-insensitive también.

3) Control fino por “Kind”
- reservedTopLevel se usa para ambos tokens, pero en OIDC hay claims que aplican distinto.
  Capaz en V2:
    - reservedAccess vs reservedID
    - allowlist por tipo de token

4) Observabilidad
- Si el hook falla (err != nil) hoy se ignora sin log.
  En V2, un log debug/trace ayuda, sin romper requests.

================================================================================
Qué NO se usa / cosas a marcar
================================================================================
- Este archivo NO expone handlers HTTP; sólo helpers internos (ok).
- Import "strings", "slices" sí se usan.
- reservedTopLevel es var global (ok) y se consume en ambos apply*.

================================================================================
Resumen
================================================================================
- claims_hook.go es el “mecanismo de extensión” para agregar claims a Access/ID tokens.
- Lo hace de forma segura: no permite pisar claims críticas ni inyectar el namespace del sistema.
- Es un buen candidato a evolucionar a una cadena de policies (Chain of Responsibility) y a optimizar
  la lógica de merge (map de prevent + case-insensitive existence) en V2.





cookieutil.go — Helpers para cookies de sesión (SameSite/Secure/Domain/TTL) [NO es handler HTTP]

Qué es este archivo
-------------------
Este archivo NO implementa endpoints HTTP. Es una caja de herramientas para construir cookies
de sesión de manera consistente y con flags de seguridad razonables, especialmente para el flujo
de “cookie session” que se usa alrededor de `/oauth2/authorize` (login/logout de sesión) y
cualquier otro handler que necesite setear/borrar una cookie.

En concreto expone:
- parseSameSite(s string) http.SameSite
- BuildSessionCookie(name, value, domain, sameSite string, secure bool, ttl time.Duration) *http.Cookie
- BuildDeletionCookie(name, domain, sameSite string, secure bool) *http.Cookie

================================================================================
Qué hace (objetivo funcional)
================================================================================
1) Normaliza/configura SameSite
- Convierte strings de config (`"", "lax", "strict", "none"`, case-insensitive) a `http.SameSite`.
- Default: Lax.
- Si recibe un valor desconocido:
  - Loguea warning
  - Vuelve a Lax (fail-safe “más permisivo” que Strict, pero usualmente correcto para compat).

2) Construye cookies de sesión con defaults seguros
- `HttpOnly=true` (protege contra lectura desde JS ante XSS).
- `Path="/"` (aplica a todo el sitio/API).
- `Secure` y `SameSite` se setean según config.
- `Domain` se setea sólo si viene no vacío (evita setear Domain accidentalmente).
- TTL:
  - setea `Expires = now + ttl` (en UTC)
  - setea `MaxAge = int(ttl.Seconds())`

3) Construye cookies de “borrado” (logout)
- Devuelve una cookie con:
  - `Expires` en el pasado
  - `MaxAge = -1`
  - Mismos `Name/Domain/SameSite/Secure/HttpOnly/Path` que la de sesión
- Objetivo: que el user-agent la sobreescriba correctamente (clave para que “borrar” funcione).

================================================================================
Cómo se usa (en el resto del package)
================================================================================
- Este helper se usa típicamente por handlers de sesión (por ejemplo `session_login.go` y `session_logout.go`)
  para:
  - Setear cookie al autenticar una sesión (value = sessionID/token de sesión)
  - Borrarla en logout

Importante: este archivo NO decide el nombre de la cookie ni el dominio; eso viene de config
y/o del wiring en el handler que la usa.

================================================================================
Flujo paso a paso (por función)
================================================================================
parseSameSite(s)
1) Trim + strings.ToLower
2) Mapea:
   - "" | "lax"    => Lax
   - "strict"      => Strict
   - "none"        => None
3) Si es "none" y `secure=false`:
   - NO lo corrige (no fuerza Secure)
   - deja un warning contextual (para no romper localhost sin HTTPS)
4) Valor desconocido:
   - log warning
   - retorna Lax

BuildSessionCookie(...)
1) Obtiene SameSite con parseSameSite
2) Si SameSite=None y secure=false:
   - log warning (“algunos navegadores pueden rechazarla”)
3) Calcula timestamps:
   - now := time.Now().UTC()
   - exp := now.Add(ttl)
4) Construye `http.Cookie` con:
   - Name/Value, Path="/"
   - Domain="" (y se asigna si `domain != ""`)
   - Expires=exp, MaxAge=int(ttl.Seconds())
   - Secure=secure, HttpOnly=true, SameSite=ss
5) Retorna la cookie lista para `http.SetCookie(w, cookie)`

BuildDeletionCookie(...)
1) Obtiene SameSite con parseSameSite
2) Construye cookie con:
   - Value=""
   - Expires=time.Unix(0,0).UTC()
   - MaxAge=-1
   - Secure/HttpOnly/SameSite/Domain/Path alineados
3) Retorna cookie lista para `http.SetCookie`

================================================================================
Dependencias reales
================================================================================
- stdlib:
  - `net/http` (tipo http.Cookie y http.SameSite)
  - `time` (TTL y expiración)
  - `strings` (normalización)
  - `log` (warnings)
- No usa `app.Container`, `Store`, `TenantSQLManager`, `cpctx.Provider`, `Issuer`, `Cache`, etc.

================================================================================
Seguridad / invariantes importantes
================================================================================
- `HttpOnly=true`:
  - Buen default para cookies de sesión (mitiga exfiltración vía JS ante XSS).
- `SameSite=None`:
  - En navegadores modernos, suele requerir `Secure=true`.
  - Este helper NO fuerza Secure (para no romper ambientes sin HTTPS), sólo loguea warning.
  - Riesgo: en prod, si alguien configura `SameSite=None` y `secure=false`, la cookie puede ser ignorada
    por el browser y el login por cookie “parece” fallar.
- `Domain`:
  - Se setea sólo si viene explícito. Esto evita errores comunes (cookies que no matchean host).
- TTL:
  - `MaxAge=int(ttl.Seconds())`: si `ttl < 1s`, MaxAge puede quedar 0 (comportamiento inesperado).
  - `Expires` se calcula siempre y se setea (UTC), lo cual ayuda a compat.

================================================================================
Patrones detectados (GoF / arquitectura)
================================================================================
- Factory / Builder (GoF-ish):
  - `BuildSessionCookie` y `BuildDeletionCookie` son “constructores” que encapsulan defaults y reglas.
- Guard + Observability:
  - Warnings al detectar combinaciones peligrosas (SameSite=None sin Secure).

No hay concurrencia ni estados globales (funciones puras salvo logging).

================================================================================
Cosas no usadas / legacy / riesgos
================================================================================
- No hay imports ni variables “muertas” en este archivo.
- Riesgo de “config inválida silenciosa”:
  - Valores SameSite desconocidos caen a Lax (con warning). Si el logging no se observa, queda oculto.
- Riesgo de compat (SameSite=None sin Secure):
  - Se advierte pero no se bloquea; en prod convendría tratarlo como error de config.

================================================================================
Ideas para V2 (sin decidir nada)
================================================================================
1) Convertir a “CookiePolicy / CookieService”
- En vez de helpers sueltos, centralizar en un componente con opciones:
  - `CookieOptions{ Name, Domain, SameSite, Secure, TTL, Path }`
  - `BuildSessionCookie(opts, value)` y `BuildDeletionCookie(opts)`
- Beneficio: una única fuente de verdad para cookies (session, csrf si aplica, mfa_trust, etc.).

2) Validación de config (fail-fast)
- En bootstrap v2, validar:
  - `SameSite=None` => exigir `Secure=true` salvo modo dev/local explícito.
  - TTL mínimo razonable.

3) Consistencia cross-handlers
- Asegurar que login/logout usen exactamente la misma política (name/domain/samesite/secure),
  porque la cookie de borrado debe matchear para que el browser realmente la elimine.

4) Observabilidad más clara
- En v2, preferible log estructurado (nivel warn + request_id/contexto de entorno) o validación
  de config con error explícito antes de levantar el server.

================================================================================
Resumen
================================================================================
- cookieutil.go es un helper (no handler HTTP) que construye cookies de sesión y de borrado con flags
  de seguridad razonables, y normaliza SameSite desde strings de config.
- Es un buen candidato a convertirse en una “CookiePolicy” central en V2, con validación fail-fast
  para evitar combinaciones que rompen en browsers (SameSite=None sin Secure).




csrf.go — Emisión de CSRF token (double-submit) vía cookie + JSON

Qué hace este handler
---------------------
Este archivo implementa un único endpoint para emitir un CSRF token efímero que se usa como
“double-submit token” para proteger endpoints basados en cookies (especialmente el login de sesión).

Endpoint que maneja
-------------------
- GET /v1/csrf
	- Setea una cookie con el CSRF token
	- Devuelve el mismo token en JSON: {"csrf_token":"..."}
	- Headers anti-cache: Cache-Control: no-store, Pragma: no-cache

Este token se valida (cuando está habilitado) con el middleware `RequireCSRF` (en
`internal/http/v1/middleware.go`), que compara:
- Header (default `X-CSRF-Token`, configurable)
- Cookie (default `csrf_token`, configurable)
Ambos deben existir y matchear exactamente.

Cómo se usa (en el wiring actual)
--------------------------------
- En `cmd/service/v1/main.go` se construye el handler:
		`handlers.NewCSRFGetHandler(getenv("CSRF_COOKIE_NAME", "csrf_token"), 30*time.Minute)`
	y se registra en el mux como:
		`GET /v1/csrf`

- El enforcement de CSRF se aplica (opcionalmente) sólo a `POST /v1/session/login`:
	- Si `CSRF_COOKIE_ENFORCED=1`, `sessionLoginHandler` se envuelve con
		`httpserver.RequireCSRF(csrfHeader, csrfCookie)`.
	- Si hay Bearer auth, el middleware saltea CSRF (porque no es un flujo cookie).

Flujo paso a paso (GET /v1/csrf)
--------------------------------
1) Validación HTTP
	 - Sólo acepta método GET.
	 - Si no es GET: 405 Method Not Allowed con `httpx.WriteError`.

2) Generación del token
	 - Genera 32 bytes aleatorios (`crypto/rand.Read`) y los serializa a hex.
	 - El resultado es un string de 64 chars (hex) típicamente.
	 - Nota: el error de `rand.Read` se ignora (best-effort).

3) Seteo de cookie
	 - Cookie name: configurable (default `csrf_token`).
	 - TTL: configurable (default 30m).
	 - Atributos actuales:
		 - SameSite=Lax
		 - HttpOnly=false  (intencional: el frontend lo lee para mandarlo en el header)
		 - Secure=false    (hardcode)
		 - Path=/
		 - Expires=now+ttl

4) Response
	 - `Cache-Control: no-store` + `Pragma: no-cache`
	 - 200 OK con JSON: {"csrf_token": tok}

Dependencias reales
-------------------
- stdlib:
	- `crypto/rand`, `encoding/hex`, `net/http`, `time`
- internas:
	- `internal/http/v1` como `httpx` para `WriteError` y `WriteJSON`

No usa `app.Container`, `Store`, `TenantSQLManager`, `cpctx.Provider`, `Issuer`, `Cache`, etc.

Seguridad / invariantes
-----------------------
- Patrón implementado: “Double-submit cookie”
	- El CSRF token vive en cookie (para el browser) y también se devuelve al JS (para header).
	- El server valida igualdad cookie/header en requests sensibles *basados en cookies*.

- SameSite=Lax
	- Reduce CSRF en muchos casos por default, pero NO reemplaza la validación double-submit.

- HttpOnly=false (por diseño)
	- Necesario para que el frontend pueda leerlo y reenviarlo.
	- Implica que ante XSS el token es legible; CSRF no protege XSS, así que es esperado.

- Secure=false (hardcode)
	- En producción con HTTPS esto debería ser Secure=true.
	- Tal cual está, depende de que el deployment acepte cookie sin Secure (y/o que haya otros controles).

- Token no firmado
	- Es un random bearer token; la “validación” es compararlo contra lo que el server mismo setea en cookie.
	- No está atado a usuario/tenant; el scope es “sesión del navegador” (cookie jar).

Patrones detectados (GoF / arquitectura)
----------------------------------------
- Factory / Constructor de handler:
	- `NewCSRFGetHandler(cookieName, ttl)` devuelve un `http.HandlerFunc` configurado.
- Security pattern:
	- Double-submit CSRF token.

No hay concurrencia (goroutines/locks) ni estado compartido; es stateless salvo el seteo de cookie.

Cosas no usadas / legacy / riesgos
---------------------------------
- `rand.Read` ignora el error:
	- Si fallara (muy raro), el token podría ser predecible/empty dependiendo del contenido del buffer.
	- Sería mejor fail-closed con 500 si no se pudo generar entropía.

- `Secure=false` hardcode:
	- Riesgo de seguridad en prod (cookie viaja por HTTP si está permitido).
	- También puede causar inconsistencias si el sitio corre sólo en HTTPS y el browser exige Secure
		bajo ciertas políticas.

- No setea `Max-Age`:
	- Sólo usa `Expires`; suele ser suficiente, pero algunos clientes se comportan distinto.

Ideas para V2 (sin decidir nada)
--------------------------------
1) CSRFService / CookiePolicy compartida
	 - Unificar criterios de cookie (Secure, SameSite, Domain, TTL) con lo que ya existe en `cookieutil.go`.
	 - Evitar hardcodes (especialmente Secure).

2) Detección HTTPS/proxy
	 - Calcular `Secure` en base a `r.TLS` / `X-Forwarded-Proto` (similar a `isHTTPS()` del middleware)
		 o validar por config de entorno.

3) Endpoints y enforcement consistentes
	 - Centralizar qué endpoints requieren CSRF en el router/middleware, no en `main.go`.
	 - Mantener el “skip si Bearer” (cookie-flow vs token-flow) como regla explícita.

4) Mejoras de robustez
	 - Manejar error de `rand.Read` (fail-closed con 500).
	 - Definir envelope de error consistente (ya se usa `httpx.WriteError`).

Guía de “desarme” en capas (para comprensión y mantenibilidad)
--------------------------------------------------------------
- DTO/transport:
	- Response: `{csrf_token: string}` (simple; podría formalizarse como struct en V2).
- Controller:
	- Validación de método + orquestación de emisión.
- Service:
	- Generación del token + política de expiración + armado de cookie.
- Infra/util:
	- Fuente de entropía (rand) + helpers de cookies (policy).

Resumen
-------
- `csrf.go` emite un CSRF token para el patrón double-submit y lo entrega por cookie + JSON.
- Se usa para proteger el flujo de login de sesión basado en cookies cuando `CSRF_COOKIE_ENFORCED=1`.
- Hay oportunidades claras para V2: unificar policy de cookies, evitar `Secure=false` hardcode y
	manejar fail-closed si la entropía falla.




Email Flows Wiring (email_flows_wiring.go)
───────────────────────────────────────────────────────────────────────────────
Qué es esto (y qué NO es)
- NO es un handler HTTP “de endpoint” directo.
- Es el “wiring / composition root” de los flows de email (verify email / forgot password / reset password):
  arma dependencias, adapta interfaces y devuelve handlers listos (`http.Handler`) para registrar en el router.

Qué construye / expone
- Función pública: `BuildEmailFlowHandlers(...)` devuelve:
  - `verifyStart`   -> handler para iniciar verificación de email (genera token + envía mail).
  - `verifyConfirm` -> handler para confirmar verificación (consume token).
  - `forgot`        -> handler para “olvidé mi contraseña” (genera token + envía mail).
  - `reset`         -> handler para resetear password (consume token; opcionalmente auto-login).
  - `efHandler`     -> instancia de `EmailFlowsHandler` ya configurada (para tests/uso interno).
  - `cleanup`       -> función de cierre (solo útil si se abrió conexión manual).
  - `err`           -> error de wiring (si faltan templates/DSN/etc.).

Cómo se usa (enrutamiento típico)
- En el bootstrap de HTTP (main/router):
  - `verifyStart, verifyConfirm, forgot, reset, _, cleanup, err := BuildEmailFlowHandlers(...)`
  - Registrar endpoints: `router.Handle("/v1/auth/verify-email/start", verifyStart)` etc. (según routes reales).
  - `defer cleanup()` (si corresponde).
- Los handlers concretos viven en `EmailFlowsHandler` (ver `email_flows.go`), acá solo se arma todo.

Flujo del wiring (paso a paso)
1) Templates:
   - `email.LoadTemplates(cfg.Email.TemplatesDir)`
   - Si falla, aborta el wiring (no tiene sentido exponer flows sin templates).
2) Política de password:
   - Construye `password.Policy` desde `cfg.Security.PasswordPolicy`.
   - Se pasa al handler para validar passwords en reset/registro (según flujo).
3) Rate limiting opcional (Redis):
   - Si `cfg.Rate.Enabled` y `cfg.Cache.Kind == "redis"`:
     - Crea `redis.Client`.
     - Crea `rate.NewRedisLimiter(...)`.
     - Lo envuelve con `flowsLimiterAdapter` para cumplir `RateLimiter`.
   - Si no, queda `flowsLimiter` nil (=> el adapter permite todo).
4) DB ops / pool reutilizable:
   - Intenta reusar el pool si `c.Store` expone `Pool() *pgxpool.Pool`.
     - Objetivo: evitar “too many connections”.
   - Fallback (legacy/riesgo): abre `pgx.Connect(ctx, cfg.Storage.DSN)` si el store no es pool-compatible.
     - En este caso `cleanup()` cierra esa conexión.
5) Stores internos de flows:
   - `TokenStore`: `store.NewTokenStore(dbOps)` (tokens de verificación/reset).
   - `UserStore`: `&store.UserStore{DB: dbOps}` (no usa constructor).
6) Adaptadores (puente entre flows y el resto del sistema):
   - `redirectValidatorAdapter`:
     - Valida `redirect_uri` contra client catalog (SQL) o fallback FS (control-plane).
   - `tokenIssuerAdapter`:
     - Emite access/refresh post-reset (auto-login) usando issuer + persistencia de refresh (preferencia TC).
   - `currentUserProviderAdapter`:
     - Extrae usuario/tenant/email desde Bearer JWT (para flows que lo necesiten).
7) Construye `EmailFlowsHandler` con todo lo anterior:
   - Inyecta SenderProvider + templates + policy + limiter + base URLs + TTLs + debug flags
   - Inyecta TenantMgr + Provider para resolver tenant/client (FS/DB).
8) Exporta handlers concretos:
   - `verifyStart = http.HandlerFunc(ef.verifyEmailStart)` etc.

Dependencias reales (qué usa y por qué)
- `config.Config`: fuente de truth de templates, cache/rate, política de password, TTLs, base URL.
- `app.Container`:
  - `c.Store`: repo/store global (catálogo clientes, users, roles/perms en algunos casos).
  - `c.TenantSQLManager`: abrir store per-tenant para persistir refresh tokens (modo TC).
  - `c.SenderProvider`: envío real de emails (abstrae SMTP/Sendgrid/etc).
  - `c.Issuer`: emite JWT (y resuelve keys por `kid`).
  - `c.ClaimsHook`: opcional, inyecta claims en access tokens.
- `cpctx.Provider`: control-plane FS (tenants/clients) para fallback y resoluciones.
- `store.TokenStore`, `store.UserStore`: stores “de flows” sobre `DBOps`.

Seguridad / invariantes que se están cuidando
- Redirect URI validation:
  - `redirectValidatorAdapter.ValidateRedirectURI(...)` busca `redirectURIs` del client:
    - Preferencia SQL (`repo.GetClientByClientID`), fallback FS (`cpctx.Provider`).
  - Verifica tenant match (`clientTenantID` vs `tenantID`) y exact match del redirect.
  - Si falla, loggea warnings (evita open redirect).
- Emisión de tokens en reset:
  - `tokenIssuerAdapter.IssueTokens(...)` valida que el client pertenezca al tenant.
  - Usa `applyAccessClaimsHook` + `helpers.PutSystemClaimsV2` para “SYS namespace” controlado.
  - Emite access con `c.Issuer.IssueAccess(...)`.
  - Emite refresh preferentemente vía `CreateRefreshTokenTC` (hash SHA256+hex interno).
  - Setea `Cache-Control: no-store` / `Pragma: no-cache`.
- Auth provider:
  - `currentUserProviderAdapter` parsea Bearer JWT:
    - valida método EdDSA y `issuer` (estricto) con `jwtv5.WithIssuer(a.issuer.Iss)`.
    - keyfunc resuelve por `kid` (JWKS/keystore).
  - Devuelve UUIDs desde claims `sub` y `tid`.

Patrones detectados (GoF + arquitectura)
- Adapter (GoF):
  - `flowsLimiterAdapter`, `redirectValidatorAdapter`, `tokenIssuerAdapter`, `currentUserProviderAdapter`
  - Todos “adaptan” contratos esperados por `EmailFlowsHandler` a implementaciones reales (rate limiter, repo, issuer, jwt parser).
- Facade / Composition Root:
  - `BuildEmailFlowHandlers` funciona como fachada de inicialización: arma y retorna handlers listos.
- Strategy (ligera):
  - “Estrategia” de storage: reusar pool vs abrir conexión manual.
  - “Estrategia” de validación client catalog: SQL primero, FS fallback.
  - “Estrategia” de refresh persistence: TC preferido, legacy fallback.
- Ports & Adapters (arquitectura):
  - `EmailFlowsHandler` consume puertos (`Redirect`, `Issuer`, `Auth`, `Limiter`) y acá se enchufan adaptadores concretos.

Cosas no usadas / legacy / riesgos (marcar sin decidir)
- `rid := w.Header().Get("X-Request-ID")`:
  - OJO: si el request-id lo setea middleware, perfecto. Si no, puede venir vacío (no rompe, pero log pierde trazabilidad).
- `goto proceed`:
  - Funciona, pero es un smell para legibilidad (podría ser un early-return + función helper).
- `redirectValidatorAdapter` depende de `cpctx.Provider` pero no checkea nil antes de usarlo en fallback:
  - Hoy se asume inicializado; si `cpctx.Provider` es nil y el SQL lookup falla, podría panickear.
- `tokenIssuerAdapter.IssueTokens` usa `ti.c.Store.GetUserByID` para metadata/RBAC:
  - En multi-tenant, “user por tenant DB” vs “global store” puede quedar inconsistente (depende cómo está modelado `Store`).
  - Hay mezcla: refresh se intenta persistir en tenant store, pero user/roles se leen del store global.
- Imports:
  - En este archivo se usan todos los imports listados (no veo “(No se usa)” obvio), pero ojo con `jwtx`:
    - Se usa solo para `currentUserProviderAdapter` (`issuer *jwtx.Issuer`), ok.
- Rate limiter:
  - `flowsLimiterAdapter.Allow` usa `context.Background()` (no request context):
    - Para rate limiting está bien (evita cancelaciones), pero si querés trazabilidad/timeout, podría pasar ctx request.

Ideas para V2 (sin decidir nada, solo guía de desarme en capas)
1) “DTO / Contracts”
   - Definir interfaces explícitas en un paquete `emailflows/ports`:
     - `RateLimiter`, `RedirectValidator`, `TokenIssuer`, `CurrentUserProvider`
   - Evitar que el handler conozca detalles de Redis/PGX/JWT.
2) “Service”
   - Crear `EmailFlowsService` (dominio) que contenga reglas:
     - generar tokens, validar TTL, aplicar policy, disparar mails, etc.
   - `EmailFlowsHandler` queda como controller HTTP liviano.
3) “Controller (HTTP)”
   - `email_flows.go` debería tener solo parseo/validación HTTP + llamada al service + response mapping.
   - Centralizar seteo de headers (`no-store`, content-type) en helpers.
4) “Infra / Clients”
   - `RedisLimiterClient` en infra/cache.
   - `ClientCatalog` (SQL/FS) como Strategy: `SQLClientCatalog` + `FSClientCatalog` + `CompositeCatalog`.
   - `RefreshTokenRepository` (TC/legacy) encapsulado.
5) “Builder / Wiring”
   - Mantener un único composition root:
     - `BuildEmailFlowHandlers` debería:
       - NO abrir conexiones si ya hay pool (idealmente siempre inyectar pool desde afuera).
       - Resolver `cpctx.Provider` nil-safe.
6) Patrones recomendables para V2
   - Composite + Strategy:
     - Para catálogo de clients (SQL + FS) y resolución de tenant/slug/uuid.
   - Template Method:
     - Para “armado de token response” repetido (access + refresh + headers).
   - Chain of Responsibility:
     - Para validaciones del flow (redirect ok -> token ok -> policy ok -> send ok), evitando ifs enormes en handlers.
   - Circuit Breaker / Bulkhead (si el mail provider es externo):
     - No está hoy, pero si el sender falla, conviene aislar y degradar sin tumbar todo.
   - Concurrencia:
     - No se usa explícitamente acá.
     - (Opcional) En envío de emails: si en algún flow se dispara “async send”, usar worker pool / queue,
       pero solo si el sistema lo necesita (hoy parece sync y controlado).

Resumen
- Este archivo arma el “pack” de handlers de email flows y sus dependencias.
- Implementa varios Adapters para desacoplar EmailFlowsHandler de Redis/DB/JWT/Issuer/ClientCatalog.
- Hay un enfoque claro de fallback (SQL -> FS, TC -> legacy) y de seguridad (redirect validation, no-store, issuer/keys).
- Para V2: separar puertos/servicios/infra y centralizar estrategias (catalog, refresh persistence) para reducir mezcla y duplicación.





Email Flows Wrappers (email_flows_wrappers.go)
───────────────────────────────────────────────────────────────────────────────
Qué es esto (y qué NO es)
- NO es un handler “nuevo” con lógica propia.
- Es un set de “wrappers/fábricas chiquitas” que devuelven `http.HandlerFunc` apuntando a métodos
  de `EmailFlowsHandler`.
- Sirve para estandarizar cómo se exponen los handlers (y evitar referenciar métodos privados
  directamente desde el wiring/router en algunos lugares).

Qué expone
- `NewVerifyEmailStartHandler(h)`   -> devuelve `h.verifyEmailStart`
- `NewVerifyEmailConfirmHandler(h)` -> devuelve `h.verifyEmailConfirm`
- `NewForgotHandler(h)`             -> devuelve `h.forgot`
- `NewResetHandler(h)`              -> devuelve `h.reset`

Cómo se usa
- En el router o en el wiring:
  - `router.HandleFunc("/v1/auth/verify-email/start", NewVerifyEmailStartHandler(ef))`
  - etc.
- Alternativa equivalente (sin wrappers): `http.HandlerFunc(ef.verifyEmailStart)`
  (estos wrappers existen por prolijidad/consistencia o para esconder métodos no-exportados).

Dependencias reales
- `net/http` solamente.
- Entrada: `*EmailFlowsHandler` (definido en `email_flows.go` / `email_flows_wiring.go`).

Seguridad / invariantes
- No aplica: acá no se parsea request, no se valida nada, no se tocan tokens.
- La seguridad vive dentro de `EmailFlowsHandler` (y sus adapters / stores).

Patrones detectados
- Factory Method (GoF, ultra simple):
  - Son “factory functions” que entregan un `http.HandlerFunc` preconfigurado (en realidad, un method value).
- Facade (micro):
  - Evitan que el código externo conozca el nombre exacto de los métodos internos (`verifyEmailStart`, etc.).

Cosas no usadas / legacy / riesgos
- (No se usa) potencialmente: si en el proyecto ya se registran endpoints con `http.HandlerFunc(ef.verifyEmailStart)`
  directamente, estas funciones quedan como “azúcar” y pueden ser redundantes.
- OJO: estos wrappers no chequean `h == nil`. Si alguien llama `NewResetHandler(nil)` te comés un panic al usarlo.
  (No es grave si el wiring está bien, pero es un edge-case a marcar.)

Ideas para V2 (sin decidir nada)
- DTO: no hay.
- Controller: no hay lógica, así que podrían eliminarse si no aportan.
- Service/Client/Repo: no hay.
- Si se quieren mantener, podría:
  - agregar un nil-check y devolver un handler que responda 500 (o panic explícito) para fallar rápido y claro.
  - o agruparlos en un `EmailRoutes(ef *EmailFlowsHandler) map[string]http.HandlerFunc` para registrar rutas en bloque.

Resumen
- Archivo utilitario minimalista: expone funciones constructoras que devuelven handlers ya implementados por `EmailFlowsHandler`.
- No agrega lógica ni seguridad; sólo ayuda a conectar métodos privados con el router de forma consistente.




Email Flows Handler (email_flows.go)
───────────────────────────────────────────────────────────────────────────────
Qué es esto
- Este archivo SÍ implementa handlers HTTP reales para flujos de email “account recovery”:
  - Verificación de email (start + confirm)
  - Forgot password (generar token + mandar mail)
  - Reset password (consumir token + setear nueva password + revocar refresh)
- También define interfaces “puerto” (RedirectValidator, TokenIssuer, CurrentUserProvider, RateLimiter)
  para desacoplar el handler del resto del sistema (DB/issuer/ratelimit/auth).

Endpoints registrados (chi)
- POST  /v1/auth/verify-email/start   -> `verifyEmailStart`
- GET   /v1/auth/verify-email         -> `verifyEmailConfirm`
- POST  /v1/auth/forgot               -> `forgot`
- POST  /v1/auth/reset                -> `reset`
(Registrados en `Register(r chi.Router)`)

Flujo general (Template Method mental)
En todos los endpoints el patrón se repite:
1) Decode JSON / query params
2) Resolve tenant (UUID o slug) + validaciones mínimas
3) (Opcional) “gating” por tenant DB (si existe TenantMgr)
4) Rate limit (si hay Limiter)
5) Abrir/usar stores correctos (tenant store preferido, fallback global)
6) Token flow (crear o consumir token)
7) Side-effect: enviar email / setear verified / update password / revocar refresh
8) Respuesta HTTP (204/200/302 o error JSON)

Dependencias y roles (qué usa y para qué)
- `store.TokenStore` (h.Tokens): crea/consume tokens de verificación y reset.
- `store.UserStore` (h.Users): lookup de usuario por email, set verified, update password hash, revocar refresh.
- `email.Templates` (h.Tmpl): templates base (VerifyHTML/TXT, ResetHTML/TXT).
- `email.SenderProvider` (h.SenderProvider): resuelve sender por tenant (multi-tenant SMTP/provider).
- `password.Policy` (h.Policy): valida fuerza de password en reset.
- `RedirectValidator` (h.Redirect): valida redirect_uri permitido por tenant/client.
- `TokenIssuer` (h.Issuer): emite access/refresh post-reset si AutoLoginReset está habilitado.
- `CurrentUserProvider` (h.Auth): intenta extraer usuario actual (Bearer) para verify-start autenticado.
- `RateLimiter` (h.Limiter): rate limit por key (IP/email/client) y agrega Retry-After.
- `controlplane.ControlPlane` (h.Provider): lookup tenant por slug/ID, y override templates / URLs por client.
- `tenantsql.Manager` (h.TenantMgr): Phase 4.1, “gating” y selección explícita de DB por tenant (evita mezclar tenants).
- `httpx` + `helpers`: errores de tenant DB (WriteTenantDBMissing/WriteTenantDBError) y OpenTenantRepo (gating).

OJO: mezcla de estilos de error
- Hay un `writeErr(...)` local que escribe OAuth-like (`error`, `error_description`).
- En algunos casos se usa `httpx.WriteTenantDBMissing/Error` (otro formato).
  ⇒ esto está bueno marcarlo para V2: respuesta consistente.

───────────────────────────────────────────────────────────────────────────────
Verify Email: POST /v1/auth/verify-email/start (verifyEmailStart)
Qué hace
- Inicia flujo de verificación (o reenvío):
  - Si viene autenticado (Bearer): usa el usuario actual y su email.
  - Si NO viene autenticado: requiere `email` en body y hace lookup (sin enumeración: si no existe, 204).

Input JSON (DTO local)
- `verifyStartIn { tenant_id, client_id, email?, redirect_uri? }`
- tenant_id puede ser UUID o slug (se resuelve de ambas formas).

Paso a paso
1) Decode JSON.
2) Resolver tenant UUID:
   - si `tenant_id` parsea como UUID => OK
   - si no, usa `Provider.GetTenantBySlug` y parsea `tenant.ID`.
3) Validar: tenantID != nil y client_id != "".
4) Gating por tenant DB (Phase 4.1):
   - si hay TenantMgr: `helpers.OpenTenantRepo(ctx, TenantMgr, tenantID.String())`
   - si falta DB => 501 (WriteTenantDBMissing) o 500 (WriteTenantDBError)
5) Rate limiting:
   - `verify_start:<ip>` (si hay Limiter)
6) Determinar “modo”:
   - Autenticado: `Auth.CurrentUserID`, luego `Auth.CurrentUserEmail` (fallback: Users.GetEmailByID)
   - No autenticado: requiere `email`, rate limit `verify_resend:<ip>`, lookup por email en UserStore tenant-aware.
     - Si no existe: 204 (anti-enumeración)
7) Validar redirect_uri si viene:
   - `Redirect.ValidateRedirectURI(tenantID, clientID, redirect)`
8) Delegar envío real:
   - `SendVerificationEmail(ctx, rid, tenantID, userID, email, redirect, clientID)`
9) Respuesta: 204 No Content.

Notas / invariantes
- Anti-enumeración: el “resend” devuelve 204 cuando no encuentra usuario.
- redirect_uri siempre validada por tenant+client para evitar open-redirect.

Cosas a marcar
- En modo autenticado: si `Auth.CurrentUserEmail` falla, hace fallback a `h.Users.GetEmailByID` (pero OJO: ahí usa el store global,
  no el userStore tenant-aware resuelto arriba). Esto puede ser bug multi-tenant (mezcla de DB) o deuda técnica.
- “Gating” abre repo sólo para chequear DB, pero no reutiliza repo/store resultante (se vuelve a pedir pool más abajo).

───────────────────────────────────────────────────────────────────────────────
SendVerificationEmail (método reusable)
Qué hace
- Crea token de verificación (per-tenant si posible), arma link, renderiza template (con overrides por tenant),
  resuelve sender por tenant y manda email.
- Está expuesto porque lo usa Register handler (para mandar mail sin duplicar lógica).

Paso a paso
1) Resolver TokenStore:
   - default `h.Tokens`
   - si `TenantMgr` permite: `tenantDB.Pool()` => `store.NewTokenStore(pool)`
2) `CreateEmailVerification(tenantID, userID, email, VerifyTTL, ipPtr, uaPtr)`
   - IP/UA están en nil (porque no hay request acá), deuda: se podrían pasar desde verifyEmailStart.
3) Armar link con `buildLink("/v1/auth/verify-email", token, redirect, clientID, tenantID)`
4) Buscar tenant por ID para:
   - nombre y overrides de templates (`tenant.Settings.Mailing.Templates`)
5) Render:
   - `renderVerify` (usa override TemplateVerify si existe, sino templates base)
6) Sender:
   - `SenderProvider.GetSender(ctx, tenantID)`
7) Send:
   - `sender.Send(to, "Verificá tu email", html, text)`
   - Manejo “soft”: si falla el send, log + `return nil` (NO rompe el flujo).
     Esto es intencional para no romper register/verify start aunque SMTP esté caído.

Riesgo importante
- “Soft fail” en envío: deja usuario sin mail. Está bien para register, pero para “verify start” quizá sería mejor retornar error
  y avisar al cliente. (No decidir ahora, sólo marcarlo).

───────────────────────────────────────────────────────────────────────────────
Verify Email: GET /v1/auth/verify-email (verifyEmailConfirm)
Qué hace
- Consume token de verificación y marca `EmailVerified=true`, luego redirige a redirect_uri con `status=verified`
  o devuelve JSON `{status:"verified"}`.

Input (query params)
- token (obligatorio)
- redirect_uri (opcional)
- client_id (opcional)
- tenant_id (opcional pero clave para seleccionar DB correcta)

Paso a paso
1) Leer query params y validar `token`.
2) Selección de stores por tenant_id:
   - default: `h.Tokens` + `h.Users`
   - si viene tenant_id y hay TenantMgr: usa pool tenant => TokenStore + UserStore tenant-specific.
3) `UseEmailVerification(token)` => retorna (tenantID?, userID).
4) Resolver redirect default si redirect vacío:
   - con Provider: GetTenantByID(tenantID) => slug
   - GetClient(slug, clientID) y elige:
     - RedirectURIs[0] si existe, sino VerifyEmailURL
5) Validar redirect si existe: `Redirect.ValidateRedirectURI(tenantID, clientID, redirect)`
6) `userStore.SetEmailVerified(userID)`
7) Respuesta:
   - si redirect != "" -> 302 Found a `redirect?status=verified`
   - si no -> JSON.

Cosas a marcar
- `tenantID` se parsea sólo si el query param viene como UUID. Si viene slug, no se resuelve (a diferencia de verifyStart/forgot/reset).
  En V2 conviene unificar “resolver tenant id/slug” para todos.
- Mezcla de formatos de error: `writeErr` vs `httpx.WriteTenantDB*`.

───────────────────────────────────────────────────────────────────────────────
Forgot Password: POST /v1/auth/forgot (forgot)
Qué hace
- Flujo “no enumerar usuarios”: siempre responde `{status:"ok"}` aunque el email no exista.
- Si existe: genera token reset + manda mail con link de reset (custom per client si está configurado).

Input JSON (forgotIn)
- tenant_id (UUID o slug)
- client_id
- email
- redirect_uri (opcional)

Paso a paso
1) Decode JSON.
2) Resolver tenant UUID (UUID o Provider.GetTenantBySlug).
3) (Opcional) gating DB: OpenTenantRepo(tenantID.String()) sólo para validar existencia de DB.
4) Construir stores tenant-aware:
   - si TenantMgr: GetPG(ctx, in.TenantID) (OJO usa in.TenantID, puede ser slug) => UserStore + TokenStore del tenant.
   - else: usa stores globales del handler.
5) Rate limit: key `forgot:<tenantUUID>:<emailLower>`
6) Lookup user por email (en tenant):
   - si no existe: devolver OK igual
7) Validar redirect_uri (si no valida, se limpia y se sigue).
8) Crear token: `CreatePasswordReset(tenantID, uid, email, ResetTTL, ipPtr, uaPtr)`
   - IP/UA se capturan del request.
9) Armar link:
   - si el client tiene `ResetPasswordURL`, lo usa y le agrega `token=...`
   - si no: backend `/v1/auth/reset` con query params (token, redirect, client_id, tenant_id)
10) Render template reset (override TemplateReset si existe)
11) SenderProvider.GetSender(tenantID) y enviar mail.
12) Respuesta final: JSON `{status:"ok"}`.

Riesgos / detalles
- El “custom reset URL” usa solo token y no agrega tenant/client; está perfecto si esa URL ya sabe el tenant por dominio/ruta.
  Si no, podría perder contexto.
- `tenant, _ := h.Provider.GetTenantBySlug(ctx, in.TenantID)` asume que `in.TenantID` es slug, pero podría ser UUID.
  (En ese caso, templates/tenantDisplayName podrían fallar o quedar pobres). Esto es deuda clara para V2:
  resolver slug de forma robusta (uuid -> tenant -> slug).

───────────────────────────────────────────────────────────────────────────────
Reset Password: POST /v1/auth/reset (reset)
Qué hace
- Consume token de reset, valida policy/blacklist, hashea password, actualiza password hash,
  revoca refresh tokens y opcionalmente hace auto-login devolviendo tokens.

Input JSON (resetIn)
- tenant_id (UUID o slug)
- client_id
- token
- new_password

Paso a paso
1) Decode JSON.
2) Resolver tenant UUID (UUID o Provider.GetTenantBySlug).
3) Crear stores tenant-aware (UserStore + TokenStore) si TenantMgr existe.
4) Rate limit: `reset:<ip>`
5) Validar password con Policy.Validate -> si falla, 400 weak_password.
6) Consumir token: `UsePasswordReset(token)` => userID
7) Blacklist opcional: `password.GetCachedBlacklist(BlacklistPath).Contains(new_password)`
8) Hash: `password.Hash(password.Default, new_password)`
9) Update password: `userStore.UpdatePasswordHash(userID, hash)`
10) Revocar sesiones: `userStore.RevokeAllRefreshTokens(userID)` (invalidate all devices)
11) Si AutoLoginReset:
   - `Issuer.IssueTokens(w,r, tenantID, clientID, userID)` (escribe JSON de tokens tipo login)
   - Headers no-store/no-cache
12) Si no: 204 No Content.

Invariantes de seguridad
- Tokens de reset/verify son de un solo uso (UsePasswordReset/UseEmailVerification).
- Rate limit para evitar abuso (enumeración, bombardeo de mails, brute force de reset).
- Validación de password + blacklist (política).
- Revocación de refresh tokens post-reset (correcto: corta sesiones viejas).
- Redirect URI validada siempre con tenant+client.

───────────────────────────────────────────────────────────────────────────────
Helpers internos (no HTTP)
- `writeErr`: emite JSON estilo OAuth (`error`, `error_description`).
- `rlOr429`: aplica rate limit y setea Retry-After; devuelve bool “se cortó” (early return).
- `renderVerify`, `renderReset`, `renderOverride`: render templates base + overrides por tenant (Template pattern).
- `sanitizeUA`, `clientIPOrEmpty`, `strPtrOrNil`: utilidades locales.

Patrones detectados (GoF / arquitectura)
- Adapter / Ports & Adapters:
  - Las interfaces RedirectValidator / TokenIssuer / CurrentUserProvider / RateLimiter son “ports”;
    el wiring (email_flows_wiring.go) inyecta adapters concretos.
- Strategy:
  - Validación de redirect (RedirectValidator) y rate limit (RateLimiter) cambian sin tocar el handler.
- Template Method:
  - Flujo repetido: parse -> validate -> resolve -> store -> action -> respond.
- Facade:
  - EmailFlowsHandler junta varias dependencias y expone endpoints simples.
- (Concurrencia) No hay goroutines acá. Todo es sync.

Cosas no usadas / legacy / riesgos (marcar)
- `context`, `tenantsql.ErrNoDBForTenant` importado en este archivo? (Acá se importa `tenantsql` pero no se usa ErrNoDBForTenant; solo `tenantsql.Manager`).
- `httpx` se usa sólo para errores de tenant DB; el resto usa writeErr. Inconsistencia intencional/accidental.
- Mezcla de “tenant_id slug vs UUID” en distintas llamadas al Provider:
  - a veces se asume slug (GetTenantBySlug) y a veces ID (GetTenantByID).
  - en forgot() se usa GetTenantBySlug(in.TenantID) aunque puede ser UUID.
- En verifyEmailStart() el fallback `h.Users.GetEmailByID` puede estar yendo al store global aunque el tenant DB sea otro.

Ideas para V2 (sin decidir nada, solo guía de “desarme” en capas)
1) DTOs (entrada/salida)
   - verifyStartIn, forgotIn, resetIn como DTOs en `dto/` (y validar campos ahí).
   - Respuestas: unificar formato (usar httpx.WriteError o un ResponseWriter común).

2) Controller (HTTP)
   - Controller por endpoint que solo haga:
     - parse + validate
     - armar “command” para service
     - mapear errores a HTTP
   - Sacar loggers y writeErr disperso a helpers comunes.

3) Services (casos de uso)
   - `EmailVerificationService`:
     - Start(tenant, client, authContext, email, redirect) -> side effect send email
     - Confirm(token, tenantHint, client, redirect) -> mark verified + redirect resolver
   - `PasswordResetService`:
     - Forgot(tenant, client, email, redirect) -> create token + send mail
     - Reset(tenant, client, token, newPassword) -> consume token + update password + revoke + optional issue tokens

4) Clients / Integraciones
   - `ControlPlaneClient` (Provider) para:
     - ResolveTenant( slug|uuid ) -> {uuid, slug, displayName}
     - ResolveClient(tenantSlug, clientID) -> {redirectURIs, resetURL, verifyURL, templates...}
   - `Mailer/SenderClient` wrapper sobre SenderProvider (con retry/diagnostics si querés).

5) Repo / Store
   - Encapsular “tenant store selection”:
     - `TenantStoreResolver` que devuelva `UserStore` + `TokenStore` correctos por tenant (uuid/slug)
     - Evitar duplicar GetPG + NewTokenStore en cada endpoint.

6) Patrones a aplicar en el refactor
   - Strategy para “LinkBuilder” (backend link vs client custom URL) y para “ErrorResponder”.
   - Facade/Service para “TenantContextResolver” (te devuelve tenantUUID, tenantSlug y te valida DB).
   - Chain of Responsibility (middlewares) para:
     - rate limit
     - tenant gating
     - parse de tenant/client
     - logging de request_id
   - Decorator para Sender (diagnóstico + retry soft para temporales).

Resumen
- Este archivo es EL núcleo de “email flows”: verify + forgot/reset, con multi-tenant y overrides por control-plane.
- Tiene buenas ideas (ports/adapters, rate limit, anti-enumeración, revocación post-reset),
  pero mezcla responsabilidades (HTTP + store selection + templating + mail + redirect validation).
- Para V2: separar “resolver tenant/client + stores” y unificar formato de error/redirect/slug-vs-uuid para que no se te escape un bug multi-tenant.




json.go — Helper interno para parseo JSON “estricto” (DisallowUnknownFields + límite 64KB) [NO es handler HTTP]

Qué es este archivo
-------------------
Este archivo NO expone endpoints HTTP directamente. Define un helper interno del package `handlers`
llamado `readStrictJSON(...)` que se usa como alternativa a `httpx.ReadJSON(...)` cuando se quiere:

- Validar Content-Type estrictamente (application/json)
- Limitar tamaño de body a 64KB
- Rechazar campos desconocidos (`json.Decoder.DisallowUnknownFields()`)
- Estandarizar errores con `httpx.WriteError(...)`

En otras palabras: es un “parser” de request bodies con política más dura que la función genérica
`internal/http/v1.ReadJSON`.

================================================================================
Qué hace (objetivo funcional)
================================================================================
`readStrictJSON(w, r, dst)`:
- Valida que `Content-Type` contenga `application/json`.
- Reemplaza `r.Body` por un `http.MaxBytesReader` de 64KB.
- Decodifica JSON en `dst` usando `json.Decoder`.
- Rechaza keys desconocidas (fail-fast por payloads inesperados).
- Responde errores HTTP y devuelve `false` si algo falla.

================================================================================
Cómo se usa (en el resto del código)
================================================================================
Este helper se invoca típicamente dentro de handlers donde el contrato de entrada es “estricto” y
querés detectar:
- typos del cliente
- payloads viejos con campos obsoletos
- inputs maliciosos que intentan “colarse” con campos extra

Ojo: muchos handlers v1 usan `httpx.ReadJSON` (tolerante) para no romper compatibilidad con clients.
Este helper es la otra cara: “preferí seguridad/claridad sobre compat”.

================================================================================
Flujo paso a paso (readStrictJSON)
================================================================================
1) Validación de Content-Type
	 - Lee `Content-Type` del request.
	 - Si NO contiene `application/json`:
		 - Responde `415 Unsupported Media Type`
		 - error_code=1101, code=`unsupported_media_type`
		 - msg: “se requiere Content-Type: application/json”
		 - retorna false.

2) Límite de tamaño
	 - Reemplaza `r.Body` por `http.MaxBytesReader(w, r.Body, 64<<10)`.
	 - Cierra body con `defer r.Body.Close()`.
	 - Impacto: protege contra payloads grandes (DoS / memory pressure).

3) Decode + unknown fields
	 - `json.NewDecoder(r.Body)`
	 - `dec.DisallowUnknownFields()`
	 - Si `dec.Decode(dst)` falla:
		 - Si `io.EOF` => trata como “body vacío”.
		 - Otros => “json inválido” (genérico).
		 - Responde `400 Bad Request` con code=`invalid_json`, error_code=1102.
		 - retorna false.

4) “Datos extra”
	 - Hace un chequeo de “sobran datos” con `dec.More()`.
	 - Si hay más tokens: 400 con error_code=1103.
	 - Nota: esta verificación es una idea correcta (un JSON válido debería terminar), pero
		 `dec.More()` sólo aplica en ciertos contextos (arrays/objetos). Si el objetivo fuera
		 “no hay trailing garbage”, el patrón usual es intentar un segundo `Decode(&struct{}{})`
		 y esperar `io.EOF`. (No cambiar acá; sólo marcarlo.)

================================================================================
Dependencias reales
================================================================================
- stdlib:
	- `encoding/json`, `io`, `net/http`, `strings`
- internas:
	- `internal/http/v1` como `httpx` para `WriteError`.

No usa `app.Container`, `Store`, `TenantSQLManager`, `cpctx.Provider`, `Issuer`, `Cache`, etc.

================================================================================
Seguridad / invariantes
================================================================================
- Límite 64KB: reduce superficie DoS por request bodies grandes.
- `DisallowUnknownFields`: ayuda a “fijar contrato” y evitar que cambios de API pasen desapercibidos.
- Errores con envelope consistente (`httpx.WriteError`).

================================================================================
Patrones detectados (GoF / arquitectura)
================================================================================
- Policy / Strategy (implícito):
	- Existen dos estrategias de parseo JSON:
		- `httpx.ReadJSON` (tolerante, 1MB, sin DisallowUnknownFields)
		- `readStrictJSON` (estricto, 64KB, DisallowUnknownFields)
	En V2 esto podría formalizarse como un “Decoder policy” configurable por endpoint.

No hay concurrencia ni estado compartido.

================================================================================
Cosas no usadas / legacy / riesgos
================================================================================
- El mensaje de error ante `Decode` es bastante genérico.
	- Está bien para no filtrar detalles, pero dificulta debugging de clientes.
	- En V2 se podría incluir un detalle acotado (ej: “campo desconocido: x”).

- Chequeo de trailing data:
	- `dec.More()` puede no cubrir todos los casos de basura trailing.
	- Si alguna ruta depende de esto como “seguridad”, conviene estandarizar el patrón.

================================================================================
Ideas para V2 (sin decidir nada)
================================================================================
1) Unificar “request decoding” como componente
	 - `RequestDecoder{ MaxBytes, StrictUnknownFields, RequireJSONContentType }`.
	 - El controller sólo pide: `DecodeJSON(r, &dto)` y recibe errores tipados.

2) Consistencia de errores
	 - Mantener el envelope `WriteError` y códigos internos.
	 - Alinear 415 vs 400 en todos los endpoints (hoy hay mezcla según handler).

3) Observabilidad
	 - En modo debug, loguear causa exacta (unknown field, syntax error), sin devolverla al cliente.

Guía de “desarme” en capas
--------------------------
- Transport/controller:
	- Decidir política (estricta vs tolerante) y mapear a error HTTP.
- Infra/util:
	- Implementación del decoder (esta función o un componente equivalente).

Resumen
------
- `json.go` no es un handler HTTP: es un helper de parseo JSON estricto que valida Content-Type,
	limita body a 64KB y rechaza campos desconocidos, devolviendo errores estándar vía `httpx.WriteError`.





jwks.go — JWKS endpoints (global + por-tenant) con cache de corto TTL

Qué hace este handler
---------------------
Este archivo implementa el handler que expone las llaves públicas (JWKS) para que:
- clientes OIDC/OAuth
- RPs
- resource servers
puedan validar JWTs emitidos por HelloJohn.

Publica DOS variantes:
1) JWKS global
2) JWKS por tenant (slug)

Esto está alineado con el modelo v1 donde el issuer puede ser:
- global
- o “path-based” por tenant (IssuerModePath), con llaves por tenant.

Rutas que maneja (registro real en router)
-----------------------------------------
Se registra desde `internal/http/v1/routes.go` de esta forma:
- `GET|HEAD /.well-known/jwks.json` -> `JWKSHandler.GetGlobal`
- `GET|HEAD /.well-known/jwks/{slug}.json` -> `JWKSHandler.GetByTenant`
	Nota: por limitaciones del stdlib `ServeMux`, en realidad se registra como prefix:
		`/.well-known/jwks/` y el handler parsea el sufijo `/{slug}.json`.

Formato de responses
--------------------
- Content-Type: `application/json; charset=utf-8`
- Cache-Control: `no-store` + `Pragma: no-cache`
- 200 OK:
	- GET: body contiene JWKS JSON (raw)
	- HEAD: devuelve sólo headers (sin body)

Errores típicos:
- 405 method_not_allowed si no es GET/HEAD (incluye `Allow: GET, HEAD`)
- 404 Not Found si el path no matchea `/.well-known/jwks/{slug}.json`
- 400 invalid_request si el slug es inválido
- 500 server_error si falla el loader del JWKSCache

================================================================================
Flujo paso a paso
================================================================================

1) GetGlobal — `GET|HEAD /.well-known/jwks.json`
------------------------------------------------
1. Valida método: sólo GET/HEAD.
2. Setea `no-store`.
3. Si HEAD: responde 200 y corta.
4. Pide el JWKS al cache:
		 `h.Cache.Get("global")`
5. Devuelve 200 con el JSON crudo.

2) GetByTenant — `GET|HEAD /.well-known/jwks/{slug}.json`
---------------------------------------------------------
1. Valida método: sólo GET/HEAD.
2. Parseo manual del path (ServeMux):
	 - Requiere prefijo `/.well-known/jwks/`
	 - Requiere sufijo `.json`
	 - Extrae slug (lo que queda en el medio)
3. Valida slug por regex: `^[a-z0-9\-]{1,64}$`
	 - Si no matchea: 400 invalid_request.
4. Setea `no-store`.
5. Si HEAD: responde 200 y corta.
6. Pide JWKS al cache:
		 `h.Cache.Get(slug)`
7. Devuelve 200 con el JSON crudo.

================================================================================
Dependencias reales
================================================================================
- stdlib:
	- `net/http` (handler, status, headers)
	- `strings` (parseo de path)
	- `regexp` (validación de slug)

- internas:
	- `internal/http/v1` como `httpx` (WriteError)
	- `internal/jwt` como `jwtx` (JWKSCache)

El handler NO usa `app.Container` directamente. La dependencia relevante se inyecta como:
- `JWKSHandler.Cache *jwtx.JWKSCache`

Cómo se construye el cache (contexto v1)
----------------------------------------
En `cmd/service/v1/main.go` se crea `container.JWKSCache = jwtx.NewJWKSCache(15s, loader)` donde:
- tenant vacío o `global` -> llama a `issuer.Keys.JWKSJSON()`.
- tenant != global -> llama a `issuer.Keys.JWKSJSONForTenant(tenant)`.

Punto importante: el loader per-tenant NO hace auto-bootstrap en v1 (por diseño HA):
- si faltan keys del tenant, la idea es fallar y que la rotación/restore cree las llaves.

Además, el cache soporta invalidación:
- `JWKSCache.Invalidate(tenant)`
	- usado por admin/cluster restore (ej: `cpctx.InvalidateJWKS` y handlers admin de tenants).

================================================================================
Seguridad / invariantes
================================================================================
- JWKS es público (no requiere authN/authZ).
- Se envían headers `no-store`.
	- Esto es conservador; evita caches intermedias.
	- El costo se amortiza con `JWKSCache` (TTL corto) del lado del server.

- Validación de slug:
	- Restringe a `[a-z0-9-]` y longitud <= 64.
	- Reduce superficie de path tricks y evita slugs raros.

- Método HEAD soportado:
	- Útil para health checks / discovery sin descargar el payload.

================================================================================
Patrones detectados (GoF / arquitectura)
================================================================================
- Cache / Memoization:
	- `jwtx.JWKSCache` implementa un cache in-memory con TTL, protegido con RWMutex.

- Dependency Injection (manual):
	- El handler depende de una abstracción “loader” encapsulada dentro de `JWKSCache`.
	- Se inyecta desde `main.go` (composition root).

- Adapter (leve):
	- `JWKSHandler` adapta el `JWKSCache` (que retorna `json.RawMessage`) al contrato HTTP.

No hay concurrencia en el handler; la concurrencia está encapsulada en el cache.

================================================================================
Cosas no usadas / legacy / riesgos
================================================================================
- Regex recompilada por request:
	- `regexp.MustCompile(...)` está dentro de `GetByTenant`, por lo que se compila en cada request.
	- No rompe funcionalidad, pero es innecesario y puede impactar performance bajo carga.
	- En V2 conviene moverlo a un `var slugRe = regexp.MustCompile(...)` a nivel package.

- Errores 500 para missing tenant keys:
	- Si el loader devuelve error por “no keys for tenant”, hoy se expone como 500 server_error.
	- Puede estar bien (depende del contrato), pero en V2 convendría tipar el error:
		- 404/400/503 según el caso (tenant inexistente vs keys no generadas vs backend caído).

================================================================================
Ideas para V2 (sin decidir nada)
================================================================================
1) Controller + Service explícitos
	 - Controller: parseo de método/path/slug + headers.
	 - Service: `GetJWKS(ctx, tenant)` que conoce loader/keystore y errores tipados.

2) Router declarativo
	 - Evitar parseo manual del path; usar router con params (chi o similar) o patrones.

3) Error taxonomy
	 - Estandarizar errores de JWKS:
		 - tenant inválido
		 - keys inexistentes
		 - backend caído
	 - Mantener envelope `httpx.WriteError` consistente.

4) Caché y cache-control
	 - Evaluar `Cache-Control: public, max-age=...` + ETag en edge/CDN si se busca performance.
	 - Mantener invalidación por tenant ante rotación/restore.

Guía de “desarme” en capas
--------------------------
- Transport/controller:
	- Método + parseo slug + response writer (headers/status).
- Service:
	- Resolver JWKS (global vs tenant) con contrato de errores.
- Infra:
	- `jwtx.JWKSCache` (TTL + locking)
	- `PersistentKeystore` / store de signing keys.

Resumen
-------
- `jwks.go` expone JWKS global y por tenant con soporte GET/HEAD.
- Usa `jwtx.JWKSCache` para amortiguar costos y permite invalidación por tenant.
- Tiene oportunidades claras para V2: router declarativo, regex precompilada y errores tipados.




me.go — Endpoint de introspección “ligera” del Access Token (parse JWT y devuelve claims seleccionadas)

Qué hace este handler
---------------------
Este handler implementa un endpoint simple para “ver quién soy” a partir del access token.
En vez de depender del middleware `RequireAuth` (que parsea JWT y mete claims en el contexto),
este handler hace el parse/validación del JWT por su cuenta usando `github.com/golang-jwt/jwt/v5`.

Ruta que maneja
--------------
- GET /v1/me

Entrada / salida
---------------
- Requiere header:
	- `Authorization: Bearer <access_token>`

- Respuesta 200 (JSON):
	{
		"sub": <claim sub>,
		"tid": <claim tid>,
		"aud": <claim aud>,
		"amr": <claim amr>,
		"custom": <claim custom>,
		"exp": <claim exp>
	}

Errores relevantes:
- 405 method_not_allowed si no es GET
- 401 missing_bearer si falta Authorization: Bearer
- 401 invalid_token si el token no valida firma/issuer o claims inválidas

================================================================================
Flujo paso a paso (GET /v1/me)
================================================================================
1) Validación de método
	 - Sólo GET.
	 - Si no: `httpx.WriteError(405, "method_not_allowed", ...)`.

2) Extracción de bearer token
	 - Lee `Authorization`.
	 - Verifica prefijo `bearer ` (case-insensitive).
	 - Si falta: 401 `missing_bearer`.

3) Parse y validación de JWT
	 - Usa `jwtv5.Parse(raw, c.Issuer.Keyfunc(), ...)`.
	 - Restricciones aplicadas:
		 - `WithValidMethods([]string{"EdDSA"})`
		 - `WithIssuer(c.Issuer.Iss)`
	 - Si falla o `!tk.Valid`: 401 `invalid_token`.

4) Extracción de claims
	 - Espera `jwtv5.MapClaims`.
	 - Si el tipo no coincide: 401 `invalid_token`.

5) Respuesta
	 - `Content-Type: application/json; charset=utf-8`
	 - Encode de un objeto con claims seleccionadas (sin transformar tipos).

================================================================================
Dependencias reales
================================================================================
- `app.Container`:
	- Usa `c.Issuer` (issuer string y Keyfunc para validar firma).

- Internas:
	- `internal/http/v1` como `httpx` para `WriteError`.

- Externas:
	- `github.com/golang-jwt/jwt/v5` para parse/validación.

No usa `Store`, `TenantSQLManager`, `cpctx.Provider`, `Cache`, `JWKSCache`.

================================================================================
Seguridad / invariantes
================================================================================
- Valida:
	- método de firma (EdDSA)
	- issuer (iss) exacto según `c.Issuer.Iss`

- NO valida explícitamente:
	- audience (aud)
	- scopes
	- expiración más allá de lo que haga el parser/claims estándar (depende del contenido y del uso de MapClaims)

- Filtración controlada:
	- Devuelve `custom` completo tal cual viene en el token.
	- Si el token contiene datos sensibles en `custom`, este endpoint los refleja.

================================================================================
Patrones detectados (GoF / arquitectura)
================================================================================
- “Inline auth” / duplicación de middleware:
	- Este handler reimplementa en pequeño lo que `RequireAuth` ya hace (parsear JWT y extraer claims).

- Controller-only:
	- No hay “service” ni “repo”; es puro parseo y response.

No hay concurrencia.

================================================================================
Cosas no usadas / legacy / riesgos
================================================================================
- Duplicación con `internal/http/v1/middleware.RequireAuth`:
	- En el codebase ya existe un mecanismo estándar para validar tokens y obtener claims del contexto.
	- Este endpoint podría ser un “legacy convenience” o un debug endpoint mantenido por compat.

- Tipos de claims:
	- `aud/amr/custom` se devuelven como `any` sin normalización; clientes deben tolerar múltiples shapes.

- Issuer único:
	- Valida issuer contra `c.Issuer.Iss` global. Si el sistema usa issuer efectivo por tenant
		(IssuerModePath / override), este endpoint podría comportarse distinto a otros flujos.
		(Depende de cómo se emiten los tokens en login/refresh y qué iss llevan.)

================================================================================
Ideas para V2 (sin decidir nada)
================================================================================
1) Unificar auth parsing
	 - Convertir `/v2/me` en un handler que dependa de middleware auth estándar:
		 - `RequireAuth` (o equivalente v2) + `GetClaims(ctx)`.
	 - Evita duplicación y asegura mismas reglas de validación en todo el stack.

2) Definir contrato de salida
	 - DTO claro (y estable) para “me”:
		 - sub, tid, aud, amr, exp
		 - y opcionalmente un subset de `custom` (o “sys namespace” controlado)
	 - Evitar exponer `custom` completo si contiene flags internos.

3) Validaciones adicionales
	 - Considerar validación de `aud` (si el endpoint se usa por clients específicos).
	 - Considerar enforcement de scopes (si /me debe requerir alguno).

Guía de “desarme” en capas
--------------------------
- Controller:
	- Método + extracción bearer + response.
- Auth component:
	- Validación de JWT + claims parsing (idealmente middleware compartido).
- DTO:
	- Definir shape estable del JSON de salida.

Resumen
-------
- `me.go` implementa `GET /v1/me` y devuelve un subset de claims del access token.
- Valida EdDSA + issuer, pero no usa el middleware estándar de auth y puede duplicar lógica.
- Es un buen candidato a normalizar en V2 alrededor de `RequireAuth` + `GetClaims` y un DTO estable.





mfa_totp.go — MFA TOTP + Recovery Codes + “MFA Challenge” (cache) + emisión de tokens con amr+=mfa

Qué es este archivo (la posta)
------------------------------
Este archivo define un handler de MFA “todo-en-uno” (mfaHandler) que implementa:
	- Enroll de TOTP (generar secreto + persistir cifrado)
	- Verify de TOTP (confirmar MFA y, si es primera vez, emitir recovery codes)
	- Challenge MFA para completar login (validar code/recovery contra un mfa_token en cache y emitir access/refresh)
	- Disable de TOTP (requiere password + segundo factor)
	- Rotate de recovery codes (requiere password + segundo factor)

Además incluye:
	- Cripto local AES-GCM con prefijo versionado (GCMV1-MFA:...) para guardar el secreto TOTP cifrado
	- Generación de recovery codes (10 chars legibles) y hashing
	- “Remember device” mediante cookie mfa_trust + trusted devices en store
	- Rate-limit por endpoint (si c.MultiLimiter está configurado)

No es solo “MFA”: también participa en el contrato de auth, porque en /challenge emite tokens
y aplica hooks de claims (applyAccessClaimsHook).

Fuente de verdad (dependencias reales)
-------------------------------------
Container usado:
	- h.c.Store: repositorio core con soporte MFA y auth (TOTP, recovery codes, trusted devices, refresh tokens)
	- h.c.Cache: cache KV para mfa_token -> mfaChallenge (one-shot)
	- h.c.Issuer: emite access token (IssueAccess)
	- h.c.MultiLimiter + helpers.EnforceMFA*Limit: rate limiting

Modelos/payloads:
	- mfaChallenge (definido en mfa_types.go): {uid, tid, cid, amr_base, scp}
		Este payload NO se crea acá: lo producen otros flows (p.ej. login) y se valida acá.

Autenticación / trust boundary (importante)
-------------------------------------------
Enroll/Verify/Disable/RotateRecovery dependen de un header:
	- X-User-ID: debe ser UUID válido (currentUserFromHeader)

Esto implica que estos endpoints asumen un “front-door” (middleware/gateway) que:
	- ya autenticó al usuario
	- inyecta X-User-ID de forma confiable

Si ese supuesto falla, cualquiera puede setear X-User-ID y operar MFA sobre otra cuenta.
En V2 esto debería ser SIEMPRE claims en context (RequireAuth + GetClaims), no headers.

Config / ENV relevantes
-----------------------
- MFA_TOTP_ISSUER: issuer del otpauth:// (default: HelloJohn)
- MFA_TOTP_WINDOW: tolerancia de ventana TOTP (0..3, default 1)
- MFA_REMEMBER_TTL: TTL de trusted device cookie (default 30d)

Cripto: cifrado del secreto TOTP
--------------------------------
Se cifra el secreto base32 usando AES-GCM con clave derivada de SIGNING_MASTER_KEY:
	- requiere len(key) >= 32 y usa key[:32] como key AES-256
	- output: "GCMV1-MFA:" + hex(nonce||ciphertext)

Notas:
	- Reutiliza SIGNING_MASTER_KEY para “encryption at rest” (MFA secret). Eso mezcla propósitos.
		V2: clave dedicada (MFA_ENC_MASTER_KEY) o un SecretsManager central.

Rutas soportadas (lo que expone realmente)
------------------------------------------
Se registran vía chi.Router (Register) y también se exponen wrappers http.Handler para ServeMux.

A) Enroll TOTP
--------------
- POST /v1/mfa/totp/enroll
		Auth: requiere X-User-ID
		Flujo:
			1) (opcional) rate limit enroll
			2) Store.GetUserByID(uid) (solo para email)
			3) totp.GenerateSecret() -> secret base32
			4) aesgcmEncrypt(secret) -> SecretEncrypted
			5) Store.UpsertMFATOTP(uid, enc)
			6) Responde: {secret_base32, otpauth_url}

Riesgo: devuelve el secreto en claro y NO setea Cache-Control: no-store.

B) Verify TOTP (confirmación)
-----------------------------
- POST /v1/mfa/totp/verify  body: {code}
		Auth: requiere X-User-ID
		Flujo:
			1) rate limit verify
			2) Store.GetMFATOTP(uid)
			3) decrypt + base32 decode
			4) totp.Verify(raw, code, now, window, lastCounter)
			5) Store.UpdateMFAUsedAt(counter)
			6) Store.ConfirmMFATOTP(now)
			7) Si es primera confirmación:
					 - generateRecoveryCodes(10)
					 - Store.InsertRecoveryCodes(hashes)
					 - Responde {enabled:true, recovery_codes:[...]}
				 Si falla generación/persistencia: no bloquea, responde {enabled:true}

Nota: usa LastUsedAt para pasar lastCounter (anti-reuse), pero la persistencia de UsedAt ignora errores.

C) Challenge MFA (completar login)
----------------------------------
- POST /v1/mfa/totp/challenge  body: {mfa_token, code|recovery, remember_device?}
		Auth: NO usa X-User-ID; valida contra el mfa_token en cache.
		Flujo:
			1) valida que exista code o recovery (400 si faltan)
			2) cache.Get("mfa:token:"+mfa_token) -> payload JSON
			3) unmarshal -> mfaChallenge {uid, tid, cid, amr_base, scp}
			4) rate limit challenge por uid
			5) valida segundo factor:
				 - si recovery: Store.UseRecoveryCode(uid, hash, now)
				 - si TOTP: Store.GetMFATOTP(uid) + decrypt + Verify + UpdateMFAUsedAt
			6) si remember_device:
				 - genera devToken, guarda hash en Store.AddTrustedDevice(exp)
				 - setea cookie HttpOnly mfa_trust; Secure depende de TLS o X-Forwarded-Proto=https
			7) emite access token:
				 - std claims: {tid, amr: amr_base+"mfa", acr:"urn:hellojohn:loa:2"}
				 - applyAccessClaimsHook(...)
				 - Issuer.IssueAccess(uid, clientID, std, custom)
			8) genera refresh token (opaque) y lo persiste:
				 - preferido: Store.(CreateRefreshTokenTC)(tenantID, clientID_text, hash_hex)
				 - fallback legacy: Store.CreateRefreshToken(uid, clientID_internal, hash_b64url)
			9) borra el mfa_token del cache SOLO al final (one-shot)
		 10) responde AuthLoginResponse con Cache-Control: no-store

Puntos clave:
	- Este endpoint “cierra” el login: por eso mezcla MFA con emisión de tokens.
	- Hay fallback legacy para refresh tokens (depende de capacidades del store).

D) Disable MFA
--------------
- POST /v1/mfa/totp/disable  body: {password, code|recovery}
		Auth: requiere X-User-ID
		Flujo:
			1) rate limit disable
			2) valida password:
				 - Store.GetUserByID(uid)
				 - Store.GetUserByEmail(tenantID, email) -> identity.PasswordHash
				 - Store.CheckPassword(hash, password)
			3) valida 2do factor (recovery o TOTP)
			4) Store.DisableMFATOTP(uid)
			5) responde {disabled:true}

Riesgo: en el path TOTP de disable se ignoran errores de decrypt/base32 decode.

E) Rotate recovery codes
------------------------
- POST /v1/mfa/recovery/rotate  body: {password, code|recovery}
		Auth: requiere X-User-ID
		Flujo:
			1) valida password (misma lógica que disable)
			2) valida 2do factor (recovery o TOTP)
			3) Store.DeleteRecoveryCodes(uid)
			4) generateRecoveryCodes(10) y Store.InsertRecoveryCodes
			5) responde {rotated:true, recovery_codes:[...]} (solo una vez)

Helpers locales y patrones
--------------------------
- mfaconfig*(): lee env con defaults
- aesgcmEncrypt/aesgcmDecrypt: esquema “GCMV1-MFA” versionado
- generateRecoveryCodes: random seguro, alfabeto sin caracteres confusos

Patrones:
	- Feature detection por type assertion (CreateRefreshTokenTC) + fallback legacy
	- One-shot token en cache para challenge (mfa_token)
	- Hook de claims antes de emitir access token (applyAccessClaimsHook)

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) Autenticación por header X-User-ID (alto riesgo)
	 Esto no debería ser un contrato público; debería ser claims en context.

2) Secret management mezclado
	 Usa SIGNING_MASTER_KEY para cifrar secretos MFA. Mezcla “signing” con “encryption at rest”.

3) Falta de no-store en respuestas sensibles
	 /enroll devuelve secret_base32 y no fuerza Cache-Control: no-store.

4) Manejo de errores inconsistente
	 En disable se ignoran errores de decrypt/decode; en verify/challenge se manejan correctamente.

5) Store capabilities parciales / runtime coupling
	 MFA depende de métodos en core.Repository (UpsertMFATOTP, UseRecoveryCode, etc.).
	 Si un store no los implementa, algunos flujos fallan en runtime (a veces con 501, a veces genérico).

6) Lógica auth mezclada con MFA
	 /challenge emite tokens y persiste refresh: esto cruza dominios y vuelve difícil testear/refactorizar.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
Objetivo: separar MFA (dominio) de Auth (emisión de tokens) y eliminar el header X-User-ID.

FASE 1 — Contrato y seguridad
	- Reemplazar X-User-ID por claims (RequireAuth + GetClaims) en enroll/verify/disable/rotate.
	- Agregar Cache-Control: no-store a enroll/verify/rotate (y cualquier respuesta con secretos).
	- Centralizar SecretsManager: Encrypt/Decrypt para secretos MFA (clave dedicada).

FASE 2 — Services
	- services/mfa_totp_service.go:
			Enroll(uid) -> secret + otpauth_url
			Verify(uid, code) -> (enabled, recoveryCodes?)
			ValidateSecondFactor(uid, code|recovery) -> ok
			Disable(uid, password, code|recovery)
			RotateRecovery(uid, password, code|recovery) -> recoveryCodes
	- services/auth_challenge_service.go:
			ConsumeMFAToken(mfa_token, code|recovery, remember?) -> access+refresh

FASE 3 — Infra/Repos
	- repos/mfa_repository.go: Get/Upsert/Confirm/Disable/UsedAt
	- repos/recovery_repository.go: Use/Insert/Delete
	- repos/trusted_device_repository.go: Add/Validate/Prune
	- cache/challenge_store.go: Get/Delete (mfa:token:*)
	- security/secrets_manager.go: EncryptString/DecryptString (sin SIGNING_MASTER_KEY compartida)






oauth_authorize.go — OAuth 2.1 / OIDC Authorization Endpoint (GET /authorize) con PKCE + sesión por cookie + fallback Bearer + step-up MFA (TOTP)

Qué es este archivo (la posta)
------------------------------
Este archivo define `NewOAuthAuthorizeHandler(c, cookieName, allowBearer)` que devuelve un `http.HandlerFunc`
para implementar el endpoint de autorización (Authorization Endpoint) del flow Authorization Code:

- Valida request OAuth/OIDC básica: response_type=code, client_id, redirect_uri, scope con openid, state/nonce opcionales.
- Obliga PKCE S256 (code_challenge + code_challenge_method=S256).
- Resuelve client + tenant (FS/DB) vía `helpers.LookupClient`.
- Autentica usuario por:
  1) Cookie de sesión (SID) contra cache (sid:<sha256(cookie)>)
  2) (Opcional) Bearer token como fallback (si allowBearer=true)
- Hace step-up de MFA: si el usuario tiene TOTP confirmada y sólo viene "pwd" en AMR, corta y devuelve JSON `mfa_required`
  (o auto-eleva AMR si hay trusted device cookie).
- Si todo OK: genera Authorization Code opaco, lo guarda en cache (`code:<code>`), y redirige a redirect_uri con `code` (+ state).

Esto NO emite tokens directamente: sólo genera el “code” para que después /token lo canjee.

Fuente de verdad / multi-tenant (FS vs DB)
------------------------------------------
- El “client lookup” y tenantSlug se resuelven por `helpers.LookupClient(ctx, r, clientID)`.
  Esto suele esconder el gating: DB primero, fallback FS provider (según cómo esté implementado helpers).
- Para features extra (MFA / trusted devices), el handler intenta elegir `activeStore` con precedencia:
  - tenantDB (si `c.TenantSQLManager != nil` y `cpctx.ResolveTenant(r)` engancha un slug válido)
  - fallback a `c.Store` global
- OJO: `tenantSlug` que devuelve LookupClient se usa para “legacy mapping” y match de tenant con la sesión.
  `activeStore` en cambio se resuelve con `cpctx.ResolveTenant(r)` (otro mecanismo). Esto puede desalinearse si:
  - resolveTenant() no coincide con el tenant del clientID
  - el request no trae suficiente info para resolver tenant (depende del middleware / host / path issuer mode)

Rutas / contrato (lo que expone realmente)
------------------------------------------
- Handler pensado para: GET (probablemente montado como /v1/oauth/authorize o similar; acá no se ve el router).
- Entradas por query string:
  - response_type=code (obligatorio)
  - client_id (obligatorio)
  - redirect_uri (obligatorio)
  - scope (obligatorio; debe incluir "openid")
  - state (opcional)
  - nonce (opcional OIDC)
  - code_challenge (obligatorio PKCE)
  - code_challenge_method=S256 (obligatorio PKCE)
  - prompt (opcional; si incluye "none" y no hay sesión, devuelve error al redirect)
- Respuestas:
  - 302 Found redirect a redirect_uri?code=...&state=...
  - 302 Found redirect con error=... (si prompt=none y falta login)
  - 200 OK JSON `{status:"mfa_required", mfa_token:"..."}` (cuando requiere step-up)
  - 4xx JSON (httpx.WriteError) para errores “antes” de validar redirect (invalid_request, invalid_client, invalid_scope, invalid_redirect_uri)
  - 405 si no es GET

Flujo paso a paso (secuencia real)
----------------------------------
1) Método: solo GET, setea `Vary: Cookie` y si allowBearer `Vary: Authorization`.
2) Elegir `activeStore`:
   - intenta tenant store via `c.TenantSQLManager.GetPG(ctx, cpctx.ResolveTenant(r))`
   - si falla o no hay manager, usa `c.Store` global
   (ScopesConsents repo aparece comentado/unusado)
3) Leer y validar query params:
   - response_type == "code"
   - client_id, redirect_uri, scope no vacíos
   - scope debe incluir "openid"
   - PKCE: code_challenge_method=S256 y code_challenge no vacío
4) Resolver client + tenantSlug:
   - `helpers.LookupClient(ctx, r, clientID)` -> (client, tenantSlug, err)
   - Validar redirect_uri: `helpers.ValidateRedirectURI(client, redirectURI)`
   - Validar scopes: `controlplane.DefaultIsScopeAllowed(client, s)` por cada scope
5) Mapear a estructura legacy `core.Client` para match de tenant y redirectURIs:
   - cl.ID = client.ClientID
   - cl.TenantID = tenantSlug (OJO: acá tenantID “legacy” es slug, no UUID)
6) Autenticación:
   A) Cookie SID:
      - busca cookie `cookieName`
      - key cache: `sid:` + SHA256Base64URL(cookieValue)
      - parsea `SessionPayload` (no está en este archivo)
      - valida: now < sp.Expires y sp.TenantID == cl.TenantID
      - setea: sub=sp.UserID, tid=sp.TenantID, amr=["pwd"]
   B) Fallback Bearer (si allowBearer && sub vacío):
      - parse JWT EdDSA con issuer `c.Issuer.Iss` y `c.Issuer.Keyfunc()`
      - saca sub, tid, y amr desde claims
7) Si no hay sesión válida o tenant mismatch:
   - si prompt contiene "none": redirect error login_required a redirect_uri (no UI)
   - si no: redirige a UI login:
     - UI_BASE_URL (ENV, default localhost:3000)
     - return_to = URL absoluta del authorize request
8) Step-up MFA (solo si amr == ["pwd"] y hay activeStore):
   - si store implementa `GetMFATOTP(userID)` y está confirmado:
     - intenta trusted device cookie `mfa_trust`
     - si store implementa `IsTrustedDevice(userID, deviceHash, now)` y da true -> trustedByCookie=true
     - si NO trusted: arma challenge `mfaChallenge{UserID,TenantID,ClientID,AMRBase:["pwd"],Scope:[]}`
       - genera mid opaco (24)
       - guarda en cache: `mfa_req:<mid>` ttl 5 min
       - responde 200 JSON {status:"mfa_required", mfa_token: mid}
     - si trusted: eleva AMR agregando "mfa","totp"
9) Generar authorization code:
   - code opaco (32)
   - payload `authCode{UserID,TenantID,ClientID,RedirectURI,Scope,Nonce,CodeChallenge,ChallengeMethod,AMR,ExpiresAt}`
   - guarda en cache: `code:<code>` ttl 10 min
10) Redirect success:
    - redirect_uri?code=<code>[&state=<state>]

Seguridad / invariantes (las importantes)
-----------------------------------------
- PKCE S256 obligatorio (bien, 2025 vibes): evita code interception.
- Validación estricta de redirect_uri contra el client (bien, corta open-redirect).
- Validación de scopes solicitados contra scopes del client (DefaultIsScopeAllowed).
- Session binding por tenant:
  - cookie session sólo vale si tenant match con el client del authorize.
  - Bearer también se chequea contra tenant del client antes de emitir code.
- Cache-Control/Pragma no-store en redirectError, pero en success redirect NO se setea explícitamente (deuda chica).
- AMR se propaga dentro del authCode para que /token pueda emitir tokens con AMR correcto.
- `prompt=none` respeta OIDC: si no hay login, devuelve login_required (sin UI).

Patrones detectados (GoF / arquitectura)
----------------------------------------
- Adapter/Ports (implícito):
  - Usa `activeStore` con type assertions para capacidades (GetMFATOTP, IsTrustedDevice). Eso es “feature detection”
    estilo plugin: el store es un “port” y se extiende por interfaces chicas.
- Strategy:
  - Autenticación por cookie vs bearer.
  - MFA step-up según estado + trusted device.
- Facade:
  - helpers.LookupClient + helpers.ValidateRedirectURI ocultan la complejidad FS/DB y normalización.

Cosas no usadas / legacy / riesgos (marcar sin piedad)
------------------------------------------------------
- `authCode` incluye `TenantID` como string y acá se setea `tid` que viene de sesión/bearer.
  En cookie flow tid = sp.TenantID, que parece ser slug (por el match con cl.TenantID). En bearer flow tid suele ser UUID (depende del issuer).
  ⇒ Riesgo: mixed “tenant slug vs tenant UUID” en el mismo campo. Después /token puede romper o permitir cosas raras si no valida.
- `trustedByCookie` depende de un cookie `mfa_trust` que NO valida binding con tenant/client acá (sólo hash y store).
  Está ok si store lo ata bien, pero conviene remarcarlo.
- `activeStore` se resuelve por `cpctx.ResolveTenant(r)` pero el tenant del client viene de `helpers.LookupClient`.
  Si esos dos divergen, el step-up MFA podría consultar el store equivocado o fallar silencioso.
- Logging “DEBUG:” a stdout con datos de sesión/tenant (ojo con PII y logs en prod).
- `redirectError` agrega query params con `addQS` (a mano). Funciona, pero no controla si ya existe error=... o si redirect_uri tiene fragment.
  OIDC recomienda no meter params en fragment acá (igual está ok para code flow), pero mejor builder robusto.

Ideas de refactor a V2 (sin decidir nada, solo guía accionable)
--------------------------------------------------------------
Objetivo: separar “OAuth/OIDC controller” de autenticación de sesión, MFA y cache persistence.

1) DTO / parsing
- Crear un `AuthorizeRequestDTO` con parse/validate:
  - required params, PKCE, openid en scope
  - normalizar scope a []string y también mantener raw
- Centralizar errores:
  - si falla antes de validar redirect -> JSON httpx.WriteError
  - si ya hay redirect válido -> redirectError (siempre)

2) Controller (HTTP)
- `AuthorizeController.Handle(w,r)` que sólo:
  - parse request
  - resolve client
  - auth session
  - decide “need_login / need_mfa / issue_code”
  - mapear a response (redirect o JSON)

3) Services
- `ClientResolverService`:
  - ResolveClient(ctx, req) -> (client, tenantCtx) y validar redirect/scopes
- `SessionAuthService`:
  - FromCookie(cookieName) -> (user, tenant, amr)
  - FromBearer(issuer) -> (user, tenant, amr)
  - En ambos: output normalizado con tenantID canonical (ideal: UUID) + tenantSlug si hace falta.
- `MFAService`:
  - RequiresStepUp(ctx, userID) -> bool, details
  - TrustedDevice(ctx, userID, deviceCookie) -> bool
  - CreateChallenge(ctx, payload) -> mfa_token
- `AuthCodeService`:
  - Create(ctx, payload) -> code
  - Persist(ctx, code, payload, ttl)
  (y que /token consuma desde el mismo service)

4) Repo / Cache
- Abstraer c.Cache con una interfaz:
  - Get/Set con prefijos tipados (sid, code, mfa_req) para no mezclar strings sueltas.
- Meter TTLs como constantes/config (10m code, 5m mfa) en un solo lugar.

5) Patrones GoF que sí sumarían
- Chain of Responsibility (middleware-ish) dentro del handler:
  - ValidateRequest -> ResolveClient -> Authenticate -> StepUpMFA -> IssueCode
  Te deja testeable por etapas sin reventar el handler.
- Strategy para AuthMethod (CookieAuthStrategy vs BearerAuthStrategy).
- Builder para redirect URL (evitar addQS manual y manejar query/fragment bien).

6) Concurrencia
- Acá NO hace falta: todo es lectura + writes chicos en cache.
  La única “paralelización” útil sería en futuras versiones si querés buscar MFA + session en paralelo,
  pero es overkill y te complica (y además cache+DB no siempre conviene).

Resumen
-------
- Este handler es el “/authorize” de tu OIDC: valida PKCE+openid, autentica por cookie/bearer, hace step-up MFA y emite auth code en cache.
- Lo más jugoso para V2 es normalizar tenant identifiers (slug vs UUID), alinear ResolveTenant (cpctx) con LookupClient (helpers),
  y separar responsabilidades (auth/mfa/code/cache) para que no sea un god handler a medida que crece.




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




oauth_introspect.go — OAuth2 Token Introspection (POST) con auth de cliente + soporte refresh opaco y access JWT (EdDSA)

Qué es este archivo (la posta)
------------------------------
Este archivo define `NewOAuthIntrospectHandler(c, auth)` que expone un endpoint de introspección estilo RFC 7662:
- Exige **autenticación del cliente** (via `clientBasicAuth` inyectado).
- Recibe `token` por **x-www-form-urlencoded** (`r.ParseForm()`).
- Devuelve siempre **200 OK** con `{ "active": true|false, ... }` para tokens inválidos (comportamiento típico de introspection).
- Soporta 2 tipos de token:
  1) **Refresh token opaco** (nuestro formato) => lookup en DB por hash.
  2) **Access token JWT** (EdDSA) => valida firma usando keystore/JWKS y revisa expiración + issuer esperado.

Rutas soportadas / contrato
---------------------------
- POST (path depende del router; el handler no fija ruta)
  Content-Type: application/x-www-form-urlencoded
  Form:
    token=<string>
  Query opcional:
    include_sys=1|true  (solo si active=true: expone roles/perms del namespace “system” dentro de claims custom)

Respuestas:
- 405 JSON si no es POST.
- 401 JSON si falla auth del cliente.
- 400 JSON si el form es inválido o falta token.
- 200 JSON:
  - `{ "active": false }` si token inválido / no encontrado / firma inválida / expired / issuer mismatch.
  - `{ "active": true, ... }` con campos según tipo de token.

Flujo paso a paso (secuencia real)
----------------------------------
1) Validación HTTP básica:
   - Método: POST.
   - Headers: setea `Cache-Control: no-store` y `Pragma: no-cache` siempre.
2) AuthN del cliente:
   - `auth.ValidateClientAuth(r)` debe dar ok.
   - Nota: devuelve (tenantID, clientID) pero acá se ignora (solo se usa el ok).
3) Parseo del request:
   - `r.ParseForm()`
   - Lee `token` de `r.PostForm.Get("token")`.
4) Ruta A: refresh token opaco (heurística por formato)
   - Condición: `len(tok) >= 40` y **no contiene "."**.
   - Hash: `tokens.SHA256Base64URL(tok)`.
   - Busca en DB global: `c.Store.GetRefreshTokenByHash(ctx, hash)`.
   - Si no existe / error => `{active:false}`.
   - Si existe:
     - `active := rt.RevokedAt == nil && rt.ExpiresAt.After(time.Now().UTC())`
     - Responde:
       - `token_type=refresh_token`
       - `sub` = rt.UserID
       - `client_id` = rt.ClientIDText
       - `exp`/`iat` desde `ExpiresAt`/`IssuedAt`
5) Ruta B: access token JWT (EdDSA)
   - `jwtv5.Parse(tok, c.Issuer.KeyfuncFromTokenClaims(), WithValidMethods(["EdDSA"]))`
     - Keyfunc “deriva” la key desde claims (por tenant/issuer/kid) vía keystore/JWKS.
   - Si parse/firma inválida => `{active:false}`.
   - Extrae claims relevantes:
     - `exp`, `iat` (float64), `sub`, `aud`(client_id), `scope` o `scp`, `tid`, `acr`, `amr[]`.
   - `active := exp > now`.
   - Validación extra de issuer (multi-tenant / issuer mode):
     - Si hay `iss` y existe `cpctx.Provider`, intenta derivar `slug` desde `iss` (modo path / heurística de `/t/<slug>/...`).
     - Carga tenant por slug y calcula issuer esperado:
       `jwtx.ResolveIssuer(c.Issuer.Iss, ten.Settings.IssuerMode, ten.Slug, ten.Settings.IssuerOverride)`
     - Si `expected != iss` => `active=false`.
   - Construye respuesta:
     - `token_type=access_token`
     - `sub`, `client_id`, `scope` (string), `exp`, `iat`, `amr` normalizado, `tid`
     - opcional: `acr`, `jti`, `iss`
6) Extras opcionales: `include_sys`
   - Si `active=true` y query `include_sys=1|true`:
     - Busca `claims["custom"]` y dentro:
       1) Namespace recomendado: `claimsNS.SystemNamespace(c.Issuer.Iss)` (map con `roles` y `perms`)
       2) Fallback compat: clave `c.Issuer.Iss` directo (legacy)
     - Normaliza roles/perms aceptando `[]any` o `[]string`.
7) Fin:
   - Hace un `uuid.Parse(sub)` “best effort” pero **no falla** si no es UUID (solo ignora).
   - Responde JSON 200.

Dependencias (reales) y cómo se usan
------------------------------------
- `clientBasicAuth` (inyectado):
  - Port para validar auth del cliente (probablemente Basic Auth o similar).
  - Acá se usa solo como “gate” (no se cruza contra el token).
- `c.Store`:
  - Requerido para refresh token introspection: `GetRefreshTokenByHash`.
  - OJO: es global store (no per-tenant).
- `c.Issuer`:
  - `KeyfuncFromTokenClaims()` para validar JWT con keystore/JWKS (según claims/kid).
  - `c.Issuer.Iss` para namespace system y para resolver issuer esperado.
- `cpctx.Provider`:
  - Fuente de verdad para buscar tenant por slug y traer settings (IssuerMode / Override).
- `jwtx.ResolveIssuer(...)`:
  - Recalcula el issuer esperado según modo (global/path/domain) y override.
- `claimsNS.SystemNamespace(...)`:
  - Convención de nombres para ubicar claims del “system namespace” (roles/perms).
- `tokens.SHA256Base64URL`:
  - Hash del refresh opaco (y criterio de lookup).

Seguridad / invariantes (lo crítico)
------------------------------------
- Introspection protegido por auth de cliente:
  - Si no hay auth => 401 invalid_client.
  - (Pero) no valida que el `client_id` autenticado coincida con el `aud`/`client_id` del token JWT ni con `rt.ClientIDText`.
  - En introspection “formal” eso suele ser esperado: el cliente solo puede introspectar tokens propios.
- Refresh token lookup:
  - Usa hash SHA256 (no almacena el token crudo) => ok.
  - Marca active=false si revocado o expirado.
- JWT access token:
  - Valida firma EdDSA con keyfunc basada en claims (multi-tenant ready).
  - Chequea exp.
  - Chequeo extra de issuer esperado por tenant (reduce riesgo de aceptar tokens con issuer “parecido”).
- No-store/no-cache:
  - Bien puesto para que proxies no cacheen introspection.

Patrones detectados
-------------------
- Strategy / Port-Adapter:
  - `clientBasicAuth` como puerto para auth; permite cambiar implementación sin tocar handler.
- Dual-path parsing (heurística por formato):
  - “opaque refresh vs JWT” por presencia de '.' y longitud.
- Policy gate por issuer-mode:
  - Recalcula issuer esperado usando settings del tenant (control-plane) y lo compara.

Cosas no usadas / legacy / riesgos
----------------------------------
- Riesgo #1 (alto): introspection no ata el token al cliente autenticado.
  - JWT: responde `client_id` desde `aud` pero no verifica contra el cliente autenticado.
  - Refresh: devuelve datos del refresh sin verificar ownership.
  ⇒ Cualquier cliente con credenciales válidas del introspection endpoint podría consultar tokens ajenos.
- Heurística refresh opaco:
  - `len >= 40` + “sin puntos” puede matchear otros tokens opacos (o algún JWT raro sin '.').
  - Si mañana cambiás formato de refresh, esto se rompe.
- Derivación de slug desde `iss`:
  - Es heurística por split de path; si cambian rutas de issuer, puede dejar de validar bien.
  - Si no puede derivar slug, directamente no aplica la comparación expected vs iss (queda “best effort”).
- `tenantID, clientID` de `ValidateClientAuth` se ignoran:
  - Parece que estaban pensados para aplicar validaciones extra (ownership) pero quedó a medio camino.

Ideas para V2 (sin decidir nada) + guía de desarme en capas
-----------------------------------------------------------
Objetivo: que introspection sea consistente, multi-tenant real, y con autorización correcta.

1) DTO / Controller
- Controller minimal:
  - parse method + form (`token`, opcionales flags)
  - llama a service y devuelve JSON

2) Services (lógica)
- `TokenClassifier`:
  - `Detect(token) -> TokenKind{RefreshOpaque, AccessJWT, Unknown}` (sin heurística mágica hardcodeada)
- `RefreshIntrospectionService`:
  - `IntrospectRefresh(ctx, rawToken) -> IntrospectionResult`
  - verifica revocación/expiración
  - (importante) valida ownership vs client autenticado
- `JWTIntrospectionService`:
  - `ParseAndValidate(ctx, jwtRaw) -> claims + active`
  - issuer check: sacar “parse slug” a una función `TenantFromIssuer(iss)` o usar claim `tid`/`tenant` si existe.
  - ownership: valida `aud` vs client autenticado (y si es array aud, soportarlo).
- `SystemClaimsProjector`:
  - `ExtractRolesPerms(claims) -> roles, perms` (con compat legacy).

3) Repos / Ports
- `ClientAuth` (ya existe) pero devolver identidad completa:
  - tenantID, clientID, authMethod, ok
- `TenantResolver`:
  - resolver tenant desde `tid` claim primero (más confiable) y fallback a `iss`.
- `RefreshTokenRepository` con método tenant/client aware:
  - `GetByHash(ctx, tenantID, clientID, hash)` o por lo menos “check ownership”.

4) Patrones GoF aplicables
- Strategy:
  - `IntrospectionStrategy` por tipo de token (refresh vs jwt).
- Chain of Responsibility:
  - pipeline de validaciones (parse -> signature -> issuer -> expiry -> ownership -> projection).
- Adapter:
  - adaptar los distintos formatos de claims (`aud` string vs array) y `scope` vs `scp`.

Concurrencia
------------
No hay ganancia real usando goroutines: es 1 lookup DB o 1 parse JWT + 1 lookup control-plane.
Si querés optimizar:
- Podés hacer issuer expected + parse claims en paralelo, pero la complejidad no lo vale.
Mejor: caching corto del tenant resolve (slug->settings) si esto pega muy seguido.

Resumen
-------
- Introspect endpoint sólido en “active false on invalid”, con soporte refresh opaco + JWT EdDSA y chequeo de issuer esperado.
- Punto rojo: falta autorización de ownership (atar token al cliente autenticado) y hay heurísticas/compat legacy que conviene encapsular.




oauth_revoke.go — OAuth2 Token Revocation (RFC 7009-ish) para refresh opaco (hash+DB) con inputs flexibles

Qué es este archivo (la posta)
------------------------------
Este archivo define `NewOAuthRevokeHandler(c)` que implementa un endpoint de “revocación” estilo RFC 7009:
- Acepta un `token` a revocar (principalmente **refresh token opaco** de HelloJohn).
- Hace la operación **idempotente** y “no filtrante”: si no existe, igual responde OK.
- Solo revoca refresh tokens persistidos en DB (lookup por hash SHA256 Base64URL).
- Es deliberadamente permisivo con el formato de entrada (form / bearer / JSON) para compat con distintos clientes.

Rutas soportadas / contrato
---------------------------
- POST (la ruta la define el router externo; el handler no fija path)
  Entrada (cualquiera de estas):
  1) x-www-form-urlencoded: `token=<...>`  (+ opcional `token_type_hint`, se ignora)
  2) Header: `Authorization: Bearer <token>` (fallback si no vino en form)
  3) JSON `{ "token": "..." }` (fallback si Content-Type incluye application/json)

Respuestas:
- 405 JSON si no es POST.
- 400 JSON si no hay token (input mal formado).
- 200 OK siempre que el input sea “bien formado”, haya o no token en DB (RFC7009 behavior).

Flujo paso a paso
-----------------
1) Validación de método:
   - Solo POST, si no => `httpx.WriteError(..., 405, ..., 1000)`.
2) Defensa de tamaño:
   - `r.Body = http.MaxBytesReader(..., 32<<10)` (32KB) antes de parsear.
3) Parseo primario:
   - `r.ParseForm()` y lee `token` de `r.PostForm.Get("token")`.
4) Fallbacks para token:
   - Fallback #1: `Authorization: Bearer ...` si no había token en el form.
   - Fallback #2: si `Content-Type` contiene `application/json`, intenta decode de `{token}` con `io.LimitReader` (32KB).
5) Validación:
   - Si token sigue vacío => 400 `invalid_request`.
6) Revocación (best effort, no filtrante):
   - Calcula `hash := tokens.SHA256Base64URL(token)`.
   - `c.Store.GetRefreshTokenByHash(ctx, hash)`:
     - Si existe => `_ = c.Store.RevokeRefreshToken(ctx, rt.ID)` (ignora error).
     - Si error != nil y != core.ErrNotFound => se ignora (para no filtrar info).
7) Respuesta final:
   - Setea `Cache-Control: no-store` y `Pragma: no-cache`.
   - `w.WriteHeader(200)` siempre (si el input fue válido).

Dependencias (reales)
---------------------
- `c.Store`:
  - `GetRefreshTokenByHash(ctx, hash)` para buscar refresh token persistido.
  - `RevokeRefreshToken(ctx, rt.ID)` para marcar revocado.
- `tokens.SHA256Base64URL(token)`:
  - Hash del token opaco (formato consistente con cómo se persisten refresh en DB).
- `httpx.WriteError(...)`:
  - Wrapper de errores JSON con códigos internos.
- `core.ErrNotFound`:
  - Permite distinguir “no existe” vs error real, pero igual no se expone nada.

Seguridad / invariantes
-----------------------
- No filtración:
  - Nunca revela si el token existía o no (200 OK igual).
  - Ignora errores internos (salvo “input inválido”), evitando side-channels.
- Limitación de payload:
  - 32KB evita DoS por bodies gigantes (bien).
- No-store:
  - Correcto para proxies/cache.

OJO / Riesgos (cosas a marcar)
------------------------------
- No autentica al cliente que revoca:
  - RFC 7009 normalmente requiere auth del cliente (o algún criterio).
  - Acá cualquiera que pegue al endpoint con un refresh token válido podría revocarlo.
  - Capaz es intencional (logout “self-service”), pero aumenta superficie si se filtra un RT.
- Solo revoca refresh opaco persistido:
  - Si te pasan un access JWT o un refresh de otro formato, no hace nada pero responde 200.
  - Está ok por idempotencia, pero conviene dejarlo explícito a nivel contrato.
- `token_type_hint` ignorado:
  - Bien para idempotencia y simpleza, pero si querés compat RFC completa, podrías usarlo como hint para evitar lookup inútil.

Patrones detectados
-------------------
- Tolerant Reader / Robust Input Handling:
  - Soporta múltiples formatos de entrada (form/header/json) como “Adapter” de request.
- Idempotent Command:
  - Revocar es “best effort” y responde OK aunque ya esté revocado/no exista.

Ideas para V2 (sin decidir nada) + guía de desarme en capas
-----------------------------------------------------------
1) Controller / DTO
- Controller minimal:
  - `ExtractToken(r) -> (token, ok, errCode)` (unificar los 3 caminos).
  - Devuelve 400 si no hay token, 200 si hay token (sin decir nada más).

2) Service (negocio)
- `RevocationService.Revoke(ctx, token, clientContext)`
  - Clasifica token por formato (refresh opaque vs jwt).
  - Si refresh opaque: lookup + revoke.
  - Si JWT: (opcional) revocar vía denylist/jti store si existe.
  - Siempre retorna éxito lógico (para idempotencia), y loggea internamente errores inesperados.

3) Ports/Repos
- `RefreshTokenRepository`:
  - `GetByHash(ctx, hash)`
  - `RevokeByID(ctx, id)`
  - (opcional) `RevokeByHash(ctx, hash)` para evitar 2 llamadas.
- `ClientAuthenticator` (si decidís exigir auth):
  - Similar a `clientBasicAuth` que ya usan otros oauth_* handlers.
  - Permite exigir client auth, o permitir modo “public” solo para ciertos flows.

4) Seguridad extra opcional
- Si querés hacerlo más estricto:
  - Requerir client auth por default.
  - O permitir sin auth solo si el token viene del mismo “session context” (cookie/session) y el request está autenticado.
- Agregar rate-limit por IP / token hash prefix (para evitar brute).

Concurrencia
------------
No suma meter goroutines: es 1 lookup + 1 update. Lo importante es robustez y auth, no paralelismo.

Resumen
-------
- Handler chico, claro y práctico: parsea token de 3 formas, hashea, busca refresh en DB y revoca si existe.
- Es idempotente y no filtra información (bien), pero hoy no valida quién está autorizado a revocar (ojo si el endpoint queda público).





oauth_token.go — Token Endpoint OAuth2/OIDC “todo-en-uno”: auth_code(PKCE)+refresh(rotación)+client_credentials(M2M)

Qué es este archivo (la posta)
------------------------------
Este archivo implementa el endpoint `/token` (OAuth2 Token Endpoint) en un solo handler gigantesco:
- Multiplexa por `grant_type`:
  1) `authorization_code` (con PKCE S256) -> emite access + refresh + id_token
  2) `refresh_token` (rotación)            -> emite access + refresh nuevo (y revoca el viejo)
  3) `client_credentials` (M2M)           -> emite access (sin refresh)
- Resuelve “store activo” con precedencia `tenantDB > globalDB` porque:
  - los refresh tokens están en DB (y en multi-tenant cada tenant tiene su propio schema/DB)
  - además lee user metadata + RBAC desde el store para armar claims “SYS namespace”
- Resuelve issuer efectivo por tenant (issuerMode: global/path/domain + override) para:
  - firmar con issuer correcto
  - poner system claims en namespace correcto (claimsNS.SystemNamespace)

Este archivo ES el core del login OIDC: si acá falla, se cae todo.

Entradas / Formatos admitidos
-----------------------------
- Solo POST.
- Content-Type esperado: `application/x-www-form-urlencoded` (OAuth2 estándar).
- Lee `grant_type` y el resto de campos desde `r.PostForm`.
- Limita body a 64KB con `http.MaxBytesReader`.
- Timeout hard: 3s para todo el handler (context.WithTimeout).

⚠️ Nota: con 3s, cualquier lookup lento (DB, provider, cache) te puede pegar timeout y devolver errores 500/timeout.

Pieza clave: “Active Store” y por qué importa tanto
---------------------------------------------------
El handler elige `activeStore` así:
1) Si hay `TenantSQLManager`, intenta `GetPG(ctx, cpctx.ResolveTenant(r))`.
2) Si no, cae a `c.Store` global.

Pero OJO: en `authorization_code` y `refresh_token`, **luego vuelve a re-seleccionar store**
basado en el `tenantSlug` real obtenido al resolver el client (`helpers.LookupClient`).
Eso es CRÍTICO porque:
- el código/consent pueden estar en cache “global”, pero el refresh token va a DB del tenant
- si usás el store equivocado, te explota por FK / “token not found” / escribir en el tenant incorrecto.

Caminos principales (por grant_type)
====================================

A) grant_type = authorization_code (OIDC code flow + PKCE)
---------------------------------------------------------
Entrada esperada (form):
- grant_type=authorization_code
- code=...
- redirect_uri=...
- client_id=...
- code_verifier=...  (PKCE S256)

Pasos:
1) Validación de campos obligatorios.
2) Resolve client + tenantSlug:
   - `client, tenantSlug := helpers.LookupClient(ctx, r, clientID)`
   - Valida existencia del client. (Acá NO valida secret todavía; está comentado.)
3) Re-selección del store por tenantSlug:
   - `TenantSQLManager.GetPG(ctx, tenantSlug)` si existe.
4) “Compat layer” a core.Client legacy:
   - cl.ID = client.ClientID
   - cl.TenantID = tenantSlug
   - cl.RedirectURIs = client.RedirectURIs
   - cl.Scopes = client.Scopes
5) Consume authorization code desde cache (one-shot):
   - key := "code:" + code   (match con authorize handler)
   - si no existe => invalid_grant
   - delete inmediato (one-shot)
   - unmarshal -> authCode payload
6) Validaciones del authCode:
   - no expirado
   - coincide `client_id` y `redirect_uri`
   - PKCE: compara challenge S256:
       verifierHash := tokens.SHA256Base64URL(code_verifier)
       ac.CodeChallenge debe == verifierHash
7) Construcción de claims para Access Token:
   - scopes: strings.Fields(ac.Scope)
   - amr: ac.AMR
   - acr: loa1 o loa2 si amr incluye "mfa"
   - std claims incluye:
       tid, amr, acr, scope, scp
   - custom claims inicial vacío
   - hook: `applyAccessClaimsHook(...)` (puede modificar std/custom)
8) Resolver issuer efectivo del tenant:
   - effIss = jwtx.ResolveIssuer(baseIss, issuerMode, slug, override)
9) SYS namespace claims (roles/perms/metadata):
   - activeStore.GetUserByID(ac.UserID)
   - si store soporta rbacReader => roles/perms
   - helpers.PutSystemClaimsV2(custom, effIss, metadata, roles, perms)
10) Emitir access token:
   - c.Issuer.IssueAccessForTenant(tenantSlug, effIss, userID, clientID, std, custom)
11) Emitir refresh token (rotación):
   - requiere hasStore=true (DB disponible)
   - si store soporta `CreateRefreshTokenTC(tenantID, clientID, userID, ttl)`:
       - intenta resolver realTenantID (UUID) via Provider.GetTenantBySlug
       - crea refresh “TC” (token crudo lo genera el store)
   - else legacy:
       - genera rawRT (opaque)
       - guarda hash en DB usando:
         a) CreateRefreshTokenTC(...) legacy raro (hash hex) o
         b) CreateRefreshToken(...) clásico (hash base64url)
       - acá hay mezcla de hashes/formats: es una fuente de bugs
12) Emitir ID Token (OIDC):
   - idStd: tid, at_hash(access), azp, acr, amr
   - idExtra: nonce si existe
   - hook: applyIDClaimsHook(...)
   - c.Issuer.IssueIDTokenForTenant(...)
13) Respuesta JSON no-store:
   - access_token, refresh_token, id_token, expires_in, scope

Puntos sensibles / donde se rompe:
- Si authorize guardó code en otro prefijo => invalid_grant.
- Si PKCE hash no coincide (ojo, acá usa SHA256Base64URL del verifier, no la fórmula exacta base64url(SHA256(verifier)) “sin hex”; asumimos que SHA256Base64URL hace eso).
- Si tenantSlug y tenantID real se confunden: refresh token TC pide tenant UUID; code trae tid como slug.

B) grant_type = refresh_token (rotación)
----------------------------------------
Entrada esperada:
- grant_type=refresh_token
- client_id=...
- refresh_token=...

Pasos:
1) Validación: requiere DB (hasStore).
2) Resolve client + tenantSlug con LookupClient.
3) Re-selección del store por tenantSlug (CRÍTICO).
4) “Compat layer” core.Client legacy (igual que arriba).
5) Lookup refresh token:
   - Si store soporta tcRefresh:
       - hash := tokens.SHA256Base64URL(refresh_token)
       - GetRefreshTokenByHashTC(ctx, tenantSlug, client.ClientID, hash)
       - OJO: acá tenantSlug se pasa como tenantID: si la implementación espera UUID, cagaste.
   - else legacy:
       - GetRefreshTokenByHash(ctx, hash)
6) Validaciones del refresh:
   - no revocado
   - no expirado
   - rt.ClientIDText == client.ClientID (mismatch => invalid_grant)
7) Construcción claims access:
   - amr=["refresh"], acr loa1, tid=tenantSlug, scp vacío
   - hook applyAccessClaimsHook
   - SYS claims igual (GetUserByID + roles/perms) con issuer efectivo
8) Emitir access:
   - IssueAccessForTenant(tenantSlug, effIss, rt.UserID, clientID, std, custom)
9) Rotación refresh:
   - Si tcRefresh:
       - RevokeRefreshTokensByUserClientTC(tenantSlug, client.ClientID, rt.UserID)
       - CreateRefreshTokenTC(tenantSlug, client.ClientID, rt.UserID, ttl)
       - (revoca “todos” del user+client; agresivo pero simple)
   - else legacy:
       - genera newRT y guarda CreateRefreshToken(... parentID=&rt.ID)
       - revoca el viejo rt
10) Respuesta:
   - access_token + refresh_token nuevo + expires_in

Puntos sensibles:
- Inconsistencia de “tenant identifier”: a veces slug, a veces UUID.
- Hashing: TC vs legacy usan funciones distintas (base64url vs hex) en el mismo archivo.
- Rotación por “revoke all user+client” te puede romper multi-device (depende del producto).

C) grant_type = client_credentials (M2M)
----------------------------------------
Entrada:
- grant_type=client_credentials
- client_id=...
- client_secret=...
- scope=... (opcional)

Pasos:
1) LookupClient -> (client, tenantSlug)
2) Validaciones:
   - client.Type debe ser "confidential"
   - ValidateClientSecret(...) debe pasar
3) Validar scopes:
   - requested scopes debe ser subset de client scopes (DefaultIsScopeAllowed)
4) Construcción claims:
   - amr=["client"], acr loa1
   - tid=tenantSlug
   - scopeOut: si viene scope en req => ese; sino default client.Scopes
   - std["scope"/"scp"] set
   - hook applyAccessClaimsHook
5) Resolver issuer efectivo por tenant
6) Emitir access:
   - sub = clientID (emite “on behalf of client”)
7) Respuesta JSON:
   - access_token + scope + expires_in (sin refresh)

Puntos sensibles:
- Si ValidateClientSecret depende de provider/secret storage lento -> timeout 3s.
- “tid” en M2M: hoy es slug; si más adelante querés UUID, esto cambia claim contract.

Problemas gordos detectables (de diseño, no de estilo)
------------------------------------------------------
1) Mezcla de responsabilidades a lo pavote:
   - parsing + validación oauth
   - lookup client / tenant
   - cache (code)
   - store selection + persist refresh
   - issuer resolution + firma
   - RBAC/metadata -> custom claims
   - hooks
   Todo en una función.

2) Identidad de tenant inconsistente (slug vs UUID):
   - authCode.TenantID a veces es slug (según authorize)
   - CreateRefreshTokenTC a veces exige UUID (por FK)
   - refresh_token flow usa tenantSlug en calls TC
   => esto es bug waiting to happen.

3) Hash formats inconsistentes:
   - SHA256Base64URL vs SHA256Hex (aparece en legacy TC path)
   - y encima el introspect/revoke usan SHA256Base64URL
   => si guardás con hex y buscás con base64url, no lo encontrás nunca.

4) Cache key inconsistente entre handlers:
   - acá usa "code:"+code
   - consent handler usa "oidc:code:"+SHA256Base64URL(code)
   => hay dos “familias” de codes. Si mezclás flows, invalid_grant.

5) Timeout fijo 3s para TODO:
   - en prod con DB medio lenta o provider remoto, te va a cortar piernas.

Cómo separarlo bien (V2) — por capas y por caminos (bien concreto)
==================================================================

Objetivo
--------
Que el handler quede como “controller” finito:
- parsea request
- llama a un service por grant_type
- traduce errores a OAuth JSON
y que lo heavy viva en servicios/repos con interfaces claras.

Carpeta sugerida
----------------
/internal/oauth/
  token_controller.go          // HTTP handler, parse + routing por grant
  token_dtos.go                // request/response structs + validation
  token_errors.go              // mapping a {error, error_description}
  services/
    token_service.go           // interface + orchestración general
    auth_code_service.go       // GrantAuthorizationCode
    refresh_service.go         // GrantRefreshToken
    client_credentials_service.go // GrantClientCredentials
  ports/
    client_registry.go         // Lookup client, validate redirect/secret, allowed scopes
    code_store.go              // guardar/consumir auth codes (cache)
    consent_store.go           // opcional
    refresh_tokens.go          // crear/buscar/revocar refresh (UNIFICADO)
    user_repo.go               // GetUserByID + RBAC
    issuer.go                  // Resolve issuer + Issue access/id tokens
    hooks.go                   // access/id hooks
  adapters/
    controlplane_client_registry.go
    cache_code_store.go
    pg_refresh_tokens_repo.go
    pg_user_repo.go
    issuer_adapter.go

1) Controller (HTTP)
--------------------
Responsabilidad:
- Enforce POST
- Parse form (64KB)
- Crear “request context” (timeout configurable)
- Construir DTO según grant_type
- Llamar `TokenService.Exchange(ctx, dto)`
- Responder JSON no-store

DTOs (ejemplo mental):
- AuthCodeTokenRequest { code, redirectURI, clientID, codeVerifier }
- RefreshTokenRequest { clientID, refreshToken }
- ClientCredentialsRequest { clientID, clientSecret, scope }

Errores:
- Mapear a OAuth estándar:
  - invalid_request
  - invalid_client
  - invalid_grant
  - unsupported_grant_type
  - invalid_scope
  - server_error
Y siempre setear no-store.

2) Service por grant (negocio/orquestación)
-------------------------------------------
Cada grant en su servicio con un flujo claro y testeable.

A) AuthCodeService.Exchange(...)
- Steps:
  1) client := ClientRegistry.Lookup(clientID) -> devuelve (client, tenantRef)
  2) ClientRegistry.ValidateRedirect(client, redirectURI)
  3) codePayload := CodeStore.Consume(code) (one-shot)  // UN SOLO FORMATO de key
  4) Validate codePayload: exp, client match, redirect match, pkce
  5) tenant := TenantResolver.Resolve(tenantRef) -> {tenantSlug, tenantUUID, issuerEffective}
  6) user := UserRepo.GetUser(codePayload.userID)
  7) claims := ClaimsBuilder.BuildAccessClaims(tenant, user, reqScopes, amr, hooks)
  8) access := Issuer.IssueAccess(tenantSlug, issuerEffective, userID, clientID, claims)
  9) refresh := RefreshTokens.RotateOrCreate(tenantUUID, clientID, userID, ttl)
  10) idToken := Issuer.IssueIDToken(... at_hash, nonce, acr/amr)
  11) return TokenResponse{access, refresh, id_token, exp, scope}

B) RefreshService.Exchange(...)
- Steps:
  1) client := ClientRegistry.Lookup(clientID) -> tenantRef
  2) tenant := TenantResolver.Resolve(tenantRef)
  3) rt := RefreshTokens.ValidateAndGet(tenantUUID, clientID, refreshToken)
  4) claims builder (amr=["refresh"])
  5) access := issue
  6) newRefresh := RefreshTokens.Rotate(tenantUUID, clientID, userID, refreshToken, ttl)
  7) return response

C) ClientCredentialsService.Exchange(...)
- Steps:
  1) client := ClientRegistry.Lookup(clientID)
  2) Validate confidential + ValidateSecret
  3) Validate scope subset
  4) tenant resolve => issuerEffective
  5) issue access sub=clientID
  6) return response

3) Ports (interfaces) — donde se corta lo feo
---------------------------------------------
A) TenantResolver / ClientRegistry
- ClientRegistry:
  - LookupClient(ctx, clientID) -> (client, tenantSlug)
  - ValidateClientSecret(ctx, tenantSlug, client, secret)
  - IsScopeAllowed(client, scope) bool
- TenantResolver:
  - ResolveBySlug(ctx, slug) -> {TenantSlug, TenantUUID, IssuerEffective}
  - (cacheable)

B) CodeStore (cache)
- Consume(code string) (*AuthCodePayload, error)
- Store(payload) (si el authorize lo hace acá también)
Regla: UN SOLO esquema de key:
- "oidc:code:"+code (raw) o hash, pero **uno**.
Yo prefiero hash:
- key := "oidc:code:"+SHA256Base64URL(code)
porque te evita keys enormes y logs con token crudo.

C) RefreshTokensRepository (UNIFICAR)
Este es EL punto más importante.
Definí una interfaz única:
- Create(ctx, tenantUUID, clientIDText, userID, ttl) (rawToken string, err)
- GetByRaw(ctx, tenantUUID, clientIDText, rawToken) (*RefreshToken, err)
- Revoke(ctx, tenantUUID, tokenID)
- Rotate(ctx, tenantUUID, clientIDText, rawToken, ttl) (newRaw string, err)

Y adentro decidís:
- siempre persistir hash con el MISMO algoritmo (ej SHA256Base64URL)
- nunca mezclar hex/base64url

D) UserRepo / RBAC
- GetUserByID(ctx, userID) -> user(metadata)
- GetRoles/GetPerms opcional
Esto evita type assertions en runtime.

E) IssuerPort
- ResolveIssuer(ctx, tenantSlug) -> string (efectivo)
- IssueAccess(ctx, tenantSlug, effIss, sub, aud, std, custom) -> (jwt, exp)
- IssueIDToken(ctx, ...) -> (jwt, exp)

F) HooksPort
- ApplyAccessClaimsHook(...)
- ApplyIDClaimsHook(...)
Como un decorator que modifica maps.

4) Qué queda en cada capa (resumen ultra claro)
-----------------------------------------------
HTTP Controller:
- parse + validate “shape” de request
- mapping errores oauth
- no-store headers

Services:
- orquestación del grant
- decisiones (rotación, scopes, acr/amr)
- llama a ports

Adapters:
- Cache (c.Cache)
- Control plane provider (cpctx.Provider)
- TenantSQLManager (GetPG)
- DB repos (refresh/user/rbac)
- Issuer real (c.Issuer)

Contrato interno recomendado (para no repetir bugs)
===================================================
- TenantRef interno SIEMPRE incluye:
  - tenantSlug (para issuer keys y rutas)
  - tenantUUID (para DB FK)
- Hash de refresh token SIEMPRE:
  - `tokens.SHA256Base64URL(raw)` (y listo)
- Key de auth code SIEMPRE:
  - prefijo único + hash (no raw) o raw consistente en TODO el sistema
  - y “consume” siempre borra

Chequeos extra que yo metería (sin cambiar tu producto)
--------------------------------------------------------
- Client auth en authorization_code (confidential):
  - hoy está TODO commented: cualquiera con code+verifier puede canjear si roba code.
  - mínimo: si client.Type == confidential => exigir secret o private_key_jwt.
- Rate limit por IP/client_id en /token (especial refresh).
- Observabilidad:
  - loggear request_id + tenantSlug + clientID + grant_type (sin tokens)
- Timeout por dependencia:
  - 3s total es medio agresivo; mejor:
    - 3s para DB
    - 1s para provider
    - 50ms para cache
  con context sub-timeouts adentro del service.


oidc_discovery.go — OIDC Discovery (global + per-tenant) + issuer/jwks por tenant

Qué es este archivo (la posta)
------------------------------
Este archivo publica documentos “discovery” para clientes OIDC:
	- Discovery global: un solo issuer (c.Issuer.Iss) + endpoints globales
	- Discovery por tenant: issuer resuelto por settings del tenant + jwks_uri por tenant

En la práctica, estos endpoints son los que consumen:
	- SPAs/Frontends (para saber authorize/token/userinfo/jwks)
	- SDKs/CLI (para bootstrap)

Ojo: acá NO se registran rutas ni se usa chi params; el handler por-tenant parsea el path “a mano”.

Dependencias reales
-------------------
- c.Issuer.Iss: base issuer global (string)
- cpctx.Provider (solo per-tenant): lookup de tenant por slug
- jwtx.ResolveIssuer(base, issuerMode, slug, issuerOverride): define issuer efectivo por tenant
- httpx.WriteJSON / httpx.WriteError: contrato de JSON error/response
- setNoStore(w): helper compartido (definido en jwks.go) para evitar cache en respuestas sensibles

Rutas soportadas (contrato efectivo)
------------------------------------
A) Discovery global
-------------------
- GET/HEAD /.well-known/openid-configuration
		(ruta exacta depende del wiring del router, pero este handler asume que se monta ahí)

		Response:
			- issuer = strings.TrimRight(c.Issuer.Iss, "/")
			- authorization_endpoint = {issuer}/oauth2/authorize
			- token_endpoint         = {issuer}/oauth2/token
			- userinfo_endpoint      = {issuer}/userinfo
			- jwks_uri               = {issuer}/.well-known/jwks.json

		Headers:
			- Cache-Control: public, max-age=600, must-revalidate
			- Expires: now+10m

B) Discovery por tenant
-----------------------
- GET/HEAD /t/{slug}/.well-known/openid-configuration
		Parsing:
			- Valida que el path tenga prefix "/t/" y suffix "/.well-known/openid-configuration"
			- Extrae slug y valida regex ^[a-z0-9\-]{1,64}$

		Fuente de verdad:
			- Requiere cpctx.Provider
			- cpctx.Provider.GetTenantBySlug(ctx, slug)
			- iss = jwtx.ResolveIssuer(base, issuerMode, slug, issuerOverride)

		Nota de compat:
			- Mantiene endpoints globales (authorize/token/userinfo) para no romper rutas existentes
			- Pero jwks_uri sí es por tenant: {base}/.well-known/jwks/{slug}.json

		Headers:
			- setNoStore(w) (no-cache/no-store) para evitar cache agresivo cuando rota issuer/jwks

Campos OIDC “de facto”
----------------------
Este discovery declara:
	- response_types_supported: ["code"]
	- grant_types_supported: ["authorization_code", "refresh_token"]
	- id_token_signing_alg_values_supported: ["EdDSA"]
	- token_endpoint_auth_methods_supported: ["none"] (public client)
	- code_challenge_methods_supported: ["S256"] (PKCE)
	- scopes_supported: ["openid","email","profile","offline_access"]

Importante: esto debe ser consistente con lo que realmente acepta /oauth2/token.

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) regexp.MustCompile por request (per-tenant)
	 En NewTenantOIDCDiscoveryHandler se compila el regex en cada request.
	 V2: declarar un var regexp global y reusar.

2) Base issuer vs “host externo”
	 Usa c.Issuer.Iss tal cual (config). Detrás de proxies puede diferir del host/scheme público.
	 Solución típica: fijar issuer explícitamente (recomendado) o derivarlo de headers confiables.

3) Contrato de endpoints parcialmente “tenant-aware”
	 El discovery por tenant cambia issuer y jwks_uri, pero deja authorize/token/userinfo globales.
	 Esto es compat-friendly, pero en V2 convendría que todo el surface sea coherente:
		 /t/{slug}/oauth2/authorize, /t/{slug}/oauth2/token, /t/{slug}/userinfo, etc.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
FASE 1 — Validación/parseo consistente
	- Usar router con params (chi) o un parser central para rutas /t/{slug}/...
	- Regex precompilada.

FASE 2 — Discovery coherente por tenant
	- Publicar endpoints tenant-scoped (si el router v2 lo soporta) o documentar formalmente
		que solo issuer/jwks varían y el resto es global.

FASE 3 — Metadata driven
	- Generar scopes/claims supported desde configuración/registries reales (evitar drift).




profile.go — “WhoAmI” / Profile resource (GET /v1/profile) basado en claims + lookup de user

Qué es este archivo (la posta)
------------------------------
Este archivo define NewProfileHandler(c) que expone un endpoint simple tipo “whoami”
para UI/CLI:
	- Lee claims desde context (inyectados por middleware RequireAuth)
	- Extrae sub (user id)
	- Busca el usuario en c.Store
	- Construye un payload “seguro” y relativamente estable (email + profile básico)

Este handler NO hace issuance de tokens, no maneja consent, y no escribe nada: es lectura.
Su valor está en ser un recurso protegido por scopes para validar que:
	- el access token es válido
	- el scope “profile:read” está funcionando
	- el aislamiento multi-tenant se respeta

Dependencias reales
-------------------
- httpx.GetClaims(ctx): claims del access token (set por RequireAuth)
- c.Store.GetUserByID(ctx, sub): lookup de usuario
- Campos de usuario usados:
		- u.ID, u.Email, u.EmailVerified
		- u.Metadata (given_name, family_name, name, picture)
		- u.TenantID (para guard multi-tenant)

Rutas soportadas (contrato efectivo)
------------------------------------
- GET /v1/profile
		Requiere:
			- middleware RequireAuth (para poblar claims)
			- middleware de scopes (en main se suele envolver con RequireScope("profile:read"))

		Response JSON (best-effort):
			{
				"sub": "<user_id>",
				"email": "...",
				"email_verified": true|false,
				"name": "...",
				"given_name": "...",
				"family_name": "...",
				"picture": "...",
				"updated_at": <unix>
			}

		Headers:
			- Content-Type: application/json
			- Cache-Control: no-store
			- Pragma: no-cache

Flujo interno (paso a paso)
----------------------------
1) Solo GET
	 - method != GET => 405

2) Claims desde context
	 - httpx.GetClaims(ctx) debe existir (si no, 401 missing_claims)
	 - Lee "sub" como string (si falta, 401 invalid_token)

3) Lookup de user
	 - c.Store.GetUserByID(ctx, sub)
	 - si no existe: 404 user_not_found

4) Guard multi-tenant (best effort)
	 - Si el token trae claim "tid": compara case-insensitive contra u.TenantID
	 - Si no coincide: 403 forbidden_tenant
	 Nota: esto es defensa-en-profundidad. El aislamiento “real” debería estar
	 garantizado antes (por issuance/validation, y por stores scoping por tenant).

5) Construcción del perfil
	 - Intenta sacar campos desde u.Metadata (map) y arma "name" si no viene
	 - updated_at hoy usa u.CreatedAt como placeholder (TODO en el código)

Seguridad / invariantes
-----------------------
- Scope enforcement:
	Este handler asume que un middleware externo exige "profile:read".
	El scope no se verifica aquí.

- Token type:
	Al depender de RequireAuth, se asume que es un access token válido.
	Hay tests e2e que buscan que no se acepte ID token como Bearer en /v1/profile.

- Cache:
	Bien: no-store para evitar que un proxy cachee PII.

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) updated_at incorrecto
	 Devuelve CreatedAt como updated_at. Es explícitamente best-effort.

2) Inconsistencia “profile” vs “metadata”
	 Toma campos de u.Metadata. En otros lugares del sistema existe profile/custom_fields.
	 Esto puede confundir a consumidores si esperan OIDC standard claims desde otro storage.

3) Multi-tenant guard depende de claim tid
	 Si tid no está presente, no hay guard acá.
	 V2: hacer que tid sea obligatorio en access tokens multi-tenant o que GetUserByID sea tenant-scoped.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
FASE 1 — Contrato de claims explícito
	- Definir un tipo Claims struct (sub, tid, scopes, amr, acr, ...)
	- Evitar map[string]any para claims.

FASE 2 — Profile service
	- services/profile_service.go:
			GetProfile(ctx, claims) -> ProfileDTO
	- Ese service decide qué campos exponer y desde qué fuente (metadata/profile/custom_fields).

FASE 3 — Aislamiento fuerte
	- Preferir repos tenant-scoped (GetUserByID(ctx, tenantID, userID))
	- O hacer obligatorio tid en tokens + validación central.



providers.go — Providers Discovery (Auth UI bootstrap): GET /v1/auth/providers (+ start_url opcional)

Qué es este archivo (la posta)
------------------------------
Este archivo implementa el endpoint “discovery” que el frontend/CLI usa para saber:
	- qué métodos de login están disponibles (password, google, ...)
	- si el provider está habilitado y correctamente configurado (Enabled/Ready)
	- si conviene abrir en popup (Popup)
	- y, para algunos providers, devuelve un start_url listo para iniciar el flujo (Google)

Es un endpoint de UX/bootstrapping, no un endpoint de seguridad.
No autentica, no emite tokens, no valida scopes: solo informa.

Dependencias reales
-------------------
- cfg (config.Config): feature flags y credenciales del provider (providers.google.* + jwt.issuer)
- c.Store: se usa indirectamente vía redirectValidatorAdapter para validar redirect_uri contra el client
	(buscar redirectValidatorAdapter en email_flows_wiring.go: se reutiliza en social/providers)

Ojo: redirectValidatorAdapter hace fallback a control-plane via cpctx.Provider y NO chequea nil.
Si corrés en un modo sin control-plane, una validación de redirect podría panic.

Ruta soportada (contrato efectivo)
----------------------------------
- GET /v1/auth/providers?tenant_id=...&client_id=...&redirect_uri=...

Query params:
	- tenant_id: UUID del tenant (opcional; solo necesario para validar redirect)
	- client_id: client_id público OIDC (opcional; solo necesario para validar redirect)
	- redirect_uri: URL final a la que el flujo social debería retornar (opcional)

Response:
	{
		"providers": [
			{
				"name": "password",
				"enabled": true,
				"ready": true,
				"popup": false
			},
			{
				"name": "google",
				"enabled": true|false,
				"ready": true|false,
				"popup": true,
				"start_url": "/v1/auth/social/google/start?..." (opcional)
				"reason": "..." (opcional; solo misconfig)
			}
		]
	}

Headers:
	- Content-Type: application/json
	- Cache-Control: no-store

Flujo interno (cómo decide lo que devuelve)
------------------------------------------
1) Método
	 - solo GET

2) Siempre incluye "password"
	 - Hardcode: Enabled=true, Ready=true
	 - Nota: esto es informativo. El gating real (si un client permite password) se aplica en otros handlers.

3) Provider Google
	 A) Enabled
			- depende de cfg.Providers.Google.Enabled

	 B) Ready (config correcta)
			- requiere ClientID y ClientSecret no vacíos
			- y además: o RedirectURL explícito, o jwt.issuer para poder derivar un callback

			Si NO está ready:
				- setea Reason con detalle “client_id/secret o redirect_url/jwt.issuer faltantes”
				- responde inmediatamente (password + google) y termina.

	 C) start_url (solo si hay suficiente contexto)
			- Si redirect_uri está vacío, intenta default:
					base = trimRight(cfg.JWT.Issuer, "/")
					redirect_uri = base + "/v1/auth/social/result"
				Si jwt.issuer está vacío, no hay redirect default.

			- Para generar start_url exige:
					tenant_id válido (UUID)
					client_id no vacío
					redirect_uri no vacío

			- Si todo está, valida redirect_uri contra el client:
					h.validator.ValidateRedirectURI(tenantUUID, clientID, redirectURI)
				Si pasa, arma:
					/v1/auth/social/google/start?tenant_id=...&client_id=...&redirect_uri=...
				Si no pasa, simplemente omite start_url (sin reason).

Qué NO hace (importante para no asumir de más)
----------------------------------------------
- No verifica que el client tenga providers ["google"] habilitado (provider gating por client).
	Solo valida redirect_uri. El gating del provider por client vive en el flujo social/auth.

- No valida allowlists de cfg.Providers.Google.AllowedTenants/AllowedClients.
	(Eso suele vivir en el handler que ejecuta el start real.)

- No diferencia readiness por tenant/client: Ready es global a la config del server.

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) Posible panic si cpctx.Provider es nil
	 redirectValidatorAdapter hace fallback a cpctx.Provider sin nil-check.
	 Acá se llama solo si hay tenant_id+client_id+redirect_uri, pero igual puede ocurrir.

2) “password” siempre Enabled/Ready
	 En el producto real, el client puede declarar providers y bloquear password.
	 Esta respuesta puede ser “optimista” y confundir al UI si no lo cruza con /v1/auth/config.

3) redirect default basado en cfg.JWT.Issuer
	 Si jwt.issuer no refleja el host público (proxy), el UI podría recibir un redirect_uri inválido.

4) Señales mezcladas (Enabled/Ready vs start_url)
	 Es posible: google Enabled+Ready pero sin start_url por falta de tenant/client/redirect.
	 Eso es correcto, pero conviene que el frontend lo trate como “necesito contexto”.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
FASE 1 — Separar “discovery global” de “availability por client/tenant”
	- /v2/auth/providers (global): Enabled/Ready por provider (sin start_url)
	- /v2/auth/providers/resolve?tenant_id&client_id&redirect_uri: devuelve start_url y gating final

FASE 2 — Validator robusto
	- Hacer que redirectValidatorAdapter no dependa de cpctx.Provider global sin nil-check.
	- Unificar validación con helpers.ValidateRedirectURI + resolver client/tenant consistente.

FASE 3 — Contrato más explícito
	- Campo "needs" o "missing" para indicar qué falta para construir start_url (tenant_id/client_id/redirect_uri)
		sin exponer “reasons” de runtime.



session_login.go — Session Cookie Login (sid) + tenant resolution (SQL/FS) + cache de sesión

Qué es este archivo (la posta)
------------------------------
Este archivo define NewSessionLoginHandler(...) que devuelve un http.HandlerFunc para:
	- Autenticar email+password ("password grant" pero en modo cookie/session)
	- Resolver el tenant de manera flexible (por tenant_id o por client_id)
	- Emitir una sesión server-side en cache (key "sid:<hash>")
	- Setear una cookie de sesión (sid) usando helpers en cookieutil.go

No emite JWT directamente: su responsabilidad es establecer una cookie para que
otros endpoints (principalmente /oauth2/* en modo browser) puedan continuar el flujo.

Dependencias reales (lo que toca)
---------------------------------
- c.Store (obligatorio):
		- GetClientByClientID (cuando viene client_id)
		- GetUserByEmail + CheckPassword (ya sea en store global o tenant repo)
- c.TenantSQLManager (opcional): abre un repo por tenant con helpers.OpenTenantRepo
- cpctx.Provider (opcional): lookup en control-plane FS para resolver "tenant slug" por client_id
- c.Cache (requerido en práctica): persiste payload de sesión bajo "sid:<sha256(rawSID)>"
- cookieutil.go: BuildSessionCookie(...) para construir cookie (SameSite/Secure/Domain/TTL)

⚠️ Nota: el handler valida c.Store != nil, pero NO valida c.Cache != nil.
Si el container se inicializa sin cache, esto puede panic.

Ruta soportada (contrato efectivo)
----------------------------------
- POST /v1/session/login
		Request JSON:
			{
				"tenant_id": "..."  (slug o UUID; opcional si viene client_id)
				"client_id": "..."  (opcional si viene tenant_id)
				"email": "...",
				"password": "..."
			}
		Response:
			- 204 No Content
			- Set-Cookie: <cookieName>=<rawSID>; ...
			- Cache-Control: no-store

Flujo interno (por etapas)
--------------------------
1) Validación básica
	 - solo POST
	 - tenant_id o client_id requerido
	 - email se normaliza (trim + lower)

2) Resolución de tenant (lo más “frágil” del archivo)
	 A) Si viene client_id:
			- Intenta SQL: c.Store.GetClientByClientID
			- Si lo encuentra, intenta “forzar slug FS”:
					* cpctx.Provider.ListTenants
					* loop por tenants + Provider.GetClient(tenantSlug, clientID)
					* si matchea: tenantSlug = t.Slug y tenantID = cl.TenantID (UUID)
			- Si no matchea en FS: helpers.ResolveTenantSlugAndID(ctx, cl.TenantID)
			- Si SQL falla, fallback FS-only:
					* itera tenants en FS buscando el client
					* tenantSlug=t.Slug y tenantID=t.ID (o fallback = slug)

	 B) Si viene tenant_id:
			- helpers.ResolveTenantSlugAndID(ctx, req.TenantID)

	 Resultado: tenantSlug se usa como "tenant final" de la sesión para ser compatible
	 con oauth_authorize (que hace fallback FS y espera slug en algunos paths).

3) Selección de store (global vs tenant DB)
	 - Define una interfaz mínima userAuthStore (GetUserByEmail + CheckPassword)
	 - Intenta abrir tenant repo via TenantSQLManager + helpers.OpenTenantRepo(tenantSlug)
	 - Si no se puede, usa store global si satisface la interfaz
	 - Esto deja un comportamiento “best effort”: puede autenticarse contra DB global
		 si la tenant DB no está disponible.

4) Auth con retries por key mismatch
	 - lookupID inicialmente tenantID (UUID) o tenantSlug
	 - llama GetUserByEmail(lookupID)
	 - si falla y tenantSlug != lookupID, reintenta con tenantSlug

5) Crear sesión en cache + cookie
	 - rawSID: token opaco 32 bytes
	 - payload JSON (SessionPayload): {user_id, tenant_id, expires}
	 - cache key: "sid:"+SHA256Base64URL(rawSID)
	 - cookie: BuildSessionCookie(cookieName, rawSID, domain, sameSite, secure, ttl)
	 - responde 204

Seguridad / invariantes
-----------------------
- CSRF:
	Este endpoint está pensado para browser cookies. El enforcement de CSRF se aplica
	(opcionalmente) desde middleware a POST /v1/session/login (ver csrf.go y tests e2e).

- Cache-Control:
	Bien: fuerza no-store en la respuesta.

- Logging:
	Hace log DEBUG con email/tenant y decisiones de routing. Esto es útil, pero puede
	filtrar metadata sensible (emails) si se habilita en producción.

- Tenant resolution por loop:
	El lookup FS de client_id hace ListTenants + GetClient por tenant: O(#tenants).
	V2 debería tener un índice (client_id -> tenant) o una API directa.

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) Resolver tenant mezclando SQL y FS
	 El flujo es complejo y tiene “override” FS. Esto existe para compatibilidad con oauth_authorize,
	 pero aumenta riesgo de inconsistencias (UUID vs slug vs fallback).

2) Fallback “best effort” a store global
	 Si la tenant DB no abre, puede autenticar contra el store global si implementa la interfaz.
	 Eso puede ser deseable (degraded mode) o peligroso (cross-tenant) dependiendo del store.

3) Respuesta con Content-Type JSON pero 204
	 Setea "application/json" aunque no devuelve body. No rompe, pero es inconsistente.

4) Cache requerido pero no validado
	 Si c.Cache es nil, c.Cache.Set paniquea.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
Objetivo: hacer que /session/login sea una capa fina y determinística:

FASE 1 — Resolver tenant de forma única
	- Introducir TenantResolver único (client_id -> tenantSlug+tenantUUID) sin loops.
	- Hacer que oauth_authorize deje de depender de slug “mágico” en sesión.

FASE 2 — Service de sesión
	- services/session_service.go:
			LoginWithPassword(ctx, tenant, email, password) -> sessionID + expires
			StoreSession(ctx, sessionID, payload, ttl)
			BuildCookie(sessionID, opts)

FASE 3 — Contratos y seguridad
	- Enforzar CSRF en este endpoint siempre en modo cookie
	- Reducir logs o moverlos a trazas con request_id y sin PII
	- Validar (o requerir) c.Cache



session_logout_util.go — helper local de hashing para sesiones (sid)

Qué es este archivo (la posta)
------------------------------
Este archivo existe únicamente para definir tokensSHA256(s) y poder reutilizarlo en
session_logout.go al construir la key de cache server-side:
	key := "sid:" + tokensSHA256(rawSID)

Funcionalmente es un duplicado del helper “canónico” que existe en
internal/security/token (tokens.SHA256Base64URL), con la misma idea:
	sha256(bytes) -> base64url sin padding.

Por qué existe / deuda técnica
------------------------------
- Históricamente, session_logout se implementó separado y no reutilizó tokens.*
- En session_login.go se usa tokens.SHA256Base64URL; en logout se usa tokensSHA256.
	Hoy parecen equivalentes (sha256 + RawURLEncoding), pero mantener dos helpers es frágil.

Riesgos
-------
1) Divergencia silenciosa
	 Si tokens.SHA256Base64URL cambia (o tokensSHA256 cambia) se rompe el contrato:
	 logout no borraría la sesión del cache aunque sí borre la cookie.

Mejora V2 (simple)
------------------
- Eliminar este archivo y usar tokens.SHA256Base64URL en session_logout.go,
	o mover a un package session y usar una única función para generar claves de cache.



session_logout.go — Session Cookie Logout (borra sid en cache + cookie) + redirect return_to allowlist

Qué es este archivo (la posta)
------------------------------
Este archivo define NewSessionLogoutHandler(...) que implementa el “logout” del modo cookie/session:
	- Si existe cookie de sesión (sid):
			* borra la sesión del cache server-side
			* setea una cookie de borrado (expirada) para limpiar el browser
	- Opcionalmente redirige a return_to si el host está en allowlist

No revoca refresh/access tokens (eso es otro subsistema); acá solo limpia la sesión tipo sid.

Dependencias reales
-------------------
- c.Cache: Delete("sid:<hash>")
- cookieutil.go: BuildDeletionCookie(...) para expirar cookie
- c.RedirectHostAllowlist: mapa host->bool para permitir redirects
- session_logout_util.go: tokensSHA256 para hashear el raw cookie value

Ruta soportada (contrato efectivo)
----------------------------------
- POST /v1/session/logout
		Cookies:
			- lee cookieName (configurable)

		Query:
			- return_to (opcional): si es URL absoluta y su host está en allowlist, 303 redirect

		Response:
			- 204 No Content (por defecto)
			- o 303 See Other a return_to (si validación OK)

Flujo interno
-------------
1) Validación de método
	 - solo POST

2) Si existe cookie de sesión
	 - r.Cookie(cookieName)
	 - si tiene valor no vacío:
			 a) server-side: key = "sid:" + tokensSHA256(cookieValue)
					c.Cache.Delete(key)
			 b) client-side: setea cookie de borrado (BuildDeletionCookie)
	 Nota: si NO hay cookie, no setea deletion cookie (logout no es 100% idempotente a nivel browser).

3) Redirect opcional return_to
	 - Requiere que return_to sea URL absoluta (scheme y host no vacíos)
	 - El host se baja a lowercase y se compara con c.RedirectHostAllowlist
	 - Si pasa: http.Redirect(..., StatusSeeOther)

4) Si no hay redirect: 204

Seguridad / invariantes
-----------------------
- Borrado server-side:
	Depende de que el hashing usado aquí coincida con el usado en session_login.go.
	Login usa tokens.SHA256Base64URL; logout usa tokensSHA256 (sha256 + RawURLEncoding).
	Hoy parecen equivalentes, pero es deuda técnica.

- Redirect allowlist:
	Bien: valida que sea absoluta y aplica allowlist.
	Pero usa u.Host tal cual (puede incluir ":port").
	Si el allowlist guarda hosts sin puerto, el match puede fallar.
	V2: usar u.Hostname() + puerto opcional, o normalizar ambos.

- CSRF:
	Logout también es endpoint cookie-based; idealmente debería estar cubierto por la misma política CSRF
	que el resto de endpoints sensibles (depende de middleware global).

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) Duplicación de hashing
	 tokensSHA256 duplicado vs tokens.SHA256Base64URL.

2) Logout no siempre limpia cookie
	 Si el cliente no manda cookie (o el nombre cambió), no se setea deletion cookie.
	 Puede ser preferible siempre setear cookie expirada para hacer logout idempotente.

3) Normalización de return_to
	 Comparar con u.Host puede incluir puerto; conviene normalizar.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
FASE 1 — Unificar contrato sid
	- session.CacheKey(sessionID) en un package único
	- Eliminar session_logout_util.go y usar tokens.SHA256Base64URL

FASE 2 — Logout idempotente
	- Siempre setear deletion cookie (aunque no exista cookie entrante)
	- Borrar server-side si cookie presente

FASE 3 — Redirect seguro y predecible
	- Normalizar host con u.Hostname()
	- Considerar allowlist por (scheme, host, port)



social_dynamic.go — comentario/diagnóstico (solo este archivo)

Qué carajo hace este handler
----------------------------
`DynamicSocialHandler` es un “router” para social login multi-tenant, donde el tenant se decide *en runtime*
y las credenciales (client_id / client_secret del proveedor) se cargan desde el Control Plane (cpctx.Provider).

Ruta esperada:
- /v1/auth/social/{provider}/{action}
  - provider: hoy solo "google"
  - action: "start" o "callback"

Flujo alto nivel:
1) Parsear provider/action desde la URL.
2) Resolver tenantSlug:
   - start: viene por query param `tenant` (fallback `tenant_id`)
   - callback: viene adentro de `state` (JWT firmado) => parseStateGeneric => claims["tenant_slug"]
3) Con tenantSlug, pedir el tenant al control plane: cpctx.Provider.GetTenantBySlug(...)
4) Leer settings.SocialProviders y verificar si el proveedor está habilitado.
5) Decrypt secret (secretbox) o fallback a plain text si parece dev.
6) Crear handler específico (googleHandler) con OIDC + pool del tenant + adapters.
7) Delegar:
   - start: generar state “compatible” con googleHandler (incluye tid/cid/redir/nonce + tenant_slug extra)
          y redirigir a Google.
   - callback: delegar a gh.callback(w,r) (que hace exchange, verify id_token, provisioning, MFA, tokens, etc.)

Puntos buenos (bien ahí)
------------------------
- Multi-tenant de verdad: credenciales por tenant desde Control Plane.
- No mezcla config global con tenant config.
- Usa pool específico del tenant cuando TenantSQLManager está disponible.
- El state es JWT EdDSA firmado por tu Issuer, no un random string (bien para integridad).
- Fallback dev para secretos no cifrados (útil, pero ojo en prod).

Riesgos / bugs / cosas flojas
-----------------------------

1) Construcción de redirectURL (scheme) es medio “fantasiosa”
   ----------------------------------------------------------
   `redirectURL := fmt.Sprintf("%s://%s/...", r.URL.Scheme, r.Host)`
   En Go server-side, r.URL.Scheme suele venir vacío porque el server no sabe si el cliente vino por http/https
   (a menos que tu reverse proxy lo setee y vos lo traduzcas).
   Luego hacés fallback:
     - https por defecto
     - http si host arranca con localhost/127.0.0.1
   Problema real: detrás de un proxy (nginx/traefik/alb) puede ser HTTPS afuera, HTTP adentro.
   Si no usás X-Forwarded-Proto, te podés clavar con redirect_uri incorrecto y Google te lo rechaza.

   Qué haría:
   - Si hay `X-Forwarded-Proto`, usar ese.
   - Si tenés en config/tenant settings un “public_base_url”, usar eso siempre.

2) Callback depende de `tenant_slug` en state (y si no, muere)
   -----------------------------------------------------------
   En callback:
   - parseStateGeneric(state) y saca claims["tenant_slug"].
   Si no está, devolvés error 1695.
   Esto es correcto para TU diseño, pero dejás comentado que googleHandler “espera tid UUID”.
   O sea: estás metiendo un claim extra para poder resolver el tenant antes de instanciar OIDC.

   Bien: necesitás tenantSlug para saber qué clientSecret usar antes de verificar id_token.
   Pero: si mañana alguien te pega a /callback de un flow viejo que no incluía tenant_slug, se rompe.

   Mitigación:
   - Guardar también `tenant_slug` en un cache “state jti -> tenant_slug” en start (one-shot / ttl),
     y en callback si no está el claim, usar jti o hash del state para buscarlo.
   - O permitir fallback a tid:
       - si cpctx.Provider soporta GetTenantByID, usarlo
       - sino, indexar tenants por ID en memoria al boot (map[uuid]slug)

3) Aud hardcodeado “google-state”
   -------------------------------
   generateState usa aud "google-state" y parseStateGeneric lo valida.
   Está ok, pero ojo con reutilización si en el futuro agregás GitHub/Microsoft/etc:
   - si todos usan "google-state", no es grave, pero semánticamente raro.
   Mejor: aud = "social-state" y además claim "p" = provider, o aud = "google-state" pero entonces
   `DynamicSocialHandler` debería validar provider también dentro del state.

4) Confusión de nombres: tenantSlug vs tenant_id
   ----------------------------------------------
   En start aceptás `tenant` y fallback a `tenant_id`, pero ambos los tratás como slug.
   Eso te puede generar quilombo porque “tenant_id” suele ser UUID.
   Si alguien te manda UUID en tenant_id, vos lo pasás a GetTenantBySlug y da not_found.

   Arreglo práctico:
   - renombrar query param a `tenant` o `tenant_slug` y chau.
   - si querés soportar UUID, detectá si parsea como uuid, y resolvé por ID.

5) `validator: redirectValidatorAdapter{repo: h.c.Store}` podría validar contra DB incorrecta
   -----------------------------------------------------------------------------------------
   Vos en dynamic instanciás gh con pool del tenant, pero el validator queda apuntando al repo global (`h.c.Store`).
   Si el redirect validator depende de “clientes” en DB global, ok.
   Pero si tus clients viven en control plane FS o DB por tenant, se te desincroniza.

   Según social_google.go, en issueSocialTokens usás ResolveClientFSByTenantID (FS), no DB.
   Entonces lo más consistente sería que el validator también valide contra FS/control plane, no contra `h.c.Store`.

6) Decrypt secret: fallback a plaintext por “no contiene |”
   --------------------------------------------------------
   Está bien para dev, pero en prod es un footgun:
   - si por algún motivo un secreto cifrado cambia formato y no trae "|", lo aceptás igual como plaintext
     y lo mandás a Google: falla y encima te hace perder tiempo.
   Mejor:
   - un flag `AllowPlainSecrets` por env (solo dev)
   - si prod: si no se puede decrypt => error y listo.

7) Seguridad de state: issuer y keyfunc
   ------------------------------------
   parseStateGeneric valida:
   - firma EdDSA con Keyfunc()
   - iss == h.c.Issuer.Iss
   - aud == google-state
   - exp (con un “grace” raro de -30s)

   Ojo:
   - El grace que hacés es “exp < now-30s => expired”; eso da 30s extra después de exp.
     Si querés tolerancia de reloj, suele ser al revés (aceptar tokens apenas antes de nbf/iat),
     pero exp normalmente se respeta estricto o con pequeña tolerancia (5-30s) está ok.
   - No validás `nbf` ni `iat` explícitamente (jwt lib puede no hacerlo si es MapClaims sin options).
     Si te importa, validalo.

8) `pgxpool.Pool` y store del tenant
   ----------------------------------
   Bien que pedís pool del tenantStore.Pool().
   Pero estás asumiendo que GetPG devuelve un tipo con Pool() (comentario dice *pg.Store).
   Si mañana cambiás store, esto rompe.

   Mejor:
   - Definir interfaz chica:
     type PoolProvider interface { Pool() *pgxpool.Pool }
     y listo (ya lo hacés para fallback, pero no para tenantStore).

Cómo lo separaría (sin reescribir todo el sistema)
--------------------------------------------------
Este archivo hoy mezcla 3 responsabilidades:
1) Router de path (/provider/action)
2) Tenant resolution (start por query, callback por state)
3) Factory de provider handler (googleHandler) + wiring (pool/oidc/adapters)

Yo lo partiría en 3 piezas (sin cambiar behavior):
- social_router.go:
  - parse provider/action
  - dispatch a SocialService
- social_tenant_resolver.go:
  - ResolveTenantSlug(r, action) -> slug
  - (start: query param; callback: state; fallback por tid si se banca)
- social_provider_factory.go:
  - BuildGoogleHandler(tenantSlug, tenantSettings, requestContext) -> handler

Eso te deja el ServeHTTP limpito y testeable.

Notas puntuales sobre números de error
--------------------------------------
Los códigos 1690..1708 están bien “secuenciados”, pero ojo que:
- 1693 missing_state en callback no considera tu debug mode (comentaste “assume standard flow first”).
  Si debug mode de verdad admite callback sin state, acá nunca llega.

Qué ajustaría YA (quick wins)
-----------------------------
- redirectURL: usar X-Forwarded-Proto si existe (y quizá X-Forwarded-Host).
- query param: matar `tenant_id` o soportar UUID de verdad.
- validator: que valide redirect_uri contra FS (control plane) como hace el resto del flow social.
- secrets: plaintext fallback solo con env flag (dev).

En resumen
----------
`social_dynamic.go` está bien encaminado como “puente” entre:
- Control plane (tenant settings)
- Proveedor (Google)
- DB por tenant (pool)
pero ahora mismo tiene dos bombas: el scheme/redirect y el validator apuntando al repo global.
Si arreglás eso, te queda bastante sólido.



social_exchange.go — comentario/diagnóstico

Qué hace este endpoint
----------------------
Este handler implementa la “segunda pata” del flow social cuando usás **login_code**:
- En el callback social (ej: google), vos emitís tokens (access/refresh) pero en vez de devolvérselos directo
  al frontend, generás un `code` corto y lo guardás en cache con key:
    "social:code:<code>"
  y redirigís al `redirect_uri` del cliente con `?code=...`.

- Después el frontend (o el backend del cliente) pega a:
    POST /v1/auth/social/exchange   (asumo esta ruta)
  con JSON:
    { "code": "...", "client_id": "...", "tenant_id": "..."? }
  y este handler devuelve el JSON final `AuthLoginResponse` (tokens).

O sea: es un “token exchanger” one-shot basado en cache.

Flujo exacto del código
-----------------------
1) Solo POST.
2) Lee JSON a `SocialExchangeRequest`.
3) Valida: code y client_id obligatorios.
4) Busca en cache `social:code:<code>`.
   - Si no existe => 404 code_not_found.
5) Unmarshal payload en:
   { client_id, tenant_id, response(AuthLoginResponse) }.
6) Valida que `req.client_id` == `stored.client_id` (case-insensitive, trim).
7) Si vino `req.tenant_id`, valida que coincida con `stored.tenant_id`.
8) Borra del cache (one-shot) **recién después de validar**.
9) Devuelve 200 con `stored.Response`.

Cosas que están bien
--------------------
- One-shot correcto: borrás el code solo si pasa validación.
- No-store/no-cache headers (clave: no querés tokens en caches).
- `client_id` binding: evita que alguien robe un code y lo canjee desde otro cliente.
- Logs debug opcionales para E2E (bien, y están controlados por env).

Puntos flojos / riesgos reales
------------------------------

1) Falta límite de tamaño del body
   -------------------------------
   Acá no ponés `http.MaxBytesReader`. En otros endpoints sí.
   Solución: antes de `ReadJSON`, meté:
     r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
   con 32KB alcanza.

2) El error 404 “code_not_found” filtra existencia de codes (leve)
   ---------------------------------------------------------------
   RFC-style a veces prefiere responder 400 genérico para no dar señales,
   pero acá es relativamente inocuo porque el code es high entropy (randB64(32) en googleHandler).
   Igual, si querés “menos oracle”, devolvé 400 “invalid_grant” siempre.

3) No validás expiración acá (depende 100% del cache TTL)
   ------------------------------------------------------
   En tu diseño, el TTL del cache define expiración, perfecto.
   Pero si el cache backend tiene bugs o TTL distinto, no hay segunda validación.

   Opcional: guardar en payload un `exp` y chequearlo acá también.

4) TenantID opcional => puede permitir canje cross-tenant si se reusa code (raro pero…)
   -----------------------------------------------------------------------------------
   Vos validás tenant_id *solo si lo mandan*.
   Si el cliente no lo manda, entonces solo validás client_id.
   Si un mismo client_id existiera en más de un tenant (depende tu modelo),
   podrías terminar aceptando canje “cruzado”.

   Si en tu sistema `client_id` es global-unique, entonces no hay drama.
   Si no lo es (por tenant), entonces: tenant_id debería ser obligatorio.
   Alternativa más limpia: en vez de pedir tenant_id al cliente, sacalo del payload y listo:
   - devolvés siempre lo que está en cache, pero igual *log* si req.TenantID no coincide.
   (o directamente hacé requerido `tenant_id`).

5) Re-jugar code si el request se cae en el medio
   ----------------------------------------------
   Hoy borrás del cache antes de escribir JSON (ok) pero si el cliente se desconecta justo después,
   perdió la chance de reintentar.
   Esto es un tradeoff:
   - Seguridad > UX: como está, más seguro.
   - UX > seguridad: podrías “consumir” con un flag `used=true` y un TTL cortito post-consumo (5-10s)
     para permitir reintentos idempotentes desde el mismo client_id.
   Yo lo dejaría como está salvo que te esté jodiendo en producción.

6) Comparación case-insensitive de client_id
   -----------------------------------------
   `EqualFold` para client_id… depende.
   Si `client_id` se trata como identifier case-sensitive (muchos sistemas lo tratan así),
   podrías estar aflojando una regla.
   Igual no es una vulnerabilidad seria, pero por prolijidad:
   - o normalizás client_id a lower-case siempre en todo el sistema
   - o comparás exacto (strings.TrimSpace y listo)

7) No autenticás al cliente
   -------------------------
   Este endpoint no exige client secret ni mTLS ni nada.
   El “secreto” acá es el `code`.
   De nuevo: si el code es fuerte y de vida corta, está ok.
   Si querés subir la vara:
   - permitir Basic Auth del cliente (confidential) en exchange
   - o exigir `PKCE-like` extra (un verifier) almacenado en payload

Qué separaría / cómo lo ubicaría en capas
-----------------------------------------
Este archivo hoy está bien “handler puro”, pero si lo querés más limpio:

- handlers/social_exchange.go (HTTP):
  - parse JSON
  - valida input
  - llama a SocialCodeService.ExchangeCode(...)

- internal/auth/social/service.go:
  type SocialCodeService interface {
      ExchangeCode(ctx, code, clientID, tenantID string) (AuthLoginResponse, error)
  }
  - encapsula cache get/unmarshal/validaciones/borrado

- internal/auth/social/store_cache.go:
  - wrapper sobre c.Cache para:
    GetSocialCodePayload(code) / DeleteSocialCode(code)

Eso te da:
- unit tests fáciles del service sin HTTP
- el handler queda chiquito y consistente con tus otros handlers.

Quick wins (cambios mínimos que te recomiendo YA)
-------------------------------------------------
1) Limitar body:
   r.Body = http.MaxBytesReader(w, r.Body, 32<<10)

2) Si tu `client_id` NO es global-unique, hacé `tenant_id` requerido y listo:
   if req.TenantID == "" => 400.

3) (Opcional) Cambiar 404 por 400 "invalid_grant" si querés menos señalización.

En resumen
----------
`social_exchange.go` está correcto para el patrón “login_code one-shot”.
Lo más urgente es meter MaxBytesReader y decidir la política de `tenant_id` (opcional vs requerido)
según si tu `client_id` es global o por-tenant. Si es por-tenant, hoy estás medio jugado.



social_google.go — comentario/diagnóstico (bien completo, con “caminos”, capas y dónde partirlo)

Qué es este archivo
-------------------
Este handler implementa **Login Social con Google** para un tenant (modo “tenant DB”) con dos endpoints:

1) START
   GET /v1/auth/social/google/start?tenant_id=...&client_id=...&redirect_uri=...
   - Arma `state` (JWT EdDSA) + `nonce`
   - Valida redirect_uri (del cliente) contra el client (tu validator)
   - Redirige a Google con AuthURL(state, nonce)

2) CALLBACK
   GET /v1/auth/social/google/callback?state=...&code=...
   - Valida `state` (firma/iss/aud/exp)
   - Intercambia code con Google, verifica ID Token contra nonce
   - Provisiona/“linkea” user e identity en DB del tenant (h.pool)
   - Aplica “hook” MFA (si corresponde)
   - Emite tokens propios (access JWT + refresh opaco) y:
     - o devuelve JSON (AuthLoginResponse)
     - o si venía redirect_uri del cliente en el state, crea login_code, guarda en cache y redirige al cliente

Así que maneja 3 problemas mezclados: (a) OIDC con Google, (b) provisioning SQL, (c) emisión de tokens / login_code.

Mapa de caminos (flow)
----------------------

A) /start (GET)
   1. Rate limit (si MultiLimiter)
   2. Lee query: tenant_id, client_id, redirect_uri (opcional)
   3. Valida tenant_id es UUID y client_id no vacío
   4. Si redirect_uri viene:
        - normaliza (sin query/fragment)
        - valida con redirectValidatorAdapter contra el client
   5. (Opcional) allowlist tenant/client (cfg.Providers.Google.Allowed*)
   6. Genera nonce + firma state (JWT) con:
        iss = issuer.Iss
        aud = "google-state"
        exp = now + 5m
        tid, cid, redir, nonce
   7. oidc.AuthURL(ctx, state, nonce)
   8. Redirect 302 a Google

B) /callback (GET) camino normal
   1. Rate limit
   2. Si query error=..., devuelve idp_error
   3. Valida que existan state y code
   4. parseState(state):
        - jwt parse EdDSA
        - tk.Valid
        - iss coincide
        - aud coincide
        - exp no vencido (con -30s skew)
   5. Extrae tid,cid,redir,nonce
   6. allowlist (isAllowed)
   7. tok := oidc.ExchangeCode(code)
   8. idc := oidc.VerifyIDToken(tok.IDToken, nonce)
      - exige email
   9. uid := ensureUserAndIdentity(tid, idc) usando h.pool
  10. MFA check:
        - si store implementa GetMFATOTP + IsTrustedDevice
        - si tiene MFA confirmada y no trusted device:
             guarda challenge en cache "mfa:token:<mid>"
             responde JSON {mfa_required:true, mfa_token:..., amr:["google"]}
             return
  11. issueSocialTokens(..., amr:["google"]) -> emite access/refresh
  12. log info final

C) /callback debug (solo si SOCIAL_DEBUG_HEADERS=true)
   - Permite simular sin Google real:
     - code=debug-... o headers X-Debug-Google-Email/Sub/Nonce
   OJO: esto es una puerta *muy* peligrosa si se te filtra a prod.

Qué está bien (posta)
----------------------
- `state` firmado con EdDSA: buen anti-CSRF + integridad.
- `nonce` y verificación de ID token con nonce: bien OIDC.
- Rate limit por IP (start/callback): suma un montón contra abuso.
- “login_code flow” para apps SPA/popups: práctico y prolijo (y ya tenés exchange/result).
- Provisioning en tenant DB con pool: consistente con multi-tenant por DB.
- MFA hook “antes de emitir tokens”: correcto (no entregás tokens si falta 2FA).

Red flags / bugs / deuda técnica (lo importante)
------------------------------------------------

1) Emisión de tokens usa h.c.Store para user/roles + client scopes (BUG multi-tenant)
   --------------------------------------------------------------------------------
   En issueSocialTokens hacés:
     h.c.Store.GetClientByClientID(...)
     h.c.Store.GetUserByID(...)
     y roles/perms desde h.c.Store.(rbacReader)

   Pero el provisioning y refresh_token insert lo hacés con h.pool (tenant DB).
   Si `h.c.Store` es global o de otro tenant, te trae:
   - scopes de otro lado
   - metadata del usuario no encontrada o del tenant equivocado
   - roles/perms incorrectos

   FIX recomendado:
   - Para social, TODO lo “tenant data” debe salir de tenant store/pool.
   - O bien, pasale a googleHandler un `repo core.Repository` ya resuelto por tenant,
     y usalo para GetUser/GetRoles/GetClient.
   - Si no tenés repo por tenant, al menos hacé queries SQL por h.pool para:
       - scopes del client (si están en FS, tomalos del FS, no de DB)
       - metadata/roles/perms del user (tenant DB)

2) Refresh token insert: `NOW() + $4::interval` con string “72h0m0s” (posible crash)
   ------------------------------------------------------------------------------
   Estás pasando `h.issuerTok.refreshTTL.String()` como intervalo.
   En Postgres, interval acepta cosas tipo '72 hours' o '5 minutes'.
   `"72h0m0s"` NO siempre parsea (de hecho, suele fallar).

   FIX:
   - Pasar segundos y usar `NOW() + ($4 * interval '1 second')`
     y mandar int64(refreshTTL.Seconds()).
   - O formatear `'72 hours'` vos.

3) El token access lo emitís con IssueAccess(uid, cid, ...) sin issuer por tenant
   -----------------------------------------------------------------------------
   En tu OAuth token endpoint ya resolviste issuer efectivo por tenant (issuer mode path/custom).
   Acá no: usás `h.c.Issuer.Iss` fijo, y `IssueAccess` (no IssueAccessForTenant).
   Eso te rompe consistencia con:
   - JWKS per-tenant
   - iss por tenant (path mode)
   - introspection que valida issuer/slug

   FIX:
   - igual que en oauth_token.go: resolver `effIss` y firmar “for tenant”
     con `IssueAccessForTenant(tenantSlug, effIss, ...)` o equivalente.
   - y el SYS namespace también debería usar effIss.

4) `helpers.ResolveClientFSByTenantID(tid.String(), cid)` dice “ONLY FS”
   --------------------------------------------------------------------
   Perfecto, pero entonces dejá de usar c.Store para client scopes.
   Tenés fsCl ahí: usalo para scopes (fsCl.Scopes).
   Hoy lo resolvés y después no lo usás.

5) ensureUserAndIdentity ignora “tid” (ok) pero entonces email debe ser único por tenant DB
   --------------------------------------------------------------------------------------
   Como cada tenant es su DB, está bien que `app_user` no tenga tenant_id.
   Pero ojo: si a futuro cambiás a shared DB, esto explota.

6) DEBUG SHORTCUT es una bomba si se habilita en prod
   --------------------------------------------------
   `SOCIAL_DEBUG_HEADERS=true` permite loguear con headers sin pasar por Google.
   Eso tiene que estar:
   - hardcodeado a “solo dev”
   - o protegido por allowlist de IP + secret adicional
   - o directamente eliminado y reemplazado por tests de integración.

7) “state_expired” usa skew de -30s raro
   -------------------------------------
   `Before(time.Now().Add(-30s))` => o sea, tolerás 30s *después* de exp.
   Está bien como clock skew, pero dejalo explícito como `skew := 30s` para claridad.

8) Falta MaxBytesReader en callbacks (no grave porque es GET, pero start/callback no consumen body)
   ------------------------------------------------------------------------------------------------
   No aplica mucho acá. Donde sí: exchange POST.

Cómo lo separaría (capas y archivos)
------------------------------------

Hoy `social_google.go` tiene 4 responsabilidades. Te lo partiría así:

1) handlers/social/google_handler.go (HTTP puro)
   - start(w,r)
   - callback(w,r)
   - parse query, headers, rate limit, http errors, redirects
   - NO debería hablar SQL directo salvo a través de un service

2) internal/auth/social/google/state.go
   - SignState(issuer, tid, cid, redir, nonce, ttl) (y parse/validate)
   - Tipos claims tipados (en vez de MapClaims) para evitar casts

3) internal/auth/social/google/oidc.go
   - wrapper: ExchangeCode + VerifyIDToken
   - aislás dependencias de google.OIDC

4) internal/auth/social/provisioning/service.go
   - EnsureUserAndIdentity(ctx, db, provider, idClaims) (usa h.pool)
   - maneja upsert con transacción (ideal):
       BEGIN
         select user
         insert/update
         ensure identity
       COMMIT
     (ahora está bien, pero sin tx puede haber carreras raras)

5) internal/auth/token/service.go
   - IssueTokensForSocial(ctx, tenant, client, user, amr, scopes, clientRedirect)
   - decide:
       - MFA required?
       - emitir access/refresh
       - login_code o JSON

6) internal/auth/social/logincode/store.go
   - SaveLoginCode(code -> payload, ttl)
   - ConsumeLoginCode(code)

Objetivo: el handler orquesta y el service decide lógica.

Quick wins para que quede sólido ya
-----------------------------------

- En issueSocialTokens:
  1) usar scopes desde FS:
       scopes := fsCl.Scopes (o default)
  2) user/roles/perms: leer desde tenant DB, no c.Store global.
     Si no querés implementar repo por tenant ahora:
       - por lo menos no intentes roles/perms (o dejalo “best-effort” pero con interfaz sobre tenant store)
  3) emitir access con issuer efectivo por tenant (como oauth_token.go)

- Refresh insert:
  cambiar a segundos:
    const q = `... expires_at = NOW() + ($4 * interval '1 second') ...`
    h.pool.QueryRow(ctx, q, cid, uid, hash, int64(refreshTTL.Seconds()))

- DEBUG:
  meter un guardia duro:
    if cfg.Env != "dev" { ignorar SOCIAL_DEBUG_HEADERS aunque esté seteado }

Cierre
------
El flow está copado y bastante completo (state/nonce/rate-limit/mfa/login_code), pero hoy tenés
inconsistencias multi-tenant fuertes: *emitís tokens mirando c.Store*, pero persistís usuario/refresh en tenant DB.
Si arreglás eso + el interval del refreshTTL + issuer efectivo por tenant, te queda una base muy sólida.




social_result.go — review (legacy / “¿hace falta?”) + riesgos + cómo lo dejaría prolijo

Qué hace este handler
---------------------
Endpoint GET que, dado un `code`, busca en cache `social:code:<code>` (payload JSON con tokens) y lo devuelve:
- como JSON “crudo” (default), o
- como HTML lindo tipo “pantalla de éxito” (si el cliente parece browser o pide text/html)

Además tiene modo `peek=1` para no consumir el code (debug).

Dónde encaja en el flow
-----------------------
Tu flow social moderno ya tiene:
- callback de Google que puede:
  - devolver JSON directo (si no hay redirect_uri cliente), o
  - redirigir al cliente con ?code=<loginCode> (login_code)
- social_exchange.go (POST) que “canjea” ese code por tokens

Entonces social_result.go es un “viewer” legacy/útil para:
- pruebas manuales (abrir el link y ver el JSON sin armar frontend)
- demos rápidas
- debug (peek)

No es estrictamente necesario para el core, pero puede ser útil como herramienta.

Caminos / comportamiento exacto
-------------------------------
1) GET /v1/auth/social/result?code=... [&peek=1]
   - Si falta code -> 400
   - Busca payload en cache -> si no existe -> 404
   - Si peek != 1 -> borra del cache (one-shot)
   - Decide formato:
       forceJSON = Accept incluye "application/json"
       wantsHTML = !forceJSON && (Accept incluye text/html o Accept vacío y UA “mozilla”)
   - HTML:
       - arma CSP con nonce (bien)
       - inyecta payload base64 (script type application/octet-stream) + JS lo decodifica y muestra
       - botones para copiar JSON y code
       - intenta cerrar popup / volver
       - `postMessage` al opener con payload (PERO ojo abajo)
   - JSON:
       - write(payload) tal cual

Lo bueno
--------
✅ One-shot del code (si no peek) está en línea con login_code.
✅ `Cache-Control: no-store` y `Pragma: no-cache` bien.
✅ CSP con nonce para inline style/script: bien.
✅ No hace “render” del JSON directo en HTML, lo mete base64 (ok).

Riesgos / problemas a corregir sí o sí
--------------------------------------

1) postMessage con target "*"
   -------------------------
   En HTML:
     window.opener.postMessage({ type: 'hellojohn:login_result', payload: data }, '*');

   Eso es peligroso: **le estás mandando access/refresh tokens a cualquier origin** que sea el opener.
   Si un sitio malicioso logra abrir esta ventana y te mete un code (o intercepta), puede recibir tokens.

   FIX:
   - Ideal: NO uses postMessage en este endpoint (si es legacy).
   - O, si lo querés, mandá a un origin explícito:
       const allowed = new URL(returnTo).origin;
       window.opener.postMessage(..., allowed);
     y calculá allowed con algo confiable (mejor: que el `code` almacenado en cache incluya `return_origin`).

2) “return_to” permite navegación solo same-origin (bien), pero no está atado al code
   --------------------------------------------------------------------------------
   Vos validás `return_to` contra window.location.origin -> ok, no open redirect.
   Peeero: el `code` no está ligado a un “return_to” esperado. Es más un UX.
   Si querés hardening:
   - guardá en el cache junto al payload: `return_to` o `client_redirect`
   - y no aceptes uno que venga del query si no matchea.

3) peek=1 es re útil, pero en prod es un agujero de replay
   -------------------------------------------------------
   Con peek=1 cualquiera que tenga el code puede ver tokens infinitas veces hasta que expire el TTL.
   Si esto existe en prod:
   - sacalo, o
   - protegelo con auth (por ejemplo solo admin / internal), o
   - aceptalo solo con env flag `SOCIAL_DEBUG_RESULT=true`.

4) HTML template gigante incrustado en código (mantenibilidad)
   -----------------------------------------------------------
   Es un ladrillo dentro de Go. Funciona, pero:
   - cuesta de testear
   - cuesta de mantener
   - y ensucia handlers

   Mejor:
   - mover tpl a `internal/http/v1/templates/social_result.html` embebido con `//go:embed`
   - y que este handler solo haga `tpl.Execute`.

5) Decodificás payload a `AuthLoginResponse` pero no lo usás
   --------------------------------------------------------
   `var resp AuthLoginResponse; _ = json.Unmarshal(payload, &resp)`
   no impacta nada. Si era para “vista” podrías:
   - mostrar “expires_in”, “token_type”, etc. en HTML
   - si no, borrarlo.

6) Content-Type del payload-b64
   ----------------------------
   Está bien como `application/octet-stream`, pero ojo: igual lo lees con JS.
   No hay un bug ahí, solo comentario.

Recomendación práctica: ¿lo dejamos o lo matamos?
-------------------------------------------------
Yo haría esto:

A) Si querés production-grade:
   - DEPRECATE: dejarlo pero apagado por default.
   - Habilitar solo si `cfg.Debug.EnableSocialResultPage == true` (o env).
   - Sacar `peek` o dejarlo solo en debug.
   - Cambiar postMessage a origin específico (o eliminarlo).

B) Si te sirve para soporte interno:
   - protegerlo con auth (admin) o al menos con un “internal key” (header) en dev.
   - y listo.

C) Si no lo usa nadie:
   - borrarlo y quedarte con social_exchange.go + (si querés) una page en el frontend.

Cambios concretos (mínimos) que yo metería YA
---------------------------------------------
1) PostMessage seguro (si lo dejás):
   - Guardar en cache junto a payload:
       { client_id, tenant_id, response, allowed_origin }
   - En HTML:
       const allowedOrigin = "..."; (inyectado server-side)
       window.opener.postMessage(..., allowedOrigin);

2) Gate de debug:
   - peek solo si env `SOCIAL_DEBUG_RESULT=true`
   - si no, ignorar peek

3) Move template a embed:
   - mejora mantenibilidad sin cambiar funcionalidad

4) Considerar “consume only on JSON/HTML success”
   - hoy consumís antes de decidir formato; está ok.
   - pero si el template falla (raro), ya consumiste.
   - podés mover el delete a después de `t.Execute` si querés fineza.

TL;DR amigo
-----------
Sirve como “visor”/legacy, pero en prod es medio picante por el `postMessage('*')` y el `peek`.
Si lo dejás: harden + gate debug. Si no lo usa nadie: a la bolsa.



userinfo.go — review bien “a lo grande” (paths, capas, responsabilidades, qué separaría, y fixes concretos)

Qué es este handler
-------------------
Implementa el endpoint OIDC UserInfo:
  GET|POST /userinfo   (o el path que lo monte tu router)
que devuelve claims del usuario a partir de un Access Token (Bearer JWT).

En tu caso:
- exige Authorization: Bearer <jwt>
- valida firma EdDSA con `c.Issuer.Keyfunc()`
- (extra) verifica “issuer esperado” por tenant comparando `iss` vs issuer resuelto del tenant
- resuelve store correcto por `tid` (tenant DB vs global)
- busca user por `sub` y arma respuesta con:
   - claims estándar (name, given_name, family_name, picture, locale)
   - email/email_verified solo si scope incluye "email"
   - custom_fields siempre (merge de metadata + columnas dinámicas)

Rutas y formas
--------------
Métodos:
- GET /userinfo
- POST /userinfo
Ambos requieren:
- Header `Authorization: Bearer <token>`
Respuestas:
- 401 + WWW-Authenticate en casos invalid_token
- 200 JSON con claims

Capas (cómo debería estar separado)
-----------------------------------
Ahora todo está en un solo handler. Funciona, pero está mezclando responsabilidades.

Yo lo separaría así (sin cambiar funcionalidad):

1) transport/http (handlers)
   - parsear método + auth header
   - llamar a un servicio `UserInfoService`
   - setear headers (cache-control, content-type, vary, www-auth si falla)
   - serializar JSON

2) domain/service (lógica de negocio)
   - ValidateAccessToken(rawToken) -> Claims (o un struct tipado)
   - ResolveTenantFromClaims(iss, tid) -> tenantSlug + expectedIssuer
   - ValidateIssuerMatch(expected, tokenIss)
   - ResolveUserStore(tenantSlug / tid) -> repo
   - BuildUserInfoResponse(user, scopes) -> map[string]any o struct

3) infra/adapters
   - tenant resolver: `cpctx.Provider` (FS control plane)
   - store resolver: `TenantSQLManager.GetPG`
   - issuer resolver: `jwtx.ResolveIssuer(...)`

Esto te deja:
- test unitarios al service sin HTTP
- el handler se vuelve “finito”, no un monstruo

Puntos fuertes del código actual
--------------------------------
✅ Aceptar GET y POST: ok (OIDC lo permite; muchas libs usan GET).
✅ `Vary: Authorization` + `no-store/no-cache`: perfecto para tokens.
✅ Scope gating de `email` está bien pensado.
✅ custom_fields siempre: consistente con tu flujo de CompleteProfile.
✅ Verificación de issuer per-tenant: está buena para evitar tokens “firmados ok” pero con iss incorrecto.

Los “che, esto lo arreglaría YA”
--------------------------------

1) LOG de token inválido (leak / ruido)
   -----------------------------------
Esto:
  log.Printf("userinfo_invalid_token_debug: err=%v raw_prefix=%s", err, rawPrefix)

- aunque cortás a 20 chars, sigue siendo “material sensible” (y encima en logs).
- además el error puede incluir cosas del parsing.

Fix:
- logueá solo `err` y un request-id, o un hash del token:
    tokHash := tokens.SHA256Base64URL(raw)[:10]
- y hacerlo solo bajo flag de debug.

2) Keyfunc “global” vs Keyfunc “per-tenant”
   -----------------------------------------
Estás usando `c.Issuer.Keyfunc()` (lookup por KID en active/retiring keys). Bien.
Pero ojo con multi-tenant + rotación:
- si tu Keyfunc ya resuelve por KID global y eso incluye keys de varios tenants,
  igual después chequeás issuer, así que ok.
- si querés más duro: primero derivar tenant por `iss` sin confiar en firma...
  (pero sin firma tampoco confías en `iss`). Entonces este orden es aceptable:
  - validar firma -> claims -> validar issuer per-tenant.

3) Resolución de slug desde iss: fallback dudoso
   --------------------------------------------
Estás asumiendo que el slug está en el path o en el último segmento.
Si el issuer override es algo tipo `https://id.acme.com/oidc`, tu fallback “último segmento”
te va a inventar slug = "oidc" y podrías intentar buscar un tenant que no existe.
Hoy eso no falla duro (solo si encuentra tenant y mismatch), pero es raro.

Mejor:
- solo derivar slug si encontrás el patrón explícito `/t/{slug}`.
- si no está, no intentes inferir slug. (O usar `tid` en claims para resolver tenant).

4) Resolver tenant por tid hace ListTenants() (O(N))
   -------------------------------------------------
Este bloque:
  if tenants, err := cpctx.Provider.ListTenants(...); err == nil { for ... }

Para userinfo puede ser muy hot-path. Si tenés 1k tenants, es un bajón.

Fix:
- agregar en provider un método `GetTenantByID(ctx, id)` (ideal)
- o cachear un map id->slug (en memoria con TTL)
- o guardar `tslug` directo en el token (tipo claim `tslug`) y listo.

5) Scopes parsing: `scp` puede ser string en tu sistema
   ----------------------------------------------------
En otros handlers vos manejás:
- scope string
- scp string
- scp []string
Acá solo hacés:
- scp []any
- scope string
Si tu access token emite `scp` como string (en oauth_token hiciste a veces string y a veces []),
userinfo puede “no ver” scopes -> no devuelve email aunque debería.

Fix (robusto):
- aceptar:
  - scp []any
  - scp []string
  - scp string (space-separated)
  - scope string

6) Validar exp/nbf explícito (opcional, pero recomendado)
   -----------------------------------------------------
jwt.Parse con MapClaims por default suele validar exp/nbf si usás `jwt.WithLeeway` / options…
pero depende de cómo lo uses. En v5, `Parse` valida “registered claims” si son tipo Claims correcto,
con MapClaims a veces no es tan estricto como querés.

Para userinfo yo lo dejaría explícito:
- check exp
- check nbf si existe
- y check aud (si querés) o al menos que token sea access token (por ejemplo `token_use=access`).

7) `sub` vacío / no string
   ------------------------
Si sub no viene, respondés {"sub":""} y después intentás GetUserByID("").
Mejor:
- si sub == "" -> 401 invalid_token.

8) GET|POST: soportás POST pero no leés body
   -----------------------------------------
OIDC userinfo POST suele ser igual que GET (bearer token en header), así que no pasa nada.
Pero si alguien manda token en form body (some implementations), vos no lo aceptás.
Está ok si vos definís tu contrato así.

Cómo lo “partiría” (archivos y funciones)
-----------------------------------------
Ejemplo de estructura:

internal/oidc/userinfo/service.go
- type Service struct { Issuer, Provider, Store, TenantSQLManager... }
- func (s *Service) Handle(ctx, rawBearer string) (UserInfoResponse, *OIDCError)

internal/oidc/userinfo/token.go
- func ParseAndValidateAccessToken(raw string) (claims Claims, err error)
- type Claims struct { Sub, Tid, Iss string; Scopes []string; ... }

internal/oidc/userinfo/tenant.go
- func ResolveTenant(ctx, claims Claims) (tenantSlug string, expectedIssuer string, err error)

internal/oidc/userinfo/response.go
- func BuildResponse(u *core.User, scopes []string) map[string]any

internal/http/v1/handlers/userinfo.go
- parse header, call service, write headers, write JSON

Bonus: contratos/errores
------------------------
En vez de repetir WriteError + WWW-Authenticate a mano, hacé helper:
- httpx.WriteOIDCAuthError(w, realm, code, desc, status)

Así te queda consistente con token/introspect/etc.

Mini lista de “cambios” que metería en este mismo archivo sin refactor
----------------------------------------------------------------------
- No loguear raw_prefix.
- Validar `sub != ""` -> 401.
- Scope parsing robusto (scp string/list).
- Derivar slug solo si path tiene `/t/{slug}` (sin fallback al último segmento).
- Evitar ListTenants O(N): cache o método GetTenantByID.

Si querés, te lo reescribo “clean” (mismo comportamiento) ya con:
- parse scopes robusto
- check sub
- safer issuer slug derivation
- debug logging seguro
y lo dejamos listo para después extraer a service.





