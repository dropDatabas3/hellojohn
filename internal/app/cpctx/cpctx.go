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
