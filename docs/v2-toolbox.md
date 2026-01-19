# V2 Toolbox - Referencia Rápida

**Documento de referencia interna** con todas las herramientas V2 disponibles para migrar handlers.

---

## Quick Index

| Categoría | Ubicación | Qué hace |
|-----------|-----------|----------|
| [Store V2](#store-v2) | `store/v2` | DAL unificado, ForTenant, ConfigAccess |
| [Repositories](#repositories) | `domain/repository` | UserRepo, TokenRepo, ClientRepo... |
| [JWT](#jwt) | `internal/jwt` | Issuer, Keystore, validación EdDSA |
| [ControlPlane](#controlplane-v2) | `controlplane/v2` | Tenants, Clients, Scopes |
| [Email](#email-v2) | `email/v2` | Verificación, Reset, Notificaciones |
| [Cache](#cache-v2) | `cache/v2` | Redis/Memory client |
| [Middlewares](#middlewares-v2) | `http/v2/middlewares` | Auth, Tenant, RBAC, Rate... |
| [Logger](#logger) | `observability/logger` | Logging estructurado |

---

## Store V2

### DataAccessLayer
```go
dal.ForTenant(ctx, slugOrID) → (TenantDataAccess, error)
dal.ConfigAccess() → ConfigAccess
dal.Mode() → OperationalMode
dal.MigrateTenant(ctx, slugOrID) → (*MigrationResult, error)
```

### ConfigAccess (Control Plane - siempre FS)
```go
cfg := dal.ConfigAccess()
cfg.Tenants().    List/GetBySlug/GetByID/Create/Update/Delete/UpdateSettings
cfg.Clients(slug) List/Get/Create/Update/Delete/DecryptSecret
cfg.Scopes(slug)  List/GetByName/Create/Delete/Upsert
```

### TenantDataAccess (Data Plane - DB)
```go
tda, _ := dal.ForTenant(ctx, "acme")
tda.Slug()        // "acme"
tda.ID()          // UUID
tda.Settings()    // *TenantSettings
tda.HasDB()       // bool
tda.RequireDB()   // error si no hay DB

// Repos (pueden ser nil si no hay DB):
tda.Users()       → UserRepository
tda.Tokens()      → TokenRepository
tda.MFA()         → MFARepository
tda.Consents()    → ConsentRepository
tda.RBAC()        → RBACRepository
tda.EmailTokens() → EmailTokenRepository
tda.Identities()  → IdentityRepository
tda.Schema()      → SchemaRepository
tda.Cache()       → cache.Client

// Siempre disponibles (FS):
tda.Clients()     → ClientRepository  
tda.Scopes()      → ScopeRepository
```

---

## Repositories

### UserRepository
```go
GetByEmail(ctx, tenantID, email) → (*User, *Identity, error)
GetByID(ctx, userID) → (*User, error)
List(ctx, tenantID, filter) → ([]User, error)
Create(ctx, input) → (*User, *Identity, error)
Update(ctx, userID, input) → error
Delete(ctx, userID) → error
Disable(ctx, userID, by, reason, until) → error
Enable(ctx, userID, by) → error
CheckPassword(hash, password) → bool
SetEmailVerified(ctx, userID, verified) → error
UpdatePasswordHash(ctx, userID, newHash) → error
```

### TokenRepository
```go
Create(ctx, input) → (tokenID, error)
GetByHash(ctx, hash) → (*RefreshToken, error)
Revoke(ctx, tokenID) → error
RevokeAllByUser(ctx, userID, clientID) → (count, error)
RevokeAllByClient(ctx, clientID) → error
```

### ClientRepository
```go
Get(ctx, tenantID, clientID) → (*Client, error)
List(ctx, tenantID, query) → ([]Client, error)
Create(ctx, tenantID, input) → (*Client, error)
Update(ctx, tenantID, input) → (*Client, error)
Delete(ctx, tenantID, clientID) → error
DecryptSecret(ctx, tenantID, clientID) → (string, error)
ValidateClientID(id) → bool
ValidateRedirectURI(uri) → bool
IsScopeAllowed(client, scope) → bool
```

### TenantRepository
```go
List(ctx) → ([]Tenant, error)
GetBySlug(ctx, slug) → (*Tenant, error)
GetByID(ctx, id) → (*Tenant, error)
Create(ctx, tenant) → error
Update(ctx, tenant) → error
Delete(ctx, slug) → error
UpdateSettings(ctx, slug, settings) → error
```

### KeyRepository
```go
GetActive(ctx, tenantID) → (*SigningKey, error)
GetByKID(ctx, kid) → (*SigningKey, error)
GetJWKS(ctx, tenantID) → (*JWKS, error)
Generate(ctx, tenantID, algorithm) → (*SigningKey, error)
Rotate(ctx, tenantID, gracePeriod) → (*SigningKey, error)
ToEdDSA(key) → (ed25519.PrivateKey, error)
```

### MFARepository
```go
UpsertTOTP(ctx, userID, secretEnc) → error
ConfirmTOTP(ctx, userID) → error
GetTOTP(ctx, userID) → (*MFATOTP, error)
UpdateTOTPUsedAt(ctx, userID) → error
DisableTOTP(ctx, userID) → error
SetRecoveryCodes(ctx, userID, hashes) → error
UseRecoveryCode(ctx, userID, hash) → (bool, error)
AddTrustedDevice(ctx, userID, hash, expiresAt) → error
IsTrustedDevice(ctx, userID, hash) → (bool, error)
```

### EmailTokenRepository
```go
Create(ctx, input) → (*EmailToken, error)
GetByHash(ctx, hash) → (*EmailToken, error)
Use(ctx, hash) → error
DeleteExpired(ctx) → (count, error)
```

### IdentityRepository (Social)
```go
GetByProvider(ctx, tenantID, provider, providerUserID) → (*SocialIdentity, error)
GetByUserID(ctx, userID) → ([]SocialIdentity, error)
Upsert(ctx, input) → (userID, isNew, error)
Link(ctx, userID, input) → (*SocialIdentity, error)
Unlink(ctx, userID, provider) → error
```

### ConsentRepository
```go
Upsert(ctx, tenantID, userID, clientID, scopes) → (*Consent, error)
Get(ctx, tenantID, userID, clientID) → (*Consent, error)
ListByUser(ctx, tenantID, userID, activeOnly) → ([]Consent, error)
Revoke(ctx, tenantID, userID, clientID) → error
```

### RBACRepository
```go
GetUserRoles(ctx, userID) → ([]string, error)
GetUserPermissions(ctx, userID) → ([]string, error)
AssignRole(ctx, tenantID, userID, role) → error
RemoveRole(ctx, tenantID, userID, role) → error
GetRolePermissions(ctx, tenantID, role) → ([]string, error)
```

### CacheRepository
```go
Get(ctx, key) → ([]byte, bool)
Set(ctx, key, value, ttl) → error
Delete(ctx, key) → error
Exists(ctx, key) → (bool, error)
GetAndDelete(ctx, key) → ([]byte, bool, error)
SetNX(ctx, key, value, ttl) → (bool, error)
DeleteByPrefix(ctx, prefix) → (count, error)
```

---

## JWT

### Issuer
```go
issuer := jwt.NewIssuer(baseURL, keystore)
issuer.AccessTTL = 15 * time.Minute

// Firma
IssueAccess(sub, aud, std, custom) → (token, exp, error)
IssueIDToken(sub, aud, std, extra) → (token, exp, error)
IssueAccessForTenant(tenant, iss, sub, aud, std, custom) → (token, exp, error)
SignRaw(claims) → (token, kid, error)

// Validación
Keyfunc() → jwt.Keyfunc                    // Global
KeyfuncForTenant(tenant) → jwt.Keyfunc     // Por tenant
KeyfuncFromTokenClaims() → jwt.Keyfunc     // Auto-detecta tenant

// JWKS
JWKSJSON() → []byte
ActiveKID() → (kid, error)
```

### PersistentKeystore
```go
ks := jwt.NewPersistentKeystore(keyRepo)
ks.EnsureBootstrap(ctx) → error

Active() → (kid, priv, pub, error)
ActiveForTenant(tenant) → (kid, priv, pub, error)
PublicKeyByKID(kid) → (ed25519.PublicKey, error)
JWKSJSON() → ([]byte, error)
JWKSJSONForTenant(tenant) → ([]byte, error)
RotateFor(tenant, graceSeconds) → (*SigningKey, error)
```

### ParseEdDSA
```go
jwt.ParseEdDSA(token, keystore, expectedIss) → (claims, error)
jwt.ResolveIssuer(baseURL, mode, tenantSlug, override) → string
```

---

## ControlPlane V2

```go
cp := controlplane.NewService(storeMgr)

// Tenants
cp.ListTenants(ctx) → ([]Tenant, error)
cp.GetTenant(ctx, slug) → (*Tenant, error)
cp.CreateTenant(ctx, name, slug, language) → (*Tenant, error)
cp.UpdateTenantSettings(ctx, slug, settings) → error
cp.DeleteTenant(ctx, slug) → error

// Clients
cp.ListClients(ctx, slug) → ([]Client, error)
cp.GetClient(ctx, slug, clientID) → (*Client, error)
cp.CreateClient(ctx, slug, input) → (*Client, error)
cp.UpdateClient(ctx, slug, input) → (*Client, error)
cp.DeleteClient(ctx, slug, clientID) → error
cp.DecryptClientSecret(ctx, slug, clientID) → (string, error)

// Scopes
cp.ListScopes(ctx, slug) → ([]Scope, error)
cp.CreateScope(ctx, slug, name, desc) → (*Scope, error)
cp.DeleteScope(ctx, slug, name) → error

// Validaciones
cp.ValidateClientID(id) → bool
cp.ValidateRedirectURI(uri) → bool
cp.IsScopeAllowed(client, scope) → bool
```

---

## Email V2

```go
svc, _ := emailv2.NewService(emailv2.ServiceConfig{
    DAL: dal, MasterKey: key, BaseURL: url,
    VerifyTTL: 24*time.Hour, ResetTTL: 1*time.Hour,
})

// Enviar emails
svc.SendVerificationEmail(ctx, req) → error
svc.SendPasswordResetEmail(ctx, req) → error
svc.SendNotificationEmail(ctx, req) → error

// Test SMTP
svc.TestSMTP(ctx, tenant, email, override) → error

// Sender directo
sender, _ := svc.GetSender(ctx, tenant)
sender.Send(to, subject, html, text) → error

// Diagnóstico
emailv2.DiagnoseSMTP(err) → SMTPDiag{Code, Temporary}
```

### Templates Multi-idioma
```yaml
mailing.templates:
  es:
    verify_email: {subject, body}
    reset_password: {...}
    user_blocked: {...}
    user_unblocked: {...}
  en:
    ...
```

**Fallback**: user.Language → tenant.Language → "es" → hardcoded

---

## Cache V2

```go
client := tda.Cache()

client.Get(ctx, key) → (string, error)
client.Set(ctx, key, value, ttl) → error
client.Delete(ctx, key) → error
client.Exists(ctx, key) → (bool, error)
client.Ping(ctx) → error
client.Stats(ctx) → (Stats, error)

cache.IsNotFound(err) → bool
```

**Prefijos estándar**:
- `sid:` Session
- `mfa:token:` MFA Challenge
- `code:` Auth Code
- `social:code:` Social login
- `rl:` Rate Limit
- `jwks:` JWKS cache

---

## Middlewares V2

### Chain
```go
handler := mw.Chain(myHandler,
    mw.WithRecover(),
    mw.WithRequestID(),
    mw.WithLogging(),
    mw.WithCORS(origins),
    mw.WithTenantResolution(dal, false),
    mw.RequireAuth(issuer),
)
```

### Infraestructura
```go
mw.WithRecover()              // Captura panics
mw.WithRequestID()            // X-Request-ID
mw.WithLogging()              // Log JSON
mw.WithCORS(origins)          // CORS
mw.WithSecurityHeaders()      // Security headers
mw.WithNoStore()              // Cache-Control: no-store
mw.WithCSRF(cfg)              // Protección CSRF
mw.WithRateLimit(cfg)         // Rate limiting
```

### Tenant
```go
mw.WithTenantResolution(dal, optional)  // Resuelve tenant → ctx
mw.RequireTenant()                       // Verifica tenant presente
mw.RequireTenantDB()                     // Verifica tenant tiene DB
```

### Autenticación
```go
mw.RequireAuth(issuer)     // JWT requerido, 401 si falta
mw.OptionalAuth(issuer)    // JWT opcional, no falla
mw.RequireUser()           // Verifica sub != ""
```

### Autorización
```go
mw.RequireAdmin(cfg)                    // Admin de tenant
mw.RequireSysAdmin(issuer, cfg)         // Admin de sistema
mw.RequireRole(issuer, "admin", ...)    // Al menos un rol
mw.RequireAllRoles(issuer, "a", "b")    // Todos los roles
mw.RequirePerm(issuer, "users:write")   // Al menos un permiso
mw.RequireScope("profile:read")         // OAuth scope
mw.RequireAnyScope("a", "b")            // Al menos un scope
mw.RequireAllScopes("a", "b")           // Todos los scopes
```

### Cluster
```go
mw.RequireLeader(clusterRepo, redirects)  // Solo líder
```

### Context Helpers
```go
mw.GetClaims(ctx) → map[string]any
mw.GetTenant(ctx) → TenantDataAccess
mw.MustGetTenant(ctx) → TenantDataAccess (panic si nil)
mw.GetUserID(ctx) → string
mw.GetRequestID(ctx) → string
```

---

## Logger

```go
logger.Init(logger.Config{Env: "prod", Level: "info"})
defer logger.Sync()

// Singleton
logger.L().Info("msg", logger.String("key", "val"))
logger.S().Infof("user %s", id)

// Desde contexto (recomendado)
log := logger.From(ctx)
log.Info("msg", logger.UserID(id))
log.Error("failed", logger.Err(err))
```

### Campos estándar
```go
// HTTP
logger.RequestID(v), Method(v), Path(v), Status(v), DurationMs(v)

// Negocio
logger.TenantID(v), TenantSlug(v), UserID(v), ClientID(v), Email(v)

// Sistema
logger.Component(v), Op(v), Layer(v), Err(err)

// Genéricos
logger.String(k, v), Int(k, v), Bool(k, v), Any(k, v)
```

---

## Errores Comunes

### Repository
```go
repository.ErrNotFound       // 404
repository.ErrConflict       // 409
repository.ErrInvalidInput   // 400
repository.ErrNoDatabase     // 503
repository.ErrTokenExpired   // 401
repository.IsNotFound(err)   // Helper
```

### Store
```go
store.ErrTenantNotFound      // Tenant no existe
store.ErrNoDBForTenant       // Sin DB configurada
store.IsTenantNotFound(err)  // Helper
```

### Email
```go
emailv2.ErrNoSMTPConfig      // Sin config SMTP
emailv2.ErrSendFailed        // Envío falló
```

---

## Patrones Comunes

### Handler típico (migrado de V1)
```go
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // 1. Context helpers
    tda := mw.MustGetTenant(ctx)
    claims := mw.GetClaims(ctx)
    log := logger.From(ctx).With(logger.Op("CreateUser"))
    
    // 2. Parse input
    var input CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
        httperr.BadRequest(w, "invalid_json")
        return
    }
    
    // 3. Business logic (delegar a service)
    user, err := h.userSvc.Create(ctx, tda, input)
    if err != nil {
        log.Error("failed", logger.Err(err))
        httperr.FromError(w, err)
        return
    }
    
    // 4. Response
    json.NewEncoder(w).Encode(user)
}
```

### Service típico
```go
func (s *UserService) Create(ctx context.Context, tda store.TenantDataAccess, input CreateUserInput) (*User, error) {
    log := logger.From(ctx).With(logger.Op("UserService.Create"))
    
    // Verificar DB
    if err := tda.RequireDB(); err != nil {
        return nil, err
    }
    
    // Usar repositorio
    user, _, err := tda.Users().Create(ctx, repository.CreateUserInput{
        TenantID:       tda.ID(),
        Email:          input.Email,
        PasswordHash:   hashPassword(input.Password),
        SourceClientID: input.ClientID,
    })
    if err != nil {
        if repository.IsConflict(err) {
            return nil, ErrEmailExists
        }
        return nil, err
    }
    
    // Enviar email verificación
    _ = s.email.SendVerificationEmail(ctx, emailv2.SendVerificationRequest{
        TenantSlugOrID: tda.Slug(),
        UserID:         user.ID,
        Email:          user.Email,
        Token:          generateToken(),
        TTL:            24 * time.Hour,
    })
    
    log.Info("user created", logger.UserID(user.ID))
    return user, nil
}
```
