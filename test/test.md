# test.ps1 / test.bat – Guía rápida y completa


---
## 1. ¿Qué valida este conjunto de pruebas?
Un recorrido integral del flujo de autenticación y autorización:

````markdown
# Carpeta `test/` – Guía profesional y práctica

Documentación completa de los scripts de prueba end‑to‑end. Objetivo: validar funcional y seguridad básica del servicio (auth, OIDC, correo, rotación de tokens, rate limit y headers). Escrita en tono simple y directo.

---
## TL;DR (lo mínimo para arrancar)

1. Levantá el backend (modo dev, con headers de depuración):

```
go run .\cmd\service\main.go -config .\configs\config.example.yaml -env-file .env.dev
```

2. Ejecutá el suite completo (interactivo):

```
cd test
test.bat
```

3. Modo automático (sin confirmar correos; usa headers X-Debug-*):

```
test.bat "" "" "" "" "" 1
```

Salida esperada al final (suite integral):

```
[DONE] Tests OK
```

---
## 1. Estructura y scripts

| Script | ¿Qué prueba? | ¿Cuándo usarlo? |
| ------ | ------------ | --------------- |
| `test.ps1` (+ `test.bat`) | **Suite integral**: health, ready, JWKS/discovery, CORS, registro/login, `/v1/me`, rotación y revocación refresh, flujos de email (verify/reset) interactivo o auto, sesión cookie, OAuth2/OIDC (Authorization Code + PKCE) + userinfo + validación `id_token` (`at_hash`, `nonce`, `azp`, `tid`), casos negativos (invalid_grant, invalid_scope, PKCE inválido, reuso code, redirect mismatch, login_required, invalid_client, unsupported_grant_type, invalid_redirect_uri), revoke RFC7009, cache headers. | Validar “todo ok” antes de merge / deploy. |
| `test_emailflows_e2e.ps1` | **E2E email flows** con auto-follow (`verify-email/start` → confirm, `forgot` → `reset` con autologin opcional). Reintenta login si la pass cambió. | Focalizado en flows de correo (requiere `EMAIL_DEBUG_LINKS=true`). |
| `test_cors_cookie.ps1` | Preflight **CORS** permitido/denegado + flags cookie de sesión (`HttpOnly`, `SameSite`, `Secure`). | Comparar comportamiento dev vs prod-like. |
| `test_rate_emailflows.ps1` | **Rate limit** sobre `/v1/auth/forgot`: 2 requests en misma ventana → `429` + `Retry-After`. | Validar umbral anti‑abuso. |
| `test_no_debug_headers.ps1` | Asegura ausencia de `X-Debug-Verify-Link` / `X-Debug-Reset-Link` (modo prod). | Comprobación de endurecimiento antes de prod. |
| `callback.html` | Página estática que imprime query/hash recibidos. | Inspección manual de redirecciones. |
| `codeprinter.bat` | Utilidad para volcar contenidos de archivos. | Soporte / diagnósticos puntuales. |

---
## 2. Matriz de entornos / variables clave

| Test | `EMAIL_DEBUG_LINKS` | `AUTH_RESET_AUTO_LOGIN` | Rate (`RATE_ENABLED / WINDOW / MAX_REQUESTS`) | Comentarios |
| ---- | ------------------- | ----------------------- | -------------------------------------------- | ----------- |
| Suite (`test.ps1`) | true recomendado en modo auto; opcional en interactivo | indiferente | indiferente | Interactivo no necesita headers debug. |
| `test_emailflows_e2e.ps1` | **true (requerido)** | opcional (200 vs 204 en reset) | indiferente | Sin headers debug no progresa. |
| `test_cors_cookie.ps1` | indiferente | indiferente | indiferente | Verificá `SERVER_CORS_ALLOWED_ORIGINS`. |
| `test_rate_emailflows.ps1` | indiferente | indiferente | **Configurar** (ej. 30s / 1) | Para reproducir 429 determinístico. |
| `test_no_debug_headers.ps1` | **false (requerido)** | indiferente | indiferente | Debe mostrar `<none>` en headers debug. |

---
## 3. Archivos `.env` sugeridos

### `.env.dev` (local con debug)
```
APP_ENV=dev
SERVER_ADDR=:8080
SERVER_CORS_ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:3000

STORAGE_DRIVER=postgres
STORAGE_DSN=postgres://user:password@localhost:5432/login?sslmode=disable
FLAGS_MIGRATE=true

CACHE_KIND=redis
REDIS_ADDR=localhost:6379
REDIS_DB=0
REDIS_PREFIX=login:
RATE_ENABLED=true
RATE_WINDOW=1m
RATE_MAX_REQUESTS=60

JWT_ISSUER=http://localhost:8080
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=720h

REGISTER_AUTO_LOGIN=true
AUTH_ALLOW_BEARER_SESSION=true
AUTH_SESSION_COOKIE_NAME=sid
AUTH_SESSION_SAMESITE=Lax
AUTH_SESSION_SECURE=false
AUTH_SESSION_TTL=12h

AUTH_VERIFY_TTL=48h
AUTH_RESET_TTL=1h
AUTH_RESET_AUTO_LOGIN=true

# SMTP (elige una opción)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USERNAME=tu@gmail.com
SMTP_PASSWORD=APP_PASSWORD
SMTP_FROM=tu@gmail.com
SMTP_TLS=starttls

# O alternativa dev (MailHog):
# SMTP_HOST=127.0.0.1
# SMTP_PORT=1025
# SMTP_FROM=dev@example.local
# SMTP_TLS=none

EMAIL_BASE_URL=http://localhost:8080
EMAIL_TEMPLATES_DIR=./templates
EMAIL_DEBUG_LINKS=true

SECURITY_PASSWORD_POLICY_MIN_LENGTH=10
SECURITY_PASSWORD_POLICY_REQUIRE_UPPER=true
SECURITY_PASSWORD_POLICY_REQUIRE_LOWER=true
SECURITY_PASSWORD_POLICY_REQUIRE_DIGIT=true
SECURITY_PASSWORD_POLICY_REQUIRE_SYMBOL=false
```

### `.env.prodlike` (sin headers debug)
```
APP_ENV=prod
SERVER_ADDR=:8080
SERVER_CORS_ALLOWED_ORIGINS=http://localhost:3000
EMAIL_DEBUG_LINKS=false
AUTH_SESSION_SECURE=true
# Resto igual a dev según necesidad
```

### `.env.rl` (rate limit determinístico)
```
APP_ENV=prod
RATE_ENABLED=true
RATE_WINDOW=30s
RATE_MAX_REQUESTS=1
# Hereda otras vars de dev/prodlike
```

Notas:
- Gmail requiere App Password (2FA habilitado).
- Para desarrollo rápido es más simple usar MailHog / Papercut.

---
## 4. Levantar backend
```
go run .\cmd\service\main.go -config .\configs\config.example.yaml -env-file .env.dev
```
Verificá logs: `service up ...` y `debug_links=true` si corresponde. Correos: buscar `smtp_send_ok`.

---
## 5. Ejecución por script

### Suite integral (`test.ps1` / `test.bat`)
Interactivo:
```
cd test
test.bat
```
Automático (sin prompts):
```
test.bat "" "" "" "" "" 1
```
Usar email exacto (sin plus-tag):
```
test.bat "" "" "" "" "tu@gmail.com" 1 1
```

### Email flows E2E
```
cd test
powershell -ExecutionPolicy Bypass -File .\test_emailflows_e2e.ps1
```
Esperado: verify 204 + link (→ 302), reset 200 (si autologin) o 204 (sin autologin).

### CORS + Cookie
```
cd test
powershell -ExecutionPolicy Bypass -File .\test_cors_cookie.ps1
```
Muestra preflights y atributos Set-Cookie.

### Rate limit forgot
```
cd test
powershell -ExecutionPolicy Bypass -File .\test_rate_emailflows.ps1
```
Esperado: primer 200, segundo 429 + Retry-After.

### Sin headers debug (prod-like)
```
cd test
powershell -ExecutionPolicy Bypass -File .\test_no_debug_headers.ps1
```
Debe imprimir `<none>` en ambos headers.

---
## 6. Parámetros del wrapper (`test.bat` → `test.ps1`)

| Pos | Nombre | Describe | Default |
| --- | ------ | -------- | ------- |
| 1 | `TENANT` | UUID tenant | `95f317cd-...` |
| 2 | `CLIENT` | Client ID registrado | `web-frontend` |
| 3 | `BASE` | URL base API | `http://localhost:8080` |
| 4 | `RATE_BURST` | Hits a `/healthz` para forzar 429 | `0` |
| 5 | `RealEmail` | Email base (para plus-tag) | (tu correo) |
| 6 | `AUTO_MAIL` | `1` = sin prompts (usa headers) | `0` |
| 7 | `USE_REAL_EMAIL` | `1` = sin plus-tag | `0` |
| 8 | `EMAIL_TAG` | Tag fijo (si vacío → random) | vacío |

Traducción interna:
- `AUTO_MAIL=1` → `-AutoMail`
- `USE_REAL_EMAIL=1` → `-UseRealEmail`
- `EMAIL_TAG` no vacío → `-EmailTag <valor>`

---
## 7. Validaciones profundas (suite integral)

- Salud: `/healthz`, `/readyz`.
- Public keys & discovery: `/.well-known/jwks.json`, `/.well-known/openid-configuration`.
- CORS: preflight permitido + denegado.
- Password flow: registro, login, `/v1/me`, verificación `alg=EdDSA`, claims (`aud`, `tid`, `iss`).
- Rotación refresh + revocación: reuse viejo → 401; logout → refresh inválido.
- Email flows: verify (start/confirm), forgot/reset (auto-login opcional).
- Sesión por cookie: `session/login` + authorize con cookie.
- OIDC Authorization Code + PKCE S256: code → token (access/refresh/id) + validación `at_hash`, `nonce`, `azp`, `tid`.
- Refresh grant: rotación y rechazo del previo.
- Revoke RFC7009: idempotente.
- Casos negativos: PKCE inválido, reuso code, redirect mismatch, scope inválido preservando `state`, sin `openid`, `login_required`, `invalid_client`, `unsupported_grant_type`, `invalid_redirect_uri`, reuse refresh.
- Headers: `Cache-Control: no-store`, `Pragma: no-cache` en endpoints de token; caching controlado en discovery/JWKS.
- UserInfo: GET y POST; 401 sin bearer.
- Rate limit (opcional): 429 y `Retry-After`.

---
## 8. Plus-tagging (Gmail)
`juan@gmail.com` acepta `juan+algo@gmail.com`. Se usa para generar usuarios únicos sin colisiones. Desactivar con `USE_REAL_EMAIL=1`. Controlar el tag con parámetro 8 (`EMAIL_TAG`).

---
## 9. Requisitos PowerShell
- Windows PowerShell 5.1 o PowerShell 7+. Scripts agregan `Add-Type` donde hace falta.
- Usar `-ExecutionPolicy Bypass` si hay restricciones.

---
## 10. Problemas comunes

| Síntoma | Causa | Solución |
| ------- | ----- | -------- |
| `smtp_send_err` / no llega mail | SMTP mal o sin App Password | Configurar App Password o usar MailHog (puerto 1025) |
| `invalid_redirect_uri` | Redirect no registrado | Usar `http://localhost:3000/callback` y registrarlo en client |
| `invalid_grant` (PKCE) inesperado | Verifier vs challenge desincronizados | No tocar pasos intermedios scripts |
| No aparece `X-Debug-*` | `EMAIL_DEBUG_LINKS=false` | Activar en `.env.dev` |
| `429` no aparece | Umbral alto o RATE_BURST=0 | Usar `.env.rl` y subir RATE_BURST |
| Login 401 en emailflows E2E | Password cambió previamente | Script hará forgot/reset; dejarlo continuar |

---
## 11. Uso en CI/CD
- Dev / Staging: `pwsh -File test/test.ps1 -AutoMail -RealEmail ci+login@example.com` (con `EMAIL_DEBUG_LINKS=true`).
- Prod-like: ejecutar `test_no_debug_headers.ps1` + `test_cors_cookie.ps1` + subset OIDC.
- Email flows aislados: `test_emailflows_e2e.ps1` (solo si headers debug habilitados).

---
## 12. Extensión futura sugerida
- MFA / TOTP.
- Listas negras de contraseñas.
- Métricas / observabilidad (latencias, tasa de errores) integradas.
- Pruebas de revocación masiva y caducidad forzada.
- Flujos de invitación / provisioning.

---
## 13. Resumen rápido final
Si el suite integral concluye en:
```
[DONE] Tests OK
```
Están cubiertos: registro, login, claims JWT, rotación/ revocación, email flows, sesión cookie, OIDC completo, errores negativos, headers de seguridad y (opcional) rate limit.

---
Fin.
````
