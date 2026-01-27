package store

import "strings"

// OperationalMode define el modo de operación del Data Layer.
type OperationalMode int

const (
	// ModeFSOnly: Solo FileSystem. Sin base de datos.
	// Útil para: desarrollo, testing, tenants sin usuarios.
	// Capacidades: tenants, clients, scopes, admins, branding.
	// NO soporta: usuarios, tokens, MFA, consents.
	ModeFSOnly OperationalMode = iota + 1

	// ModeFSGlobalDB: FileSystem + DB Global.
	// La DB global almacena config como backup/sync del FS.
	// Útil para: clusters grandes que no quieren Raft.
	// Capacidades: todo de ModeFSOnly + backup en DB.
	ModeFSGlobalDB

	// ModeFSTenantDB: FileSystem + DB por Tenant.
	// Cada tenant tiene su propia base de datos para user data.
	// Útil para: SaaS multi-tenant con aislamiento fuerte.
	// Capacidades: todo de ModeFSOnly + users, tokens, MFA por tenant.
	ModeFSTenantDB

	// ModeFullDB: FileSystem + DB Global + DB por Tenant.
	// Máxima capacidad. Config en global, data en tenant.
	// Útil para: empresas grandes, compliance estricto.
	// Capacidades: todas.
	ModeFullDB
)

// String retorna nombre legible del modo.
func (m OperationalMode) String() string {
	switch m {
	case ModeFSOnly:
		return "fs_only"
	case ModeFSGlobalDB:
		return "fs_global_db"
	case ModeFSTenantDB:
		return "fs_tenant_db"
	case ModeFullDB:
		return "full_db"
	default:
		return "unknown"
	}
}

// Description retorna descripción del modo.
func (m OperationalMode) Description() string {
	switch m {
	case ModeFSOnly:
		return "Solo FileSystem (sin DB)"
	case ModeFSGlobalDB:
		return "FileSystem + DB Global"
	case ModeFSTenantDB:
		return "FileSystem + DB por Tenant"
	case ModeFullDB:
		return "FileSystem + DB Global + DB por Tenant"
	default:
		return "Modo desconocido"
	}
}

// SupportsUsers indica si el modo soporta operaciones de usuarios.
func (m OperationalMode) SupportsUsers() bool {
	return m == ModeFSTenantDB || m == ModeFullDB
}

// SupportsGlobalDB indica si hay DB global disponible.
func (m OperationalMode) SupportsGlobalDB() bool {
	return m == ModeFSGlobalDB || m == ModeFullDB
}

// SupportsTenantDB indica si soporta DB por tenant.
func (m OperationalMode) SupportsTenantDB() bool {
	return m == ModeFSTenantDB || m == ModeFullDB
}

// ModeConfig configuración para detectar el modo.
type ModeConfig struct {
	// FSRoot path al directorio del control plane (requerido).
	FSRoot string

	// GlobalDB configuración de DB global (opcional).
	GlobalDB *DBConfig

	// DefaultTenantDB configuración default para tenants nuevos (opcional).
	DefaultTenantDB *DBConfig
}

// DBConfig configuración de base de datos.
type DBConfig struct {
	Driver string // postgres, mysql, mongo
	DSN    string
	Schema string // para multi-schema en misma DB

	// Pool settings
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime string // e.g. "5m"
}

// Valid verifica si la config de DB es válida.
func (c *DBConfig) Valid() bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.Driver) != "" && strings.TrimSpace(c.DSN) != ""
}

// DetectMode detecta automáticamente el modo operacional.
//
// Reglas:
//   - Si hay GlobalDB Y DefaultTenantDB → ModeFullDB
//   - Si solo hay DefaultTenantDB → ModeFSTenantDB
//   - Si solo hay GlobalDB → ModeFSGlobalDB
//   - Sin ninguna DB → ModeFSOnly
func DetectMode(cfg ModeConfig) OperationalMode {
	hasGlobal := cfg.GlobalDB.Valid()
	hasTenantDefault := cfg.DefaultTenantDB.Valid()

	switch {
	case hasGlobal && hasTenantDefault:
		return ModeFullDB
	case !hasGlobal && hasTenantDefault:
		return ModeFSTenantDB
	case hasGlobal && !hasTenantDefault:
		return ModeFSGlobalDB
	default:
		return ModeFSOnly
	}
}

// ParseMode parsea un string a OperationalMode.
func ParseMode(s string) OperationalMode {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "fs_only", "fs-only", "1":
		return ModeFSOnly
	case "fs_global_db", "fs-global-db", "2":
		return ModeFSGlobalDB
	case "fs_tenant_db", "fs-tenant-db", "3":
		return ModeFSTenantDB
	case "full_db", "full-db", "4":
		return ModeFullDB
	default:
		return 0 // inválido
	}
}

// ModeCapabilities describe las capacidades de cada modo.
type ModeCapabilities struct {
	Mode OperationalMode

	// Config operations (siempre disponibles con FS)
	Tenants  bool
	Clients  bool
	Scopes   bool
	Admins   bool
	Branding bool

	// Data operations (requieren DB)
	Users    bool
	Tokens   bool
	MFA      bool
	Consents bool
	RBAC     bool

	// Infra
	GlobalDBSync bool
	TenantDB     bool
	Cache        bool
}

// GetCapabilities retorna las capacidades del modo.
func GetCapabilities(mode OperationalMode) ModeCapabilities {
	caps := ModeCapabilities{
		Mode: mode,
		// Config siempre disponible (FS)
		Tenants:  true,
		Clients:  true,
		Scopes:   true,
		Admins:   true,
		Branding: true,
		// Cache siempre disponible (al menos memory)
		Cache: true,
	}

	switch mode {
	case ModeFSGlobalDB:
		caps.GlobalDBSync = true
	case ModeFSTenantDB:
		caps.TenantDB = true
		caps.Users = true
		caps.Tokens = true
		caps.MFA = true
		caps.Consents = true
		caps.RBAC = true
	case ModeFullDB:
		caps.GlobalDBSync = true
		caps.TenantDB = true
		caps.Users = true
		caps.Tokens = true
		caps.MFA = true
		caps.Consents = true
		caps.RBAC = true
	}

	return caps
}
