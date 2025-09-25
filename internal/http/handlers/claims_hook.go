package handlers

import (
	"context"
	"slices"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
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
