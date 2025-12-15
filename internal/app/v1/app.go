package app

import (
	"context"
	"sync/atomic"

	"github.com/dropDatabas3/hellojohn/internal/cache"
	"github.com/dropDatabas3/hellojohn/internal/cluster"
	"github.com/dropDatabas3/hellojohn/internal/email"
	"github.com/dropDatabas3/hellojohn/internal/http/v1/helpers"
	"github.com/dropDatabas3/hellojohn/internal/infra/tenantcache"
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
	JWKSCache      *jwtx.JWKSCache
	Stores         *store.Stores                 // wrapper opcional con Close()
	ScopesConsents core.ScopesConsentsRepository // puede ser nil si driver != postgres

	// MultiLimiter para rate limits específicos por endpoint
	MultiLimiter helpers.MultiLimiter

	// TenantSQLManager para bases de datos por tenant (S3/S4)
	TenantSQLManager *tenantsql.Manager

	// TenantCacheManager para caches por tenant
	TenantCacheManager *tenantcache.Manager

	// ClusterNode provee acceso a estado/rol de Raft embebido
	ClusterNode *cluster.Node

	// LeaderRedirects: nodeID -> baseURL para 307 opcional hacia el líder
	LeaderRedirects map[string]string

	// RedirectHostAllowlist: optional set of allowed hosts for 307 redirects
	// If empty or nil, legacy behavior applies (no host restriction beyond URL scheme check)
	RedirectHostAllowlist map[string]bool

	// FSDegraded is set to true when the FS control plane detects write errors;
	// readyz should surface this as a degraded status.
	FSDegraded atomic.Bool

	// ClaimsHook es opcional. Si está seteado, permite inyectar/alterar claims
	// de Access/ID Tokens a partir de una policy (CEL, webhooks, reglas estáticas, etc).
	// Convención:
	//   - Kind: "access" o "id"
	//   - Devuelve (addStd, addExtra) :
	//       * para "access": addStd -> claims top-level; addExtra -> claims bajo "custom"
	//       * para "id":     addStd -> claims top-level; addExtra -> claims top-level (extras benignos)
	ClaimsHook func(ctx context.Context, ev ClaimsEvent) (addStd map[string]any, addExtra map[string]any, err error)

	// SenderProvider resolves email sender for tenants
	SenderProvider email.SenderProvider
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

// SetFSDegraded flips the degraded flag; used by fs provider via cpctx hooks.
func (c *Container) SetFSDegraded(v bool) {
	if c == nil {
		return
	}
	if v {
		c.FSDegraded.Store(true)
	} else {
		c.FSDegraded.Store(false)
	}
}
