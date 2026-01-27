// Package migrations embeds SQL migration files.
package migrations

import "embed"

// TenantFS contains the tenant migrations for per-tenant databases.
//
//go:embed tenant/*.sql
var TenantFS embed.FS

// TenantDir is the directory within TenantFS where migrations live.
const TenantDir = "tenant"
