# Load .env.v2 environment variables into current PowerShell session

if (Test-Path ".env.v2") {
    Write-Host "Loading environment variables from .env.v2..." -ForegroundColor Cyan

    Get-Content ".env.v2" | ForEach-Object {
        # Skip comments and empty lines
        if ($_ -match '^\s*#' -or $_ -match '^\s*$') {
            return
        }

        # Parse KEY=VALUE
        if ($_ -match '^([^=]+)=(.*)$') {
            $key = $matches[1].Trim()
            $value = $matches[2].Trim()

            # Set environment variable for current process
            [Environment]::SetEnvironmentVariable($key, $value, "Process")
            Write-Host "  ✓ $key" -ForegroundColor Green
        }
    }

    Write-Host ""
    Write-Host "✅ Environment variables loaded!" -ForegroundColor Green
    Write-Host "Run: go run cmd/service_v2/main.go" -ForegroundColor Yellow
} else {
    Write-Host "❌ .env.v2 not found. Run setup script first:" -ForegroundColor Red
    Write-Host "   .\scripts\setup_v2_minimal.ps1" -ForegroundColor Yellow
}
