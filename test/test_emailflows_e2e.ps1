param(
  [string]$Base   = "http://localhost:8080",
  [string]$Tenant = "95f317cd-28f0-4bae-a0aa-4a4ddff92a16",
  [string]$Client = "web-frontend",
  [string]$Admin  = "admin@rem.com",
  [string]$Pwd    = "supersecreta",
  [string]$Cb     = "http://localhost:3000/callback"
)

# Helpers (PS 5.1)
Add-Type -AssemblyName System.Web
Add-Type -AssemblyName System.Net.Http   # <-- NECESARIO en Windows PowerShell 5.1

function Get-QueryValueFromUrl($url, $name) {
  $u = [System.Uri]$url
  $q = [System.Web.HttpUtility]::ParseQueryString($u.Query)
  return $q[$name]
}

# ===== HttpClient helpers =====
$global:__hcHandler = New-Object System.Net.Http.HttpClientHandler
$global:__hcHandler.AllowAutoRedirect = $false
$global:__hc = New-Object System.Net.Http.HttpClient($global:__hcHandler)

function Send-PostJson([string]$url, [hashtable]$obj, [hashtable]$headers=@{}) {
  $json = ($obj | ConvertTo-Json -Compress -Depth 8)
  $content = New-Object System.Net.Http.StringContent($json, [System.Text.Encoding]::UTF8, "application/json")

  $req = New-Object System.Net.Http.HttpRequestMessage([System.Net.Http.HttpMethod]::Post, $url)
  $req.Content = $content
  foreach ($k in $headers.Keys) {
    $req.Headers.Remove($k) | Out-Null
    $req.Headers.Add($k, [string]$headers[$k])
  }

  $resp = $global:__hc.SendAsync($req).Result
  $body = $resp.Content.ReadAsStringAsync().Result
  $h = @{}
  foreach ($kvp in $resp.Headers) { $h[$kvp.Key] = ($kvp.Value -join ", ") }
  foreach ($kvp in $resp.Content.Headers) { $h[$kvp.Key] = ($kvp.Value -join ", ") }

  [pscustomobject]@{
    Status  = [int]$resp.StatusCode
    Body    = $body
    Headers = $h
  }
}

function Get-StatusNoRedirect([string]$url) {
  $req = New-Object System.Net.Http.HttpRequestMessage([System.Net.Http.HttpMethod]::Get, $url)
  $resp = $global:__hc.SendAsync($req).Result
  return [int]$resp.StatusCode
}

function Do-ForgotReset([string]$email) {
  Write-Host ">> ejecutando forgot/reset (email=$email)..."

  $fgBody = @{ tenant_id=$Tenant; client_id=$Client; email=$email; redirect_uri=$Cb }
  $fg = Send-PostJson "$Base/v1/auth/forgot" $fgBody
  if ($fg.Status -lt 200 -or $fg.Status -ge 300) {
    throw "forgot devolvió $($fg.Status): $($fg.Body)"
  }

  $resetLink = $fg.Headers['X-Debug-Reset-Link']
  if (-not $resetLink) { throw "No llegó X-Debug-Reset-Link. Para auto-reset, poné APP_ENV=dev y EMAIL_DEBUG_LINKS=true." }

  $token = Get-QueryValueFromUrl $resetLink "token"
  if (-not $token) { throw "No se pudo extraer token de reset del header." }

  $newPwd = "Nuev4Clave!" + (Get-Random -Minimum 1000 -Maximum 9999)
  $rsBody = @{ tenant_id=$Tenant; client_id=$Client; token=$token; new_password=$newPwd }
  $rs = Send-PostJson "$Base/v1/auth/reset" $rsBody

  if     ($rs.Status -eq 204) { Write-Host "reset -> 204 (sin autologin)" }
  elseif ($rs.Status -eq 200) { Write-Host "reset -> 200 (autologin: tokens emitidos)" }
  else   { throw "reset devolvió $($rs.Status): $($rs.Body)" }

  Write-Host ">> NUEVA CONTRASEÑA: $newPwd"
  return $newPwd
}

# ===== Flow =====
"== login =="
$access = $null
$currentPwd = $Pwd

# intento de login con la password actual
$login1 = Send-PostJson "$Base/v1/auth/login" @{ tenant_id=$Tenant; client_id=$Client; email=$Admin; password=$currentPwd }
if ($login1.Status -eq 200) {
  $login = $login1.Body | ConvertFrom-Json
  $access = $login.access_token
  "login -> 200"
} elseif ($login1.Status -eq 401) {
  "login falló (401), intentando forgot/reset para recuperar acceso..."
  try {
    $currentPwd = Do-ForgotReset $Admin
    "== login (nuevo) =="
    $login2 = Send-PostJson "$Base/v1/auth/login" @{ tenant_id=$Tenant; client_id=$Client; email=$Admin; password=$currentPwd }
    if ($login2.Status -ne 200) { throw "login (nuevo) devolvió $($login2.Status): $($login2.Body)" }
    $login  = $login2.Body | ConvertFrom-Json
    $access = $login.access_token
    "login (nuevo) -> 200"
  } catch {
    "No se pudo recuperar sesión: $($_.Exception.Message)"
    "== ABORTADO =="
    # cleanup
    if ($global:__hc) { $global:__hc.Dispose() }
    if ($global:__hcHandler) { $global:__hcHandler.Dispose() }
    exit 1
  }
} else {
  "login devolvió $($login1.Status): $($login1.Body)"
  "== ABORTADO =="
  if ($global:__hc) { $global:__hc.Dispose() }
  if ($global:__hcHandler) { $global:__hcHandler.Dispose() }
  exit 1
}

"== verify-email/start =="
$vs = Send-PostJson "$Base/v1/auth/verify-email/start" @{ tenant_id=$Tenant; client_id=$Client; redirect_uri=$Cb } @{ Authorization = "Bearer $access" }
$verifyLink = $vs.Headers['X-Debug-Verify-Link']; if (-not $verifyLink) { $verifyLink = '<none>' }
"verify start -> $($vs.Status)  link=$verifyLink"

if ($verifyLink -ne '<none>') {
  $code = Get-StatusNoRedirect $verifyLink
  "verify confirm GET -> $code (esperado 302 si redirect_uri presente)"
} else {
  "saltando verify confirm (sin header; ¿APP_ENV=dev + EMAIL_DEBUG_LINKS=true?)"
}

"== forgot/reset (forzando un segundo cambio) =="
try {
  $newPwd2 = Do-ForgotReset $Admin
  "== login con nueva contraseña =="
  $login3 = Send-PostJson "$Base/v1/auth/login" @{ tenant_id=$Tenant; client_id=$Client; email=$Admin; password=$newPwd2 }
  "login (nuevo) -> $($login3.Status) (esperado 200)"
} catch {
  "saltando reset final: $($_.Exception.Message)"
}

"== listo =="

# cleanup httpclient
if ($global:__hc) { $global:__hc.Dispose() }
if ($global:__hcHandler) { $global:__hcHandler.Dispose() }
