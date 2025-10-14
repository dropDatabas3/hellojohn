# Dev smoke tests (unit + core e2e subset)
# Usage: powershell -ExecutionPolicy Bypass -File scripts/dev-smoke.ps1

$ErrorActionPreference = 'Stop'

$env:DISABLE_DOTENV = '1'

Write-Host '==> Unit tests' -ForegroundColor Cyan
go test ./test/unit/... -count=1
if ($LASTEXITCODE -ne 0) { throw 'Unit tests failed' }

Write-Host '==> E2E basic subset' -ForegroundColor Cyan
go test ./test/e2e -run '00_|01_|02_|04_|08_|22_|35_|36_|37_|38_' -count=1 -timeout=10m
if ($LASTEXITCODE -ne 0) { throw 'E2E subset failed' }

Remove-Item Env:DISABLE_DOTENV -ErrorAction SilentlyContinue

Write-Host 'Smoke tests OK' -ForegroundColor Green
