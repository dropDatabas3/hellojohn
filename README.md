# HelloJohn

Plataforma de autenticación y autorización (en desarrollo) que unifica login clásico (usuario/contraseña), emisión de JWT / ID Tokens OIDC, sesiones por cookie para flujos OAuth2 Authorization Code + PKCE y flujos de verificación / recuperación por email. El objetivo es ofrecer un "Universal Login" extensible multi‑tenant con soporte para múltiples proveedores (password / social / enterprise) y drivers de base de datos intercambiables.

> Estado: MVP temprano. Implementado: registro + login password, refresh / revoke, sesiones, OAuth2/OIDC básico (authorize, token, revoke, discovery, userinfo), emisión de JWT (Ed25519), verificación de email y reset de contraseña, rate limiting, política de contraseñas, caché (mem/redis) y migraciones Postgres. Pendiente: callback OAuth social, rotación y persistencia de claves, multi‑tenant avanzado, panel admin, MFA, flujos enterprise, métricas, auditoría avanzada.

---
## Tabla de Contenido
1. Visión General
2. Arquitectura Lógica
3. Modelo de Datos (Postgres actual)
4. Endpoints y Flujos Principales
5. Diagramas de Secuencia (Mermaid)
6. Configuración y Variables de Entorno
7. Ciclo de Vida de Tokens (Access / ID / Refresh / Email)
8. Seguridad
9. Ejecución y Flags
10. Roadmap Breve
11. Contribuir

---
## 1. Visión General
HelloJohn provee un núcleo de autenticación multi‑tenant donde:
- Usuarios se registran (email + password) y gestionan sesión.
- Aplicaciones cliente (public/confidential) obtienen códigos y tokens vía Authorization Code + PKCE.
- Se emiten Access Tokens (para APIs) e ID Tokens (para confianza del cliente) firmados EdDSA (Ed25519) y publicados vía JWKS.
- Refresh tokens opacos con rotación y revocación.
- Verificación de email y restablecimiento de contraseña: enlaces temporales enviados por SMTP.
- Extensible: drivers de storage (actual Postgres; placeholders MySQL / Mongo), caché (memory/redis), hooks de claims.

## 2. Arquitectura Lógica
Componentes principales:
- HTTP Layer: `net/http` con middlewares (CORS, Security Headers, Logging, Rate Limit, Recover, RequestID).
- Handlers: /v1/auth/*, /oauth2/*, /userinfo, JWKS, email flows.
- Store: abstracciones + implementación Postgres (usuarios, identidades, clientes, refresh, tokens email).
- Cache: memoria (go-cache) o Redis (para sesiones, rate, lookups).
- JWT: emisión y rotación (clave actual en memoria - TODO persistir/rotar multi‑KID).
- Seguridad: Argon2id para password, políticas configurables, headers estrictos (CSP, HSTS condicional, Frame / MIME / Permissions Policy).
- Email: SMTP + plantillas (HTML/TXT); en entornos no prod se exponen links debug vía headers opcionales.
- Rate Limiting: ventana fija Redis configurable.

Flujos soportados: Login/registro, refresh, revoke, userinfo, authorize/token, verificación email, reset password.

## 3. Modelo de Datos (Postgres)
Tablas clave (resumen):
- tenant: organización / espacio lógico (multi‑tenant inicial).
- client / client_version: aplicaciones y versiones de configuración de claims/crypto (versionado inicial).
- app_user: usuarios (por tenant) con email y metadata JSON.
- identity: credenciales / enlaces a proveedores (password, google, facebook).
- refresh_token: tokens de refresco opacos (solo hash, rotación, revocación).
- email_verification_token / password_reset_token: flujos temporales verificación y reset.

Índices cuidan unicidad por tenant/email, búsqueda por user/provider, eficiencia en expiraciones y lookups activos.

### Diagrama Entidad-Relación (simplificado)
```mermaid
erDiagram
  TENANT ||--o{ CLIENT : has
  CLIENT ||--o{ CLIENT_VERSION : versions
  TENANT ||--o{ APP_USER : has
  APP_USER ||--o{ IDENTITY : has
  APP_USER ||--o{ REFRESH_TOKEN : issues
  CLIENT ||--o{ REFRESH_TOKEN : for
  APP_USER ||--o{ EMAIL_VERIFICATION_TOKEN : verifies
  APP_USER ||--o{ PASSWORD_RESET_TOKEN : resets
```

## 4. Endpoints y Flujos Principales
Base: `http://host:port`

Salud:
- GET `/healthz` (liveness)
- GET `/readyz` (dependencias OK)

JWKS / Discovery:
- GET `/.well-known/jwks.json`
- GET `/.well-known/openid-configuration`

OAuth2 / OIDC:
- GET `/oauth2/authorize` (Authorization Code + PKCE S256, sesión por cookie)
- POST `/oauth2/token` (code -> tokens; refresh -> tokens)
- POST `/oauth2/revoke` (RFC7009 refresh revocation)
- GET `/userinfo` (OIDC user claims, bearer access token)

Autenticación clásica (JSON):
- POST `/v1/auth/register` (email, password) -> crea usuario (+ auto-login opcional)
- POST `/v1/auth/login` (email, password) -> access + refresh
- POST `/v1/auth/refresh` (refresh token) -> rotate + nuevos tokens
- POST `/v1/auth/logout` (revoca refresh asociado; cleanup sesión)
- GET  `/v1/me` (datos usuario autenticado)

Sesiones (para navegador / authorize):
- POST `/v1/session/login` (crea cookie sid)
- POST `/v1/session/logout` (borra cookie)

Email Flows:
- POST `/v1/auth/verify-email/start` (genera token + email)
- GET  `/v1/auth/verify-email?token=...` (consume token)
- POST `/v1/auth/forgot` (genera token reset + email)
- POST `/v1/auth/reset` (consume reset con nuevo password)

## 5. Diagramas de Secuencia (Mermaid)
### 5.1 Authorization Code + PKCE
```mermaid
sequenceDiagram
  participant U as User Browser
  participant C as Client App
  participant S as HelloJohn
  U->>C: Navega /login
  C->>S: GET /oauth2/authorize?client_id&code_challenge&redirect_uri
  S-->>U: Form (si sin sesión) / consentimiento (futuro)
  U->>S: Credenciales (o cookie existente)
  S->>S: Valida PKCE, client, redir
  S-->>C: 302 redirect_uri?code=XYZ&state
  C->>S: POST /oauth2/token (code + code_verifier)
  S->>S: Valida code, PKCE, expira code
  S-->>C: access_token + id_token + refresh_token
  C->>S: (Opcional) /userinfo con access_token
```

### 5.2 Refresh Token Rotación
```mermaid
sequenceDiagram
  participant C as Client
  participant S as HelloJohn
  C->>S: POST /v1/auth/refresh (refresh_token R1)
  S->>S: Valida hash(R1), verifica no revocado
  S->>S: Crea R2 (registra rotated_from=R1), marca R1 revoked
  S-->>C: access_token nuevo + refresh_token R2
```

### 5.3 Reset Password
```mermaid
sequenceDiagram
  participant U as User
  participant S as HelloJohn
  U->>S: POST /v1/auth/forgot (email)
  S->>S: Genera token T (hash guarda)
  S-->>U: Email con link /v1/auth/reset?token=T
  U->>S: POST /v1/auth/reset (token T + new password)
  S->>S: Valida hash(T), marca usado, actualiza password
  S-->>U: OK (auto_login si config)
```

### 5.4 Verificación de Email
```mermaid
sequenceDiagram
  participant U as User
  participant S as HelloJohn
  U->>S: POST /v1/auth/verify-email/start
  S->>S: Crea token EV, guarda hash
  S-->>U: Email con link /v1/auth/verify-email?token=EV
  U->>S: GET /v1/auth/verify-email?token=EV
  S->>S: Valida, marca email_verified
  S-->>U: Redirección / confirmación
```

## 6. Configuración y Variables de Entorno
Se carga YAML (`configs/config.example.yaml`) y luego overrides por entorno. Campos principales:

App:
- `APP_ENV` (dev|staging|prod) controla headers debug email.

Servidor:
- `SERVER_ADDR` (default :8080)
- `SERVER_CORS_ALLOWED_ORIGINS` (CSV)

Storage:
- `STORAGE_DRIVER` (postgres|mysql|mongo) actual implementado: postgres
- `STORAGE_DSN` (si se usa directo) Ej: `postgres://user:pass@host:5432/db?sslmode=disable`
- Postgres pool: `POSTGRES_MAX_OPEN_CONNS`, `POSTGRES_MAX_IDLE_CONNS`, `POSTGRES_CONN_MAX_LIFETIME`
- MySQL: `MYSQL_DSN` (placeholder), Mongo: `MONGO_URI`, `MONGO_DATABASE`

Cache:
- `CACHE_KIND` (memory|redis)
- Redis: `REDIS_ADDR`, `REDIS_DB`, `REDIS_PREFIX`
- Memory: `MEMORY_DEFAULT_TTL`

JWT:
- `JWT_ISSUER` (URL base)
- `JWT_ACCESS_TTL` (duración Access/ID) default 15m
- `JWT_REFRESH_TTL` (default 720h = 30d)

Registro / Auth:
- `REGISTER_AUTO_LOGIN` (bool)
- `AUTH_ALLOW_BEARER_SESSION` (true en dev para flexibilidad)
- Sesión cookie: `AUTH_SESSION_COOKIE_NAME`, `AUTH_SESSION_DOMAIN`, `AUTH_SESSION_SAMESITE`, `AUTH_SESSION_SECURE`, `AUTH_SESSION_TTL`
- Reset: `AUTH_RESET_TTL`, `AUTH_RESET_AUTO_LOGIN`
- Verify: `AUTH_VERIFY_TTL`

Rate Limit:
- `RATE_ENABLED`, `RATE_WINDOW`, `RATE_MAX_REQUESTS`

Flags:
- `FLAGS_MIGRATE` (ejecuta migraciones al iniciar)

SMTP:
- `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`, `SMTP_TLS` (auto|starttls|ssl|none), `SMTP_INSECURE_SKIP_VERIFY`

Email:
- `EMAIL_BASE_URL`, `EMAIL_TEMPLATES_DIR`, `EMAIL_DEBUG_LINKS` (forzado a false en prod)

Seguridad:
- Password policy: `SECURITY_PASSWORD_POLICY_MIN_LENGTH`, `SECURITY_PASSWORD_POLICY_REQUIRE_UPPER|LOWER|DIGIT|SYMBOL`

### Ejemplo rápido (.env)
```
APP_ENV=dev
SERVER_ADDR=:8080
STORAGE_DRIVER=postgres
STORAGE_DSN=postgres://user:pass@localhost:5432/login?sslmode=disable
FLAGS_MIGRATE=true
JWT_ISSUER=http://localhost:8080
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=720h
CACHE_KIND=redis
REDIS_ADDR=localhost:6379
EMAIL_BASE_URL=http://localhost:8080
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=you@example.com
SMTP_PASSWORD=app-password
SMTP_FROM=you@example.com
SMTP_TLS=starttls
EMAIL_DEBUG_LINKS=true
```

## 7. Ciclo de Vida de Tokens
- Access Token: JWT EdDSA (headers: kid, typ). TTL corto (15m). Scope / AMR / custom claims soportados.
- ID Token: Igual TTL que Access por ahora; estándar OIDC (iss, sub, aud, iat, exp) + extras.
- Refresh Token: Opaque; solo se guarda hash (sha256). Rotación en cada uso; chain via `rotated_from`.
- Email Verif / Reset: Tokens aleatorios base64url; se almacena sha256 (BYTEA). TTL configurable; un solo uso.
- Revocación: refresh revocado al rotar o logout; endpoints de revoke manejan estado.

## 8. Seguridad
- Hash de contraseñas: Argon2id (parámetros configurados en código; TODO exponer ajustes).
- Política: requerimientos mínimos y composición configurables.
- Cabeceras: CSP estricta (sin inline), HSTS solo si secure, X-Frame-Options DENY, X-Content-Type-Options nosniff, Permissions-Policy mínima.
- PKCE S256 obligatorio en `/oauth2/authorize`.
- En prod se fuerza `EMAIL_DEBUG_LINKS=false`.
- Sesiones: cookie HttpOnly, SameSite configurable, Secure opcional (recomendado true en prod), TTL configurable.
- Rotación de refresh y almacenamiento hashed evita replay persistente.
- TODO: Rotación/persistencia de claves (actualmente en memoria), MFA, device management, detección anomalías.

## 9. Ejecución y Flags
Compilación local:
```bash
go run ./cmd/service
```
O con migraciones forzadas:
```bash
go run ./cmd/service -migrate
```
Flags actuales:
- `-config=path` (si se implementa lectura alternativa; revisar main)
- `-migrate` (equivalente a `FLAGS_MIGRATE=true`)

Docker (ejemplo simplificado): ver `deployments/docker-compose.yml` y `Dockerfile`.

## 10. Roadmap Breve
- Persistencia/rotación de claves + JWKS multi-KID.
- OAuth social callbacks y linking identities.
- MFA (TOTP / WebAuthn).
- Admin UI + panel multi-tenant.
- Auditoría granular y logs estructurados centralizados.
- Metrics / tracing (OpenTelemetry).
- Gestión de scopes dinámica y consent screen.
- Introspección de tokens / revocation lists caché.
- Hardening de sesión (SameSite=strict + CSRF tokens en formularios).
- Pruebas unitarias y contract tests (ahora scripts E2E PowerShell).

## 11. Contribuir
1. Fork & branch.
2. Agrega tests (o scripts) representativos.
3. Sigue estilo Go estándar, evita dependencias innecesarias.
4. Documenta nuevas variables en este README y en `config.example.yaml`.
5. Abre PR describiendo cambios, migraciones y riesgos.

---
© 2025 HelloJohn (Proyecto en desarrollo).
