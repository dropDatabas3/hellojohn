# Set envs and run the HelloJohn service in this console
param(
  [string]$ApiPort = ":8080",
  [string]$UiPort = ":8081",
  [string]$UiDir = ""
)

Set-Location -Path (Split-Path -Parent $MyInvocation.MyCommand.Path) | Out-Null
Set-Location -Path ..

if (-not [string]::IsNullOrWhiteSpace($UiDir)) {
  $resolved = (Resolve-Path $UiDir).Path
  Set-Item Env:ADMIN_UI_DIR -Value $resolved
}

Set-Item Env:UI_SERVER_ADDR -Value $UiPort
Set-Item Env:UI_PUBLIC_ORIGIN -Value ("http://localhost:{0}" -f ($UiPort.TrimStart(':')))
Set-Item Env:SERVER_ADDR -Value $ApiPort
Set-Item Env:DISABLE_DOTENV -Value '1'

if (-not $env:SIGNING_MASTER_KEY) {
  Set-Item Env:SIGNING_MASTER_KEY -Value '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
}
if (-not $env:SECRETBOX_MASTER_KEY) {
  $bytes = New-Object byte[] 32
  $rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
  try { $rng.GetBytes($bytes) } finally { $rng.Dispose() }
  $b64 = [Convert]::ToBase64String($bytes)
  Set-Item Env:SECRETBOX_MASTER_KEY -Value $b64
}

Write-Host "[run-service] API at $ApiPort | UI at $UiPort | ADMIN_UI_DIR=$env:ADMIN_UI_DIR" -ForegroundColor Green
& go run ./cmd/service -env
