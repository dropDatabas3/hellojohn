/*
profile.go — “WhoAmI” / Profile resource (GET /v1/profile) basado en claims + lookup de user

Qué es este archivo (la posta)
------------------------------
Este archivo define NewProfileHandler(c) que expone un endpoint simple tipo “whoami”
para UI/CLI:
	- Lee claims desde context (inyectados por middleware RequireAuth)
	- Extrae sub (user id)
	- Busca el usuario en c.Store
	- Construye un payload “seguro” y relativamente estable (email + profile básico)

Este handler NO hace issuance de tokens, no maneja consent, y no escribe nada: es lectura.
Su valor está en ser un recurso protegido por scopes para validar que:
	- el access token es válido
	- el scope “profile:read” está funcionando
	- el aislamiento multi-tenant se respeta

Dependencias reales
-------------------
- httpx.GetClaims(ctx): claims del access token (set por RequireAuth)
- c.Store.GetUserByID(ctx, sub): lookup de usuario
- Campos de usuario usados:
		- u.ID, u.Email, u.EmailVerified
		- u.Metadata (given_name, family_name, name, picture)
		- u.TenantID (para guard multi-tenant)

Rutas soportadas (contrato efectivo)
------------------------------------
- GET /v1/profile
		Requiere:
			- middleware RequireAuth (para poblar claims)
			- middleware de scopes (en main se suele envolver con RequireScope("profile:read"))

		Response JSON (best-effort):
			{
				"sub": "<user_id>",
				"email": "...",
				"email_verified": true|false,
				"name": "...",
				"given_name": "...",
				"family_name": "...",
				"picture": "...",
				"updated_at": <unix>
			}

		Headers:
			- Content-Type: application/json
			- Cache-Control: no-store
			- Pragma: no-cache

Flujo interno (paso a paso)
----------------------------
1) Solo GET
	 - method != GET => 405

2) Claims desde context
	 - httpx.GetClaims(ctx) debe existir (si no, 401 missing_claims)
	 - Lee "sub" como string (si falta, 401 invalid_token)

3) Lookup de user
	 - c.Store.GetUserByID(ctx, sub)
	 - si no existe: 404 user_not_found

4) Guard multi-tenant (best effort)
	 - Si el token trae claim "tid": compara case-insensitive contra u.TenantID
	 - Si no coincide: 403 forbidden_tenant
	 Nota: esto es defensa-en-profundidad. El aislamiento “real” debería estar
	 garantizado antes (por issuance/validation, y por stores scoping por tenant).

5) Construcción del perfil
	 - Intenta sacar campos desde u.Metadata (map) y arma "name" si no viene
	 - updated_at hoy usa u.CreatedAt como placeholder (TODO en el código)

Seguridad / invariantes
-----------------------
- Scope enforcement:
	Este handler asume que un middleware externo exige "profile:read".
	El scope no se verifica aquí.

- Token type:
	Al depender de RequireAuth, se asume que es un access token válido.
	Hay tests e2e que buscan que no se acepte ID token como Bearer en /v1/profile.

- Cache:
	Bien: no-store para evitar que un proxy cachee PII.

Problemas principales (cuellos de botella + bugs probables)
-----------------------------------------------------------
1) updated_at incorrecto
	 Devuelve CreatedAt como updated_at. Es explícitamente best-effort.

2) Inconsistencia “profile” vs “metadata”
	 Toma campos de u.Metadata. En otros lugares del sistema existe profile/custom_fields.
	 Esto puede confundir a consumidores si esperan OIDC standard claims desde otro storage.

3) Multi-tenant guard depende de claim tid
	 Si tid no está presente, no hay guard acá.
	 V2: hacer que tid sea obligatorio en access tokens multi-tenant o que GetUserByID sea tenant-scoped.

Cómo lo refactorizaría a V2 (plan concreto)
-------------------------------------------
FASE 1 — Contrato de claims explícito
	- Definir un tipo Claims struct (sub, tid, scopes, amr, acr, ...)
	- Evitar map[string]any para claims.

FASE 2 — Profile service
	- services/profile_service.go:
			GetProfile(ctx, claims) -> ProfileDTO
	- Ese service decide qué campos exponer y desde qué fuente (metadata/profile/custom_fields).

FASE 3 — Aislamiento fuerte
	- Preferir repos tenant-scoped (GetUserByID(ctx, tenantID, userID))
	- O hacer obligatorio tid en tokens + validación central.

*/

package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
)

// NewProfileHandler exposes a real "whoami"/profile endpoint for UI/CLI use.
// Must be wrapped by RequireAuth and a scope check (profile:read).
func NewProfileHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo GET", 1000)
			return
		}
		// Claims are set by RequireAuth
		cl := httpx.GetClaims(r.Context())
		if cl == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "missing_claims", "no claims in context", 4012)
			return
		}
		sub := ""
		if v, ok := cl["sub"].(string); ok {
			sub = strings.TrimSpace(v)
		}
		if sub == "" {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid_token", "sub faltante", 4013)
			return
		}

		u, err := c.Store.GetUserByID(r.Context(), sub)
		if err != nil || u == nil {
			httpx.WriteError(w, http.StatusNotFound, "user_not_found", "usuario no encontrado", 2401)
			return
		}

		// Multi-tenant guard: if token has tid claim, ensure it matches the user's tenant
		if tidRaw, ok := cl["tid"].(string); ok && strings.TrimSpace(tidRaw) != "" {
			if !strings.EqualFold(strings.TrimSpace(tidRaw), strings.TrimSpace(u.TenantID)) {
				httpx.WriteError(w, http.StatusForbidden, "forbidden_tenant", "tenant mismatch", 2402)
				return
			}
		}

		// Build a safe, useful profile payload
		given := ""
		family := ""
		name := ""
		picture := ""
		if u.Metadata != nil {
			if s, ok := u.Metadata["given_name"].(string); ok {
				given = strings.TrimSpace(s)
			}
			if s, ok := u.Metadata["family_name"].(string); ok {
				family = strings.TrimSpace(s)
			}
			if s, ok := u.Metadata["name"].(string); ok {
				name = strings.TrimSpace(s)
			}
			if s, ok := u.Metadata["picture"].(string); ok {
				picture = strings.TrimSpace(s)
			}
		}
		if name == "" && (given != "" || family != "") {
			name = strings.TrimSpace(strings.TrimSpace(given + " " + family))
		}

		// TODO: persist and return real UpdatedAt (user.UpdatedAt) when available
		resp := map[string]any{
			"sub":            u.ID,
			"email":          u.Email,
			"email_verified": u.EmailVerified,
			"name":           name,
			"given_name":     given,
			"family_name":    family,
			"picture":        picture,
			"updated_at":     u.CreatedAt.Unix(), // best-effort; replace with updated_at when available
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
