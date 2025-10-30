package cpctx

import (
	"net/http"

	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
)

// Provider es el ControlPlane activo (FS en MVP).
var Provider cp.ControlPlane

// ResolveTenant resuelve el slug de tenant a partir del request.
// Por defecto: header > query > "local".
var ResolveTenant = func(r *http.Request) string {
	if v := r.Header.Get("X-Tenant-Slug"); v != "" {
		return v
	}
	if v := r.URL.Query().Get("tenant"); v != "" {
		return v
	}
	return "local"
}

// InvalidateJWKS permite invalidar la caché de JWKS (global o por tenant) desde capas sin acceso al contenedor.
// Si es nil, las llamadas deben ser no-ops en los consumidores.
var InvalidateJWKS func(tenant string)

// MarkFSDegraded allows lower layers (e.g., FS control-plane) to signal a degraded FS state.
// ClearFSDegraded clears the degraded flag when operations succeed again.
// These are optional hooks and should be no-ops if unset.
var MarkFSDegraded func(reason string)
var ClearFSDegraded func()
