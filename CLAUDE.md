# CLAUDE.md — HelloJohn V2 Architecture Guide

> **Documento de Referencia Definitivo para Claude AI**
> Última actualización: 2026-01-20
> Arquitectura: V2 (Cascada)

---

## 📋 ÍNDICE

1. [Visión de Producto](#-visión-de-producto)
2. [Arquitectura V2 (Cascada)](#-arquitectura-v2-cascada)
3. [Estructura de Directorios](#-estructura-de-directorios)
4. [Flujo de Migración V1→V2](#-flujo-de-migración-v1v2)
5. [Guía de Implementación](#-guía-de-implementación)
6. [Data Access Layer (DAL)](#-data-access-layer-dal)
7. [Cluster/Raft (4 Modos)](#-clusterraft-4-modos)
8. [Referencias Rápidas](#-referencias-rápidas)

---

## 🎯 VISIÓN DE PRODUCTO

### ¿Qué es HelloJohn?

**HelloJohn** es una plataforma **self-hosted, multi-tenant** de autenticación e identidad para desarrolladores y consultoras de software. Alternativa open-source a Auth0/Keycloak.

### Usuarios Finales
- **Desarrolladores** que no quieren reinventar auth/login/register
- **Software factories** gestionando múltiples clientes
- **Empresas** buscando centralizar identidad sin vendor lock-in

### Casos de Uso
1. **OAuth2/OIDC Provider** (como Auth0/Keycloak)
2. **Identity Management** interno
3. **Multi-tenant SaaS Authentication** centralizada

### Diferenciadores Clave

| Feature | HelloJohn | Auth0/Keycloak |
|---------|-----------|----------------|
| **Hosting** | Self-hosted | Cloud/Self-hosted |
| **Multi-tenant** | Nativo (aislamiento DB por tenant) | Limitado |
| **Multi-DB drivers** | Sí (Postgres/MySQL/Mongo por tenant) | No |
| **Control Plane** | FileSystem (sin DB requerida) | DB obligatoria |
| **HA Multi-nodo** | Raft consensus | Requiere DB compartida |
| **Costo** | Gratis (infraestructura propia) | $$$$ |

### Arquitectura de Alto Nivel

```
┌─────────────────────────────────────────────────────────────┐
│                    DESARROLLADOR FINAL                      │
│  Integra via SDK (Go/JS/Python) + OAuth2/OIDC               │
└───────────────────────────┬─────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────┐
│                    HELLOJOHN CLUSTER                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐                   │
│  │  Node 1  │  │  Node 2  │  │  Node 3  │  (Raft)           │
│  │ (Leader) │◄─┤(Follower)│◄─┤(Follower)│                   │
│  └──────────┘  └──────────┘  └──────────┘                   │
└───────────────────────────┬─────────────────────────────────┘
                            │
        ┌───────────────────┴───────────────────┐
        ▼                                       ▼
┌──────────────────┐                  ┌───────────────────┐
│  CONTROL PLANE   │                  │   DATA PLANE      │
│  (FileSystem)    │                  │   (Multi-DB)      │
│  • Tenants       │                  │  • Tenant A: PG   │
│  • Clients       │                  │  • Tenant B: MySQL│
│  • Scopes        │                  │  • Tenant C: Mongo│
│  • Keys (JWKS)   │                  │  (Users/Tokens)   │
└──────────────────┘                  └───────────────────┘
```

---

## 🏗️ ARQUITECTURA V2 (CASCADA)

### Principio Fundamental

V2 elimina el monolito de handlers V1 mediante **Inyección en Cascada** de dependencias:

```
Infrastructure → Services → Controllers → Router → HTTP
```

### Flujo de Inicialización

```
cmd/service_v2/main.go
    │
    ├─► Carga ENV (SIGNING_MASTER_KEY, FS_ROOT, etc)
    │
    ▼
internal/http/v2/server/wiring.go (BuildV2Handler)
    │
    ├─► 1. Init Store Manager (DAL)
    ├─► 2. Init Control Plane Service
    ├─► 3. Init Email Service V2
    ├─► 4. Init JWT Issuer (PersistentKeystore)
    ├─► 5. Init Cache (Redis/Memory)
    │
    ▼
internal/app/v2/app.go (App.New)
    │
    ├─► CAPA 1: Services Aggregator
    │   └─► services.New(deps) → Services{}
    │       ├─► Auth: services/auth/services.go
    │       ├─► Admin: services/admin/services.go
    │       ├─► OIDC: services/oidc/services.go
    │       ├─► OAuth: services/oauth/services.go
    │       ├─► Social: services/social/services.go
    │       └─► ... (Email, Session, Security, Health)
    │
    ├─► CAPA 2: Controllers Aggregator
    │   └─► controllers.New(svcs) → Controllers{}
    │       ├─► Auth: controllers/auth/controllers.go
    │       ├─► Admin: controllers/admin/controllers.go
    │       └─► ...
    │
    ├─► CAPA 3: Router Registration
    │   └─► router.RegisterV2Routes(deps)
    │       ├─► RegisterAuthRoutes()
    │       ├─► RegisterAdminRoutes()
    │       ├─► RegisterOIDCRoutes()
    │       └─► ...
    │
    ▼
http.Server{Handler: app.Handler}
```

### Arquitectura por Capas

```
┌─────────────────────────────────────────────────────────────┐
│ CAPA 0: INFRAESTRUCTURA (Singleton, inicializado en main)   │
├─────────────────────────────────────────────────────────────┤
│ • store.Manager (DataAccessLayer)                           │
│ • controlplane.Service                                      │
│ • emailv2.Service                                           │
│ • jwtx.Issuer (PersistentKeystore)                          │
│ • cache.Client (Redis/Memory)                               │
└────────────────────────┬────────────────────────────────────┘
                         │ inyectados en ↓
┌─────────────────────────────────────────────────────────────┐
│ CAPA 1: SERVICES (Lógica de Negocio)                        │
├─────────────────────────────────────────────────────────────┤
│ services/services.go → Aggregator Principal                 │
│   ├─► Auth:     services/auth/services.go                   │
│   ├─► Admin:    services/admin/services.go                  │
│   ├─► OIDC:     services/oidc/services.go                   │
│   ├─► OAuth:    services/oauth/services.go                  │
│   ├─► Social:   services/social/services.go                 │
│   ├─► Email:    services/email/services.go                  │
│   ├─► Session:  services/session/services.go                │
│   ├─► Security: services/security/services.go               │
│   └─► Health:   services/health/services.go                 │
│                                                             │
│ Patrón:                                                     │
│   type Services struct {                                    │
│       Login    LoginService                                 │
│       Register RegisterService                              │
│   }                                                         │
│   func NewServices(deps Deps) Services { ... }              │
└────────────────────────┬────────────────────────────────────┘
                         │ inyectados en ↓
┌─────────────────────────────────────────────────────────────┐
│ CAPA 2: CONTROLLERS (HTTP Handling)                         │
├─────────────────────────────────────────────────────────────┤
│ controllers/controllers.go → Aggregator Principal           │
│   ├─► Auth:  controllers/auth/controllers.go                │
│   ├─► Admin: controllers/admin/controllers.go               │
│   └─► ...                                                   │
│                                                             │
│ Patrón:                                                     │
│   type Controllers struct {                                 │
│       Login    *LoginController                             │
│       Register *RegisterController                          │
│   }                                                         │
│   func NewControllers(svcs Services) *Controllers { ... }   │
└────────────────────────┬────────────────────────────────────┘
                         │ inyectados en ↓
┌─────────────────────────────────────────────────────────────┐
│ CAPA 3: ROUTER (Registro de Rutas + Middlewares)            │
├─────────────────────────────────────────────────────────────┤
│ router/router.go → RegisterV2Routes(deps)                   │
│   ├─► router/auth_routes.go                                 │
│   ├─► router/admin_routes.go                                │
│   ├─► router/oidc_routes.go                                 │
│   └─► ...                                                   │
│                                                             │
│ Patrón:                                                     │
│   func RegisterAuthRoutes(mux, deps) {                      │
│       mux.Handle("/v2/auth/login",                          │
│           withMiddlewares(deps.Controllers.Login.Login))    │
│   }                                                         │
└─────────────────────────────────────────────────────────────┘
```

### Ejemplo Concreto: Login Flow

```go
// 1. REQUEST
POST /v2/auth/login
Body: {"tenant_id":"acme","email":"user@example.com","password":"***"}

// 2. ROUTER (router/auth_routes.go:23)
mux.Handle("/v2/auth/login",
    authHandler(deps.RateLimiter,
        http.HandlerFunc(c.Login.Login)))

// 3. MIDDLEWARES
WithRecover() → WithRequestID() → WithSecurityHeaders() →
WithRateLimit() → WithLogging()

// 4. CONTROLLER (controllers/auth/login_controller.go:31)
func (c *LoginController) Login(w http.ResponseWriter, r *http.Request) {
    var req dto.LoginRequest
    json.NewDecoder(r.Body).Decode(&req)

    // Delega al SERVICE
    result, err := c.service.LoginPassword(ctx, req)
    if err != nil {
        writeLoginError(w, err)
        return
    }

    json.NewEncoder(w).Encode(result)
}

// 5. SERVICE (services/auth/login_service.go:53)
func (s *loginService) LoginPassword(ctx, in) (*dto.LoginResult, error) {
    // Lógica de negocio:
    tda, _ := s.deps.DAL.ForTenant(ctx, in.TenantID)
    user, _, err := tda.Users().GetByEmail(ctx, tda.ID(), in.Email)
    if !tda.Users().CheckPassword(identity.PasswordHash, in.Password) {
        return nil, ErrInvalidCredentials
    }

    // Emitir tokens
    accessToken := s.deps.Issuer.IssueAccess(...)
    refreshToken, _ := tda.Tokens().Create(...)

    return &dto.LoginResult{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
    }, nil
}

// 6. DAL (store/v2/manager.go + adapters)
tda.Users() → PG Adapter → SELECT FROM users WHERE email=...
tda.Tokens().Create() → PG Adapter → INSERT INTO refresh_tokens ...
```

---

## 📁 ESTRUCTURA DE DIRECTORIOS

### Árbol V2 Completo

```
hellojohn/
├── cmd/
│   ├── service_v2/main.go          ← Entry Point V2
│   ├── service/main.go             ← Entry Point V1 (legacy)
│   ├── migrate/                    ← Migraciones manuales
│   └── keys/                       ← Rotación de keys
│
├── internal/
│   ├── app/
│   │   ├── v1/app.go               ← App V1 (legacy)
│   │   └── v2/app.go               ← App V2 (Wiring Principal)
│   │
│   ├── http/
│   │   ├── v1/handlers/            ← Handlers monolíticos (48 archivos)
│   │   └── v2/                     ← Arquitectura V2
│   │       ├── controllers/        ← HTTP Handling
│   │       │   ├── controllers.go  ← Aggregator Principal
│   │       │   ├── auth/
│   │       │   │   ├── controllers.go
│   │       │   │   ├── login_controller.go
│   │       │   │   ├── register_controller.go
│   │       │   │   └── ...
│   │       │   ├── admin/
│   │       │   ├── oidc/
│   │       │   ├── oauth/
│   │       │   ├── social/
│   │       │   ├── email/
│   │       │   ├── session/
│   │       │   └── health/
│   │       │
│   │       ├── services/           ← Lógica de Negocio
│   │       │   ├── services.go     ← Aggregator Principal
│   │       │   ├── auth/
│   │       │   │   ├── services.go ← Auth Aggregator
│   │       │   │   ├── login_service.go
│   │       │   │   ├── register_service.go
│   │       │   │   ├── contracts.go (interfaces)
│   │       │   │   └── ...
│   │       │   ├── admin/
│   │       │   ├── oidc/
│   │       │   ├── oauth/
│   │       │   ├── social/
│   │       │   ├── email/
│   │       │   ├── session/
│   │       │   └── health/
│   │       │
│   │       ├── router/             ← Registro de Rutas
│   │       │   ├── router.go       ← RegisterV2Routes()
│   │       │   ├── auth_routes.go
│   │       │   ├── admin_routes.go
│   │       │   ├── oidc_routes.go
│   │       │   ├── oauth_routes.go
│   │       │   └── ...
│   │       │
│   │       ├── dto/                ← Data Transfer Objects
│   │       │   ├── auth/
│   │       │   │   ├── login.go
│   │       │   │   ├── register.go
│   │       │   │   └── ...
│   │       │   ├── admin/
│   │       │   ├── oauth/
│   │       │   └── common/
│   │       │
│   │       ├── middlewares/        ← HTTP Middlewares
│   │       │   ├── auth.go
│   │       │   ├── ratelimit.go
│   │       │   ├── logging.go
│   │       │   ├── recover.go
│   │       │   └── chain.go
│   │       │
│   │       ├── errors/             ← HTTP Error Handling
│   │       │   ├── errors.go       ← WriteError()
│   │       │   └── types.go        ← Error definitions
│   │       │
│   │       ├── helpers/            ← HTTP Utilities
│   │       └── server/
│   │           └── wiring.go       ← BuildV2Handler()
│   │
│   ├── store/v2/                   ← Data Access Layer
│   │   ├── manager.go              ← Manager (caching wrapper)
│   │   ├── factory.go              ← Factory (multi-adapter)
│   │   ├── mode.go                 ← Operational Modes
│   │   ├── cluster.go              ← ClusterHook (Raft)
│   │   ├── pool.go                 ← Connection Pool
│   │   ├── adapters/
│   │   │   ├── dal/                ← Auto-registro de adapters
│   │   │   ├── fs/                 ← FileSystem Adapter (Control Plane)
│   │   │   ├── pg/                 ← PostgreSQL Adapter (Data Plane)
│   │   │   └── noop/               ← NoOp Adapter (Testing)
│   │   └── README.md               ← DAL Documentation
│   │
│   ├── controlplane/v2/            ← Control Plane Service
│   │   ├── service.go              ← Service Interface
│   │   ├── tenants.go
│   │   ├── clients.go
│   │   ├── scopes.go
│   │   └── README.md
│   │
│   ├── email/v2/                   ← Email Service V2
│   │   ├── service.go
│   │   ├── sender.go
│   │   ├── templates.go
│   │   └── README.md
│   │
│   ├── jwt/                        ← JWT Issuer
│   │   ├── issuer.go
│   │   ├── keystore.go
│   │   ├── jwks_cache.go
│   │   └── README.md
│   │
│   ├── cache/v2/                   ← Cache Abstraction
│   │   ├── cache.go
│   │   ├── memory.go
│   │   └── redis/
│   │
│   ├── domain/repository/          ← Repository Interfaces
│   │   ├── user.go
│   │   ├── token.go
│   │   ├── client.go
│   │   ├── tenant.go
│   │   ├── errors.go
│   │   └── README.md
│   │
│   ├── observability/logger/       ← Structured Logging
│   └── security/                   ← Security Utilities
│
├── data/hellojohn/                 ← Control Plane FileSystem
│   ├── tenants/
│   │   ├── acme/
│   │   │   ├── tenant.yaml
│   │   │   ├── clients.yaml
│   │   │   └── scopes.yaml
│   │   └── local/
│   └── keys/
│       ├── active.json             ← Global signing key
│       └── acme/
│           └── active.json         ← Tenant signing key
│
├── migrations/                     ← DB Migrations
│   └── postgres/tenant/
│
└── docs/
    ├── refactor_docs/
    │   ├── V1_HANDLERS_INVENTORY.md
    │   ├── V1_ROUTES_MASTER_LIST.md
    │   └── V2_DAL_COVERAGE_REPORT.md
    └── v2-toolbox.md
```

### Convenciones de Nombres

| Tipo | Patrón | Ejemplo |
|------|--------|---------|
| **Service Interface** | `{Nombre}Service` | `LoginService`, `RegisterService` |
| **Service Impl** | `{nombre}Service` (struct privado) | `loginService`, `registerService` |
| **Controller** | `{Nombre}Controller` | `LoginController`, `AdminClientsController` |
| **DTO Request** | `{Accion}Request` | `LoginRequest`, `RegisterRequest` |
| **DTO Response** | `{Accion}Result` o `{Accion}Response` | `LoginResult`, `TokenResponse` |
| **Router File** | `{domain}_routes.go` | `auth_routes.go`, `admin_routes.go` |
| **Aggregator** | `services.go` o `controllers.go` | En cada subdirectorio |

---

## 🔄 FLUJO DE MIGRACIÓN V1→V2

### 📝 DOCUMENTACIÓN DE MIGRACIÓN (OBLIGATORIO)

**ANTES de migrar cualquier handler**, debes crear/actualizar el log de migración:

#### Archivo: `MIGRATION_LOG.md`

Si no existe, créalo en el root del proyecto con esta estructura:

```markdown
# Migration Log V1 → V2

## Handlers Migrados

### ✅ auth_login.go → v2/auth/login_service.go
- **Fecha**: 2026-01-20
- **Rutas migradas**:
  - `POST /v1/auth/login` → `POST /v2/auth/login`
- **Archivos creados**:
  - `internal/http/v2/dto/auth/login.go`
  - `internal/http/v2/services/auth/login_service.go`
  - `internal/http/v2/controllers/auth/login_controller.go`
- **Archivos editados**:
  - `internal/http/v2/services/auth/services.go` (agregado LoginService)
  - `internal/http/v2/controllers/auth/controllers.go` (agregado LoginController)
  - `internal/http/v2/router/auth_routes.go` (agregado /v2/auth/login)
- **Herramientas V2 usadas**:
  - `store.DataAccessLayer.ForTenant()`
  - `jwtx.Issuer.IssueAccess()`
  - `repository.UserRepository.GetByEmail()`
  - `repository.TokenRepository.Create()`
- **Dependencias**:
  - DAL (store.Manager)
  - Issuer (jwtx.Issuer)
  - RefreshTTL (time.Duration)
- **Descripción**:
  Login con password. Valida credenciales, verifica estado del usuario, emite access token (JWT) y refresh token (opaco en DB).
- **Notas**:
  - Agregado soporte para MFA (si está habilitado en tenant)
  - Errores mapeados: ErrInvalidCredentials, ErrUserDisabled
- **Wiring verificado**: ✅
  - `app/v2/app.go:78` (AuthControllers inyectado)
  - `router/router.go:94` (RegisterAuthRoutes llamado)

---

### ⏳ auth_register.go → v2/auth/register_service.go
- **Fecha**: [Pendiente]
- **Rutas**: `POST /v1/auth/register` → `POST /v2/auth/register`
- **Estado**: En progreso
- **Bloqueadores**: Email verification flow necesita testing

---

## Handlers Pendientes (de V1_HANDLERS_INVENTORY.md)

- [ ] admin_consents.go
- [ ] admin_rbac.go (users/roles)
- [ ] admin_rbac.go (roles/perms)
- [ ] admin_users.go (disable/enable)
- [ ] mfa_totp.go (enroll/verify/challenge)
- [ ] oauth_authorize.go
- [ ] oauth_token.go
- [ ] social_dynamic.go

---

## Estadísticas

- **Total handlers V1**: 48
- **Migrados a V2**: 12
- **En progreso**: 3
- **Pendientes**: 33
- **Progreso**: 25%

---

## Convenciones

### Formato de Entrada

Para cada handler migrado, documenta:

1. **Título**: `✅ {handler_v1}.go → v2/{domain}/{nombre}_service.go`
2. **Fecha**: Fecha de migración completa
3. **Rutas migradas**: Mapeo V1 → V2 (todas las rutas del handler)
4. **Archivos creados**: Lista completa de archivos nuevos
5. **Archivos editados**: Aggregators + router modificados
6. **Herramientas V2 usadas**: DAL, JWT, Email, Cache (métodos específicos)
7. **Dependencias**: Inyecciones del service
8. **Descripción**: Qué hace el handler en 1-2 líneas
9. **Notas**: Edge cases, mejoras vs V1, decisiones de diseño
10. **Wiring verificado**: Checkmarks + referencias a líneas de código

### Estados

- ✅ **Migrado**: Completado, testeado, wiring verificado
- ⏳ **En progreso**: Archivos creados pero sin testear
- ❌ **Bloqueado**: Dependencia faltante
- 📝 **Pendiente**: No iniciado
```

#### Proceso de Actualización

**Después de cada migración**:

1. Abre `MIGRATION_LOG.md`
2. Copia el template de entrada
3. Llena todos los campos basado en tu trabajo
4. Mueve el handler de "Pendientes" a "Migrados"
5. Actualiza estadísticas
6. Commit: `git add MIGRATION_LOG.md && git commit -m "docs: migrated {handler}"`

#### Beneficios

- **Trazabilidad**: Saber exactamente qué se migró y cuándo
- **Onboarding**: Nuevos desarrolladores ven el progreso
- **Debugging**: Rastrear qué archivos tocar si algo falla
- **Métricas**: Velocidad de migración, handlers críticos pendientes

---

### Proceso Oficial (Paso a Paso)

#### **PASO 1: Análisis del Handler V1**

```bash
# Localizar handler a migrar
internal/http/v1/handlers/{nombre}_handler.go

# Identificar:
# 1. Rutas manejadas (revisar routes.go + handler)
# 2. Dependencias (c.Store, c.Issuer, cpctx.Provider, etc)
# 3. Lógica de negocio
# 4. DTOs implícitos (request/response structs)
# 5. Errores retornados
```

**Ejemplo**: `auth_login.go`
```go
// V1 Handler (monolítico)
func (h *AuthLoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Parsing request
    var req struct {
        Email    string `json:"email"`
        Password string `json:"password"`
        TenantID string `json:"tenant_id"`
        ClientID string `json:"client_id"`
    }
    json.NewDecoder(r.Body).Decode(&req)

    // Lógica de negocio (mezclada)
    tenant, _ := cpctx.Provider.GetTenantBySlug(ctx, req.TenantID)
    user, _ := h.store.GetUserByEmail(ctx, req.Email)
    if !checkPassword(user.PasswordHash, req.Password) {
        http.Error(w, "invalid credentials", 401)
        return
    }

    // Emitir tokens (mezclado con HTTP)
    accessToken := h.issuer.IssueAccess(...)
    refreshToken := generateRefreshToken()
    h.store.CreateRefreshToken(...)

    // Response
    json.NewEncoder(w).Encode(map[string]string{
        "access_token": accessToken,
        "refresh_token": refreshToken,
    })
}
```

**Rutas identificadas** (revisar `routes.go` + handler):
- `POST /v1/auth/login` (routes.go:111)

#### **PASO 2: Crear DTOs**

```bash
# Ubicación
internal/http/v2/dto/auth/login.go
```

```go
package auth

// LoginRequest es el DTO de entrada
type LoginRequest struct {
    TenantID           string `json:"tenant_id"`
    ClientID           string `json:"client_id"`
    Email              string `json:"email"`
    Password           string `json:"password"`
    TrustedDeviceToken string `json:"trusted_device_token,omitempty"`
}

// LoginResult es el DTO de salida
type LoginResult struct {
    AccessToken       string `json:"access_token"`
    RefreshToken      string `json:"refresh_token"`
    ExpiresIn         int    `json:"expires_in"`
    TokenType         string `json:"token_type"`
    MFARequired       bool   `json:"mfa_required,omitempty"`
    MFAToken          string `json:"mfa_token,omitempty"`
}
```

**TODO**: Agregar validación con struct tags:
```go
type LoginRequest struct {
    TenantID string `json:"tenant_id" validate:"required,min=1"`
    ClientID string `json:"client_id" validate:"required,min=1"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}
```

#### **PASO 3: Crear Service Interface**

```bash
# Ubicación
internal/http/v2/services/auth/contracts.go
```

```go
package auth

import (
    "context"
    dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/auth"
)

// LoginService maneja la lógica de login
type LoginService interface {
    LoginPassword(ctx context.Context, req dto.LoginRequest) (*dto.LoginResult, error)
}
```

#### **PASO 4: Implementar Service**

```bash
# Ubicación
internal/http/v2/services/auth/login_service.go
```

```go
package auth

import (
    "context"
    "fmt"
    "strings"

    "github.com/dropDatabas3/hellojohn/internal/domain/repository"
    dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/auth"
    jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
    store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// LoginDeps son las dependencias del servicio
type LoginDeps struct {
    DAL        store.DataAccessLayer
    Issuer     *jwtx.Issuer
    RefreshTTL time.Duration
    ClaimsHook ClaimsHook
}

type loginService struct {
    deps LoginDeps
}

func NewLoginService(deps LoginDeps) LoginService {
    if deps.ClaimsHook == nil {
        deps.ClaimsHook = NoOpClaimsHook{}
    }
    return &loginService{deps: deps}
}

// Errores del servicio
var (
    ErrInvalidCredentials = fmt.Errorf("invalid credentials")
    ErrUserDisabled       = fmt.Errorf("user disabled")
    ErrEmailNotVerified   = fmt.Errorf("email not verified")
)

func (s *loginService) LoginPassword(ctx context.Context, in dto.LoginRequest) (*dto.LoginResult, error) {
    // 1. Normalización
    in.Email = strings.TrimSpace(strings.ToLower(in.Email))
    in.TenantID = strings.TrimSpace(in.TenantID)
    in.ClientID = strings.TrimSpace(in.ClientID)

    // 2. Validación básica
    if in.Email == "" || in.Password == "" {
        return nil, fmt.Errorf("missing required fields")
    }

    // 3. Resolver tenant via DAL
    tda, err := s.deps.DAL.ForTenant(ctx, in.TenantID)
    if err != nil {
        return nil, fmt.Errorf("invalid tenant")
    }

    // 4. Verificar client (Control Plane - siempre disponible)
    client, err := tda.Clients().Get(ctx, tda.ID(), in.ClientID)
    if err != nil {
        return nil, fmt.Errorf("invalid client")
    }

    // 5. Verificar que tenant tenga DB (Data Plane)
    if err := tda.RequireDB(); err != nil {
        return nil, fmt.Errorf("tenant has no database")
    }

    // 6. Buscar usuario
    user, identity, err := tda.Users().GetByEmail(ctx, tda.ID(), in.Email)
    if err != nil {
        if repository.IsNotFound(err) {
            return nil, ErrInvalidCredentials
        }
        return nil, err
    }

    // 7. Verificar password
    if !tda.Users().CheckPassword(identity.PasswordHash, in.Password) {
        return nil, ErrInvalidCredentials
    }

    // 8. Verificar estado del usuario
    if user.DisabledAt != nil {
        return nil, ErrUserDisabled
    }

    // 9. Crear tokens
    accessToken, err := s.deps.Issuer.IssueAccess(ctx, jwtx.AccessTokenClaims{
        TenantID: tda.ID(),
        UserID:   user.ID,
        ClientID: in.ClientID,
        Scopes:   client.DefaultScopes,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to issue access token: %w", err)
    }

    refreshToken := generateOpaqueToken()
    _, err = tda.Tokens().Create(ctx, repository.CreateRefreshTokenInput{
        TenantID:   tda.ID(),
        UserID:     user.ID,
        ClientID:   in.ClientID,
        TokenHash:  hashToken(refreshToken),
        TTLSeconds: int(s.deps.RefreshTTL.Seconds()),
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create refresh token: %w", err)
    }

    return &dto.LoginResult{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    3600,
        TokenType:    "Bearer",
    }, nil
}
```

**Clave**:
- Usar `DAL.ForTenant()` para acceso a datos
- Separar Control Plane (Clients) de Data Plane (Users)
- Errores específicos del dominio
- Sin dependencias HTTP (w, r)

#### **PASO 5: Agregar al Aggregator de Services**

```bash
# Ubicación
internal/http/v2/services/auth/services.go
```

```go
package auth

type Services struct {
    Login    LoginService      // ← AGREGAR
    Refresh  RefreshService
    Register RegisterService
    // ...
}

func NewServices(d Deps) Services {
    return Services{
        Login: NewLoginService(LoginDeps{  // ← AGREGAR
            DAL:        d.DAL,
            Issuer:     d.Issuer,
            RefreshTTL: d.RefreshTTL,
            ClaimsHook: d.ClaimsHook,
        }),
        // ...
    }
}
```

#### **PASO 6: Crear Controller**

```bash
# Ubicación
internal/http/v2/controllers/auth/login_controller.go
```

```go
package auth

import (
    "encoding/json"
    "net/http"

    dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/auth"
    httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
    svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/auth"
)

type LoginController struct {
    service svc.LoginService
}

func NewLoginController(service svc.LoginService) *LoginController {
    return &LoginController{service: service}
}

func (c *LoginController) Login(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 1. Validar método
    if r.Method != http.MethodPost {
        w.Header().Set("Allow", "POST")
        httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
        return
    }

    // 2. Parse request
    var req dto.LoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httperrors.WriteError(w, httperrors.ErrInvalidJSON)
        return
    }

    // 3. Delegar al service
    result, err := c.service.LoginPassword(ctx, req)
    if err != nil {
        writeLoginError(w, err)
        return
    }

    // 4. Response
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(result)
}

func writeLoginError(w http.ResponseWriter, err error) {
    switch err {
    case svc.ErrInvalidCredentials:
        httperrors.WriteError(w, httperrors.ErrUnauthorized.WithDetail("invalid credentials"))
    case svc.ErrUserDisabled:
        httperrors.WriteError(w, httperrors.ErrForbidden.WithDetail("user disabled"))
    default:
        httperrors.WriteError(w, httperrors.ErrInternalServer)
    }
}
```

**Clave**:
- Controller NO tiene lógica de negocio
- Solo: parse → service → response
- Errores HTTP via `httperrors.WriteError()`

#### **PASO 7: Agregar al Aggregator de Controllers**

```bash
# Ubicación
internal/http/v2/controllers/auth/controllers.go
```

```go
package auth

type Controllers struct {
    Login    *LoginController  // ← AGREGAR
    Refresh  *RefreshController
    Register *RegisterController
    // ...
}

func NewControllers(s svc.Services) *Controllers {
    return &Controllers{
        Login: NewLoginController(s.Login),  // ← AGREGAR
        // ...
    }
}
```

#### **PASO 8: Registrar Ruta en Router**

```bash
# Ubicación
internal/http/v2/router/auth_routes.go
```

```go
package router

func RegisterAuthRoutes(mux *http.ServeMux, deps AuthRouterDeps) {
    c := deps.Controllers

    // POST /v2/auth/login ← AGREGAR
    mux.Handle("/v2/auth/login",
        authHandler(deps.RateLimiter,
            http.HandlerFunc(c.Login.Login)))

    // ... otras rutas
}

// authHandler aplica middlewares
func authHandler(limiter mw.RateLimiter, handler http.Handler) http.Handler {
    chain := []mw.Middleware{
        mw.WithRecover(),
        mw.WithRequestID(),
        mw.WithSecurityHeaders(),
        mw.WithNoStore(),
    }

    if limiter != nil {
        chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
            Limiter: limiter,
            KeyFunc: mw.IPPathRateKey,
        }))
    }

    chain = append(chain, mw.WithLogging())

    return mw.Chain(handler, chain...)
}
```

**Clave**:
- Replicar rutas originales V1 (`/v1/auth/login` → `/v2/auth/login`)
- Aplicar middlewares consistentes
- Documentar rate limiting si aplica

#### **PASO 9: Verificar Wiring Completo**

```bash
# Verificar en internal/app/v2/app.go
# que los aggregators estén inyectados:

svcs := services.New(services.Deps{
    DAL:        deps.DAL,
    Issuer:     deps.Issuer,
    RefreshTTL: 30 * 24 * time.Hour,
    // ...
})

authControllers := authctrl.NewControllers(svcs.Auth)

router.RegisterV2Routes(router.V2RouterDeps{
    Mux:             mux,
    AuthControllers: authControllers,
    // ...
})
```

### Checklist de Migración

- [ ] **PASO 1**: Handler V1 analizado
- [ ] **PASO 2**: DTOs creados en `dto/{domain}/`
- [ ] **PASO 3**: Service interface definida en `services/{domain}/contracts.go`
- [ ] **PASO 4**: Service implementado en `services/{domain}/{nombre}_service.go`
- [ ] **PASO 5**: Service agregado a `services/{domain}/services.go`
- [ ] **PASO 6**: Controller creado en `controllers/{domain}/{nombre}_controller.go`
- [ ] **PASO 7**: Controller agregado a `controllers/{domain}/controllers.go`
- [ ] **PASO 8**: Ruta registrada en `router/{domain}_routes.go`
- [ ] **PASO 9**: Wiring verificado en `app/v2/app.go`
- [ ] **PASO 10**: Herramientas V2 usadas (DAL V2, JWT V2, Email V2, Cache V2)
- [ ] **PASO 11**: Errores mapeados a HTTP via `httperrors`
- [ ] **PASO 12**: Logging agregado con `logger.From(ctx)`

---

## 🛠️ GUÍA DE IMPLEMENTACIÓN

### Service Pattern

#### Estructura de un Service

```go
// 1. CONTRACTS (Interfaces)
// services/{domain}/contracts.go
package domain

type FooService interface {
    DoSomething(ctx context.Context, req dto.FooRequest) (*dto.FooResult, error)
}

// 2. DEPENDENCIES
type FooDeps struct {
    DAL        store.DataAccessLayer
    Issuer     *jwtx.Issuer
    Cache      cache.Client
    // ... otras deps
}

// 3. IMPLEMENTATION (struct privado)
type fooService struct {
    deps FooDeps
}

// 4. CONSTRUCTOR
func NewFooService(deps FooDeps) FooService {
    // Validar deps si es necesario
    if deps.DAL == nil {
        panic("DAL required")
    }
    return &fooService{deps: deps}
}

// 5. MÉTODOS
func (s *fooService) DoSomething(ctx context.Context, req dto.FooRequest) (*dto.FooResult, error) {
    // Lógica de negocio pura
    // Sin HTTP (w, r)
    // Sin referencias a handlers
    return &dto.FooResult{}, nil
}
```

#### Acceso a Datos via DAL

```go
func (s *myService) PerformAction(ctx context.Context, req dto.Request) error {
    // 1. Obtener acceso al tenant
    tda, err := s.deps.DAL.ForTenant(ctx, req.TenantID)
    if err != nil {
        if store.IsTenantNotFound(err) {
            return fmt.Errorf("tenant not found")
        }
        return err
    }

    // 2. Control Plane (siempre disponible - FS)
    client, err := tda.Clients().Get(ctx, tda.ID(), req.ClientID)
    if err != nil {
        return fmt.Errorf("client not found")
    }

    scopes, err := tda.Scopes().List(ctx, tda.ID())

    // 3. Data Plane (requiere DB)
    if err := tda.RequireDB(); err != nil {
        return fmt.Errorf("tenant has no database")
    }

    user, _, err := tda.Users().GetByEmail(ctx, tda.ID(), req.Email)
    if repository.IsNotFound(err) {
        return fmt.Errorf("user not found")
    }

    token, err := tda.Tokens().Create(ctx, repository.CreateRefreshTokenInput{
        TenantID: tda.ID(),
        UserID:   user.ID,
        // ...
    })

    return nil
}
```

#### Error Handling en Services

```go
// 1. Definir errores específicos del dominio
var (
    ErrInvalidInput       = fmt.Errorf("invalid input")
    ErrResourceNotFound   = fmt.Errorf("resource not found")
    ErrUnauthorized       = fmt.Errorf("unauthorized")
    ErrConflict           = fmt.Errorf("conflict")
)

// 2. Retornar errores con contexto
func (s *service) DoThing(ctx, req) error {
    if req.Field == "" {
        return fmt.Errorf("%w: field is required", ErrInvalidInput)
    }

    user, err := s.deps.DAL.ForTenant(...).Users().GetByID(...)
    if repository.IsNotFound(err) {
        return ErrResourceNotFound
    }

    return nil
}

// 3. Controller mapea a HTTP
func (c *controller) Handler(w, r) {
    err := c.service.DoThing(...)
    switch {
    case errors.Is(err, svc.ErrInvalidInput):
        httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(err.Error()))
    case errors.Is(err, svc.ErrResourceNotFound):
        httperrors.WriteError(w, httperrors.ErrNotFound)
    case errors.Is(err, svc.ErrUnauthorized):
        httperrors.WriteError(w, httperrors.ErrUnauthorized)
    default:
        httperrors.WriteError(w, httperrors.ErrInternalServer)
    }
}
```

### Controller Pattern

#### Estructura de un Controller

```go
package domain

type FooController struct {
    service svc.FooService
}

func NewFooController(service svc.FooService) *FooController {
    return &FooController{service: service}
}

func (c *FooController) HandleFoo(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    // 1. Validar método HTTP
    if r.Method != http.MethodPost {
        w.Header().Set("Allow", "POST")
        httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
        return
    }

    // 2. Parse request (JSON, Form, Query, Path)
    var req dto.FooRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httperrors.WriteError(w, httperrors.ErrInvalidJSON)
        return
    }

    // 3. Delegar al service
    result, err := c.service.DoFoo(ctx, req)
    if err != nil {
        c.writeFooError(w, err)
        return
    }

    // 4. Response
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(result)
}

func (c *FooController) writeFooError(w http.ResponseWriter, err error) {
    // Mapear errores del service a HTTP
    switch {
    case errors.Is(err, svc.ErrInvalidInput):
        httperrors.WriteError(w, httperrors.ErrBadRequest)
    case errors.Is(err, svc.ErrNotFound):
        httperrors.WriteError(w, httperrors.ErrNotFound)
    default:
        httperrors.WriteError(w, httperrors.ErrInternalServer)
    }
}
```

#### Parsing de Requests

```go
// JSON Body
var req dto.LoginRequest
if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
    httperrors.WriteError(w, httperrors.ErrInvalidJSON)
    return
}

// Form Data
if err := r.ParseForm(); err != nil {
    httperrors.WriteError(w, httperrors.ErrBadRequest)
    return
}
clientID := r.FormValue("client_id")

// Query Params
tenantSlug := r.URL.Query().Get("tenant")

// Path Params (manual con http.ServeMux)
// /v2/admin/clients/{id}
path := strings.TrimPrefix(r.URL.Path, "/v2/admin/clients/")
clientID := strings.Split(path, "/")[0]

// Headers
tenantID := r.Header.Get("X-Tenant-ID")
```

### Router Pattern

#### Estructura de un Router Module

```go
// router/auth_routes.go
package router

import (
    "net/http"
    ctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/auth"
    mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
)

type AuthRouterDeps struct {
    Controllers *ctrl.Controllers
    RateLimiter mw.RateLimiter
    Issuer      *jwtx.Issuer
}

func RegisterAuthRoutes(mux *http.ServeMux, deps AuthRouterDeps) {
    c := deps.Controllers

    // Rutas públicas
    mux.Handle("/v2/auth/login",
        publicHandler(deps.RateLimiter,
            http.HandlerFunc(c.Login.Login)))

    mux.Handle("/v2/auth/register",
        publicHandler(deps.RateLimiter,
            http.HandlerFunc(c.Register.Register)))

    // Rutas autenticadas
    mux.Handle("/v2/me",
        authedHandler(deps.RateLimiter, deps.Issuer,
            http.HandlerFunc(c.Me.Me)))

    // Rutas con scope
    mux.Handle("/v2/profile",
        scopedHandler(deps.RateLimiter, deps.Issuer, "profile:read",
            http.HandlerFunc(c.Profile.GetProfile)))
}
```

#### Middleware Chains (Orden Recomendado)

```go
func publicHandler(limiter mw.RateLimiter, handler http.Handler) http.Handler {
    chain := []mw.Middleware{
        // 1. Recover PRIMERO (catch panics)
        mw.WithRecover(),

        // 2. Request ID (tracing)
        mw.WithRequestID(),

        // 3. Security Headers (CORS, CSP, etc)
        mw.WithSecurityHeaders(),

        // 4. Cache Control
        mw.WithNoStore(),

        // 5. Rate Limiting (antes de lógica pesada)
        // Si está habilitado
    }

    if limiter != nil {
        chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
            Limiter: limiter,
            KeyFunc: mw.IPPathRateKey,
        }))
    }

    // 6. Logging AL FINAL (captura response status)
    chain = append(chain, mw.WithLogging())

    return mw.Chain(handler, chain...)
}

func authedHandler(limiter mw.RateLimiter, issuer *jwtx.Issuer, handler http.Handler) http.Handler {
    chain := []mw.Middleware{
        mw.WithRecover(),
        mw.WithRequestID(),
        mw.WithSecurityHeaders(),
        mw.WithNoStore(),
    }

    if limiter != nil {
        chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
            Limiter: limiter,
            KeyFunc: mw.IPPathRateKey,
        }))
    }

    // Auth ANTES de la lógica de negocio
    chain = append(chain, mw.RequireAuth(issuer))

    chain = append(chain, mw.WithLogging())

    return mw.Chain(handler, chain...)
}

func scopedHandler(limiter mw.RateLimiter, issuer *jwtx.Issuer, scope string, handler http.Handler) http.Handler {
    chain := []mw.Middleware{
        mw.WithRecover(),
        mw.WithRequestID(),
        mw.WithSecurityHeaders(),
        mw.WithNoStore(),
    }

    if limiter != nil {
        chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
            Limiter: limiter,
            KeyFunc: mw.IPPathRateKey,
        }))
    }

    chain = append(chain, mw.RequireAuth(issuer))
    chain = append(chain, mw.RequireScope(scope))  // DESPUÉS de auth

    chain = append(chain, mw.WithLogging())

    return mw.Chain(handler, chain...)
}
```

**Orden de Middlewares (Regla de Oro)**:
1. **Recover** (catch panics)
2. **RequestID** (tracing)
3. **SecurityHeaders** (CORS, CSP)
4. **CacheControl** (No-Store)
5. **RateLimit** (protección DDoS)
6. **Auth** (JWT validation)
7. **Scope** (permisos)
8. **Logging** (AL FINAL para capturar status code)

### DTO Pattern

#### Estructura de DTOs

```go
// dto/auth/login.go
package auth

// Request DTO
type LoginRequest struct {
    TenantID string `json:"tenant_id"`
    ClientID string `json:"client_id"`
    Email    string `json:"email"`
    Password string `json:"password"`

    // Campos opcionales
    TrustedDeviceToken string `json:"trusted_device_token,omitempty"`
}

// Response DTO
type LoginResult struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
    ExpiresIn    int    `json:"expires_in"`
    TokenType    string `json:"token_type"`

    // Campos condicionales (MFA)
    MFARequired bool   `json:"mfa_required,omitempty"`
    MFAToken    string `json:"mfa_token,omitempty"`
}
```

#### Validación de DTOs (TODO - Futuro)

```go
// Usar validator library
import "github.com/go-playground/validator/v10"

type LoginRequest struct {
    TenantID string `json:"tenant_id" validate:"required,min=1"`
    ClientID string `json:"client_id" validate:"required,min=1"`
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

// Middleware de validación (futuro)
func ValidateDTO(v *validator.Validate) mw.Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Extract DTO from context
            // Validate
            // Return 400 if invalid
            next.ServeHTTP(w, r)
        })
    }
}
```

### Error Handling Pattern

#### Errores HTTP Centralizados

```go
// internal/http/v2/errors/types.go
package errors

var (
    // 4xx Client Errors
    ErrBadRequest          = &HTTPError{Code: 400, Message: "Bad Request"}
    ErrUnauthorized        = &HTTPError{Code: 401, Message: "Unauthorized"}
    ErrForbidden           = &HTTPError{Code: 403, Message: "Forbidden"}
    ErrNotFound            = &HTTPError{Code: 404, Message: "Not Found"}
    ErrMethodNotAllowed    = &HTTPError{Code: 405, Message: "Method Not Allowed"}
    ErrConflict            = &HTTPError{Code: 409, Message: "Conflict"}
    ErrInvalidJSON         = &HTTPError{Code: 400, Message: "Invalid JSON"}

    // 5xx Server Errors
    ErrInternalServer      = &HTTPError{Code: 500, Message: "Internal Server Error"}
    ErrNotImplemented      = &HTTPError{Code: 501, Message: "Not Implemented"}
    ErrServiceUnavailable  = &HTTPError{Code: 503, Message: "Service Unavailable"}
)

type HTTPError struct {
    Code    int    `json:"-"`
    Message string `json:"error"`
    Detail  string `json:"detail,omitempty"`
}

func (e *HTTPError) WithDetail(detail string) *HTTPError {
    return &HTTPError{
        Code:    e.Code,
        Message: e.Message,
        Detail:  detail,
    }
}
```

```go
// internal/http/v2/errors/errors.go
package errors

import (
    "encoding/json"
    "net/http"
)

func WriteError(w http.ResponseWriter, err *HTTPError) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(err.Code)
    json.NewEncoder(w).Encode(err)
}
```

#### Uso en Controllers

```go
func (c *Controller) Handle(w, r) {
    result, err := c.service.DoSomething(...)
    if err != nil {
        c.writeError(w, err)
        return
    }
    // ...
}

func (c *Controller) writeError(w http.ResponseWriter, err error) {
    switch {
    case errors.Is(err, svc.ErrInvalidInput):
        httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(err.Error()))
    case errors.Is(err, svc.ErrNotFound):
        httperrors.WriteError(w, httperrors.ErrNotFound)
    case errors.Is(err, svc.ErrUnauthorized):
        httperrors.WriteError(w, httperrors.ErrUnauthorized)
    case errors.Is(err, svc.ErrConflict):
        httperrors.WriteError(w, httperrors.ErrConflict)
    default:
        httperrors.WriteError(w, httperrors.ErrInternalServer)
    }
}
```

---

## 💾 DATA ACCESS LAYER (DAL)

### Conceptos Clave

**DAL V2** abstrae el acceso a datos detrás de interfaces de repositorio, soportando múltiples drivers (FS, Postgres, MySQL, Mongo) de forma transparente.

### Interfaces Principales

```go
// DataAccessLayer - Punto de entrada principal
type DataAccessLayer interface {
    ForTenant(ctx, slugOrID) (TenantDataAccess, error)
    ConfigAccess() ConfigAccess
    Mode() OperationalMode
    Capabilities() ModeCapabilities
    Close() error
}

// TenantDataAccess - Acceso a datos de un tenant
type TenantDataAccess interface {
    // Identificación
    Slug() string
    ID() string
    Settings() *repository.TenantSettings

    // Control Plane (siempre disponible - FS)
    Clients() repository.ClientRepository
    Scopes() repository.ScopeRepository

    // Data Plane (requiere DB)
    Users() repository.UserRepository
    Tokens() repository.TokenRepository
    MFA() repository.MFARepository
    Consents() repository.ConsentRepository
    Identities() repository.IdentityRepository
    EmailTokens() repository.EmailTokenRepository

    // Helpers
    HasDB() bool
    RequireDB() error
}

// ConfigAccess - Acceso al Control Plane global
type ConfigAccess interface {
    Tenants() repository.TenantRepository
    Clients(tenantSlug) repository.ClientRepository
    Scopes(tenantSlug) repository.ScopeRepository
    Keys() repository.KeyRepository
}
```

### Patrón de Uso: ForTenant()

```go
// En un service
func (s *myService) DoAction(ctx context.Context, req dto.Request) error {
    // 1. Obtener acceso al tenant
    tda, err := s.deps.DAL.ForTenant(ctx, req.TenantID)
    if err != nil {
        if store.IsTenantNotFound(err) {
            return fmt.Errorf("tenant not found")
        }
        return err
    }

    // 2. Control Plane (siempre disponible)
    client, _ := tda.Clients().Get(ctx, tda.ID(), req.ClientID)
    scopes, _ := tda.Scopes().List(ctx, tda.ID())
    settings := tda.Settings()  // *repository.TenantSettings

    // 3. Data Plane (verificar DB primero)
    if !tda.HasDB() {
        return fmt.Errorf("tenant has no database configured")
    }

    // Alternativa: RequireDB() retorna error si no hay DB
    if err := tda.RequireDB(); err != nil {
        return err
    }

    user, _, _ := tda.Users().GetByEmail(ctx, tda.ID(), req.Email)
    token, _ := tda.Tokens().Create(ctx, ...)

    return nil
}
```

### Patrón de Uso: ConfigAccess()

```go
// Para operaciones admin que afectan múltiples tenants
func (s *adminService) ListAllTenants(ctx context.Context) ([]dto.Tenant, error) {
    config := s.deps.DAL.ConfigAccess()

    tenants, err := config.Tenants().List(ctx)
    if err != nil {
        return nil, err
    }

    // Mapear a DTOs
    result := make([]dto.Tenant, len(tenants))
    for i, t := range tenants {
        result[i] = dto.Tenant{
            ID:   t.ID,
            Slug: t.Slug,
            Name: t.Name,
        }
    }
    return result, nil
}
```

### Errores del DAL

```go
// Verificar errores específicos
tda, err := dal.ForTenant(ctx, slug)
if err != nil {
    switch {
    case store.IsTenantNotFound(err):
        // 404 - Tenant no existe
    case store.IsNoDBForTenant(err):
        // 503 - Tenant sin DB configurada
    default:
        // 500 - Error interno
    }
}

// Verificar errores de repositorio
user, err := tda.Users().GetByEmail(...)
if repository.IsNotFound(err) {
    // Usuario no encontrado
}
if repository.IsConflict(err) {
    // Email duplicado
}
```

### Control Plane: FileSystem Structure

```
data/hellojohn/
├── tenants/
│   ├── acme/
│   │   ├── tenant.yaml          ← Config del tenant
│   │   ├── clients.yaml         ← OAuth clients
│   │   └── scopes.yaml          ← Scopes disponibles
│   │
│   └── local/                   ← Tenant por defecto
│       ├── tenant.yaml
│       ├── clients.yaml
│       └── scopes.yaml
│
└── keys/
    ├── active.json              ← Clave global (EdDSA)
    ├── retiring.json            ← Clave en rotación
    │
    └── acme/                    ← Claves por tenant (opcional)
        ├── active.json
        └── retiring.json
```

**tenant.yaml**:
```yaml
id: "550e8400-e29b-41d4-a716-446655440000"
slug: "acme"
name: "ACME Corp"
language: "en"
created_at: "2025-01-15T10:00:00Z"

settings:
  issuer_mode: "path"  # path | subdomain

  user_db:
    driver: "postgres"
    dsn_enc: "encrypted_dsn_here"
    max_open_conns: 25
    max_idle_conns: 5

  smtp:
    host: "smtp.sendgrid.net"
    port: 587
    from: "noreply@acme.com"
    password_enc: "encrypted_password_here"

  cache:
    kind: "redis"
    addr: "localhost:6379"
    pass_enc: "encrypted_password_here"

  branding:
    logo_url: "/v1/assets/acme/logo.png"
    primary_color: "#0066cc"
```

**clients.yaml**:
```yaml
clients:
  - client_id: "web-app"
    name: "ACME Web App"
    type: "public"
    redirect_uris:
      - "https://app.acme.com/callback"
    default_scopes:
      - "openid"
      - "profile"
      - "email"
    providers:
      - "password"
      - "google"
    created_at: "2025-01-15T10:00:00Z"

  - client_id: "mobile-app"
    name: "ACME Mobile"
    type: "confidential"
    secret_enc: "encrypted_secret_here"
    redirect_uris:
      - "acme://callback"
    default_scopes:
      - "openid"
    providers:
      - "password"
    created_at: "2025-01-15T11:00:00Z"
```

---

## 🌐 CLUSTER/RAFT (4 MODOS)

### ¿Por qué Raft?

HelloJohn puede ejecutarse en **múltiples nodos** para alta disponibilidad. El problema: ¿cómo sincronizar cambios de configuración (crear tenant, modificar client) entre nodos sin una DB centralizada?

**Solución**: Raft consensus algorithm para replicar mutaciones del Control Plane.

### 4 Modos Operacionales

```
┌──────────────────────────────────────────────────────────────┐
│                       MODO 1: FS ONLY                        │
├──────────────────────────────────────────────────────────────┤
│ Control Plane: FileSystem                                    │
│ Data Plane:    ❌ Sin DB                                      │
│ Multi-nodo:    ⚠️ Requiere Raft para sincronizar FS          │
│                                                              │
│ Uso: Desarrollo, testing, demos sin usuarios                │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│              MODO 2: FS + TENANT DB (Híbrido)                │
├──────────────────────────────────────────────────────────────┤
│ Control Plane: FileSystem (tenants, clients, scopes)         │
│ Data Plane:    DB dedicada por tenant (users, tokens)        │
│ Multi-nodo:    ⚠️ Requiere Raft para sincronizar FS          │
│                                                              │
│ Multi-driver:  ✅ Tenant A: Postgres, Tenant B: MySQL        │
│ Uso: Producción SaaS, aislamiento fuerte por tenant         │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│                MODO 3: FS + GLOBAL DB                        │
├──────────────────────────────────────────────────────────────┤
│ Control Plane: DB Global (replica del FS)                    │
│ Data Plane:    ❌ Sin DB de usuarios                          │
│ Multi-nodo:    ✅ Sin Raft (DB Global es source of truth)    │
│                                                              │
│ Uso: Multi-nodo en cloud, evitar Raft, solo config          │
└──────────────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────────────┐
│           MODO 4: FS + GLOBAL DB + TENANT DB (Full)          │
├──────────────────────────────────────────────────────────────┤
│ Control Plane: DB Global (replica del FS)                    │
│ Data Plane:    DB dedicada por tenant                        │
│ Multi-nodo:    ✅ Sin Raft (DB Global es source of truth)    │
│                                                              │
│ Multi-driver:  ✅ Tenant A: Postgres, Tenant B: MySQL        │
│ Uso: Producción empresarial, HA completo                    │
└──────────────────────────────────────────────────────────────┘
```

### Tabla Comparativa

| Feature | FS Only | FS+TenantDB | FS+GlobalDB | Full |
|---------|---------|-------------|-------------|------|
| **Control Plane** | FS | FS | FS+Global DB | FS+Global DB |
| **Data Plane** | ❌ | DB por tenant | ❌ | DB por tenant |
| **Multi-nodo sin Raft** | ❌ | ❌ | ✅ | ✅ |
| **Multi-driver** | N/A | ✅ | N/A | ✅ |
| **Users/Tokens** | ❌ | ✅ | ❌ | ✅ |
| **Complejidad** | Baja | Media | Media | Alta |
| **Uso típico** | Dev/Testing | SaaS | Multi-nodo Config | Enterprise HA |

### Raft: Cuándo y Cómo

**Cuándo habilitar Raft**:
- Modo 1 (FS Only) + Multi-nodo
- Modo 2 (FS + TenantDB) + Multi-nodo

**Cuándo NO usar Raft**:
- Modo 3 o 4 (Global DB disponible)
- Single-nodo

### Flujo de Mutación con Raft

```go
// 1. Admin quiere crear un client
POST /v2/admin/clients
Body: {
    "tenant_id": "acme",
    "client_id": "new-app",
    "name": "New App",
    "type": "public"
}

// 2. Controller → Service
func (s *adminService) CreateClient(ctx, req) error {
    // 3. Verificar liderazgo (si Raft habilitado)
    hook := s.deps.DAL.ClusterHook()
    if err := hook.RequireLeaderForMutation(ctx); err != nil {
        return ErrNotLeader  // 503 - Redirigir a líder
    }

    // 4. Aplicar mutación localmente (FS)
    tda, _ := s.deps.DAL.ForTenant(ctx, req.TenantID)
    client, err := tda.Clients().Create(ctx, req.TenantID, input)
    if err != nil {
        return err
    }

    // 5. Replicar via Raft
    mutation := store.NewClientMutation(
        store.MutationClientCreate,
        req.TenantID,
        client.ClientID,
        client,
    )

    index, err := hook.Apply(ctx, mutation)
    if err != nil {
        return err
    }

    // 6. Esperar confirmación de mayoría
    // (Apply ya espera commit del quorum)

    return nil
}

// 7. Followers reciben mutación y aplican a su FS local
```

### Configuración de Raft

```bash
# ENV Variables
CLUSTER_MODE=embedded           # embedded | disabled
CLUSTER_BOOTSTRAP=true          # true en primer nodo, false en seguidores
CLUSTER_NODE_ID=node-1          # Identificador único del nodo
CLUSTER_BIND_ADDR=0.0.0.0:7000  # Dirección de bind para Raft
CLUSTER_PEERS=node-2:7000,node-3:7000  # Peers iniciales
```

**Startup de 3 nodos**:

```bash
# Nodo 1 (Líder inicial)
CLUSTER_MODE=embedded \
CLUSTER_BOOTSTRAP=true \
CLUSTER_NODE_ID=node-1 \
CLUSTER_BIND_ADDR=0.0.0.0:7000 \
./hellojohn

# Nodo 2 (Follower)
CLUSTER_MODE=embedded \
CLUSTER_BOOTSTRAP=false \
CLUSTER_NODE_ID=node-2 \
CLUSTER_BIND_ADDR=0.0.0.0:7000 \
CLUSTER_PEERS=node-1:7000 \
./hellojohn

# Nodo 3 (Follower)
CLUSTER_MODE=embedded \
CLUSTER_BOOTSTRAP=false \
CLUSTER_NODE_ID=node-3 \
CLUSTER_BIND_ADDR=0.0.0.0:7000 \
CLUSTER_PEERS=node-1:7000 \
./hellojohn
```

### Tipos de Mutaciones

```go
const (
    MutationTenantCreate   = "tenant.create"
    MutationTenantUpdate   = "tenant.update"
    MutationTenantDelete   = "tenant.delete"

    MutationClientCreate   = "client.create"
    MutationClientUpdate   = "client.update"
    MutationClientDelete   = "client.delete"

    MutationScopeCreate    = "scope.create"
    MutationScopeDelete    = "scope.delete"

    MutationKeyRotate      = "key.rotate"
    MutationSettingsUpdate = "settings.update"
)
```

### ClusterHook Interface

```go
type ClusterHook interface {
    // RequireLeaderForMutation verifica que somos el líder
    RequireLeaderForMutation(ctx context.Context) error

    // Apply replica una mutación al cluster
    Apply(ctx context.Context, mutation Mutation) (uint64, error)

    // Stats retorna estadísticas del cluster
    Stats() ClusterStats

    // IsLeader indica si este nodo es el líder
    IsLeader() bool

    // LeaderAddr retorna la dirección del líder actual
    LeaderAddr() string
}
```

---

## 📚 REFERENCIAS RÁPIDAS

### Comandos Útiles

```bash
# Compilar V2
go build -o hellojohn ./cmd/service_v2

# Ejecutar V2
FS_ROOT=./data/hellojohn \
SIGNING_MASTER_KEY=your-64-char-hex-key \
SECRETBOX_MASTER_KEY=your-base64-key \
V2_SERVER_ADDR=:8082 \
./hellojohn

# Migrar tenant
./migrate -tenant=acme

# Rotar keys
./keys rotate -tenant=acme -grace=7d
```

### Variables de Entorno Clave

| Variable | Descripción | Ejemplo |
|----------|-------------|---------|
| `FS_ROOT` | Directorio del Control Plane | `./data/hellojohn` |
| `SIGNING_MASTER_KEY` | Master key para JWT (hex, ≥32 bytes) | `abcd1234...` (64 chars) |
| `SECRETBOX_MASTER_KEY` | Key para cifrado de secrets (base64, 32 bytes) | `base64string==` |
| `V2_SERVER_ADDR` | Puerto del servidor V2 | `:8082` |
| `V2_BASE_URL` | URL base para issuer | `http://localhost:8082` |
| `REGISTER_AUTO_LOGIN` | Auto-login tras registro | `true` |
| `FS_ADMIN_ENABLE` | Permitir registro de FS admins | `false` |
| `CLUSTER_MODE` | Modo cluster | `embedded` / `disabled` |
| `CLUSTER_BOOTSTRAP` | Bootstrap Raft | `true` (solo primer nodo) |

### Endpoints V2 Principales

| Endpoint | Método | Descripción |
|----------|--------|-------------|
| `/readyz` | GET | Health check |
| `/v2/auth/login` | POST | Login con password |
| `/v2/auth/register` | POST | Registro de usuario |
| `/v2/auth/refresh` | POST | Refresh token |
| `/v2/me` | GET | User info (autenticado) |
| `/oauth2/authorize` | GET | OAuth2 authorization |
| `/oauth2/token` | POST | OAuth2 token exchange |
| `/.well-known/openid-configuration` | GET | OIDC discovery |
| `/.well-known/jwks.json` | GET | JWKS público |
| `/v2/admin/clients` | GET/POST | Admin: gestión de clients |
| `/v2/admin/tenants` | GET/POST | Admin: gestión de tenants |

### Archivos Clave para Nuevas Features

| Tarea | Archivos a Modificar |
|-------|----------------------|
| **Nuevo endpoint de Auth** | 1. `dto/auth/{nombre}.go`<br>2. `services/auth/contracts.go`<br>3. `services/auth/{nombre}_service.go`<br>4. `services/auth/services.go`<br>5. `controllers/auth/{nombre}_controller.go`<br>6. `controllers/auth/controllers.go`<br>7. `router/auth_routes.go` |
| **Nuevo dominio completo** | 1. `dto/{domain}/`<br>2. `services/{domain}/`<br>3. `controllers/{domain}/`<br>4. `services/services.go`<br>5. `controllers/controllers.go`<br>6. `router/{domain}_routes.go`<br>7. `router/router.go` |
| **Nuevo adapter de DB** | 1. `store/v2/adapters/{driver}/`<br>2. `store/v2/adapters/dal/register.go` |
| **Nuevo middleware** | 1. `middlewares/{nombre}.go`<br>2. Aplicar en `router/{domain}_routes.go` |

### Herramientas V2 (Imports Comunes)

```go
// DAL
import store "github.com/dropDatabas3/hellojohn/internal/store/v2"

// Control Plane
import cp "github.com/dropDatabas3/hellojohn/internal/controlplane/v2"

// Email
import emailv2 "github.com/dropDatabas3/hellojohn/internal/email/v2"

// JWT
import jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"

// Cache
import "github.com/dropDatabas3/hellojohn/internal/cache/v2"

// Repository Interfaces
import "github.com/dropDatabas3/hellojohn/internal/domain/repository"

// HTTP Errors
import httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"

// Middlewares
import mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"

// Logging
import "github.com/dropDatabas3/hellojohn/internal/observability/logger"
```

### Patrones Anti-Pattern (Evitar)

❌ **NO HACER**:
```go
// Lógica de negocio en Controller
func (c *Controller) Handle(w, r) {
    user, _ := db.Query("SELECT ...")  // ❌
    if user.Password != req.Password { // ❌
        http.Error(w, "invalid", 401)  // ❌
    }
}

// Dependencias HTTP en Service
func (s *Service) DoThing(w http.ResponseWriter, r *http.Request) { // ❌
    json.NewDecoder(r.Body).Decode(&req)  // ❌
}

// Acceso directo a DB sin DAL
db := s.openPostgres()  // ❌
```

✅ **SÍ HACER**:
```go
// Separación de responsabilidades
// Controller: Parse → Service → Response
func (c *Controller) Handle(w, r) {
    var req dto.Request
    json.NewDecoder(r.Body).Decode(&req)

    result, err := c.service.DoThing(ctx, req)
    if err != nil {
        c.writeError(w, err)
        return
    }

    json.NewEncoder(w).Encode(result)
}

// Service: Lógica pura + DAL
func (s *Service) DoThing(ctx, req) (*dto.Result, error) {
    tda, _ := s.deps.DAL.ForTenant(ctx, req.TenantID)
    user, _ := tda.Users().GetByEmail(...)
    // ...
}
```

### Recursos de Documentación

**Confiables**:
- `docs/v2-toolbox.md`
- `docs/refactor_docs/V1_HANDLERS_INVENTORY.md`
- `docs/refactor_docs/V1_ROUTES_MASTER_LIST.md`
- `internal/store/v2/README.md`
- `internal/controlplane/v2/README.md`
- `internal/jwt/README.md`

**Contrastar con código real** (docs pueden estar desactualizados):
- `internal/http/v2/services/services.go`
- `internal/http/v2/controllers/controllers.go`
- `internal/http/v2/router/router.go`
- `internal/app/v2/app.go`

---

## 🎯 CHECKLIST DE MIGRACIÓN COMPLETA

### Pre-Migración
- [ ] Handler V1 identificado y analizado
- [ ] Rutas originales documentadas (routes.go + handler)
- [ ] Dependencias mapeadas (Store, Issuer, ControlPlane, etc)
- [ ] Lógica de negocio extraída mentalmente

### Implementación
- [ ] DTOs creados en `dto/{domain}/`
- [ ] Service interface definida en `services/{domain}/contracts.go`
- [ ] Service implementado en `services/{domain}/{nombre}_service.go`
- [ ] Service agregado a `services/{domain}/services.go`
- [ ] Controller creado en `controllers/{domain}/{nombre}_controller.go`
- [ ] Controller agregado a `controllers/{domain}/controllers.go`
- [ ] Rutas registradas en `router/{domain}_routes.go`
- [ ] Middlewares aplicados correctamente

### Validación
- [ ] Errores del service mapeados a HTTP
- [ ] Herramientas V2 usadas (DAL V2, JWT V2, Email V2)
- [ ] Logging agregado con `logger.From(ctx)`
- [ ] Sin lógica de negocio en Controller
- [ ] Sin referencias HTTP en Service
- [ ] Control Plane vs Data Plane separados correctamente

### Post-Migración
- [ ] Wiring verificado en `app/v2/app.go`
- [ ] Testing manual con cURL/Postman
- [ ] Comparar respuestas V1 vs V2
- [ ] Handler V1 marcado como legacy (comentario)

---

## 📌 NOTAS FINALES

### Filosofía V2

1. **Separación de Responsabilidades**: Controller → Service → Repository
2. **Inyección de Dependencias**: Cascada (Infrastructure → Services → Controllers → Router)
3. **Abstracción de Datos**: DAL oculta drivers (FS, Postgres, MySQL, Mongo)
4. **Consistencia**: Todos los dominios siguen el mismo patrón
5. **Escalabilidad**: Multi-tenant, multi-DB, multi-nodo via Raft

### Roadmap V2

- [x] DAL V2 (4 modos operacionales)
- [x] Services pattern (Auth, Admin, OIDC, OAuth)
- [x] Controllers pattern (HTTP handling)
- [x] Router modular (por dominio)
- [x] Email V2 (templates, SMTP)
- [x] JWT V2 (PersistentKeystore)
- [ ] **TODO**: DTO validation (struct tags + middleware)
- [ ] **TODO**: Rate limiting V2 (por tenant)
- [ ] **TODO**: Metrics/Observability (Prometheus)
- [ ] **TODO**: Testing suite completo

### Contacto

- **Repositorio**: `hellojohn` (privado)
- **Arquitectura**: V2 (Cascada)
- **Última actualización**: 2026-01-20

---

**FIN DEL DOCUMENTO**
