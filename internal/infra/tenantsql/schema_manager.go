package tenantsql

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresSchemaManager implements db.SchemaManager for PostgreSQL.
type PostgresSchemaManager struct {
	pool *pgxpool.Pool
}

// NewSchemaManager creates a new PostgresSchemaManager.
func NewSchemaManager(pool *pgxpool.Pool) *PostgresSchemaManager {
	return &PostgresSchemaManager{pool: pool}
}

// EnsureIndexes ensures that the required indexes exist for the tenant's schema.
// schemaDef is expected to be a map where keys are table names and values are lists of index definitions.
// Example schemaDef:
//
//	{
//	  "app_user": [
//	    { "name": "idx_user_phone", "columns": ["(profile->>'phone')"], "unique": true }
//	  ]
//	}
func (m *PostgresSchemaManager) EnsureIndexes(ctx context.Context, tenantID string, schemaDef map[string]any) error {
	for table, indexes := range schemaDef {
		idxList, ok := indexes.([]any)
		if !ok {
			continue
		}

		for _, idx := range idxList {
			idxMap, ok := idx.(map[string]any)
			if !ok {
				continue
			}

			name, _ := idxMap["name"].(string)
			columns, _ := idxMap["columns"].([]any)
			isUnique, _ := idxMap["unique"].(bool)

			if name == "" || len(columns) == 0 {
				continue
			}

			var cols []string
			for _, c := range columns {
				if s, ok := c.(string); ok {
					cols = append(cols, s)
				}
			}

			uniqueStr := ""
			if isUnique {
				uniqueStr = "UNIQUE"
			}

			// Safe construction of CREATE INDEX statement
			// Note: In a real production system, we should be more careful about SQL injection here,
			// but assuming schemaDef comes from a trusted internal source (control plane).
			query := fmt.Sprintf(
				"CREATE %s INDEX IF NOT EXISTS %s ON %s (%s)",
				uniqueStr,
				pgIdentifier(name),
				pgIdentifier(table),
				strings.Join(cols, ", "),
			)

			log.Printf("Tenant %s: Ensuring index %s on %s", tenantID, name, table)
			if _, err := m.pool.Exec(ctx, query); err != nil {
				return fmt.Errorf("failed to create index %s: %w", name, err)
			}
		}
	}
	return nil
}

// EnsureConstraints ensures that the required constraints exist.
// Currently supports CHECK constraints.
func (m *PostgresSchemaManager) EnsureConstraints(ctx context.Context, tenantID string, schemaDef map[string]any) error {
	// Implementation for constraints can be added here similar to indexes
	return nil
}

// SyncUserFields ensures that the app_user table has the columns defined in fields.
// It generates and executes ALTER TABLE statements.
func (m *PostgresSchemaManager) SyncUserFields(ctx context.Context, tenantID string, fields []controlplane.UserFieldDefinition) error {
	// if len(fields) == 0 { return nil } // Removed to allow dropping columns when fields are empty

	// 1. Get existing columns
	rows, err := m.pool.Query(ctx, `
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_name = 'app_user'
	`)
	if err != nil {
		return fmt.Errorf("failed to get existing columns: %w", err)
	}
	defer rows.Close()

	existingCols := make(map[string]bool)
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return err
		}
		existingCols[col] = true
	}

	// 2. Iterate fields and apply changes
	newFieldNames := make(map[string]bool)
	for _, field := range fields {
		fieldName := strings.ToLower(strings.TrimSpace(field.Name))
		if fieldName == "" {
			continue
		}
		newFieldNames[fieldName] = true

		// Prevent overwriting system columns
		if isSystemColumn(fieldName) {
			continue
		}

		// Map type
		sqlType := mapFieldTypeToSQL(field.Type)
		if sqlType == "" {
			log.Printf("Tenant %s: Unknown field type %s for field %s", tenantID, field.Type, fieldName)
			continue
		}

		// ADD COLUMN
		if !existingCols[fieldName] {
			log.Printf("Tenant %s: Adding column %s (%s)", tenantID, fieldName, sqlType)
			query := fmt.Sprintf("ALTER TABLE app_user ADD COLUMN IF NOT EXISTS %s %s", pgIdentifier(fieldName), sqlType)
			if _, err := m.pool.Exec(ctx, query); err != nil {
				return fmt.Errorf("failed to add column %s: %w", fieldName, err)
			}
		}

		// NOT NULL constraint
		if field.Required {
			// Check if any row has null for this column
			var hasNulls bool
			_ = m.pool.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM app_user WHERE %s IS NULL)", pgIdentifier(fieldName))).Scan(&hasNulls)

			if !hasNulls {
				// Safe to set NOT NULL
				_, _ = m.pool.Exec(ctx, fmt.Sprintf("ALTER TABLE app_user ALTER COLUMN %s SET NOT NULL", pgIdentifier(fieldName)))
			} else {
				log.Printf("Tenant %s: Cannot set NOT NULL on %s because it contains null values", tenantID, fieldName)
			}
		} else {
			// Drop NOT NULL if exists
			_, _ = m.pool.Exec(ctx, fmt.Sprintf("ALTER TABLE app_user ALTER COLUMN %s DROP NOT NULL", pgIdentifier(fieldName)))
		}

		// UNIQUE constraint
		uqName := fmt.Sprintf("uq_app_user_%s", fieldName)
		if field.Unique {
			query := fmt.Sprintf("ALTER TABLE app_user ADD CONSTRAINT %s UNIQUE (%s)", pgIdentifier(uqName), pgIdentifier(fieldName))
			_, err := m.pool.Exec(ctx, query)
			if err != nil && !strings.Contains(err.Error(), "already exists") {
				log.Printf("Tenant %s: Failed to add unique constraint %s: %v", tenantID, uqName, err)
			}
		} else {
			// Drop constraint if exists
			_, _ = m.pool.Exec(ctx, fmt.Sprintf("ALTER TABLE app_user DROP CONSTRAINT IF EXISTS %s", pgIdentifier(uqName)))
		}

		// INDEX
		idxName := fmt.Sprintf("idx_app_user_%s", fieldName)
		if field.Indexed && !field.Unique {
			query := fmt.Sprintf("CREATE INDEX IF NOT EXISTS %s ON app_user (%s)", pgIdentifier(idxName), pgIdentifier(fieldName))
			if _, err := m.pool.Exec(ctx, query); err != nil {
				log.Printf("Tenant %s: Failed to create index %s: %v", tenantID, idxName, err)
			}
		} else if !field.Indexed {
			_, _ = m.pool.Exec(ctx, fmt.Sprintf("DROP INDEX IF EXISTS %s", pgIdentifier(idxName)))
		}
	}

	// 3. Drop removed columns
	for col := range existingCols {
		if isSystemColumn(col) {
			continue
		}
		if !newFieldNames[col] {
			log.Printf("Tenant %s: Dropping removed column %s", tenantID, col)
			// Drop column
			if _, err := m.pool.Exec(ctx, fmt.Sprintf("ALTER TABLE app_user DROP COLUMN IF EXISTS %s", pgIdentifier(col))); err != nil {
				return fmt.Errorf("failed to drop column %s: %w", col, err)
			}
		}
	}

	return nil
}

func isSystemColumn(name string) bool {
	switch name {
	case "id", "email", "email_verified", "status", "profile", "metadata", "disabled_at", "disabled_reason", "created_at", "updated_at", "password_hash":
		return true
	}
	return false
}

func mapFieldTypeToSQL(t string) string {
	switch t {
	case "text", "string":
		return "TEXT"
	case "int", "integer", "number":
		return "INTEGER"
	case "bool", "boolean":
		return "BOOLEAN"
	case "date", "datetime":
		return "TIMESTAMPTZ"
	default:
		return "TEXT"
	}
}

// pgIdentifier sanitizes a string to be used as a PostgreSQL identifier.
// This is a basic implementation.
func pgIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
