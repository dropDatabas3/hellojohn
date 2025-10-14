# Dev HA test runner for Phase 6
<#
Usage:
  powershell -ExecutionPolicy Bypass -File scripts/dev-ha.ps1
  powershell -ExecutionPolicy Bypass -File scripts/dev-ha.ps1 -Clean

Switches:
  -Clean  Elimina directorios previos de datos/raft para evitar arranques con estado viejo.
#>

param(
  [switch]$Clean
)

$ErrorActionPreference = 'Stop'

if ($Clean) {
  Write-Host '==> Cleaning previous local cluster data (data/hellojohn/raft/ ...)' -ForegroundColor Yellow
  $paths = @('data/hellojohn/raft','data/hellojohn/node1','data/hellojohn/node2','data/hellojohn/node3')
  foreach ($p in $paths) { if (Test-Path $p) { Remove-Item -Recurse -Force $p; Write-Host "Removed $p" } }
}

Write-Host '==> Setting env vars for HA E2E subset'
$env:E2E_SKIP_GLOBAL_SERVER = '1'
$env:DISABLE_DOTENV = '1'

function Invoke-HATest {
  param(
    [string]$Name,
    [string]$Pattern,
    [string]$Timeout
  )
  Write-Host "\n==> Running $Name ($Pattern)" -ForegroundColor Cyan
  go test ./test/e2e -run $Pattern -count=1 -timeout=$Timeout
  if ($LASTEXITCODE -ne 0) { throw "Test failed: $Name" }
}

Invoke-HATest -Name 'Leader Gating Redirect' -Pattern 'Test_40_Leader_Gating_Redirect' -Timeout '4m'
Invoke-HATest -Name 'Snapshot Restore JWKS' -Pattern 'Test_41_SnapshotRestore_JWKSIdentical' -Timeout '6m'
Invoke-HATest -Name 'RequireLeader Wiring Canary' -Pattern 'Test_42_RequireLeader_Wiring_Smoke' -Timeout '2m'

Write-Host '\nAll HA tests passed.' -ForegroundColor Green

Write-Host '==> Cleaning env vars'
Remove-Item Env:E2E_SKIP_GLOBAL_SERVER -ErrorAction SilentlyContinue
Remove-Item Env:DISABLE_DOTENV -ErrorAction SilentlyContinue
