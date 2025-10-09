package tenantsql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/security/secretbox"
	"github.com/dropDatabas3/hellojohn/internal/store/pg"
)

var (
	ErrNoDBForTenant           = errors.New("no database configured for tenant")
	ErrResolverNotConfigured   = errors.New("tenant resolver not configured")
	ErrControlPlaneUnavailable = errors.New("control plane provider not initialized")
	ErrTenantNotFound          = errors.New("tenant not found")
)

// IsNoDBForTenant indicates whether the error means a tenant lacks DB configuration.
func IsNoDBForTenant(err error) bool { return errors.Is(err, ErrNoDBForTenant) }

// TenantConnection representa la configuración mínima necesaria para abrir un pool.
type TenantConnection struct {
	Driver string
	DSN    string
	Schema string
}

// TenantResolver resuelve la configuración de conexión para un tenant.
type TenantResolver func(ctx context.Context, slug string) (*TenantConnection, error)

// PoolConfig define parámetros del pool por tenant.
type PoolConfig struct {
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// MigrationMetricsFunc callback para reportar métricas de migraciones
type MigrationMetricsFunc func(tenant, result string, duration time.Duration)

// Config permite personalizar la instancia del Manager.
type Config struct {
	Resolve       TenantResolver
	Pool          PoolConfig
	MigrationsDir string
	MetricsFunc   MigrationMetricsFunc // Opcional: callback para métricas
}

// PoolStat es un snapshot del estado de un pool específico.
type PoolStat struct {
	Tenant   string
	Acquired int32
	Idle     int32
	Total    int32
}

// Manager administra pools de base de datos por tenant, aplicando migraciones on-demand
// y evitando creaciones en paralelo mediante singleflight.
type Manager struct {
	resolver      TenantResolver
	poolCfg       PoolConfig
	migrationsDir string
	metricsFunc   MigrationMetricsFunc

	mu     sync.RWMutex
	stores map[string]*pg.Store
	sf     singleflight.Group
}

// New crea un nuevo Manager con la configuración indicada.
func New(cfg Config) (*Manager, error) {
	resolver := cfg.Resolve
	if resolver == nil {
		resolver = defaultResolver
	}
	if resolver == nil {
		return nil, ErrResolverNotConfigured
	}

	poolCfg := cfg.Pool
	if poolCfg.MaxOpenConns <= 0 {
		poolCfg.MaxOpenConns = 15
	}
	if poolCfg.MaxIdleConns <= 0 {
		poolCfg.MaxIdleConns = 3
	}
	if poolCfg.ConnMaxLifetime <= 0 {
		poolCfg.ConnMaxLifetime = 30 * time.Minute
	}

	dir := strings.TrimSpace(cfg.MigrationsDir)
	if dir == "" {
		dir = "migrations/postgres"
	}

	return &Manager{
		resolver:      resolver,
		poolCfg:       cfg.Pool,
		migrationsDir: cfg.MigrationsDir,
		metricsFunc:   cfg.MetricsFunc,
		stores:        make(map[string]*pg.Store),
	}, nil
}

// defaultResolver utiliza el Control Plane activo (cpctx.Provider) para resolver la conexión.
func defaultResolver(ctx context.Context, slug string) (*TenantConnection, error) {
	if cpctx.Provider == nil {
		return nil, ErrControlPlaneUnavailable
	}

	tenant, err := cpctx.Provider.GetTenantBySlug(ctx, slug)
	if err != nil {
		// Unknown tenant in control-plane
		return nil, ErrTenantNotFound
	}
	if tenant == nil {
		return nil, ErrNoDBForTenant
	}
	settings := tenant.Settings
	if settings.UserDB == nil {
		return nil, ErrNoDBForTenant
	}

	driver := strings.ToLower(strings.TrimSpace(settings.UserDB.Driver))
	if driver == "" {
		driver = "pg"
	}
	if driver != "pg" && driver != "postgres" {
		return nil, fmt.Errorf("unsupported tenant driver: %s", settings.UserDB.Driver)
	}
	if strings.TrimSpace(settings.UserDB.DSNEnc) == "" {
		return nil, ErrNoDBForTenant
	}

	dsn, err := secretbox.Decrypt(settings.UserDB.DSNEnc)
	if err != nil {
		return nil, fmt.Errorf("decrypt dsn: %w", err)
	}

	return &TenantConnection{
		Driver: driver,
		DSN:    dsn,
		Schema: settings.UserDB.Schema,
	}, nil
}

// GetPG devuelve (o crea) el store PG asociado al tenant solicitado.
func (m *Manager) GetPG(ctx context.Context, slug string) (*pg.Store, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		slug = "local"
	}

	m.mu.RLock()
	if store, ok := m.stores[slug]; ok {
		m.mu.RUnlock()
		// Reportar cache hit como skipped
		if m.metricsFunc != nil {
			m.metricsFunc(slug, "skipped", 0) // 0 duration for cache hits
		}
		log.Printf(`{"level":"debug","msg":"tenant_migrations_skipped","tenant":"%s"}`, slug)
		return store, nil
	}
	m.mu.RUnlock()

	result, err, shared := m.sf.Do(slug, func() (interface{}, error) {
		return m.createStore(ctx, slug)
	})
	if err != nil {
		return nil, err
	}
	store := result.(*pg.Store)

	if !shared {
		m.mu.Lock()
		m.stores[slug] = store
		m.mu.Unlock()
	}

	return store, nil
}

func (m *Manager) createStore(ctx context.Context, slug string) (*pg.Store, error) {
	conn, err := m.resolver(ctx, slug)
	if err != nil {
		return nil, err
	}
	if conn == nil || strings.TrimSpace(conn.DSN) == "" {
		return nil, ErrNoDBForTenant
	}

	driver := strings.ToLower(strings.TrimSpace(conn.Driver))
	if driver != "" && driver != "pg" && driver != "postgres" {
		return nil, fmt.Errorf("tenant %s: unsupported driver %q", slug, conn.Driver)
	}

	store, err := pg.New(ctx, conn.DSN, struct {
		MaxOpenConns, MaxIdleConns int
		ConnMaxLifetime            string
	}{
		MaxOpenConns:    m.poolCfg.MaxOpenConns,
		MaxIdleConns:    m.poolCfg.MaxIdleConns,
		ConnMaxLifetime: m.poolCfg.ConnMaxLifetime.String(),
	})
	if err != nil {
		return nil, err
	}

	start := time.Now()
	applied, err := RunMigrationsWithLock(ctx, store.Pool(), m.migrationsDir, slug)
	migrationDuration := time.Since(start)

	if err != nil {
		// Reportar migración fallida
		if m.metricsFunc != nil {
			m.metricsFunc(slug, "failed", migrationDuration)
		}
		store.Close()
		return nil, err
	}

	// Reportar resultado de migración
	result := "applied"
	if applied == 0 {
		result = "skipped"
	}
	if m.metricsFunc != nil {
		m.metricsFunc(slug, result, migrationDuration)
	}

	log.Printf(`{"level":"info","msg":"tenant_migrations_applied","tenant":"%s","count":%d}`, slug, applied)

	log.Printf(`{"level":"info","msg":"tenant_pg_pool_ready","tenant":"%s","max_conns":%d}`, slug, m.poolCfg.MaxOpenConns)
	return store, nil
}

// PoolCount retorna el número de pools activos.
func (m *Manager) PoolCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.stores)
}

// Stats devuelve un snapshot con los stats actuales de cada pool.
func (m *Manager) Stats() map[string]PoolStat {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]PoolStat, len(m.stores))
	for slug, store := range m.stores {
		if store == nil {
			continue
		}
		if stat := store.PoolStats(); stat != nil {
			out[slug] = PoolStat{
				Tenant:   slug,
				Acquired: stat.AcquiredConns(),
				Idle:     stat.IdleConns(),
				Total:    stat.TotalConns(),
			}
		}
	}
	return out
}

// Close cierra todos los pools activos.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for slug, store := range m.stores {
		if store != nil {
			store.Close()
		}
		delete(m.stores, slug)
	}
	return nil
}
