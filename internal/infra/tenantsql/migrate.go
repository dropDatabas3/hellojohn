package tenantsql

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// tenantLockID genera un ID único para pg_advisory_lock basado en el tenant slug
func tenantLockID(slug string) int64 {
	h := sha256.Sum256([]byte("tenant_migration:" + slug))
	// Usar los primeros 8 bytes como int64
	return int64(binary.BigEndian.Uint64(h[:8]))
}

// RunMigrationsWithLock ejecuta migraciones con advisory lock para evitar race conditions
func RunMigrationsWithLock(ctx context.Context, pool *pgxpool.Pool, dir, tenantSlug string) (int, error) {
	lockID := tenantLockID(tenantSlug)

	// Intentar obtener el advisory lock con timeout
	lockCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var acquired bool
	if err := pool.QueryRow(lockCtx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired); err != nil {
		return 0, fmt.Errorf("failed to acquire migration lock for tenant %s: %w", tenantSlug, err)
	}

	if !acquired {
		log.Printf("Migration lock for tenant %s is already held by another process, waiting...", tenantSlug)
		// Si no podemos obtener try_lock, usar lock bloqueante con timeout
		if err := pool.QueryRow(lockCtx, "SELECT pg_advisory_lock($1)", lockID).Scan(&acquired); err != nil {
			return 0, fmt.Errorf("failed to wait for migration lock for tenant %s: %w", tenantSlug, err)
		}
	}

	// Asegurar que liberamos el lock al final
	defer func() {
		if _, err := pool.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", lockID); err != nil {
			log.Printf("Warning: failed to release migration lock for tenant %s: %v", tenantSlug, err)
		}
	}()

	log.Printf("Acquired migration lock for tenant %s (lock_id: %d)", tenantSlug, lockID)
	return runMigrationsUnsafe(ctx, pool, dir)
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
