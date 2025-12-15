/*
json.go — Helper interno para parseo JSON “estricto” (DisallowUnknownFields + límite 64KB) [NO es handler HTTP]

Qué es este archivo
-------------------
Este archivo NO expone endpoints HTTP directamente. Define un helper interno del package `handlers`
llamado `readStrictJSON(...)` que se usa como alternativa a `httpx.ReadJSON(...)` cuando se quiere:

- Validar Content-Type estrictamente (application/json)
- Limitar tamaño de body a 64KB
- Rechazar campos desconocidos (`json.Decoder.DisallowUnknownFields()`)
- Estandarizar errores con `httpx.WriteError(...)`

En otras palabras: es un “parser” de request bodies con política más dura que la función genérica
`internal/http/v1.ReadJSON`.

================================================================================
Qué hace (objetivo funcional)
================================================================================
`readStrictJSON(w, r, dst)`:
- Valida que `Content-Type` contenga `application/json`.
- Reemplaza `r.Body` por un `http.MaxBytesReader` de 64KB.
- Decodifica JSON en `dst` usando `json.Decoder`.
- Rechaza keys desconocidas (fail-fast por payloads inesperados).
- Responde errores HTTP y devuelve `false` si algo falla.

================================================================================
Cómo se usa (en el resto del código)
================================================================================
Este helper se invoca típicamente dentro de handlers donde el contrato de entrada es “estricto” y
querés detectar:
- typos del cliente
- payloads viejos con campos obsoletos
- inputs maliciosos que intentan “colarse” con campos extra

Ojo: muchos handlers v1 usan `httpx.ReadJSON` (tolerante) para no romper compatibilidad con clients.
Este helper es la otra cara: “preferí seguridad/claridad sobre compat”.

================================================================================
Flujo paso a paso (readStrictJSON)
================================================================================
1) Validación de Content-Type
	 - Lee `Content-Type` del request.
	 - Si NO contiene `application/json`:
		 - Responde `415 Unsupported Media Type`
		 - error_code=1101, code=`unsupported_media_type`
		 - msg: “se requiere Content-Type: application/json”
		 - retorna false.

2) Límite de tamaño
	 - Reemplaza `r.Body` por `http.MaxBytesReader(w, r.Body, 64<<10)`.
	 - Cierra body con `defer r.Body.Close()`.
	 - Impacto: protege contra payloads grandes (DoS / memory pressure).

3) Decode + unknown fields
	 - `json.NewDecoder(r.Body)`
	 - `dec.DisallowUnknownFields()`
	 - Si `dec.Decode(dst)` falla:
		 - Si `io.EOF` => trata como “body vacío”.
		 - Otros => “json inválido” (genérico).
		 - Responde `400 Bad Request` con code=`invalid_json`, error_code=1102.
		 - retorna false.

4) “Datos extra”
	 - Hace un chequeo de “sobran datos” con `dec.More()`.
	 - Si hay más tokens: 400 con error_code=1103.
	 - Nota: esta verificación es una idea correcta (un JSON válido debería terminar), pero
		 `dec.More()` sólo aplica en ciertos contextos (arrays/objetos). Si el objetivo fuera
		 “no hay trailing garbage”, el patrón usual es intentar un segundo `Decode(&struct{}{})`
		 y esperar `io.EOF`. (No cambiar acá; sólo marcarlo.)

================================================================================
Dependencias reales
================================================================================
- stdlib:
	- `encoding/json`, `io`, `net/http`, `strings`
- internas:
	- `internal/http/v1` como `httpx` para `WriteError`.

No usa `app.Container`, `Store`, `TenantSQLManager`, `cpctx.Provider`, `Issuer`, `Cache`, etc.

================================================================================
Seguridad / invariantes
================================================================================
- Límite 64KB: reduce superficie DoS por request bodies grandes.
- `DisallowUnknownFields`: ayuda a “fijar contrato” y evitar que cambios de API pasen desapercibidos.
- Errores con envelope consistente (`httpx.WriteError`).

================================================================================
Patrones detectados (GoF / arquitectura)
================================================================================
- Policy / Strategy (implícito):
	- Existen dos estrategias de parseo JSON:
		- `httpx.ReadJSON` (tolerante, 1MB, sin DisallowUnknownFields)
		- `readStrictJSON` (estricto, 64KB, DisallowUnknownFields)
	En V2 esto podría formalizarse como un “Decoder policy” configurable por endpoint.

No hay concurrencia ni estado compartido.

================================================================================
Cosas no usadas / legacy / riesgos
================================================================================
- El mensaje de error ante `Decode` es bastante genérico.
	- Está bien para no filtrar detalles, pero dificulta debugging de clientes.
	- En V2 se podría incluir un detalle acotado (ej: “campo desconocido: x”).

- Chequeo de trailing data:
	- `dec.More()` puede no cubrir todos los casos de basura trailing.
	- Si alguna ruta depende de esto como “seguridad”, conviene estandarizar el patrón.

================================================================================
Ideas para V2 (sin decidir nada)
================================================================================
1) Unificar “request decoding” como componente
	 - `RequestDecoder{ MaxBytes, StrictUnknownFields, RequireJSONContentType }`.
	 - El controller sólo pide: `DecodeJSON(r, &dto)` y recibe errores tipados.

2) Consistencia de errores
	 - Mantener el envelope `WriteError` y códigos internos.
	 - Alinear 415 vs 400 en todos los endpoints (hoy hay mezcla según handler).

3) Observabilidad
	 - En modo debug, loguear causa exacta (unknown field, syntax error), sin devolverla al cliente.

Guía de “desarme” en capas
--------------------------
- Transport/controller:
	- Decidir política (estricta vs tolerante) y mapear a error HTTP.
- Infra/util:
	- Implementación del decoder (esta función o un componente equivalente).

Resumen
------
- `json.go` no es un handler HTTP: es un helper de parseo JSON estricto que valida Content-Type,
	limita body a 64KB y rechaza campos desconocidos, devolviendo errores estándar vía `httpx.WriteError`.
*/

package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
)

func readStrictJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	ct := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Type")))
	if !strings.Contains(ct, "application/json") {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "se requiere Content-Type: application/json", 1101)
		return false
	}

	// limitar body a 64KB
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	defer r.Body.Close()

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		msg := "json inválido"
		switch err {
		case io.EOF:
			msg = "body vacío"
		default:
			// caemos en mensaje genérico; podemos refinar si querés
		}
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", msg, 1102)
		return false
	}

	// No debe haber datos extra
	if dec.More() {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_json", "sobran datos en el body", 1103)
		return false
	}

	return true
}
