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
  H --> Cache[(Redis|Memory)]
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

Inicio rápido (dev):
```bash
cp configs/config.example.yaml configs/config.yaml
export STORAGE_DSN=postgres://user:password@localhost:5432/login?sslmode=disable
export JWT_ISSUER=http://localhost:8080
export FLAGS_MIGRATE=true
go run ./cmd/service -env
```

---
## 11. E2E Tests y validación
Suite ubicada en `test/e2e` levanta el servicio, corre migraciones, genera claves, hace seed y ejecuta casos: registro/login, refresh rotativo, email flows, OAuth2, social login_code, MFA, introspección, blacklist.  
Los tests garantizan: no-store en respuestas con credenciales, rotación de refresh, revocación correcta, uso único de login_code y recuperación MFA.

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

---
© 2025 HelloJohn – Documentación funcional completa.
