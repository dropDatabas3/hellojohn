package store

import (
	"context"
	"embed"
	"fmt"
	"log"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/cache/v2"
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/security/secretbox"
)

// Factory crea y configura el DataAccessLayer completo.
type Factory struct {
	cfg      FactoryConfig
	mode     OperationalMode
	caps     ModeCapabilities
	fsConn   AdapterConnection
	pool     *ConnectionPool
	migrator *Migrator
	cluster  repository.ClusterRepository // opcional, para replicación
}

// FactoryConfig configuración para crear el Factory.
type FactoryConfig struct {
	// FSRoot path al directorio de datos (requerido)
	FSRoot string

	// GlobalDB configuración de DB global (opcional, para Modo 2 y 4)
	GlobalDB *DBConfig

	// DefaultTenantDB configuración default para nuevos tenants (opcional)
	DefaultTenantDB *DBConfig

	// Cluster repositorio para replicación de control plane (opcional)
	// Si es nil, operaciones mutantes son single-node.
	Cluster repository.ClusterRepository

	// MigrationsFS sistema de archivos embebido con migraciones SQL
	MigrationsFS  embed.FS
	MigrationsDir string

	// Mode fuerza un modo específico (0 = auto-detect)
	Mode OperationalMode

	// Logger para debug (opcional)
	Logger *log.Logger

	// SigningMasterKey para encriptar/desencriptar claves de firma (hex, 64 chars)
	SigningMasterKey string

	// OnTenantConnect callback cuando se conecta un tenant
	OnTenantConnect func(slug string, driver string)
}

// NewFactory crea un nuevo Factory.
func NewFactory(ctx context.Context, cfg FactoryConfig) (*Factory, error) {
	// Detectar modo
	mode := cfg.Mode
	if mode == 0 {
		mode = DetectMode(ModeConfig{
			FSRoot:          cfg.FSRoot,
			GlobalDB:        cfg.GlobalDB,
			DefaultTenantDB: cfg.DefaultTenantDB,
		})
	}

	f := &Factory{
		cfg:     cfg,
		mode:    mode,
		caps:    GetCapabilities(mode),
		cluster: cfg.Cluster,
	}

	// Log modo
	if cfg.Logger != nil {
		cfg.Logger.Printf("store/v2: operating in mode %s", mode)
	}

	// Conectar al FileSystem (siempre requerido)
	fsConn, err := OpenAdapter(ctx, AdapterConfig{
		Name:             "fs",
		FSRoot:           cfg.FSRoot,
		SigningMasterKey: cfg.SigningMasterKey,
	})
	if err != nil {
		return nil, fmt.Errorf("factory: connect fs: %w", err)
	}
	f.fsConn = fsConn

	// Crear pool de conexiones para tenants
	f.pool = NewConnectionPool(f.createTenantConnection, PoolConfig{
		OnConnect: func(slug string, conn AdapterConnection) {
			if cfg.OnTenantConnect != nil {
				cfg.OnTenantConnect(slug, conn.Name())
			}
		},
	})

	// Crear migrator si hay migraciones
	if cfg.MigrationsDir != "" {
		f.migrator = NewMigrator(cfg.MigrationsFS, cfg.MigrationsDir)
	}

	return f, nil
}

// Mode retorna el modo operacional detectado/configurado.
func (f *Factory) Mode() OperationalMode {
	return f.mode
}

// Cluster retorna el repositorio de cluster (nil si no configurado).
func (f *Factory) Cluster() repository.ClusterRepository {
	return f.cluster
}

// ClusterHook retorna un hook para aplicar mutaciones al cluster.
func (f *Factory) ClusterHook() *ClusterHook {
	return NewClusterHook(f.cluster, f.mode)
}

// Capabilities retorna las capacidades del modo actual.
func (f *Factory) Capabilities() ModeCapabilities {
	return f.caps
}

// ConfigAccess retorna acceso al control plane (FS).
func (f *Factory) ConfigAccess() ConfigAccess {
	return &factoryConfigAccess{fs: f.fsConn}
}

// ForTenant retorna TenantDataAccess para un tenant específico.
func (f *Factory) ForTenant(ctx context.Context, slugOrID string) (TenantDataAccess, error) {
	// Resolver tenant desde FS
	tenant, err := f.resolveTenant(ctx, slugOrID)
	if err != nil {
		return nil, err
	}

	// Obtener o crear conexión de datos
	dataConn, err := f.getDataConnection(ctx, tenant)
	if err != nil {
		return nil, err
	}

	// Obtener o crear cache
	cacheClient := f.getCache(ctx, tenant)

	return &tenantAccess{
		tenant:   tenant,
		dataConn: dataConn,
		fsConn:   f.fsConn,
		cache:    cacheClient,
		mode:     f.mode,
	}, nil
}

// Close cierra todas las conexiones.
func (f *Factory) Close() error {
	var errs []error

	if f.pool != nil {
		if err := f.pool.CloseAll(); err != nil {
			errs = append(errs, err)
		}
	}

	if f.fsConn != nil {
		if err := f.fsConn.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("factory close errors: %v", errs)
	}
	return nil
}

// Stats retorna estadísticas del factory.
func (f *Factory) Stats() FactoryStats {
	poolStats := f.pool.Stats()
	return FactoryStats{
		Mode:        f.mode.String(),
		ActiveConns: poolStats.TotalActive,
		Connections: poolStats.Connections,
	}
}

// FactoryStats estadísticas del factory.
type FactoryStats struct {
	Mode        string
	ActiveConns int
	Connections map[string]ConnectionStats
}

// MigrateTenant ejecuta migraciones para un tenant específico.
// Retorna ErrNoDBForTenant si el tenant no tiene DB configurada.
func (f *Factory) MigrateTenant(ctx context.Context, slugOrID string) (*MigrationResult, error) {
	if f.migrator == nil {
		return &MigrationResult{}, nil // No hay migrator configurado
	}

	// Resolver tenant
	tenant, err := f.resolveTenant(ctx, slugOrID)
	if err != nil {
		return nil, err
	}

	// Obtener conexión de datos
	dataConn, err := f.getDataConnection(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if dataConn == nil {
		return nil, ErrNoDBForTenant
	}

	// Verificar si la conexión soporta migraciones
	migratable, ok := dataConn.(MigratableConnection)
	if !ok {
		return &MigrationResult{}, nil // Conexión no soporta migraciones (ej: FS)
	}

	// Obtener executor y ejecutar migraciones
	executor := migratable.GetMigrationExecutor()
	result, err := f.migrator.RunWithPgxPool(ctx, executor)
	if err != nil {
		return result, err
	}

	return result, nil
}

// ─── Helpers internos ───

func (f *Factory) resolveTenant(ctx context.Context, slugOrID string) (*repository.Tenant, error) {
	tenants := f.fsConn.Tenants()
	if tenants == nil {
		return nil, fmt.Errorf("factory: fs adapter doesn't support tenants")
	}

	// Intentar por slug
	tenant, err := tenants.GetBySlug(ctx, slugOrID)
	if err == nil {
		return tenant, nil
	}

	// Intentar por ID
	tenant, err = tenants.GetByID(ctx, slugOrID)
	if err != nil {
		return nil, ErrTenantNotFound
	}

	return tenant, nil
}

func (f *Factory) getDataConnection(ctx context.Context, tenant *repository.Tenant) (AdapterConnection, error) {
	// Nota: No hacemos early-return para ModeFSOnly porque un tenant individual
	// puede tener su propia DB configurada aunque no haya GlobalDB ni DefaultTenantDB.

	// Determinar configuración de DB
	var dbCfg *DBConfig

	// Prioridad: tenant-specific > default
	if tenant.Settings.UserDB != nil && tenant.Settings.UserDB.Driver != "" {
		dbCfg = &DBConfig{
			Driver: tenant.Settings.UserDB.Driver,
			DSN:    tenant.Settings.UserDB.DSN,
			Schema: tenant.Settings.UserDB.Schema,
		}
		// Si tiene DSNEnc, descifrar con secretbox
		if dbCfg.DSN == "" && tenant.Settings.UserDB.DSNEnc != "" {
			decrypted, err := decryptDSN(tenant.Settings.UserDB.DSNEnc)
			if err != nil {
				if f.cfg.Logger != nil {
					f.cfg.Logger.Printf("store/v2: failed to decrypt DSN for tenant %s: %v", tenant.Slug, err)
				}
				// Propagar el error en lugar de fallar silenciosamente
				return nil, fmt.Errorf("failed to decrypt DSN for tenant %s: %w", tenant.Slug, err)
			}
			dbCfg.DSN = decrypted
		}
	} else if f.cfg.DefaultTenantDB != nil {
		dbCfg = f.cfg.DefaultTenantDB
	}

	// Sin configuración de DB
	if dbCfg == nil || !dbCfg.Valid() {
		return nil, nil
	}

	// Obtener del pool
	return f.pool.Get(ctx, tenant.Slug, AdapterConfig{
		Name:         dbCfg.Driver,
		DSN:          dbCfg.DSN,
		Schema:       dbCfg.Schema,
		MaxOpenConns: dbCfg.MaxOpenConns,
		MaxIdleConns: dbCfg.MaxIdleConns,
	})
}

func (f *Factory) createTenantConnection(ctx context.Context, slug string, cfg AdapterConfig) (AdapterConnection, error) {
	conn, err := OpenAdapter(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("factory: connect %s for %s: %w", cfg.Name, slug, err)
	}

	// Ejecutar migraciones si están configuradas
	if f.migrator != nil && (cfg.Name == "postgres" || cfg.Name == "pg") {
		// TODO: Implementar migraciones con pgx
		if f.cfg.Logger != nil {
			f.cfg.Logger.Printf("store/v2: would run migrations for %s", slug)
		}
	}

	return conn, nil
}

func (f *Factory) getCache(ctx context.Context, tenant *repository.Tenant) cache.Client {
	// Si no hay configuración de cache, usar memory default
	if tenant.Settings.Cache == nil || !tenant.Settings.Cache.Enabled {
		return cache.NewMemory(tenant.Slug + ":")
	}

	cfg := cache.Config{
		Driver:   tenant.Settings.Cache.Driver,
		Host:     tenant.Settings.Cache.Host,
		Port:     tenant.Settings.Cache.Port,
		Password: tenant.Settings.Cache.Password,
		DB:       tenant.Settings.Cache.DB,
		Prefix:   tenant.Settings.Cache.Prefix,
	}

	c, err := cache.New(cfg)
	if err != nil {
		// Fallback a memory
		return cache.NewMemory(tenant.Slug + ":")
	}

	return c
}

// ─── ConfigAccess Implementation ───

type factoryConfigAccess struct {
	fs AdapterConnection
}

func (c *factoryConfigAccess) Tenants() repository.TenantRepository {
	return c.fs.Tenants()
}

func (c *factoryConfigAccess) Clients(tenantSlug string) repository.ClientRepository {
	return c.fs.Clients()
}

func (c *factoryConfigAccess) Scopes(tenantSlug string) repository.ScopeRepository {
	return c.fs.Scopes()
}

func (c *factoryConfigAccess) Keys() repository.KeyRepository {
	return c.fs.Keys()
}

func (c *factoryConfigAccess) Admins() repository.AdminRepository {
	return c.fs.Admins()
}

func (c *factoryConfigAccess) AdminRefreshTokens() repository.AdminRefreshTokenRepository {
	return c.fs.AdminRefreshTokens()
}

// ─── TenantDataAccess Implementation ───

type tenantAccess struct {
	tenant   *repository.Tenant
	dataConn AdapterConnection // nil si modo FS-only
	fsConn   AdapterConnection
	cache    cache.Client
	mode     OperationalMode
}

func (t *tenantAccess) Slug() string { return t.tenant.Slug }
func (t *tenantAccess) ID() string   { return t.tenant.ID }

func (t *tenantAccess) Settings() *repository.TenantSettings {
	return &t.tenant.Settings
}

func (t *tenantAccess) Driver() string {
	if t.dataConn == nil {
		return "none"
	}
	return t.dataConn.Name()
}

// Data plane repos (desde dataConn)
func (t *tenantAccess) Users() repository.UserRepository {
	if t.dataConn == nil {
		return nil
	}
	return t.dataConn.Users()
}

func (t *tenantAccess) Tokens() repository.TokenRepository {
	if t.dataConn == nil {
		return nil
	}
	return t.dataConn.Tokens()
}

func (t *tenantAccess) MFA() repository.MFARepository {
	if t.dataConn == nil {
		return nil
	}
	return t.dataConn.MFA()
}

func (t *tenantAccess) Consents() repository.ConsentRepository {
	if t.dataConn == nil {
		return nil
	}
	return t.dataConn.Consents()
}

func (t *tenantAccess) RBAC() repository.RBACRepository {
	if t.dataConn == nil {
		return nil
	}
	return t.dataConn.RBAC()
}

func (t *tenantAccess) Schema() repository.SchemaRepository {
	if t.dataConn == nil {
		return nil
	}
	return t.dataConn.Schema()
}

func (t *tenantAccess) EmailTokens() repository.EmailTokenRepository {
	if t.dataConn == nil {
		return nil
	}
	return t.dataConn.EmailTokens()
}

func (t *tenantAccess) Identities() repository.IdentityRepository {
	if t.dataConn == nil {
		return nil
	}
	return t.dataConn.Identities()
}

// Config repos (desde fsConn - control plane)
func (t *tenantAccess) Clients() repository.ClientRepository {
	return t.fsConn.Clients()
}

func (t *tenantAccess) Scopes() repository.ScopeRepository {
	return t.fsConn.Scopes()
}

func (t *tenantAccess) Cache() cache.Client {
	return t.cache
}

func (t *tenantAccess) Mailer() MailSender {
	return nil // TODO: Implementar
}

func (t *tenantAccess) HasDB() bool {
	return t.dataConn != nil
}

func (t *tenantAccess) RequireDB() error {
	if t.dataConn == nil {
		return ErrNoDBForTenant
	}
	return nil
}

func (t *tenantAccess) CacheRepo() repository.CacheRepository {
	return NewCacheRepoWrapper(t.cache)
}

func (t *tenantAccess) InfraStats(ctx context.Context) (*TenantInfraStats, error) {
	stats := &TenantInfraStats{
		TenantSlug:   t.tenant.Slug,
		TenantID:     t.tenant.ID,
		Mode:         t.mode.String(),
		HasDB:        t.dataConn != nil,
		CacheEnabled: t.cache != nil,
		CollectedAt:  time.Now(),
	}

	// Stats de DB si hay conexión
	if t.dataConn != nil {
		healthy := true
		if err := t.dataConn.Ping(ctx); err != nil {
			healthy = false
		}
		stats.DBStats = &DBConnectionStats{
			Driver:  t.dataConn.Name(),
			Healthy: healthy,
		}
	}

	// Stats de cache si está habilitado
	if t.cache != nil {
		cacheStats, err := t.cache.Stats(ctx)
		if err == nil {
			stats.CacheStats = &CacheInfo{
				Driver:     cacheStats.Driver,
				Keys:       cacheStats.Keys,
				UsedMemory: cacheStats.UsedMemory,
				Hits:       cacheStats.Hits,
				Misses:     cacheStats.Misses,
			}
		}
	}

	return stats, nil
}

// decryptDSN descifra un DSN encriptado usando secretbox.
// Retorna error si el formato no es válido o la clave no está configurada.
func decryptDSN(encryptedDSN string) (string, error) {
	if encryptedDSN == "" {
		return "", fmt.Errorf("empty encrypted DSN")
	}
	return secretbox.Decrypt(encryptedDSN)
}
