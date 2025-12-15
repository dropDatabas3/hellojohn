/*
session_logout_util.go — helper local de hashing para sesiones (sid)

Qué es este archivo (la posta)
------------------------------
Este archivo existe únicamente para definir tokensSHA256(s) y poder reutilizarlo en
session_logout.go al construir la key de cache server-side:
	key := "sid:" + tokensSHA256(rawSID)

Funcionalmente es un duplicado del helper “canónico” que existe en
internal/security/token (tokens.SHA256Base64URL), con la misma idea:
	sha256(bytes) -> base64url sin padding.

Por qué existe / deuda técnica
------------------------------
- Históricamente, session_logout se implementó separado y no reutilizó tokens.*
- En session_login.go se usa tokens.SHA256Base64URL; en logout se usa tokensSHA256.
	Hoy parecen equivalentes (sha256 + RawURLEncoding), pero mantener dos helpers es frágil.

Riesgos
-------
1) Divergencia silenciosa
	 Si tokens.SHA256Base64URL cambia (o tokensSHA256 cambia) se rompe el contrato:
	 logout no borraría la sesión del cache aunque sí borre la cookie.

Mejora V2 (simple)
------------------
- Eliminar este archivo y usar tokens.SHA256Base64URL en session_logout.go,
	o mover a un package session y usar una única función para generar claves de cache.

*/

package handlers

import (
	"crypto/sha256"
	"encoding/base64"
)

func tokensSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
