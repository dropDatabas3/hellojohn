# Setup V2 - Minimal Viable Configuration (PowerShell)
# Creates the minimum required setup to run cmd/service_v2/main.go

Write-Host "🚀 Setting up HelloJohn V2 (Minimal Configuration)..." -ForegroundColor Green

# 1. Create directory structure
Write-Host "📁 Creating data directory structure..." -ForegroundColor Cyan
New-Item -ItemType Directory -Force -Path "data\hellojohn\tenants\local" | Out-Null
New-Item -ItemType Directory -Force -Path "data\hellojohn\keys" | Out-Null

# 2. Create default tenant
Write-Host "🏢 Creating default tenant (local)..." -ForegroundColor Cyan
$tenantYaml = @"
id: "00000000-0000-0000-0000-000000000001"
slug: "local"
name: "Local Development"
language: "en"
created_at: "$(Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ" -AsUTC)"

settings:
  issuer_mode: "path"
"@
$tenantYaml | Out-File -FilePath "data\hellojohn\tenants\local\tenant.yaml" -Encoding UTF8

# 3. Create default client
Write-Host "🔑 Creating default OAuth client..." -ForegroundColor Cyan
$clientsYaml = @"
clients:
  - client_id: "dev-client"
    name: "Development Client"
    type: "public"
    redirect_uris:
      - "http://localhost:3000/callback"
      - "http://localhost:8080/callback"
    default_scopes:
      - "openid"
      - "profile"
      - "email"
    providers:
      - "password"
    created_at: "$(Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ" -AsUTC)"
"@
$clientsYaml | Out-File -FilePath "data\hellojohn\tenants\local\clients.yaml" -Encoding UTF8

# 4. Create default scopes
Write-Host "🎯 Creating default scopes..." -ForegroundColor Cyan
$scopesYaml = @"
scopes:
  - name: "openid"
    description: "OpenID Connect scope"
  - name: "profile"
    description: "User profile information"
  - name: "email"
    description: "User email address"
  - name: "offline_access"
    description: "Refresh token access"
"@
$scopesYaml | Out-File -FilePath "data\hellojohn\tenants\local\scopes.yaml" -Encoding UTF8

# 5. Generate keys
Write-Host "🔐 Generating master keys..." -ForegroundColor Cyan

# Generate SIGNING_MASTER_KEY (64 hex chars = 32 bytes)
$signingBytes = New-Object byte[] 32
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($signingBytes)
$SIGNING_KEY = ($signingBytes | ForEach-Object { $_.ToString("x2") }) -join ""

# Generate SECRETBOX_MASTER_KEY (32 bytes base64)
$secretboxBytes = New-Object byte[] 32
$rng.GetBytes($secretboxBytes)
$SECRETBOX_KEY = [Convert]::ToBase64String($secretboxBytes)

# 6. Create .env file
Write-Host "📝 Creating .env.v2 file..." -ForegroundColor Cyan
$envContent = @"
# HelloJohn V2 Environment Configuration
# Generated: $(Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ" -AsUTC)

# REQUIRED: JWT Signing Master Key (64 hex chars = 32 bytes)
SIGNING_MASTER_KEY=$SIGNING_KEY

# REQUIRED: Secrets Encryption Key (32 bytes base64)
SECRETBOX_MASTER_KEY=$SECRETBOX_KEY

# REQUIRED: FileSystem Root for Control Plane
FS_ROOT=./data/hellojohn

# Server Configuration
V2_SERVER_ADDR=:8082
V2_BASE_URL=http://localhost:8082

# Auth Configuration
REGISTER_AUTO_LOGIN=true
FS_ADMIN_ENABLE=false

# Social Configuration
SOCIAL_DEBUG_PEEK=false

# Optional: Database for tenant 'local' (uncomment to enable)
# LOCAL_DB_DRIVER=postgres
# LOCAL_DB_DSN=postgres://user:pass@localhost:5432/hellojohn_local?sslmode=disable
"@
$envContent | Out-File -FilePath ".env.v2" -Encoding UTF8

Write-Host ""
Write-Host "✅ Setup complete!" -ForegroundColor Green
Write-Host ""
Write-Host "📋 Next steps:" -ForegroundColor Yellow
Write-Host ""
Write-Host "1. Load environment variables:" -ForegroundColor White
Write-Host "   Get-Content .env.v2 | ForEach-Object { `$k,`$v = `$_.Split('=',2); [Environment]::SetEnvironmentVariable(`$k, `$v) }" -ForegroundColor Gray
Write-Host ""
Write-Host "2. Or use this helper:" -ForegroundColor White
Write-Host "   . .\scripts\load_env_v2.ps1" -ForegroundColor Gray
Write-Host ""
Write-Host "3. Run V2 server:" -ForegroundColor White
Write-Host "   go run cmd/service_v2/main.go" -ForegroundColor Gray
Write-Host ""
Write-Host "4. Test health endpoint:" -ForegroundColor White
Write-Host "   curl http://localhost:8082/readyz" -ForegroundColor Gray
Write-Host ""
Write-Host "🔒 IMPORTANT: .env.v2 contains sensitive keys. Add to .gitignore!" -ForegroundColor Red
Write-Host ""
