# Configuración CORS para HelloJohn V2

## ✅ Problema Resuelto

El error de CORS:
```
Access to fetch at 'http://localhost:8080/v2/auth/login' from origin 'http://localhost:3000'
has been blocked by CORS policy: Response to preflight request doesn't pass access control check:
No 'Access-Control-Allow-Origin' header is present on the requested resource.
```

**Causa**: El servidor V2 no tenía configurado el middleware CORS globalmente.

**Solución**: Agregado middleware CORS global en `internal/app/v2/app.go`.

---

## 🔧 Cambios Realizados

### 1. Modificado `internal/app/v2/app.go`

**Agregado:**
- Import de `os` y `strings`
- Función `applyGlobalMiddlewares()` para wrappear el mux con CORS
- Función `getCORSOrigins()` para leer origins permitidos desde ENV

**Middleware CORS aplicado globalmente:**
```go
// 4. Apply global middlewares (CORS, etc)
handler := applyGlobalMiddlewares(mux)

return &App{
    Handler: handler,
}, nil
```

---

## ⚙️ Configuración

### Variables de Entorno

**`CORS_ALLOWED_ORIGINS`** - Lista de orígenes permitidos (separados por coma)

```bash
# .env o variables de entorno
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001,https://app.example.com

# O permitir todos (SOLO DESARROLLO):
CORS_ALLOWED_ORIGINS=*
```

**Default** (si no se especifica):
```
http://localhost:3000,http://localhost:3001
```

---

## 🚀 Uso

### Desarrollo Local

**Terminal 1: Backend V2**
```bash
cd hellojohn

# Con CORS por defecto (localhost:3000, localhost:3001)
FS_ROOT=./data/hellojohn \
SIGNING_MASTER_KEY=your-signing-key \
SECRETBOX_MASTER_KEY=your-secretbox-key \
V2_SERVER_ADDR=:8080 \
./hellojohn

# Con CORS personalizado
FS_ROOT=./data/hellojohn \
SIGNING_MASTER_KEY=your-signing-key \
SECRETBOX_MASTER_KEY=your-secretbox-key \
V2_SERVER_ADDR=:8080 \
CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:5173 \
./hellojohn
```

**Terminal 2: Frontend UI**
```bash
cd hellojohn/ui
npm run dev
```

### Producción

```bash
# Orígenes específicos (RECOMENDADO)
CORS_ALLOWED_ORIGINS=https://app.tudominio.com,https://admin.tudominio.com

# NUNCA uses "*" en producción
# CORS_ALLOWED_ORIGINS=*  ← ❌ INSEGURO
```

---

## 🔍 Verificación

### Comprobar CORS en el navegador

1. Abre DevTools (F12)
2. Ve a Network tab
3. Haz login desde el UI
4. Deberías ver:
   ```
   Request Headers:
   Origin: http://localhost:3000

   Response Headers:
   Access-Control-Allow-Origin: http://localhost:3000
   Access-Control-Allow-Credentials: true
   Access-Control-Allow-Methods: GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS
   Access-Control-Allow-Headers: Content-Type, Authorization, X-Request-ID, ...
   ```

### Probar desde curl

```bash
# Preflight request (OPTIONS)
curl -X OPTIONS http://localhost:8080/v2/auth/login \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: POST" \
  -H "Access-Control-Request-Headers: Content-Type" \
  -v

# Deberías ver:
# < Access-Control-Allow-Origin: http://localhost:3000
# < Access-Control-Allow-Credentials: true
# < HTTP/1.1 204 No Content
```

```bash
# Request real (POST)
curl -X POST http://localhost:8080/v2/auth/login \
  -H "Origin: http://localhost:3000" \
  -H "Content-Type: application/json" \
  -d '{"tenant_id":"local","client_id":"web-app","email":"test@example.com","password":"password"}' \
  -v

# Deberías ver:
# < Access-Control-Allow-Origin: http://localhost:3000
# < Access-Control-Allow-Credentials: true
# < HTTP/1.1 200 OK (o 401 si credenciales incorrectas)
```

---

## 📚 Detalles Técnicos

### Headers CORS Configurados

El middleware `WithCORS` configura los siguientes headers:

```go
Access-Control-Allow-Origin: <origin solicitado>
Access-Control-Allow-Credentials: true
Access-Control-Allow-Methods: GET,POST,PUT,PATCH,DELETE,HEAD,OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization, X-Request-ID, If-Match, X-Tenant-ID, X-Tenant-Slug, X-CSRF-Token
Access-Control-Expose-Headers: ETag, X-Request-ID, X-RateLimit-Remaining, X-RateLimit-Limit, X-RateLimit-Reset, Retry-After, WWW-Authenticate, Location
Access-Control-Max-Age: 600  // Preflight cache 10 minutos
```

### Vary Headers

Para compatibilidad con proxies/CDNs:
```
Vary: Origin
Vary: Access-Control-Request-Method
Vary: Access-Control-Request-Headers
```

### Preflight Requests

El middleware maneja automáticamente requests `OPTIONS` (preflight):
- Retorna `204 No Content`
- Incluye todos los headers CORS necesarios
- No ejecuta el handler real

---

## 🛡️ Seguridad

### ⚠️ Advertencias

1. **NUNCA uses `*` en producción**
   ```bash
   # ❌ INSEGURO en producción
   CORS_ALLOWED_ORIGINS=*

   # ✅ SEGURO en producción
   CORS_ALLOWED_ORIGINS=https://app.tudominio.com,https://admin.tudominio.com
   ```

2. **Siempre especifica protocolo (https/http)**
   ```bash
   # ❌ INCORRECTO
   CORS_ALLOWED_ORIGINS=tudominio.com

   # ✅ CORRECTO
   CORS_ALLOWED_ORIGINS=https://tudominio.com
   ```

3. **No incluyas trailing slashes**
   ```bash
   # ❌ INCORRECTO
   CORS_ALLOWED_ORIGINS=https://tudominio.com/

   # ✅ CORRECTO
   CORS_ALLOWED_ORIGINS=https://tudominio.com
   ```

### Mejores Prácticas

1. **Lista blanca específica en producción**
   - Solo orígenes conocidos y controlados
   - Revisar periódicamente la lista

2. **Diferentes configuraciones por ambiente**
   ```bash
   # Development
   CORS_ALLOWED_ORIGINS=http://localhost:3000,http://localhost:3001

   # Staging
   CORS_ALLOWED_ORIGINS=https://staging.tudominio.com

   # Production
   CORS_ALLOWED_ORIGINS=https://app.tudominio.com,https://admin.tudominio.com
   ```

3. **Monitorear requests CORS rechazados**
   - Logs de preflight fallidos
   - Alertas de orígenes no permitidos

4. **Habilitar CORS solo cuando sea necesario**
   - Si UI y API están en el mismo dominio: no necesitas CORS
   - Si usas proxy reverso (nginx): CORS puede no ser necesario

---

## 🔧 Troubleshooting

### Problema: Sigo viendo error CORS

**Soluciones:**

1. **Verificar que la variable esté configurada**
   ```bash
   echo $CORS_ALLOWED_ORIGINS
   ```

2. **Reiniciar el servidor**
   ```bash
   # Ctrl+C en el terminal del backend
   # Volver a ejecutar con la variable configurada
   ```

3. **Verificar ortografía del origin**
   ```bash
   # Frontend en: http://localhost:3000
   # CORS debe tener: http://localhost:3000 (exactamente igual)
   ```

4. **Limpiar caché del navegador**
   - Preflight responses se cachean 10 minutos
   - Abre DevTools → Network → Disable cache

5. **Verificar puerto correcto**
   ```bash
   # UI corre en :3000
   # API corre en :8080
   # CORS debe permitir localhost:3000
   ```

### Problema: Preflight request falla

**Verificar:**
```bash
curl -X OPTIONS http://localhost:8080/v2/auth/login \
  -H "Origin: http://localhost:3000" \
  -H "Access-Control-Request-Method: POST" \
  -v
```

**Debe retornar:**
- Status: `204 No Content`
- Header: `Access-Control-Allow-Origin: http://localhost:3000`

**Si no funciona:**
1. Verificar que el servidor esté usando la build actualizada
2. Recompilar: `go build -o hellojohn.exe ./cmd/service`
3. Reiniciar servidor

### Problema: CORS funciona pero API retorna 401

**Esto es normal**. CORS está funcionando correctamente. El error 401 es porque:
- Las credenciales son incorrectas
- El tenant no existe
- El client_id es inválido

**Verificar logs del backend** para ver el error real.

---

## 📝 Archivos Modificados

- `internal/app/v2/app.go` - Agregado middleware CORS global

---

## ✅ Checklist Post-Configuración

- [ ] Variable `CORS_ALLOWED_ORIGINS` configurada
- [ ] Servidor recompilado con `go build`
- [ ] Servidor reiniciado
- [ ] Preflight request exitoso (204)
- [ ] Request real retorna headers CORS
- [ ] UI puede hacer login sin error CORS

---

## 📚 Referencias

- `internal/http/v2/middlewares/cors.go` - Implementación del middleware
- `internal/app/v2/app.go` - Aplicación del middleware
- [MDN: CORS](https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS)
- [MDN: Preflight request](https://developer.mozilla.org/en-US/docs/Glossary/Preflight_request)
