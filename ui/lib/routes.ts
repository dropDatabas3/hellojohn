/**
 * Route mapping utility for V1 → V2 migration
 *
 * This file contains the mapping between V1 and V2 API endpoints.
 * Use `mapRoute()` to automatically convert V1 routes to V2 routes.
 *
 * See UI_ROUTES_MIGRATION.md for full migration documentation.
 */

/**
 * API version configuration
 *
 * Set API_VERSION to control which API version to use:
 * - "v1": Use legacy V1 endpoints
 * - "v2": Use new V2 endpoints (default)
 */
export const API_VERSION = (process.env.NEXT_PUBLIC_API_VERSION || "v2") as "v1" | "v2"

/**
 * Routes that remain the same across versions (Standard OAuth2/OIDC)
 */
const STANDARD_ROUTES = [
  "/.well-known/openid-configuration",
  "/.well-known/jwks.json",
  "/oauth2/authorize",
  "/oauth2/token",
  "/oauth2/revoke",
  "/oauth2/introspect",
  "/userinfo",
  "/readyz",
]

/**
 * Route mapping V1 → V2
 *
 * Special cases:
 * - Standard routes (OAuth2/OIDC) remain unchanged
 * - All other routes change from /v1/* to /v2/*
 */
export function mapRoute(route: string): string {
  // 1. Standard routes don't change
  if (STANDARD_ROUTES.includes(route)) {
    return route
  }

  // 2. If already V2 or no version prefix, return as-is
  if (route.startsWith("/v2/") || (!route.startsWith("/v1/") && !route.startsWith("/v2/"))) {
    return route
  }

  // 3. If API_VERSION is v1, return V1 route
  if (API_VERSION === "v1") {
    return route.startsWith("/v1/") ? route : `/v1${route.startsWith("/") ? route.slice(1) : route}`
  }

  // 4. Convert V1 → V2
  if (route.startsWith("/v1/")) {
    return route.replace("/v1/", "/v2/")
  }

  // 5. Add V2 prefix if no version
  return `/v2${route.startsWith("/") ? route : `/${route}`}`
}

/**
 * API Route Constants (V2)
 *
 * Use these constants instead of hardcoded strings for type safety
 * and easier refactoring.
 */
export const API_ROUTES = {
  // ─── Health ───
  READYZ: "/readyz",

  // ─── Auth ───
  AUTH_LOGIN: "/v2/auth/login",
  AUTH_REGISTER: "/v2/auth/register",
  AUTH_REFRESH: "/v2/auth/refresh",
  AUTH_LOGOUT: "/v2/auth/logout",
  AUTH_LOGOUT_ALL: "/v2/auth/logout-all",
  AUTH_PROVIDERS: "/v2/auth/providers",
  AUTH_CONFIG: "/v2/auth/config",
  AUTH_ME: "/v2/me",
  AUTH_PROFILE: "/v2/profile",
  AUTH_COMPLETE_PROFILE: "/v2/auth/complete-profile",

  // ─── Session ───
  SESSION_LOGIN: "/v2/session/login",
  SESSION_LOGOUT: "/v2/session/logout",

  // ─── Admin - Tenants ───
  ADMIN_TENANTS: "/v2/admin/tenants",
  ADMIN_TENANT: (id: string) => `/v2/admin/tenants/${id}`,
  ADMIN_TENANT_SETTINGS: (id: string) => `/v2/admin/tenants/${id}/settings`,
  ADMIN_TENANT_USERS: (id: string) => `/v2/admin/tenants/${id}/users`,
  ADMIN_TENANT_USER: (id: string, userId: string) => `/v2/admin/tenants/${id}/users/${userId}`,
  ADMIN_TENANT_MIGRATE: (id: string) => `/v2/admin/tenants/${id}/migrate`,
  ADMIN_TENANT_SCHEMA_APPLY: (id: string) => `/v2/admin/tenants/${id}/schema/apply`,
  ADMIN_TENANT_KEYS_ROTATE: (id: string) => `/v2/admin/tenants/${id}/keys/rotate`,
  ADMIN_TENANTS_TEST_CONNECTION: "/v2/admin/tenants/test-connection",
  ADMIN_TENANT_INFRA_STATS: (id: string) => `/v2/admin/tenants/${id}/infra-stats`,
  ADMIN_TENANT_CACHE_TEST: (id: string) => `/v2/admin/tenants/${id}/cache/test-connection`,
  ADMIN_TENANT_MAILING_TEST: (id: string) => `/v2/admin/tenants/${id}/mailing/test`,
  ADMIN_TENANT_DB_TEST: (id: string) => `/v2/admin/tenants/${id}/user-store/test-connection`,
  // ISS-11-02: Import endpoints
  ADMIN_TENANT_IMPORT: (id: string) => `/v2/admin/tenants/${id}/import`,
  ADMIN_TENANT_IMPORT_VALIDATE: (id: string) => `/v2/admin/tenants/${id}/import/validate`,
  // ISS-11-03: Export endpoint
  ADMIN_TENANT_EXPORT: (id: string, options?: { 
    clients?: boolean; 
    scopes?: boolean; 
    users?: boolean; 
    roles?: boolean; 
    download?: boolean 
  }) => {
    const params = new URLSearchParams()
    if (options?.clients === false) params.set('clients', 'false')
    if (options?.scopes === false) params.set('scopes', 'false')
    if (options?.users === true) params.set('users', 'true')
    if (options?.roles === true) params.set('roles', 'true')
    if (options?.download === true) params.set('download', 'true')
    const query = params.toString()
    return `/v2/admin/tenants/${id}/export${query ? `?${query}` : ''}`
  },

  // ─── Admin - Clients ───
  ADMIN_CLIENTS: "/v2/admin/clients",
  ADMIN_CLIENT: (clientId: string) => `/v2/admin/clients/${clientId}`,
  ADMIN_CLIENT_REVOKE: (clientId: string) => `/v2/admin/clients/${clientId}/revoke`,

  // ─── Admin - Scopes ───
  ADMIN_SCOPES: "/v2/admin/scopes",
  ADMIN_SCOPE: (name: string) => `/v2/admin/scopes/${name}`,

  // ─── Admin - Claims ───
  ADMIN_CLAIMS: "/v2/admin/claims",
  ADMIN_CLAIMS_CUSTOM: "/v2/admin/claims/custom",
  ADMIN_CLAIMS_CUSTOM_BY_ID: (id: string) => `/v2/admin/claims/custom/${id}`,
  ADMIN_CLAIMS_STANDARD: (name: string) => `/v2/admin/claims/standard/${name}`,
  ADMIN_CLAIMS_SETTINGS: "/v2/admin/claims/settings",
  ADMIN_CLAIMS_MAPPINGS: "/v2/admin/claims/mappings",

  // ─── Admin - Consents (tenant-scoped) ───
  ADMIN_TENANT_CONSENTS: (tenantId: string) => `/v2/admin/tenants/${tenantId}/consents`,
  ADMIN_TENANT_CONSENT: (tenantId: string, consentId: string) => `/v2/admin/tenants/${tenantId}/consents/${consentId}`,

  // ─── Admin - RBAC ───
  ADMIN_RBAC_USER_ROLES: (userId: string) => `/v2/admin/rbac/users/${userId}/roles`,
  ADMIN_RBAC_ROLE_PERMS: (role: string) => `/v2/admin/rbac/roles/${encodeURIComponent(role)}/perms`,
  ADMIN_RBAC_PERMISSIONS: "/v2/admin/rbac/permissions",

  // ─── Admin - Users ───
  ADMIN_USERS_DISABLE: "/v2/admin/users/disable",
  ADMIN_USERS_ENABLE: "/v2/admin/users/enable",
  ADMIN_USERS_RESEND_VERIFICATION: "/v2/admin/users/resend-verification",

  // ─── OAuth2/OIDC ───
  OAUTH_AUTHORIZE: "/oauth2/authorize",
  OAUTH_TOKEN: "/oauth2/token",
  OAUTH_REVOKE: "/oauth2/revoke",
  OAUTH_INTROSPECT: "/oauth2/introspect",
  OAUTH_CONSENT_INFO: "/v2/auth/consent/info",
  OAUTH_CONSENT_ACCEPT: "/v2/auth/consent/accept",
  OIDC_DISCOVERY: "/.well-known/openid-configuration",
  OIDC_DISCOVERY_TENANT: (slug: string) => `/t/${slug}/.well-known/openid-configuration`,
  OIDC_JWKS: "/.well-known/jwks.json",
  OIDC_JWKS_TENANT: (slug: string) => `/.well-known/jwks/${slug}.json`,
  OIDC_USERINFO: "/userinfo",

  // ─── MFA ───
  MFA_TOTP_ENROLL: "/v2/mfa/totp/enroll",
  MFA_TOTP_VERIFY: "/v2/mfa/totp/verify",
  MFA_TOTP_CHALLENGE: "/v2/mfa/totp/challenge",
  MFA_TOTP_DISABLE: "/v2/mfa/totp/disable",
  MFA_RECOVERY_ROTATE: "/v2/mfa/recovery/rotate",

  // ─── Email Flows ───
  EMAIL_VERIFY_START: "/v2/auth/verify-email/start",
  EMAIL_VERIFY_CONFIRM: "/v2/auth/verify-email",
  EMAIL_FORGOT: "/v2/auth/forgot",
  EMAIL_RESET: "/v2/auth/reset",

  // ─── Social Auth ───
  SOCIAL_EXCHANGE: "/v2/auth/social/exchange",
  SOCIAL_RESULT: "/v2/auth/social/result",
  SOCIAL_START: (provider: string) => `/v2/auth/social/${provider}/start`,
  SOCIAL_CALLBACK: (provider: string) => `/v2/auth/social/${provider}/callback`,

  // ─── Providers ───
  PROVIDERS_STATUS: "/v2/providers/status",

  // ─── Admin - Keys ───
  ADMIN_KEYS: "/v2/admin/keys",
  ADMIN_KEY_DETAIL: (kid: string) => `/v2/admin/keys/${kid}`,
  ADMIN_KEYS_ROTATE: "/v2/admin/keys/rotate",
  ADMIN_KEY_REVOKE: (kid: string) => `/v2/admin/keys/${kid}/revoke`,

  // ─── Admin - Cluster ───
  ADMIN_CLUSTER_NODES: "/v2/admin/cluster/nodes",
  ADMIN_CLUSTER_STATS: "/v2/admin/cluster/stats",
  ADMIN_CLUSTER_NODE_REMOVE: (nodeId: string) => `/v2/admin/cluster/nodes/${nodeId}`,

  // ─── TODO: Not yet implemented in V2 ───
  // ADMIN_STATS: "/v2/admin/stats",
  // ADMIN_CONFIG: "/v2/admin/config",
  // CSRF: "/v2/csrf",
} as const

/**
 * Legacy V1 Routes (for reference)
 *
 * Use API_ROUTES constants instead for V2 compatibility.
 * These are kept for backward compatibility during migration.
 */
export const V1_ROUTES = {
  AUTH_LOGIN: "/v1/auth/login",
  AUTH_REGISTER: "/v1/auth/register",
  AUTH_REFRESH: "/v1/auth/refresh",
  ADMIN_TENANTS: "/v2/admin/tenants",
  ADMIN_CLIENTS: "/v2/admin/clients",
  // ... (complete list in UI_ROUTES_MIGRATION.md)
} as const

/**
 * Get the base API URL from environment
 */
export const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"

/**
 * Build full URL for an API endpoint
 */
export function buildApiUrl(route: string): string {
  const mappedRoute = mapRoute(route)
  return `${API_BASE_URL}${mappedRoute}`
}

/**
 * Wrapper for fetch that automatically uses API_BASE_URL and mapRoute
 * 
 * @example
 * const res = await apiFetch('/v2/admin/tenants/123/users', { headers: { Authorization: 'Bearer ...' } })
 * // Calls http://localhost:8080/v2/admin/tenants/123/users
 */
export function apiFetch(route: string, options?: RequestInit): Promise<Response> {
  const url = buildApiUrl(route)
  return fetch(url, {
    ...options,
    credentials: options?.credentials || "include", // Required for cross-site cookies
  })
}

/**
 * Wrapper for fetch with automatic X-Tenant-ID header injection
 * 
 * This is the RECOMMENDED way to call admin endpoints that require tenant context.
 * The backend middleware WithTenantResolution() looks for the tenant in headers.
 * 
 * @param route - API route (e.g., '/v2/admin/users')
 * @param tenantId - Tenant ID to inject in X-Tenant-ID header
 * @param options - Standard fetch options
 * 
 * @example
 * const res = await apiFetchWithTenant('/v2/admin/users', tenantId, {
 *   headers: { Authorization: 'Bearer ...' }
 * })
 * // Automatically adds X-Tenant-ID header
 */
export function apiFetchWithTenant(route: string, tenantId: string, options?: RequestInit): Promise<Response> {
  const url = buildApiUrl(route)
  return fetch(url, {
    ...options,
    credentials: options?.credentials || "include",
    headers: {
      ...options?.headers,
      'X-Tenant-ID': tenantId,
    },
  })
}