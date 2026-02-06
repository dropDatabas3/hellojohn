# CLAUDE.md - HelloJohn Development Guide

> **Documento de Referencia para Claude AI**
> Version: 2.0 (Post-V1 Cleanup)
> Fecha: 2026-01-28

---

## 1. VISION DEL PRODUCTO

### Que es HelloJohn?

**HelloJohn** es una plataforma **self-hosted, multi-tenant** de autenticacion e identidad para desarrolladores. Es una alternativa open-source a Auth0/Keycloak/Clerk optimizada para:

- **Developers** que no quieren reinventar auth
- **Software factories** gestionando multiples clientes
- **Empresas** buscando centralizar identidad sin vendor lock-in
- **Proyectos con IA** que necesitan auth simple de integrar (AI-friendly)

### Diferenciadores Clave

| Feature | HelloJohn | Auth0/Clerk/Keycloak |
|---------|-----------|----------------------|
| **Self-hosted** | Nativo | Cloud/Limited |
| **Multi-tenant** | DB aislada por tenant | Limitado |
| **Multi-DB drivers** | Postgres/MySQL/Mongo por tenant | No |
| **Control Plane** | FileSystem (sin DB requerida) | DB obligatoria |
| **Costo** | Gratis | $$$$ |
| **AI-Friendly** | SDKs minimos, integracion simple | Complejo |

---

## 2. ARQUITECTURA ACTUAL

### 2.1 Stack Tecnologico

**Backend:**
- Go 1.21+
- net/http (sin frameworks)
- pgx/v5 (PostgreSQL)
- embed.FS (migraciones)
- EdDSA (JWT signing)

**Frontend (Admin UI):**
- Next.js 16 (App Router)
- React 19
- Tailwind CSS 4.1
- Radix UI components
- TanStack React Query
- i18next

**SDKs:**
- JavaScript/TypeScript (browser)
- React (hooks + components)
- Node.js (server-side)
- Go (planned improvements)

### 2.2 Estructura de Directorios

```
hellojohn/
├── cmd/
│   └── service/main.go              # Entry point principal
│
├── internal/
│   ├── app/app.go                   # Wiring principal (Services -> Controllers -> Router)
│   │
│   ├── http/                        # Capa HTTP
│   │   ├── controllers/             # HTTP handlers por dominio
│   │   │   ├── controllers.go       # Aggregator principal
│   │   │   ├── admin/               # Admin operations
│   │   │   ├── auth/                # Login, register, refresh
│   │   │   ├── oauth/               # OAuth2 flows
│   │   │   ├── oidc/                # OIDC discovery, userinfo
│   │   │   ├── social/              # Social login
│   │   │   ├── session/             # Session management
│   │   │   ├── email/               # Email flows
│   │   │   ├── security/            # CSRF, security
│   │   │   └── health/              # Health checks
│   │   │
│   │   ├── services/                # Business logic por dominio
│   │   │   ├── services.go          # Aggregator principal
│   │   │   ├── admin/
│   │   │   ├── auth/
│   │   │   ├── oauth/
│   │   │   ├── oidc/
│   │   │   ├── social/
│   │   │   ├── session/
│   │   │   ├── email/
│   │   │   └── health/
│   │   │
│   │   ├── dto/                     # Data Transfer Objects
│   │   │   ├── admin/
│   │   │   ├── auth/
│   │   │   ├── oauth/
│   │   │   └── common/
│   │   │
│   │   ├── router/                  # Route registration
│   │   │   ├── router.go            # RegisterV2Routes()
│   │   │   ├── admin_routes.go
│   │   │   ├── auth_routes.go
│   │   │   ├── oauth_routes.go
│   │   │   ├── oidc_routes.go
│   │   │   └── ...
│   │   │
│   │   ├── middlewares/             # HTTP middlewares
│   │   │   ├── auth.go              # JWT validation
│   │   │   ├── tenant.go            # Tenant resolution
│   │   │   ├── cors.go              # CORS
│   │   │   ├── rate.go              # Rate limiting
│   │   │   └── ...
│   │   │
│   │   ├── errors/                  # HTTP error handling
│   │   ├── helpers/                 # HTTP utilities
│   │   └── server/wiring.go         # BuildV2Handler()
│   │
│   ├── store/                       # Data Access Layer (DAL)
│   │   ├── manager.go               # Manager (caching wrapper)
│   │   ├── factory.go               # Factory (multi-adapter)
│   │   ├── mode.go                  # 4 operational modes
│   │   ├── adapters/
│   │   │   ├── fs/                  # FileSystem (Control Plane)
│   │   │   ├── pg/                  # PostgreSQL
│   │   │   ├── mysql/               # MySQL (planned)
│   │   │   └── mongo/               # MongoDB (planned)
│   │   └── ...
│   │
│   ├── controlplane/                # Control Plane Service
│   │   ├── service.go
│   │   ├── tenants.go
│   │   ├── clients.go
│   │   └── scopes.go
│   │
│   ├── email/                       # Email Service
│   │   ├── service.go
│   │   └── smtp.go
│   │
│   ├── jwt/                         # JWT Issuer
│   │   ├── issuer.go
│   │   ├── keystore.go
│   │   └── jwks_cache.go
│   │
│   ├── domain/repository/           # Repository interfaces
│   ├── cache/                       # Cache abstraction
│   ├── security/                    # Password, TOTP, tokens
│   └── ...
│
├── data/hellojohn/                  # Control Plane FileSystem
│   ├── tenants/{slug}/
│   │   ├── tenant.yaml
│   │   ├── clients.yaml
│   │   └── scopes.yaml
│   └── keys/
│
├── migrations/postgres/tenant/      # SQL migrations
│
├── ui/                              # Admin Dashboard (Next.js)
│   ├── app/
│   │   ├── (admin)/admin/           # Admin pages
│   │   └── (auth)/                  # Auth pages
│   ├── components/
│   └── lib/
│
├── sdks/                            # Client SDKs
│   ├── js/                          # Browser SDK
│   ├── react/                       # React SDK
│   ├── node/                        # Node.js SDK
│   └── go/                          # Go SDK
│
└── docs/
```

### 2.3 Flujo de Dependencias (Cascada)

```
cmd/service/main.go
    │
    ▼
internal/http/server/wiring.go (BuildV2Handler)
    │
    ├── Init Store Manager (DAL)
    ├── Init Control Plane Service
    ├── Init Email Service
    ├── Init JWT Issuer (PersistentKeystore)
    │
    ▼
internal/app/app.go (App.New)
    │
    ├── CAPA 1: Services Aggregator
    │   └── services.New(deps) -> Services{}
    │
    ├── CAPA 2: Controllers Aggregator
    │   └── controllers.New(svcs) -> Controllers{}
    │
    ├── CAPA 3: Router Registration
    │   └── router.RegisterV2Routes(deps)
    │
    ▼
http.Server{Handler: app.Handler}
```

---

## 3. PATRONES DE CODIGO

### 3.1 Service Pattern

```go
// services/{domain}/contracts.go - Interfaces
type LoginService interface {
    LoginPassword(ctx context.Context, req dto.LoginRequest) (*dto.LoginResult, error)
}

// services/{domain}/login_service.go - Implementation
type loginService struct {
    deps LoginDeps
}

type LoginDeps struct {
    DAL        store.DataAccessLayer
    Issuer     *jwtx.Issuer
    RefreshTTL time.Duration
}

func NewLoginService(deps LoginDeps) LoginService {
    return &loginService{deps: deps}
}

func (s *loginService) LoginPassword(ctx context.Context, in dto.LoginRequest) (*dto.LoginResult, error) {
    // 1. Get tenant access
    tda, err := s.deps.DAL.ForTenant(ctx, in.TenantID)
    if err != nil {
        return nil, err
    }

    // 2. Verify DB exists
    if err := tda.RequireDB(); err != nil {
        return nil, err
    }

    // 3. Business logic
    user, identity, err := tda.Users().GetByEmail(ctx, tda.ID(), in.Email)
    // ... validate password, create tokens ...

    return &dto.LoginResult{...}, nil
}
```

### 3.2 Controller Pattern

```go
// controllers/{domain}/login_controller.go
type LoginController struct {
    service svc.LoginService
}

func NewLoginController(service svc.LoginService) *LoginController {
    return &LoginController{service: service}
}

func (c *LoginController) Login(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 1. Parse request
    var req dto.LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httperrors.WriteError(w, httperrors.ErrInvalidJSON)
        return
    }

    // 2. Call service
    result, err := c.service.LoginPassword(ctx, req)
    if err != nil {
        c.writeError(w, err)
        return
    }

    // 3. Write response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result)
}
```

### 3.3 DAL Usage Pattern

```go
// Get tenant data access
tda, err := s.deps.DAL.ForTenant(ctx, tenantID)
if err != nil {
    return err
}

// Control Plane (siempre disponible - FileSystem)
client, err := tda.Clients().Get(ctx, tda.ID(), clientID)
scopes, err := tda.Scopes().List(ctx, tda.ID())

// Data Plane (requiere DB)
if err := tda.RequireDB(); err != nil {
    return fmt.Errorf("tenant has no database")
}

user, identity, err := tda.Users().GetByEmail(ctx, tda.ID(), email)
token, err := tda.Tokens().Create(ctx, ...)
```

### 3.4 Error Handling

```go
// Service errors (domain-specific)
var (
    ErrInvalidCredentials = fmt.Errorf("invalid credentials")
    ErrUserDisabled       = fmt.Errorf("user disabled")
)

// Controller maps to HTTP
switch {
case errors.Is(err, svc.ErrInvalidCredentials):
    httperrors.WriteError(w, httperrors.ErrUnauthorized)
case errors.Is(err, svc.ErrUserDisabled):
    httperrors.WriteError(w, httperrors.ErrForbidden)
default:
    httperrors.WriteError(w, httperrors.ErrInternalServer)
}
```

---

## 4. CONVENCIONES

### 4.1 Naming

| Elemento | Patron | Ejemplo |
|----------|--------|---------|
| Service Interface | `{Name}Service` | `LoginService` |
| Service Impl | `{name}Service` (private) | `loginService` |
| Controller | `{Name}Controller` | `LoginController` |
| DTO Request | `{Action}Request` | `LoginRequest` |
| DTO Response | `{Action}Result` | `LoginResult` |
| Router File | `{domain}_routes.go` | `auth_routes.go` |
| Aggregator | `services.go`, `controllers.go` | Per domain |
| Deps Struct | `{Name}Deps` | `LoginDeps` |

### 4.2 Middleware Order

```go
chain := []mw.Middleware{
    mw.WithRecover(),           // 1. Catch panics
    mw.WithRequestID(),         // 2. Tracing
    mw.WithSecurityHeaders(),   // 3. CORS, CSP
    mw.WithNoStore(),           // 4. Cache control
    mw.WithRateLimit(...),      // 5. DDoS protection
    mw.RequireAuth(issuer),     // 6. JWT validation
    mw.RequireScope("scope"),   // 7. Permissions
    mw.WithLogging(),           // 8. Logging (last)
}
```

### 4.3 File Organization

- **NO** crear archivos nuevos si se puede editar uno existente
- **NO** crear documentacion (.md) a menos que se pida
- Helpers en `/helpers`, controllers en `/controllers/{domain}`
- Un controller por archivo: `{action}_controller.go`
- Un service por archivo: `{action}_service.go`

### 4.4 Imports (Post-V1 Cleanup)

```go
// DAL
import store "github.com/dropDatabas3/hellojohn/internal/store"

// Control Plane
import cp "github.com/dropDatabas3/hellojohn/internal/controlplane"

// Email
import emailv2 "github.com/dropDatabas3/hellojohn/internal/email"

// JWT
import jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"

// Cache
import "github.com/dropDatabas3/hellojohn/internal/cache"

// Repository Interfaces
import "github.com/dropDatabas3/hellojohn/internal/domain/repository"

// HTTP Errors
import httperrors "github.com/dropDatabas3/hellojohn/internal/http/errors"

// Middlewares
import mw "github.com/dropDatabas3/hellojohn/internal/http/middlewares"

// Services
import "github.com/dropDatabas3/hellojohn/internal/http/services"
import authsvc "github.com/dropDatabas3/hellojohn/internal/http/services/auth"

// Controllers
import "github.com/dropDatabas3/hellojohn/internal/http/controllers"
import authctrl "github.com/dropDatabas3/hellojohn/internal/http/controllers/auth"
```

---

## 5. CONFIGURACION

### 5.1 Variables de Entorno Criticas

| Variable | Descripcion | Requerida |
|----------|-------------|-----------|
| `SIGNING_MASTER_KEY` | JWT signing key (hex, 64 chars) | Si |
| `SECRETBOX_MASTER_KEY` | Encryption key (base64, 32 bytes) | Si |
| `FS_ROOT` | Control plane root | No (default: `data`) |
| `V2_BASE_URL` | Base URL for issuer | No |
| `V2_SERVER_ADDR` | HTTP bind address | No (`:8080`) |
| `CORS_ALLOWED_ORIGINS` | CORS origins (CSV) | No |
| `REGISTER_AUTO_LOGIN` | Auto-login after registration | No (`true`) |
| `FS_ADMIN_ENABLE` | Allow FS-admin registration | No (`false`) |

### 5.2 Estructura tenant.yaml

```yaml
id: "uuid"
slug: "acme"
name: "ACME Corp"
language: "en"

settings:
  issuer_mode: "path"  # path | subdomain

  user_db:
    driver: "postgres"
    dsn_enc: "encrypted_dsn"

  smtp:
    host: "smtp.sendgrid.net"
    port: 587
    from: "noreply@acme.com"
    password_enc: "encrypted"

  cache:
    driver: "redis"
    host: "localhost"
    port: 6379
```

---

## 6. ENDPOINTS PRINCIPALES

### Auth
- `POST /v2/auth/login` - Login con password
- `POST /v2/auth/register` - Registro
- `POST /v2/auth/refresh` - Refresh token

### OAuth2
- `GET /oauth2/authorize` - Authorization endpoint
- `POST /oauth2/token` - Token exchange
- `POST /oauth2/revoke` - Revoke token

### OIDC
- `GET /.well-known/openid-configuration` - Discovery
- `GET /.well-known/jwks.json` - JWKS
- `GET /userinfo` - User info

### Admin
- `GET/POST /v2/admin/tenants` - Tenant management
- `GET/POST /v2/admin/clients` - Client management
- `GET/POST /v2/admin/users` - User management

### Social
- `GET /v2/auth/social/{provider}/start` - Start social login
- `POST /v2/auth/social/exchange` - Exchange code for tokens

---

## 7. SCHEMA REFERENCE

### RBAC Tables (IMPORTANTE)

**CRITICAL: Estas son las tablas reales en tenant databases. NO usar nombres antiguos.**

#### `rbac_role` - Definición de Roles
```sql
CREATE TABLE rbac_role (
  id UUID DEFAULT gen_random_uuid(),
  name TEXT PRIMARY KEY,
  description TEXT,
  permissions TEXT[] NOT NULL DEFAULT '{}',  -- Array de permisos
  inherits_from TEXT,
  system BOOLEAN NOT NULL DEFAULT false,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**Campos clave:**
- `permissions`: Array de strings con permisos (e.g., `['users:read', 'users:write']`)
- `system`: Roles de sistema no se pueden eliminar
- NO hay tabla `role_permission` separada - los permisos están en la columna array

#### `rbac_user_role` - Asignación Usuario-Rol
```sql
CREATE TABLE rbac_user_role (
  user_id UUID NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
  role_name TEXT NOT NULL REFERENCES rbac_role(name) ON DELETE CASCADE,
  assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, role_name)
);
```

**IMPORTANTE - Convenciones de Queries:**
- **NO usar `tenant_id` en queries RBAC** - El aislamiento viene por pool de conexión (cada tenant = DB separada)
- **Permisos via UNNEST:** `SELECT perm FROM rbac_role CROSS JOIN UNNEST(permissions) AS perm`
- **Agregar permiso:** `UPDATE rbac_role SET permissions = array_append(permissions, $1) WHERE ...`
- **Eliminar permiso:** `UPDATE rbac_role SET permissions = array_remove(permissions, $1) WHERE ...`

#### Ejemplo de Query Correcto
```go
// ✅ CORRECTO - Sin tenant_id, usando UNNEST
const query = `
    SELECT DISTINCT perm
    FROM rbac_user_role ur
    JOIN rbac_role r ON r.name = ur.role_name
    CROSS JOIN UNNEST(r.permissions) AS perm
    WHERE ur.user_id = $1
`

// ❌ INCORRECTO - NO usar estas tablas/columnas
FROM role WHERE tenant_id = $1                  -- tabla 'role' no existe
FROM role_permission WHERE tenant_id = $1       -- tabla 'role_permission' no existe
```

---

## 8. DATA ACCESS LAYER (DAL)

### 4 Modos Operacionales

| Modo | Control Plane | Data Plane | Multi-nodo | Uso |
|------|---------------|------------|------------|-----|
| **FS Only** | FileSystem | None | Requiere Raft | Dev/Test |
| **FS + Tenant DB** | FileSystem | DB por tenant | Requiere Raft | SaaS |
| **FS + Global DB** | FS + Global DB | None | Sin Raft | Config backup |
| **Full DB** | FS + Global | DB por tenant | Sin Raft | Enterprise |

### Interfaces Principales

```go
type DataAccessLayer interface {
    ForTenant(ctx, slugOrID) (TenantDataAccess, error)
    ConfigAccess() ConfigAccess
    Mode() OperationalMode
    Close() error
}

type TenantDataAccess interface {
    Slug() string
    ID() string
    Settings() *repository.TenantSettings

    // Control Plane (siempre disponible)
    Clients() repository.ClientRepository
    Scopes() repository.ScopeRepository

    // Data Plane (requiere DB)
    Users() repository.UserRepository
    Tokens() repository.TokenRepository
    MFA() repository.MFARepository
    Consents() repository.ConsentRepository
    RBAC() repository.RBACRepository

    HasDB() bool
    RequireDB() error
}
```

---

## 8. SDKs (Estado Actual)

### JavaScript SDK (`sdks/js/`)

```typescript
import { createAuthClient } from '@hellojohn/js';

const auth = createAuthClient({
    domain: 'https://auth.example.com',
    clientID: 'my-app',
    redirectURI: 'https://app.example.com/callback',
    tenantID: 'acme',
});

// OAuth2 PKCE flow
await auth.loginWithRedirect();
await auth.handleRedirectCallback();

// Direct login
const user = await auth.loginWithCredentials(email, password);

// Social login
await auth.loginWithSocialProvider('google');
```

### React SDK (`sdks/react/`)

```tsx
import { HelloJohnProvider, useAuth, SignIn, SignUp } from '@hellojohn/react';

<HelloJohnProvider
    domain="https://auth.example.com"
    clientID="my-app"
    redirectURI="https://app.example.com/callback"
>
    <App />
</HelloJohnProvider>

// Hooks
const { isAuthenticated, user, login, logout } = useAuth();

// Components
<SignIn />
<SignUp />
```

**NOTA:** SDKs actualmente usan rutas `/v1/auth/*` - pendiente actualizar a `/v2/auth/*`.

---

## 9. ROADMAP Y TAREAS PENDIENTES

### 9.1 Analisis de Competencia (GAP Analysis)

Comparar con Clerk, Auth0, Firebase Auth, Keycloak:

| Feature | Clerk | Auth0 | HelloJohn | Status |
|---------|-------|-------|-----------|--------|
| Social Login | 6+ providers | 30+ | 1 (Google) | **PENDING** |
| MFA/TOTP | Full | Full | Partial | **PENDING** |
| RBAC | Built-in | Extension | Partial | **PENDING** |
| Custom Claims | Yes | Rules | Planned | **PENDING** |
| User Management UI | Yes | Yes | Partial | **IN PROGRESS** |
| Webhooks | Yes | Yes | No | **PENDING** |
| Session Management | Yes | Yes | Partial | **PENDING** |
| Import/Export | Yes | Yes | No | **PENDING** |
| SDK Coverage | 10+ langs | 15+ langs | 4 | **IN PROGRESS** |
| Biometrics/WebAuthn | Yes | Yes | No | **FUTURE** |

### 9.2 Prioridades Inmediatas

1. **Completar UI Admin** - 100% funcional, minimalista, estilo Google/Meta
2. **Manejo de Clients** - Flujo completo pub/priv clients con interaccion clara
3. **Dynamic Claims** - Sistema de claims customizables en tokens
4. **MFA/TOTP** - Completar implementacion
5. **RBAC** - Roles, permisos, scopes completos
6. **Consents** - Flujo de consentimiento OAuth

### 9.3 Tareas a Mediano Plazo

- **Social Providers** - Facebook, Microsoft, GitHub, Apple (5-6 principales)
- **Export/Import Tenants** - JSON download/upload via GUI
- **CLI Tool** - Administracion via terminal paralela al servicio
- **SDKs Actualizados** - Migrar de `/v1/` a `/v2/`, agregar Flutter, Angular, C#, Python

### 9.4 Tareas Futuras

- **Biometrics** - WebAuthn/FIDO2 support
- **Webhooks** - Event notifications
- **Audit Logs** - Compliance ready

---

## 10. GUIA DE DESARROLLO

### 10.1 Agregar Nuevo Endpoint

1. **DTO**: `internal/http/dto/{domain}/{action}.go`
2. **Service Interface**: `internal/http/services/{domain}/contracts.go`
3. **Service Impl**: `internal/http/services/{domain}/{action}_service.go`
4. **Aggregator**: Agregar a `services/{domain}/services.go`
5. **Controller**: `internal/http/controllers/{domain}/{action}_controller.go`
6. **Aggregator**: Agregar a `controllers/{domain}/controllers.go`
7. **Router**: Agregar ruta en `router/{domain}_routes.go`

### 10.2 Agregar Nuevo Dominio

1. Crear directorio en `services/{domain}/`
2. Crear directorio en `controllers/{domain}/`
3. Crear directorio en `dto/{domain}/`
4. Agregar al aggregator principal (`services/services.go`)
5. Agregar al aggregator principal (`controllers/controllers.go`)
6. Crear `router/{domain}_routes.go`
7. Registrar en `router/router.go`

### 10.3 Modificar UI

- Pages en `ui/app/(admin)/admin/`
- Components en `ui/components/`
- API calls en `ui/lib/api.ts`
- Usar TanStack Query para fetching

---

## 11. PRINCIPIOS DE DESARROLLO

### 11.1 Codigo

- **Simple > Clever**: Codigo legible y mantenible
- **No over-engineering**: Solo lo necesario
- **Separacion clara**: Controllers NO tienen logica de negocio
- **Services puros**: Sin dependencias HTTP (no w, r)
- **DAL para datos**: Todo acceso via `tda.Users()`, etc.

### 11.2 Seguridad

- Nunca exponer secretos en logs
- Usar `secretbox` para cifrar datos sensibles
- Validar inputs siempre
- Rate limiting en endpoints publicos
- CSRF en forms
- CORS estricto

### 11.3 AI-Friendly

- SDKs con minimo codigo para integracion
- Documentacion clara y completa
- Errores descriptivos
- Ejemplos en cada SDK
- Configuracion simple (pocos parametros)

---

## 12. COMANDOS UTILES

```bash
# Build
go build -o hellojohn ./cmd/service

# Run
FS_ROOT=./data/hellojohn \
SIGNING_MASTER_KEY=your-64-char-hex-key \
SECRETBOX_MASTER_KEY=your-base64-key \
./hellojohn

# UI Dev
cd ui && npm run dev

# Go mod tidy (limpiar dependencias)
go mod tidy
```

---

## 13. NOTAS IMPORTANTES PARA CLAUDE

### Contexto Actual (Post-V1 Cleanup)

- V1 fue eliminado completamente (2026-01-28)
- Codigo V2 movido de `internal/http/v2/` a `internal/http/`
- Imports actualizados (sin `/v2/` en paths internos)
- Rutas HTTP mantienen prefijo `/v2/` para versionado API
- SDKs apuntan a `/v1/` - **necesitan migracion a `/v2/`**

### Cuando Desarrolles

1. **Lee antes de escribir** - Siempre leer archivos existentes primero
2. **Usa DAL** - No SQL directo, siempre via repositorios
3. **Sigue patrones** - Copiar estructura de endpoints existentes
4. **No crear archivos innecesarios** - Editar existentes cuando sea posible
5. **Tests** - Agregar tests para logica critica
6. **UI consistente** - Seguir patrones de componentes existentes

### Errores Comunes a Evitar

- Crear handlers monoliticos (separar service/controller)
- Logica de negocio en controllers
- Acceso directo a DB sin DAL
- Olvidar `RequireDB()` antes de usar Data Plane
- No manejar errores correctamente
- Crear archivos .md sin que se pida
- Usar imports con `/v2/` que ya no existen

---

## 14. CLI TOOL (FUTURO)

### Objetivo

Desarrollar una CLI que funcione en paralelo al servicio HTTP, permitiendo:

- Administracion via comandos
- Conexion via SSH a contenedores
- PATH de entorno configurable
- Gestion de tenants, clients, users desde terminal

### Arquitectura Propuesta

```
hellojohn
├── serve              # Inicia servidor HTTP
├── admin
│   ├── tenant list
│   ├── tenant create <slug>
│   ├── client list <tenant>
│   └── user list <tenant>
├── migrate
│   └── run <tenant>
├── keys
│   └── rotate <tenant>
└── config
    └── show
```

### Consideraciones

- Usar `cobra` o similar para parsing de comandos
- Socket/gRPC para comunicacion con servicio corriendo
- Autenticacion admin via token o clave

---

## 15. EXPORT/IMPORT TENANTS (FUTURO)

### Objetivo

Permitir exportar configuracion de tenant a JSON e importar en otro entorno.

### Contenido del Export

```json
{
  "version": "1.0",
  "exported_at": "2026-01-28T10:00:00Z",
  "tenant": {
    "slug": "acme",
    "name": "ACME Corp",
    "settings": {...}
  },
  "clients": [...],
  "scopes": [...],
  "custom_fields": [...],
  "claims_config": {...}
}
```

### Flujo

1. Admin selecciona tenant en UI
2. Click "Export" -> descarga `.json`
3. En otro entorno, click "Import"
4. Sube archivo, preview de cambios
5. Confirma, tenant creado/actualizado

---

**FIN DEL DOCUMENTO**
