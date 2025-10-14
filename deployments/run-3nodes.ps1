Param(
  [string]$Root = "${PWD}",
  [string]$FsRoot1 = "${PWD}\data\node1",
  [string]$FsRoot2 = "${PWD}\data\node2",
  [string]$FsRoot3 = "${PWD}\data\node3"
)

$ErrorActionPreference = "Stop"

New-Item -ItemType Directory -Force -Path $FsRoot1 | Out-Null
New-Item -ItemType Directory -Force -Path $FsRoot2 | Out-Null
New-Item -ItemType Directory -Force -Path $FsRoot3 | Out-Null

# Common env
$env:SECRETBOX_MASTER_KEY = "e3wlUfaN91WoNvHa9aB47ARoAz1DusF2I+hV7Uyz/wU="
$env:SIGNING_MASTER_KEY   = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
$env:RATE_ENABLED         = "false"

# Static peers (nodeID=addr)
$peers = "n1=127.0.0.1:8201;n2=127.0.0.1:8202;n3=127.0.0.1:8203"

Write-Host "Starting node 1 (8081 / 8201)"
Start-Process -NoNewWindow powershell -ArgumentList @(
  "-NoLogo","-NoProfile","-Command",
  "`$env:SERVER_ADDR=':8081';`$env:CONTROL_PLANE_FS_ROOT='$FsRoot1';`$env:CLUSTER_MODE='embedded';`$env:NODE_ID='n1';`$env:RAFT_ADDR='127.0.0.1:8201';`$env:CLUSTER_NODES='$peers'; go run ./cmd/service"
)

Start-Sleep -Seconds 1

Write-Host "Starting node 2 (8082 / 8202)"
Start-Process -NoNewWindow powershell -ArgumentList @(
  "-NoLogo","-NoProfile","-Command",
  "`$env:SERVER_ADDR=':8082';`$env:CONTROL_PLANE_FS_ROOT='$FsRoot2';`$env:CLUSTER_MODE='embedded';`$env:NODE_ID='n2';`$env:RAFT_ADDR='127.0.0.1:8202';`$env:CLUSTER_NODES='$peers'; go run ./cmd/service"
)

Start-Sleep -Seconds 1

Write-Host "Starting node 3 (8083 / 8203)"
Start-Process -NoNewWindow powershell -ArgumentList @(
  "-NoLogo","-NoProfile","-Command",
  "`$env:SERVER_ADDR=':8083';`$env:CONTROL_PLANE_FS_ROOT='$FsRoot3';`$env:CLUSTER_MODE='embedded';`$env:NODE_ID='n3';`$env:RAFT_ADDR='127.0.0.1:8203';`$env:CLUSTER_NODES='$peers'; go run ./cmd/service"
)

Write-Host "Launched 3 nodes. Endpoints: 8081/8082/8083. Raft: 8201/8202/8203."