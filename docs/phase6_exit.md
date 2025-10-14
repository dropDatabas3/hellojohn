# Fase 6 – Exit Checklist (Development)

## Alcance Cubierto (DONE)
- [x] Bootstrap determinista (CLUSTER_BOOTSTRAP + menor NODE_ID)
- [x] Middleware RequireLeader aplicado a rutas FS (tenants, clients, scopes, key rotate)
- [x] Exclusión de rutas DB (consents, RBAC, users) del gating
- [x] Semántica followers 409 + headers (X-Leader / X-Leader-URL)
- [x] Redirección opt-in 307 (Location + headers) cuando `?leader_redirect=1` o `X-Leader-Redirect: 1`
- [x] Snapshot/Restore: restaurar follower sin raft state => JWKS idéntico (Test 41)
- [x] Rotación determinista de claves (contenido replicado exacto)
- [x] Tests E2E: 40 (gating redirect), 41 (snapshot jwks), 42 (canario wiring)
- [x] Dockerfile multi-stage (uso local, sin CI todavía)
- [x] Config sanitizada (placeholders; secretos vía ENV)
- [x] Scripts locales dev-ha / dev-smoke
- [x] Documentación README + docs/e2e_ha.md actualizada (cluster, ejecución local, sin CI todavía)

## Pendiente Próxima Fase (P1)
- [ ] mTLS para transporte Raft (RAFT_TLS_* variables, certificados rotables)
- [ ] Métricas avanzadas (apply_latency_ms histogram, raft_log_size_bytes gauge)
- [ ] Runbook extendido backup / rollback / expansión dinámica de nodos
- [ ] Endpoint o comando para forzar snapshot manual
- [ ] IssuerMode=domain (hostnames personalizados por tenant)
- [ ] Canary ampliado (Test 42 iterar dinámicamente sobre catálogo rutas gated)

## Notas Operativas
- `E2E_SKIP_GLOBAL_SERVER=1` evita hang residual en suites HA → usar en scripts.
- `DISABLE_DOTENV=1` garantiza que no se carguen variables de `.env` durante pruebas (aislamiento).
- Limitar conexiones Postgres en pruebas HA (`POSTGRES_MAX_OPEN_CONNS=1`) reduce saturación en entornos locales.

## Verificación Manual Rápida
```powershell
$env:E2E_SKIP_GLOBAL_SERVER='1'; $env:DISABLE_DOTENV='1'
go test ./test/e2e -run Test_40_Leader_Gating_Redirect -count=1 -timeout=4m
go test ./test/e2e -run Test_41_SnapshotRestore_JWKSIdentical -count=1 -timeout=6m
go test ./test/e2e -run Test_42_RequireLeader_Wiring_Smoke -count=1 -timeout=2m
Remove-Item env:E2E_SKIP_GLOBAL_SERVER; Remove-Item env:DISABLE_DOTENV
```

## Riesgos Conocidos
- Falta de mTLS: tráfico Raft en claro (usar red interna confinada).
- No hay snapshot manual: dependemos de umbral RAFT_SNAPSHOT_EVERY.
- Redirect depende de mapping estático LEADER_REDIRECTS (sin descubrimiento dinámico).

## Conclusión
Fase 6 lista a nivel desarrollo: consistencia de metadatos y comportamiento HA validado sin pipelines aún. Próxima fase enfocará seguridad del canal Raft, observabilidad y capacidades operativas avanzadas.
