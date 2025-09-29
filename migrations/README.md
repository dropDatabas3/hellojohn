# Migrations – Estado actual y referencia

Este documento describe las migraciones vigentes, cómo se ejecutan y qué crea cada una. Está alineado con el estado actual del repositorio.

## Índice
1. Archivos presentes
2. Cómo se ejecutan (runner)
3. PostgreSQL – Detalle por migración
4. MySQL / Mongo – Estado
5. Rollback y buenas prácticas

---
## 1. Archivos presentes

PostgreSQL (fuente de verdad):
- `postgres/0001_core_up.sql` y `postgres/0001_core_down.sql`
- `postgres/0002_auth_extras_up.sql` y `postgres/0002_auth_extras_down.sql`
- `postgres/0003_rbac_up.sql` y `postgres/0003_rbac_down.sql`

Iniciales (parciales) para otros motores:
- `mysql/0001_init.sql`
- `mongo/0001_init.js`

Los antiguos `0002_mfa_*`, `0003_scope_consent_*`, `0004_rbac_*`, `0005_user_disable_*` fueron eliminados tras la consolidación en las tres migraciones listadas arriba.

---
## 2. Cómo se ejecutan (runner)

El runner de Postgres (ver `internal/store/pg/store.go`) aplica todos los archivos que terminen en `_up.sql`, ordenados por nombre de archivo; para revertir, ejecuta los `_down.sql` en orden inverso.

Convención recomendada:
- Nombrar con prefijo numérico (0001, 0002, 0003) y sufijo `_up.sql`/`_down.sql`.
- Mantener idempotencia razonable (IF NOT EXISTS) en DDL donde aporte.

---
## 3. PostgreSQL – Detalle por migración

### 0001_core
Crea el núcleo multi‑tenant y artefactos base:
- `tenant`, `client`, `client_version`
- `app_user`, `identity`
- `refresh_token`
- `email_verification_token`, `password_reset_token`
- `signing_keys`

Incluye índices para email por tenant, búsquedas por provider/IDs sociales, filtros de refresh activos y selección de claves firmantes.

### 0002_auth_extras
Agrega autenticación avanzada y consentimiento:
- MFA TOTP y auxiliares: `user_mfa_totp` (con trigger `updated_at`), `mfa_recovery_code`, `trusted_device`
- Scopes/Consent: `scope`, `user_consent` + índices (`tenant_name`, activos por usuario/cliente, GIN por `granted_scopes`)
- Campos de usuario para administración: `app_user.disabled_at`, `app_user.disabled_reason`

### 0003_rbac
Mapa de roles/permisos por tenant y asignación a usuarios:
- `rbac_role`, `rbac_perm`, `rbac_role_perm`, `rbac_user_role`
- Índices de ayuda (`rbac_user_role(user_id)`, `rbac_role_perm(tenant_id, perm)`)

---
## 4. MySQL / Mongo – Estado

- MySQL: `mysql/0001_init.sql` (parcial)
- Mongo: `mongo/0001_init.js` (parcial)

La implementación completa y la paridad con PostgreSQL están pendientes para estos motores.

---
## 5. Rollback y buenas prácticas

- Rollback: usar los archivos `_down.sql` correspondientes; el runner los ejecuta en orden descendente por nombre.
- Mantener las migraciones pequeñas y con descripciones claras en comentarios.
- Evitar cambios destructivos sin ventana de compatibilidad (añadir columnas/índices antes que renombrar/eliminar; usar `IF NOT EXISTS`).

© 2025 HelloJohn – Documento de migraciones actualizado.
