<div align="center">

# HelloJohn – Suite de Tests (End-to-End & Funcionales)

Documentación completa y amigable para ejecutar, entender y extender las pruebas automatizadas del servicio.

</div>

---
## 1. Objetivo de la suite
Validar que el servicio funciona igual que lo haría en producción: configuración real, claves firmantes, base de datos migrada, semillas iniciales, emisión y validación de tokens, MFA, flujos de email, OAuth2/OIDC, rotación de claves y límites de uso.

En una sola ejecución se cubren: autenticación básica, rotación de refresh, sesiones navegador + PKCE, errores OIDC, MFA TOTP + recovery + trusted device, social login Google (opcional), introspección, revocación global, rate limiting, email flows, blacklist de passwords y rotación de claves JWT.

---
## 2. Estructura de carpetas
```
test/
├── e2e/
│   ├── TestMain_bootstrap_test.go   (bootstrap global: migra, seed, keys, arranca servicio)
│   ├── helpers.go                   (HTTP client, comandos, utilidades JSON, start server)
│   ├── totp.go                      (helpers TOTP)
│   ├── seed_types.go                (estructuras tipadas del seed)
│   ├── e2e_test.go                  (tests legacy iniciales)
│   ├── 00_smoke_discovery_test.go   (discovery + jwks)
│   ├── 01_auth_basic_test.go        (login básico + userinfo)
│   ├── 02_refresh_logout_test.go    (refresh rotativo + logout)
│   ├── 03_email_flows_test.go       (verify + reset tokens)
│   ├── 04_session_oidc_test.go      (authorize code + PKCE + userinfo)
│   ├── 05_oidc_negative_test.go     (casos de error OIDC)
│   ├── 06_mfa_test.go               (enroll + verify + challenge TOTP)
│   ├── 07_mfa_recovery_test.go      (uso/rotación recovery codes)
│   ├── 08_revoke_introspect_test.go (revocar y introspect)
│   ├── 09_rate_limit_test.go        (rate login/forgot/email flows)
│   ├── 10_rotate_keys_test.go       (rotación manual asistida)
│   ├── 11_social_google_test.go     (flujo Google real si config)
│   ├── 12_rate_emailflows_test.go   (límites email flows específicos)
│   ├── 13_emailflows_e2e_test.go    (cadenas email end-to-end)
│   ├── 14_jwt_rotation_test.go      (rotación programática JWT)
│   ├── 15_trusted_device_skip_test.go (salto TOTP por trusted device)
│   ├── 16_logout_all_test.go        (revocación masiva refresh)
│   ├── 17_mfa_negative_test.go      (fallos esperados MFA)
│   ├── 18_social_login_code_test.go (login_code exchange social)
│   ├── 20_introspect_test.go        (introspección avanzada)
│   ├── 21_password_blacklist_test.go(password blacklist)
│   ├── 22_admin_clients_test.go      (CRUD clientes + revoke sesiones)
│   ├── 23_admin_scopes_test.go       (CRUD scopes + delete guard 409)
│   ├── 24_admin_consents_test.go     (upsert/list/revoke consentimiento)
│   ├── 25_admin_users_disable_test.go(placeholder / futura desactivación)
│   ├── 26_token_claims_test.go       (claims token extendidos)
│   └── 99_social_google_manual_test.go (ejecución manual)
└── assets/
    └── callback.html                (página callback OAuth usada en tests)
```

---
## 3. Flujo de ejecución (Bootstrap)
El archivo `TestMain_bootstrap_test.go` realiza estos pasos antes de correr cualquier test:
1. Asegura `SIGNING_MASTER_KEY` (si falta genera una dummy de 64 hex para entorno de prueba).
2. Establece variables mínimas si no existen (`JWT_ISSUER`, `EMAIL_BASE_URL`, `INTROSPECT_BASIC_USER/PASS`, `SECURITY_PASSWORD_BLACKLIST_PATH`).
3. Ejecuta migraciones (`cmd/migrate`).
4. Genera al menos una clave firmante (`cmd/keys -rotate`).
5. Ejecuta seed (`cmd/seed`) creando tenant, usuario admin y client.
6. Arranca el servicio (`cmd/service`) en background sobre `:8080`.
7. Espera a que `/readyz` responda 200 y verifica JWKS / discovery.
8. Ejecuta todos los tests secuencialmente.
9. Finaliza proceso del servicio.

Si algún paso falla el resto de la suite se aborta—por eso es importante revisar logs en ejecuciones fallidas.

### 3.1 Notas bootstrap
* Semilla: si el YAML define `sub` pero no `id`, el loader asigna `ID = Sub` para compatibilidad retroactiva (evita 400 en upsert consents admin).
* Tests admin (22–24) asumen migración 0003 aplicada (`scope`, `user_consent`).
* OIDC tests (04/05) esperan ahora 302 directo en casos de autoconsent (sin paso intermedio consent_required).

---
## 4. Variables de entorno relevantes
Mínimas para un ciclo local completo:
| Variable | Propósito | Valor ejemplo |
|----------|----------|---------------|
| STORAGE_DSN | Conexión PostgreSQL | postgres://user:pass@localhost:5432/login?sslmode=disable |
| JWT_ISSUER | Base URL de emisión | http://localhost:8080 |
| SIGNING_MASTER_KEY | Cifrado de claves privadas | 64 hex chars |
| INTROSPECT_BASIC_USER | Basic auth introspect | introspect_user |
| INTROSPECT_BASIC_PASS | Basic auth introspect | introspect_pass |
| SECURITY_PASSWORD_BLACKLIST_PATH | Ruta blacklist | ./security_password_blacklist.txt |
| RATE_ENABLED | Activa rate tests | true |
| CACHE_KIND | memory o redis | memory (o redis) |
| REDIS_ADDR | Host:puerto (si redis) | localhost:6379 |
| GOOGLE_ENABLED | Habilitar tests sociales | false / true |
| GOOGLE_CLIENT_ID | OAuth Google | ...apps.googleusercontent.com |
| GOOGLE_CLIENT_SECRET | OAuth Google | secreto |
| SOCIAL_LOGIN_CODE_TTL | TTL login_code | 60s |

Cambios puntuales:
* Para forzar rate limit en memoria: `RATE_TEST_ALLOW_MEMORY=true`.
* Para saltar tests sociales, dejar `GOOGLE_ENABLED` vacío o false.

Orden de precedencia sigue: defaults → config.yaml → .env.dev → variables exportadas → flags.

---
## 5. Cómo ejecutar
### Todos los tests
```bash
cd test/e2e
go test -v -count=1 ./...
```

### Test(s) específicos por número
```bash
go test -v -run Test_06_MFA_TOTP
go test -v -run "Test_0[3-5]"    # tests 03,04,05
```

### Solo rotación JWT (test largo)
```bash
go test -v -run TestJWTKeyRotation
```

### Con timeout y cobertura
```bash
go test -v -timeout=6m -cover ./...
```

### Modo diagnóstico (fail rápido)
```bash
go test -v -failfast ./...
```

---
## 6. Descripción de cada conjunto de pruebas
| Archivo | Tema | Aspectos validados clave |
|---------|------|--------------------------|
| 00_smoke_discovery | Salud inicial | OIDC discovery, JWKS integridad, formato JSON |
| 01_auth_basic | Login base | Registro/login admin seed, userinfo, claims básicos |
| 02_refresh_logout | Ciclo refresh | Rotación (rotated_from), logout invalidando hash previo |
| 03_email_flows | Email tokens | Start verify, consumo verify, forgot, reset con password nuevo |
| 04_session_oidc | Authorization Code | PKCE S256, intercambio token, id token claims (nonce si aplica) |
| 05_oidc_negative | Errores OIDC | `invalid_grant`, misuse de code, scopes inválidos |
| 06_mfa_test | MFA TOTP básico | Enroll (otpauth), verify inicial, challenge, emisión con acr=loa:2 |
| 07_mfa_recovery | Recovery codes | Generación, uso único, rotación posterior segura |
| 08_revoke_introspect | Revocación + introspect | Revocar refresh y validar campo active=false |
| 09_rate_limit | Límite | Enforce 429 tras exceder ventanas login/forgot |
| 10_rotate_keys | Rotación manual | Inserción nueva clave, states active→retiring |
| 11_social_google | Google real | State firmado, intercambio id_token, upsert identity |
| 12_rate_emailflows | Rate email flows | Forgotten/reset beyond threshold = 429 |
| 13_emailflows_e2e | Secuencia completa | Verify + reset + login bajo escenarios combinados |
| 14_jwt_rotation | JWT rotation extendida | Múltiples claves, tokens viejos aún válidos, JWKS contiene retiring |
| 15_trusted_device_skip | Trusted device | Salta MFA en segundo login dentro del TTL remember |
| 16_logout_all | Revocación masiva | Todos los refresh del usuario quedan invalidados |
| 17_mfa_negative | MFA errores | Código inválido, recovery incorrecto, reuso recovery |
| 18_social_login_code | login_code | Generación y canje one-use social exchange |
| 20_introspect | Introspección | Access vs refresh, auth básica, campos active/exp/sub |
| 21_password_blacklist | Blacklist | Rechazo password débil y aceptación de segura |
| 22_admin_clients | Admin clientes | Crear/listar/editar, revoke y delete (soft/hard) |
| 23_admin_scopes | Admin scopes | Validación regex, conflicto, delete in-use=409 |
| 24_admin_consents | Admin consents | Upsert unión scopes, listar activos, revoke refresh |
| 25_admin_users_disable | Admin usuarios | (Roadmap) desactivar usuario y bloquear login |
| 26_token_claims | Claims extendidos | (Roadmap) validar claims agregados/RBAC |
| e2e_test (legacy) | Conjunto histórico | Smoke adicional + MFA gating simplificada |
| 99_social_google_manual | Manual | Iteraciones locales sin romper pipeline |

---
## 7. Helpers principales
| Función | Uso |
|---------|-----|
| startServer(...) | Lanza binario `cmd/service` con env controlado |
| runCmd(...) | Ejecuta comandos auxiliares (migrate, seed, keys) |
| newHTTPClient() | HTTP client con cookie jar para sesiones |
| mustJSON(...) | Decodifica respuesta JSON y falla test al error |
| GenerateTOTPCode(...) | Produce código válido 30s para secret base32 |
| randomEmail(tag) | Fabricar cuentas únicas (evita colisiones) |

---
## 8. Datos seed y reutilización
Los datos semilla se cargan una sola vez; los tests no recrean el estado. Para escenarios que requieren nuevos usuarios se usan correos aleatorios. Esto reduce el tiempo total y evita re-migraciones.

Estructura modelada en `seed_types.go` para acceso tipado (ej: `seed.Users.Admin.Email`).

---
## 9. Buenas prácticas al añadir un test nuevo
1. Evita dependencias en el orden (no asumas efectos colaterales de otro test).
2. Usa usuarios nuevos (random) si mutarás estado sensible (password, MFA, logout-all).
3. Usa helpers existentes; no re-implementar login o enroll TOTP.
4. Documenta en comentario inicial qué invariantes valida.
5. No hagas sleeps arbitrarios largos; preferí polling con timeout pequeño.

Plantilla mínima:
```go
func Test_XX_Descripcion(t *testing.T) {
  t.Helper()
  // Arrange
  email := randomEmail("xx")
  // Act: registrar, loguear, etc.
  // Assert: verificar status codes y campos esenciales
}
```

### 9.1 Pruebas Admin API específicas
* Evitar dependencias de orden: 22 crea clientes propios; 23/24 crean scopes/consents aislados.
* Reutilizar helpers para login/authorize en lugar de emitir tokens manualmente.
* Verificar códigos de error (`scope_in_use`, `scope_exists`, `missing_fields`) para robustez contract.

---
## 10. Troubleshooting
| Síntoma | Causa frecuente | Solución |
|---------|-----------------|----------|
| connection refused | Servicio no arrancó / puerto ocupado | Revisa logs bootstrap; libera :8080 |
| missing JWT keys | Falló `cmd/keys -rotate` | Ejecuta manualmente y reintenta |
| 401 inesperado | Clave cambiada o token expirado | Asegura refresh válido o revisa rotación |
| MFA siempre pide código | Trusted device TTL vencido / cookie no seteada | Revisa flag remember_device y cookie jar |
| Rate tests skip | RATE_ENABLED != true | Exporta RATE_ENABLED=true |
| Social tests skip | GOOGLE_ENABLED vacío | Exporta GOOGLE_* credenciales |
| Blacklist test falla | Ruta blacklist ausente | Define SECURITY_PASSWORD_BLACKLIST_PATH |
| scope delete 200 cuando esperaba 409 | Consent inexistente o no activo | Crear consentimiento primero (upsert) antes de delete |
| upsert consent 400 missing_fields | seed sin user_id válido | Confirmar mapping sub->id aplicado y user_id UUID |
| OIDC test espera consent_required | Autoconsent activo | Desactivar con CONSENT_AUTO=0 para validar rama estricta |

Logs detallados: `go test -v 2>&1 | tee out.log`.

---
## 11. Rendimiento estimado
| Sección | Tiempo (aprox) |
|---------|---------------|
| Bootstrap inicial | 10–15s |
| Cada test corto | 1–3s |
| JWT rotation (14) | 30–45s |
| Social Google (11) | 10–20s (variable red) |
| Suite completa | 2–3 min |

Optimizar: ejecutar subconjunto crítico usando `-run` o saltar rotación cuando no se modifica keystore.

---
## 12. Seguridad cubierta por tests
* Validación PKCE en intercambio.
* No reutilización refresh tras rotación.
* Uso único recovery codes y login_code social.
* Revocación masiva efectivamente invalida introspección.
* Headers de descubrimiento correctos y JWKS consistente.
* Rechazo de contraseñas débiles listadas.
* Enforced MFA gating antes de elevar ACR.

---
## 13. Ejemplos de ejecución avanzada
### Ejecutar sólo MFA (positivo + negativo + trusted)
```bash
go test -v -run "Test_06_MFA_TOTP|Test_07_MFA_Recovery|Test_15_TrustedDeviceSkip|Test_17_MFA_Negative"
```
### Ejecutar smoke + OIDC
```bash
go test -v -run "Test_00_Smoke_Discovery|Test_04_Session_OIDC_Code_PKCE"
```

---
## 14. Criterios de éxito
Indicadores mínimos tras una ejecución completa:
* Todos los tests PASS.
* Endpoint `/readyz` responde 200 consistentemente.
* JWKS contiene al menos 1 clave active y (si rotaste) otra retiring.
* Introspección marca `active:false` tras revocaciones.
* MFA expone `acr=urn:hellojohn:loa:2` en tokens post-challenge.

---
## 15. Roadmap de la suite
| Próxima mejora | Descripción |
|----------------|-------------|
| Tests WebAuthn | Cuando se implemente credencial FIDO | 
| Métricas Prometheus | Validar endpoint /metrics y etiquetas | 
| Pruebas de carga ligeras | Smoke de rendimiento (p50/p95) en login | 
| Tests multi-tenant avanzados | Aislamiento cross-tenant en social y refresh | 
| Admin disable user | Implementar test 25 al completar feature |
| Admin token claims | Llenar 26_token_claims_test.go con asserts de scopes extras |

---
### 16. Changelog de la suite
| Cambio | Detalle |
|--------|---------|
| Migración 0003 cubierta | Tests consumen scopes/consents persistidos |
| Nuevos tests 22–24 | Cobertura CRUD clientes / scopes / consents |
| Password blacklist | Test 21 añade seguridad a política de contraseñas |
| Semilla compatible | Fallback sub->id asegura estabilidad upsert consents |
| Autoconsent | Ajustes en expectativas de OIDC tests (04/05) |

---
Última actualización: Septiembre 2025 – Suite alineada con el estado actual.
