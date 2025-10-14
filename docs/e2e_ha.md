# Pruebas E2E – Cluster HA (Fase 6)

> IMPORTANTE: Para correr los tests HA siempre exportar `E2E_SKIP_GLOBAL_SERVER=1` y `DISABLE_DOTENV=1`.
> - PowerShell:
>   ```powershell
>   $env:E2E_SKIP_GLOBAL_SERVER='1'; $env:DISABLE_DOTENV='1'; go test ./test/e2e -run "(40_|41_|42_)" -count=1
>   Remove-Item env:E2E_SKIP_GLOBAL_SERVER; Remove-Item env:DISABLE_DOTENV
>   ```
> - Bash:
>   ```bash
>   E2E_SKIP_GLOBAL_SERVER=1 DISABLE_DOTENV=1 go test ./test/e2e -run '(40_|41_|42_)' -count=1
>   ```

Este documento explica cómo ejecutar la sub‑suite de pruebas E2E relacionadas con Alta Disponibilidad (Tests 40+), y cómo aislarlas del servidor global de pruebas.

## 1. Objetivos de las pruebas
| Test | Propósito |
|------|-----------|
| 40_leader_gating | Verifica 409 en follower (sin redirect) y 307 cuando se solicita redirección al líder. |
| 41_snapshot_restore_jwks | Garantiza que tras rotar claves, borrar el estado Raft de un follower y reingresarlo, el JWKS coincide (consistencia de snapshot/restore). |
| (Opcional futuro) 42_wiring_canary | Smoke rápido sobre cada ruta gated para asegurar 409/307 y evitar regresiones en middleware RequireLeader. |

## 2. Variables relevantes
- `E2E_SKIP_GLOBAL_SERVER=1`: evita que `TestMain` levante el servidor monolítico base; las pruebas gestionan nodos explícitos.
- `CLUSTER_MODE=1`: habilita el modo Raft embebido.
- `CLUSTER_BOOTSTRAP=1`: sólo para el primer nodo (bootstrap). Luego 0 en nodos subsiguientes.
- `RAFT_ADDR`: dirección de transporte Raft (por nodo).
- `CLUSTER_NODES`: lista estática `nodeID=host:port` separada por comas.
- `LEADER_REDIRECTS`: mapping `nodeID=httpBaseURL` usado para construir `Location` en 307.
- `RAFT_SNAPSHOT_EVERY`: frecuencia de snapshot (menor = snapshots más frecuentes en pruebas).

### 2.1 Tabla de variables mínimas para HA local
| Variable | Uso |
|----------|-----|
| CLUSTER_MODE=embedded | Activa modo Raft embebido |
| NODE_ID | Identificador único del nodo (n1, n2, etc.) |
| RAFT_ADDR | Dirección Raft (host:puerto) |
| CLUSTER_NODES | Lista estática peers `id=host:port;...` |
| LEADER_REDIRECTS | Mapping `id=httpBaseURL;...` para 307 (opcional) |
| CLUSTER_BOOTSTRAP=1 | Sólo primer nodo (bootstrap). Luego unset |

## 3. Ejecutar sólo los tests HA
Desde la raíz del proyecto:
```
$env:E2E_SKIP_GLOBAL_SERVER="1"; go test ./test/e2e -run "(40_|41_)" -count=1 -timeout=15m
```
En PowerShell el patrón anterior filtra tests 40 y 41.

### 3.1 Ejecutar también el canario 42
```
$env:E2E_SKIP_GLOBAL_SERVER="1"; go test ./test/e2e -run "(40_|41_|42_)" -count=1 -timeout=6m
```

### 3.2 Script rápido
Para correr la trilogía HA (40,41,42) usar:
```
powershell -ExecutionPolicy Bypass -File scripts/dev-ha.ps1
```
Limpia variables al final.

## 4. Puertos fijos usados en pruebas
| Nodo | HTTP | Raft |
|------|------|------|
| node1 | 18081 | 18201 |
| node2 | 18082 | 18202 |
| node3 | 18083 | 18203 |

Si el puerto está ocupado: matar proceso previo (en Windows usar `Get-Process -Id (Get-NetTCPConnection -LocalPort 18081).OwningProcess`).

## 5. Tips de debug
- Elección lenta: aumentar timeout de `waitReady` o verificar latencia local (AV, firewall).
- Puerto ocupado: limpiar nodos previos; revisar que ningún contenedor/demonio use `:1808x`.
- Windows file lock: si un test falla al borrar `raft/`, esperar unos segundos o cerrar visor de archivos.
- 307 ausente: confirmar `LEADER_REDIRECTS` y envío de `?leader_redirect=1` o header `X-Leader-Redirect: 1`.
- 409 inesperado en líder: verificar que realmente el request va al líder (`/readyz`).

## 6. Estado futuro (CI)
La integración CI todavía no se publica; por ahora los scripts locales (`scripts/dev-ha.ps1`, `scripts/dev-smoke.ps1`) cubren el loop de desarrollo. CI se añadirá en fase posterior.

## 7. Cómo correr solo HA local (resumen)
```
$env:E2E_SKIP_GLOBAL_SERVER='1'
$env:DISABLE_DOTENV='1'
go test ./test/e2e -run Test_40_Leader_Gating_Redirect -count=1 -timeout=4m
go test ./test/e2e -run Test_41_SnapshotRestore_JWKSIdentical -count=1 -timeout=6m
go test ./test/e2e -run Test_42_RequireLeader_Wiring_Smoke -count=1 -timeout=2m
Remove-Item env:E2E_SKIP_GLOBAL_SERVER; Remove-Item env:DISABLE_DOTENV
```


## 4. Cómo funcionan los tests
### Test 40
1. Inicia 3 nodos con puertos determinísticos.
2. Espera elección y detecta líder.
3. Intenta una escritura (PUT tenant) contra un follower => 409 + `X-Leader`.
4. Repite con redirect habilitado (`?leader_redirect=1`) => 307 + `Location` al líder.

### Test 41
1. Inicia clúster y crea un tenant + rota claves.
2. Elimina el directorio `raft/` del nodo 3 (simula pérdida de estado).
3. Reinicia nodo 3 y espera rejoin.
4. Descarga JWKS normalizando JSON y compara (mismo conjunto de claves).

## 5. Debug rápido
- Revisar logs: cada test captura stdout/err de procesos hijos.
- Problemas de elección: aumentar timeout en helper `waitReady` o verificar colisión de puertos.
- 307 que no aparece: confirmar que `LEADER_REDIRECTS` contiene el `nodeID` del follower y que se envió el query/header de opt-in.

## 6. Diseño del middleware RequireLeader
Comportamiento simplificado:
1. Si el request es escritura control-plane y el nodo es follower =>
   - Sin opt-in redirect -> 409 JSON (`follower_conflict`).
   - Con opt-in y mapping -> 307 Temporary Redirect (Location + headers informativos).
2. El líder procesa normalmente.

Headers incluidos en ambos casos:
- `X-Leader`: nodeID del líder actual.
- `X-Leader-URL`: base URL líder (si definida en mapping).

## 7. Futuro (Test 42 propuesto)
Smoke minimalista que itera sobre cada ruta gated y confirma:
- 409 en follower
- 307 con redirect
- 200 en líder

Esto reduce tiempo de detectar un wiring roto cuando se agreguen rutas nuevas.

## 8. Integración CI sugerida
Pipeline segmentado:
1. `go test ./...` (unit + small) excluyendo `test/e2e`.
2. Subconjunto E2E básico (login, oauth, mfa) todavía en modo single-node.
3. Job `ha-e2e` con `E2E_SKIP_GLOBAL_SERVER=1` y filtro `-run "(40_|41_)"`.

Cache de módulos Go compartida entre jobs para acelerar.

---
© 2025 HelloJohn – E2E HA Docs.
