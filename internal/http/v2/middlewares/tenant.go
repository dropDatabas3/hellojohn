package middlewares

import (
	"log"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/http/v2/helpers"
	"github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// TenantResolver define cómo obtener el tenant slug de un request.
type TenantResolver func(r *http.Request) string

// HeaderTenantResolver resuelve usando el header X-Tenant-ID.
func HeaderTenantResolver(headerName string) TenantResolver {
	if headerName == "" {
		headerName = "X-Tenant-ID"
	}
	return func(r *http.Request) string {
		return r.Header.Get(headerName)
	}
}

// TenantMiddleware inyecta el TenantDataAccess en el contexto.
type TenantMiddleware struct {
	manager  *store.Manager
	resolver TenantResolver
}

// NewTenantMiddleware crea un nuevo middleware de tenant.
// Si resolver es nil, usa HeaderTenantResolver("X-Tenant-ID").
func NewTenantMiddleware(mgr *store.Manager, resolver TenantResolver) *TenantMiddleware {
	if resolver == nil {
		resolver = HeaderTenantResolver("")
	}
	return &TenantMiddleware{
		manager:  mgr,
		resolver: resolver,
	}
}

// Handle intercepta el request, resuelve el tenant y carga el DataAccess.
func (m *TenantMiddleware) Handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slug := m.resolver(r)
		if slug == "" {
			// Intento fallback: Subdominio (ej: acme.hellojohn.com)
			host := r.Host
			if strings.Count(host, ".") > 1 {
				parts := strings.Split(host, ".")
				slug = parts[0]
			}
		}

		if slug == "" {
			helpers.WriteErrorJSON(w, http.StatusBadRequest, "missing tenant identifier")
			return
		}

		// Obtener Data Access desde Store V2 Manager
		// El Manager ya tiene cache interno, por lo que esta llamada es rápida.
		tda, err := m.manager.ForTenant(r.Context(), slug)
		if err != nil {
			log.Printf("TenantMiddleware: error loading tenant %q: %v", slug, err)
			// Asumimos que si hay error es porque el tenant no existe o no se puede cargar
			// Podríamos diferenciar errores del store en el futuro.
			helpers.WriteErrorJSON(w, http.StatusNotFound, "tenant not found")
			return
		}

		// Inyectar en contexto
		ctx := helpers.WithTenantDataAccess(r.Context(), tda)

		// También inyectar logger con prefijo tenant si es necesario
		// ...

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
