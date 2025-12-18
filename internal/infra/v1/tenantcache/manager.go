package tenantcache

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/security/secretbox"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/singleflight"
)

var (
	ErrNoCacheForTenant        = errors.New("no cache configured for tenant")
	ErrResolverNotConfigured   = errors.New("tenant resolver not configured")
	ErrControlPlaneUnavailable = errors.New("control plane provider not initialized")
	ErrTenantNotFound          = errors.New("tenant not found")
)

// TenantConnection representa la configuración mínima necesaria para conectar al cache.
type TenantConnection struct {
	Driver   string
	Host     string
	Port     int
	Password string
	DB       int
	Prefix   string
}

// TenantResolver resuelve la configuración de conexión para un tenant.
type TenantResolver func(ctx context.Context, slug string) (*TenantConnection, error)

// CacheClient define la interfaz mínima que debe cumplir un cliente de cache.
type CacheClient interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Close() error
	Ping(ctx context.Context) error
	Stats(ctx context.Context) (map[string]any, error)
}

// Config permite personalizar la instancia del Manager.
type Config struct {
	Resolve TenantResolver
}

// Manager administra conexiones de cache por tenant.
type Manager struct {
	resolver TenantResolver

	mu      sync.RWMutex
	clients map[string]CacheClient
	sf      singleflight.Group
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

	return &Manager{
		resolver: resolver,
		clients:  make(map[string]CacheClient),
	}, nil
}

// defaultResolver utiliza el Control Plane activo (cpctx.Provider) para resolver la conexión.
func defaultResolver(ctx context.Context, slug string) (*TenantConnection, error) {
	if cpctx.Provider == nil {
		return nil, ErrControlPlaneUnavailable
	}

	tenant, err := cpctx.Provider.GetTenantBySlug(ctx, slug)
	if err != nil {
		return nil, ErrTenantNotFound
	}
	if tenant == nil {
		return nil, ErrNoCacheForTenant
	}
	settings := tenant.Settings
	if settings.Cache == nil {
		return nil, ErrNoCacheForTenant
	}

	driver := strings.ToLower(strings.TrimSpace(settings.Cache.Driver))
	if driver == "" {
		driver = "memory" // Default to memory if not specified
	}

	password := settings.Cache.Password
	if settings.Cache.PassEnc != "" {
		if decrypted, err := secretbox.Decrypt(settings.Cache.PassEnc); err == nil {
			password = decrypted
		}
	}

	return &TenantConnection{
		Driver:   driver,
		Host:     settings.Cache.Host,
		Port:     settings.Cache.Port,
		Password: password,
		DB:       settings.Cache.DB,
		Prefix:   settings.Cache.Prefix,
	}, nil
}

// Get devuelve (o crea) el cliente de cache asociado al tenant solicitado.
func (m *Manager) Get(ctx context.Context, slug string) (CacheClient, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		slug = "local"
	}

	m.mu.RLock()
	if client, ok := m.clients[slug]; ok {
		m.mu.RUnlock()
		return client, nil
	}
	m.mu.RUnlock()

	result, err, shared := m.sf.Do(slug, func() (interface{}, error) {
		return m.createClient(ctx, slug)
	})
	if err != nil {
		return nil, err
	}
	client := result.(CacheClient)

	if !shared {
		m.mu.Lock()
		m.clients[slug] = client
		m.mu.Unlock()
	}

	return client, nil
}

func (m *Manager) createClient(ctx context.Context, slug string) (CacheClient, error) {
	conn, err := m.resolver(ctx, slug)
	if err != nil {
		return nil, err
	}

	switch conn.Driver {
	case "memory":
		return NewMemoryClient(conn.Prefix), nil
	case "redis":
		return NewRedisClient(conn), nil
	default:
		return nil, fmt.Errorf("unsupported cache driver: %s", conn.Driver)
	}
}

// Close cierra todos los clientes activos.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for slug, client := range m.clients {
		if client != nil {
			client.Close()
		}
		delete(m.clients, slug)
	}
	return nil
}

// --- Memory Client Implementation ---

type MemoryClient struct {
	prefix string
	data   map[string]string
	mu     sync.RWMutex
}

func NewMemoryClient(prefix string) *MemoryClient {
	return &MemoryClient{
		prefix: prefix,
		data:   make(map[string]string),
	}
}

func (c *MemoryClient) Get(ctx context.Context, key string) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.data[c.prefix+key]
	if !ok {
		return "", fmt.Errorf("key not found")
	}
	return val, nil
}

func (c *MemoryClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[c.prefix+key] = value
	// Note: TTL not implemented in simple memory client for now
	return nil
}

func (c *MemoryClient) Delete(ctx context.Context, key string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.data, c.prefix+key)
	return nil
}

func (c *MemoryClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = nil
	return nil
}

func (c *MemoryClient) Ping(ctx context.Context) error {
	return nil
}

func (c *MemoryClient) Stats(ctx context.Context) (map[string]any, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]any{
		"driver": "memory",
		"keys":   len(c.data),
	}, nil
}

// --- Redis Client Implementation ---

type RedisClient struct {
	client *redis.Client
	prefix string
}

func NewRedisClient(conn *TenantConnection) *RedisClient {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", conn.Host, conn.Port),
		Password: conn.Password,
		DB:       conn.DB,
	})
	return &RedisClient{
		client: rdb,
		prefix: conn.Prefix,
	}
}

func (c *RedisClient) Get(ctx context.Context, key string) (string, error) {
	return c.client.Get(ctx, c.prefix+key).Result()
}

func (c *RedisClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return c.client.Set(ctx, c.prefix+key, value, ttl).Err()
}

func (c *RedisClient) Delete(ctx context.Context, key string) error {
	return c.client.Del(ctx, c.prefix+key).Err()
}

func (c *RedisClient) Close() error {
	return c.client.Close()
}

func (c *RedisClient) Ping(ctx context.Context) error {
	return c.client.Ping(ctx).Err()
}

func (c *RedisClient) Stats(ctx context.Context) (map[string]any, error) {
	// Info memory
	info, err := c.client.Info(ctx, "memory").Result()
	if err != nil {
		return nil, err
	}
	// Parse used_memory_human
	var usedMemory string
	for _, line := range strings.Split(info, "\r\n") {
		if strings.HasPrefix(line, "used_memory_human:") {
			usedMemory = strings.TrimPrefix(line, "used_memory_human:")
			break
		}
	}

	// DB Size (keys in current DB)
	keys, err := c.client.DBSize(ctx).Result()
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"driver":      "redis",
		"keys":        keys,
		"used_memory": usedMemory,
	}, nil
}
