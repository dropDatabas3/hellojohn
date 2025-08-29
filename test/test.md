# test.ps1 / test.bat – Guía rápida y completa


---
## 1. ¿Qué valida este conjunto de pruebas?
Un recorrido integral del flujo de autenticación y autorización:

1. Salud y readiness del servicio: `/healthz`, `/readyz`.
2. Publicación de claves (JWKS) y metadatos OIDC (discovery).
3. CORS: origen permitido vs no permitido.
4. Registro de usuario + login con password (emisión de access / refresh tokens).
5. Lectura de perfil autenticado (`/v1/me`).
6. Validación estructural del Access Token (alg=EdDSA, claims aud/tid/iss, etc.).
7. Rotación de refresh tokens y revocación (logout) + rechazo de usos viejos.
8. Flujos de correo: verificación de email y reset de contraseña (manual o auto‑follow de links de depuración).
9. Sesiones por cookie (`/v1/session/login` / `/v1/session/logout`).
10. Flujo OAuth 2.1 + OIDC Authorization Code + PKCE (authorize → token → userinfo → id_token claims & at_hash/nonce/azp/tid).
11. Grant refresh para obtener nuevos tokens y rotación consecutiva.
12. Revocación RFC7009 de refresh tokens (idempotente: siempre 200 / 204).
13. Casos negativos OIDC (PKCE inválido, reuso de code, redirect_uri incorrecto, scope inválido, falta de `openid`, login_required sin sesión, invalid_client, invalid_grant, unsupported_grant_type, invalid_redirect_uri, invalid_scope preservando state, revoke de token inexistente).
14. Comprobaciones de cabeceras de seguridad y caching (no-store / no-cache en endpoints de token, Cache-Control en discovery y JWKS, content-type correcto).
15. UserInfo por GET y POST + errores sin Bearer.
16. (Opcional) Rate limiting disparando 429 al exceder umbral.

Resultado esperado al final si todo pasa:
```
[DONE] Tests OK
[OK] Tests completos
```

---
## 2. Requisitos previos
- Backend corriendo localmente (por defecto: `http://localhost:8080`).
- Cliente / tenant de ejemplo cargados (coinciden con los defaults del script).
- SMTP configurado para que el backend emita los correos de verificación y reset (el script sólo pregunta si llegaron; no los lee del inbox).
- PowerShell (Windows) o pwsh (Core) disponible. El `.bat` actúa de wrapper para simplificar en Windows.

> Nota: Los enlaces de verificación / reset se obtienen desde headers de depuración (`X-Debug-Verify-Link`, `X-Debug-Reset-Link`). En producción normal no existirían.

---
## 3. Ejecución básica
La forma más simple (manual, sin auto‑seguir links, con e‑mail plus‐tag aleatorio):
```
(test) > test.bat
```
Se te pedirá confirmar si llegaron los dos correos (VERIFICACION y RESET). Responde:
- `y` si lo ves en tu inbox
- `n` para abortar (no llegó)
- `auto` si querés que el script siga el link devuelto por el backend (sin abrir tu correo)

---
## 4. Modos rápidos
1. Modo automático completo (no pregunta por correos, sigue links vía headers):
```
test.bat "" "" "" "" "" 1
```
2. Enviar correos exactamente al e‑mail real (sin plus tagging):
```
test.bat "" "" "" "" "" 0 1
```
3. Usar etiqueta fija para el plus tagging (en vez de una aleatoria):
```
test.bat "" "" "" "" "" 1 0 miEtiqueta
```
Tip: Podés editar los valores por defecto dentro de `test.bat` para evitar escribir tantos `""`.

---
## 5. Argumentos posicionales
`test.bat` pasa hasta 8 parámetros a `test.ps1`. Cualquier posición que dejes vacía mantiene su valor por defecto.

| # | Nombre | Explicación simple | Ejemplo / Default |
| - | ------- | ------------------ | ------------------ |
| 1 | `TENANT` | ID del tenant/espacio lógico de la instancia. | `b7268f99-...` (default) |
| 2 | `CLIENT` | ID del cliente (frontend) registrado. | `web-frontend` (default) |
| 3 | `BASE` | URL base del API bajo prueba. | `http://localhost:8080` |
| 4 | `RATE_BURST` | Nº de requests rápidos a `/healthz` para intentar gatillar 429. 0 = omitir. | `0` (default) |
| 5 | `RealEmail` | Tu e‑mail real (Gmail recomendado). | `tuusuario@gmail.com` |
| 6 | `AUTO_MAIL` | `1` = No pregunta; sigue links de verificación / reset automáticamente. | `0` (default) |
| 7 | `USE_REAL_EMAIL` | `1` = Usa el e‑mail exacto. `0` = aplica plus tagging. | `0` (default) |
| 8 | `EMAIL_TAG` | Etiqueta fija para plus tagging (si se quiere controlar). | (vacío → aleatoria) |

Internamente el `.bat` traduce:
- `AUTO_MAIL=1` → flag `-AutoMail` en PowerShell
- `USE_REAL_EMAIL=1` → flag `-UseRealEmail`
- `EMAIL_TAG` no vacío → `-EmailTag <valor>`

---
## 6. ¿Qué es el plus tagging de e‑mail?
Si tu correo es `juan@gmail.com`, Gmail también entrega mensajes enviados a:
```
juan+loquesea@gmail.com
```
El script aprovecha esto para generar usuarios únicos sin ensuciar tu inbox con colisiones de registros anteriores. Si activás `USE_REAL_EMAIL=1`, se omite el `+tag`.

---
## 7. Salida típica durante la ejecución
Prefijos que vas a ver:
- `[step]` Pasos generales (registro, login, refresh, etc.)
- `[email]` Inicio de flujo de correo (verificación / reset)
- `[oidc]` Operaciones del flujo OAuth / OIDC
- `[oidc:neg]` Casos negativos controlados
- `[session]` Sesiones basadas en cookie
- `[cfg]` Configuración dinámica (e‑mail usado)

Al final, si no hubo excepciones: `"[DONE] Tests OK"` y `"[OK] Tests completos"`.

Si algo falla, se lanza un error con texto tipo `ASSERT FAILED: <motivo>` o un `[FAIL]` desde el wrapper.

---
## 8. Detalle de validaciones clave
| Área | ¿Qué se comprueba? | Motivo |
| ---- | ------------------ | ------ |
| Salud | 200 en `/healthz` y `/readyz` | Servicio vivo y dependencias listas |
| CORS | Origen permitido vs denegado | Evitar exposición indebida |
| Tokens | Claims, algoritmo EdDSA, rotación refresh, revocación | Seguridad de autenticación |
| Email flows | Verificación y reset operativos | Onboarding y recuperación de cuenta |
| OIDC | Authorization Code + PKCE + ID Token integrity (`at_hash`, `nonce`, `azp`, `tid`) | Estándar interoperable |
| UserInfo | Datos disponibles con scope correcto | Conformidad OIDC |
| Cabeceras | `no-store`, `no-cache`, `Cache-Control` en discovery/JWKS | Buenas prácticas de caching y confidencialidad |
| Errores | invalid_grant, invalid_client, invalid_scope, etc. | Hardening y comportamiento seguro |
| Rate limit (opt) | Recepción de 429 | Protección contra abuso |

---
## 9. Problemas frecuentes y soluciones
| Problema | Causa probable | Acción |
| -------- | -------------- | ------ |
| No llega el mail | SMTP no configurado o va a Spam | Revisar logs del backend (buscar `smtp_send_ok`), carpeta Spam |
| Usuario ya existe | Reutilizás el mismo `+tag` | Dejar `EMAIL_TAG` vacío o poner otro |
| No aparece 429 | `RATE_BURST=0` o umbral alto | Subir valor de `RATE_BURST` |
| invalid_redirect_uri | Redirect no permitido | Asegurar que el redirect está registrado; usar el default del script |
| PKCE / invalid_grant inesperado | Verifier/challenge desincronizados | No modificar manualmente pasos internos |

---
## 10. Extensiones / Personalización
- Agregar nuevos checks: insertar nuevos `[step]` en `test.ps1` usando `Assert` para abortar limpiamente.
- Integración CI: Ejecutar directamente `pwsh -File test/test.ps1 -AutoMail -UseRealEmail -RealEmail "ci+login@example.com"` (si el entorno puede leer headers debug). En CI se recomienda desactivar prompts usando `-AutoMail`.
- Logs verbosos: se puede envolver las llamadas `Invoke-WebRequest` con más `Write-Host` si necesitás trazabilidad fina.

---
## 11. Resumen rápido (tl;dr)
Ejecutá `test.bat` con el backend levantado. Confirmá los dos correos. Si ves al final:
```
[DONE] Tests OK
[OK] Tests completos
```
está todo alineado: registro, login, tokens, OIDC, correos, revocación, seguridad básica y (si activaste) rate limit.

---
## 12. Contacto / Siguientes pasos
Si querés ampliar cobertura: agrega pruebas de MFA, listas negras de contraseñas, flujos de invitación o métricas de observabilidad. Este script ya es una base robusta para detectar regresiones críticas en autenticación.

---
Fin.
