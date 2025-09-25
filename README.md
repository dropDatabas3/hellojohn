autonumber
<div align="center">

# HelloJohn – Universal Login & Identity Service

Autenticación unificada (password, OAuth2/OIDC, social Google, MFA TOTP), gestión de sesiones y emisión segura de tokens multi‑tenant.

</div>

---
**Tabla de contenido**
1. Visión general
2. Modelo de datos y entidades
3. Arquitectura lógica y componentes
4. Flujos detallados (diagramas de secuencia)
5. MFA (TOTP) y niveles de autenticación (AMR / ACR)
6. Ciclo de vida de tokens y rotación de claves
7. Catálogo de endpoints (resumen operativo)
8. Seguridad (controles aplicados)
9. Rate limiting semántico
10. Configuración y variables de entorno (referencia unificada)
11. E2E Tests y validación
12. Operación (CLI, migraciones, rotación de claves)
13. Glosario rápido
14. Futuro inmediato (dirección evolutiva)
15. Changelog Sprint 6 (resumen)

---
## 1. Visión general
HelloJohn es un servicio de identidad orientado a aplicaciones web y backend que necesitan:
* Registro y autenticación por email/password con política de contraseñas y blacklist opcional.
* OAuth2 / OpenID Connect Authorization Code + PKCE (S256) para terceros confiables (SPAs, Mobile, Backends).
* Emisión de Access / ID / Refresh tokens firmados con EdDSA (Ed25519), rotación de claves y JWKS dinámico.
* Login social (Google) con state firmado, protección contra CSRF y canje de login_code one‑use.
* Flujos de verificación de email y recuperación de contraseña mediante tokens de un solo uso.
* MFA TOTP con remember device (trusted devices) y códigos de recuperación.
* Introspección de tokens (refresh + access) y revocación masiva de sesiones (/v1/auth/logout-all).
* Rate limiting semántico (login, email flows, MFA) + global.
* Separación clara de responsabilidades: emisión, almacenamiento, cache, cryptografía.

Todo expone respuestas JSON consistentes, encabezados de seguridad y convenciones pensadas para integraciones front/back multi‑tenant.

---
## 2. Modelo de datos y entidades
Principales tablas (PostgreSQL):
| Entidad | Propósito | Campos clave |
|---------|-----------|--------------|
| tenant | Aislamiento lógico multi‑tenant | id, name, slug, settings |
| client | Aplicación registrada (public/confidential) | client_id, redirect_uris, scopes |
| client_version | Esquema/mapeo/versionado claims | status(draft/active/retired) |
| app_user | Usuario final | email (unique por tenant), email_verified |
| identity | Métodos de autenticación (password, google) | provider, password_hash / provider_user_id |
| refresh_token | Cadena rotativa de refresh opacos | token_hash, rotated_from, revoked_at |
| email_verification_token | Token single-use verificación | token_hash, expires_at, used_at |
| password_reset_token | Token reset password | idem verify |
| signing_keys | Claves Ed25519 con estados | kid, status(active/retiring/retired) |
| user_mfa_totp | Secreto TOTP cifrado + metadata | secret_encrypted, confirmed_at, last_used_at |
| mfa_recovery_code | Códigos recuperación hash | code_hash, used_at |
| trusted_device | Dispositivos confiables (remember) | device_hash, expires_at |

Relaciones simplificadas:
```mermaid
erDiagram
  TENANT ||--o{ CLIENT : has
  CLIENT ||--o{ CLIENT_VERSION : versions
  TENANT ||--o{ APP_USER : has
  APP_USER ||--o{ IDENTITY : methods
  APP_USER ||--o{ REFRESH_TOKEN : sessions
  APP_USER ||--o| USER_MFA_TOTP : mfa
  APP_USER ||--o{ MFA_RECOVERY_CODE : recovery
  APP_USER ||--o{ TRUSTED_DEVICE : devices
```

---
## 3. Arquitectura lógica y componentes
```mermaid
flowchart LR
  subgraph Edge
    Client[Browser / SPA / Backend]
  end
  Client -->|OAuth2 / REST| API[HTTP Layer]
  API --> MW[Middlewares: CORS, SecHeaders, Rate, Logging, Recover]
  API --> H[Handlers /v1/* /oauth2/*]
  H --> Issuer[JWT Issuer]
  H --> Store[(Postgres)]
  H --> Cache[(Cache)]
  Cache --- CNote
  CNote["Redis o Memory (configurable)"]
  H --> Email[SMTP + Templates]
  H --> Rate[Redis Limiter]
  Issuer --> Keystore[(signing_keys)]
  H --> MFA[Trusted Devices + TOTP]
```

Características clave:
* Capa HTTP concentra validaciones, parseo de entradas y respuesta consistente.
* Keystore persistente permite rotar claves sin reinicios (JWKS refleja estados).
* Cache abstracta (memory/redis) para códigos efímeros (login_code, mfa_token).
* Rate limiter multi‑pool: límites distintos por endpoint semántico.
* Seguridad de secretos MFA y claves privadas: AES‑GCM (prefijo GCMV1 / GCMV1-MFA:).

---
## 4. Flujos detallados (diagramas de secuencia)
### 4.1 Registro + Login
```mermaid
sequenceDiagram
autonumber
participant U as Usuario
participant S as Servicio
U->>S: POST /v1/auth/register
S-->>U: 200 (tokens opcional) / 201 / 204
U->>S: POST /v1/auth/login (email,pwd)
alt MFA activa y no trusted
  S-->>U: {mfa_required:true, mfa_token}
  U->>S: POST /v1/mfa/totp/challenge (mfa_token + code)
  S-->>U: access + refresh
else Sin MFA o trusted device
  S-->>U: access + refresh
end
```

### 4.2 OAuth2 Authorization Code (+ PKCE)
```mermaid
sequenceDiagram
participant B as Browser
participant S as Servicio
participant A as App Backend
B->>S: GET /oauth2/authorize (code_challenge)
S-->>B: (si sin sesión) pedir /v1/session/login
B->>S: POST /v1/session/login (crea cookie)
B->>S: GET /oauth2/authorize (cookie válida)
alt MFA requerida en sesión
  S-->>B: JSON {mfa_required,true,mfa_token}
  B->>S: POST /v1/mfa/totp/challenge
  S-->>B: continuar authorize
end
S-->>B: 302 redirect_uri?code=CODE&state
A->>S: POST /oauth2/token (code + code_verifier)
S-->>A: access + id + refresh
```

### 4.3 Refresh rotativo
```mermaid
sequenceDiagram
participant C as Cliente
participant S as Servicio
C->>S: POST /v1/auth/refresh (R1)
S->>S: valida hash(R1) & no revoked
S->>S: crea R2(rotated_from=R1), revoca R1
S-->>C: access + R2
```

### 4.4 Password Reset
```mermaid
sequenceDiagram
participant U as Usuario
participant S as Servicio
U->>S: POST /v1/auth/forgot
S-->>U: 204 (responde siempre neutro)
U->>S: POST /v1/auth/reset (token,new_password)
S-->>U: 200/204 (y tokens si auto-login habilitado)
```

### 4.5 Verificación Email
```mermaid
sequenceDiagram
U->>S: POST /v1/auth/verify-email/start
S-->>U: 204 (email enviado)
U->>S: GET /v1/auth/verify-email?token=XYZ
S-->>U: 200 email_verified=true
```

### 4.6 Social Google + login_code
```mermaid
sequenceDiagram
Browser->>S: /v1/auth/social/google/start
S-->>Browser: 302 Google (state firmado)
Google-->>Browser: 302 callback (state, code)
Browser->>S: /callback (valida state, obtiene tokens ó genera login_code)
alt redirect_uri presente
  S-->>Browser: 302 redirect_uri?code=login_code
  Browser->>S: POST /v1/auth/social/exchange {code}
  S-->>Browser: access + refresh
else sin redirect_uri
  S-->>Browser: tokens JSON directos
end
```

### 4.7 MFA Login (password) con remember device
```mermaid
sequenceDiagram
participant U as Usuario
participant S as Servicio
U->>S: POST /v1/auth/login
S->>S: detecta MFA habilitada sin trusted device
S-->>U: {mfa_required,mfa_token}
U->>S: POST /v1/mfa/totp/challenge (code,remember_device:true)
S->>S: valida code, registra trusted_device
S-->>U: access + refresh (amr=[pwd,mfa], acr=loa:2)
U->>S: POST /v1/auth/login (posterior)
S-->>U: tokens directos (trusted)
```

---
## 5. MFA (TOTP) y niveles de autenticación
Componentes:
* user_mfa_totp: guarda secreto en Base32 cifrado AES‑GCM (prefijo GCMV1-MFA:).
* mfa_recovery_code: 10 códigos alfanuméricos (A-Z 2-9 sin ILOU) hash SHA256 base64url(lower).
* trusted_device: hash de token aleatorio (cookie mfa_trust) con TTL configurable.

Endpoints:
| Método | Path | Uso |
|--------|------|-----|
| POST | /v1/mfa/totp/enroll | Genera secreto y otpauth:// |
| POST | /v1/mfa/totp/verify | Confirma primer código y entrega recovery codes |
| POST | /v1/mfa/totp/challenge | Completa login/authorize (code o recovery) |
| POST | /v1/mfa/totp/disable | Requiere password + code/recovery |
| POST | /v1/mfa/recovery/rotate | Password + segundo factor; nuevos codes |

AMR / ACR:
* Sin MFA: amr=["pwd"], acr=urn:hellojohn:loa:1
* Con MFA exitosa / trusted device: amr incluye "mfa", acr=urn:hellojohn:loa:2

Ventana de verificación TOTP configurable con `MFA_TOTP_WINDOW` (±N intervalos de 30s). Remember TTL por `MFA_REMEMBER_TTL`.

---
## 6. Ciclo de vida de tokens y rotación de claves
Tipos de tokens:
| Tipo | Formato | Persistencia | Rotación | Revocación |
|------|---------|-------------|----------|------------|
| Access | JWT EdDSA | Volátil (no DB) | TTL (default ~15m) | Expira / al perder refresh chain |
| ID | JWT EdDSA | Volátil | Igual Access | Igual |
| Refresh | Opaque (hash) | Tabla refresh_token | Por uso (rotated_from) | revoked_at / logout / logout-all |
| Email Verify | Opaque hash | email_verification_token | N/A | used_at / TTL |
| Password Reset | Opaque hash | password_reset_token | N/A | used_at / TTL |
| login_code (social) | Opaque (cache) | Redis/Memory | One-use | Eliminado al canjear |
| mfa_token | Opaque (cache) | Redis/Memory | TTL corto (5m) | Delete al éxito |

Rotación de claves:
1. `go run ./cmd/keys -rotate` → nueva clave `active`, anterior pasa a `retiring`.
2. Tras ventana de gracia: `go run ./cmd/keys -retire -retire-after=168h` marca como `retired`.
3. JWKS expone `active` y `retiring` (para verificación externa).

Firmas Ed25519: header contiene KID. Privadas opcionalmente cifradas con master key (`SIGNING_MASTER_KEY`).

---
## 7. Catálogo de endpoints (resumen operativo)
Autenticación básica:
`POST /v1/auth/register`, `POST /v1/auth/login`, `POST /v1/auth/refresh`, `POST /v1/auth/logout`, `GET /v1/me`, `POST /v1/auth/logout-all`

Sesiones navegador:
`POST /v1/session/login`, `POST /v1/session/logout`

OAuth2 / OIDC:
`GET /oauth2/authorize`, `POST /oauth2/token`, `POST /oauth2/revoke`, `GET|POST /userinfo`, `GET /.well-known/openid-configuration`, `GET /.well-known/jwks.json`, `POST /oauth2/introspect`

Email flows:
`POST /v1/auth/verify-email/start`, `GET /v1/auth/verify-email`, `POST /v1/auth/forgot`, `POST /v1/auth/reset`

MFA:
`POST /v1/mfa/totp/enroll`, `POST /v1/mfa/totp/verify`, `POST /v1/mfa/totp/challenge`, `POST /v1/mfa/totp/disable`, `POST /v1/mfa/recovery/rotate`

Social:
`GET /v1/auth/social/google/start`, `GET /v1/auth/social/google/callback`, `POST /v1/auth/social/exchange`, `GET /v1/auth/providers`

Salud:
`GET /readyz`

### 7.1 Endpoints Administración (Sprint 6)
API base `/v1/admin/*` protegida (futuro: middleware admin/RBAC). Actualmente expone operaciones CRUD y gestión de consentimientos.

Scopes & Consents introducen persistencia dinámica de permisos de OAuth2 por usuario/cliente, con validación estricta de nombres.

| Método | Path | Descripción | Notas |
|--------|------|-------------|-------|
| GET | /v1/admin/clients?tenant_id= | Lista clientes por tenant | Filtro `q` opcional |
| POST | /v1/admin/clients | Crea cliente | Campos oblig: tenant_id, client_id, name, client_type |
| GET | /v1/admin/clients/{id} | Obtiene cliente + versión activa | id UUID |
| PUT | /v1/admin/clients/{id} | Actualiza (sin cambiar client_id) | 204 sin body |
| DELETE | /v1/admin/clients/{id}?soft=true | Elimina (o solo revoca sesiones si soft) | Revoca refresh antes |
| POST | /v1/admin/clients/{id}/revoke | Revoca todas las sesiones del cliente | Idempotente |
| GET | /v1/admin/scopes?tenant_id= | Lista scopes registrados | Orden alfabético |
| POST | /v1/admin/scopes | Crea scope | Valida regex y minúsculas, conflicto 409 |
| PUT | /v1/admin/scopes/{id} | Actualiza descripción | No renombra, 204 idempotente |
| DELETE | /v1/admin/scopes/{id} | Elimina scope si no está en uso | 409 si user_consent activo lo referencia |
| POST | /v1/admin/consents/upsert | Inserta o amplía consentimiento | Acepta client_id público o UUID |
| GET | /v1/admin/consents?user_id=&client_id=&active_only= | Filtra consentimientos | user+client => 0..1 |
| GET | /v1/admin/consents/by-user/{userID} | Lista consentimientos de usuario | `active_only=true` opcional |
| POST | /v1/admin/consents/revoke | Revoca consentimiento (soft) | Revoca refresh tokens asociados |
| DELETE | /v1/admin/consents/{user_id}/{client_id} | Alias a revoke now() | Idempotente |
| GET | /v1/admin/rbac/users/{userID}/roles | Lista roles | Repos opcionales RBAC |
| POST | /v1/admin/rbac/users/{userID}/roles | Añade / quita roles | Campos add/remove |
| GET | /v1/admin/rbac/roles/{role}/perms | Lista permisos rol | Requiere bearer válido |
| POST | /v1/admin/rbac/roles/{role}/perms | Añade / quita permisos | Dedup interno |

#### 7.1.1 Validación de nombres de scope
Patrón aplicado (regex): `^[a-z0-9](?:[a-z0-9:_\.-]{0,62}[a-z0-9])?$`
Reglas derivadas:
* Longitud 1–64.
* Solo minúsculas; debe iniciar y terminar alfanumérico.
* Caracteres internos permitidos: `:` `_` `.` `-`.
* Rechazo explícito de mayúsculas o espacios.
* Conflicto de unicidad (tenant_id, name) ⇒ 409 `scope_exists`.

#### 7.1.2 Eliminación segura de scopes
Antes: delete directo podía dejar consentimientos apuntando a scopes inexistentes.
Ahora: `DELETE /v1/admin/scopes/{id}` ejecuta comprobación manual `EXISTS` sobre `user_consent` activo en el mismo tenant; si está en uso ⇒ 409 `scope_in_use`.
Racional: `granted_scopes` es `TEXT[]` y no puede tener FK directa; se preserva integridad referencial lógica.

#### 7.1.3 Upsert de consentimientos
`POST /v1/admin/consents/upsert` realiza unión de scopes evitando duplicados y re-activa consentimientos revocados (pone `revoked_at=NULL`). Escenarios de OIDC con autoconsent generan el registro sin interacción extra para scopes básicos.

#### 7.1.4 Autoconsent (reiteración)
Ver sección 10.0 — habilita consentimiento implícito cuando todos los scopes solicitados ⊆ set permitido (`CONSENT_AUTO_SCOPES`). Mejora DX de first‑party apps.

#### 7.1.5 Revocación de consentimientos
Revocar (POST/DELETE) marca `revoked_at` y revoca refresh tokens del par (user, client) para impedir refresh posterior. Access tokens existentes expiran naturalmente.

---
## 8. Seguridad (controles aplicados)
| Control | Descripción |
|---------|-------------|
| Argon2id | Hash fuerte de contraseñas (memoria elevada). |
| Blacklist opcional | Rechazo de contraseñas débiles conocidas. |
| PKCE S256 | Mitiga interceptación del authorization code. |
| Refresh rotation | Minimiza replay y disminuye ventana de abuso. |
| Ed25519 + JWKS | Firmas modernas y validación pública rotativa. |
| AES-GCM reposo | Protege secretos TOTP y claves privadas (prefijos GCMV1 / GCMV1-MFA). |
| No-store headers | Tokens nunca cacheados (Cache-Control no-store, Pragma no-cache). |
| CSP & Headers | CSP estricta, X-Frame-Options DENY, Referrer-Policy, HSTS (según entorno). |
| Introspección | Validación estado de refresh/access (firma/exp). |
| Logout-all | Revocación masiva para respuesta a incidentes. |
| MFA TOTP + trusted | Aumenta ACR y añade segunda barrera. |
| Rate semántico | Limita abuso contextual (login/email/MFA). |

---
## 9. Rate limiting semántico
Dos capas:
1. Global (IP+path).  
2. Pools específicos: login, forgot/reset, verify-email, MFA (enroll/verify/challenge/disable).  
Redis almacena contadores con TTL igual a la ventana. Ante fallo de Redis el sistema es fail‑open (registro en log).

---
## 10. Configuración y variables de entorno
Precedencia: defaults → config.yaml → env → flags.

Categorías principales (clave = variable, valor = función):
* Servidor: SERVER_ADDR, SERVER_CORS_ALLOWED_ORIGINS
* JWT: JWT_ISSUER, JWT_ACCESS_TTL, JWT_REFRESH_TTL
* Storage: STORAGE_DRIVER, STORAGE_DSN, POSTGRES_MAX_OPEN_CONNS, POSTGRES_MAX_IDLE_CONNS, POSTGRES_CONN_MAX_LIFETIME
* Cache/Redis: CACHE_KIND, REDIS_ADDR, REDIS_DB, REDIS_PREFIX, MEMORY_DEFAULT_TTL
* Registro/Auth: REGISTER_AUTO_LOGIN, AUTH_ALLOW_BEARER_SESSION
* Sesión: AUTH_SESSION_COOKIE_NAME, AUTH_SESSION_DOMAIN, AUTH_SESSION_SAMESITE, AUTH_SESSION_SECURE, AUTH_SESSION_TTL
* Email flows: AUTH_VERIFY_TTL, AUTH_RESET_TTL, AUTH_RESET_AUTO_LOGIN, EMAIL_BASE_URL, EMAIL_TEMPLATES_DIR, EMAIL_DEBUG_LINKS
* SMTP: SMTP_HOST, SMTP_PORT, SMTP_USERNAME, SMTP_PASSWORD, SMTP_FROM, SMTP_TLS, SMTP_INSECURE_SKIP_VERIFY
* Rate global: RATE_ENABLED, RATE_WINDOW, RATE_MAX_REQUESTS
* Rate específicos: RATE_LOGIN_LIMIT, RATE_LOGIN_WINDOW, RATE_FORGOT_LIMIT, RATE_FORGOT_WINDOW, RATE_MFA_ENROLL_LIMIT, RATE_MFA_ENROLL_WINDOW, RATE_MFA_VERIFY_LIMIT, RATE_MFA_VERIFY_WINDOW, RATE_MFA_CHALLENGE_LIMIT, RATE_MFA_CHALLENGE_WINDOW, RATE_MFA_DISABLE_LIMIT, RATE_MFA_DISABLE_WINDOW
* Password policy: SECURITY_PASSWORD_POLICY_* , SECURITY_PASSWORD_BLACKLIST_PATH
* Social: GOOGLE_ENABLED, GOOGLE_CLIENT_ID, GOOGLE_CLIENT_SECRET, GOOGLE_REDIRECT_URL, GOOGLE_SCOPES, GOOGLE_ALLOWED_TENANTS, GOOGLE_ALLOWED_CLIENTS, SOCIAL_LOGIN_CODE_TTL
* Introspección: INTROSPECT_BASIC_USER, INTROSPECT_BASIC_PASS
* MFA: MFA_TOTP_ISSUER, MFA_TOTP_WINDOW, MFA_REMEMBER_TTL
* Claves: SIGNING_MASTER_KEY

### 10.0 Autoconsent de scopes básicos (novedad)
Contexto del cambio: anteriormente, cuando un usuario autenticado (sesión cookie o bearer) iniciaba un flujo `GET /oauth2/authorize` y aún no existía consentimiento almacenado para todos los scopes solicitados, el endpoint devolvía `200 {"consent_required": true}` obligando a una interacción adicional incluso para scopes totalmente básicos (`openid email profile`).

Con la nueva lógica se incorporó un modo de autoconsent seguro y opinado que mejora la DX en entornos first‑party:

Qué se arregló y por qué ahora pasan los tests 04/05:
* Antes: falta de consentimiento ⇒ `200 consent_required` (los tests esperaban un `302` con `code`).
* Ahora: si todos los scopes solicitados están dentro del conjunto autoconsentible, se persiste el consentimiento inmediatamente y se emite el authorization code (`302 redirect ...?code=...`).

Reglas del autoconsent:
* Habilitado por defecto (opt‑out) mediante `CONSENT_AUTO` (default `1`).
* Conjunto permitido por defecto: `openid email profile` (configurable vía `CONSENT_AUTO_SCOPES`).
* Sólo se aplica si TODOS los scopes pedidos ⊆ set permitido.
* Si algún scope queda fuera (ej: `offline_access`), se mantiene el comportamiento anterior (`consent_required`).
* La persistencia usa `UpsertConsent` (multi‑tenant compatible) y respeta revocaciones previas (si el registro está revocado se sigue pidiendo consentimiento explícito).

Variables nuevas / documentadas:
```
CONSENT_AUTO=1                     # 1 (default) = autoconsent activo, 0 = modo estricto (siempre pedir)
CONSENT_AUTO_SCOPES="openid email profile"  # Lista separada por espacios; vacía => se usa el default interno
```

Seguridad vs Facilidad:
* Seguridad: no se autoconsiente nada fuera del set configurado. Por defecto NO incluye `offline_access` ni scopes sensibles personalizados.
* Facilidad: la mayoría de SPAs / first‑party inmediatamente obtienen el code sin un paso intermedio, alineando la UX con otros IdPs.
* Personalizable: basta con poner `CONSENT_AUTO=0` para endurecer, o ampliar `CONSENT_AUTO_SCOPES` según políticas locales.

Sugerencias futuras opcionales (no implementadas todavía):
* Flag `Trusted` en el client para limitar autoconsent sólo a aplicaciones de primera parte.
* Registro de auditoría cuando se aplique autoconsent (user, client_id, scopes) para trazabilidad.

### 10.0.1 Endpoint /oauth2/revoke (tolerancia de formato)
Se reforzó la robustez del handler de revocación (RFC 7009):
* Fuente primaria: `application/x-www-form-urlencoded` (`token=...`).
* Fallbacks adicionales: header `Authorization: Bearer <token>` o cuerpo JSON `{ "token": "..." }` si el form viene vacío.
* Si tras todos los métodos no hay token ⇒ `400` con mismo código de error previo (no cambia semántica ni filtra información sobre tokens inexistentes).
* Idempotencia preservada: revocar un token inválido o ya revocado devuelve respuesta neutra (200) cuando corresponde.

Motivación: mejorar compatibilidad con clientes que por error envían Bearer / JSON en vez de form sin relajar requisitos ni revelar existencia de tokens.

Resumen impacto: ninguna migración requerida; servicios que ya funcionaban no necesitan cambios. Tests 04/05 (OIDC) vuelven a verde gracias al 302 con code.


Inicio rápido (dev):
```bash
cp configs/config.example.yaml configs/config.yaml
export STORAGE_DSN=postgres://user:password@localhost:5432/login?sslmode=disable
export JWT_ISSUER=http://localhost:8080
export FLAGS_MIGRATE=true
go run ./cmd/service -env
```

### 10.1 Activar blacklist de contraseñas

La blacklist es opcional y rechaza contraseñas exactamente iguales (case-insensitive) a alguna entrada conocida como débil.

Pasos:
1. Crear un archivo (por ejemplo `./configs/password_blacklist.txt`). Formato: una contraseña por línea. Líneas vacías o que comiencen con `#` se ignoran.
2. Añadir entradas comunes (ej: `password`, `123456`, `qwerty`, `letmein`). Evita incluir contraseñas reales de usuarios.
3. Definir la ruta por YAML (`security.password_blacklist_path`) o por ENV:
   ```bash
   export SECURITY_PASSWORD_BLACKLIST_PATH=./configs/password_blacklist.txt
   ```
4. Reiniciar el servicio (el archivo se carga y se cachea una sola vez al arranque).

Verificación rápida (dev):
```bash
curl -s -X POST http://localhost:8080/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password"}' | jq
```
Debe responder 400 con error `policy_violation` indicando que la contraseña no cumple política / blacklist.

Deshabilitar: dejar el campo vacío en YAML o no exportar la variable de entorno.

---
## 11. E2E Tests y validación
Suite ubicada en `test/e2e` levanta el servicio, corre migraciones, genera claves, hace seed y ejecuta casos: registro/login, refresh rotativo, email flows, OAuth2, social login_code, MFA, introspección, blacklist.  
Los tests garantizan: no-store en respuestas con credenciales, rotación de refresh, revocación correcta, uso único de login_code y recuperación MFA.

### 11.1 Nuevos casos Sprint 6
| Archivo | Propósito clave |
|---------|-----------------|
| 21_password_blacklist_test.go | Verifica rechazo por blacklist (parte hardening password) |
| 22_admin_clients_test.go | CRUD básico de clientes + revocación sesiones (pending si no mergeado) |
| 23_admin_scopes_test.go | Creación, listado, delete in-use (409), update descripción |
| 24_admin_consents_test.go | Upsert, listado activo, revocación y refresh revoke |
| 25_admin_users_disable_test.go | Escenario de desactivación usuario (placeholder si pendiente) |
| 26_token_claims_test.go | Validaciones avanzadas de claims token (placeholder) |

### 11.2 Cambios relevantes existentes
* OIDC tests (04/05) ahora reciben `302` directo cuando aplica autoconsent.
* Rate y MFA sin cambios de contrato.

---
## 12. Operación (CLI, migraciones, claves)
| Acción | Comando |
|--------|---------|
| Migrar DB | `go run ./cmd/migrate` (usa FLAGS_MIGRATE en service) |
| Seed inicial | `go run ./cmd/seed` (variables SEED_*) |
| Servidor | `go run ./cmd/service -env -env-file .env` |
| Rotar clave | `go run ./cmd/keys -rotate` |
| Listar claves | `go run ./cmd/keys -list` |
| Retirar claves | `go run ./cmd/keys -retire -retire-after=168h` |

Health: `/readyz` realiza ping a DB, valora cache Redis y firma+parsea un JWT para verificar keystore y cifrado.

---
## 13. Glosario rápido
| Término | Explicación |
|---------|------------|
| AMR | Métodos de autenticación empleados (pwd, mfa, google, refresh...). |
| ACR | Nivel de autenticación alcanzado (loa:1 básico, loa:2 con MFA). |
| PKCE S256 | Protección de authorization code mediante code_challenge hash. |
| login_code | Código efímero social one‑use para intercambiar por tokens. |
| MFA Trusted Device | Cookie + hash persistido que evita re-desafío TOTP dentro de TTL. |
| Refresh Rotation | Patrón de revocar el refresh anterior al emitir uno nuevo. |
| JWKS | Conjunto JSON de claves públicas activas/retiring. |
| GCMV1 / GCMV1-MFA | Prefijos de blobs cifrados AES-GCM (claves/mfa). |

---
## 14. Futuro inmediato (dirección evolutiva)
* Extender providers sociales y soporte WebAuthn.
* Observabilidad: métricas Prometheus y trazas OpenTelemetry.
* Panel / API de administración (sessions listing, per-client revocation, scopes dinámicos).
* Hooks dinámicos de claims (CEL / webhooks) para personalización avanzada.
* Mayor gobernanza de sesiones: revocación selectiva por device/trusted.

### 14.1 Próximos pasos administración
* Middleware Admin Auth (API Key / cookie / RBAC bearer) pendiente.
* UI React Panel (`/ui/admin`) para gestionar clientes, scopes, consents, usuarios.
* Enforzar validación de scopes solicitados vs registro al authorize (actualmente permitir libre + registro incremental opcional).

---
## 15. Changelog Sprint 6 (resumen)
| Ítem | Descripción |
|------|-------------|
| Migración 0003 | Tablas `scope` y `user_consent` + índices GIN y activos |
| Scopes API | CRUD con validación regex y delete seguro (409 in-use) |
| Consents API | Upsert union, revocación soft + revoca refresh tokens |
| Autoconsent | `CONSENT_AUTO` + `CONSENT_AUTO_SCOPES` para baseline scopes |
| Revocar robusto | /oauth2/revoke acepta form, JSON, Authorization Bearer |
| Password blacklist | Archivo configurable y test dedicado |
| Seed compat | Soporte campo `sub` -> `id` en seed YAML tests |
| RBAC inicial | Handlers roles/perms (lectura/escritura) sujetos a repos opcionales |

---
© 2025 HelloJohn – Documentación funcional completa.
