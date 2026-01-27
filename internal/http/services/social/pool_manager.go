package social

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DefaultPoolManager is the shared instance used by services if no other is provided.
var DefaultPoolManager = NewPoolManager()

// PoolManager handles caching of pgxpools by hashed DSN.
type PoolManager struct {
	pools sync.Map // map[string]*pgxpool.Pool (key is hash of DSN)
	mu    sync.Mutex
}

// NewPoolManager creates a new PoolManager.
func NewPoolManager() *PoolManager {
	return &PoolManager{}
}

// GetPool returns a cached pool for the given DSN or creates a new one.
func (m *PoolManager) GetPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	key := hashDSN(dsn)

	// Fast path: check read-only
	if match, ok := m.pools.Load(key); ok {
		return match.(*pgxpool.Pool), nil
	}

	// Slow path: lock to avoid duplicate creation
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check
	if match, ok := m.pools.Load(key); ok {
		return match.(*pgxpool.Pool), nil
	}

	log := logger.From(ctx).With(logger.Component("social.pool_manager"))

	// Create new pool
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}

	// Set reasonable defaults for social provisioning/token ops if not in DSN
	if config.MaxConns == 0 {
		config.MaxConns = 10
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	// Log safe connection info
	safeInfo := fmt.Sprintf("%s@%s:%d/%s", config.ConnConfig.User, config.ConnConfig.Host, config.ConnConfig.Port, config.ConnConfig.Database)
	log.Debug("created new pgxpool",
		logger.String("pool_key", key),
		logger.String("db_target", safeInfo),
	)

	m.pools.Store(key, pool)
	return pool, nil
}

// CloseAll closes all managed pools.
func (m *PoolManager) CloseAll() {
	m.pools.Range(func(key, value any) bool {
		if p, ok := value.(*pgxpool.Pool); ok {
			p.Close()
		}
		return true
	})
}

// hashDSN returns a secure hash of the DSN to use as cache key.
func hashDSN(dsn string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(dsn)))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
