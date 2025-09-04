<div align="center">

# HelloJohn – Universal Login Service

Autenticación y emisión de tokens (JWT / OAuth2 / OIDC / flujos de email / social Google) para entornos multi‑tenant.

</div>

## Índice rápido
1. Objetivo y alcance
2. Arquitectura (mapa de carpetas + módulos)
3. Comandos CLI y flags
4. Endpoints (catálogo resumido)
5. Flujos principales (diagramas)
6. Ciclo de vida de tokens y claves
7. Configuración & variables de entorno
8. Rate limiting semántico
9. Seguridad y buenas prácticas
10. Tests (scripts E2E)
11. Glosario de términos clave
12. Roadmap / TODO visibles

---
## 1. Objetivo y alcance
Servicio autónomo que centraliza:
* Registro y login email/password (Argon2id).
* Sesiones via cookie para flujos OIDC browser.
* OAuth2 / OpenID Connect Authorization Code + PKCE S256.
* Emisión de Access / ID / Refresh tokens firmados EdDSA (Ed25519) con rotación de claves.
* Verificación de email y restablecimiento de contraseña (enlaces single‑use).
* Social login (Google) con state firmado y provisión automática de identidad.
* Rate limiting global y específico (login / forgot / email flows) usando Redis.
* Hooks opcionales de claims (extensión futura: CEL / webhooks).

Estado: funcional para los flujos listados; algunos archivos marcados TODO (p.ej. `oauth_start.go`, `oauth_callback.go`, `registry_clients.go`) indican futuras ampliaciones.

---
## 2. Arquitectura
Organización de carpetas relevantes (solo lo esencial):

| Carpeta | Rol | Notas |
|---------|-----|-------|
| `cmd/service` | Entrada principal HTTP | Lee config, migra opcional, arma middlewares y rutas. |
| `cmd/migrate` | Runner simple de migraciones up/down | Ordena y aplica archivos SQL. |
| `cmd/seed` | Creación de tenant, usuario admin, client y versión | Usa variables `SEED_*`. |
| `cmd/keys` | Rotación / listado / retiro de signing keys | Puede cifrar privada con `SIGNING_MASTER_KEY`. |
| `internal/http` | Middleware, routing y handlers | CORS, security headers, rate limit, logging, recover. |
| `internal/http/handlers` | Endpoints REST & OAuth/OIDC | Password, refresh, authorize, token, email flows, social. |
| `internal/jwt` | Emisión y keystore persistente | Ed25519, JWKS, cache local, descifrado GCMV1. |
| `internal/store/pg` | Implementación Postgres | Usuarios, identidades, refresh, signing keys, clients. |
| `internal/security/password` | Hash y política | Argon2id + validación configurable. |
| `internal/security/token` | Tokens opacos | generate + sha256 base64url. |
| `internal/rate` | Limiter Redis & multi‑limiter | Fixed window con TTL y adaptador dinámico. |
| `internal/cache/{memory,redis}` | Cache abstracta | Códigos auth, sesiones, tokens sociales. |
| `internal/email` | SMTP + plantillas | Render HTML/TXT y diagnóstico errores. |
| `templates` | Plantillas de email | verify / reset. |
| `migrations/postgres` | SQL schema & evoluciones | Incluye signing_keys y refresh chain. |
| `test` | Scripts E2E PowerShell / BAT | Validan flows completos. |

Diagrama alto nivel:
```mermaid
flowchart LR
Client[Browser / SPA / Backend] -->|HTTP / OAuth2| API[HelloJohn HTTP]
API --> AuthH[Handlers /v1/* /oauth2/*]
API --> MW[Middlewares]
AuthH --> Issuer[JWT Issuer]
Issuer --> Keystore[(Signing Keys)]
AuthH --> Store[(Postgres)]
AuthH --> Cache[(Redis / Memory)]
AuthH --> SMTP[SMTP]
AuthH --> Rate[Redis Limiter]
Keystore --> Store
```

---
## 3. Comandos CLI
| Comando | Uso principal | Flags / Ejemplos |
|---------|---------------|------------------|
| `cmd/service` | Levantar API | `-config` `-env` `-env-file` `-print-config` `-migrate` |
| `cmd/migrate` | Ejecutar migraciones SQL (up/down) | `go run ./cmd/migrate -dir migrations/postgres up` / `down 1` |
| `cmd/seed` | Seed inicial (tenant, admin, client) | Variables `SEED_TENANT_NAME`, `SEED_ADMIN_EMAIL`, etc. |
| `cmd/keys` | Rotar / listar / retirar claves | `-list`, `-rotate`, `-retire -retire-after=168h` |

Ejemplos rápidos:
```bash
# Servicio con migraciones y .env
FLAGS_MIGRATE=true go run ./cmd/service -env-file .env.dev

# Solo listar claves
go run ./cmd/keys -list

# Rotar y cifrar privada (exportá SIGNING_MASTER_KEY antes)
SIGNING_MASTER_KEY=... go run ./cmd/keys -rotate
```

---
## 4. Catálogo de Endpoints
Formato unificado para facilitar lectura. Columnas:
* Auth: No | B (Bearer) | C (Cookie de sesión) | Token (link de email) | Refresh (usa refresh token)
* RL (Rate Limit): Global (middleware), Login, Forgot/Verify/Reset (semánticos) o Cacheable (respuestas públicas cacheables).

### 4.1 Autenticación clásica (JSON)
| Método | Path | Descripción | Auth | RL |
|--------|------|-------------|------|----|
| POST | /v1/auth/register | Crea usuario (opción auto-login) | No | Global |
| POST | /v1/auth/login | Email + password → access + refresh | No | Login |
| POST | /v1/auth/refresh | Usa refresh válido y rota a uno nuevo | Refresh | Global |
| POST | /v1/auth/logout | Revoca refresh actual (idempotente) | Refresh | Global |
| GET  | /v1/me | Devuelve claims básicos del access token | B | Global |

### 4.2 Sesiones navegador (para flujo /oauth2/authorize)
| Método | Path | Descripción | Auth | RL |
|--------|------|-------------|------|----|
| POST | /v1/session/login | Crea cookie sid (login interactivo) | No | Global |
| POST | /v1/session/logout | Borra cookie y sesión en cache | C | Global |

### 4.3 OAuth2 / OIDC
| Método | Path | Descripción | Auth | RL |
|--------|------|-------------|------|----|
| GET | /oauth2/authorize | Authorization Code + PKCE S256 | C (o B si permitido) | Global |
| POST | /oauth2/token | Intercambio code o refresh → tokens | No / Refresh | Global |
| POST | /oauth2/revoke | Revoca refresh (RFC7009, idempotente) | No | Global |
| GET / POST | /userinfo | Devuelve claims OIDC (scope email) | B | Global |
| GET | /.well-known/openid-configuration | Documento Discovery OIDC | No | Cacheable |
| GET | /.well-known/jwks.json | Claves públicas (active + retiring) | No | Cacheable |

### 4.4 Flujos de Email (verify / reset)
| Método | Path | Descripción | Auth | RL |
|--------|------|-------------|------|----|
| POST | /v1/auth/verify-email/start | Envía link para verificar email | B | Verify |
| GET  | /v1/auth/verify-email | Consume token y marca verificado | Token | - |
| POST | /v1/auth/forgot | Envia email con link de reset | No (respuesta neutra) | Forgot |
| POST | /v1/auth/reset | Aplica nuevo password (auto-login opc.) | Token | Reset |

### 4.5 Social (Google) – si habilitado
| Método | Path | Descripción | Auth | RL |
|--------|------|-------------|------|----|
| GET | /v1/auth/social/google/start | Redirige a Google (state firmado) | No | Global |
| GET | /v1/auth/social/google/callback | Intercambia code → tokens / login_code | No | Global |
Nota: si se pasa `redirect_uri`, se devuelve 302 con `code=<login_code>` (uso único en cache). Endpoint de intercambio de `login_code` aún pendiente.

### 4.6 Salud
| Método | Path | Descripción |
|--------|------|-------------|
| GET | /healthz | Liveness simple |
| GET | /readyz | DB + firma/verificación + Redis opcional |

### 4.7 Errores
Todas las respuestas de error JSON siguen:
```json
{ "error": "<codigo_corto>", "error_description": "Mensaje legible" }
```
Los códigos internos detallados no se exponen para evitar filtración de lógica.

---
## 5. Flujos (diagramas)
### Authorization Code + PKCE
```mermaid
sequenceDiagram
autonumber
participant B as Browser
participant S as HelloJohn
participant C as Client App
B->>C: Inicia login
C->>S: GET /oauth2/authorize (code_challenge S256)
S-->>B: 302 redirect (si cookie no válida → requerir sesión)
B->>S: (sesión ya creada) repite authorize
S-->>C: 302 redirect_uri?code=CODE&state
C->>S: POST /oauth2/token (code_verifier)
S-->>C: access + id + refresh
C->>S: GET /userinfo (Bearer)
S-->>C: sub + (email si scope)
```

### Refresh rotativo
```mermaid
sequenceDiagram
participant C as Client
participant S as HelloJohn
C->>S: refresh R1
S->>S: valida hash(R1) & no revoked
S->>S: crea R2(rotated_from=R1), revoca R1
S-->>C: nuevo access + R2
```

### Reset password
```mermaid
sequenceDiagram
participant U as User
participant S as HelloJohn
U->>S: POST /v1/auth/forgot
S-->>U: (email con link token T)
U->>S: POST /v1/auth/reset (T,new pwd)
S->>S: invalida tokens previos
S-->>U: 204 (ó tokens si auto-login)
```

### Verificación email
```mermaid
sequenceDiagram
U->>S: POST verify-email/start
S-->>U: link
U->>S: GET verify-email?token
S-->>U: 200/302 status=verified
```

### Social Google
```mermaid
sequenceDiagram
Browser->>S: /social/google/start (tenant_id, client_id, redirect_uri?)
S-->>Browser: 302 Google (state JWT)
Browser->>Google: Auth + consent
Google-->>Browser: 302 callback?state&code
Browser->>S: callback
S->>S: Exchange + verify id_token + upsert user/identity
alt redirect_uri provisto
  S-->>Browser: 302 redirect_uri?code=login_code
else
  S-->>Browser: JSON tokens
end
```

---
## 6. Ciclo de vida de tokens y claves
| Tipo | Formato | Almacenamiento | Rotación | Revocación |
|------|---------|----------------|----------|------------|
| Access | JWT EdDSA | No persistido | TTL (≈15m) | Al expirar / invalidar refresh chain |
| ID | JWT EdDSA | No | Igual Access | Igual |
| Refresh | Opaque (hash sha256) | Tabla `refresh_token` | Cada uso (rotated_from) | Flag `revoked_at` |
| Email verify | Opaque (hash) | Tabla específica (migraciones) | N/A | Uso único / TTL |
| Password reset | Igual verify | Igual | N/A | Uso único / TTL |
| Social login_code | Cache (Redis/memory) | Clave `social:code:<id>` | 1 uso | Auto-expira |

Firmado JWT:
* Algoritmo: EdDSA (Ed25519) – header: `{alg:EdDSA, kid:<KID>, typ:JWT}`.
* Claves en tabla `signing_keys` con estados: `active` → `retiring` → `retired`.
* Rotación: `cmd/keys -rotate` inserta nueva `active` y mueve anterior a `retiring`.
* Retiro: `cmd/keys -retire -retire-after=168h` marca `retiring` antiguas como `retired` (dejando margen para validaciones en clientes).
* Cifrado opcional de la clave privada: `SIGNING_MASTER_KEY` (hex 64) → AES‑GCM con header `GCMV1`.

Claims destacados:
* `tid` (tenant), `amr` (Authentication Methods References: pwd|refresh|google|reset), `scp`/`scope`.
* ID Token agrega `at_hash`, `azp` (authorized party), `nonce` (si se envió en authorize).

---
## 7. Configuración (guía clara)
La configuración se puede definir en YAML (`configs/config.yaml`) y luego cada campo puede ser sobre‑escrito por variables de entorno. Flags CLI (p.ej. `-env`, `-env-file`, `-migrate`) controlan cómo se cargan.

Orden de precedencia (menor → mayor): `defaults en código` → `config.yaml` → `variables de entorno` → `flags`.

Formato de duraciones: usa notación Go (`15m`, `1h`, `48h`, `30s`).

### 7.1 Variables mínimas para arrancar en desarrollo
Configura sólo lo imprescindible (usa cache en memoria y PostgreSQL local):
```bash
export STORAGE_DSN=postgres://user:pass@localhost:5432/login?sslmode=disable
export JWT_ISSUER=http://localhost:8080
export FLAGS_MIGRATE=true
go run ./cmd/service -env
```

### 7.2 Categorías y explicación simple

#### Servidor
| Variable | Qué hace | Ejemplo / Default |
|----------|---------|--------------------|
| SERVER_ADDR | Dirección y puerto de escucha | :8080 |
| SERVER_CORS_ALLOWED_ORIGINS | Lista orígenes (CSV) habilitados | http://localhost:5173 |

#### JWT / Tokens
| Variable | Qué hace | Default |
|----------|---------|---------|
| JWT_ISSUER | URL base que aparecerá como `iss` | (vacío) |
| JWT_ACCESS_TTL | Vida access & id tokens | 15m aprox (si no se ajustó) |
| JWT_REFRESH_TTL | Vida de refresh token | 720h (30 días) |

#### Almacenamiento
| Variable | Qué hace | Ejemplo |
|----------|---------|---------|
| STORAGE_DRIVER | postgres | postgres |
| STORAGE_DSN | Cadena conexión principal | postgres://user:pass@host:5432/db?sslmode=disable |
| POSTGRES_MAX_OPEN_CONNS | Conexiones máximas (pool) | 10 / 20 |
| POSTGRES_MAX_IDLE_CONNS | Conexiones inactivas pool | 5 |
| POSTGRES_CONN_MAX_LIFETIME | Reciclado conexiones (duración) | 1h |

#### Cache
| Variable | Qué hace | Default |
|----------|---------|---------|
| CACHE_KIND | memory o redis | memory |
| REDIS_ADDR | host:puerto Redis | localhost:6379 |
| REDIS_DB | Número DB Redis | 0 |
| REDIS_PREFIX | Prefijo claves | (vacío) |
| MEMORY_DEFAULT_TTL | TTL por defecto (cache memoria) | 2m |

#### Registro & Auth general
| Variable | Qué hace | Default |
|----------|---------|---------|
| REGISTER_AUTO_LOGIN | Tras registrarse emite tokens | false |
| AUTH_ALLOW_BEARER_SESSION | Permite usar bearer en /authorize (dev) | true |

#### Sesión (cookie del navegador)
| Variable | Qué hace | Default |
|----------|---------|---------|
| AUTH_SESSION_COOKIE_NAME | Nombre cookie | sid |
| AUTH_SESSION_DOMAIN | Dominio (opcional) | (vacío) |
| AUTH_SESSION_SAMESITE | Lax / Strict / None | Lax |
| AUTH_SESSION_SECURE | Requiere HTTPS si true | false (poner true en prod) |
| AUTH_SESSION_TTL | Duración sesión | 12h |

#### Flujos de email
| Variable | Qué hace | Default |
|----------|---------|---------|
| AUTH_VERIFY_TTL | Vigencia token verificación email | 48h |
| AUTH_RESET_TTL | Vigencia token reset password | 60m |
| AUTH_RESET_AUTO_LOGIN | Auto-login tras reset | false |
| EMAIL_BASE_URL | Base para construir links email | (usa issuer si vacío) |
| EMAIL_TEMPLATES_DIR | Carpeta plantillas | ./templates |
| EMAIL_DEBUG_LINKS | Exponer links en headers (no prod) | true en dev / forzado false en prod |

#### SMTP
| Variable | Qué hace | Ejemplo |
|----------|---------|---------|
| SMTP_HOST | Servidor SMTP | smtp.gmail.com |
| SMTP_PORT | Puerto | 587 |
| SMTP_USERNAME | Usuario | you@example.com |
| SMTP_PASSWORD | Password / App password | **** |
| SMTP_FROM | Dirección remitente | you@example.com |
| SMTP_TLS | auto | auto / starttls / ssl / none |
| SMTP_INSECURE_SKIP_VERIFY | Saltar verificación TLS (solo dev) | false |

#### Rate limiting
| Variable | Qué hace | Default |
|----------|---------|---------|
| RATE_ENABLED | Activa capa global | false |
| RATE_WINDOW | Ventana global | 1m |
| RATE_MAX_REQUESTS | Peticiones permitidas ventana global | 60 |
| RATE_LOGIN_LIMIT | Intentos login ventana login | 10 |
| RATE_LOGIN_WINDOW | Ventana login | 1m |
| RATE_FORGOT_LIMIT | Intentos forgot | 5 |
| RATE_FORGOT_WINDOW | Ventana forgot | 10m |

#### Política de password
| Variable | Qué hace | Default |
|----------|---------|---------|
| SECURITY_PASSWORD_POLICY_MIN_LENGTH | Longitud mínima | 10 |
| SECURITY_PASSWORD_POLICY_REQUIRE_UPPER | Requiere mayúscula | false |
| SECURITY_PASSWORD_POLICY_REQUIRE_LOWER | Requiere minúscula | false |
| SECURITY_PASSWORD_POLICY_REQUIRE_DIGIT | Requiere dígito | false |
| SECURITY_PASSWORD_POLICY_REQUIRE_SYMBOL | Requiere símbolo | false |

#### Social Google
| Variable | Qué hace | Ejemplo / Default |
|----------|---------|--------------------|
| GOOGLE_ENABLED | Habilita flujo Google | false |
| GOOGLE_CLIENT_ID | Client ID OAuth Google | (vacío) |
| GOOGLE_CLIENT_SECRET | Client Secret | (vacío) |
| GOOGLE_REDIRECT_URL | Callback (auto si vacío) | <issuer>/v1/auth/social/google/callback |
| GOOGLE_SCOPES | Scopes CSV | openid,email,profile |
| GOOGLE_ALLOWED_TENANTS | Limitar a tenants (CSV) | (vacío = todos) |
| GOOGLE_ALLOWED_CLIENTS | Limitar a client_ids | (vacío = todos) |
| SOCIAL_LOGIN_CODE_TTL | TTL código efímero login_code | 60s |

#### Claves de firma
| Variable | Qué hace | Notas |
|----------|---------|-------|
| SIGNING_MASTER_KEY | Hex 64 chars (32 bytes) para cifrar privada | Si no se define la clave privada queda en claro en DB |

### 7.3 Tips de producción
* Forzá `AUTH_SESSION_SECURE=true` y serví detrás de HTTPS.
* Usa Redis para consistencia de rate limit y sesiones multi‑instancia.
* Programa rotación de claves (`cmd/keys -rotate`) y retiro (`-retire`).
* Mantén `EMAIL_DEBUG_LINKS=false` (ya se fuerza en prod) para no filtrar enlaces sensibles.
* Ajusta política de password si tu modelo de riesgo lo exige.

### 7.4 Depuración rápida
Si algo “no arranca”, revisa:
1. Cadena DSN postgres correcta.
2. `JWT_ISSUER` accesible (usa http://host:puerto en dev).
3. Migraciones corriendo (`FLAGS_MIGRATE=true`).
4. Redis (si configurado) responde en `REDIS_ADDR`.
5. Tiempos (duraciones) sin typos (`1m30s`, no `1min30`).

---
## 8. Rate limiting semántico
Capas:
1. Global (middleware): clave = IP|path|client_id → cabeceras `X-RateLimit-*` + `Retry-After`.
2. Específico login: clave `login:<tenant>:<email>` con ventana y límite propios (evita enumeración).
3. Email flows (verify / forgot / reset): usa adaptador con prefijo `mailflows:` para aislar métricas.
Fail‑open: ante error Redis se permite la petición (log nivel warn).

---
## 9. Seguridad
* Argon2id con parámetros fuertes (64MiB memoria, t=3).
* Política de password configurable (mínimo, upper, lower, digit, symbol).
* PKCE S256 obligatorio → evita interceptación del code.
* Cabeceras defensivas: CSP estricta (`default-src 'none'`), X-Frame-Options DENY, Referrer-Policy no-referrer, HSTS (solo si HTTPS detectado), Permissions-Policy mínima.
* Respuestas con tokens: `Cache-Control: no-store` y `Pragma: no-cache`.
* Refresh rotativo + hashing evita reutilización masiva y exfiltración útil.
* Claves privadas opcionalmente cifradas (AES-GCM) reposando en DB.
* `readyz` firma y valida un JWT efímero para asegurar keystore operativo.
* `EMAIL_DEBUG_LINKS` forzado a false en `APP_ENV=prod` (evita filtración de enlaces de verificación / reset).

---
## 10. Tests (scripts E2E)
Ver `test/test.md` para guía extensa. Resumen:
| Script | Propósito |
|--------|-----------|
| `test.ps1` / `test.bat` | Suite integral (auth, OIDC, email, rotación refresh, errores). |
| `test_emailflows_e2e.ps1` | Verificación + reset password end‑to‑end. |
| `test_cors_cookie.ps1` | Preflight + flags cookie sesión. |
| `test_rate_emailflows.ps1` | Límite sobre forgot/reset. |
| `test_no_debug_headers.ps1` | Garantiza ausencia de headers debug en prod. |
| `test_social_google.ps1` | Flujo Google (si habilitado). |

Ejemplo ejecución (PowerShell):
```powershell
pwsh -File .\test\test.ps1 -AutoMail
```

---
## 11. Glosario
| Término | Definición breve |
|---------|------------------|
| OIDC Discovery | Documento JSON estándar con endpoints y capacidades (`/.well-known/openid-configuration`). |
| JWKS | Set de claves públicas actuales (active + retiring) para verificar firmas JWT. |
| PKCE S256 | Mecanismo que agrega `code_challenge` / `code_verifier` asegurando el Authorization Code contra interceptación. |
| AMR | Authentication Methods References: lista de métodos usados (pwd, refresh, google, reset). |
| `at_hash` | Mitad izquierda de SHA-256(access_token) base64url; permite al cliente validar que el access recibido corresponde a su ID Token. |
| Key Rotation | Proceso de introducir una nueva clave `active`, moviendo la anterior a `retiring` y luego `retired`. |
| AES-GCM (GCMV1) | Modo autenticado de cifrado simétrico usado para proteger claves privadas en reposo. Prefijo `GCMV1` identifica blob cifrado. |
| Refresh Rotation | Patrón donde cada uso de refresh invalida el anterior, limitando replay. |
| Nonce | Valor aleatorio que el cliente envía en authorize y recibe en ID Token para mitigar replay. |
| `azp` | Authorized Party: identifica el cliente final destinatario del ID Token. |
| Login Code Social | Código efímero (cache) devuelto tras social callback para intercambio futuro (endpoint pendiente). |

---
## 12. Roadmap / TODO visibles
* Endpoint canje `login_code` social.
* Administración dinámica de scopes y consent screen.
* MFA (TOTP / WebAuthn) + AMR enriquecido.
* Auditoría detallada (tabla events/audit).
* Introspección / revocación centralizada (OAuth2 introspection endpoint).
* Support MySQL & Mongo implementaciones completas (placeholders presentes).
* Hook de claims CEL / webhooks (estructura ya prevista en `Container.ClaimsHook`).
* Métricas Prometheus / OpenTelemetry tracing.
* UI de administración multi‑tenant.

---
## Inicio rápido mínimo
```bash
cp configs/config.example.yaml configs/config.yaml
export STORAGE_DSN=postgres://user:pass@localhost:5432/login?sslmode=disable
export JWT_ISSUER=http://localhost:8080
export FLAGS_MIGRATE=true
go run ./cmd/service -env
```

---
© 2025 HelloJohn – Documento vivo.
