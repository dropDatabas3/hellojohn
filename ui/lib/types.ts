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
  session_lifetime_seconds?: number
  refresh_token_lifetime_seconds?: number
  mfa_enabled?: boolean
  social_login_enabled?: boolean
  smtp?: {
    host: string
    port: number
    username?: string
    password?: string
    fromEmail?: string
    useTLS?: boolean
  }
  userDb?: {
    driver?: string
    dsn?: string
    schema?: string
  }
  security?: {
    passwordMinLength?: number
    mfaRequired?: boolean
  }
  issuerMode?: "global" | "path" | "domain"
  issuerOverride?: string
  user_fields?: UserFieldDefinition[]
  mailing?: MailingSettings
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
  system?: boolean
}

export type Consent = {
  id: string
  userId: string
  clientId: string
  scopes: string[]
  createdAt: string
}

export type User = {
  id: string
  tenant_id: string
  email: string
  email_verified: boolean
  metadata: Record<string, any>
  created_at: string
  updated_at?: string
  disabled_at?: string
  disabled_until?: string
  disabled_reason?: string
  custom_fields?: Record<string, any>
}

export type Role = {
  name: string
  permissions: string[]
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
