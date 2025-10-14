# Opens local Windows Firewall rules for HelloJohn E2E cluster tests.
# Ports:
# - HTTP: 18081-18083
# - Raft: 18201-18203
# Idempotent: skips creating rules if they already exist.

param(
    [switch]$Force
)

function Ensure-ModuleAvailable {
    param([string]$ModuleName)
    try {
        Import-Module -Name $ModuleName -ErrorAction Stop | Out-Null
        return $true
    } catch {
        return $false
    }
}

function Ensure-Rule-NetSecurity {
    param(
        [string]$DisplayName,
        [string]$Ports
    )
    $rule = Get-NetFirewallRule -DisplayName $DisplayName -ErrorAction SilentlyContinue
    if ($null -ne $rule) {
        Write-Host "Firewall rule exists: $DisplayName" -ForegroundColor Yellow
        return
    }
    New-NetFirewallRule -DisplayName $DisplayName -Direction Inbound -Action Allow -Protocol TCP -LocalPort $Ports | Out-Null
    New-NetFirewallRule -DisplayName "$DisplayName (Outbound)" -Direction Outbound -Action Allow -Protocol TCP -LocalPort $Ports | Out-Null
    Write-Host "Firewall rules added: $DisplayName (Inbound/Outbound) for TCP ports $Ports" -ForegroundColor Green
}

function Ensure-Rule-Netsh {
    param(
        [string]$Name,
        [string]$Ports
    )
    $exists = $false
    $out = & netsh advfirewall firewall show rule name="$Name" 2>&1
    if ($LASTEXITCODE -eq 0 -and $out -match "Rule Name:") { $exists = $true }
    if (-not $exists -or $Force) {
        & netsh advfirewall firewall add rule name="$Name" dir=in action=allow protocol=TCP localport=$Ports | Out-Null
    }
    $out = & netsh advfirewall firewall show rule name="$Name (Outbound)" 2>&1
    $exists = ($LASTEXITCODE -eq 0 -and $out -match "Rule Name:")
    if (-not $exists -or $Force) {
        & netsh advfirewall firewall add rule name="$Name (Outbound)" dir=out action=allow protocol=TCP localport=$Ports | Out-Null
    }
    Write-Host "Firewall rules ensured via netsh: $Name for TCP ports $Ports" -ForegroundColor Green
}

$portsHttp = "18081-18083"
$portsRaft = "18201-18203"

if (Ensure-ModuleAvailable -ModuleName NetSecurity) {
    Ensure-Rule-NetSecurity -DisplayName "HelloJohn E2E HTTP Ports" -Ports $portsHttp
    Ensure-Rule-NetSecurity -DisplayName "HelloJohn E2E Raft Ports" -Ports $portsRaft
} else {
    Write-Host "NetSecurity module not available; falling back to netsh" -ForegroundColor Yellow
    Ensure-Rule-Netsh -Name "HelloJohn E2E HTTP Ports" -Ports $portsHttp
    Ensure-Rule-Netsh -Name "HelloJohn E2E Raft Ports" -Ports $portsRaft
}

Write-Host "Done. You can now run the 3-node HA E2E tests locally." -ForegroundColor Cyan
