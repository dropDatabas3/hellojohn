package helpers

import (
	"net/http"
	"strings"
)

// ResolveTenantSlug extracts tenant identifier from request.
// Priority order:
// 1. X-Tenant-Slug header
// 2. X-Tenant-ID header
// 3. tenant query parameter
// 4. tenant_id query parameter
func ResolveTenantSlug(r *http.Request) string {
	// Headers first (most explicit)
	if slug := r.Header.Get("X-Tenant-Slug"); slug != "" {
		return strings.TrimSpace(slug)
	}
	if id := r.Header.Get("X-Tenant-ID"); id != "" {
		return strings.TrimSpace(id)
	}

	// Query params as fallback
	if slug := r.URL.Query().Get("tenant"); slug != "" {
		return strings.TrimSpace(slug)
	}
	if id := r.URL.Query().Get("tenant_id"); id != "" {
		return strings.TrimSpace(id)
	}

	return ""
}

// ResolveTenantSlugFromContext returns the tenant ID from context if it was
// set by middleware. Falls back to request resolution if not found.
func ResolveTenantSlugFromContext(r *http.Request) string {
	// First try from context (middleware may have set it)
	if slug := GetTenantID(r.Context()); slug != "" {
		return slug
	}
	// Fall back to request resolution
	return ResolveTenantSlug(r)
}
