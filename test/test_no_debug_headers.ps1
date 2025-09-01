param(
  [string]$Base   = "http://localhost:8080",
  [string]$Tenant = "95f317cd-28f0-4bae-a0aa-4a4ddff92a16",
  [string]$Client = "web-frontend",
  [string]$Admin  = "admin@rem.com",
  [string]$Pwd    = "supersecreta",
  [string]$Cb     = "http://localhost:3000/callback"
)

function Post-Json {
  param(
    [string]$Url,
    [hashtable]$Body,
    [hashtable]$Headers = @{}
  )
  $json = $Body | ConvertTo-Json -Compress
  Invoke-WebRequest -Uri $Url `
    -Method POST `
    -ContentType 'application/json' `
    -Headers $Headers `
    -Body $json `
    -UseBasicParsing `
    -ErrorAction Stop
}

Write-Host "== login =="
try {
  $loginResp = Post-Json "$Base/v1/auth/login" @{ tenant_id=$Tenant; client_id=$Client; email=$Admin; password=$Pwd }
  $login     = $loginResp.Content | ConvertFrom-Json
  $access    = $login.access_token
  if (-not $access) { throw "login sin access_token" }
  Write-Host ("login -> status={0}" -f $loginResp.StatusCode)

  Write-Host "== verify-email/start =="
  $vsResp = Post-Json "$Base/v1/auth/verify-email/start" @{ tenant_id=$Tenant; client_id=$Client; redirect_uri=$Cb } @{ Authorization=("Bearer {0}" -f $access) }
  $h1 = $vsResp.Headers['X-Debug-Verify-Link']; if (-not $h1) { $h1 = '<none>' }
  Write-Host ("verify start -> status={0} X-Debug-Verify-Link={1}" -f $vsResp.StatusCode, $h1)

  Write-Host "== forgot =="
  $fgResp = Post-Json "$Base/v1/auth/forgot" @{ tenant_id=$Tenant; client_id=$Client; email=$Admin; redirect_uri=$Cb }
  $h2 = $fgResp.Headers['X-Debug-Reset-Link']; if (-not $h2) { $h2 = '<none>' }
  Write-Host ("forgot -> status={0} X-Debug-Reset-Link={1}" -f $fgResp.StatusCode, $h2)

  exit 0
}
catch {
  Write-Host "[ERROR]" $_.Exception.Message
  if ($_.Exception.Response) {
    $resp = $_.Exception.Response
    try {
      $sr = New-Object System.IO.StreamReader($resp.GetResponseStream())
      $body = $sr.ReadToEnd(); $sr.Close()
      Write-Host ("status={0} code={1} body={2}" -f $resp.StatusCode, [int]$resp.StatusCode, $body)
    } catch {}
  }
  exit 1
}
