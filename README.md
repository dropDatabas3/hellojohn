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
15. Changelog (resumen)

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
<div align="center">

# HelloJohn – Universal Login & Identity Service

Autenticación unificada (password, OAuth2/OIDC, social Google, MFA TOTP), gestión de sesiones y emisión segura de tokens multi‑tenant.

</div>

---

## Índice
- [1. Introducción](#1-introducción)
- [2. Características](#2-características)
- [3. Arquitectura](#3-arquitectura)
- [4. Estructura del proyecto](#4-estructura-del-proyecto)
- [5. Requisitos](#5-requisitos)
- [6. Puesta en marcha rápida](#6-puesta-en-marcha-rápida)
  - [6.1 Con Docker (DB/Cache)](#61-con-docker-dbcache)
  - [6.2 Ejecutar el servicio](#62-ejecutar-el-servicio)
- [7. Configuración](#7-configuración)
  - [7.1 Variables clave](#71-variables-clave)
  - [7.2 Blacklist de contraseñas](#72-blacklist-de-contraseñas)
- [8. Catálogo de endpoints](#8-catálogo-de-endpoints)
  - [8.1 Administración (JWT admin)](#81-administración-jwt-admin)
- [9. Admin: autenticación y autorización](#9-admin-autenticación-y-autorización)
- [10. Flujos principales](#10-flujos-principales)
- [11. Seguridad](#11-seguridad)
- [12. Rate limiting](#12-rate-limiting)
- [13. Migraciones, seed y claves](#13-migraciones-seed-y-claves)
- [14. Pruebas E2E](#14-pruebas-e2e)
- [15. Operación y salud](#15-operación-y-salud)
- [16. Roadmap](#16-roadmap)
- [17. Troubleshooting](#17-troubleshooting)

---

## 1. Introducción
HelloJohn es un servicio de identidad orientado a apps web y backends que necesitan:
- Registro/login por email/password con política de contraseñas y blacklist opcional.
- OAuth2/OIDC Authorization Code + PKCE (S256) para SPAs, mobile y backends.
- Emisión de Access/ID/Refresh tokens (EdDSA Ed25519) con JWKS y rotación de claves.
- Login social (Google) con state firmado y código efímero (login_code).
- Flujos de verificación de email y recuperación de contraseña.
- MFA TOTP con trusted devices y códigos de recuperación.
- Introspección de tokens y revocación masiva de sesiones.
- Rate limiting semántico.

Todo expone JSON consistente, headers de seguridad y convenciones pensadas para integraciones multi‑tenant.

---

## 2. Características
- Multi‑tenant y clients versionados.
- Consentimientos por usuario/cliente con scopes dinámicos.
- Revocación de refresh tokens en cascada al revocar consentimientos/cliente o desactivar usuario.
- Keystore persistente con rotación de claves y JWKS.
- Cache abstracta (memory/redis) y rate limiter semántico.

---

## 3. Arquitectura
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

---

## 4. Estructura del proyecto
```
cmd/
  service/        # Servicio HTTP principal
  migrate/        # Migraciones DB
  seed/           # Seed de datos
  keys/           # Gestión de claves (rotate/list/retire)
configs/          # YAML (ejemplo y real) + blacklist opcional
deployments/      # docker-compose (Postgres/Redis) y Dockerfile (WIP)
internal/         # Código del dominio (HTTP, handlers, stores, JWT, MFA, etc.)
migrations/       # SQL/JS para motores soportados
templates/        # Emails (txt/html)
test/             # Suite E2E
```

---

## 5. Requisitos
- Go 1.23+
- Postgres 16 (dev mediante docker‑compose) y Redis (opcional, recomendado para rate/cache)
- SMTP para emails (en desarrollo puede usarse un servidor de pruebas)

---

## 6. Puesta en marcha rápida

### 6.1 Con Docker (DB/Cache)
En desarrollo podés levantar Postgres y Redis con `deployments/docker-compose.yml`.

Opcional
```
docker compose -f deployments/docker-compose.yml up -d
```

### 6.2 Ejecutar el servicio
1) Copiá el ejemplo y/o usa variables de entorno:
```
cp configs/config.example.yaml configs/config.yaml
```
2) Definí una master key para cifrado (mínimo 32 bytes):
```
setx SIGNING_MASTER_KEY 0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef
```
3) Iniciá el servicio (con migraciones automáticas):
```
go run ./cmd/service -env
```

Notas
- Por defecto escucha en :8080.
- El flag `-env` usa solo variables de entorno (y `.env` si pasás `-env-file`).

---

## 7. Configuración
Precedencia: defaults → config.yaml → env → flags.

### 7.1 Variables clave
- Servidor: SERVER_ADDR, SERVER_CORS_ALLOWED_ORIGINS
- JWT: JWT_ISSUER, JWT_ACCESS_TTL, JWT_REFRESH_TTL
- Storage: STORAGE_DRIVER, STORAGE_DSN, POSTGRES_MAX_OPEN_CONNS, POSTGRES_MAX_IDLE_CONNS, POSTGRES_CONN_MAX_LIFETIME
- Cache/Redis: CACHE_KIND, REDIS_ADDR, REDIS_DB, REDIS_PREFIX, CACHE_MEMORY_DEFAULT_TTL
- Registro/Auth: REGISTER_AUTO_LOGIN, AUTH_ALLOW_BEARER_SESSION
- Sesión: AUTH_SESSION_COOKIE_NAME, AUTH_SESSION_DOMAIN, AUTH_SESSION_SAMESITE, AUTH_SESSION_SECURE, AUTH_SESSION_TTL
- Introspección: INTROSPECT_BASIC_USER, INTROSPECT_BASIC_PASS
- Email flows/SMTP: AUTH_VERIFY_TTL, AUTH_RESET_TTL, AUTH_RESET_AUTO_LOGIN, EMAIL_BASE_URL, EMAIL_TEMPLATES_DIR, EMAIL_DEBUG_LINKS, SMTP_*
- Rate global/por‑endpoint: RATE_ENABLED, RATE_WINDOW, RATE_MAX_REQUESTS, RATE_* (login/forgot/MFA)
- Password: SECURITY_PASSWORD_POLICY_*, SECURITY_PASSWORD_BLACKLIST_PATH
- Social Google: GOOGLE_ENABLED, GOOGLE_CLIENT_ID/SECRET, GOOGLE_REDIRECT_URL, GOOGLE_SCOPES, GOOGLE_ALLOWED_TENANTS/CLIENTS, SOCIAL_LOGIN_CODE_TTL
- Claves: SIGNING_MASTER_KEY

Autoconsent (seguro por defecto):
- CONSENT_AUTO=1
- CONSENT_AUTO_SCOPES="openid email profile"

### 7.2 Blacklist de contraseñas
1. Crear archivo con una contraseña por línea. Líneas vacías o con `#` se ignoran.
2. Definir ruta vía YAML o env `SECURITY_PASSWORD_BLACKLIST_PATH`.
3. Reiniciar el servicio.

---

## 8. Catálogo de endpoints
Autenticación básica:
`POST /v1/auth/register`, `POST /v1/auth/login`, `POST /v1/auth/refresh`, `POST /v1/auth/logout`, `POST /v1/auth/logout-all`, `GET /v1/me`

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

### 8.1 Administración (JWT admin)
API base `/v1/admin/*` protegida por RequireAuth + RequireSysAdmin (ver sección 9).

Scopes & Consents introducen persistencia dinámica de permisos por usuario/cliente, con validación estricta y borrado seguro.

| Método | Path | Descripción | Notas |
|--------|------|-------------|-------|
| GET | /v1/admin/clients?tenant_id= | Lista clientes por tenant | Filtro `q` opcional |
| POST | /v1/admin/clients | Crea cliente | Requiere tenant_id, client_id, name, client_type |
| GET | /v1/admin/clients/{id} | Obtiene cliente + versión activa | id UUID |
| PUT | /v1/admin/clients/{id} | Actualiza (sin cambiar client_id) | 204 |
| DELETE | /v1/admin/clients/{id}?soft=true | Elimina o solo revoca sesiones | Revoca refresh antes |
| POST | /v1/admin/clients/{id}/revoke | Revoca todas las sesiones del cliente | Idempotente |
| GET | /v1/admin/scopes?tenant_id= | Lista scopes | Orden alfabético |
| POST | /v1/admin/scopes | Crea scope | Valida regex/minúsculas, 409 si existe |
| PUT | /v1/admin/scopes/{id} | Actualiza descripción | No renombra |
| DELETE | /v1/admin/scopes/{id} | Elimina si no está en uso | 409 si en uso |
| POST | /v1/admin/consents/upsert | Inserta o amplía consentimiento | Acepta client_id público o UUID |
| GET | /v1/admin/consents?user_id=&client_id=&active_only= | Filtra consentimientos | user+client ⇒ 0..1 |
| GET | /v1/admin/consents/by-user/{userID} | Lista consentimientos de usuario | `active_only` opcional |
| POST | /v1/admin/consents/revoke | Revoca consentimiento (soft) | Revoca refresh tokens |
| DELETE | /v1/admin/consents/{user_id}/{client_id} | Alias de revoke now() | Idempotente |
| GET | /v1/admin/rbac/users/{userID}/roles | Lista roles | Repos RBAC opcionales |
| POST | /v1/admin/rbac/users/{userID}/roles | Añade/Quita roles | Campos add/remove |
| GET | /v1/admin/rbac/roles/{role}/perms | Lista permisos rol | |
| POST | /v1/admin/rbac/roles/{role}/perms | Añade/Quita permisos | |

Validación de scopes (regex): `^[a-z0-9](?:[a-z0-9:_\.-]{0,62}[a-z0-9])?$` (1–64, minúsculas, caracteres permitidos `:_.-`).

Eliminación segura: antes de borrar un scope se verifica que no esté referenciado por `user_consent` activo en el mismo tenant; si lo está ⇒ 409 `scope_in_use`.

Upsert de consentimientos: unión de scopes sin duplicar y reactivación (revoked_at=NULL) cuando corresponda. La revocación marca `revoked_at` y revoca refresh del par (user, client).

---

## 9. Admin: autenticación y autorización
Las rutas `/v1/admin/*` se protegen con:
- RequireAuth: exige JWT Bearer válido emitido por el issuer configurado.
- RequireSysAdmin: verifica privilegios de administrador del sistema.

Política (middleware `RequireSysAdmin`):
1) Si `ADMIN_ENFORCE` ≠ "1" ⇒ permitir (modo dev/compatibilidad).
2) En claims.custom[SystemNamespace(iss)].is_admin == true ⇒ permitir.
3) En claims.custom[SystemNamespace(iss)].roles incluye "sys:admin" ⇒ permitir.
4) Fallback de emergencia: `sub` ∈ `ADMIN_SUBS` (CSV) ⇒ permitir.
Si no, 403.

Variables relevantes:
- ADMIN_ENFORCE=1
- ADMIN_SUBS="uuid-1,uuid-2"

Ejemplo (payload recortado):
```
{
  "iss": "http://localhost:8080",
  "sub": "<user-uuid>",
  "custom": {
    "urn:hellojohn:sys": {
      "is_admin": true,
      "roles": ["sys:admin"]
    }
  }
}
```

---

## 10. Flujos principales
### 10.1 Registro + Login (con MFA opcional)
```mermaid
sequenceDiagram
autonumber
participant U as Usuario
participant S as Servicio
U->>S: POST /v1/auth/register
S-->>U: 200/201/204
U->>S: POST /v1/auth/login (email,pwd)
alt MFA requerida y no trusted
  S-->>U: {mfa_required:true, mfa_token}
  U->>S: POST /v1/mfa/totp/challenge
  S-->>U: access + refresh
else Sin MFA o trusted device
  S-->>U: access + refresh
end
```

### 10.2 OAuth2 Authorization Code (+ PKCE) con autoconsent
```mermaid
sequenceDiagram
participant B as Browser
participant S as Servicio
participant A as App Backend
B->>S: GET /oauth2/authorize (code_challenge)
S-->>B: (si sin sesión) pedir /v1/session/login
B->>S: POST /v1/session/login (cookie)
B->>S: GET /oauth2/authorize (cookie válida)
S-->>B: 302 redirect_uri?code=CODE&state (si scopes ⊆ CONSENT_AUTO_SCOPES)
A->>S: POST /oauth2/token (code + code_verifier)
S-->>A: access + id + refresh
```

### 10.3 Refresh rotativo
```mermaid
sequenceDiagram
participant C as Cliente
participant S as Servicio
C->>S: POST /v1/auth/refresh (R1)
S->>S: valida hash(R1), no revoked
S->>S: emite R2, revoca R1
S-->>C: access + R2
```

### 10.4 Password Reset / Verificación Email / Social Google
Se mantienen los flujos estándar documentados en los endpoints (ver sección 8).

---

## 11. Seguridad
- Argon2id (passwords) con parámetros seguros.
- Blacklist opcional de contraseñas.
- PKCE S256.
- Refresh rotation.
- Ed25519 + JWKS.
- AES‑GCM en reposo para secretos TOTP y claves privadas.
- Headers de seguridad (CSP, no‑store, X‑Frame‑Options DENY, HSTS si HTTPS, etc.).
- Introspección y logout‑all.

---

## 12. Rate limiting
Dos capas:
1) Global (IP+path)
2) Pools específicos: login, forgot/reset, verify-email, MFA (enroll/verify/challenge/disable)
Redis almacena contadores con TTL de la ventana. Fallo de Redis ⇒ fail‑open (se registra en logs).

---

## 13. Migraciones, seed y claves
- Migrar DB: `go run ./cmd/migrate`
- Seed inicial: `go run ./cmd/seed`
- Servidor: `go run ./cmd/service -env`
- Rotar claves: `go run ./cmd/keys -rotate`
- Listar claves: `go run ./cmd/keys -list`
- Retirar claves: `go run ./cmd/keys -retire -retire-after=168h`

El servicio puede ejecutar migraciones al arrancar si `FLAGS_MIGRATE=true`.

---

## 14. Pruebas E2E
La suite `test/e2e` cubre registro/login, refresh, email flows, OAuth2, social (login_code), MFA, introspección, blacklist y administración (clients, scopes, consents, users disable/enable).

---

## 15. Operación y salud
- Health: `GET /readyz` verifica DB, cache y keystore.
- Logs estructurados con request id.
- Timeouts y graceful shutdown configurables por ENV (`HTTP_*`).

---

## 16. Roadmap
- CLI y GUI de administración.
- Endpoints adicionales de administración (listado de usuarios, vistas/sesiones).
- Observabilidad (métricas, trazas).
- Extensión de providers y WebAuthn.

---

## 17. Troubleshooting
- 401 en `/v1/admin/*`: asegurate de enviar Bearer JWT válido y definir `ADMIN_ENFORCE=1` solo cuando el token tenga claims admin. `ADMIN_SUBS` puede servir de emergencia.
- 401 en `/oauth2/introspect`: definí `INTROSPECT_BASIC_USER/PASS` para habilitar basic auth del endpoint.
- 500 al iniciar: `SIGNING_MASTER_KEY` faltante o corto (mínimo 32 bytes).
- Emails no salen: revisá `SMTP_*` y `EMAIL_DEBUG_LINKS` (en prod se fuerza false).

---

© 2025 HelloJohn – Documentación actualizada y alineada al código.
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

### 11.1 Nuevos casos
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
## 15. Changelog (resumen)
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
