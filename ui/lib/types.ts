// Core TypeScript types for HelloJohn Admin

export type Tenant = {
  id: string
  name: string
  display_name?: string
  slug: string
  createdAt: string
  updatedAt: string
  settings: TenantSettings
}

export type TenantSettings = {
  logoUrl?: string
  brandColor?: string
  secondaryColor?: string
  faviconUrl?: string
  sessionLifetimeSeconds?: number
  refreshTokenLifetimeSeconds?: number
  mfaEnabled?: boolean
  socialLoginEnabled?: boolean
  smtp?: {
    host: string
    port: number
    username?: string
    password?: string
    passwordEnc?: string
    fromEmail?: string
    useTLS?: boolean
  }
  userDb?: {
    driver?: string
    dsn?: string
    dsnEnc?: string
    schema?: string
  }
  cache?: {
    enabled: boolean
    driver: string
    host?: string
    port?: number
    password?: string
    passEnc?: string
    db?: number
    prefix?: string
  }
  security?: SecuritySettings
  socialProviders?: {
    googleEnabled?: boolean
    googleClient?: string
    googleSecret?: string
    googleSecretEnc?: string
    githubEnabled?: boolean
    githubClient?: string
    githubSecret?: string
    githubSecretEnc?: string
  }
  consentPolicy?: ConsentPolicy
  issuerMode?: "global" | "path" | "domain"
  issuerOverride?: string
  userFields?: UserFieldDefinition[]
  mailing?: MailingSettings
}

// Security policies settings
export type SecuritySettings = {
  passwordMinLength?: number
  requireUppercase?: boolean
  requireNumbers?: boolean
  requireSpecialChars?: boolean
  mfaRequired?: boolean
  maxLoginAttempts?: number
  lockoutDurationMinutes?: number
}

export type ConsentPolicy = {
  consent_mode: "per_scope" | "single"
  expiration_days?: number | null
  reprompt_days?: number | null
  remember_scope_decisions: boolean
  show_consent_screen: boolean
  allow_skip_consent_for_first_party: boolean
}

export type MailingSettings = {
  templates?: Record<string, EmailTemplate>
}

export type EmailTemplate = {
  subject: string
  body: string
}

export type UserFieldDefinition = {
  name: string
  type: "string" | "number" | "boolean" | "phone" | "country" | "text" | "int" | "date"
  required?: boolean
}

export type ClientInput = {
  name: string
  clientId?: string
  type: "public" | "confidential"
  redirectUris: string[]
  allowedOrigins?: string[]
  providers?: string[]
  scopes?: string[]
  secret?: string
  // Email verification & password reset
  requireEmailVerification?: boolean
  resetPasswordUrl?: string
  verifyEmailUrl?: string
  // OAuth2/OIDC advanced fields
  grantTypes?: string[]
  accessTokenTtl?: number
  refreshTokenTtl?: number
  idTokenTtl?: number
  postLogoutUris?: string[]
  description?: string
}

export type Client = ClientInput & {
  id: string
  tenantId: string
  createdAt: string
  updatedAt: string
}

export type Scope = {
  name: string
  description?: string
  displayName?: string
  claims?: string[]
  dependsOn?: string
  system?: boolean
  createdAt?: string
  updatedAt?: string
}

export type Consent = {
  id: string
  userId: string
  clientId: string
  scopes: string[]
  createdAt: string
  updatedAt?: string
  revokedAt?: string | null
}

export type ConsentListResponse = {
  consents: Consent[]
  total: number
  page: number
  page_size: number
}

export type User = {
  id: string
  tenant_id: string
  email: string
  email_verified: boolean
  name?: string
  given_name?: string
  family_name?: string
  picture?: string
  locale?: string
  source_client_id?: string
  metadata?: Record<string, any>
  custom_fields?: Record<string, any>
  created_at: string
  updated_at?: string
  disabled_at?: string
  disabled_until?: string
  disabled_reason?: string
  disabled_by?: string
}

// Role básico (para asignación de roles a usuarios)
export type Role = {
  name: string
  permissions: string[]
}

// Role completo para CRUD de roles
export type RoleDetail = {
  id: string
  name: string
  description: string
  inherits_from: string | null
  system: boolean
  permissions: string[]
  users_count: number
  created_at: string
  updated_at: string
}

export type RoleListResponse = {
  roles: RoleDetail[]
}

export type CreateRoleRequest = {
  name: string
  description?: string
  inherits_from?: string
  permissions?: string[]
}

export type UpdateRoleRequest = {
  description?: string
  inherits_from?: string
  permissions?: string[]
}

export type PermissionInfo = {
  name: string
  resource: string
  action: string
  description: string
}

export type PermissionsListResponse = {
  permissions: PermissionInfo[]
}

export type ReadyzResponse = {
  status: "ready" | "degraded" | "unavailable"
  version: string
  commit: string
  active_key_id: string
  cluster: {
    mode: string
    role: "leader" | "follower"
    leader_id: string
    peers_configured: number
    peers_connected: number
    raft?: {
      state: string
      term: number
      commit_index: number
      last_applied: number
      last_contact: string
    }
  }
  fs_degraded: boolean
  // Some backends may return plain strings ("ok"/"error"), others objects like { status: "ok" }
  // Use a flexible record to support both.
  components: Record<string, string | { status: string;[k: string]: any }>
}

export type LoginRequest = {
  tenant_id?: string
  client_id?: string
  email: string
  password: string
}

export type LoginResponse = {
  access_token: string
  token_type: string
  expires_in: number
  refresh_token?: string
}

export type MeResponse = {
  sub: string
  email: string
  scopes: string[]
  tenant_id?: string
  client_id?: string
  [key: string]: any
}

export type OIDCDiscovery = {
  issuer: string
  authorization_endpoint: string
  token_endpoint: string
  userinfo_endpoint: string
  jwks_uri: string
  response_types_supported: string[]
  subject_types_supported: string[]
  id_token_signing_alg_values_supported: string[]
  scopes_supported: string[]
  token_endpoint_auth_methods_supported: string[]
  claims_supported: string[]
  code_challenge_methods_supported: string[]
  // Optional endpoints
  revocation_endpoint?: string
  introspection_endpoint?: string
  end_session_endpoint?: string
  registration_endpoint?: string
  // Optional supported features
  grant_types_supported?: string[]
  response_modes_supported?: string[]
  acr_values_supported?: string[]
  display_values_supported?: string[]
  claim_types_supported?: string[]
  service_documentation?: string
  claims_locales_supported?: string[]
  ui_locales_supported?: string[]
  claims_parameter_supported?: boolean
  request_parameter_supported?: boolean
  request_uri_parameter_supported?: boolean
  require_request_uri_registration?: boolean
  op_policy_uri?: string
  op_tos_uri?: string
}

export type ApiError = {
  error: string
  error_description?: string
  status?: number
}

export type AuthConfigResponse = {
  tenant_name: string
  tenant_slug?: string
  client_name?: string
  logo_url?: string
  primary_color?: string
  social_providers?: string[]
  password_enabled: boolean
}

// ─── Sessions ───

export type Session = {
  id: string
  user_id: string
  user_email?: string
  client_id?: string
  ip_address?: string
  user_agent?: string
  device_type?: "desktop" | "mobile" | "tablet" | "unknown"
  browser?: string
  os?: string
  location?: {
    city?: string
    country?: string
    country_code?: string
  }
  created_at: string
  last_activity?: string
  expires_at?: string
  is_current?: boolean
  status?: "active" | "idle" | "expired"
}

export type SessionPolicy = {
  session_lifetime_seconds: number
  inactivity_timeout_seconds: number
  max_concurrent_sessions: number
  notify_on_new_device: boolean
  require_2fa_new_device: boolean
}

export type SessionsResponse = {
  sessions: Session[]
  total_count: number
  page: number
  page_size: number
}

// ─── Tokens (Admin) ───

export type TokenResponse = {
  id: string
  user_id: string
  user_email?: string
  client_id: string
  issued_at: string
  expires_at: string
  revoked_at?: string
  status: "active" | "expired" | "revoked"
}

export type ListTokensResponse = {
  tokens: TokenResponse[]
  total_count: number
  page: number
  page_size: number
}

export type TokenStats = {
  total_active: number
  issued_today: number
  revoked_today: number
  avg_lifetime_hours: number
  by_client: ClientTokenCount[]
}

export type ClientTokenCount = {
  client_id: string
  count: number
}

export type RevokeResponse = {
  revoked_count: number
  message?: string
}

export type TokenFilters = {
  page?: number
  page_size?: number
  user_id?: string
  client_id?: string
  status?: "active" | "expired" | "revoked"
  search?: string
}

// ─── Claims ───

export type ClaimSource = "user_field" | "static" | "expression" | "external"

export type ClaimDefinition = {
  id: string
  name: string
  description?: string
  source: ClaimSource
  // For user_field: field name from user profile
  // For static: literal value
  // For expression: CEL expression
  // For external: webhook URL
  value: string
  // Whether claim is always included or only when specific scopes requested
  always_include?: boolean
  // Scopes that trigger this claim's inclusion
  scopes?: string[]
  // Whether claim is enabled
  enabled: boolean
  // System claims cannot be deleted
  system?: boolean
  // Timestamps
  created_at?: string
  updated_at?: string
}

export type StandardClaim = {
  name: string
  description: string
  enabled: boolean
  scope?: string // Which OIDC scope provides this claim
}

export type ClaimMapping = {
  scope: string
  claims: string[]
}

export type ClaimsConfig = {
  // Standard OIDC claims configuration
  standard_claims: StandardClaim[]
  // Custom claims definitions
  custom_claims: ClaimDefinition[]
  // Scope to claims mappings
  scope_mappings: ClaimMapping[]
  // Global settings
  settings: ClaimsSettings
}

export type ClaimsSettings = {
  // Include claims in access token (vs only ID token)
  include_in_access_token: boolean
  // Use namespaced claims (e.g., https://issuer/claims/custom)
  use_namespaced_claims: boolean
  // Namespace prefix for custom claims
  namespace_prefix?: string
}

// ─── Sessions (Admin) ───

export type SessionResponse = {
  id: string
  user_id: string
  user_email?: string
  status: "active" | "expired" | "revoked" | "idle"
  ip_address?: string
  device_type?: string
  browser?: string
  os?: string
  country?: string
  city?: string
  created_at: string
  last_activity: string
  expires_at: string
  revoked_at?: string
  revoked_reason?: string
}

export type ListSessionsResponse = {
  sessions: SessionResponse[]
  total: number
  page: number
  page_size: number
  total_pages: number
}

export type SessionStats = {
  total_active: number
  total_today: number
  by_device: SessionDeviceCount[]
  by_country: SessionCountryCount[]
}

export type SessionDeviceCount = {
  device_type: string
  count: number
}

export type SessionCountryCount = {
  country: string
  count: number
}

export type RevokeSessionResponse = {
  status: string
}

export type RevokeSessionsResponse = {
  revoked_count: number
}

export type SessionFilters = {
  page?: number
  page_size?: number
  user_id?: string
  device_type?: string
  status?: "active" | "expired" | "revoked"
  search?: string
}

// ─── Import/Export Types ───

export type TenantImportRequest = {
  version: string
  exportedAt?: string
  mode?: "merge" | "replace"
  tenant?: TenantImportInfo
  settings?: TenantSettings
  clients?: ClientImportData[]
  scopes?: ScopeImportData[]
  users?: UserImportData[]
  roles?: RoleImportData[]
}

export type TenantImportInfo = {
  name: string
  slug: string
  display_name?: string
  language?: string
}

export type ClientImportData = {
  client_id: string
  name: string
  description?: string
  client_type: "public" | "confidential"
  redirect_uris?: string[]
  allowed_scopes?: string[]
  token_ttl?: number
  refresh_ttl?: number
}

export type ScopeImportData = {
  name: string
  description?: string
  claims?: string[]
  system?: boolean
}

export type UserImportData = {
  email: string
  username?: string
  email_verified?: boolean
  disabled?: boolean
  roles?: string[]
  metadata?: Record<string, unknown>
  set_password_on_import?: boolean
}

export type RoleImportData = {
  name: string
  description?: string
  inherits_from?: string
  permissions?: string[]
}

export type ImportValidationResult = {
  valid: boolean
  errors?: string[]
  warnings?: string[]
  conflicts?: ConflictInfo[]
  summary: ImportSummary
}

export type ConflictInfo = {
  type: "client" | "scope" | "user" | "role"
  identifier: string
  existing: string
  incoming: string
  action: "skip" | "overwrite" | "merge"
}

export type ImportSummary = {
  tenant_name: string
  settings_included: boolean
  clients_count: number
  scopes_count: number
  users_count: number
  roles_count: number
}

export type ImportResultResponse = {
  success: boolean
  message?: string
  tenant_id?: string
  tenant_slug?: string
  items_imported: ImportCounts
  items_skipped: ImportCounts
  errors?: ImportError[]
  users_needing_password?: string[]
}

export type ImportCounts = {
  settings: number
  clients: number
  scopes: number
  users: number
  roles: number
}

export type ImportError = {
  type: string
  identifier?: string
  error: string
}

export type ExportOptionsRequest = {
  include_settings?: boolean
  include_clients?: boolean
  include_scopes?: boolean
  include_users?: boolean
  include_roles?: boolean
}

export type TenantExportResponse = {
  version: string
  exportedAt: string
  tenant: TenantImportInfo
  settings?: TenantSettings
  clients?: ClientImportData[]
  scopes?: ScopeImportData[]
  users?: UserImportData[]
  roles?: RoleImportData[]
}
