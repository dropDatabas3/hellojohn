// ============================================================================
// CLIENT WIZARD - TYPES
// Tipos compartidos para el flujo de creacion/edicion de clients OAuth2
// ============================================================================

export type ClientType = "public" | "confidential"

export type AppSubType = "spa" | "mobile" | "api_server" | "m2m"

export interface ClientFormState {
    name: string
    clientId: string
    type: ClientType
    subType: AppSubType
    description: string
    redirectUris: string[]
    allowedOrigins: string[]
    postLogoutUris: string[]
    scopes: string[]
    providers: string[]
    grantTypes: string[]
    accessTokenTTL: number
    refreshTokenTTL: number
    idTokenTTL: number
    requireEmailVerification: boolean
    resetPasswordUrl: string
    verifyEmailUrl: string
    frontChannelLogoutUrl: string
    backChannelLogoutUrl: string
}

export interface ClientRow {
    id: string
    client_id: string
    name: string
    type: "public" | "confidential"
    description?: string
    redirect_uris: string[]
    allowed_origins?: string[]
    post_logout_uris?: string[]
    providers?: string[]
    scopes?: string[]
    grant_types?: string[]
    secret?: string
    secret_hash?: string
    access_token_ttl?: number
    refresh_token_ttl?: number
    id_token_ttl?: number
    require_email_verification?: boolean
    reset_password_url?: string
    verify_email_url?: string
    front_channel_logout_url?: string
    back_channel_logout_url?: string
    created_at?: string
    updated_at?: string
}

export interface WizardStep {
    id: number
    label: string
    icon: string
    description: string
}
