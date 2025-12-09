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
	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/dropDatabas3/hellojohn/internal/security/secretbox"
	"github.com/dropDatabas3/hellojohn/internal/store/pg"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrResolverNotConfigured   = errors.New("tenant resolver not configured")
	ErrControlPlaneUnavailable = errors.New("control plane unavailable")
	ErrTenantNotFound          = errors.New("tenant not found")
	ErrNoDBForTenant           = errors.New("no database configured for tenant")
)

// IsNoDBForTenant checks if the error indicates that no database is configured for the tenant.
func IsNoDBForTenant(err error) bool {
	return errors.Is(err, ErrNoDBForTenant)
}

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
		// Fallback: Check if it's a UUID and try resolving by ID if provider supports it
		if fsProv, ok := controlplane.AsFSProvider(cpctx.Provider); ok {
			if t, errID := fsProv.GetTenantByID(ctx, slug); errID == nil {
				tenant = t
			} else {
				return nil, ErrTenantNotFound
			}
		} else {
			// Unknown tenant in control-plane
			return nil, ErrTenantNotFound
		}
	}
	if tenant == nil {
		return nil, ErrNoDBForTenant
	}
	settings := tenant.Settings
	if settings.UserDB == nil {
		// For legacy or non-configured tenants, if they exist but no DB is explicit,
		// we return ErrNoDBForTenant.
		return nil, ErrNoDBForTenant
	}

	driver := strings.ToLower(strings.TrimSpace(settings.UserDB.Driver))
	if driver == "" {
		driver = "pg"
	}
	if driver != "pg" && driver != "postgres" {
		return nil, fmt.Errorf("unsupported tenant driver: %s", settings.UserDB.Driver)
	}
	if settings.UserDB.DSNEnc != "" {
		// Encrypted DSN Logic
		dsn, err := secretbox.Decrypt(settings.UserDB.DSNEnc)
		if err != nil {
			return nil, fmt.Errorf("decrypt dsn: %w", err)
		}
		return &TenantConnection{
			Driver: driver,
			DSN:    dsn,
			Schema: settings.UserDB.Schema,
		}, nil
	} else if settings.UserDB.DSN != "" {
		// Plain DSN Logic (should be avoided in prod but handled)
		return &TenantConnection{
			Driver: driver,
			DSN:    settings.UserDB.DSN,
			Schema: settings.UserDB.Schema,
		}, nil
	}

	return nil, ErrNoDBForTenant
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
	// Preferir tenantID UUID si está disponible desde el control-plane; fallback al slug
	tenantID := slug
	if cpctx.Provider != nil {
		if t, terr := cpctx.Provider.GetTenantBySlug(ctx, slug); terr == nil && t != nil && strings.TrimSpace(t.ID) != "" {
			tenantID = t.ID
		}
	}
	applied, err := RunMigrationsWithLock(ctx, store.Pool(), m.migrationsDir, tenantID, conn.Schema)
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

// MigrateTenant forces migration execution for a specific tenant.
func (m *Manager) MigrateTenant(ctx context.Context, slug string) (int, error) {
	// 1. Resolve connection
	conn, err := m.resolver(ctx, slug)
	if err != nil {
		return 0, err
	}
	if conn == nil || strings.TrimSpace(conn.DSN) == "" {
		return 0, ErrNoDBForTenant
	}

	// 2. Create transient pool
	pool, err := pgxpool.New(ctx, conn.DSN)
	if err != nil {
		return 0, fmt.Errorf("connect: %w", err)
	}
	defer pool.Close()

	// 3. Run Migrations (Tracked)
	// Use slug as tenantIdent for locking
	return RunMigrationsWithLock(ctx, pool, m.migrationsDir, slug, conn.Schema)
}

// MigrateAll runs migrations for all tenants concurrently.
// It requires a list of tenants (slugs) to iterate over.
func (m *Manager) MigrateAll(ctx context.Context, tenants []string) (map[string]string, error) {
	results := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Worker pool size
	concurrency := 5
	sem := make(chan struct{}, concurrency)

	for _, slug := range tenants {
		wg.Add(1)
		go func(s string) {
			defer wg.Done()
			sem <- struct{}{}        // Acquire
			defer func() { <-sem }() // Release

			count, err := m.MigrateTenant(ctx, s)

			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				results[s] = fmt.Sprintf("error: %v", err)
				log.Printf("Migration failed for %s: %v", s, err)
			} else {
				results[s] = fmt.Sprintf("success: %d applied", count)
				log.Printf("Migration success for %s: %d applied", s, count)
			}
		}(slug)
	}

	wg.Wait()
	return results, nil
}

// HasPendingMigrations returns whether the tenant likely has pending migrations.
// Heuristic MVP: if the critical table from 0001_init (app_user) exists, assume no pending.
// Accepts either tenant slug or UUID; resolves to slug when needed.
func (m *Manager) HasPendingMigrations(ctx context.Context, ident string) (bool, error) {
	// Resolve ident -> slug if needed
	slug := strings.TrimSpace(ident)
	if slug == "" {
		slug = "local"
	}
	if cpctx.Provider != nil {
		if _, err := cpctx.Provider.GetTenantBySlug(ctx, slug); err != nil {
			// Try map UUID->slug
			if tenants, lerr := cpctx.Provider.ListTenants(ctx); lerr == nil {
				for _, t := range tenants {
					if t.ID == ident {
						slug = t.Slug
						break
					}
				}
			}
		}
	}

	// Resolve connection using the configured resolver (no migrations)
	conn, err := m.resolver(ctx, slug)
	if err != nil {
		return false, err
	}
	if conn == nil || strings.TrimSpace(conn.DSN) == "" {
		return false, ErrNoDBForTenant
	}

	// Open a transient, tiny pool; do not cache it in Manager
	store, err := pg.New(ctx, conn.DSN, struct {
		MaxOpenConns, MaxIdleConns int
		ConnMaxLifetime            string
	}{MaxOpenConns: 1, MaxIdleConns: 0, ConnMaxLifetime: "1m"})
	if err != nil {
		return false, err
	}
	defer store.Close()

	// Check existence of a critical table from 0001_init_up.sql
	var exists bool
	if err := store.Pool().QueryRow(ctx, "select to_regclass('app_user') is not null").Scan(&exists); err != nil {
		return false, err
	}
	// If exists -> assume no pending; else -> pending
	return !exists, nil
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

// GetStats returns database statistics for a tenant.
func (m *Manager) GetStats(ctx context.Context, slug string) (map[string]any, error) {
	// Get store (creates connection if needed)
	store, err := m.GetPG(ctx, slug)
	if err != nil {
		return nil, err
	}

	var size string
	var tableCount int

	// Execute queries
	// 1. DB Size
	if err := store.Pool().QueryRow(ctx, "SELECT pg_size_pretty(pg_database_size(current_database()))").Scan(&size); err != nil {
		return nil, fmt.Errorf("failed to get db size: %w", err)
	}

	// 2. Table Count (public schema)
	if err := store.Pool().QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public'").Scan(&tableCount); err != nil {
		return nil, fmt.Errorf("failed to get table count: %w", err)
	}

	return map[string]any{
		"size":        size,
		"table_count": tableCount,
	}, nil
}
