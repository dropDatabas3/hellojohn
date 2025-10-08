package app

import (
	"context"

	"github.com/dropDatabas3/hellojohn/internal/cache"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantsql"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/store"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

// Container es el contenedor DI simple que usamos en los handlers.
type Container struct {
	Store          core.Repository
	Issuer         *jwtx.Issuer
	Cache          cache.Cache
	Stores         *store.Stores                 // wrapper opcional con Close()
	ScopesConsents core.ScopesConsentsRepository // puede ser nil si driver != postgres

	// MultiLimiter para rate limits específicos por endpoint
	MultiLimiter helpers.MultiLimiter

	// TenantSQLManager para bases de datos por tenant (S3/S4)
	TenantSQLManager *tenantsql.Manager

	// ClaimsHook es opcional. Si está seteado, permite inyectar/alterar claims
	// de Access/ID Tokens a partir de una policy (CEL, webhooks, reglas estáticas, etc).
	// Convención:
	//   - Kind: "access" o "id"
	//   - Devuelve (addStd, addExtra) :
	//       * para "access": addStd -> claims top-level; addExtra -> claims bajo "custom"
	//       * para "id":     addStd -> claims top-level; addExtra -> claims top-level (extras benignos)
	ClaimsHook func(ctx context.Context, ev ClaimsEvent) (addStd map[string]any, addExtra map[string]any, err error)
}

// ClaimsEvent encapsula el contexto de emisión que se expone al hook.
type ClaimsEvent struct {
	Kind     string // "access" | "id"
	TenantID string
	ClientID string
	UserID   string
	Scope    []string
	AMR      []string
	// Campos libres para futuros usos (acr, auth_time, etc.)
	Extras map[string]any
}

// Close intenta cerrar recursos opcionales del contenedor (si existen).
func (c *Container) Close() error {
	if c.Stores != nil && c.Stores.Close != nil {
		return c.Stores.Close()
	}
	return nil
}
