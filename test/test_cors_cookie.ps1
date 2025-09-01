$ErrorActionPreference = "Stop"
$Base   = "http://localhost:8080"
$Tenant = "95f317cd-28f0-4bae-a0aa-4a4ddff92a16"
$Client = "web-frontend"
$Admin  = "admin@rem.com"
$Pwd    = "supersecreta"
$Cb     = "http://localhost:3000/callback"

# ---- CORS permitido
$originOK = "http://localhost:3000"
$r1 = Invoke-WebRequest "$Base/v1/auth/login" -Method OPTIONS -Headers @{
  "Origin"=$originOK; "Access-Control-Request-Method"="POST"; "Access-Control-Request-Headers"="content-type"
}
$h1 = $r1.Headers['Access-Control-Allow-Origin']
if (-not $h1) { $h1 = '<none>' }
"OPTIONS permitido -> $originOK  status=$($r1.StatusCode)  A-C-A-Origin=$h1"

# ---- CORS NO permitido
$originBAD = "http://evil.local"
$r2 = Invoke-WebRequest "$Base/v1/auth/login" -Method OPTIONS -Headers @{
  "Origin"=$originBAD; "Access-Control-Request-Method"="POST"; "Access-Control-Request-Headers"="content-type"
}
$h2 = $r2.Headers['Access-Control-Allow-Origin']
if (-not $h2) { $h2 = '<none>' }
"OPTIONS NO permitido -> $originBAD  status=$($r2.StatusCode)  A-C-A-Origin=$h2"

# ---- Session login para inspeccionar Set-Cookie
$body = @{ tenant_id=$Tenant; client_id=$Client; email=$Admin; password=$Pwd } | ConvertTo-Json
$r3 = Invoke-WebRequest "$Base/v1/session/login" -Method POST -ContentType 'application/json' -Body $body
$setCookie = $r3.Headers['Set-Cookie']
"Set-Cookie: $setCookie"
"HttpOnly? " + ($setCookie -match "HttpOnly")
# SameSite puede venir en múltiples cookies, lo listamos todos:
$sc = @()
if ($setCookie) {
  foreach ($c in $setCookie) {
    $m = [regex]::Match($c, "SameSite=\w+")
    if ($m.Success) { $sc += $m.Value }
  }
}
if ($sc.Count -eq 0) { $sc = @('<none>') }
"SameSite? " + ($sc -join ",")
"Secure?   " + ($setCookie -match "Secure")
