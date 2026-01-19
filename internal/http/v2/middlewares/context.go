package middlewares

import (
	"context"

	storev2 "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// =================================================================================
// CONTEXT KEYS
// =================================================================================

type ctxKey string

const (
	// ctxClaimsKey guarda las claims JWT parseadas
	ctxClaimsKey ctxKey = "claims"
	// ctxTenantKey guarda el TenantDataAccess
	ctxTenantKey ctxKey = "tenant"
	// ctxUserIDKey guarda el user ID extraído del token
	ctxUserIDKey ctxKey = "user_id"
	// ctxRequestIDKey guarda el request ID
	ctxRequestIDKey ctxKey = "request_id"
)

// =================================================================================
// CONTEXT SETTERS (Internos, usados por middlewares)
// =================================================================================

// WithClaims inyecta claims en el contexto
func WithClaims(ctx context.Context, claims map[string]any) context.Context {
	return context.WithValue(ctx, ctxClaimsKey, claims)
}

// WithTenant inyecta TenantDataAccess en el contexto
func WithTenant(ctx context.Context, tda storev2.TenantDataAccess) context.Context {
	return context.WithValue(ctx, ctxTenantKey, tda)
}

// WithUserID inyecta el user ID en el contexto
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, ctxUserIDKey, userID)
}

// setRequestID inyecta el request ID en el contexto (interno)
func setRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, ctxRequestIDKey, requestID)
}

// =================================================================================
// CONTEXT GETTERS (Públicos, usados por handlers/services)
// =================================================================================

// GetClaims obtiene las claims JWT del contexto.
// Retorna nil si no hay claims (token no validado o middleware no aplicado).
func GetClaims(ctx context.Context) map[string]any {
	if v := ctx.Value(ctxClaimsKey); v != nil {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}

// GetTenant obtiene el TenantDataAccess del contexto.
// Retorna nil si no hay tenant (middleware no aplicado o ruta sin tenant).
func GetTenant(ctx context.Context) storev2.TenantDataAccess {
	if v := ctx.Value(ctxTenantKey); v != nil {
		if tda, ok := v.(storev2.TenantDataAccess); ok {
			return tda
		}
	}
	return nil
}

// MustGetTenant obtiene el TenantDataAccess o hace panic.
// Usar solo en rutas donde el middleware de tenant SIEMPRE se aplica.
func MustGetTenant(ctx context.Context) storev2.TenantDataAccess {
	tda := GetTenant(ctx)
	if tda == nil {
		panic("middlewares: no tenant in context")
	}
	return tda
}

// GetUserID obtiene el user ID del contexto.
// Retorna cadena vacía si no hay user ID.
func GetUserID(ctx context.Context) string {
	if v := ctx.Value(ctxUserIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetRequestID obtiene el request ID del contexto.
// Retorna cadena vacía si no hay request ID.
func GetRequestID(ctx context.Context) string {
	if v := ctx.Value(ctxRequestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// =================================================================================
// CLAIM HELPERS
// =================================================================================

// ClaimString extrae un string de las claims.
func ClaimString(claims map[string]any, key string) string {
	if claims == nil {
		return ""
	}
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ClaimBool extrae un bool de las claims.
func ClaimBool(claims map[string]any, key string) bool {
	if claims == nil {
		return false
	}
	if v, ok := claims[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// ClaimStringSlice extrae un slice de strings de las claims.
func ClaimStringSlice(claims map[string]any, key string) []string {
	if claims == nil {
		return nil
	}
	v, ok := claims[key]
	if !ok {
		return nil
	}
	switch arr := v.(type) {
	case []string:
		return arr
	case []any:
		result := make([]string, 0, len(arr))
		for _, item := range arr {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	}
	return nil
}

// ClaimMap extrae un map de las claims.
func ClaimMap(claims map[string]any, key string) map[string]any {
	if claims == nil {
		return nil
	}
	if v, ok := claims[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}
