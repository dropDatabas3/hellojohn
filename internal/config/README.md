# Config - Configuration Loader

> Carga y validación centralizada de la configuración del sistema

## Propósito

Este módulo es responsable de cargar la configuración de la aplicación desde dos fuentes principales:
1.  **Archivo YAML**: Configuración base estructurada.
2.  **Variables de Entorno**: Sobreescritura de valores para despliegues (Docker/K8s) y secretos.

Define la estructura global `Config` que es utilizada por `internal/app` para inicializar todos los componentes.

## Estructura

```
internal/config/
└── config.go      # Structs de config, Defaults, Env Overrides y Validación (800+ líneas)
```

## Componentes Principales

| Sección | Descripción |
|---------|-------------|
| `App` | Entorno (`dev`, `prod`) |
| `Server` | Dirección HTTP y CORS |
| `Storage` | Drivers de BD (Postgres, MySQL, Mongo) y DSNs |
| `Cache` | Redis o Memory con TTLs |
| `JWT` | Issuer, TTLs de tokens |
| `Auth` | Config de Sesiones, Cookies, Reset Password, MFA |
| `Rate` | Rate Limiting global y por endpoint (Login, MFA, etc.) |
| `SMTP` | Configuración de servidor de correo |
| `Email` | Templates y Links |
| `Security` | Políticas de password y claves maestras (SecretBox) |
| `ControlPlane` | Path raíz para el FS-based control plane (V2) |
| `Cluster` | Configuración de Raft (Nodos, TLS, Snapshots) |
| `Providers` | Social Login (Google) |

## Carga y Precedencia

El orden de carga es:
1.  **Defaults**: Valores seguros por defecto definidos en código.
2.  **YAML**: Se carga el archivo especificado en `Load(path)`.
3.  **Env Vars**: Las variables de entorno sobreescriben cualquier valor anterior.

```go
cfg, err := config.Load("config.yaml")
```

## Variables de Entorno Clave

| Variable | Config Map | Descripción |
|----------|------------|-------------|
| `APP_ENV` | `App.Env` | `dev` o `prod` |
| `SERVER_ADDR` | `Server.Addr` | Puerto de escucha (ej: `:8080`) |
| `STORAGE_DSN` | `Storage.DSN` | Connection string de la DB |
| `REDIS_ADDR` | `Cache.Redis.Addr` | Dirección de Redis |
| `JWT_ISSUER` | `JWT.Issuer` | Identificador del issuer |
| `SECRETBOX_MASTER_KEY`| `Security.SecretBoxMasterKey` | Clave maestra para encriptar datos en FS |
| `CONTROL_PLANE_FS_ROOT`| `ControlPlane.FSRoot` | Directorio raíz de datos del tenant |

## Seguridad

- **Secretos**: Se recomienda pasar secretos (DB passwords, Client Secrets, Master Keys) **solo** por variables de entorno.
- **Producción**: En modo `prod`, se deshabilitan características de debug como `DebugEchoLinks` (que muestra links de email en logs).
- **Sanitización**: Se limpian paths para evitar Path Traversal en `PasswordBlacklistPath`.

## Validación

El método `Validate()` realiza comprobaciones críticas. Actualmente es ligero, pero se valida el parsing de todas las duraciones (`time.Duration`) durante la carga.

## Referencia V2

Este módulo soporta la arquitectura V2 basada en Filesystem mediante `ControlPlane.FSRoot` y la encriptación de secretos en reposo con `SecretBoxMasterKey`.

## Ver También

- [internal/app](../app/README.md) - Consumidor principal de la configuración.
