# Dev helper: build static Admin UI and run the service with embedded UI on a dedicated port
# Requires: Node.js 18+ (or 20+ recommended). PNPM will be used via Corepack when available.
# Usage:
#   pwsh -File scripts/dev-ui.ps1

param(
  [string]$UiDir = "ui",
  [string]$UiOut = "ui/out",
  [string]$UiPort = "8081",
  [string]$ApiPort = "8080",
  [switch]$SkipInstall,
  [switch]$NoBuild
)

$ErrorActionPreference = "Stop"

function Test-Command {
  param([string]$Name)
  $cmd = Get-Command $Name -ErrorAction SilentlyContinue
  return ($null -ne $cmd)
}

Write-Host "[dev-ui] API port: $ApiPort | UI port: $UiPort" -ForegroundColor Cyan

# Verify Node
if (-not (Test-Command node)) {
  Write-Error "Node.js is required. Please install Node 18+ (https://nodejs.org) and re-run."
}

# Prefer pnpm if available; else try Corepack (skip when -NoBuild to avoid permissions issues)
$usePnpm = $false
if (-not $NoBuild) {
  if (Test-Command pnpm) { $usePnpm = $true }
  else {
    if (Test-Command corepack) {
      Write-Host "[dev-ui] Enabling Corepack..." -ForegroundColor DarkCyan
      corepack enable
      try {
        corepack prepare pnpm@latest --activate
        if (Test-Command pnpm) { $usePnpm = $true }
      } catch {
        Write-Warning "[dev-ui] Failed to activate pnpm via Corepack. Falling back to npm."
      }
    }
  }
}

if (-not $NoBuild) {
  Push-Location $UiDir
  try {
    if (-not $SkipInstall) {
      if ($usePnpm) {
        Write-Host "[dev-ui] Installing UI deps (pnpm i)..." -ForegroundColor Cyan
        pnpm install
      } else {
        Write-Host "[dev-ui] Installing UI deps (npm i)..." -ForegroundColor Cyan
        npm install
      }
    }

    # Export static build
    if ($usePnpm) {
      Write-Host "[dev-ui] Building static UI (pnpm run export)..." -ForegroundColor Cyan
      pnpm run export
    } else {
      Write-Host "[dev-ui] Building static UI (npm run export)..." -ForegroundColor Cyan
      npm run export
    }
  } finally {
    Pop-Location
  }
}

# Verify out folder
$resolvedOut = $null
if (-not $NoBuild) {
  $resolvedOut = Resolve-Path $UiOut
  if (-not (Test-Path $resolvedOut)) {
    Write-Error "[dev-ui] UI output folder not found at $resolvedOut."
  }
}

# Set environment for the Go service
# Only set ADMIN_UI_DIR if we have a built UI
if ($resolvedOut) {
  $env:ADMIN_UI_DIR = "$resolvedOut"
}
$env:UI_SERVER_ADDR = ":$UiPort"
$env:UI_PUBLIC_ORIGIN = "http://localhost:$UiPort"
$env:SERVER_ADDR = ":$ApiPort"
$env:DISABLE_DOTENV = "1"

# Minimal bootstrap for secrets in dev
if (-not $env:SIGNING_MASTER_KEY) {
  Write-Warning "[dev-ui] SIGNING_MASTER_KEY not set. Generating a weak dev-only key."
  $env:SIGNING_MASTER_KEY = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
if (-not $env:SECRETBOX_MASTER_KEY) {
  Write-Warning "[dev-ui] SECRETBOX_MASTER_KEY not set. Generating a dev-only base64(32) key."
  $rand = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try { $rng.GetBytes($rand) } finally { $rng.Dispose() }
  $env:SECRETBOX_MASTER_KEY = [Convert]::ToBase64String($rand)
}

# Run the Go service
Write-Host "[dev-ui] Starting service with UI at http://localhost:$UiPort and API at http://localhost:$ApiPort" -ForegroundColor Green
& go run ./cmd/service -env
