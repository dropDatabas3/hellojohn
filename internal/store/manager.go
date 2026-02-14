package store

import (
	"context"
	"embed"
	"fmt"
	"log"
	"sync"

	"github.com/dropDatabas3/hellojohn/internal/cache"
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
)

// DataAccessLayer es el punto de entrada principal para acceso a datos.
// Implementado por Factory.
type DataAccessLayer interface {
	// ForTenant retorna acceso a datos para un tenant específico.
	ForTenant(ctx context.Context, slugOrID string) (TenantDataAccess, error)

	// ConfigAccess retorna acceso al control plane (siempre disponible).
	ConfigAccess() ConfigAccess

	// Mode retorna el modo operacional actual.
	Mode() OperationalMode

	// Capabilities retorna las capacidades del modo actual.
	Capabilities() ModeCapabilities

	// Stats retorna estadísticas de conexiones.
	Stats() FactoryStats

	// Cluster retorna el repositorio de cluster (nil si no configurado).
	Cluster() repository.ClusterRepository

	// MigrateTenant ejecuta migraciones para un tenant específico.
	MigrateTenant(ctx context.Context, slugOrID string) (*MigrationResult, error)

	// Close cierra todas las conexiones.
	Close() error
}

// TenantDataAccess agrupa todos los repositorios para un tenant específico.
type TenantDataAccess interface {
	// Identificación
	Slug() string
	ID() string
	Settings() *repository.TenantSettings
	Driver() string

	// Data plane (requieren DB)
	Users() repository.UserRepository
	Tokens() repository.TokenRepository
	MFA() repository.MFARepository
	Consents() repository.ConsentRepository
	RBAC() repository.RBACRepository
	Schema() repository.SchemaRepository
	EmailTokens() repository.EmailTokenRepository
	Identities() repository.IdentityRepository
	Sessions() repository.SessionRepository

	// Control plane (siempre disponibles vía FS)
	Clients() repository.ClientRepository
	Scopes() repository.ScopeRepository

	// Infraestructura
	Cache() cache.Client
	CacheRepo() repository.CacheRepository
	Mailer() MailSender

	// Operations
	InfraStats(ctx context.Context) (*TenantInfraStats, error)

	// Helpers
	HasDB() bool
	RequireDB() error
}

// ConfigAccess provee acceso al control plane (configuración global).
type ConfigAccess interface {
	Tenants() repository.TenantRepository
	Clients(tenantSlug string) repository.ClientRepository
	Scopes(tenantSlug string) repository.ScopeRepository
	Claims(tenantSlug string) repository.ClaimRepository
	Keys() repository.KeyRepository
	Admins() repository.AdminRepository
	AdminRefreshTokens() repository.AdminRefreshTokenRepository
}

// MailSender interface para envío de emails.
type MailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// ─── Manager (wrapper simplificado sobre Factory) ───

// Manager es un wrapper thread-safe sobre Factory que cachea TenantDataAccess.
// Útil cuando se quiere reutilizar TenantDataAccess entre requests.
type Manager struct {
	factory *Factory

	// Cache de TenantDataAccess
	tenants sync.Map // map[slug]TenantDataAccess
}

// ManagerConfig configuración para crear un Manager.
type ManagerConfig struct {
	FSRoot           string
	GlobalDB         *DBConfig
	DefaultTenantDB  *DBConfig
	MigrationsFS     embed.FS
	MigrationsDir    string
	Logger           *log.Logger
	SigningMasterKey string // hex, 64 chars - for encrypting signing keys
}

// NewManager crea un nuevo Manager.
func NewManager(ctx context.Context, cfg ManagerConfig) (*Manager, error) {
	factory, err := NewFactory(ctx, FactoryConfig{
		FSRoot:           cfg.FSRoot,
		GlobalDB:         cfg.GlobalDB,
		DefaultTenantDB:  cfg.DefaultTenantDB,
		MigrationsFS:     cfg.MigrationsFS,
		MigrationsDir:    cfg.MigrationsDir,
		Logger:           cfg.Logger,
		SigningMasterKey: cfg.SigningMasterKey,
	})
	if err != nil {
		return nil, err
	}

	return &Manager{factory: factory}, nil
}

// ForTenant retorna TenantDataAccess, cacheando el resultado.
// Si el tenant tiene DB configurada pero la conexión falló (lazy connection),
// el TDA NO se cachea para permitir reintentos en requests posteriores.
func (m *Manager) ForTenant(ctx context.Context, slugOrID string) (TenantDataAccess, error) {
	// Verificar cache
	if val, ok := m.tenants.Load(slugOrID); ok {
		return val.(TenantDataAccess), nil
	}

	// Crear nuevo
	tda, err := m.factory.ForTenant(ctx, slugOrID)
	if err != nil {
		return nil, err
	}

	// Solo cachear si la conexión DB está completa o si el tenant no tiene DB configurada.
	// Si el tenant tiene DB configurada pero la conexión falló (HasDB() == false),
	// NO cachear para que el próximo request reintente la conexión.
	dbConfigured := tda.Settings() != nil &&
		tda.Settings().UserDB != nil &&
		tda.Settings().UserDB.Driver != ""
	if tda.HasDB() || !dbConfigured {
		m.tenants.Store(tda.Slug(), tda)
		if tda.ID() != tda.Slug() {
			m.tenants.Store(tda.ID(), tda)
		}
	}

	return tda, nil
}

// ConfigAccess retorna acceso al control plane.
func (m *Manager) ConfigAccess() ConfigAccess {
	return m.factory.ConfigAccess()
}

// Mode retorna el modo operacional.
func (m *Manager) Mode() OperationalMode {
	return m.factory.Mode()
}

// Cluster retorna el repositorio de cluster (nil si no configurado).
func (m *Manager) Cluster() repository.ClusterRepository {
	return m.factory.Cluster()
}

// Capabilities retorna las capacidades del modo.
func (m *Manager) Capabilities() ModeCapabilities {
	return m.factory.Capabilities()
}

// Stats retorna estadísticas.
func (m *Manager) Stats() FactoryStats {
	return m.factory.Stats()
}

// ClearCache limpia el cache de TenantDataAccess.
func (m *Manager) ClearCache() {
	m.tenants = sync.Map{}
}

// ClearTenant limpia el cache para un tenant específico.
func (m *Manager) ClearTenant(slug string) {
	m.tenants.Delete(slug)
}

// RefreshTenant cierra la conexión existente y la recrea con la configuración actualizada.
// Útil cuando se cambia la configuración de DB de un tenant.
func (m *Manager) RefreshTenant(ctx context.Context, slug string) error {
	// 1. Limpiar cache del TDA
	m.tenants.Delete(slug)

	// 2. Cerrar conexión del pool (si existe)
	if m.factory != nil && m.factory.pool != nil {
		if err := m.factory.pool.Close(slug); err != nil {
			return fmt.Errorf("failed to close pool connection: %w", err)
		}
	}

	// La próxima llamada a ForTenant creará una nueva conexión
	return nil
}

// MigrateTenant ejecuta migraciones para un tenant específico.
func (m *Manager) MigrateTenant(ctx context.Context, slugOrID string) (*MigrationResult, error) {
	return m.factory.MigrateTenant(ctx, slugOrID)
}

// BootstrapResult contiene el resultado de hacer bootstrap de una DB de tenant.
type BootstrapResult struct {
	MigrationResult *MigrationResult // Resultado de migraciones SQL
	SyncedFields    []string         // Campos custom sincronizados
	Warnings        []string         // Warnings no fatales
	Error           error            // Error fatal (no se pudo conectar/migrar)
}

// BootstrapTenantDB inicializa la DB de un tenant: ejecuta migraciones y sincroniza custom fields.
// Retorna un BootstrapResult con info detallada. Si Error != nil, el bootstrap falló.
func (m *Manager) BootstrapTenantDB(ctx context.Context, slugOrID string) (result *BootstrapResult, err error) {
	result = &BootstrapResult{
		MigrationResult: &MigrationResult{}, // Initialize to avoid nil pointer
	}

	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			result.Error = fmt.Errorf("panic during bootstrap: %v", r)
			err = result.Error
		}
	}()

	// 1. Obtener TDA (esto conecta a la DB)
	tda, tdaErr := m.ForTenant(ctx, slugOrID)
	if tdaErr != nil {
		result.Error = fmt.Errorf("failed to connect to tenant DB: %w", tdaErr)
		return result, result.Error
	}

	// Verificar que tiene DB
	if !tda.HasDB() {
		result.Warnings = append(result.Warnings, "tenant has no database configured, skipping bootstrap")
		return result, nil // No es error, simplemente no hay DB
	}

	// 2. Ejecutar migraciones SQL
	migResult, migErr := m.MigrateTenant(ctx, slugOrID)
	if migResult != nil {
		result.MigrationResult = migResult
	}
	if migErr != nil {
		result.Error = fmt.Errorf("migration failed: %w", migErr)
		return result, result.Error
	}

	// 3. Sincronizar custom fields desde settings del tenant
	settings := tda.Settings()
	if settings != nil && len(settings.UserFields) > 0 {
		schema := tda.Schema()
		if schema != nil {
			// Convertir TenantSettings.UserFields a repository.UserFieldDefinition
			var fields []repository.UserFieldDefinition
			for _, f := range settings.UserFields {
				fields = append(fields, repository.UserFieldDefinition{
					Name:     f.Name,
					Type:     f.Type,
					Required: f.Required,
					Unique:   f.Unique,
					Indexed:  f.Indexed,
				})
				result.SyncedFields = append(result.SyncedFields, f.Name)
			}

			if syncErr := schema.SyncUserFields(ctx, tda.ID(), fields); syncErr != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("sync user fields partial error: %v", syncErr))
				// No retornamos error, solo warning
			}
		}
	}

	return result, nil
}

// Close cierra el manager y todas sus conexiones.
func (m *Manager) Close() error {
	return m.factory.Close()
}

// Ensure Manager implements DataAccessLayer
var _ DataAccessLayer = (*Manager)(nil)
