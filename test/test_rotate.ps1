Param(
  [string]$Base = "http://localhost:8080",
  [Parameter(Mandatory=$true)][string]$Tenant,
  [Parameter(Mandatory=$true)][string]$Client,
  [Parameter(Mandatory=$true)][string]$Email,
  [Parameter(Mandatory=$true)][string]$Password,
  # .env en el ROOT del repo
  [string]$EnvFile = ".env"
)

$ErrorActionPreference = "Stop"
Add-Type -AssemblyName System.Web | Out-Null

function Assert([bool]$c,[string]$m){ if(-not $c){ throw $m } }
function B64UrlDec([string]$b){ $b=$b.Replace('-','+').Replace('_','/'); switch($b.Length%4){2{$b+='=='};3{$b+='='}}; [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($b)) }
function Decode-JWT([string]$jwt){ $p=$jwt.Split('.'); if($p.Length -lt 2){return $null}; [PSCustomObject]@{Header=(B64UrlDec $p[0] | ConvertFrom-Json); Payload=(B64UrlDec $p[1] | ConvertFrom-Json)} }

$session = New-Object Microsoft.PowerShell.Commands.WebRequestSession
$JsonCT = "application/json; charset=utf-8"

function POST($path,$body){
  $r = Invoke-WebRequest -Uri ($Base+$path) -Method POST -ContentType $JsonCT -Body ($body | ConvertTo-Json -Compress) -WebSession $session
  $j = if($r.Content){ try{$r.Content|ConvertFrom-Json}catch{$null} } else { $null }
  [PSCustomObject]@{ Code=[int]$r.StatusCode; Json=$j; Headers=$r.Headers }
}
function GET($path){
  $r = Invoke-WebRequest -Uri ($Base+$path) -Method GET -WebSession $session
  $j = if($r.Content){ try{$r.Content|ConvertFrom-Json}catch{$null} } else { $null }
  [PSCustomObject]@{ Code=[int]$r.StatusCode; Json=$j; Headers=$r.Headers; Raw=$r.Content }
}

# Ejecuta el CLI de rotación desde el root del repo
$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
function Run-KeysCLI {
  param([string[]]$Args)
  Push-Location $RepoRoot
  try {
    Write-Host "Ejecutando: go run ./cmd/keys/main.go $($Args -join ' ')" -ForegroundColor Gray
    & go run ./cmd/keys/main.go @Args 2>&1 | Write-Host
    $exit = $LASTEXITCODE
    Write-Host "Exit code: $exit" -ForegroundColor Gray
  } finally {
    Pop-Location
  }
  return $exit
}

Write-Host "== Login inicial" -ForegroundColor Cyan
$login = POST "/v1/auth/login" @{ tenant_id=$Tenant; client_id=$Client; email=$Email; password=$Password }
Assert (($login.Code -eq 200) -and ($login.Json.access_token -ne $null)) "login falló"
$at_old = $login.Json.access_token
$idt_old = if ($login.Json.id_token) { $login.Json.id_token } else { $login.Json.access_token }
$kid1 = (Decode-JWT $idt_old).Header.kid
Write-Host "kid1=$kid1" -ForegroundColor Yellow

Write-Host "== JWKS antes" -ForegroundColor Cyan
$jw1 = GET "/.well-known/jwks.json"
Assert ($jw1.Code -eq 200) "jwks 1 falló"
$jw1kids = @($jw1.Json.keys | ForEach-Object { $_.kid })
$kcount1 = $jw1kids.Count
Write-Host "jwks keys=$kcount1  kids=[$($jw1kids -join ',')]" -ForegroundColor Yellow

Write-Host "== Rotar clave" -ForegroundColor Cyan
Push-Location (Resolve-Path (Join-Path $PSScriptRoot ".."))
try {
    Write-Host "Ejecutando: go run ./cmd/keys/main.go -rotate -env -env-file $EnvFile" -ForegroundColor Gray
    & go run ./cmd/keys/main.go -rotate -env -env-file $EnvFile
    $exit = $LASTEXITCODE
    Write-Host "Exit code: $exit" -ForegroundColor Gray
} finally {
    Pop-Location
}
Assert ($exit -eq 0) "rotate CLI falló"

Write-Host "== Esperando para que expire el cache del keystore (35s)" -ForegroundColor Cyan
Start-Sleep -Seconds 35

Write-Host "== /userinfo con token viejo (debe seguir ok)" -ForegroundColor Cyan
$ui = Invoke-WebRequest -Uri ($Base+"/userinfo") -Method GET -Headers @{Authorization="Bearer $at_old"} -WebSession $session -ErrorAction SilentlyContinue
Assert ([int]$ui.StatusCode -eq 200) "/userinfo con token viejo no está aceptando"

Write-Host "== Login nuevo" -ForegroundColor Cyan
$login2 = POST "/v1/auth/login" @{ tenant_id=$Tenant; client_id=$Client; email=$Email; password=$Password }
Write-Host "Login2 Code: $($login2.Code)" -ForegroundColor Gray
if ($login2.Json) {
    Write-Host "Login2 Json: $($login2.Json | ConvertTo-Json -Compress)" -ForegroundColor Gray
} else {
    Write-Host "Login2 Json: null" -ForegroundColor Gray
}
Assert (($login2.Code -eq 200) -and (($login2.Json.access_token -ne $null) -or ($login2.Json.id_token -ne $null))) "login2 falló"
$token2 = if ($login2.Json.id_token) { $login2.Json.id_token } else { $login2.Json.access_token }
$kid2 = (Decode-JWT $token2).Header.kid
Write-Host "kid2=$kid2" -ForegroundColor Yellow

Write-Host "== JWKS después" -ForegroundColor Cyan
Start-Sleep -Seconds 1
$jw2 = GET "/.well-known/jwks.json"
Assert ($jw2.Code -eq 200) "jwks 2 falló"
$jw2kids = @($jw2.Json.keys | ForEach-Object { $_.kid })
$kcount2 = $jw2kids.Count
Write-Host "jwks keys=$kcount2  kids=[$($jw2kids -join ',')]" -ForegroundColor Yellow
Assert ($kcount2 -ge 2) "jwks no publica 2+ KIDs tras rotación (active + retiring)"

# Si hay KID en tokens, validar cambio y presencia en JWKS
if ($kid1) {
  Assert ($jw2kids -contains $kid1) "old KID no aparece en JWKS post-rotación"
}
if ($kid2) {
  Assert ($jw2kids -contains $kid2) "new KID no aparece en JWKS post-rotación"
  if ($kid1) {
    Assert ($kid1 -ne $kid2) "KID no cambió tras rotación"
  }
} else {
  Write-Host "[warn] id_token sin 'kid' en header; integrar keystore + setear header 'kid' en issuer para validar cambio." -ForegroundColor Yellow
}

Write-Host "[OK] Rotación validada (token viejo OK, JWKS>=2; ver KID cuando esté cableado)" -ForegroundColor Green
