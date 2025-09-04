# e2e.ps1 (ASCII-safe para PS 5.1)
# Ejemplo:
# .\e2e.ps1 -Base http://localhost:8080 -Tenant 7bee1e9e-5003-482b-abd6-ffe9e66f7b37 -Client web-frontend `
#           -EmailBase john@gmail.com -Password 'Test1234A!' -DevAllowBearer `
#           -Redirect 'http://localhost:3000/callback'
# Si activas DebugEchoLinks en el YAML para email flows, podes usar:
#    -AutoEmail

Param(
  [string]$Base = "http://localhost:8080",
  [Parameter(Mandatory=$true)][string]$Tenant,
  [Parameter(Mandatory=$true)][string]$Client,
  [Parameter(Mandatory=$true)][string]$EmailBase,
  [string]$Password = "Test1234A!",
  [string]$Redirect = "http://localhost:3000/callback",
  [switch]$AutoEmail,
  [switch]$DevAllowBearer
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Web | Out-Null

function Step($msg){ Write-Host "[step] $msg" -ForegroundColor Cyan }
function Ok($msg){ Write-Host "[ok]  $msg" -ForegroundColor Green }
function Warn($msg){ Write-Host "[warn] $msg" -ForegroundColor Yellow }
function Fail($msg){ Write-Host "[FAIL] $msg" -ForegroundColor Red; exit 1 }
function Assert($cond, [string]$msg){ if(-not [bool]$cond){ Fail $msg } }

$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$JsonCT = "application/json; charset=utf-8"

# --- GET sin seguir redirecciones (evita excepcion en 3xx con PS 5.1) ---
function Invoke-NoFollowGet {
  Param(
    [Parameter(Mandatory=$true)][string]$Uri,
    [hashtable]$Headers
  )
  $req = [System.Net.HttpWebRequest]::Create($Uri)
  $req.Method = 'GET'
  $req.AllowAutoRedirect = $false
  if($session -and $session.Cookies){
    $req.CookieContainer = $session.Cookies
  } else {
    $req.CookieContainer = New-Object System.Net.CookieContainer
  }

  if($Headers){
    foreach($k in $Headers.Keys){
      $val = [string]$Headers[$k]
      if($k -ieq 'User-Agent'){ $req.UserAgent = $val }
      elseif($k -ieq 'Authorization'){ $req.Headers['Authorization'] = $val }
      else { $req.Headers[$k] = $val }
    }
  }

  try{
    $resp = $req.GetResponse()
  } catch [System.Net.WebException] {
    if($_.Response){ $resp = $_.Response } else { throw }
  }

  $sr = New-Object System.IO.StreamReader($resp.GetResponseStream())
  $content = $sr.ReadToEnd(); $sr.Close()

  return [PSCustomObject]@{
    StatusCode = [int]$resp.StatusCode
    Headers    = $resp.Headers
    Content    = $content
    Json       = $null
  }
}
# -------------------------------------------------------------------------

function Invoke-API {
  Param(
    [Parameter(Mandatory=$true)][ValidateSet("GET","POST","OPTIONS","HEAD")] [string]$Method,
    [Parameter(Mandatory=$true)][string]$Path,
    [hashtable]$Headers,
    $BodyJson,
    $BodyForm,
    [switch]$NoFollow,
    [switch]$ExpectJson
  )
  $uri = ($Base.TrimEnd('/')) + $Path

  # GET + NoFollow: usar HttpWebRequest sin auto-redirect
  if($NoFollow -and $Method -eq 'GET'){
    return Invoke-NoFollowGet -Uri $uri -Headers $Headers
  }

  $args = @{
    Uri = $uri
    Method = $Method
    WebSession = $session
    ErrorAction = 'Stop'
  }
  if($NoFollow){ $args.MaximumRedirection = 0 } else { $args.MaximumRedirection = 5 }
  if($Headers){ $args.Headers = $Headers }

  if($BodyJson){
    $args.ContentType = $JsonCT
    $args.Body = ($BodyJson | ConvertTo-Json -Depth 6 -Compress)
  } elseif ($BodyForm){
    $args.ContentType = "application/x-www-form-urlencoded"
    $pairs = @()
    foreach($k in $BodyForm.Keys){
      $pairs += ("{0}={1}" -f [System.Web.HttpUtility]::UrlEncode($k), [System.Web.HttpUtility]::UrlEncode([string]$BodyForm[$k]))
    }
    $args.Body = ($pairs -join "&")
  }

  try{
    $res = Invoke-WebRequest @args
    $jsonOut = $null
    if($ExpectJson){
      if(-not [string]::IsNullOrWhiteSpace($res.Content)){
        try { $jsonOut = $res.Content | ConvertFrom-Json } catch { $jsonOut = $null }
      }
    }
    return [PSCustomObject]@{
      StatusCode = [int]$res.StatusCode
      Headers    = $res.Headers
      Content    = $res.Content
      Json       = $jsonOut
    }
  } catch {
    if($_.Exception.Response){
      $resp = $_.Exception.Response
      $sr = New-Object System.IO.StreamReader($resp.GetResponseStream())
      $content = $sr.ReadToEnd(); $sr.Close()
      $jsonOut = $null
      if($ExpectJson){
        try { $jsonOut = $content | ConvertFrom-Json } catch { $jsonOut = $null }
      }
      return [PSCustomObject]@{
        StatusCode = [int]$resp.StatusCode
        Headers    = $resp.Headers
        Content    = $content
        Json       = $jsonOut
      }
    }
    throw
  }
}

function B64Url([byte[]]$bytes){ [Convert]::ToBase64String($bytes).TrimEnd('=').Replace('+','-').Replace('/','_') }
function SHA256B64Url([string]$s){ $sha=[System.Security.Cryptography.SHA256]::Create(); B64Url($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($s))) }
function AtHash([string]$access){ $sha=[System.Security.Cryptography.SHA256]::Create(); $raw=$sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($access)); B64Url($raw[0..15]) }
function Decode-JWT([string]$jwt){
  $parts=$jwt.Split('.'); if($parts.Length -lt 2){ return $null }
  function dec($b64){ $b64=$b64.Replace('-','+').Replace('_','/'); switch($b64.Length % 4){2{$b64+='=='};3{$b64+='='}}; [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($b64)) }
  $hdr = dec($parts[0]) | ConvertFrom-Json
  $pld = dec($parts[1]) | ConvertFrom-Json
  [PSCustomObject]@{ Header=$hdr; Payload=$pld }
}

# --- Builder de authorize path sin '&' literales ---
function Make-AuthorizePath([string]$Scope,[string]$State,[string]$Nonce,$CodeChallenge){
  $params = @{
    response_type = 'code'
    client_id = $Client
    redirect_uri = $Redirect
    scope = $Scope
    state = $State
    code_challenge = $CodeChallenge
    code_challenge_method = 'S256'
  }
  if($Nonce){ $params['nonce'] = $Nonce }
  $pairs = @()
  foreach($k in $params.Keys){
    $pairs += ("{0}={1}" -f [System.Web.HttpUtility]::UrlEncode($k), [System.Web.HttpUtility]::UrlEncode([string]$params[$k]))
  }
  return "/oauth2/authorize?" + ($pairs -join "&")
}
# ---------------------------------------------------

$rand = Get-Random -Minimum 10000 -Maximum 99999
$localPart, $domain = $EmailBase.Split('@')[0], $EmailBase.Split('@')[1]
$Email = "$localPart+e2e$rand@$domain"
$NewPassword = "Nuev0Pass$rand!"

Write-Host "E2E starting -> BASE=$Base  TENANT=$Tenant  CLIENT=$Client  EMAIL=$Email" -ForegroundColor Magenta

# 0) Health
Step "GET /healthz"; $r = Invoke-API -Method GET -Path "/healthz"
Assert ($r.StatusCode -eq 200 -and $r.Content -eq "ok") "healthz not OK"
Step "GET /readyz"; $r = Invoke-API -Method GET -Path "/readyz"
Assert ($r.StatusCode -eq 200 -and $r.Content -eq "ready") "readyz not OK"
Ok "Health OK"

# 1) JWKS / Discovery
Step "GET /.well-known/jwks.json"
$r = Invoke-API -Method GET -Path "/.well-known/jwks.json" -ExpectJson
Assert ($r.StatusCode -eq 200 -and $r.Json -and $r.Json.keys.Count -ge 1) "jwks empty/invalid"
Step "GET /.well-known/openid-configuration"
$disc = Invoke-API -Method GET -Path "/.well-known/openid-configuration" -ExpectJson
Assert ($disc.StatusCode -eq 200 -and $disc.Json.issuer) "discovery invalid"
Ok "Discovery/JWKS OK"

# 2) CORS
Step "OPTIONS /v1/auth/login (CORS permitido)"
$h = @{ Origin="http://localhost:3000"; "Access-Control-Request-Method"="POST"; "Access-Control-Request-Headers"="content-type" }
$r = Invoke-API -Method OPTIONS -Path "/v1/auth/login" -Headers $h
Assert ($r.StatusCode -eq 204 -and $r.Headers["Access-Control-Allow-Origin"] -eq "http://localhost:3000") "CORS allowed preflight failed"
Step "OPTIONS /v1/auth/login (CORS NO permitido)"
$h.Origin = "http://evil.local"; $r = Invoke-API -Method OPTIONS -Path "/v1/auth/login" -Headers $h
Assert (-not $r.Headers["Access-Control-Allow-Origin"]) "CORS disallowed origin should not echo ACAO"
Ok "CORS OK"

# 3) Registro / Login
Step "POST /v1/auth/register"
$reg = Invoke-API -Method POST -Path "/v1/auth/register" -ExpectJson -BodyJson @{ tenant_id=$Tenant; client_id=$Client; email=$Email; password=$Password }
Assert ($reg.StatusCode -in 200,201) "register failed"

$Access = $null; $Refresh = $null
if($reg.Json -and $reg.Json.access_token){
  $Access = $reg.Json.access_token; $Refresh = $reg.Json.refresh_token
  Ok "Register returned tokens"
}else{
  Step "POST /v1/auth/login"
  $login = Invoke-API -Method POST -Path "/v1/auth/login" -ExpectJson -BodyJson @{ tenant_id=$Tenant; client_id=$Client; email=$Email; password=$Password }
  Assert ($login.StatusCode -eq 200 -and $login.Json.access_token) "login failed"
  $Access = $login.Json.access_token; $Refresh = $login.Json.refresh_token
}

# 4) /v1/me
Step "GET /v1/me (Bearer)"; $me = Invoke-API -Method GET -Path "/v1/me" -Headers @{ Authorization = "Bearer $Access" } -ExpectJson
Assert ($me.StatusCode -eq 200 -and $me.Json.sub -and $me.Json.tid) "/v1/me failed"
Ok "Password auth OK"

# 5) Refresh + logout
Step "POST /v1/auth/refresh"
$oldRT = $Refresh
$refr = Invoke-API -Method POST -Path "/v1/auth/refresh" -ExpectJson -BodyJson @{ client_id=$Client; refresh_token=$Refresh }
Assert ($refr.StatusCode -eq 200 -and $refr.Json.refresh_token) "refresh rotation failed"
$Access = $refr.Json.access_token; $Refresh = $refr.Json.refresh_token

Step "Reusar refresh viejo (esperado 401)"
$bad = Invoke-API -Method POST -Path "/v1/auth/refresh" -ExpectJson -BodyJson @{ client_id=$Client; refresh_token=$oldRT }
Assert ($bad.StatusCode -eq 401) "old refresh should be invalid"

Step "POST /v1/auth/logout"
$lg = Invoke-API -Method POST -Path "/v1/auth/logout" -ExpectJson -BodyJson @{ refresh_token=$Refresh }
Assert ($lg.StatusCode -eq 204) "logout failed"

Step "Refresh luego del logout (esperado 401)"
$bad2 = Invoke-API -Method POST -Path "/v1/auth/refresh" -ExpectJson -BodyJson @{ client_id=$Client; refresh_token=$Refresh }
Assert ($bad2.StatusCode -eq 401) "revoked refresh should be invalid"
Ok "Refresh/logout OK"

# 6) Verify email
Step "POST /v1/auth/verify-email/start"
$vs = Invoke-API -Method POST -Path "/v1/auth/verify-email/start" -Headers @{ Authorization = "Bearer $Access" } -BodyJson @{ tenant_id=$Tenant; client_id=$Client; redirect_uri=$Redirect } -NoFollow
Assert ($vs.StatusCode -eq 204) "verify-email start failed"

$verifyLink = $vs.Headers["X-Debug-Verify-Link"]
if($AutoEmail){ Assert ($verifyLink) "AutoEmail true pero no vino X-Debug-Verify-Link" }
else{
  if(-not $verifyLink){
    Warn "Pega la URL del mail de VERIFICACION (o solo el token=...):"
    $paste = Read-Host "Link/token"
    if($paste -match "token="){ $verifyLink = $paste } else { $verifyLink = "$Base/v1/auth/verify-email?token=$paste" }
  }
}

Step "GET verify link"
$vc = Invoke-API -Method GET -Path ($verifyLink.Replace($Base,'')) -NoFollow
if($vc.StatusCode -eq 302){ $loc = $vc.Headers['Location']; Assert ($loc -and $loc -match "status=verified") "verify redirect sin status=verified" }
else{
  $json = $null; try{ $json = $vc.Content | ConvertFrom-Json }catch{}
  Assert ($vc.StatusCode -eq 200 -and $json -and $json.status -eq "verified") "verify JSON no dice verified"
}
Ok "Email verify OK"

# 7) Forgot / Reset
Step "POST /v1/auth/forgot"
$fg = Invoke-API -Method POST -Path "/v1/auth/forgot" -BodyJson @{ tenant_id=$Tenant; client_id=$Client; email=$Email; redirect_uri=$Redirect } -NoFollow
Assert ($fg.StatusCode -eq 200) "forgot failed"
$resetLink = $fg.Headers["X-Debug-Reset-Link"]
if($AutoEmail){ Assert ($resetLink) "AutoEmail true pero no vino X-Debug-Reset-Link" }
else{
  if(-not $resetLink){
    Warn "Pega la URL del mail de RESET (o solo el token=...):"
    $paste = Read-Host "Link/token"
    if($paste -match "token="){ $resetLink = $paste } else { $resetLink = "$Base/v1/auth/reset?token=$paste" }
  }
}
function Extract-Query([string]$url,[string]$k){ $u=[Uri]$url; $q=[System.Web.HttpUtility]::ParseQueryString($u.Query); $q[$k] }
$resetToken = Extract-Query $resetLink "token"
Assert ($resetToken) "no pude extraer token de reset"

Step "POST /v1/auth/reset (autologin on/off)"
$rr = Invoke-API -Method POST -Path "/v1/auth/reset" -ExpectJson -BodyJson @{ tenant_id=$Tenant; client_id=$Client; token=$resetToken; new_password=$NewPassword }
if($rr.StatusCode -eq 204){
  Step "POST /v1/auth/login con nueva pass"
  $login2 = Invoke-API -Method POST -Path "/v1/auth/login" -ExpectJson -BodyJson @{ tenant_id=$Tenant; client_id=$Client; email=$Email; password=$NewPassword }
  Assert ($login2.StatusCode -eq 200) "login con nueva pass fallo"
  $Access = $login2.Json.access_token; $Refresh = $login2.Json.refresh_token
} elseif ($rr.StatusCode -eq 200) {
  $Access = $rr.Json.access_token; $Refresh = $rr.Json.refresh_token
} else { Fail "reset password fallo: HTTP $($rr.StatusCode)" }

$me2 = Invoke-API -Method GET -Path "/v1/me" -Headers @{ Authorization = "Bearer $Access" } -ExpectJson
Assert ($me2.StatusCode -eq 200) "me tras reset fallo"
Ok "Reset password OK"

# 8) OIDC Code + PKCE + UserInfo + Refresh/Revoke
Step "POST /v1/session/login (set-cookie)"
$sl = Invoke-API -Method POST -Path "/v1/session/login" -BodyJson @{ tenant_id=$Tenant; client_id=$Client; email=$Email; password=$NewPassword }
Assert ($sl.StatusCode -eq 204) "session login fallo"

$code_verifier = -join ((48..122) | Get-Random -Count 64 | % {[char]$_})
$code_challenge = SHA256B64Url $code_verifier

# Llamadas a /oauth2/authorize usando Make-AuthorizePath

Step "GET /oauth2/authorize (cookie) -> 302 code"
$authPath = Make-AuthorizePath "openid email profile" "st$rand" "nn$rand" $code_challenge
$auth = Invoke-API -Method GET -Path $authPath -NoFollow
$code = Extract-Query $auth.Headers['Location'] "code"
Assert ($auth.StatusCode -eq 302 -and $code) "authorize deberia 302 con code"

Step "POST /oauth2/token (authorization_code + PKCE)"
$tok = Invoke-API -Method POST -Path "/oauth2/token" -ExpectJson -BodyForm @{ grant_type="authorization_code"; code=$code; redirect_uri=$Redirect; client_id=$Client; code_verifier=$code_verifier }
Assert ($tok.StatusCode -eq 200 -and $tok.Json.access_token -and $tok.Json.id_token -and $tok.Json.refresh_token) "token exchange failed"

$at = $tok.Json.access_token; $idt = $tok.Json.id_token; $idc = Decode-JWT $idt
Assert ($idc) "id_token no decodificable"
Assert ($idc.Payload.azp -eq $Client) "id_token.azp mismatch"
Assert ($idc.Payload.tid -eq $Tenant) "id_token.tid mismatch"
Assert ( (AtHash $at) -eq $idc.Payload.at_hash ) "id_token.at_hash mismatch"

Step "GET /userinfo"
$ui = Invoke-API -Method GET -Path "/userinfo" -Headers @{ Authorization = "Bearer $at" } -ExpectJson
Assert ($ui.StatusCode -eq 200) "/userinfo fallo"

Step "POST /oauth2/token (refresh_token)"
$old = $tok.Json.refresh_token
$rtok = Invoke-API -Method POST -Path "/oauth2/token" -ExpectJson -BodyForm @{ grant_type="refresh_token"; refresh_token=$old; client_id=$Client }
Assert ($rtok.StatusCode -eq 200 -and $rtok.Json.refresh_token) "refresh grant fallo"

Step "Reusar refresh viejo (esperado invalid_grant)"
$re2 = Invoke-API -Method POST -Path "/oauth2/token" -ExpectJson -BodyForm @{ grant_type="refresh_token"; refresh_token=$old; client_id=$Client }
Assert ($re2.StatusCode -eq 400) "old refresh deberia dar invalid_grant"

Step "POST /oauth2/revoke (refresh actual) => 200"
$rvk = Invoke-API -Method POST -Path "/oauth2/revoke" -BodyForm @{ token = $rtok.Json.refresh_token }
Assert ($rvk.StatusCode -eq 200) "revoke fallo"

Step "Refresh luego de revoke (esperado invalid_grant)"
$re3 = Invoke-API -Method POST -Path "/oauth2/token" -ExpectJson -BodyForm @{ grant_type="refresh_token"; refresh_token=$rtok.Json.refresh_token; client_id=$Client }
Assert ($re3.StatusCode -eq 400) "revoked refresh deberia dar invalid_grant"
Ok "OIDC core OK"

# 8.b) Negativos
$authPath2 = Make-AuthorizePath "openid email profile" "st2$rand" "nn2$rand" $code_challenge
$auth2 = Invoke-API -Method GET -Path $authPath2 -NoFollow
$code2 = Extract-Query $auth2.Headers['Location'] "code"

Step "PKCE invalido (code_verifier wrong)"
$badpkce = Invoke-API -Method POST -Path "/oauth2/token" -ExpectJson -BodyForm @{ grant_type="authorization_code"; code=$code2; redirect_uri=$Redirect; client_id=$Client; code_verifier="wrongverifier" }
Assert ($badpkce.StatusCode -eq 400) "PKCE invalido deberia fallar"

Step "Reuso de code (invalid_grant)"
$reuse = Invoke-API -Method POST -Path "/oauth2/token" -ExpectJson -BodyForm @{ grant_type="authorization_code"; code=$code; redirect_uri=$Redirect; client_id=$Client; code_verifier=$code_verifier }
Assert ($reuse.StatusCode -eq 400) "reuso de code deberia fallar"

Step "redirect_uri distinto (invalid_grant)"
$badru = Invoke-API -Method POST -Path "/oauth2/token" -ExpectJson -BodyForm @{ grant_type="authorization_code"; code=$code2; redirect_uri="http://localhost:9999/wrong"; client_id=$Client; code_verifier=$code_verifier }
Assert ($badru.StatusCode -eq 400) "redirect_uri mismatch deberia fallar"

Step "scope no permitido -> invalid_scope en redirect (state preservado)"
$state = "neg$rand"
$negPath = Make-AuthorizePath "openid phone" $state $null $code_challenge
$neg = Invoke-API -Method GET -Path $negPath -NoFollow
Assert ($neg.StatusCode -eq 302 -and ($neg.Headers['Location'] -match "error=invalid_scope") -and ($neg.Headers['Location'] -match "state=$state")) "invalid_scope/state esperado"

Step "sin openid => 400"
$noOpenIdPath = Make-AuthorizePath "email profile" "st3$rand" $null $code_challenge
$noOpenId = Invoke-API -Method GET -Path $noOpenIdPath
Assert ($noOpenId.StatusCode -eq 400) "sin openid deberia 400"

# 8.c) login_required y (opcional) Bearer dev
Step "POST /v1/session/logout"
$slout = Invoke-API -Method POST -Path "/v1/session/logout"
Assert ($slout.StatusCode -eq 204) "session logout fallo"

Step "authorize sin sesion => login_required"
$lrPath = Make-AuthorizePath "openid email" "st4$rand" $null $code_challenge
$lr = Invoke-API -Method GET -Path $lrPath -NoFollow
Assert ($lr.StatusCode -eq 302 -and ($lr.Headers['Location'] -match "error=login_required")) "login_required esperado"

if($DevAllowBearer){
  Step "authorize con Authorization: Bearer (modo dev)"
  $adbPath = Make-AuthorizePath "openid email" "st5$rand" $null $code_challenge
  $adb = Invoke-API -Method GET -Path $adbPath -Headers @{ Authorization = "Bearer $Access" } -NoFollow
  Assert ($adb.StatusCode -eq 302 -and ( ( [Uri]$adb.Headers['Location'] | Out-Null ) -or ( ( $adb.Headers['Location'] -match "code=" ) ) )) "Bearer dev authorize deberia devolver code"
}

Ok "E2E OK - Todo funcionando"
exit 0
