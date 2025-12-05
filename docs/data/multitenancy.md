# Gestión de Datos y Multi-tenancy

## Introducción

La arquitectura de datos de HelloJohn está diseñada para garantizar el aislamiento estricto de los datos de cada tenant ("Organización"), permitiendo al mismo tiempo flexibilidad en el esquema de datos (Campos Dinámicos) y escalabilidad.

Categorías cubiertas: `@[internal/infra/db]`, `@[internal/store/pg]`, `@[internal/store]`.

---

## Estrategia Multi-tenancy: "Schema-per-Tenant"

En lugar de utilizar una única tabla gigante con una columna `tenant_id` (Row-level isolation), HelloJohn utiliza el patrón de **esquemas dedicados** en PostgreSQL.

### Ventajas
1.  **Aislamiento**: Los datos de un tenant están físicamente separados en su propio namespace. Es imposible leer datos de otro tenant por error en una query `WHERE` mal formada.
2.  **Performance**: Índices más pequeños y posibilidad de optimizar tablas individualmente.
3.  **Gestión**: Facilita backup/restore granular y borrado seguro de datos (DROP SCHEMA).

### Implementación (`internal/infra/tenantsql`)

El `Manager` de bases de datos es el componente central que orquesta el acceso a los datos.

1.  **Resolución de Conexión (`TenantResolver`)**:
    Dada un `slug` (identificador del tenant), el manager consulta al Control Plane para obtener la cadena de conexión (DSN) encriptada. Esto permite que diferentes tenants vivan en diferentes servidores de base de datos físicos (Sharding).

2.  **Pooling Dinámico**:
    Se mantiene un pool de conexiones `pgxpool` activo para cada tenant que está siendo utilizado. Los pools inactivos se cierran automáticamente para ahorrar recursos.

3.  **Migraciones Automáticas**:
    Cada vez que se establece conexión con un tenant por primera vez, el sistema verifica y aplica las migraciones SQL pendientes (`RunMigrationsWithLock`) para asegurar que el esquema esté actualizado.

---

## Campos Dinámicos (Dynamic User Fields)

HelloJohn permite que cada tenant defina campos personalizados para sus usuarios (ej: `department`, `employee_id`, `is_vip`).

### Schema Manager (`schema_manager.go`)
A diferencia de soluciones que guardan un JSON blob (`metadata`), HelloJohn **altera la tabla real** (`app_user`) para agregar columnas tipadas.

*   **SyncUserFields**: Este método compara la definición lógica de campos (`TenantSettings.UserFields`) con el esquema físico actual de la base de datos.
*   **DDL Generado**:
    *   `ADD COLUMN`: Si hay un campo nuevo.
    *   `DROP COLUMN`: Si un campo fue eliminado de la configuración.
    *   `Create Index / Constraints`: Se crean índices y restricciones (UNIQUE, NOT NULL) automáticamente.

Esta estrategia permite realizar consultas SQL nativas y performantes sobre campos personalizados (ej: `SELECT * FROM tenant_a.app_user WHERE department = 'IT'`).

---

## Soporte Multi-Driver (Roadmap)

Aunque actualmente la implementación principal es **PostgreSQL** (`pgx`), la arquitectura está diseñada con interfaces (`store.Repository` en `internal/store`) que permitirán agregar soporte para:

*   **MySQL / MariaDB**: Para infraestructuras legacy.
*   **SQLite / LibSQL**: Para despliegues en el borde (Edge) o en dispositivos IoT, donde postgres sería muy pesado.
*   **CockroachDB**: Para escala global distribuida nativa.

El `TenantConnection` ya incluye un campo `Driver`, preparando el terreno para esta evolución.
