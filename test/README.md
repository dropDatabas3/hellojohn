# HelloJohn - Test Suite

Testing completo para el servicio de autenticación OAuth2/OIDC HelloJohn.

---

## 🚀 Quick Start

### Prerequisitos
- Go 1.21+
- PostgreSQL running
- Configuración `.env.dev` lista

### Ejecutar todos los tests
```bash
cd test/e2e
go test -v
```

### Ejecutar test específico
```bash
go test -v -run TestJWTKeyRotation
go test -v -run Test_01_Auth_Basic
```

---

## 🧪 Test Architecture

### 📁 Estructura Actual
```
test/
├── e2e/                             ← Go E2E Tests (21 archivos)
│   ├── TestMain_bootstrap_test.go   ← Setup automático
│   ├── helpers.go                   ← Utilidades compartidas
│   ├── totp.go                     ← Helpers MFA/TOTP
│   ├── seed_types.go               ← Tipos para datos de seed
│   ├── e2e_test.go                 ← Tests básicos (legacy)
│   ├── 00_smoke_discovery_test.go  ← Discovery/JWKS
│   ├── 01_auth_basic_test.go       ← Auth básico
│   ├── 02_refresh_logout_test.go   ← Tokens/logout
│   ├── 03_email_flows_test.go      ← Email flows
│   ├── 04_session_oidc_test.go     ← OIDC/PKCE
│   ├── 05_oidc_negative_test.go    ← Error cases
│   ├── 06_mfa_test.go              ← MFA/TOTP
│   ├── 07_mfa_recovery_test.go     ← Recovery codes
│   ├── 08_revoke_introspect_test.go ← Token introspection
│   ├── 09_rate_limit_test.go       ← Rate limiting
│   ├── 10_rotate_keys_test.go      ← Key rotation (manual)
│   ├── 11_social_google_test.go    ← Google OAuth
│   ├── 12_rate_emailflows_test.go  ← Email rate limiting
│   ├── 13_emailflows_e2e_test.go   ← Email E2E
│   ├── 14_jwt_rotation_test.go     ← JWT key rotation (auto)
│   └── 99_social_google_manual_test.go ← Manual Google test
├── assets/
│   └── callback.html               ← OAuth callback page para tests
└── README.md                       ← Esta documentación
```

### ⚙️ Configuración

**Variables de entorno primarias** (`.env.dev`):
```bash
SERVER_ADDR=:8080
JWT_ISSUER=http://localhost:8080
EMAIL_BASE_URL=http://localhost:8080
STORAGE_DSN=postgres://user:password@localhost:5432/login
SIGNING_MASTER_KEY=0123456789abcdef...  # 64 hex chars
```

**Jerarquía de configuración**:
```
Variables ENV > .env.dev > config.yaml > defaults código
```

---

## 📋 Test Suite Completo

### 🔄 Bootstrap Automático
El `TestMain` ejecuta setup completo antes de cualquier test:

1. ✅ **Set master key** (64 hex chars para cifrado)
2. ✅ **Run migrations** (`go run ./cmd/migrate`)
3. ✅ **Generate JWT keys** (`go run ./cmd/keys -rotate`)
4. ✅ **Seed database** (`go run ./cmd/seed`)
5. ✅ **Start service** (puerto 8080 con `.env.dev`)
6. ✅ **Health check** (wait for readyz)
7. ✅ **Run tests**
8. ✅ **Cleanup**

### 🎯 Tests Detallados (18 archivos de test)

| Test File | Funcionalidad | Qué Valida |
|-----------|---------------|------------|
| **00_smoke_discovery** | Discovery/JWKS | `/.well-known/jwks.json`, `/.well-known/openid-configuration` |
| **01_auth_basic** | Autenticación básica | Login con email/password, tokens válidos |
| **02_refresh_logout** | Token management | Refresh tokens, logout, invalidación |
| **03_email_flows** | Email flows | Reset password, verify email, templates |
| **04_session_oidc** | OIDC Core | Authorization Code + PKCE flow completo |
| **05_oidc_negative** | Error cases | invalid_grant, invalid_scope, malformed requests |
| **06_mfa_totp** | MFA Setup | TOTP enrollment, secret sharing, validation |
| **07_mfa_recovery** | MFA Recovery | Recovery codes generation/usage |
| **08_revoke_introspect** | Token introspection | Revoke tokens, introspect endpoints |
| **09_rate_limit** | Rate limiting | Burst protection, 429 responses |
| **10_rotate_keys** | Key rotation (manual) | Manual key rotation testing |
| **11_social_google** | Social auth | Google OAuth integration |
| **12_rate_emailflows** | Email rate limiting | Anti-abuse for password reset |
| **13_emailflows_e2e** | Email E2E | Complete email flow testing |
| **14_jwt_rotation** | **JWT Key Rotation (auto)** | **Key lifecycle, multi-key JWKS** |
| **99_social_google_manual** | Manual Google test | Manual Google OAuth testing |
| **e2e_test** | Legacy tests | Discovery, login, MFA (legacy) |
| **TestMain_bootstrap** | Bootstrap | Setup automático para todos los tests |

---

## 🔑 JWT Key Rotation (Test Crítico)

### ¿Qué hace?
- Testa rotación completa de claves JWT EdDSA
- Valida que tokens antiguos siguen funcionando
- Verifica que nuevos tokens usan nueva clave  
- Confirma múltiples claves en JWKS

### Subtests incluidos:
1. **FullKeyRotationFlow**: Flow completo de rotación
2. **KeyRotationEnvironmentValidation**: Validación de entorno
3. **MultipleKeyValidation**: Verificación de múltiples keys

### Ejecutar solo JWT rotation:
```bash
go test -v -run TestJWTKeyRotation
```

---

## 📊 Test Data (Seed)

Los tests usan **datos seeded automáticamente**:

```yaml
# Datos cargados en TestMain
tenant:
  id: "7bee1e9e-5003-482b-abd6-ffe9e66f7b37"
users:
  admin:
    email: "admin@example.com"
    password: "Test1234A!"
clients:
  web:
    client_id: "web-frontend"
```

**Global access**:
```go
var seed *seedData  // Disponible en todos los tests
```

---

## 🔧 Utilities & Helpers

### Helper Files

#### `helpers.go` - Utilidades principales
```go
func newHTTPClient() *http.Client        // Client con cookies habilitadas
func getBaseURL() string                 // JWT_ISSUER > EMAIL_BASE_URL > fallback
func mustJSON(r io.Reader, v interface{}) error
func mustLoadSeedYAML() (*seedData, error)
func randomEmail(tag string) string
func findRepoRoot() (string, error)     // Encuentra go.mod
func startServer(ctx context.Context, envFile string) (*serverProc, error)
func runCmd(ctx context.Context, _ string, args ...string) (string, error)
```

#### `totp.go` - Helpers MFA/TOTP
```go
func GenerateTOTPCode(secret string) (string, error)
func ValidateTOTPCode(secret, code string) bool
```

#### `seed_types.go` - Tipos para datos de seed
```go
type seedData struct {
    Tenant struct { ID string }
    Users  struct { Admin struct { Email, Password string } }
    Clients struct { Web struct { ClientID string } }
    // ... más estructuras
}
```

### Test Assets

#### `test/assets/callback.html`
- Página HTML para callback OAuth
- Usada en tests de OIDC/OAuth flows
- Maneja códigos de autorización y tokens

---

## 🚦 Health Checks

### Service Readiness
El servicio debe pasar todas las validaciones antes de tests:

- ✅ **Database connectivity**
- ✅ **JWT signing keys exist**  
- ✅ **Self-check token generation/validation**

### Endpoint: `GET /readyz`
```bash
curl http://localhost:8080/readyz
# Response: 200 OK + "OK"
```

---

## 🐛 Troubleshooting

### Tests fallan con "connection refused"
```bash
# Verificar que el servicio esté corriendo
curl http://localhost:8080/readyz

# Si no responde, revisar logs del TestMain
go test -v  # Logs completos del bootstrap
```

### Tests fallan con "missing JWT keys"
```bash
# El bootstrap debería generar keys automáticamente
# Si falla, ejecutar manualmente:
go run ./cmd/keys -rotate
```

### Tests fallan con "database connection"
```bash
# Verificar PostgreSQL y DSN en .env.dev
STORAGE_DSN=postgres://user:password@localhost:5432/login?sslmode=disable
```

### Port conflicts (8080 en uso)
```bash
# Matar procesos en puerto 8080
netstat -ano | findstr :8080
taskkill /PID <PID> /F
```

---

## 📈 Performance & Timing

### Test Durations (aprox)
- **Bootstrap**: ~10-15s (migration + keys + seed + startup)
- **Individual tests**: 1-5s cada uno
- **JWT Rotation**: ~40s (incluye cache expiration wait)
- **Social Google tests**: Skip if no config
- **Full suite**: ~2-3 minutos (21 archivos Go)

### Optimizaciones
- Tests se ejecutan en **secuencia** (no paralelo por shared state)
- **Reutilización** de seed data entre tests
- **Single service instance** para toda la suite

---

## 🔒 Security Testing

### Validaciones de seguridad incluidas:
- ✅ **CORS policy enforcement**
- ✅ **Rate limiting protection**  
- ✅ **JWT signature validation**
- ✅ **PKCE code challenge verification**
- ✅ **Token expiration handling**
- ✅ **MFA enforcement paths**
- ✅ **Recovery code single-use**
- ✅ **Email link security** (reset/verify)

---

## 🎯 Comandos Útiles

### Testing específico
```bash
# Test completo con logs detallados
go test -v

# Test específico con timeout
go test -v -timeout=5m -run TestJWTKeyRotation

# Test con coverage
go test -v -cover

# Test en modo short (skip tests largos)
go test -v -short

# Solo tests de un número específico
go test -v -run "Test_0[1-5]"  # Tests 01 a 05
```

### Debugging
```bash
# Logs del servicio durante tests
go test -v 2>&1 | tee test.log

# Solo tests que fallen
go test -v -failfast

# Re-run tests que fallen
go test -v -count=1
```

### Development
```bash
# Migrar DB manualmente
go run ./cmd/migrate

# Generar JWT keys manualmente  
go run ./cmd/keys -rotate

# Seed data manualmente
go run ./cmd/seed

# Start service manualmente
go run ./cmd/service -env -env-file .env.dev
```

---

## ✅ Success Criteria

### Tests exitosos deben mostrar:
```
=== RUN   TestJWTKeyRotation
=== RUN   TestJWTKeyRotation/FullKeyRotationFlow
=== RUN   TestJWTKeyRotation/KeyRotationEnvironmentValidation  
=== RUN   TestJWTKeyRotation/MultipleKeyValidation
--- PASS: TestJWTKeyRotation (39.49s)
PASS
ok      github.com/dropDatabas3/hellojohn/test/e2e    48.359s
```

### Service health check:
- Status: `200 OK`
- Response: `"OK"`
- JWT keys: ≥1 active key in JWKS

### Archivos principales:
- `21 archivos Go` en `/test/e2e/`
- `18 archivos de test` específicos
- `1 callback.html` en `/test/assets/`

---

**Última actualización**: Septiembre 2025  
**Versión**: Sprint 5 - JWT Rotation Implementation Complete  
**Tests totales**: 18 test files + 3 utility files
