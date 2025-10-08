package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/dropDatabas3/hellojohn/internal/store/mongo"
	"github.com/dropDatabas3/hellojohn/internal/store/mysql"
	"github.com/dropDatabas3/hellojohn/internal/store/pg"
)

type Config struct {
	Driver   string
	DSN      string
	Postgres struct {
		MaxOpenConns, MaxIdleConns int
		ConnMaxLifetime            string
	}
	MySQL struct{ DSN string }
	Mongo struct{ URI, Database string }
}

// Open conserva comportamiento actual: devuelve sólo el core.Repository.
func Open(ctx context.Context, cfg Config) (core.Repository, error) {
	d := strings.ToLower(cfg.Driver)
	switch d {
	case "postgres", "pg", "postgresql":
		return pg.New(ctx, cfg.DSN, cfg.Postgres)
	case "mysql":
		dsn := cfg.DSN
		if cfg.MySQL.DSN != "" {
			dsn = cfg.MySQL.DSN
		}
		return mysql.New(ctx, dsn)
	case "mongo", "mongodb":
		return mongo.New(ctx, cfg.Mongo.URI, cfg.Mongo.Database)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
}

// Stores expone el repositorio principal y el complemento de Scopes/Consents.
type Stores struct {
	Repository     core.Repository
	ScopesConsents core.ScopesConsentsRepository // nil si el driver no lo soporta
	Close          func() error
}

// OpenStores abre el repo principal y, si es Postgres, instancia también
// el repositorio de Scopes/Consents usando un pgxpool dedicado.
func OpenStores(ctx context.Context, cfg Config) (*Stores, error) {
	d := strings.ToLower(cfg.Driver)

	switch d {
	case "postgres", "pg", "postgresql":
		// Repo principal (como siempre)
		repo, err := pg.New(ctx, cfg.DSN, cfg.Postgres)
		if err != nil {
			return nil, err
		}

		// Pool dedicado para el repo Scopes/Consents
		pool, err := openPGPool(ctx, cfg.DSN, cfg.Postgres)
		if err != nil {
			return nil, fmt.Errorf("open pgx pool for scopes/consents: %w", err)
		}

		return &Stores{
			Repository:     repo,
			ScopesConsents: pg.NewScopesConsentsPG(pool),
			Close:          func() error { pool.Close(); return nil },
		}, nil

	case "mysql":
		dsn := cfg.DSN
		if cfg.MySQL.DSN != "" {
			dsn = cfg.MySQL.DSN
		}
		repo, err := mysql.New(ctx, dsn)
		if err != nil {
			return nil, err
		}
		return &Stores{Repository: repo, ScopesConsents: nil}, nil

	case "mongo", "mongodb":
		repo, err := mongo.New(ctx, cfg.Mongo.URI, cfg.Mongo.Database)
		if err != nil {
			return nil, err
		}
		return &Stores{Repository: repo, ScopesConsents: nil}, nil

	default:
		return nil, fmt.Errorf("unsupported driver: %s", cfg.Driver)
	}
}

// openPGPool crea un *pgxpool.Pool aplicando parámetros básicos.
// Nota: pgxpool no tiene MaxOpen/MaxIdle; maneja tamaño con MaxConns.
// Usamos MaxOpenConns si viene configurado; si no, dejamos default de pgx.
func openPGPool(ctx context.Context, dsn string, pc struct {
	MaxOpenConns, MaxIdleConns int
	ConnMaxLifetime            string
}) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse pgxpool config: %w", err)
	}

	// Ajustes razonables desde tu config:
	if pc.MaxOpenConns > 0 {
		cfg.MaxConns = int32(pc.MaxOpenConns)
	}
	// Mapear MaxIdleConns → MinConns (pgxpool)
	if pc.MaxIdleConns > 0 {
		cfg.MinConns = int32(pc.MaxIdleConns)
	}

	if pc.ConnMaxLifetime != "" {
		if dur, err := time.ParseDuration(pc.ConnMaxLifetime); err == nil {
			// pgxpool no expone directamente ConnMaxLifetime como en database/sql,
			// pero podemos usar MaxConnLifetime para reciclar conexiones.
			cfg.MaxConnLifetime = dur
			// También configurar MaxConnIdleTime si queremos
			cfg.MaxConnIdleTime = dur
		}
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("new pgxpool: %w", err)
	}
	// Conectar para fallar rápido si hay problema
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgxpool ping: %w", err)
	}
	return pool, nil
}
