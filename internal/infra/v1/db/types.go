package db

import (
	"context"
)

// MigrationRunner defines the interface for running database migrations.
type MigrationRunner interface {
	// Run applies pending migrations for a specific tenant.
	// Returns the number of applied migrations and any error.
	Run(ctx context.Context, tenantID string, dir string) (int, error)
}

// SchemaManager defines the interface for dynamic schema operations.
type SchemaManager interface {
	// EnsureIndexes ensures that the required indexes exist for the tenant's schema.
	EnsureIndexes(ctx context.Context, tenantID string, schemaDef map[string]any) error

	// EnsureConstraints ensures that the required constraints exist.
	EnsureConstraints(ctx context.Context, tenantID string, schemaDef map[string]any) error
}

// StatsProvider defines the interface for retrieving database statistics.
type StatsProvider interface {
	// GetStats returns usage statistics for a tenant's database.
	GetStats(ctx context.Context, tenantID string) (map[string]any, error)
}
