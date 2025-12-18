# Store V2 — Data Layer Multi-Driver

## Modos de Operación

| Modo | FS | DB Global | DB Tenant | Uso |
|------|----|----|-----|-----|
| 1 - FSOnly | ✅ | ❌ | ❌ | Solo config (dev/testing) |
| 2 - FSGlobalDB | ✅ | ✅ | ❌ | Config en DB global |
| 3 - FSTenantDB | ✅ | ❌ | ✅ | User data por tenant |
| 4 - FullDB | ✅ | ✅ | ✅ | Máximo (enterprise) |

## Estructura

```
store/v2/
├── mode.go        # OperationalMode, DetectMode(), ModeCapabilities
├── pool.go        # ConnectionPool con singleflight, health checks
├── migrate.go     # Migrator con embed.FS
├── registry.go    # Adapter registry (Register/Get/Open)
├── factory.go     # Factory que ensambla el DAL
├── manager.go     # DataAccessLayer, TenantDataAccess, ConfigAccess
└── adapters/
    ├── pg/        # PostgreSQL (pgxpool) — 38 métodos
    ├── fs/        # FileSystem (YAML) — 22 métodos
    ├── mysql/     # Placeholder
    ├── mongo/     # Placeholder
    └── noop/      # Fallback — 38 métodos
```

## Uso Rápido

```go
import (
    "github.com/dropDatabas3/hellojohn/internal/store/v2"
    _ "github.com/dropDatabas3/hellojohn/internal/store/v2/adapters/pg"
    _ "github.com/dropDatabas3/hellojohn/internal/store/v2/adapters/fs"
)

// Crear manager
mgr, err := store.NewManager(ctx, store.ManagerConfig{
    FSRoot: "./data",
    DefaultTenantDB: &store.DBConfig{
        Driver: "postgres",
        DSN:    "postgres://...",
    },
})
defer mgr.Close()

// Control plane (siempre disponible)
tenants, _ := mgr.ConfigAccess().Tenants().List(ctx)
clients, _ := mgr.ConfigAccess().Clients("acme").List(ctx, "acme", "")

// Data plane (por tenant)
tda, err := mgr.ForTenant(ctx, "acme")
if err != nil {
    // Tenant no existe
}

user, _, _ := tda.Users().GetByEmail(ctx, "acme", "user@email.com")
token, _ := tda.Tokens().GetByHash(ctx, "hash...")
```

## Agregar Adapter

```go
// adapters/cassandra/adapter.go
func init() {
    store.RegisterAdapter(&cassandraAdapter{})
}

type cassandraAdapter struct{}

func (a *cassandraAdapter) Name() string { return "cassandra" }

func (a *cassandraAdapter) Connect(ctx context.Context, cfg store.AdapterConfig) (store.AdapterConnection, error) {
    // Implementar conexión
}
```
