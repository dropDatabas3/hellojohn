# API, Seguridad y Endpoints

## Introducción

La capa HTTP de HelloJohn es la puerta de entrada para todas las interacciones, tanto de usuarios finales (Login) como de administradores y aplicaciones cliente. Está construida sobre `net/http` estándar mas el router ligero `chi`.

Categorías cubiertas: `@[internal/http]`, `@[internal/oauth]`, `@[internal/jwt]`, `@[internal/rate]`, `@[internal/audit]`.

---

## Estructura de Endpoints (`internal/http/routes.go`)

La API se divide en grandes áreas funcionales:

### 1. Auth & OIDC (`/oauth2`, `/v1/auth`)
Endpoints públicos para flujos de autenticación.
*   `/oauth2/authorize`: Punto de inicio del flujo Authorization Code.
*   `/oauth2/token`: Intercambio de código por tokens (Access/ID/Refresh).
*   `/v1/auth/login`, `/register`: APIs para la UI de login/registro (consumidas por el Frontend).
*   `/.well-known/openid-configuration`: Discovery document para clientes OIDC.

### 2. Admin API (`/v1/admin`)
Endpoints protegidos para gestión del sistema. Requieren autenticación y roles de administrador.
*   `/tenants`: Gestión de organizaciones.
*   `/users`: Gestión de usuarios (Disable, Delete, Campos Custom).
*   `/clients`: Gestión de aplicaciones OAuth.

### 3. User Self-Service (`/v1/me`, `/v1/mfa`)
Endpoints para que el usuario gestione su propia cuenta.
*   `/v1/me`: Datos del perfil actual.
*   `/v1/mfa/*`: Enroll y verificación de 2FA.

---

## Seguridad y Autenticación

### JWT y Manejo de Tokens
Implementación: `internal/jwt` y `internal/claims`

*   **Access Tokens**: JWTs firmados con claves RSA (RS256). Contienen `scopes` y `claims` estándar. Tienen vida corta (ej: 1 hora).
*   **Refresh Tokens**: Tokens opacos almacenados en base de datos (con hash). Permiten obtener nuevos access tokens sin re-login.
*   **Key Rotation**: El sistema rota automáticamente las claves de firma y expone las claves públicas en el endpoint JWKS (`/.well-known/jwks.json`).

### Middlewares (`internal/http/middleware.go`)

El pipeline de peticiones incluye validaciones críticas:
1.  **Rate Limiting (`internal/rate`)**: Protege contra fuerza bruta y DoS. Se aplica por IP y por Tenant.
2.  **Audit Logger (`internal/audit`)**: Registra eventos de seguridad (Login Success, Login Failed, Sensitive Config Change).
3.  **Panic Recovery**: Evita que el servidor caiga por errores no controlados.
4.  **Tenant Context**: Identifica el tenant destino basado en el dominio, path o cabecera, e inyecta el contexto adecuado.

---

## Flujo OAuth2 implementado (`internal/oauth`)

HelloJohn actúa como un **Authorization Server** certificable.

1.  **Authorize**: Valida `client_id`, `redirect_uri` y `scopes`. Si no hay sesión válida, redirige al Login.
2.  **Consent**: Si la aplicación requiere permisos explícitos (ej: "Leer contactos"), muestra la pantalla de consentimiento.
3.  **Token Minting**: Genera los tokens firmados incluyendo información del usuario (`sub`, `email`, `custom_claims`).
