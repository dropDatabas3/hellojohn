# Plan de Implementación: Sistema de Autenticación de Administradores

## 📋 Resumen Ejecutivo

**Objetivo**: Separar completamente la autenticación de administradores de la autenticación de usuarios, implementando un sistema basado en permisos granulares por tenant.

**Problema Actual**:
- Mezcla de lógica de admin y usuario en `/v2/auth/login`
- Admins necesitan proporcionar `tenant_id` y `client_id` innecesariamente
- No hay control granular de acceso por tenant

**Solución**:
- Endpoint dedicado `/v2/admin/login` para admins
- Claims JWT específicos con lista de tenants asignados
- Middleware de autorización que verifica acceso por tenant
- Admin Global (acceso total) vs Tenant Admin (acceso limitado)

---

## 🎯 Fases de Implementación

### **FASE 1: Backend - DTOs y Contratos** (30 min)
- [ ] Crear DTOs de request/response para admin login
- [ ] Definir estructura de AdminClaims para JWT
- [ ] Crear interfaces de servicio

### **FASE 2: Backend - Servicios** (1 hora)
- [ ] Implementar AdminAuthService (login, refresh)
- [ ] Extender Issuer para emitir tokens de admin
- [ ] Implementar verificación de permisos por tenant

### **FASE 3: Backend - Controllers** (30 min)
- [ ] Crear AdminAuthController
- [ ] Implementar handlers HTTP
- [ ] Mapear errores de servicio a HTTP

### **FASE 4: Backend - Router y Middleware** (45 min)
- [ ] Registrar rutas `/v2/admin/login`, `/v2/admin/refresh`
- [ ] Crear middleware `RequireAdminTenantAccess`
- [ ] Aplicar middleware en rutas admin existentes

### **FASE 5: Frontend - UI y API** (30 min)
- [ ] Agregar constantes de rutas admin
- [ ] Crear página de login admin (o actualizar existente)
- [ ] Actualizar auth store para manejar admin tokens

### **FASE 6: Limpieza** (30 min)
- [ ] Remover lógica temporal de `loginAsAdmin()` en auth/login
- [ ] Restaurar validación estricta en `/v2/auth/login`
- [ ] Limpiar código muerto

### **FASE 7: Testing y Documentación** (1 hora)
- [ ] Testing manual de flujos
- [ ] Documentación de API
- [ ] Actualizar guías de migración

**Tiempo Total Estimado**: 4-5 horas

---

## 📐 Arquitectura Detallada

### 1. Modelo de Datos

```
┌─────────────────────────────────────────────────────────────┐
│ ADMIN (Control Plane - FS)                                  │
├─────────────────────────────────────────────────────────────┤
│ id: UUID                                                    │
│ email: string                                               │
│ password_hash: string (argon2id)                            │
│ type: "global" | "tenant"                                   │
│ assigned_tenants: ["uuid1", "uuid2"] | null                │
│ created_at: timestamp                                       │
│ disabled_at: timestamp | null                               │
└─────────────────────────────────────────────────────────────┘
```

### 2. JWT Claims

```go
// Access Token Claims
{
  "sub": "admin-uuid-123",           // Admin ID
  "email": "admin@example.com",
  "admin_type": "global",            // "global" | "tenant"
  "tenants": ["uuid1", "uuid2"],     // null para global
  "iss": "http://localhost:8080",
  "aud": "hellojohn:admin",
  "exp": 1738512000,
  "iat": 1738508400
}

// Refresh Token Claims
{
  "sub": "admin-uuid-123",
  "type": "admin_refresh",
  "iss": "http://localhost:8080",
  "aud": "hellojohn:admin",
  "exp": 1741100400
}
```

### 3. Endpoints

```
POST /v2/admin/login
  Request:  { email, password }
  Response: { access_token, refresh_token, expires_in, admin: {...} }

POST /v2/admin/refresh
  Request:  { refresh_token }
  Response: { access_token, refresh_token, expires_in }

POST /v2/admin/logout
  Request:  { refresh_token? }
  Response: { success }
```

### 4. Middleware Chain

```
Ruta Admin → RequireAuth → RequireAdmin → RequireAdminTenantAccess → Handler
              ↓              ↓              ↓
              JWT válido     aud=admin      tiene acceso al tenant
```

---

## 📁 Estructura de Archivos

### Archivos a Crear

```
internal/http/v2/
├── dto/admin/
│   └── auth.go                        ← AdminLoginRequest, AdminLoginResult
├── services/admin/
│   ├── auth_service.go                ← AdminAuthService interface + impl
│   └── contracts.go                   ← Interfaces (si no existe)
├── controllers/admin/
│   └── auth_controller.go             ← AdminAuthController
├── router/
│   └── admin_auth_routes.go           ← Rutas de admin auth
└── middlewares/
    └── admin.go                        ← RequireAdminTenantAccess (actualizar)

internal/jwt/
└── admin_claims.go                     ← AdminAccessClaims, métodos del Issuer

ui/lib/
└── routes.ts                           ← Agregar ADMIN_LOGIN, ADMIN_REFRESH
```

### Archivos a Modificar

```
internal/http/v2/services/admin/services.go     ← Agregar Auth service
internal/http/v2/controllers/admin/controllers.go ← Agregar Auth controller
internal/http/v2/router/router.go               ← RegisterAdminAuthRoutes
internal/http/v2/services/auth/login_service.go ← Limpiar loginAsAdmin()
internal/jwt/issuer.go                          ← Métodos admin
```

---

## 🔧 Detalles de Implementación

### FASE 1: DTOs y Contratos

#### 1.1. `internal/http/v2/dto/admin/auth.go`

```go
package admin

// AdminLoginRequest es el request para login de admin
type AdminLoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

// AdminLoginResult es la respuesta de login exitoso
type AdminLoginResult struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    ExpiresIn    int       `json:"expires_in"`
    TokenType    string    `json:"token_type"`
    Admin        AdminInfo `json:"admin"`
}

// AdminInfo contiene información del admin autenticado
type AdminInfo struct {
    ID      string   `json:"id"`
    Email   string   `json:"email"`
    Type    string   `json:"type"` // "global" | "tenant"
    Tenants []string `json:"tenants,omitempty"`
}

// AdminRefreshRequest es el request para refresh de token
type AdminRefreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}
```

#### 1.2. `internal/jwt/admin_claims.go`

```go
package jwtx

// AdminAccessClaims son los claims del access token de admin
type AdminAccessClaims struct {
    AdminID   string   `json:"sub"`
    Email     string   `json:"email"`
    AdminType string   `json:"admin_type"` // "global" | "tenant"
    Tenants   []string `json:"tenants,omitempty"`
}

// AdminRefreshClaims son los claims del refresh token de admin
type AdminRefreshClaims struct {
    AdminID string `json:"sub"`
    Type    string `json:"type"` // "admin_refresh"
}
```

#### 1.3. `internal/http/v2/services/admin/contracts.go`

```go
package admin

import (
    "context"
    dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/admin"
)

// AuthService maneja la autenticación de administradores
type AuthService interface {
    Login(ctx context.Context, req dto.AdminLoginRequest) (*dto.AdminLoginResult, error)
    Refresh(ctx context.Context, refreshToken string) (*dto.AdminLoginResult, error)
}
```

### FASE 2: Servicios

#### 2.1. `internal/http/v2/services/admin/auth_service.go`

```go
package admin

import (
    "context"
    "fmt"
    "strings"
    "time"

    "github.com/dropDatabas3/hellojohn/internal/domain/repository"
    dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/admin"
    jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
    store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// AuthDeps son las dependencias del servicio de auth admin
type AuthDeps struct {
    DAL            store.DataAccessLayer
    Issuer         *jwtx.Issuer
    AccessTokenTTL time.Duration // Default: 1 hora
    RefreshTTL     time.Duration // Default: 30 días
}

type authService struct {
    deps AuthDeps
}

func NewAuthService(deps AuthDeps) AuthService {
    // Defaults
    if deps.AccessTokenTTL == 0 {
        deps.AccessTokenTTL = 1 * time.Hour
    }
    if deps.RefreshTTL == 0 {
        deps.RefreshTTL = 30 * 24 * time.Hour
    }
    return &authService{deps: deps}
}

var (
    ErrInvalidCredentials = fmt.Errorf("invalid credentials")
    ErrAdminDisabled      = fmt.Errorf("admin account disabled")
    ErrInvalidRefreshToken = fmt.Errorf("invalid refresh token")
)

func (s *authService) Login(ctx context.Context, req dto.AdminLoginRequest) (*dto.AdminLoginResult, error) {
    // 1. Normalización
    email := strings.TrimSpace(strings.ToLower(req.Email))
    if email == "" || req.Password == "" {
        return nil, fmt.Errorf("email and password required")
    }

    // 2. Obtener AdminRepository del Control Plane
    adminRepo := s.deps.DAL.ConfigAccess().Admins()
    if adminRepo == nil {
        return nil, fmt.Errorf("admin repository not available")
    }

    // 3. Buscar admin por email
    admin, err := adminRepo.GetByEmail(ctx, email)
    if err != nil {
        if repository.IsNotFound(err) {
            return nil, ErrInvalidCredentials
        }
        return nil, err
    }

    // 4. Verificar password
    if !adminRepo.CheckPassword(admin.PasswordHash, req.Password) {
        return nil, ErrInvalidCredentials
    }

    // 5. Verificar que no esté deshabilitado
    if admin.DisabledAt != nil {
        return nil, ErrAdminDisabled
    }

    // 6. Actualizar last_seen
    _ = adminRepo.UpdateLastSeen(ctx, admin.ID)

    // 7. Emitir tokens
    accessToken, err := s.deps.Issuer.IssueAdminAccess(ctx, jwtx.AdminAccessClaims{
        AdminID:   admin.ID,
        Email:     admin.Email,
        AdminType: string(admin.Type),
        Tenants:   admin.AssignedTenants,
    }, s.deps.AccessTokenTTL)
    if err != nil {
        return nil, fmt.Errorf("failed to issue access token: %w", err)
    }

    refreshToken, err := s.deps.Issuer.IssueAdminRefresh(ctx, jwtx.AdminRefreshClaims{
        AdminID: admin.ID,
        Type:    "admin_refresh",
    }, s.deps.RefreshTTL)
    if err != nil {
        return nil, fmt.Errorf("failed to issue refresh token: %w", err)
    }

    return &dto.AdminLoginResult{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    int(s.deps.AccessTokenTTL.Seconds()),
        TokenType:    "Bearer",
        Admin: dto.AdminInfo{
            ID:      admin.ID,
            Email:   admin.Email,
            Type:    string(admin.Type),
            Tenants: admin.AssignedTenants,
        },
    }, nil
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*dto.AdminLoginResult, error) {
    // 1. Validar y parsear refresh token
    claims, err := s.deps.Issuer.ValidateAdminRefresh(ctx, refreshToken)
    if err != nil {
        return nil, ErrInvalidRefreshToken
    }

    // 2. Obtener admin actualizado
    adminRepo := s.deps.DAL.ConfigAccess().Admins()
    admin, err := adminRepo.GetByID(ctx, claims.AdminID)
    if err != nil {
        if repository.IsNotFound(err) {
            return nil, ErrInvalidRefreshToken
        }
        return nil, err
    }

    // 3. Verificar que no esté deshabilitado
    if admin.DisabledAt != nil {
        return nil, ErrAdminDisabled
    }

    // 4. Emitir nuevos tokens
    accessToken, err := s.deps.Issuer.IssueAdminAccess(ctx, jwtx.AdminAccessClaims{
        AdminID:   admin.ID,
        Email:     admin.Email,
        AdminType: string(admin.Type),
        Tenants:   admin.AssignedTenants,
    }, s.deps.AccessTokenTTL)
    if err != nil {
        return nil, fmt.Errorf("failed to issue access token: %w", err)
    }

    newRefreshToken, err := s.deps.Issuer.IssueAdminRefresh(ctx, jwtx.AdminRefreshClaims{
        AdminID: admin.ID,
        Type:    "admin_refresh",
    }, s.deps.RefreshTTL)
    if err != nil {
        return nil, fmt.Errorf("failed to issue refresh token: %w", err)
    }

    return &dto.AdminLoginResult{
        AccessToken:  accessToken,
        RefreshToken: newRefreshToken,
        ExpiresIn:    int(s.deps.AccessTokenTTL.Seconds()),
        TokenType:    "Bearer",
        Admin: dto.AdminInfo{
            ID:      admin.ID,
            Email:   admin.Email,
            Type:    string(admin.Type),
            Tenants: admin.AssignedTenants,
        },
    }, nil
}
```

#### 2.2. `internal/jwt/issuer.go` - Métodos Admin

```go
// Agregar estos métodos al Issuer existente

// IssueAdminAccess emite un access token para un admin
func (iss *Issuer) IssueAdminAccess(ctx context.Context, claims AdminAccessClaims, ttl time.Duration) (string, error) {
    now := time.Now()

    stdClaims := jwt.RegisteredClaims{
        Subject:   claims.AdminID,
        Issuer:    iss.baseURL,
        Audience:  jwt.ClaimStrings{"hellojohn:admin"},
        ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
        IssuedAt:  jwt.NewNumericDate(now),
    }

    // Combinar claims
    fullClaims := map[string]interface{}{
        "sub":        stdClaims.Subject,
        "iss":        stdClaims.Issuer,
        "aud":        stdClaims.Audience,
        "exp":        stdClaims.ExpiresAt.Unix(),
        "iat":        stdClaims.IssuedAt.Unix(),
        "email":      claims.Email,
        "admin_type": claims.AdminType,
    }

    if claims.Tenants != nil && len(claims.Tenants) > 0 {
        fullClaims["tenants"] = claims.Tenants
    }

    // Firmar con clave global
    key, err := iss.keystore.GetActiveKey(ctx, "") // "" = global
    if err != nil {
        return "", err
    }

    token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims(fullClaims))
    token.Header["kid"] = key.ID

    return token.SignedString(key.PrivateKey)
}

// IssueAdminRefresh emite un refresh token para un admin
func (iss *Issuer) IssueAdminRefresh(ctx context.Context, claims AdminRefreshClaims, ttl time.Duration) (string, error) {
    now := time.Now()

    stdClaims := jwt.RegisteredClaims{
        Subject:   claims.AdminID,
        Issuer:    iss.baseURL,
        Audience:  jwt.ClaimStrings{"hellojohn:admin"},
        ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
        IssuedAt:  jwt.NewNumericDate(now),
    }

    fullClaims := map[string]interface{}{
        "sub":  stdClaims.Subject,
        "iss":  stdClaims.Issuer,
        "aud":  stdClaims.Audience,
        "exp":  stdClaims.ExpiresAt.Unix(),
        "iat":  stdClaims.IssuedAt.Unix(),
        "type": claims.Type,
    }

    key, err := iss.keystore.GetActiveKey(ctx, "")
    if err != nil {
        return "", err
    }

    token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims(fullClaims))
    token.Header["kid"] = key.ID

    return token.SignedString(key.PrivateKey)
}

// ValidateAdminRefresh valida un refresh token de admin
func (iss *Issuer) ValidateAdminRefresh(ctx context.Context, tokenString string) (*AdminRefreshClaims, error) {
    token, err := iss.Parse(ctx, tokenString)
    if err != nil {
        return nil, err
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
        return nil, fmt.Errorf("invalid claims")
    }

    // Verificar audience
    aud, ok := claims["aud"].([]interface{})
    if !ok || len(aud) == 0 || aud[0].(string) != "hellojohn:admin" {
        return nil, fmt.Errorf("invalid audience")
    }

    // Verificar tipo
    typ, ok := claims["type"].(string)
    if !ok || typ != "admin_refresh" {
        return nil, fmt.Errorf("invalid token type")
    }

    sub, _ := claims["sub"].(string)

    return &AdminRefreshClaims{
        AdminID: sub,
        Type:    typ,
    }, nil
}
```

### FASE 3: Controllers

```go
// internal/http/v2/controllers/admin/auth_controller.go
package admin

import (
    "encoding/json"
    "net/http"

    dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/admin"
    httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
    svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/admin"
)

type AuthController struct {
    service svc.AuthService
}

func NewAuthController(service svc.AuthService) *AuthController {
    return &AuthController{service: service}
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    if r.Method != http.MethodPost {
        w.Header().Set("Allow", "POST")
        httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
        return
    }

    var req dto.AdminLoginRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httperrors.WriteError(w, httperrors.ErrInvalidJSON)
        return
    }

    result, err := c.service.Login(ctx, req)
    if err != nil {
        c.writeLoginError(w, err)
        return
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(result)
}

func (c *AuthController) Refresh(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    if r.Method != http.MethodPost {
        w.Header().Set("Allow", "POST")
        httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
        return
    }

    var req dto.AdminRefreshRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        httperrors.WriteError(w, httperrors.ErrInvalidJSON)
        return
    }

    result, err := c.service.Refresh(ctx, req.RefreshToken)
    if err != nil {
        c.writeRefreshError(w, err)
        return
    }

    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(result)
}

func (c *AuthController) writeLoginError(w http.ResponseWriter, err error) {
    switch err {
    case svc.ErrInvalidCredentials:
        httperrors.WriteError(w, httperrors.ErrUnauthorized.WithDetail("invalid credentials"))
    case svc.ErrAdminDisabled:
        httperrors.WriteError(w, httperrors.ErrForbidden.WithDetail("admin account disabled"))
    default:
        httperrors.WriteError(w, httperrors.ErrInternalServer)
    }
}

func (c *AuthController) writeRefreshError(w http.ResponseWriter, err error) {
    switch err {
    case svc.ErrInvalidRefreshToken:
        httperrors.WriteError(w, httperrors.ErrUnauthorized.WithDetail("invalid refresh token"))
    case svc.ErrAdminDisabled:
        httperrors.WriteError(w, httperrors.ErrForbidden.WithDetail("admin account disabled"))
    default:
        httperrors.WriteError(w, httperrors.ErrInternalServer)
    }
}
```

### FASE 4: Router y Middleware

#### 4.1. Router

```go
// internal/http/v2/router/admin_auth_routes.go
package router

import (
    "net/http"

    ctrl "github.com/dropDatabas3/hellojohn/internal/http/v2/controllers/admin"
    mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
)

type AdminAuthRouterDeps struct {
    AuthController *ctrl.AuthController
    RateLimiter    mw.RateLimiter
}

func RegisterAdminAuthRoutes(mux *http.ServeMux, deps AdminAuthRouterDeps) {
    c := deps.AuthController

    // POST /v2/admin/login
    mux.Handle("/v2/admin/login", adminAuthHandler(deps.RateLimiter, http.HandlerFunc(c.Login)))

    // POST /v2/admin/refresh
    mux.Handle("/v2/admin/refresh", adminAuthHandler(deps.RateLimiter, http.HandlerFunc(c.Refresh)))
}

func adminAuthHandler(limiter mw.RateLimiter, handler http.Handler) http.Handler {
    chain := []mw.Middleware{
        mw.WithRecover(),
        mw.WithRequestID(),
        mw.WithSecurityHeaders(),
        mw.WithNoStore(),
    }

    if limiter != nil {
        chain = append(chain, mw.WithRateLimit(mw.RateLimitConfig{
            Limiter: limiter,
            KeyFunc: mw.IPOnlyRateKey, // Más estricto para admin
        }))
    }

    chain = append(chain, mw.WithLogging())

    return mw.Chain(handler, chain...)
}
```

#### 4.2. Middleware de Autorización

```go
// internal/http/v2/middlewares/admin.go - Agregar esta función

// RequireAdminTenantAccess verifica que el admin tenga acceso al tenant del contexto
func RequireAdminTenantAccess() Middleware {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            ctx := r.Context()

            // Obtener claims de admin del contexto (ya validados por RequireAuth)
            claims := GetAdminClaimsFromContext(ctx)
            if claims == nil {
                http.Error(w, "Unauthorized: admin claims not found", 401)
                return
            }

            // Obtener tenant del contexto (ya resuelto por WithTenantResolution)
            tenantID := GetTenantIDFromContext(ctx)
            if tenantID == "" {
                // Si no hay tenant en contexto, permitir (ej: listado global de tenants)
                next.ServeHTTP(w, r)
                return
            }

            // Admin global: acceso total
            if claims.AdminType == "global" {
                next.ServeHTTP(w, r)
                return
            }

            // Tenant admin: verificar que tenga el tenant asignado
            if !contains(claims.Tenants, tenantID) {
                http.Error(w, "Forbidden: no access to this tenant", 403)
                return
            }

            next.ServeHTTP(w, r)
        })
    }
}

func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}
```

---

## ✅ Criterios de Éxito

1. **Endpoints funcionando**:
   - `POST /v2/admin/login` retorna tokens válidos
   - `POST /v2/admin/refresh` renueva tokens correctamente

2. **Autorización correcta**:
   - Admin global puede acceder a todos los tenants
   - Tenant admin solo puede acceder a sus tenants asignados
   - Tenant admin recibe 403 al intentar acceder a tenant no asignado

3. **UI funcional**:
   - Página de login admin funciona
   - Tokens se almacenan correctamente
   - Dashboard carga con permisos correctos

4. **Limpieza completa**:
   - Código temporal removido
   - `/v2/auth/login` valida tenant_id y client_id estrictamente
   - Sin referencias a lógica antigua

---

## 📊 Métricas de Validación

- [ ] Compilación exitosa sin errores
- [ ] Admin global puede listar todos los tenants
- [ ] Tenant admin solo ve sus tenants
- [ ] 403 al intentar acceder a tenant no permitido
- [ ] Refresh token funciona correctamente
- [ ] UI admin login funcional
- [ ] Sin código temporal o comentarios TODO

---

**Inicio de Implementación**: A continuación
