# Local developer helper: migrate DB, seed admin data, then run E2E tests.
# - Assumes Postgres DSN is available in STORAGE_DSN env var.
# - Optionally runs migrations using cmd/migrate.
# - Seeds using cmd/seed.
# - Runs "go test ./test/e2e -run <filter>" with DISABLE_DOTENV=1 and no .env loading.

param(
    [string]$Dsn = $env:STORAGE_DSN,
    [switch]$NoMigrate,
    [string]$Run = ".*",  # regex filter for tests
    [switch]$Short
)

$ErrorActionPreference = "Stop"

function Resolve-RepoRoot {
    $dir = Get-Location
    for ($i = 0; $i -lt 8; $i++) {
        if (Test-Path (Join-Path $dir "go.mod")) { return $dir }
        $parent = Split-Path $dir -Parent
        if ($parent -eq $dir) { break }
        $dir = $parent
    }
    throw "go.mod not found up to 8 levels"
}

$root = Resolve-RepoRoot
Write-Host "Repo root: $root" -ForegroundColor Cyan

if (-not $Dsn) {
    throw "STORAGE_DSN not set. Example: postgres://user:pass@127.0.0.1:5432/hellojohn?sslmode=disable"
}

# 1) Migrations (optional)
if (-not $NoMigrate) {
    Write-Host "Running migrations..." -ForegroundColor Cyan
    Push-Location $root
    go run ./cmd/migrate up | Write-Host
    Pop-Location
}

# 2) Seed (admin, users, clients, RBAC)
Write-Host "Seeding database..." -ForegroundColor Cyan
$env:EMAIL_BASE_URL = $env:EMAIL_BASE_URL  # preserve if set; else seed will default
$env:JWT_ISSUER = $env:JWT_ISSUER
# MFA_ENC_KEY or SIGNING_MASTER_KEY for AES-GCM of TOTP
if (-not $env:MFA_ENC_KEY -and -not $env:SIGNING_MASTER_KEY) {
    $env:SIGNING_MASTER_KEY = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
Push-Location $root
go run ./cmd/seed | Write-Host
Pop-Location

# 3) Run E2E tests with env isolation
Write-Host "Running E2E tests..." -ForegroundColor Cyan
$env:DISABLE_DOTENV = "1"
# Reduce test flakiness on Windows by increasing overall test timeout
$extra = @()
if ($Short) { $extra += "-short" }
Push-Location $root
go test ./test/e2e -run $Run @extra | Write-Host
Pop-Location

Write-Host "Done." -ForegroundColor Green
