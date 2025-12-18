package tenantsql

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hash/fnv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// lockIDForTenant genera un ID determinístico (64-bit) a partir del tenantID (UUID preferido)
func lockIDForTenant(tenantID string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte("hj:migrate:"))
	_, _ = h.Write([]byte(strings.TrimSpace(tenantID)))
	return int64(h.Sum64())
}

// WithTenantMigrationLock adquiere un advisory lock por tenant en la MISMA conexión y lo libera al salir.
// Ejecuta fn dentro de la sección crítica. Usa tenantID (UUID recomendado) para evitar colisiones por slug.
func WithTenantMigrationLock(ctx context.Context, pool *pgxpool.Pool, tenantID string, wait time.Duration, fn func(ctx context.Context) error) error {
	lockID := lockIDForTenant(tenantID)

	// 1) tomar una conexión dedicada
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()

	// 2) establecer timeout de espera
	if wait <= 0 {
		wait = 30 * time.Second
	}
	lctx, cancel := context.WithTimeout(ctx, wait)
	defer cancel()

	// 3) intentar non-blocking primero
	var got bool
	if err := conn.QueryRow(lctx, "select pg_try_advisory_lock($1)", lockID).Scan(&got); err != nil {
		return err
	}
	if !got {
		log.Printf("Migration lock is already held, waiting... (lock_id: %d)", lockID)
		// 4) bloqueo hasta liberar o timeout
		if _, err := conn.Exec(lctx, "select pg_advisory_lock($1)", lockID); err != nil {
			return err
		}
	}

	// 5) siempre liberar al final en la MISMA conexión
	defer func() {
		if _, err := conn.Exec(context.Background(), "select pg_advisory_unlock($1)", lockID); err != nil {
			log.Printf("Warning: failed to release migration lock (lock_id: %d): %v", lockID, err)
		} else {
			log.Printf("Released migration lock (lock_id: %d)", lockID)
		}
	}()

	log.Printf("Acquired migration lock (lock_id: %d)", lockID)
	// 6) ejecutar lo crítico
	return fn(ctx)
}

// RunMigrationsWithLock (compat) ahora usa WithTenantMigrationLock; el parámetro identifica al tenant
func RunMigrationsWithLock(ctx context.Context, pool *pgxpool.Pool, dir, tenantIdent, schema string) (int, error) {
	var applied int
	err := WithTenantMigrationLock(ctx, pool, tenantIdent, 30*time.Second, func(c context.Context) error {
		var e error
		applied, e = runMigrationsTracked(c, pool, dir, schema)
		return e
	})
	if err != nil {
		return 0, err
	}
	return applied, nil
}

// RunMigrations ejecuta todos los *_up.sql del dir (ordenados lexicográficamente)
// y devuelve cuántos scripts se aplicaron.
// DEPRECATED: Use RunMigrationsWithLock para evitar race conditions
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, dir string) (int, error) {
	return runMigrationsTracked(ctx, pool, dir, "")
}

// runMigrationsTracked implementation with schema_migrations table
func runMigrationsTracked(ctx context.Context, pool *pgxpool.Pool, dir, schema string) (int, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "migrations/postgres"
	}

	// 0. Set search_path if schema is provided
	if schema != "" {
		// We use a transaction-less exec here because search_path is session-local,
		// but pgxpool might reuse connections. However, since we are inside a lock
		// and likely using the same connection or will use a tx later, we should be careful.
		// Actually, the best way is to ensure every transaction sets the search path,
		// OR set it for the session.
		// Given we are about to run migrations in transactions, let's just ensure
		// the schema exists and we use it.
		// BUT: schema creation should probably happen before this if it doesn't exist.
		// Assuming schema exists (created by provisioner or manually).

		// Let's try to create schema if not exists just in case?
		// No, let's stick to setting search_path.
		_, err := pool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pgIdentifier(schema)))
		if err != nil {
			return 0, fmt.Errorf("failed to ensure schema %s: %w", schema, err)
		}
	}

	// 1. Ensure schema_migrations table exists
	// We need to make sure this table is created in the correct schema.
	// We will wrap everything in a function that sets search_path for the commands.

	ensureTableQuery := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)
	`
	if schema != "" {
		// Set search_path for the session/connection used by pool.Exec is tricky with pool.
		// Better to be explicit or use a transaction for setup.
		// Let's use a setup transaction.
		tx, err := pool.Begin(ctx)
		if err != nil {
			return 0, err
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, fmt.Sprintf("SET search_path TO %s", pgIdentifier(schema))); err != nil {
			return 0, fmt.Errorf("set search_path: %w", err)
		}
		if _, err := tx.Exec(ctx, ensureTableQuery); err != nil {
			return 0, fmt.Errorf("create schema_migrations: %w", err)
		}
		if err := tx.Commit(ctx); err != nil {
			return 0, err
		}
	} else {
		if _, err := pool.Exec(ctx, ensureTableQuery); err != nil {
			return 0, fmt.Errorf("failed to ensure schema_migrations: %w", err)
		}
	}

	// 2. Get applied versions
	var rows pgx.Rows
	var err error

	queryVersions := "SELECT version FROM schema_migrations"
	if schema != "" {
		// We need to ensure we read from the correct schema.
		// Since we can't easily guarantee the same connection as before without a long-lived tx,
		// we'll just be explicit or use a tx.
		// Using a tx is safer.
		tx, txErr := pool.Begin(ctx)
		if txErr != nil {
			return 0, txErr
		}
		defer tx.Rollback(ctx)

		if _, err := tx.Exec(ctx, fmt.Sprintf("SET search_path TO %s", pgIdentifier(schema))); err != nil {
			return 0, fmt.Errorf("set search_path: %w", err)
		}
		rows, err = tx.Query(ctx, queryVersions)
		if err != nil {
			return 0, fmt.Errorf("failed to query applied migrations: %w", err)
		}
		// We must load all rows before commit/rollback
		// Actually, we can just defer rollback and if we commit later it's fine, but here we just read.
		// Let's read into memory.
	} else {
		rows, err = pool.Query(ctx, queryVersions)
		if err != nil {
			return 0, fmt.Errorf("failed to query applied migrations: %w", err)
		}
	}

	appliedVersions := make(map[string]bool)
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return 0, err
		}
		appliedVersions[v] = true
	}
	rows.Close()

	// 3. Read and sort migration files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, err
	}
	var files []string
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(strings.ToLower(e.Name()), "_up.sql") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)

	// 4. Apply new migrations
	var appliedCount int
	for _, f := range files {
		filename := filepath.Base(f)
		// Version is the filename without extension, or just the filename.
		// Usually migrations are like "0001_init_up.sql".
		// We use the full filename as version ID for simplicity.
		version := filename

		if appliedVersions[version] {
			continue
		}

		log.Printf("Applying migration: %s (schema: %s)", version, schema)
		b, err := os.ReadFile(f)
		if err != nil {
			return appliedCount, err
		}

		// Execute in transaction
		tx, err := pool.Begin(ctx)
		if err != nil {
			return appliedCount, fmt.Errorf("begin tx: %w", err)
		}

		if schema != "" {
			if _, err := tx.Exec(ctx, fmt.Sprintf("SET search_path TO %s", pgIdentifier(schema))); err != nil {
				tx.Rollback(ctx)
				return appliedCount, fmt.Errorf("set search_path: %w", err)
			}
		}

		if _, err := tx.Exec(ctx, string(b)); err != nil {
			tx.Rollback(ctx)
			return appliedCount, fmt.Errorf("exec %s: %w", f, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
			tx.Rollback(ctx)
			return appliedCount, fmt.Errorf("record version %s: %w", version, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return appliedCount, fmt.Errorf("commit tx: %w", err)
		}

		appliedCount++
	}
	return appliedCount, nil
}
