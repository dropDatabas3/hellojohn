/*
Email Flows Wrappers (email_flows_wrappers.go)
───────────────────────────────────────────────────────────────────────────────
Qué es esto (y qué NO es)
- NO es un handler “nuevo” con lógica propia.
- Es un set de “wrappers/fábricas chiquitas” que devuelven `http.HandlerFunc` apuntando a métodos
  de `EmailFlowsHandler`.
- Sirve para estandarizar cómo se exponen los handlers (y evitar referenciar métodos privados
  directamente desde el wiring/router en algunos lugares).

Qué expone
- `NewVerifyEmailStartHandler(h)`   -> devuelve `h.verifyEmailStart`
- `NewVerifyEmailConfirmHandler(h)` -> devuelve `h.verifyEmailConfirm`
- `NewForgotHandler(h)`             -> devuelve `h.forgot`
- `NewResetHandler(h)`              -> devuelve `h.reset`

Cómo se usa
- En el router o en el wiring:
  - `router.HandleFunc("/v1/auth/verify-email/start", NewVerifyEmailStartHandler(ef))`
  - etc.
- Alternativa equivalente (sin wrappers): `http.HandlerFunc(ef.verifyEmailStart)`
  (estos wrappers existen por prolijidad/consistencia o para esconder métodos no-exportados).

Dependencias reales
- `net/http` solamente.
- Entrada: `*EmailFlowsHandler` (definido en `email_flows.go` / `email_flows_wiring.go`).

Seguridad / invariantes
- No aplica: acá no se parsea request, no se valida nada, no se tocan tokens.
- La seguridad vive dentro de `EmailFlowsHandler` (y sus adapters / stores).

Patrones detectados
- Factory Method (GoF, ultra simple):
  - Son “factory functions” que entregan un `http.HandlerFunc` preconfigurado (en realidad, un method value).
- Facade (micro):
  - Evitan que el código externo conozca el nombre exacto de los métodos internos (`verifyEmailStart`, etc.).

Cosas no usadas / legacy / riesgos
- (No se usa) potencialmente: si en el proyecto ya se registran endpoints con `http.HandlerFunc(ef.verifyEmailStart)`
  directamente, estas funciones quedan como “azúcar” y pueden ser redundantes.
- OJO: estos wrappers no chequean `h == nil`. Si alguien llama `NewResetHandler(nil)` te comés un panic al usarlo.
  (No es grave si el wiring está bien, pero es un edge-case a marcar.)

Ideas para V2 (sin decidir nada)
- DTO: no hay.
- Controller: no hay lógica, así que podrían eliminarse si no aportan.
- Service/Client/Repo: no hay.
- Si se quieren mantener, podría:
  - agregar un nil-check y devolver un handler que responda 500 (o panic explícito) para fallar rápido y claro.
  - o agruparlos en un `EmailRoutes(ef *EmailFlowsHandler) map[string]http.HandlerFunc` para registrar rutas en bloque.

Resumen
- Archivo utilitario minimalista: expone funciones constructoras que devuelven handlers ya implementados por `EmailFlowsHandler`.
- No agrega lógica ni seguridad; sólo ayuda a conectar métodos privados con el router de forma consistente.
*/

package handlers

import "net/http"

func NewVerifyEmailStartHandler(h *EmailFlowsHandler) http.HandlerFunc   { return h.verifyEmailStart }
func NewVerifyEmailConfirmHandler(h *EmailFlowsHandler) http.HandlerFunc { return h.verifyEmailConfirm }
func NewForgotHandler(h *EmailFlowsHandler) http.HandlerFunc             { return h.forgot }
func NewResetHandler(h *EmailFlowsHandler) http.HandlerFunc              { return h.reset }
