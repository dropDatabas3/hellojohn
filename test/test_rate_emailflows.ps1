$ErrorActionPreference = "Stop"
$Base   = "http://localhost:8080"
$Tenant = "95f317cd-28f0-4bae-a0aa-4a4ddff92a16"
$Client = "web-frontend"
$Email  = "admin@rem.com"
$Cb     = "http://localhost:3000/callback"

$body = @{ tenant_id=$Tenant; client_id=$Client; email=$Email; redirect_uri=$Cb } | ConvertTo-Json

"== forgot #1 =="
$r1 = Invoke-WebRequest "$Base/v1/auth/forgot" -Method POST -ContentType 'application/json' -Body $body
$ra1 = $r1.Headers['Retry-After']; if (-not $ra1) { $ra1 = '<none>' }
"status=$($r1.StatusCode) Retry-After=$ra1"

"== forgot #2 (misma ventana; debe 429) =="
try {
  $r2 = Invoke-WebRequest "$Base/v1/auth/forgot" -Method POST -ContentType 'application/json' -Body $body
  $ra2 = $r2.Headers['Retry-After']; if (-not $ra2) { $ra2 = '<none>' }
  "status=$($r2.StatusCode) Retry-After=$ra2"
} catch {
  $resp = $_.Exception.Response
  if ($resp) {
    $hdr = $resp.Headers['Retry-After']; if (-not $hdr) { $hdr = '<none>' }
    "status=$([int]$resp.StatusCode) Retry-After=$hdr"
  } else {
    "error: $_"
  }
}
