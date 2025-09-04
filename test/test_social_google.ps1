param(
  [string]$Base = "http://localhost:8080",
  [string]$TenantId = "9d9a424f-6d5f-47f0-9f86-bf2346bc0678",
  [string]$ClientId = "web-frontend",
  # por defecto, modo prueba (no consume el code en la primera carga)
  [string]$RedirectUri = "$Base/v1/auth/social/result?peek=1"
)

$ErrorActionPreference = "Stop"
try { [Console]::OutputEncoding = [System.Text.Encoding]::UTF8 } catch {}

Write-Host "== Descubrir providers ==" -ForegroundColor Cyan
$prov = Invoke-RestMethod -Method GET "$Base/v1/auth/providers"
$prov | ConvertTo-Json -Depth 6
$g = $prov.providers | Where-Object { $_.name -eq "google" }
if (-not $g -or -not $g.enabled) {
  throw "Google no está habilitado en /v1/auth/providers"
}

# Abrimos el flujo con peek=1 (la vista muestra el 'code' y NO lo consume)
$startUrl = "$Base/v1/auth/social/google/start?tenant_id=$TenantId&client_id=$ClientId&redirect_uri=$([uri]::EscapeDataString($RedirectUri))"
Write-Host "`nAbrir navegador para login: $startUrl" -ForegroundColor Yellow
Start-Process $startUrl

Write-Host "`nCuando termine el login:" -ForegroundColor Yellow
Write-Host " - Copiá el 'Código de login' que aparece en la página (no se consumió por peek=1)." -ForegroundColor Yellow
Write-Host " - O copiá la URL completa de la barra y yo intentaré extraer el code." -ForegroundColor Yellow

$code = Read-Host "Pegá aquí el 'code' (o Enter para leer el portapapeles)"
if (-not $code) {
  try {
    $clip = Get-Clipboard
    if ($clip -match "code=([A-Za-z0-9_\-]+)") {
      $code = $matches[1]
      Write-Host "Code detectado desde portapapeles: $code" -ForegroundColor DarkGray
    }
  } catch {}
}
if (-not $code) { throw "Sin code" }

Write-Host "`n== Canjear code en /social/result ==" -ForegroundColor Cyan
# Importante: ahora SIN peek => se consume y devuelve los tokens
$tokens = Invoke-RestMethod -Method GET "$Base/v1/auth/social/result?code=$code" -Headers @{ "Accept" = "application/json" }
$tokens | ConvertTo-Json -Depth 5

$access = $tokens.access_token
$refresh = $tokens.refresh_token

Write-Host "`n== Verificar /v1/me con access_token ==" -ForegroundColor Cyan
$me = Invoke-RestMethod -Method GET "$Base/v1/me" -Headers @{ "Authorization" = "Bearer $access" }
$me | ConvertTo-Json -Depth 5

Write-Host "`n== Probar refresh_token ==" -ForegroundColor Cyan
# 1) Intento con JSON (muchos handlers internos usan JSON)
try {
  $ref = Invoke-RestMethod -Method POST "$Base/v1/auth/refresh" `
    -ContentType "application/json" `
    -Body (@{ client_id = $ClientId; refresh_token = $refresh } | ConvertTo-Json)
} catch {
  Write-Host "Intento JSON falló, probando form-urlencoded..." -ForegroundColor DarkYellow
  # 2) Fallback a x-www-form-urlencoded (estilo OAuth clásico)
  $ref = Invoke-RestMethod -Method POST "$Base/v1/auth/refresh" `
    -ContentType "application/x-www-form-urlencoded" `
    -Body @{ client_id = $ClientId; refresh_token = $refresh }
}
$ref | ConvertTo-Json -Depth 5

Write-Host "`nOK: flujo social Google validado." -ForegroundColor Green
