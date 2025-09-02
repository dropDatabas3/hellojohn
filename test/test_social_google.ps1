Param(
  [string]$Base = $env:BASE,
  [string]$Tenant = $env:TENANT,
  [string]$Client = $env:CLIENT
)

if (-not $Base)   { $Base   = "http://localhost:8080" }
if (-not $Tenant) { Write-Host "[FAIL] Falta TENANT (env o parámetro)"; exit 1 }
if (-not $Client) { Write-Host "[FAIL] Falta CLIENT (env o parámetro)"; exit 1 }

Write-Host "[info] BASE=$Base  TENANT=$Tenant  CLIENT=$Client"

function Get-LocationHeader {
  param([string]$Url)
  try {
    $resp = Invoke-WebRequest -Uri $Url -MaximumRedirection 0 -ErrorAction Stop
    return $resp.Headers["Location"]
  } catch {
    # Para 302, PowerShell tira excepción; capturamos headers
    if ($_.Exception.Response) {
      return $_.Exception.Response.Headers["Location"]
    }
    throw
  }
}

# 1) START → 302 a Google con state
$startUrl = "$Base/v1/auth/social/google/start?tenant_id=$Tenant&client_id=$Client"
Write-Host "[step] GET $startUrl"
$loc = Get-LocationHeader -Url $startUrl
if (-not $loc) { Write-Host "[FAIL] no hubo Location en la respuesta"; exit 1 }
if ($loc -notmatch "accounts\.google\.com") { Write-Host "[FAIL] redirect no va a Google: $loc"; exit 1 }
Write-Host "[ok] redirect a Google: $loc"

# extrae state de la Location
try {
  $uri = [System.Uri]$loc
  $q = [System.Web.HttpUtility]::ParseQueryString($uri.Query)
  $state = $q["state"]
} catch { $state = $null }

if (-not $state) { Write-Host "[FAIL] no se encontró 'state' en la Location"; exit 1 }
Write-Host "[info] state capturado (len=$($state.Length))"

# 2) Negativo: callback con state adulterado (debe 400 invalid_request)
$tampered = $state.Substring(0, [Math]::Max(0, $state.Length-2)) + "zz"
$cbTamper = "$Base/v1/auth/social/google/callback?state=$tampered&code=fake"
Write-Host "[step] NEG: GET $cbTamper"
try {
  $r = Invoke-WebRequest -Uri $cbTamper -ErrorAction Stop
  Write-Host "[FAIL] callback adulterado devolvió $($r.StatusCode) (esperado 400)"
  exit 1
} catch {
  $resp = $_.Exception.Response
  if ($resp.StatusCode.value__ -ne 400) {
    Write-Host "[FAIL] callback adulterado status=$($resp.StatusCode) (esperado 400)"
    exit 1
  }
  Write-Host "[ok] state adulterado → 400 OK"
}

# 3) Negativo: callback sin params (debe 400)
$cbMissing = "$Base/v1/auth/social/google/callback"
Write-Host "[step] NEG: GET $cbMissing (sin params)"
try {
  $r2 = Invoke-WebRequest -Uri $cbMissing -ErrorAction Stop
  Write-Host "[FAIL] callback sin params devolvió $($r2.StatusCode) (esperado 400)"
  exit 1
} catch {
  $resp = $_.Exception.Response
  if ($resp.StatusCode.value__ -ne 400) {
    Write-Host "[FAIL] callback sin params status=$($resp.StatusCode) (esperado 400)"
    exit 1
  }
  Write-Host "[ok] sin params → 400 OK"
}

# 4) Paso manual E2E: abrir Google y finalizar login; el backend devolverá JSON con access/refresh.
Write-Host "[info] Abrimos navegador para login real en Google…"
Start-Process $loc | Out-Null
Write-Host "[info] Completá el login en Google. Al regresar a '$Base', vas a ver JSON con access_token/refresh_token."
$ans = Read-Host "¿Listo? (y/n)"
if ($ans -ne "y") { Write-Host "[SKIP] E2E manual cancelado"; exit 0 }

# 5) Validar /v1/me con access_token pegado por el usuario
$atk = Read-Host "Pegá aquí el access_token"
if (-not $atk) { Write-Host "[FAIL] sin access_token"; exit 1 }

Write-Host "[step] GET /v1/me con Bearer"
try {
  $me = Invoke-WebRequest -Uri "$Base/v1/me" -Headers @{ "Authorization" = "Bearer $atk" } -ErrorAction Stop
  Write-Host "[ok] /v1/me OK"
  Write-Host $me.Content
} catch {
  $resp = $_.Exception.Response
  Write-Host "[FAIL] /v1/me status=$($resp.StatusCode)"
  $body = New-Object System.IO.StreamReader($resp.GetResponseStream()).ReadToEnd()
  Write-Host $body
  exit 1
}

Write-Host "[DONE] Social Google OK"
exit 0
