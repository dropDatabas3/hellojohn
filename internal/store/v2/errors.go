package store

import "errors"

// Errores comunes del DAL.
var (
	// ErrTenantNotFound indica que el tenant no existe en el control plane.
	ErrTenantNotFound = errors.New("tenant not found")

	// ErrNoDBForTenant indica que el tenant no tiene DB configurada.
	ErrNoDBForTenant = errors.New("no database configured for tenant")

	// ErrNotLeader indica que la operación requiere ser leader del cluster.
	ErrNotLeader = errors.New("operation requires cluster leader")

	// ErrPreconditionFailed indica fallo de control de concurrencia optimista.
	ErrPreconditionFailed = errors.New("store: precondition failed")
)

// IsNoDBForTenant helper para verificar si el error es por falta de DB.
func IsNoDBForTenant(err error) bool {
	return errors.Is(err, ErrNoDBForTenant)
}

// IsTenantNotFound helper para verificar si el error es por tenant no encontrado.
func IsTenantNotFound(err error) bool {
	return errors.Is(err, ErrTenantNotFound)
}
