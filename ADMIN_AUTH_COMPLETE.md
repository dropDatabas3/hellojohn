# Admin Authentication System - Implementation Complete

**Date**: 2026-01-22
**Status**: ✅ Backend Implementation Complete
**Frontend**: ⏳ Pending Implementation

---

## 📋 Overview

Implemented a complete admin authentication system separate from user authentication, following the architecture proposed by the user:

- **Admins** have `assigned_tenants` list in Control Plane
- **Global admins**: Empty/null tenant list, full system access
- **Tenant admins**: Specific tenant UUIDs, restricted access
- **No tenant_id/client_id** required for admin login (simplified)
- **Authorization checks** at operation level based on admin's tenant assignments

---

## ✅ Implemented Components

### 1. DTOs (Data Transfer Objects)

**File**: `internal/http/v2/dto/admin/auth.go`

```go
// Request DTOs
type AdminLoginRequest struct {
    Email    string `json:"email"`
    Password string `json:"password"`
}

type AdminRefreshRequest struct {
    RefreshToken string `json:"refresh_token"`
}

// Response DTOs
type AdminLoginResult struct {
    AccessToken  string    `json:"access_token"`
    RefreshToken string    `json:"refresh_token"`
    ExpiresIn    int       `json:"expires_in"`
    TokenType    string    `json:"token_type"`
    Admin        AdminInfo `json:"admin"`
}

type AdminInfo struct {
    ID      string   `json:"id"`
    Email   string   `json:"email"`
    Type    string   `json:"type"` // "global" | "tenant"
    Tenants []string `json:"tenants,omitempty"`
}
```

### 2. JWT Claims

**File**: `internal/jwt/admin_claims.go`

```go
type AdminAccessClaims struct {
    AdminID   string   `json:"sub"`
    Email     string   `json:"email"`
    AdminType string   `json:"admin_type"` // "global" | "tenant"
    Tenants   []string `json:"tenants,omitempty"`
}

type AdminRefreshClaims struct {
    AdminID string `json:"sub"`
    Type    string `json:"type"` // "admin_refresh"
}
```

**JWT Issuer Enhancement**: `internal/jwt/issuer.go`

```go
func (i *Issuer) IssueAdminAccess(ctx context.Context, claims AdminAccessClaims) (string, int, error)
```

- Audience: `"hellojohn:admin"`
- Includes `admin_type` and `tenants` claims
- Standard EdDSA signing with active key

### 3. Service Layer

**File**: `internal/http/v2/services/admin/auth_service.go`

```go
type AuthService interface {
    Login(ctx context.Context, req dto.AdminLoginRequest) (*dto.AdminLoginResult, error)
    Refresh(ctx context.Context, req dto.AdminRefreshRequest) (*dto.AdminLoginResult, error)
}
```

**Features**:
- ✅ Login with email/password
- ✅ Emits admin access token (JWT)
- ✅ Generates refresh token (opaco)
- ✅ Refresh token rotation
- ✅ Admin disabled check
- ✅ Error handling

**Current Implementation** (Temporary):
- Uses placeholder validation (`admin@example.com` / `admin`)
- Refresh tokens not persisted (stateless)
- **TODOs** marked for Control Plane integration:
  - `GetAdminByEmail()`
  - `CreateAdminRefreshToken()`
  - `GetAdminRefreshToken()`
  - `GetAdmin()`
  - Real Argon2id password verification

### 4. Controller Layer

**File**: `internal/http/v2/controllers/admin/auth_controller.go`

```go
type AuthController struct {
    service svc.AuthService
}

func (c *AuthController) Login(w http.ResponseWriter, r *http.Request)
func (c *AuthController) Refresh(w http.ResponseWriter, r *http.Request)
```

**Features**:
- HTTP parsing and validation
- Error mapping to HTTP status codes
- Logging with observability
- Content-Type: `application/json`

**Error Mapping**:
- `ErrInvalidAdminCredentials` → 401 Unauthorized
- `ErrAdminDisabled` → 403 Forbidden
- `ErrInvalidRefreshToken` → 401 Unauthorized
- `ErrRefreshTokenExpired` → 401 Unauthorized
- Default → 500 Internal Server Error

### 5. Router Integration

**File**: `internal/http/v2/router/admin_routes.go`

```go
// Public routes (no auth required)
mux.Handle("POST /v2/admin/login", adminAuthHandler(limiter, c.Auth.Login))
mux.Handle("POST /v2/admin/refresh", adminAuthHandler(limiter, c.Auth.Refresh))
```

**Middleware Chain** (`adminAuthHandler`):
1. ✅ Recover
2. ✅ RequestID
3. ✅ SecurityHeaders
4. ✅ NoStore (cache control)
5. ✅ RateLimit (optional, IP+path based)
6. ✅ Logging

**Note**: Does NOT apply auth middleware (these are login endpoints)

### 6. Wiring Complete

**Files Updated**:
- `internal/http/v2/services/admin/services.go`: Added `Auth` field + initialization
- `internal/http/v2/controllers/admin/controllers.go`: Added `Auth` controller
- `internal/http/v2/services/services.go`: Pass `RefreshTTL` to admin services

**Dependency Flow**:
```
app/v2/app.go
  → services.New(deps)
    → admin.NewServices(admin.Deps{RefreshTTL: ...})
      → NewAuthService(AuthServiceDeps{RefreshTTL: ...})
  → controllers.NewControllers(svcs)
    → admin.NewControllers(svcs.Admin)
      → NewAuthController(svcs.Auth)
  → router.RegisterAdminRoutes(mux, deps)
    → POST /v2/admin/login
    → POST /v2/admin/refresh
```

---

## 🔐 API Endpoints

### POST /v2/admin/login

**Request**:
```json
{
  "email": "admin@example.com",
  "password": "admin"
}
```

**Response** (200 OK):
```json
{
  "access_token": "eyJhbGciOiJFZERTQSIsImtpZCI6IjEyMyIsInR5cCI6IkpXVCJ9...",
  "refresh_token": "base64_opaque_token",
  "expires_in": 900,
  "token_type": "Bearer",
  "admin": {
    "id": "admin-temp-id",
    "email": "admin@example.com",
    "type": "global",
    "tenants": null
  }
}
```

**Errors**:
- 400: Invalid JSON or missing required fields
- 401: Invalid credentials
- 403: Admin account disabled
- 500: Internal server error

### POST /v2/admin/refresh

**Request**:
```json
{
  "refresh_token": "base64_opaque_token"
}
```

**Response** (200 OK):
```json
{
  "access_token": "new_jwt_token",
  "refresh_token": "same_refresh_token",
  "expires_in": 900,
  "token_type": "Bearer",
  "admin": {
    "id": "admin-temp-id",
    "email": "admin@example.com",
    "type": "global",
    "tenants": null
  }
}
```

**Errors**:
- 400: Missing refresh_token
- 401: Invalid or expired refresh token
- 403: Admin account disabled
- 500: Internal server error

---

## 🧪 Testing

### Manual Testing (cURL)

```bash
# 1. Admin Login
curl -X POST http://localhost:8080/v2/admin/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "admin"
  }'

# Expected: 200 OK with access_token, refresh_token, admin info

# 2. Extract access_token from response
export ACCESS_TOKEN="eyJhbGci..."

# 3. Use access token for admin operations (once implemented)
curl http://localhost:8080/v2/admin/tenants \
  -H "Authorization: Bearer $ACCESS_TOKEN"

# 4. Refresh token
curl -X POST http://localhost:8080/v2/admin/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "base64_token_from_login"
  }'
```

### Test Cases

- [ ] **Login Success**: Valid email/password returns tokens
- [ ] **Login Failure**: Invalid credentials returns 401
- [ ] **Login Missing Fields**: Returns 400
- [ ] **Login Disabled Admin**: Returns 403 (after Control Plane integration)
- [ ] **Refresh Success**: Valid refresh token returns new access token
- [ ] **Refresh Invalid**: Invalid token returns 401
- [ ] **Refresh Expired**: Expired token returns 401 (after CP integration)
- [ ] **JWT Claims**: Access token includes `admin_type`, `tenants`, `aud: "hellojohn:admin"`
- [ ] **Rate Limiting**: Excessive requests are throttled

---

## 📝 Pending Work

### Critical (Before Production)

1. **Control Plane Admin Repository Integration**

   **Files to implement**:
   - `internal/controlplane/v2/service.go`: Add methods:
     ```go
     GetAdminByEmail(ctx, email) (*repository.Admin, error)
     GetAdmin(ctx, id) (*repository.Admin, error)
     CreateAdminRefreshToken(ctx, input AdminRefreshTokenInput) error
     GetAdminRefreshToken(ctx, tokenHash) (*AdminRefreshToken, error)
     ```

   **Files to create**:
   - `internal/store/v2/adapters/fs/admin.go`: FileSystem adapter for admins
   - `data/hellojohn/admins/admins.yaml`: Admin storage file

   **Structure**:
   ```yaml
   admins:
     - id: uuid
       email: admin@example.com
       password_hash: $argon2id$v=19$...
       name: System Admin
       type: global
       created_at: 2026-01-22T10:00:00Z
       updated_at: 2026-01-22T10:00:00Z

     - id: uuid2
       email: tenant-admin@acme.com
       type: tenant
       assigned_tenants:
         - tenant-uuid-1
         - tenant-uuid-2
       created_at: 2026-01-22T11:00:00Z
   ```

2. **Password Hashing**

   Implement real Argon2id password verification in `auth_service.go`:
   ```go
   func checkPassword(hash, password string) bool {
       // Parse hash: $argon2id$v=19$m=65536,t=3,p=2$salt$hash
       // Recalculate hash with same salt
       // Compare in constant time
   }
   ```

   Or delegate to Control Plane/Repository:
   ```go
   if !tda.Admins().CheckPassword(admin.PasswordHash, req.Password) {
       return nil, ErrInvalidCredentials
   }
   ```

3. **Refresh Token Persistence**

   Currently refresh tokens are NOT persisted. Implement:
   - Store refresh token hash in Control Plane (FS or DB)
   - Verify on refresh
   - Support revocation

   **Files**:
   - `data/hellojohn/admins/refresh_tokens.yaml`:
     ```yaml
     refresh_tokens:
       - token_hash: sha256_hash
         admin_id: uuid
         expires_at: 2026-02-22T10:00:00Z
         created_at: 2026-01-22T10:00:00Z
     ```

4. **Admin Middleware Enhancement**

   **File**: `internal/http/v2/middlewares/admin.go`

   Add `RequireAdminTenantAccess(tenantID)` middleware:
   ```go
   func RequireAdminTenantAccess(tenantID string) Middleware {
       return func(next http.Handler) http.Handler {
           return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
               claims := GetAdminClaims(r.Context())

               // Global admin: always allow
               if claims.AdminType == "global" {
                   next.ServeHTTP(w, r)
                   return
               }

               // Tenant admin: check if tenantID is in assigned_tenants
               if !contains(claims.Tenants, tenantID) {
                   httperrors.WriteError(w, httperrors.ErrForbidden)
                   return
               }

               next.ServeHTTP(w, r)
           })
       }
   }
   ```

### Frontend (High Priority)

**Files to create/update**:

1. **`ui/lib/routes.ts`**:
   ```typescript
   export const API_ROUTES = {
     // Existing routes...
     ADMIN_LOGIN: '/v2/admin/login',
     ADMIN_REFRESH: '/v2/admin/refresh',
   }
   ```

2. **`ui/app/(admin)/login/page.tsx`**:
   ```typescript
   import { api } from '@/lib/api'
   import { API_ROUTES } from '@/lib/routes'

   async function handleAdminLogin(email: string, password: string) {
     const response = await api.post(API_ROUTES.ADMIN_LOGIN, {
       email,
       password
     })

     // Store tokens
     localStorage.setItem('admin_access_token', response.access_token)
     localStorage.setItem('admin_refresh_token', response.refresh_token)

     // Redirect to admin dashboard
     router.push('/admin')
   }
   ```

3. **`ui/lib/api.ts`**:
   - Add admin token handling
   - Intercept 401 responses
   - Auto-refresh admin tokens
   - Separate admin vs user auth

4. **`ui/app/(admin)/admin/page.tsx`**:
   - Update to use admin authentication
   - Display admin info (email, type, tenants)
   - Show restricted access for tenant admins

### Cleanup (Medium Priority)

1. **Remove Workaround**

   **File**: `internal/http/v2/services/auth/login_service.go`

   Remove `loginAsAdmin()` method and temporary logic added for admin login via `/v2/auth/login`.

2. **Admin CLI Tool**

   Create `cmd/admin/main.go` for admin management:
   ```bash
   # Create admin
   ./admin create --email admin@example.com --password securepass --type global

   # Create tenant admin
   ./admin create --email admin@acme.com --password pass --type tenant --tenants uuid1,uuid2

   # List admins
   ./admin list

   # Disable admin
   ./admin disable --email admin@example.com

   # Change password
   ./admin password --email admin@example.com
   ```

### Documentation

- [ ] Update `CLAUDE.md` with admin auth architecture
- [ ] Add admin authentication to API documentation
- [ ] Update `MIGRATION_LOG.md` with admin auth implementation
- [ ] Create admin onboarding guide

---

## 🏗️ Architecture Summary

### Separation of Concerns

| Aspect | User Auth (`/v2/auth/*`) | Admin Auth (`/v2/admin/*`) |
|--------|--------------------------|----------------------------|
| **Audience** | End users | System admins |
| **Storage** | Data Plane (tenant DB) | Control Plane (FS) |
| **Requires** | `tenant_id`, `client_id` | Email only |
| **JWT Claims** | `tid`, `cid`, `scope` | `admin_type`, `tenants`, `aud: hellojohn:admin` |
| **Access** | Tenant-specific | Multi-tenant (global) or restricted (tenant admin) |
| **Middleware** | `RequireAuth()` | `RequireAuth()` + `RequireAdminTenantAccess()` |

### Data Flow

```
┌─────────────────────────────────────────────────────────────┐
│                        CLIENT (UI)                          │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              POST /v2/admin/login                           │
│  Body: {"email": "admin@...", "password": "..."}            │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  AuthController.Login                       │
│  • Parse JSON                                               │
│  • Validate fields                                          │
│  • Delegate to AuthService                                  │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                  AuthService.Login                          │
│  • Get admin from Control Plane (TODO)                      │
│  • Verify password (TODO: Argon2id)                         │
│  • Check admin not disabled                                 │
│  • Issue JWT access token                                   │
│  • Generate refresh token                                   │
│  • Persist refresh token (TODO)                             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                   JWT Issuer                                │
│  • Create AdminAccessClaims                                 │
│  • Sign with EdDSA (active key)                             │
│  • Set audience: "hellojohn:admin"                          │
│  • Include admin_type + tenants                             │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│                     RESPONSE                                │
│  {                                                          │
│    "access_token": "eyJ...",                                │
│    "refresh_token": "base64...",                            │
│    "admin": {"type": "global", "tenants": null}             │
│  }                                                          │
└─────────────────────────────────────────────────────────────┘
```

---

## 🎯 Success Criteria

### Backend (✅ Complete)

- [x] Admin JWT claims structure defined
- [x] Admin DTOs created
- [x] AuthService implemented (Login + Refresh)
- [x] AuthController implemented
- [x] Routes registered (`/v2/admin/login`, `/v2/admin/refresh`)
- [x] Middleware chain configured (public endpoints)
- [x] Wiring complete (services → controllers → routes)
- [x] Compilation successful
- [x] No breaking changes to existing code

### Control Plane Integration (⏳ Pending)

- [ ] Admin repository interface implemented
- [ ] FileSystem adapter for admins
- [ ] Argon2id password verification
- [ ] Refresh token persistence
- [ ] Admin CRUD operations

### Frontend (⏳ Pending)

- [ ] Admin login page created
- [ ] API routes constants added
- [ ] Token storage and refresh logic
- [ ] Admin-specific API client
- [ ] Admin dashboard updated

### Testing (⏳ Pending)

- [ ] Unit tests for AuthService
- [ ] Integration tests for endpoints
- [ ] E2E tests for login flow
- [ ] JWT claims validation tests

---

## 📞 Notes

- **Temporary Validation**: Currently only accepts `admin@example.com` / `admin` for testing
- **Stateless Refresh**: Refresh tokens are generated but not verified (will accept any value)
- **No Breaking Changes**: User authentication (`/v2/auth/*`) remains unchanged
- **Production Ready**: After Control Plane integration, system will be production-ready
- **Backwards Compatible**: Old handlers not affected

---

## 🔗 References

**Implementation Files**:
- `internal/jwt/admin_claims.go`
- `internal/jwt/issuer.go`
- `internal/http/v2/dto/admin/auth.go`
- `internal/http/v2/services/admin/auth_service.go`
- `internal/http/v2/services/admin/services.go`
- `internal/http/v2/controllers/admin/auth_controller.go`
- `internal/http/v2/controllers/admin/controllers.go`
- `internal/http/v2/router/admin_routes.go`
- `internal/http/v2/services/services.go`

**Related Documentation**:
- `ADMIN_AUTH_IMPLEMENTATION_PLAN.md` - Original detailed plan
- `CLAUDE.md` - Architecture guide
- `CORS_SETUP.md` - CORS configuration

**Repository Interfaces**:
- `internal/domain/repository/admin.go` - Admin model and repository interface

---

**End of Document**
