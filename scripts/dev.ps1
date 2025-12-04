# Get the root directory (one level up from scripts)
$root = Resolve-Path "$PSScriptRoot\.."
Write-Host "Starting HelloJohn Dev Environment..." -ForegroundColor Green

# Start Backend
Write-Host "Starting Backend (Go)..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd ..; go run ./cmd/service --env"

# Start Frontend
Write-Host "Starting Frontend (Next.js)..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$root/ui'; npm run dev"

Write-Host "Services started in separate windows." -ForegroundColor Green
