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
func RunMigrationsWithLock(ctx context.Context, pool *pgxpool.Pool, dir, tenantIdent string) (int, error) {
	var applied int
	err := WithTenantMigrationLock(ctx, pool, tenantIdent, 30*time.Second, func(c context.Context) error {
		var e error
		applied, e = runMigrationsUnsafe(c, pool, dir)
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
	return runMigrationsUnsafe(ctx, pool, dir)
}

// runMigrationsUnsafe implementación interna sin locks
func runMigrationsUnsafe(ctx context.Context, pool *pgxpool.Pool, dir string) (int, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "migrations/postgres"
	}
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
	var applied int
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return applied, err
		}
		if _, err := pool.Exec(ctx, string(b)); err != nil {
			return applied, fmt.Errorf("exec %s: %w", f, err)
		}
		applied++
	}
	return applied, nil
}
