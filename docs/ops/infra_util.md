# Infraestructura, Operaciones y Utilidades

## Introducción

Para soportar la lógica de negocio, HelloJohn cuenta con una base sólida de utilidades transversales y configuración.

Categorías cubiertas: `@[internal/config]`, `@[internal/metrics]`, `@[internal/util]`, `@[internal/email]`, `@[internal/cache]`, `@[internal/app]`.

---

## Configuración y Entorno (`internal/config`)

La aplicación sigue los principios de **Twelve-Factor App**, configurándose estrictamente a través de variables de entorno.

*   **Configuración Global**: `PORT`, `LOG_LEVEL`.
*   **Cluster**: `RAFT_NODE_ID`, `RAFT_PEERS`, `RAFT_ADDR`.
*   **Infraestructura**: `DATABASE_URL` (para DB pública), `REDIS_URL`.
*   **Seguridad**: `SIGNING_MASTER_KEY` (para encriptar secretos en reposo).

El paquete `config` carga y valida estas variables al inicio, fallando rápido (Fail Fast) si falta algo crítico.

---

## Observabilidad (`internal/metrics`)

HelloJohn expone métricas en formato **Prometheus** en el endpoint `/metrics`.

### Métricas Clave
*   `http_requests_total`: Tráfico global y códigos de error.
*   `raft_state`: Estado del nodo (Líder/Seguidor).
*   `db_pool_stats`: Conexiones abiertas/en uso por tenant.
*   `login_attempts`: Éxitos y fallos de autenticación.

---

## Utilidades y Servicios Internos

### Cache (`internal/cache`)
Interfaz abstracta para almacenamiento temporal.
*   **Implementaciones**: Memoria (in-process) para desarrollo y Redis para producción distribuida.
*   **Uso**: Almacenamiento de sesiones de login, códigos OAuth de corta duración y rate limiting counters.

### Email (`internal/email`)
Servicio de envío de correos transaccionales (Verificación, Reset Password).
*   Soporta configuración SMTP por Tenant. Esto significa que el Tenant A puede enviar correos desde `soporte@a.com` usando SendGrid, mientras que el Tenant B usa `info@b.com` con AWS SES.

### App Container (`internal/app`)
Utilizamos inyección de dependencias manual (o via Container struct) para orquestar los componentes. El paquete `app` define el contenedor principal que inicializa y conecta todos los servicios (DB, Cache, Raft, HTTP), facilitando el testing al permitir inyectar mocks.

---

## Estructura de Directorios (Standard Go Layout)

*   `cmd/`: Puntos de entrada (main).
*   `internal/`: Código privado de la aplicación (no importable por terceros).
*   `pkg/` (o `sdks/`): Código público y librerías cliente.
*   `ui/`: Frontend (Next.js).
*   `migrations/`: Archivos SQL de migración.
