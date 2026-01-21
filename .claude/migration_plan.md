# Migration V2 Completion Plan

## Objetivo
Completar la migración V1→V2 existente mediante auditoría, wiring y mejoras incrementales.

## Agente Autónomo: Migration Completion Agent

### Configuración
- **Tipo**: feature-dev:code-reviewer + code-architect
- **Modo**: Autónomo (background)
- **Iteraciones**: 48 handlers (1 por iteración)
- **Output**: MIGRATION_LOG.md actualizado por cada handler

### Workflow por Iteración

```bash
# ITERACIÓN N (por handler)
1. Leer MIGRATION_LOG.md → Identificar siguiente handler pendiente
2. Leer handler V1 (internal/http/v1/handlers/{handler}.go)
3. Buscar equivalente V2 en:
   - internal/http/v2/services/*/{handler}_service.go
   - internal/http/v2/controllers/*/{handler}_controller.go
4. Auditar wiring:
   - ¿Service en aggregator? (services/{domain}/services.go)
   - ¿Controller en aggregator? (controllers/{domain}/controllers.go)
   - ¿Ruta en router? (router/{domain}_routes.go)
   - ¿Wiring en app.go?
5. Generar reporte de inconsistencias
6. SI faltan archivos → Crear (service, controller, DTO, router)
7. SI falta wiring → Editar (aggregators, router, app.go)
8. Actualizar MIGRATION_LOG.md con evidencias
9. Commit: git add . && git commit -m "feat(v2): migrated {handler}"
```

### Estados de Salida por Handler

- ✅ **Migrado completo**: Service + Controller + Router + Wiring OK
- ⏳ **Parcial**: Service OK, pero falta controller o wiring
- ❌ **Bloqueado**: Dependencia externa faltante (ej: Email V2 incompleto)
- 🔧 **Mejorado**: Ya existía pero se refactorizó para seguir patrón

### Criterios de Éxito (por handler)

1. DTO existe en `dto/{domain}/{nombre}.go`
2. Service existe en `services/{domain}/{nombre}_service.go`
3. Service está en `services/{domain}/services.go` aggregator
4. Controller existe en `controllers/{domain}/{nombre}_controller.go`
5. Controller está en `controllers/{domain}/controllers.go` aggregator
6. Ruta registrada en `router/{domain}_routes.go`
7. Wiring en `app.go:New()` (controllers inyectados)
8. Wiring en `router.RegisterV2Routes()` (controllers pasados)
9. Errores mapeados a `httperrors`
10. Logging con `logger.From(ctx)`
11. Herramientas V2 usadas (DAL V2, JWT V2, etc)
12. MIGRATION_LOG.md actualizado con evidencias

---

## Comando de Invocación

```bash
# Opción A: Ejecutar agente una vez (1 handler)
claude task run migration-agent --handler=auth_login

# Opción B: Ejecutar agente en loop (todos los pendientes)
claude task run migration-agent --mode=auto --max-iterations=48

# Opción C: Ejecutar agente en background
claude task run migration-agent --background --output=migration_progress.log
```

---

## Configuración de Permisos

El agente necesita:
- ✅ Leer archivos (V1 handlers, V2 services, aggregators)
- ✅ Escribir archivos (DTOs, services, controllers, routers)
- ✅ Editar archivos (aggregators, app.go, router.go)
- ✅ Ejecutar git (commit por cada migración)
- ❌ NO necesita push (manual para review)

---

## Output Esperado

Después de ejecutar el agente completo:

```
MIGRATION_LOG.md:
- Total handlers: 48
- Migrados: 48
- En progreso: 0
- Bloqueados: 0
- Pendientes: 0
- Progreso: 100%

Git log:
feat(v2): migrated auth_login
feat(v2): migrated auth_register
feat(v2): migrated auth_refresh
...
(48 commits)
```

---

## Monitoreo de Progreso

```bash
# Ver progreso en tiempo real
tail -f migration_progress.log

# Contar handlers completados
grep "✅" MIGRATION_LOG.md | wc -l

# Ver handlers bloqueados
grep "❌" MIGRATION_LOG.md -A 5

# Ver último commit
git log -1 --oneline
```

---

## Rollback en Caso de Error

Si el agente falla en medio de una migración:

```bash
# Revertir último commit
git reset --soft HEAD~1

# O revertir cambios no committeados
git checkout -- .

# Marcar handler como "en progreso" en MIGRATION_LOG.md
# Reiniciar agente desde ese handler
```
