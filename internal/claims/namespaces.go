package claims

import "strings"

const devSysNSFallback = "https://hellojohn.local/claims/sys"

// SystemNamespace construye el namespace de claims "de sistema" anclado al issuer.
// Ej: https://issuer.example/claims/sys
func SystemNamespace(issuer string) string {
	iss := strings.TrimSpace(issuer)
	if iss == "" {
		return devSysNSFallback // sólo dev
	}
	return strings.TrimRight(iss, "/") + "/claims/sys"
}
