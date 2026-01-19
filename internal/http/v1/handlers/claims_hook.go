/*
claims_hook.go — Hook de claims (Access/ID) + merge “seguro” para no romper invariantes del token

Este archivo NO es un handler HTTP en sí, pero es una pieza clave que usan varios handlers
(login/refresh/register/oauth token) para “dejar enchufar” lógica externa (policies / reglas / scripts)
que agregue claims a los tokens sin permitir que pisen campos críticos.

================================================================================
Qué hace (objetivo funcional)
================================================================================
1) Define un set de claims “reservadas” (reservedTopLevel) que NO se pueden sobreescribir desde policies/hooks,
   porque romperían invariantes de seguridad y compat OIDC:
   - iss/sub/aud/exp/iat/nbf/jti/typ
   - at_hash/azp/nonce/tid
   - scp/scope/amr (se gestionan internamente por el servidor)

2) Implementa mergeSafe(dst, src, prevent):
   - Copia claves desde src a dst
   - NO copia si:
       a) la clave está en la lista prevent (comparación case-insensitive)
       b) dst ya tenía esa clave (no sobreescribe)
   - O sea: es “merge no destructivo” + “denylist” de claves.

3) Expone dos helpers:
   - applyAccessClaimsHook(...): para Access Token
   - applyIDClaimsHook(...): para ID Token
   Ambos:
     - Ejecutan c.ClaimsHook si está seteado
     - Fusionan addStd/addExtra en std/custom (o std/extra) usando mergeSafe
     - Bloquean explícitamente la inyección del namespace “del sistema” (sysNS) dentro de custom/top-level

================================================================================
Cómo se usa (en el resto del código)
================================================================================
- Handlers construyen mapas base:
    std   => claims estándar que el server controla (tid, amr, acr, scp, etc.)
    custom/extra => claims adicionales
- Luego llaman:
    std, custom = applyAccessClaimsHook(...)
  para que un hook externo agregue cosas sin romper lo importante.

Esto permite personalización por tenant/client/user (ej: “meter plan=pro”, “feature_flags”, “region”, etc.)
sin tocar el core del emisor de tokens.

================================================================================
Detalles importantes / invariantes de seguridad
================================================================================
A) reservedTopLevel
- Protege claims críticas y las que el server quiere controlar:
  - “amr” (métodos de autenticación), “scp/scope” (scopes) son sensibles.
  - “tid” también: cambiarlo rompe multi-tenancy.

B) Bloqueo del System Namespace (sysNS)
- sysNS = claims.SystemNamespace(c.Issuer.Iss)
  Esto parece ser el “namespace” donde el server mete claims internas (roles/perms/is_admin, etc.)
- applyAccessClaimsHook hace 2 defensas:
  1) En top-level: agrega sysNS a prevent (para que no lo inyecten “arriba”)
  2) En custom: delete(addExtra, sysNS) antes de mergear
  => evita que un hook se haga pasar por el sistema y fuerce roles/permisos, etc.

C) mergeSafe NO sobreescribe
- Esto define una regla clara: el core gana.
- Si el hook quiere cambiar algo ya seteado, no puede. (bien para seguridad, pero a veces limita casos de uso)

================================================================================
Patrones (GoF / arquitectura) que aparecen acá
================================================================================
1) Hook / Plugin (arquitectura)
- c.ClaimsHook funciona como punto de extensión (plugin).
- Muy en la línea de “Inversion of Control”: el core llama al hook, no al revés.

2) Chain of Responsibility (potencial)
- Hoy hay un solo hook (ClaimsHook). Pero la idea ya calza perfecto con CoR:
    - multiples policies/hooklets ejecutándose en orden
    - cada una agrega claims y el mergeSafe aplica reglas
  En V2, podrías permitir []ClaimsPolicy y componerlas.

3) Decorator (potencial)
- Podés pensar applyAccessClaimsHook como decorador del “claims builder”.
- Envuelve los mapas base y añade comportamiento (merge + bloqueos).

4) Guard Clauses / Fail-closed
- Si hook tira error => se ignora y se devuelve std/custom tal cual.
  (Ojo: esto es fail-open respecto a “enriquecimiento”, pero fail-closed respecto a seguridad: no agrega nada.)

================================================================================
Cosas a revisar / mejoras para V2 (sin decidir todavía)
================================================================================
1) Eficiencia micro
- slices.ContainsFunc dentro del loop por cada clave => O(n*m) (n=claves, m=prevent).
  Para pocos claims no importa, pero se puede:
    - precomputar un map[string]struct{} con prevent en lowercase
    - lookup O(1)
  (No es urgente, pero queda prolijo.)

2) Normalización de claves
- mergeSafe compara prevent con lk := strings.ToLower(k) pero después usa dst[k] “tal cual”.
  Esto puede permitir duplicados raros por case:
    dst["Foo"]=1 y src["foo"]=2 => no lo pisa porque “exists” mira k exacto, no lower.
  En JWT claims suele ser lowercase, pero no siempre.
  En V2, conviene:
    - o normalizar siempre a lower (excepto si querés mantener formato)
    - o chequear existencias case-insensitive también.

3) Control fino por “Kind”
- reservedTopLevel se usa para ambos tokens, pero en OIDC hay claims que aplican distinto.
  Capaz en V2:
    - reservedAccess vs reservedID
    - allowlist por tipo de token

4) Observabilidad
- Si el hook falla (err != nil) hoy se ignora sin log.
  En V2, un log debug/trace ayuda, sin romper requests.

================================================================================
Qué NO se usa / cosas a marcar
================================================================================
- Este archivo NO expone handlers HTTP; sólo helpers internos (ok).
- Import "strings", "slices" sí se usan.
- reservedTopLevel es var global (ok) y se consume en ambos apply*.

================================================================================
Resumen
================================================================================
- claims_hook.go es el “mecanismo de extensión” para agregar claims a Access/ID tokens.
- Lo hace de forma segura: no permite pisar claims críticas ni inyectar el namespace del sistema.
- Es un buen candidato a evolucionar a una cadena de policies (Chain of Responsibility) y a optimizar
  la lógica de merge (map de prevent + case-insensitive existence) en V2.
*/

package handlers

import (
	"context"
	"slices"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/claims"
)

// Reservadas que NO deben ser sobreescritas por policies para evitar romper invariantes.
var reservedTopLevel = []string{
	"iss", "sub", "aud", "exp", "iat", "nbf",
	"jti", "typ",
	"at_hash", "azp", "nonce", "tid",
	"scp", "scope", "amr", // las gestionamos nosotros
}

// mergeSafe copia src->dst evitando pisar claves reservadas (o las que ya estén en dst).
func mergeSafe(dst map[string]any, src map[string]any, prevent []string) {
	if dst == nil || src == nil {
		return
	}
	for k, v := range src {
		lk := strings.ToLower(k)
		if slices.ContainsFunc(prevent, func(s string) bool { return strings.EqualFold(s, lk) }) {
			continue
		}
		// no sobreescribimos si ya existe
		if _, exists := dst[k]; exists {
			continue
		}
		dst[k] = v
	}
}

// applyAccessClaimsHook ejecuta el hook (si existe) y fusiona en std/custom.
// Además, BLOQUEA que se intente escribir el namespace del sistema dentro de "custom".
func applyAccessClaimsHook(ctx context.Context, c *app.Container, tenantID, clientID, userID string, scopes, amr []string, std, custom map[string]any) (map[string]any, map[string]any) {
	if std == nil {
		std = map[string]any{}
	}
	if custom == nil {
		custom = map[string]any{}
	}
	if c != nil && c.ClaimsHook != nil {
		addStd, addExtra, err := c.ClaimsHook(ctx, app.ClaimsEvent{
			Kind:     "access",
			TenantID: tenantID,
			ClientID: clientID,
			UserID:   userID,
			Scope:    scopes,
			AMR:      amr,
			Extras:   map[string]any{},
		})
		if err == nil {
			// top-level: prevenir también el namespace del sistema por si algún hook intenta inyectarlo arriba
			sysNS := claims.SystemNamespace(c.Issuer.Iss)
			prevent := append(append([]string{}, reservedTopLevel...), sysNS)

			mergeSafe(std, addStd, prevent)

			// "custom": bloquear explícitamente el sysNS
			if addExtra != nil {
				delete(addExtra, sysNS)
				for k, v := range addExtra {
					if _, exists := custom[k]; !exists {
						custom[k] = v
					}
				}
			}
		}
	}
	return std, custom
}

// applyIDClaimsHook ejecuta el hook (si existe) y fusiona en std y extra (ambos top-level).
// También bloquea el namespace del sistema en el top-level del ID Token.
func applyIDClaimsHook(ctx context.Context, c *app.Container, tenantID, clientID, userID string, scopes, amr []string, std, extra map[string]any) (map[string]any, map[string]any) {
	if std == nil {
		std = map[string]any{}
	}
	if extra == nil {
		extra = map[string]any{}
	}
	if c != nil && c.ClaimsHook != nil {
		addStd, addExtra, err := c.ClaimsHook(ctx, app.ClaimsEvent{
			Kind:     "id",
			TenantID: tenantID,
			ClientID: clientID,
			UserID:   userID,
			Scope:    scopes,
			AMR:      amr,
			Extras:   map[string]any{},
		})
		if err == nil {
			sysNS := claims.SystemNamespace(c.Issuer.Iss)
			prevent := append(append([]string{}, reservedTopLevel...), sysNS)
			mergeSafe(std, addStd, prevent)
			mergeSafe(extra, addExtra, prevent)
		}
	}
	return std, extra
}
