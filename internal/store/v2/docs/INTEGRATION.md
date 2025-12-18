# Integración Profesional de Store V2

Para lograr una arquitectura "prolija, simple y profesional", NO se recomienda instanciar todos los tenants al inicio (si tienes 10,000 tenants, el startup sería lentísimo).

El patrón estándar en Go para aplicaciones multi-tenant es **Resolución por Request (Lazy Loading)** con inyección en Contexto.

## 1. El Flujo Ideal

1.  **Global**: `store.Manager` se inicializa una sola vez en el `main.go`.
2.  **Request**: Llega una petición HTTP.
3.  **Middleware**: Detecta el tenant (subdominio/header), pide al Manager el acceso, y lo guarda en el `context`.
4.  **Handler**: Saca el acceso del `context` y lo usa.

Este enfoque es escalable (carga tenants bajo demanda) y limpio (los handlers no saben de managers, solo reciben sus datos).

---

## 2. Implementación Paso a Paso

### A. Inicialización en `main.go`

```go
func main() {
    // 1. Iniciar Manager (Singleton)
    storeMgr, err := store.NewManager(ctx, store.ManagerConfig{FSRoot: "./data"})
    if err != nil { panic(err) }
    
    // 2. Inyectar en Server
    srv := server.New(storeMgr)
    srv.Start()
}
```

### B. Middleware de Tenant (La Clave)

Este middleware intercepta cada request, resuelve el tenant y prepara el "escenario" para el handler.

```go
// internal/http/middleware/tenant.go

type TenantMiddleware struct {
    manager *store.Manager
}

func (m *TenantMiddleware) Handle(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // 1. Detectar slug (ej: header "X-Tenant-ID" o subdominio)
        slug := r.Header.Get("X-Tenant-ID")
        
        // 2. Obtener Data Access del Manager (Cacheado internamente)
        // Esto es muy rápido (microsegundos si ya está en cache)
        tda, err := m.manager.ForTenant(r.Context(), slug)
        if err != nil {
            http.Error(w, "Tenant not found", http.StatusNotFound)
            return
        }
        
        // 3. Inyectar en el Contexto del Request
        ctx := context.WithValue(r.Context(), tenantKey, tda)
        
        // 4. Pasar al siguiente handler
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### C. Helper para Handlers

Para que el código de los handlers quede limpio:

```go
// internal/http/context.go

// GetTenant devuelve el DataAccess del contexto
func GetTenant(ctx context.Context) (store.TenantDataAccess, error) {
    tda, ok := ctx.Value(tenantKey).(store.TenantDataAccess)
    if !ok {
        return nil, errors.New("no tenant in context")
    }
    return tda, nil
}
```

### D. Uso en un Service/Handler

Fíjate lo limpio que queda el handler. No se preocupa por conexiones, pools, o configs. Solo pide "Usuarios".

```go
// internal/http/handlers/users.go

func (h *UserHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
    // 1. Obtener acceso al tenant actual
    tenant, err := httpContext.GetTenant(r.Context())
    if err != nil {
        // Manejar error 500
        return
    }

    // 2. Usar lógica de negocio (Store V2)
    // El tenant ya viene configurado con SU base de datos y SU cache
    user, _, err := tenant.Users().Create(r.Context(), repository.CreateUserInput{
        Email: "jane@doe.com",
    })
    
    // ...
}
```

---

## 3. ¿Por qué es mejor esto que instanciar todo al inicio?

1.  **Escalabilidad Infinita**: Si tienes 1 tenant o 1 millón, el `main.go` arranca igual de rápido. Los tenants se cargan en memoria solo cuando reciben tráfico.
2.  **Eficiencia de Recursos**: No mantienes conexiones abiertas a bases de datos de tenants inactivos. El `pool.go` del Store V2 cierra conexiones ociosas automáticamente.
3.  **Hot Reloading**: Si cambias la configuración de un tenant (YAML), el Manager puede invalidar su cache y recargarla en el siguiente request sin reiniciar el servidor.

## Resumen

La "forma profesional" es usar el `store.Manager` como un **Factory con Cache** detrás de un middleware.

- **Request** -> **Middleware** (Resuelve Tenant) -> **Context** -> **Handler** (Usa Repositorios)
