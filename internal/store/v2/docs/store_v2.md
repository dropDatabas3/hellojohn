# Store V2 — Documentación Completa

## ¿Qué es Store V2?

Store V2 es la **capa de acceso a datos** de la aplicación. Su trabajo es:
1. **Leer/escribir datos** de usuarios, tokens, MFA, etc. (Data Plane)
2. **Leer/escribir configuración** de tenants, clientes OAuth, scopes (Control Plane)
3. **Manejar múltiples bases de datos** (PostgreSQL, MySQL, MongoDB)
4. **Aislar datos por tenant** (multi-tenancy)

---

## Estructura de Archivos

```
internal/store/v2/
│
├── mode.go        # Define los 4 modos de operación
├── registry.go    # Registro global de adapters
├── pool.go        # Pool de conexiones por tenant
├── factory.go     # Crea el DAL según configuración
├── manager.go     # Interfaces principales + Manager
├── migrate.go     # Sistema de migraciones SQL
├── errors.go      # Errores comunes del store
│
└── adapters/      # Implementaciones concretas
    ├── pg/        # PostgreSQL (pgxpool)
    ├── fs/        # FileSystem (YAML)
    ├── mysql/     # MySQL (placeholder)
    ├── mongo/     # MongoDB (placeholder)
    ├── noop/      # No-op (fallback)
    └── dal/       # Auto-import de todos los adapters
```

---

## Los 4 Modos de Operación

Store V2 soporta 4 modos según la infraestructura disponible:

| Modo | FileSystem | DB Global | DB Tenant | Para qué sirve |
|------|:----------:|:---------:|:---------:|----------------|
| **ModeFSOnly** | ✅ | ❌ | ❌ | Desarrollo, testing |
| **ModeFSGlobalDB** | ✅ | ✅ | ❌ | Backup de config en DB |
| **ModeFSTenantDB** | ✅ | ❌ | ✅ | SaaS típico |
| **ModeFullDB** | ✅ | ✅ | ✅ | Enterprise |

### ¿Qué datos van en cada lugar?

```
┌─────────────────────────────────────────────────────────────┐
│                   CONTROL PLANE (FileSystem)                │
│  Siempre disponible. Almacena configuración.                │
├─────────────────────────────────────────────────────────────┤
│  • Tenants (nombre, slug, settings)                         │
│  • Clientes OAuth (clientId, redirectUris, scopes)          │
│  • Scopes (nombre, descripción)                             │
│  • Admins (usuarios administradores)                        │
│  • Branding (logo, colores)                                 │
└─────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────┐
│                   DATA PLANE (Base de Datos)                │
│  Solo disponible si el tenant tiene DB configurada.         │
├─────────────────────────────────────────────────────────────┤
│  • Users (email, password hash, perfil)                     │
│  • Identities (proveedores de login)                        │
│  • Refresh Tokens                                           │
│  • MFA (TOTP, recovery codes, trusted devices)              │
│  • Consents (scopes autorizados por usuario)                │
│  • RBAC (roles, permisos)                                   │
└─────────────────────────────────────────────────────────────┘
```

---

## Archivo por Archivo

### 1. `mode.go` — Modos de Operación

**Propósito**: Define los 4 modos y detecta cuál usar según la configuración.

**Tipos principales**:

```go
type OperationalMode int  // ModeFSOnly, ModeFSGlobalDB, ModeFSTenantDB, ModeFullDB

type DBConfig struct {
    Driver string  // "postgres", "mysql", "mongo"
    DSN    string  // Connection string
    Schema string  // Para multi-schema
}

type ModeCapabilities struct {
    Users    bool  // ¿Puede manejar usuarios?
    Tokens   bool  // ¿Puede manejar tokens?
    MFA      bool  // etc...
}
```

**Funciones clave**:

```go
// Detecta el modo automáticamente
mode := store.DetectMode(store.ModeConfig{
    FSRoot:          "./data",
    GlobalDB:        nil,               // No hay DB global
    DefaultTenantDB: &store.DBConfig{   // Sí hay DB por tenant
        Driver: "postgres",
        DSN:    "postgres://...",
    },
})
// Resultado: ModeFSTenantDB

// Consultar capacidades
caps := store.GetCapabilities(mode)
if caps.Users {
    // Podemos crear usuarios
}
```

---

### 2. `registry.go` — Registro de Adapters

**Propósito**: Sistema de plugins para agregar nuevas bases de datos.

**¿Cómo funciona?**

```
┌──────────────────────────────────────────────────────────────┐
│                    REGISTRY GLOBAL                           │
│                                                              │
│   ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐        │
│   │postgres │  │  mysql  │  │  mongo  │  │   fs    │        │
│   └────┬────┘  └────┬────┘  └────┬────┘  └────┬────┘        │
│        │            │            │            │              │
│        └──────────┬─┴────────────┴────────────┘              │
│                   │                                          │
│                   ▼                                          │
│         map[string]Adapter                                   │
└──────────────────────────────────────────────────────────────┘
```

**Interfaces clave**:

```go
// Lo que debe implementar cada adapter
type Adapter interface {
    Name() string
    Connect(ctx, cfg) (AdapterConnection, error)
}

// Lo que provee una conexión activa
type AdapterConnection interface {
    Name() string
    Ping(ctx) error
    Close() error
    
    // Repositorios (nil si no soportado)
    Users() repository.UserRepository
    Tokens() repository.TokenRepository
    MFA() repository.MFARepository
    // ... etc
}
```

**Cómo registrar un adapter nuevo**:

```go
// En adapters/cassandra/adapter.go
func init() {
    store.RegisterAdapter(&cassandraAdapter{})
}
```

---

### 3. `pool.go` — Pool de Conexiones

**Propósito**: Reutilizar conexiones entre requests (evitar reconectar cada vez).

**Características**:
- Thread-safe (múltiples goroutines pueden usarlo)
- Singleflight (evita crear múltiples conexiones para el mismo tenant)
- Health checks (cierra conexiones muertas automáticamente)
- Estadísticas (cuántas conexiones hay, cuándo se crearon)

**Flujo**:

```
Request 1 (tenant: acme)                    Request 2 (tenant: acme)
        │                                           │
        ▼                                           ▼
┌───────────────────┐                     ┌───────────────────┐
│  pool.Get("acme") │                     │  pool.Get("acme") │
└─────────┬─────────┘                     └─────────┬─────────┘
          │                                         │
          ▼                                         │
    ¿Existe en cache?                               │
          │                                         │
    NO ───┼───── YES ─────────────────────────────►│
          │                                         │
          ▼                                         ▼
    Crear conexión                          Retornar conexión
    Guardar en cache                        existente del cache
          │                                         │
          └─────────────────────────────────────────┘
                              │
                              ▼
                    Retornar AdapterConnection
```

---

### 4. `factory.go` — La Fábrica

**Propósito**: Punto central que arma todo el sistema según la configuración.

**¿Qué hace?**

```
┌─────────────────────────────────────────────────────────────────┐
│                           FACTORY                               │
│                                                                 │
│  1. Detecta el modo de operación                                │
│  2. Conecta al FileSystem (siempre)                            │
│  3. Crea el pool de conexiones para DBs de tenants             │
│  4. Provee ForTenant() para acceder a datos de un tenant       │
│                                                                 │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐         │
│  │  FS Conn    │    │   Pool      │    │  Migrator   │         │
│  │  (YAML)     │    │  (por DB)   │    │  (SQLs)     │         │
│  └─────────────┘    └─────────────┘    └─────────────┘         │
└─────────────────────────────────────────────────────────────────┘
```

**Estructura del Factory**:

```go
type Factory struct {
    cfg      FactoryConfig      // Configuración
    mode     OperationalMode    // Modo detectado
    caps     ModeCapabilities   // Capacidades
    fsConn   AdapterConnection  // Conexión al FileSystem
    pool     *ConnectionPool    // Pool de DBs por tenant
    migrator *Migrator          // Para migraciones SQL
}
```

---

### 5. `manager.go` — El Punto de Entrada

**Propósito**: API pública que usan los handlers/services.

**Interfaces principales**:

```go
// Punto de entrada principal
type DataAccessLayer interface {
    ForTenant(ctx, slugOrID) (TenantDataAccess, error)
    ConfigAccess() ConfigAccess
    Mode() OperationalMode
    Close() error
}

// Acceso a datos de un tenant específico
type TenantDataAccess interface {
    // Identificación
    Slug() string
    ID() string
    
    // Data plane (requieren DB)
    Users() repository.UserRepository
    Tokens() repository.TokenRepository
    MFA() repository.MFARepository
    Consents() repository.ConsentRepository
    RBAC() repository.RBACRepository
    
    // Control plane (siempre disponible)
    Clients() repository.ClientRepository
    Scopes() repository.ScopeRepository
    
    // Infraestructura
    Cache() cache.Client
    
    // Helpers
    HasDB() bool     // ¿Tiene DB?
    RequireDB() error // Error si no hay DB
}

// Acceso a configuración global
type ConfigAccess interface {
    Tenants() repository.TenantRepository
    Clients(tenantSlug) repository.ClientRepository
    Scopes(tenantSlug) repository.ScopeRepository
}
```

---

### 6. `migrate.go` — Migraciones SQL

**Propósito**: Aplicar cambios de esquema a las bases de datos.

**¿Cómo funciona?**

```
migrations/
├── 0001_init.sql          # CREATE TABLE users...
├── 0002_add_mfa.sql       # ALTER TABLE...
└── 0003_add_rbac.sql      # CREATE TABLE roles...

Al conectar a una DB:
1. Verificar tabla _migrations
2. Ver qué migraciones faltan
3. Aplicarlas en orden
4. Registrar en _migrations
```

---

### 7. `errors.go` — Errores Comunes

```go
var (
    ErrTenantNotFound = errors.New("tenant not found")
    ErrNoDBForTenant  = errors.New("no database configured for tenant")
    ErrNotLeader      = errors.New("operation requires cluster leader")
)
```

---

## Adapters

### `adapters/pg/` — PostgreSQL

Implementa todos los repositorios usando **pgxpool** (driver nativo de Go):

| Archivo | Repositorios |
|---------|--------------|
| `adapter.go` | pgConnection, UserRepository, TokenRepository |
| `repos.go` | MFARepository, ConsentRepository, ScopeRepository, RBACRepository |

**Total: ~38 métodos**

---

### `adapters/fs/` — FileSystem

Lee y escribe archivos YAML en disco:

```
data/tenants/
├── acme/
│   ├── tenant.yaml       # Configuración del tenant
│   ├── clients.yaml      # Clientes OAuth
│   └── scopes.yaml       # Scopes custom
└── demo/
    ├── tenant.yaml
    ├── clients.yaml
    └── scopes.yaml
```

**Repositorios**: TenantRepository, ClientRepository, ScopeRepository

**Total: ~22 métodos**

---

### `adapters/noop/` — No-Operation

Retorna `ErrNoDatabase` para todas las operaciones. Útil como fallback cuando no hay DB configurada.

---

## Flujo Completo: Request → Datos

```
┌─────────────────────────────────────────────────────────────────┐
│                      HANDLER / SERVICE                          │
│                                                                 │
│  func GetUser(ctx, tenantSlug, email string) (*User, error) {  │
│                                                                 │
│      // 1. Obtener acceso al tenant                            │
│      tda, err := dal.ForTenant(ctx, tenantSlug)                │
│      if err != nil {                                           │
│          return nil, err  // Tenant no existe                  │
│      }                                                         │
│                                                                 │
│      // 2. Verificar que tenga DB                              │
│      if err := tda.RequireDB(); err != nil {                   │
│          return nil, err  // No hay DB                         │
│      }                                                         │
│                                                                 │
│      // 3. Usar el repositorio                                 │
│      user, _, err := tda.Users().GetByEmail(ctx, tenantSlug, email)
│      return user, err                                          │
│  }                                                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         MANAGER                                 │
│                                                                 │
│  ForTenant("acme"):                                            │
│    1. Buscar en cache de TenantDataAccess                      │
│    2. Si no existe, delegar a Factory.ForTenant()              │
│    3. Cachear resultado                                        │
│    4. Retornar TenantDataAccess                                │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                         FACTORY                                 │
│                                                                 │
│  ForTenant("acme"):                                            │
│    1. resolveTenant() → Buscar en FS                           │
│    2. getDataConnection() → Obtener/crear conexión del pool    │
│    3. getCache() → Crear cache client                          │
│    4. Retornar tenantAccess{}                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                      CONNECTION POOL                            │
│                                                                 │
│  Get("acme", config):                                          │
│    1. ¿Existe conexión? → Retornarla                           │
│    2. Singleflight → Evitar duplicados                         │
│    3. OpenAdapter() → Crear conexión nueva                     │
│    4. Guardar en pool                                          │
│    5. Retornar conexión                                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    ADAPTER (pg, fs, etc)                        │
│                                                                 │
│  Connect(config):                                              │
│    1. Parsear DSN                                              │
│    2. Crear pool de conexiones (pgxpool)                       │
│    3. Ping para verificar                                      │
│    4. Retornar pgConnection                                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    REPOSITORY (userRepo)                        │
│                                                                 │
│  GetByEmail(ctx, tenantID, email):                             │
│    1. Ejecutar query SQL                                       │
│    2. Mapear resultado a repository.User                       │
│    3. Retornar                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## Ejemplos de Uso

### Inicialización Básica

```go
package main

import (
    "context"
    
    store "github.com/dropDatabas3/hellojohn/internal/store/v2"
    _ "github.com/dropDatabas3/hellojohn/internal/store/v2/adapters/dal"
)

func main() {
    ctx := context.Background()
    
    // Crear manager
    mgr, err := store.NewManager(ctx, store.ManagerConfig{
        FSRoot: "./data",
    })
    if err != nil {
        panic(err)
    }
    defer mgr.Close()
    
    // Verificar modo
    fmt.Println("Modo:", mgr.Mode()) // fs_only
}
```

### Con Base de Datos

```go
mgr, _ := store.NewManager(ctx, store.ManagerConfig{
    FSRoot: "./data",
    DefaultTenantDB: &store.DBConfig{
        Driver: "postgres",
        DSN:    "postgres://user:pass@localhost:5432/app?sslmode=disable",
    },
})
// Modo: fs_tenant_db
```

### Acceso a Control Plane

```go
// Listar todos los tenants
tenants, err := mgr.ConfigAccess().Tenants().List(ctx)

// Listar clientes de un tenant
clients, err := mgr.ConfigAccess().Clients("acme").List(ctx, "acme", "")

// Crear un scope
scope, err := mgr.ConfigAccess().Scopes("acme").Create(ctx, "acme", "billing", "Access to billing")
```

### Acceso a Data Plane

```go
// Obtener acceso al tenant
tda, err := mgr.ForTenant(ctx, "acme")
if err != nil {
    // Tenant no existe
}

// Verificar que tenga DB
if !tda.HasDB() {
    return errors.New("este tenant no tiene base de datos")
}

// Crear usuario
user, identity, err := tda.Users().Create(ctx, repository.CreateUserInput{
    TenantID:     "acme",
    Email:        "user@example.com",
    PasswordHash: hashedPassword,
})

// Buscar usuario
user, identity, err := tda.Users().GetByEmail(ctx, "acme", "user@example.com")

// Verificar contraseña
if tda.Users().CheckPassword(identity.PasswordHash, "password123") {
    // OK
}

// Crear refresh token
tokenID, err := tda.Tokens().Create(ctx, repository.CreateRefreshTokenInput{
    TenantID:   "acme",
    UserID:     user.ID,
    ClientID:   "my-app",
    TokenHash:  hash,
    TTLSeconds: 86400,
})

// Usar cache
tda.Cache().Set(ctx, "session:123", sessionData, 30*time.Minute)
data, _ := tda.Cache().Get(ctx, "session:123")
```

---

## Agregar un Nuevo Adapter

1. Crear directorio `adapters/cassandra/`

2. Crear `adapter.go`:

```go
package cassandra

import (
    "context"
    store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

func init() {
    store.RegisterAdapter(&cassandraAdapter{})
}

type cassandraAdapter struct{}

func (a *cassandraAdapter) Name() string { return "cassandra" }

func (a *cassandraAdapter) Connect(ctx context.Context, cfg store.AdapterConfig) (store.AdapterConnection, error) {
    // Conectar a Cassandra
    session, err := gocql.NewSession(...)
    if err != nil {
        return nil, err
    }
    return &cassandraConnection{session: session}, nil
}

type cassandraConnection struct {
    session *gocql.Session
}

func (c *cassandraConnection) Name() string { return "cassandra" }
func (c *cassandraConnection) Ping(ctx context.Context) error { ... }
func (c *cassandraConnection) Close() error { return c.session.Close() }

// Implementar repositorios...
func (c *cassandraConnection) Users() repository.UserRepository { return &cassandraUserRepo{...} }
// etc...
```

3. Agregar import en `adapters/dal/all.go`:

```go
_ "github.com/dropDatabas3/hellojohn/internal/store/v2/adapters/cassandra"
```

---

## Resumen

| Componente | Responsabilidad |
|------------|-----------------|
| **mode.go** | Detectar qué infraestructura hay disponible |
| **registry.go** | Registrar y buscar adapters de DB |
| **pool.go** | Reutilizar conexiones entre requests |
| **factory.go** | Armar todo el sistema |
| **manager.go** | API pública para handlers |
| **migrate.go** | Actualizar esquemas de DB |
| **adapters/** | Implementaciones concretas (PG, FS, etc) |
