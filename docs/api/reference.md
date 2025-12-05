# Referencia de API Endpoints

Este documento detalla los endpoints disponibles en la API de HelloJohn, organizados por funcionalidad.

## 🔐 Autenticación y OIDC

Endpoints públicos utilizados para iniciar sesión, obtener tokens y gestionar sesiones.

| Método | Endpoint | Descripción |
| :--- | :--- | :--- |
| `GET` | `/oauth2/authorize` | **Inicio del flujo OAuth2**. Valida el cliente y redirige al usuario a la página de login. |
| `POST` | `/oauth2/token` | **Intercambio de Token**. Canjea el `authorization_code` por `access_token` e `id_token`. |
| `GET` | `/userinfo` | Retorna información del usuario autenticado (OIDC Standard). Requiere token Bearer. |
| `POST` | `/oauth2/revoke` | Revoca un `refresh_token` o `access_token` específico. |
| `GET` | `/.well-known/openid-configuration` | Documento de descubrimiento OIDC. Lista endpoints y capacidades del servidor. |
| `GET` | `/.well-known/jwks.json` | Claves públicas (JSON Web Key Set) para verificar la firma de los tokens JWT. |

---

## 👤 Gestión de Identidad (Frontend API)

Endpoints utilizados por la interfaz de usuario de Login/Registro (UI) para interactuar con el backend.

| Método | Endpoint | Descripción |
| :--- | :--- | :--- |
| `POST` | `/v1/auth/login` | Valida credenciales (email/password) e inicia sesión. |
| `POST` | `/v1/auth/register` | Registra un nuevo usuario en el tenant actual. |
| `POST` | `/v1/auth/logout` | Cierra la sesión activa del usuario. |
| `POST` | `/v1/auth/forgot` | Inicia el flujo de recuperación de contraseña (envía email). |
| `POST` | `/v1/auth/reset` | Resetea la contraseña utilizando el token enviado por email. |
| `GET` | `/v1/auth/verify-email` | Verifica la dirección de correo electrónico del usuario. |

---

## 🛠 Admin API: Tenants

Gestión de inquilinos y su configuración. Requiere autenticación de administrador.

| Método | Endpoint | Descripción |
| :--- | :--- | :--- |
| `GET` | `/v1/admin/tenants` | Lista todos los tenants registrados en el clúster. |
| `POST` | `/v1/admin/tenants` | Crea un nuevo tenant. Inicializa su configuración en Raft. |
| `GET` | `/v1/admin/tenants/{slug}` | Obtiene los detalles de un tenant específico. |
| `PUT` | `/v1/admin/tenants/{slug}/settings` | Actualiza la configuración, incluyendo **Campos de Usuario** y conexiones a BD. |

---

## 👥 Admin API: Usuarios

Gestión de usuarios dentro de un tenant.

| Método | Endpoint | Descripción |
| :--- | :--- | :--- |
| `GET` | `/v1/admin/tenants/{slug}/users` | Lista paginada de usuarios del tenant. Incluye campos dinámicos. |
| `POST` | `/v1/admin/tenants/{slug}/users` | Crea un usuario administrativo o manual. Soporta `custom_fields`. |
| `POST` | `/v1/admin/users/disable` | Deshabilita el acceso de un usuario. |
| `POST` | `/v1/admin/users/enable` | Rehabilita el acceso de un usuario suspendido. |

---

## 📱 Admin API: Clientes OAuth

Gestión de aplicaciones que pueden usar HelloJohn para autenticar usuarios.

| Método | Endpoint | Descripción |
| :--- | :--- | :--- |
| `GET` | `/v1/admin/clients` | Lista las aplicaciones registradas. |
| `POST` | `/v1/admin/clients` | Registra una nueva aplicación (Client ID / Secret). |
| `PUT` | `/v1/admin/clients/{id}` | Actualiza redirecciones permitidas y scopes de una app. |
| `DELETE`| `/v1/admin/clients/{id}` | Elimina una aplicación cliente. |

---

## 🛡 Seguridad y MFA

Endpoints para autenticación de doble factor (Self-service).

| Método | Endpoint | Descripción |
| :--- | :--- | :--- |
| `POST` | `/v1/mfa/totp/enroll` | Inicia el registro de un dispositivo TOTP (Genera QR). |
| `POST` | `/v1/mfa/totp/verify` | Verifica el código TOTP para completar el registro o login. |
| `POST` | `/v1/mfa/totp/disable` | Desactiva MFA para el usuario actual. |

---

## ⚙️ Sistema y Utilidades

| Método | Endpoint | Descripción |
| :--- | :--- | :--- |
| `GET` | `/healthz` | Health check simple (Liveness probe). Retorna 200 OK si el servidor responde. |
| `GET` | `/readyz` | Readiness probe. Verifica conectividad a DB y estado de Raft. |
| `GET` | `/metrics` | Métricas de Prometheus (tráfico, errores, latencia). |
