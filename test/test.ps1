param(
  [string]$TenantId     = '77407778-1e19-4623-a61a-0593ebcfa08e',
  [string]$ClientId     = 'web-frontend',
  [string]$Base         = 'http://localhost:8080',
  [int]$RateBurst       = 0,
  [string]$RealEmail    = 'juan@gmail.com',
  [switch]$AutoMail     = $false,   # auto-click a links (no prompts)
  [switch]$UseRealEmail = $false,   # manda a RealEmail exacto (sin plus)
  [string]$EmailTag     = ''        # si no se da, se usa random para plus
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
Set-StrictMode -Version Latest

# ───────── Helpers ─────────
function B64Url([byte[]]$bytes){
  [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+','-').Replace('/','_')
}
function Sha256Url([string]$s){
  $sha = [System.Security.Cryptography.SHA256]::Create()
  $hash = $sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($s))
  B64Url $hash
}
function AtHash([string]$access){
  $sha = [System.Security.Cryptography.SHA256]::Create()
  $hash = $sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($access))
  B64Url $hash[0..15]
}
function New-CodeVerifier([int]$n=32){
  $rnd = New-Object byte[] ($n)
  [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($rnd)
  B64Url $rnd
}
function New-RandomStr([int]$n=16){
  $rnd = New-Object byte[] ($n)
  [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($rnd)
  B64Url $rnd
}
function QS([string]$url, [string]$key){
  $qs = ([Uri]$url).Query.TrimStart('?')
  foreach($p in ($qs -split '&')){
    if(-not $p){ continue }
    $kv = $p -split '=',2
    if(($kv[0]) -eq $key){
      if($kv.Count -gt 1){ return [Uri]::UnescapeDataString($kv[1]) } else { return '' }
    }
  }
  return $null
}
function Decode-Part([string]$p){
  $p=$p.Replace('-','+').Replace('_','/'); while(($p.Length%4)-ne 0){$p+='='}
  [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($p))
}
function Assert($cond, [string]$msg){
  if (-not $cond) { throw "ASSERT FAILED: $msg" }
}
function Parse-Err([string]$s){
  try { return ($s | ConvertFrom-Json) } catch { return $null }
}
# ConvertFrom-Json compatible (usa -Depth si existe)
function FromJson([string]$s){
  try {
    $cmd = Get-Command ConvertFrom-Json -ErrorAction SilentlyContinue
    if ($cmd -and $cmd.Parameters.ContainsKey('Depth')) {
      return ($s | ConvertFrom-Json -Depth 50)
    } else {
      return ($s | ConvertFrom-Json)
    }
  } catch {
    return ($s | ConvertFrom-Json)
  }
}
# Obtiene un claim de forma robusta
function Get-Claim($obj, [string]$name){
  if ($null -eq $obj) { return $null }
  $p = $obj.PSObject.Properties[$name]
  if ($p) { return $p.Value }
  $p2 = $obj.PSObject.Properties | Where-Object { $_.Name -ieq $name } | Select-Object -First 1
  if ($p2) { return $p2.Value }
  if ($obj -is [System.Collections.IDictionary]) {
    if ($obj.Contains($name)) { return $obj[$name] }
    foreach($k in $obj.Keys){ if (($k -is [string]) -and ($k -ieq $name)) { return $obj[$k] } }
  }
  return $null
}

# Email plus addressing para Gmail (juan+tag@gmail.com)
function Compose-PlusEmail([string]$base,[string]$tag){
  $m = [regex]::Match($base, '^(?<local>[^@+]+)(?:\+[^@]+)?@(?<domain>.+)$')
  if(-not $m.Success){ return $base }
  return ("{0}+{1}@{2}" -f $m.Groups['local'].Value,$tag,$m.Groups['domain'].Value)
}

# NUEVO: sanitiza el tag (solo letras/números . _ -)
function Sanitize-EmailTag([string]$tag){
  if ([string]::IsNullOrWhiteSpace($tag)) { return $null }
  $clean = ($tag -replace '[^A-Za-z0-9._-]','')
  if ([string]::IsNullOrWhiteSpace($clean)) { return $null }
  return $clean
}

# 302 robusto
function Get-Redirect302 {
  param(
    [Parameter(Mandatory=$true)][string]$Url,
    $Headers = $null,
    $WebSession = $null
  )
  $req = [System.Net.HttpWebRequest]::Create($Url)
  $req.Method = 'GET'
  $req.AllowAutoRedirect = $false
  $req.Timeout = 15000

  if ($Headers) {
    foreach($k in $Headers.Keys){
      $v = [string]$Headers[$k]
      switch -Regex ($k.ToLower()) {
        '^user-agent$'       { $req.UserAgent = $v; continue }
        '^accept$'           { $req.Accept    = $v; continue }
        '^referer$'          { $req.Referer   = $v; continue }
        '^host$'             { continue }           # no setear
        '^content-length$'   { continue }           # no setear
        default              { $req.Headers[$k] = $v }
      }
    }
  }

  if ($WebSession -and $WebSession.PSObject.Properties['Cookies']) {
    $req.CookieContainer = New-Object System.Net.CookieContainer
    $uri = [Uri]$Url
    try {
      $cookies = $WebSession.Cookies.GetCookies($uri)
      foreach($c in $cookies){ $req.CookieContainer.Add($uri, $c) }
    } catch {}
  }

  try { $resp = $req.GetResponse() } catch [System.Net.WebException] { $resp = $_.Exception.Response; if (-not $resp) { throw } }
  $sc  = [int]$resp.StatusCode
  $loc = $resp.Headers['Location']
  return [pscustomobject]@{ StatusCode = $sc; Location = $loc }
}

# ───────── Smoke ─────────
Write-Host "[step] GET /healthz"
(Invoke-WebRequest -Uri "$Base/healthz").Content | Write-Host

Write-Host "[step] GET /readyz"
(Invoke-WebRequest -Uri "$Base/readyz").Content | Write-Host

Write-Host "[step] GET /.well-known/jwks.json"
$jwks = (Invoke-WebRequest -Uri "$Base/.well-known/jwks.json")
Assert ($jwks.StatusCode -eq 200) "JWKS debe 200"
Assert ($jwks.Content -match '"kty":"OKP"') "JWKS debe contener Ed25519"

# ───────── CORS ─────────
Write-Host "[step] OPTIONS /v1/auth/login (CORS permitido)"
$pre1 = Invoke-WebRequest "$Base/v1/auth/login" -Method OPTIONS -Headers @{
  Origin='http://localhost:3000'
  'Access-Control-Request-Method'='POST'
  'Access-Control-Request-Headers'='content-type, authorization'
}
Assert ($pre1.StatusCode -eq 204) "preflight permitido debe 204"
Assert ($pre1.Headers['Access-Control-Allow-Origin'] -eq 'http://localhost:3000') "ACAO incorrecto"

Write-Host "[step] OPTIONS /v1/auth/login (CORS NO permitido)"
$pre2 = Invoke-WebRequest "$Base/v1/auth/login" -Method OPTIONS -Headers @{
  Origin='http://malote.com'
  'Access-Control-Request-Method'='POST'
  'Access-Control-Request-Headers'='content-type'
}
Assert ($pre2.StatusCode -eq 204) "preflight denegado igual 204"
Assert (-not $pre2.Headers['Access-Control-Allow-Origin']) "No debe ACAO"

# ───────── Email destino ─────────
if ($UseRealEmail) {
  $EMAIL = $RealEmail
  Write-Host "[cfg] EMAIL destino para pruebas: $EMAIL (exacto)"
} else {
  $t = Sanitize-EmailTag $EmailTag
  if (-not $t) { $t = "login" + (Get-Random -Max 100000) }
  $EMAIL = Compose-PlusEmail $RealEmail $t
  Write-Host "[cfg] EMAIL destino para pruebas: $EMAIL (base: $RealEmail, tag: $t)"
}

# ───────── Password flow ─────────
$InitialPassword = 'SuperSecreta1!'
$NewPassword     = 'NuevaPassFuerte1!'

Write-Host "[step] POST /v1/auth/register email=$($EMAIL)"
$regBody = @{ tenant_id=$TenantId; client_id=$ClientId; email=$EMAIL; password=$InitialPassword } | ConvertTo-Json
$regResp = Invoke-WebRequest "$Base/v1/auth/register" -Method POST -ContentType 'application/json' -Body $regBody
$reg = $regResp.Content | ConvertFrom-Json
Assert ($reg.access_token)  "register debe access_token"
Assert ($reg.refresh_token) "register debe refresh_token"

Write-Host "[step] POST /v1/auth/login]"
$loginBody = @{ tenant_id=$TenantId; client_id=$ClientId; email=$EMAIL; password=$InitialPassword } | ConvertTo-Json
$loginResp = Invoke-WebRequest "$Base/v1/auth/login" -Method POST -ContentType 'application/json' -Body $loginBody
$login = $loginResp.Content | ConvertFrom-Json

$access  = if ($login.access_token)  { $login.access_token  } else { $reg.access_token }
$refresh = if ($login.refresh_token) { $login.refresh_token } else { $reg.refresh_token }

Write-Host "[step] GET /v1/me (Bearer access)"
$meResp = Invoke-WebRequest "$Base/v1/me" -Headers @{ Authorization = "Bearer $access" }
$me = $meResp.Content | ConvertFrom-Json
Assert ($me.sub) "me.sub presente"
Assert ($me.tid -eq $TenantId) "me.tid != tenant"

$parts = $access -split '\.'
$hdr = Decode-Part $parts[0] | ConvertFrom-Json
$pld = Decode-Part $parts[1] | ConvertFrom-Json
Assert ($hdr.alg -eq 'EdDSA') "alg != EdDSA"
Assert ($pld.aud -eq $ClientId) "aud != client"
Assert ($pld.tid -eq $TenantId) "tid != tenant"
Assert ($pld.iss -eq $Base) "iss != base"

Write-Host "[step] POST /v1/auth/refresh (rotación)"
$refBody = @{ tenant_id=$TenantId; client_id=$ClientId; refresh_token=$refresh } | ConvertTo-Json
$ref1Resp = Invoke-WebRequest "$Base/v1/auth/refresh" -Method POST -ContentType 'application/json' -Body $refBody
$ref1 = $ref1Resp.Content | ConvertFrom-Json
$newRefresh = $ref1.refresh_token
Assert ($newRefresh) "refresh debe rotarse"

Write-Host "[step] reuse old refresh (esperado 401)"
try {
  $null = (Invoke-WebRequest "$Base/v1/auth/refresh" -Method POST -ContentType 'application/json' -Body $refBody).Content
  throw 'Old refresh NO fue rechazado'
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  $err  = Parse-Err $_.ErrorDetails.Message
  Assert ($code -eq 401) "Old refresh debe 401"
  Assert ($err.error) "Respuesta error estructurada"
}

Write-Host "[step] POST /v1/auth/logout (revoca)"
$logoutBody = @{ tenant_id=$TenantId; client_id=$ClientId; refresh_token=$newRefresh } | ConvertTo-Json
$logoutResp = Invoke-WebRequest "$Base/v1/auth/logout" -Method POST -ContentType 'application/json' -Body $logoutBody
Assert ($logoutResp.StatusCode -eq 204) "logout debe 204"

Write-Host "[step] refresh con token revocado (esperado 401)"
try {
  $null = (Invoke-WebRequest "$Base/v1/auth/refresh" -Method POST -ContentType 'application/json' -Body $logoutBody).Content
  throw 'Refresh revocado NO fue rechazado'
} catch {
  Assert ($_.Exception.Response.StatusCode.value__ -eq 401) "Revocado debe 401"
}

# ───────── Email flows: verify + forgot/reset ─────────
$redirectUri = 'http://localhost:3000/callback'  # debe estar permitido en el client

# VERIFY EMAIL
Write-Host "[email] POST /v1/auth/verify-email/start -> debería llegarte VERIFICACION"
$vstart = Invoke-WebRequest "$Base/v1/auth/verify-email/start" `
  -Method POST -ContentType 'application/json' `
  -Headers @{ Authorization = "Bearer $access" } `
  -Body (@{ tenant_id=$TenantId; client_id=$ClientId; redirect_uri=$redirectUri } | ConvertTo-Json)

$vlink = $vstart.Headers['X-Debug-Verify-Link']
if ($AutoMail) {
  if ($vlink) {
    $vres = Invoke-WebRequest $vlink -MaximumRedirection 0 -ErrorAction SilentlyContinue
    Write-Host " [auto] GET verify -> HTTP $($vres.StatusCode)"
  } else { Write-Host " [auto] verify: no X-Debug-Verify-Link header; skipping" }
} else {
  $ansV = (Read-Host ("¿Llegó el mail de VERIFICACION a {0}? (y/n/auto)" -f $EMAIL)).Trim().ToLower()
  if ($ansV -eq 'auto' -and $vlink) {
    $vres = Invoke-WebRequest $vlink -MaximumRedirection 0 -ErrorAction SilentlyContinue
    Write-Host " [auto] GET verify -> HTTP $($vres.StatusCode)"
  } elseif ($ansV -ne 'y') {
    throw "No llegó el mail de verificación (o elegiste 'n')"
  }
}

# FORGOT / RESET
Write-Host "[email] POST /v1/auth/forgot -> debería llegarte RESET"
$fresp = Invoke-WebRequest "$Base/v1/auth/forgot" `
  -Method POST -ContentType 'application/json' `
  -Body (@{ tenant_id=$TenantId; client_id=$ClientId; email=$EMAIL; redirect_uri=$redirectUri } | ConvertTo-Json)

$flink = $fresp.Headers['X-Debug-Reset-Link']
if ($AutoMail) {
  if ($flink) {
    $token = QS $flink 'token'
    $r = Invoke-WebRequest "$Base/v1/auth/reset" `
      -Method POST -ContentType 'application/json' `
      -Body (@{ tenant_id=$TenantId; client_id=$ClientId; token=$token; new_password=$NewPassword } | ConvertTo-Json)
    try {
      $rt = $r.Content | ConvertFrom-Json
      if ($rt.access_token) { Write-Host " [auto] reset -> tokens emitidos (auto_login=true)" }
    } catch { Write-Host " [auto] reset -> 204 (sin tokens) o respuesta no JSON" }
  } else { Write-Host " [auto] reset: no X-Debug-Reset-Link header; skipping" }
} else {
  $ansR = (Read-Host ("¿Llegó el mail de RESET a {0}? (y/n/auto)" -f $EMAIL)).Trim().ToLower()
  if ($ansR -eq 'auto' -and $flink) {
    $token = QS $flink 'token'
    $r = Invoke-WebRequest "$Base/v1/auth/reset" `
      -Method POST -ContentType 'application/json' `
      -Body (@{ tenant_id=$TenantId; client_id=$ClientId; token=$token; new_password=$NewPassword } | ConvertTo-Json)
    try {
      $rt = $r.Content | ConvertFrom-Json
      if ($rt.access_token) { Write-Host " [auto] reset -> tokens emitidos (auto_login=true)" }
    } catch { Write-Host " [auto] reset -> 204 (sin tokens) o respuesta no JSON" }
  } elseif ($ansR -ne 'y') {
    throw "No llegó el mail de reset (o elegiste 'n')"
  }
}

# ───────── OIDC: sesión por cookie ─────────
$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession

Write-Host "[oidc] POST /v1/session/login (set-cookie)"
$null = Invoke-WebRequest "$Base/v1/session/login" -Method POST -WebSession $session -ContentType 'application/json' -Body ($loginBody)
Assert ($session.Cookies.Count -ge 1) "No se setearon cookies de sesión"

$state = New-RandomStr 16
$nonce = New-RandomStr 16
$verifier = New-CodeVerifier 40
$challenge = Sha256Url $verifier

$authUrl = "$Base/oauth2/authorize?response_type=code&client_id=$([uri]::EscapeDataString($ClientId))&redirect_uri=$([uri]::EscapeDataString($redirectUri))&scope=$([uri]::EscapeDataString('openid email'))&state=$state&nonce=$nonce&code_challenge=$challenge&code_challenge_method=S256"

Write-Host "[oidc] GET /oauth2/authorize (cookie session) -> 302 code"
$auth302 = Get-Redirect302 -Url $authUrl -WebSession $session
Assert ($auth302.StatusCode -eq 302) "authorize debe 302"
$loc = $auth302.Location
Assert ($loc) "Location vacío"
$code = QS $loc 'code'
$stRet = QS $loc 'state'
Assert ($code) "code vacío"
Assert ($stRet -eq $state) "state no coincide"

Write-Host "[oidc] POST /oauth2/token (authorization_code + PKCE)"
$tokenResp = Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
  grant_type    = 'authorization_code'
  code          = $code
  redirect_uri  = $redirectUri
  client_id     = $ClientId
  code_verifier = $verifier
}
$tok = $tokenResp.Content | ConvertFrom-Json
Assert ($tok.access_token)  "token: access vacío"
Assert ($tok.refresh_token) "token: refresh vacío"
Assert ($tok.id_token)      "token: id_token vacío"
Assert ($tok.token_type -eq 'Bearer') "token_type != Bearer"

# Decodificar el payload del ID Token (compatible PS5/PS7)
$idpJson = Decode-Part (($tok.id_token -split '\.')[1])
$idp = FromJson $idpJson

# Obtener at_hash de forma robusta
$atHashClaim = Get-Claim $idp 'at_hash'
if (-not $atHashClaim) {
  $props = ($idp.PSObject.Properties.Name | Sort-Object) -join ', '
  Write-Host "[debug] id_token claims: $props"
}
Assert ($atHashClaim) "id_token.at_hash ausente"
Assert ($atHashClaim -eq (AtHash $tok.access_token)) "id_token.at_hash no coincide"

# Verificar nonce si se envió
$nonceClaim = Get-Claim $idp 'nonce'
if ($nonce) { Assert ($nonceClaim -eq $nonce) "nonce no coincide" }

Write-Host "[oidc] GET /userinfo (email scope)"
$ui = (Invoke-WebRequest "$Base/userinfo" -Headers @{ Authorization="Bearer $($tok.access_token)" }).Content | ConvertFrom-Json
Assert ($ui.sub) "userinfo.sub vacío"
Assert ($ui.email) "userinfo.email vacío (scope email solicitado)"

# ───────── OAuth2: refresh grant + rotación ─────────
Write-Host "[oidc] POST /oauth2/token (refresh_token)"
$rt1 = $tok.refresh_token
$ref2 = Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
  grant_type    = 'refresh_token'
  client_id     = $ClientId
  refresh_token = $rt1
}
$tk2 = $ref2.Content | ConvertFrom-Json
Assert ($tk2.refresh_token -and $tk2.refresh_token -ne $rt1) "debe rotar refresh"

Write-Host "[oidc] reusar refresh viejo (esperado invalid_grant)"
try {
  $null = (Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
    grant_type    = 'refresh_token'
    client_id     = $ClientId
    refresh_token = $rt1
  }).Content
  throw 'refresh viejo NO fue rechazado'
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  $err  = Parse-Err $_.ErrorDetails.Message
  Assert ($code -in 400,401) "esperado 400/401"
  Assert ($err.error -eq 'invalid_grant') "esperado invalid_grant"
}

# ───────── RFC7009 revoke ─────────
Write-Host "[oidc] POST /oauth2/revoke (refresh actual) => 200 idempotente"
$rev = Invoke-WebRequest "$Base/oauth2/revoke" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
  token = $tk2.refresh_token
  token_type_hint = 'refresh_token'
}
Assert ($rev.StatusCode -eq 200) "revoke debe 200"

Write-Host "[oidc] refresh luego de revoke (esperado invalid_grant)"
try {
  $null = (Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
    grant_type    = 'refresh_token'
    client_id     = $ClientId
    refresh_token = $tk2.refresh_token
  }).Content
  throw 'refresh post-revoke NO fue rechazado'
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  $err  = Parse-Err $_.ErrorDetails.Message
  Assert ($code -in 400,401) "esperado 400/401"
  Assert ($err.error -eq 'invalid_grant') "esperado invalid_grant"
}

# ───────── Negativos OIDC ─────────
Write-Host "[oidc:neg] code_verifier inválido"
$verifier2  = New-CodeVerifier 40
$challenge2 = Sha256Url $verifier2
$state2 = New-RandomStr 8
$nonce2 = New-RandomStr 8
$authUrl2 = "$Base/oauth2/authorize?response_type=code&client_id=$([uri]::EscapeDataString($ClientId))&redirect_uri=$([uri]::EscapeDataString($redirectUri))&scope=openid&state=$state2&nonce=$nonce2&code_challenge=$challenge2&code_challenge_method=S256"
$ar2 = Get-Redirect302 -Url $authUrl2 -Headers @{ Authorization="Bearer $access" }
$loc2 = $ar2.Location
$code2 = QS $loc2 'code'
try {
  $null = (Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
    grant_type    = 'authorization_code'
    code          = $code2
    redirect_uri  = $redirectUri
    client_id     = $ClientId
    code_verifier = 'wr0ngverifier'
  }).Content
  throw 'PKCE inválido NO falló'
} catch {
  $err = Parse-Err $_.ErrorDetails.Message
  Assert ($err.error -eq 'invalid_grant') "esperado invalid_grant (PKCE)"
}

Write-Host "[oidc:neg] reuso de authorization code"
try {
  $null = (Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
    grant_type    = 'authorization_code'
    code          = $code2
    redirect_uri  = $redirectUri
    client_id     = $ClientId
    code_verifier = $verifier2
  }).Content
  throw 'reuso de code NO falló'
} catch {
  $err = Parse-Err $_.ErrorDetails.Message
  Assert ($err.error -eq 'invalid_grant') "esperado invalid_grant (reuso)"
}

Write-Host "[oidc:neg] redirect_uri distinto"
$state3 = New-RandomStr 6
$verifier3  = New-CodeVerifier 40
$challenge3 = Sha256Url $verifier3
$ar3 = Get-Redirect302 -Url "$Base/oauth2/authorize?response_type=code&client_id=$ClientId&redirect_uri=$([uri]::EscapeDataString($redirectUri))&scope=openid&state=$state3&code_challenge=$challenge3&code_challenge_method=S256" -Headers @{ Authorization="Bearer $access" }
$loc3  = $ar3.Location; $code3 = QS $loc3 'code'
try {
  $null = (Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
    grant_type    = 'authorization_code'
    code          = $code3
    redirect_uri  = 'http://localhost:3000/otra'
    client_id     = $ClientId
    code_verifier = $verifier3
  }).Content
  throw 'redirect_uri mismatch NO falló'
} catch {
  $err = Parse-Err $_.ErrorDetails.Message
  Assert ($err.error -eq 'invalid_grant') "esperado invalid_grant (redirect_uri)"
}

Write-Host "[oidc:neg] scope no permitido -> invalid_scope en redirect"
$ar4 = Get-Redirect302 -Url "$Base/oauth2/authorize?response_type=code&client_id=$ClientId&redirect_uri=$([uri]::EscapeDataString($redirectUri))&scope=$([uri]::EscapeDataString('openid admin'))&state=stX&code_challenge=$challenge3&code_challenge_method=S256" -Headers @{ Authorization="Bearer $access" }
$loc4 = $ar4.Location
Assert ((QS $loc4 'error') -eq 'invalid_scope') "esperado error=invalid_scope"

Write-Host "[oidc:neg] sin openid"
try {
  $null = Invoke-WebRequest "$Base/oauth2/authorize?response_type=code&client_id=$ClientId&redirect_uri=$([uri]::EscapeDataString($redirectUri))&scope=email&state=stY&code_challenge=$challenge3&code_challenge_method=S256" -MaximumRedirection 0 -Headers @{ Authorization="Bearer $access" } -ErrorAction Stop
  throw 'sin openid NO falló'
} catch {
  $code = try { $_.Exception.Response.StatusCode.value__ } catch { 400 }
  Assert ($code -eq 400) "esperado 400 invalid_scope/invalid_request"
}

Write-Host "[oidc:neg] login_required (sin sesión)"
$ar5 = Get-Redirect302 -Url "$Base/oauth2/authorize?response_type=code&client_id=$ClientId&redirect_uri=$([uri]::EscapeDataString($redirectUri))&scope=openid&state=stZ&code_challenge=$challenge3&code_challenge_method=S256"
$loc5 = $ar5.Location
Assert ((QS $loc5 'error') -eq 'login_required') "esperado error=login_required"

Write-Host "[oidc] authorize con Authorization: Bearer (modo dev)"
$stateB = New-RandomStr 8
$verifierB = New-CodeVerifier 40
$challengeB = Sha256Url $verifierB
$arB = Get-Redirect302 -Url "$Base/oauth2/authorize?response_type=code&client_id=$ClientId&redirect_uri=$([uri]::EscapeDataString($redirectUri))&scope=openid%20email&state=$stateB&nonce=nb&code_challenge=$challengeB&code_challenge_method=S256" -Headers @{ Authorization="Bearer $access" }
$locB = $arB.Location; $codeB = QS $locB 'code'
Assert ($codeB) "no llegó code con bearer-session"

Write-Host "[session] POST /v1/session/logout"
$sl = Invoke-WebRequest "$Base/v1/session/logout" -Method POST -WebSession $session
Assert ($sl.StatusCode -in 200,204) "logout sesión debe 200/204"

# ───────── Rate limit (opcional) ─────────
if ($RateBurst -gt 0) {
  Write-Host "[step] rate limit: $RateBurst hits rápidos a /healthz"
  $got429 = $false
  for ($i=1; $i -le $RateBurst; $i++) {
    try {
      $resp = Invoke-WebRequest "$Base/healthz"
      $rem  = $resp.Headers['X-RateLimit-Remaining']
      if ($rem) { Write-Host ("  hit {0} remaining={1}" -f $i, $rem) }
    } catch {
      if ($_.Exception.Response.StatusCode.value__ -eq 429) {
        $got429 = $true
        $ra = $_.Exception.Response.Headers['Retry-After']
        Write-Host "  -> 429 recibido (Retry-After=$ra)"
        break
      } else { throw $_ }
    }
  }
  Assert ($got429) "No se alcanzó 429 (¿rate.limit off o umbral alto?)"
}

# ───────── Extra OIDC hardening checks ─────────
Write-Host "[oidc] GET /.well-known/openid-configuration (discovery)"
$disc = Invoke-WebRequest "$Base/.well-known/openid-configuration"
$discObj = $disc.Content | ConvertFrom-Json
Assert ($discObj.issuer -eq $Base) "discovery.issuer != base"
Assert ($discObj.authorization_endpoint -eq "$Base/oauth2/authorize") "authorization_endpoint mal"
Assert ($discObj.token_endpoint -eq "$Base/oauth2/token") "token_endpoint mal"
Assert ($discObj.userinfo_endpoint -eq "$Base/userinfo") "userinfo_endpoint mal"
Assert ($discObj.jwks_uri -eq "$Base/.well-known/jwks.json") "jwks_uri mal"
Assert ($discObj.id_token_signing_alg_values_supported -contains "EdDSA") "id_token alg faltante"
Assert ($discObj.code_challenge_methods_supported -contains "S256") "PKCE S256 faltante"
Assert ($discObj.token_endpoint_auth_methods_supported -contains "none") "token_endpoint_auth_methods faltante"

Write-Host "[oidc] HEAD discovery/jwks + cache headers"
$discHead = Invoke-WebRequest "$Base/.well-known/openid-configuration" -Method HEAD
Assert ($discHead.StatusCode -eq 200) "HEAD discovery debe 200"
Assert ($discHead.Headers['Cache-Control'] -match 'max-age') "discovery sin Cache-Control"

$jwksHead = Invoke-WebRequest "$Base/.well-known/jwks.json" -Method HEAD
Assert ($jwksHead.StatusCode -eq 200) "HEAD jwks debe 200"
Assert ($jwksHead.Headers['Cache-Control'] -match 'max-age') "jwks sin Cache-Control"

# Ya teníamos GET JWKS; validemos Content-Type
Assert ($jwks.Headers['Content-Type'] -like 'application/json*') "JWKS content-type"

Write-Host "[oidc] token headers (no-store/no-cache) + campos"
Assert ($tokenResp.Headers['Cache-Control'] -eq 'no-store') "token (auth_code) sin no-store"
Assert ($tokenResp.Headers['Pragma'] -eq 'no-cache') "token (auth_code) sin no-cache"
Assert ($tok.scope -match 'openid' -and $tok.scope -match 'email') "scope echo faltante"
Assert ([int]$tok.expires_in -gt 0) "expires_in <= 0"

Assert ($ref2.Headers['Cache-Control'] -eq 'no-store') "token (refresh) sin no-store"
Assert ($ref2.Headers['Pragma'] -eq 'no-cache') "token (refresh) sin no-cache"

Write-Host "[oidc] ID Token: azp y tid"
$azpClaim = Get-Claim $idp 'azp'
$tidClaim = Get-Claim $idp 'tid'
Assert ($azpClaim -eq $ClientId) "id_token.azp != client_id"
Assert ($tidClaim -eq $TenantId) "id_token.tid != tenant_id"

Write-Host "[oidc] /userinfo POST"
$uiPost = (Invoke-WebRequest "$Base/userinfo" -Method POST -Headers @{ Authorization="Bearer $($tok.access_token)" }).Content | ConvertFrom-Json
Assert ($uiPost.sub) "userinfo POST: sub vacío"

Write-Host "[oidc:neg] /userinfo sin bearer => 401"
try {
  $null = (Invoke-WebRequest "$Base/userinfo" -Method GET).Content
  throw "userinfo sin bearer NO falló"
} catch {
  Assert ($_.Exception.Response.StatusCode.value__ -eq 401) "userinfo sin bearer debe 401"
}

Write-Host "[oidc:neg] /oauth2/token unsupported_grant_type"
try {
  $null = (Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{ grant_type='foobar' }).Content
  throw "unsupported_grant_type NO falló"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  $err  = Parse-Err $_.ErrorDetails.Message
  Assert ($code -eq 400) "esperado 400"
  Assert ($err.error -eq 'unsupported_grant_type') "error esperado unsupported_grant_type"
}

Write-Host "[oidc:neg] /oauth2/token refresh con client_id inexistente => invalid_client"
try {
  $null = (Invoke-WebRequest "$Base/oauth2/token" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
    grant_type='refresh_token'; client_id='no-such-client'; refresh_token=$tk2.refresh_token
  }).Content
  throw "invalid_client NO falló"
} catch {
  $code = $_.Exception.Response.StatusCode.value__
  $err  = Parse-Err $_.ErrorDetails.Message
  Assert ($code -in 400,401) "esperado 400/401"
  Assert ($err.error -eq 'invalid_client') "esperado invalid_client"
}

Write-Host "[oidc:neg] /oauth2/authorize con redirect NO permitido => 400 invalid_redirect_uri"
$badRedirect = "http://evil.example/cb"
try {
  $null = Invoke-WebRequest "$Base/oauth2/authorize?response_type=code&client_id=$ClientId&redirect_uri=$([uri]::EscapeDataString($badRedirect))&scope=openid&state=bad&code_challenge=$challenge&code_challenge_method=S256" `
    -Headers @{ Authorization="Bearer $access" } -ErrorAction Stop
  throw "invalid_redirect_uri NO falló"
} catch {
  $code = try { $_.Exception.Response.StatusCode.value__ } catch { 400 }
  $err  = Parse-Err $_.ErrorDetails.Message
  Assert ($code -eq 400) "esperado 400"
  Assert ($err.error -eq 'invalid_redirect_uri') "esperado invalid_redirect_uri"
}

Write-Host "[oidc] invalid_scope preserva state en redirect"
Assert ((QS $loc4 'state') -eq 'stX') "state no preservado en error redirect"

Write-Host "[oidc] revoke de token inexistente => 200"
$rev2 = Invoke-WebRequest "$Base/oauth2/revoke" -Method POST -ContentType 'application/x-www-form-urlencoded' -Body @{
  token = "not-a-real-refresh-token"
  token_type_hint = 'refresh_token'
}
Assert ($rev2.StatusCode -eq 200) "revoke inexistente debe 200"

Write-Host "[DONE] Tests OK"
exit 0
