// ============================================================================
// CLIENT WIZARD - CONSTANTS
// Constantes, presets y configuracion para el flujo de clients OAuth2
// ============================================================================

import type { ClientFormState, ClientType, AppSubType, WizardStep } from "./types"

// ----------------------------------------------------------------------------
// WIZARD STEPS
// ----------------------------------------------------------------------------

export const WIZARD_STEPS: WizardStep[] = [
    { id: 1, label: "Tipo", icon: "LayoutGrid", description: "Tipo de aplicacion" },
    { id: 2, label: "Info", icon: "FileText", description: "Informacion basica" },
    { id: 3, label: "URIs", icon: "Link2", description: "URLs de redireccion" },
    { id: 4, label: "Resumen", icon: "CheckCircle2", description: "Revisar y crear" },
]

// ----------------------------------------------------------------------------
// APP SUB-TYPES (presets por tipo de aplicacion)
// ----------------------------------------------------------------------------

export interface AppSubTypeConfig {
    type: ClientType
    label: string
    description: string
    icon: string
    features: string[]
    defaultGrantTypes: string[]
    defaultProviders: string[]
    suggestedRedirectUris: string[]
    suggestedOrigins: string[]
    tagLabel: string | null
    // UX Metadata - Fase 0
    hasInteractiveUsers: boolean      // Tiene usuarios que hacen login
    requiresRedirectUris: boolean     // Necesita redirect URIs
    scopeType: "user" | "api" | "custom"  // Tipo de scopes a mostrar
    showProviders: boolean            // Mostrar providers de auth
    relevantGrantTypes: string[]      // Grants relevantes para este tipo
    wizardSteps: number[]             // IDs de pasos del wizard
    infoMessage: string               // Mensaje informativo contextual
}

export const APP_SUB_TYPES: Record<AppSubType, AppSubTypeConfig> = {
    spa: {
        type: "public",
        label: "Single Page App",
        description: "React, Vue, Angular, Svelte",
        icon: "Globe",
        features: [
            "Corre en el navegador",
            "Sin secreto (usa PKCE)",
            "Redirect URI requerida",
        ],
        defaultGrantTypes: ["authorization_code", "refresh_token"],
        defaultProviders: ["password", "google"],
        suggestedRedirectUris: [
            "http://localhost:3000/callback",
            "http://localhost:5173/callback",
        ],
        suggestedOrigins: [
            "http://localhost:3000",
            "http://localhost:5173",
        ],
        tagLabel: "Mas comun",
        // UX Metadata
        hasInteractiveUsers: true,
        requiresRedirectUris: true,
        scopeType: "user",
        showProviders: true,
        relevantGrantTypes: ["authorization_code", "refresh_token"],
        wizardSteps: [1, 2, 3, 4],
        infoMessage: "Tu app corre en el navegador. Usa PKCE automaticamente para mayor seguridad.",
    },
    mobile: {
        type: "public",
        label: "App Movil / Nativa",
        description: "React Native, Flutter, Swift, Kotlin",
        icon: "Smartphone",
        features: [
            "App nativa o hibrida",
            "Sin secreto (usa PKCE)",
            "Deep link redirect",
        ],
        defaultGrantTypes: ["authorization_code", "refresh_token"],
        defaultProviders: ["password", "google"],
        suggestedRedirectUris: ["myapp://callback"],
        suggestedOrigins: [],
        tagLabel: null,
        // UX Metadata
        hasInteractiveUsers: true,
        requiresRedirectUris: true,
        scopeType: "user",
        showProviders: true,
        relevantGrantTypes: ["authorization_code", "refresh_token"],
        wizardSteps: [1, 2, 3, 4],
        infoMessage: "App nativa o hibrida. Configura deep links para el callback de autenticacion.",
    },
    api_server: {
        type: "confidential",
        label: "API / Web Server",
        description: "Express, NestJS, Django, Rails, Go",
        icon: "Server",
        features: [
            "Corre en servidor seguro",
            "Tiene client_secret",
            "Auth code + PKCE soportado",
        ],
        defaultGrantTypes: ["authorization_code", "refresh_token"],
        defaultProviders: ["password"],
        suggestedRedirectUris: ["http://localhost:8080/callback"],
        suggestedOrigins: [],
        tagLabel: null,
        // UX Metadata
        hasInteractiveUsers: true,
        requiresRedirectUris: true,
        scopeType: "user",
        showProviders: true,
        relevantGrantTypes: ["authorization_code", "refresh_token", "client_credentials"],
        wizardSteps: [1, 2, 3, 4],
        infoMessage: "Backend seguro con client_secret. Puede autenticar usuarios o actuar como servicio.",
    },
    m2m: {
        type: "confidential",
        label: "Machine-to-Machine",
        description: "Cron jobs, microservicios, workers",
        icon: "Cpu",
        features: [
            "Sin usuario interactivo",
            "Solo client_credentials",
            "Comunicacion entre servicios",
        ],
        defaultGrantTypes: ["client_credentials"],
        defaultProviders: [],
        suggestedRedirectUris: [],
        suggestedOrigins: [],
        tagLabel: null,
        // UX Metadata
        hasInteractiveUsers: false,
        requiresRedirectUris: false,
        scopeType: "api",
        showProviders: false,
        relevantGrantTypes: ["client_credentials"],
        wizardSteps: [1, 2, 4],  // SALTA Step 3 (URIs)
        infoMessage: "Comunicacion servidor-a-servidor sin usuarios. El secret tiene acceso total - protegelo bien.",
    },
}

// ----------------------------------------------------------------------------
// GRANT TYPES
// ----------------------------------------------------------------------------

export interface GrantTypeConfig {
    id: string
    label: string
    description: string
    recommended?: boolean
    deprecated?: boolean
    confidentialOnly?: boolean
}

export const GRANT_TYPES: GrantTypeConfig[] = [
    {
        id: "authorization_code",
        label: "Authorization Code",
        description: "Flujo OAuth2 estandar con PKCE",
        recommended: true,
    },
    {
        id: "refresh_token",
        label: "Refresh Token",
        description: "Permitir renovacion de tokens",
        recommended: true,
    },
    {
        id: "client_credentials",
        label: "Client Credentials",
        description: "Autenticacion M2M (solo backend)",
        confidentialOnly: true,
    },
    {
        id: "implicit",
        label: "Implicit",
        description: "Flujo legacy - NO RECOMENDADO",
        deprecated: true,
    },
]

// ----------------------------------------------------------------------------
// SCOPES
// ----------------------------------------------------------------------------

export const DEFAULT_SCOPES = ["openid", "profile", "email", "offline_access"]

// Scopes de usuario (OIDC) - para SPA, Mobile, API Server
export const PREDEFINED_SCOPES = [
    { id: "openid", label: "openid", description: "Identidad basica del usuario" },
    { id: "profile", label: "profile", description: "Nombre, avatar y datos del perfil" },
    { id: "email", label: "email", description: "Direccion de correo electronico" },
    { id: "offline_access", label: "offline_access", description: "Permite renovar tokens (refresh)" },
]

// Scopes de API/Recursos - para M2M
export const API_SCOPES_EXAMPLES = [
    { id: "read:users", label: "read:users", description: "Leer datos de usuarios" },
    { id: "write:users", label: "write:users", description: "Crear/modificar usuarios" },
    { id: "read:data", label: "read:data", description: "Leer datos de la API" },
    { id: "write:data", label: "write:data", description: "Escribir datos en la API" },
    { id: "admin", label: "admin", description: "Acceso administrativo completo" },
]

export const M2M_SCOPE_PLACEHOLDER = "Ej: read:users, write:data, api:admin"

// ----------------------------------------------------------------------------
// PROVIDERS
// ----------------------------------------------------------------------------

export interface ProviderConfig {
    id: string
    label: string
    icon: string
    enabled: boolean
    comingSoon?: boolean
}

export const AVAILABLE_PROVIDERS: ProviderConfig[] = [
    { id: "password", label: "Email + Password", icon: "🔑", enabled: true },
    { id: "google", label: "Google", icon: "G", enabled: true },
    { id: "github", label: "GitHub", icon: "🐙", enabled: false, comingSoon: true },
    { id: "apple", label: "Apple", icon: "🍎", enabled: false, comingSoon: true },
]

// ----------------------------------------------------------------------------
// TOKEN TTL OPTIONS
// ----------------------------------------------------------------------------

export interface TokenTTLOption {
    value: number
    label: string
    description: string
}

export const TOKEN_TTL_OPTIONS = {
    access: [
        { value: 5, label: "5 min", description: "Alta seguridad" },
        { value: 15, label: "15 min", description: "Recomendado" },
        { value: 30, label: "30 min", description: "Balance" },
        { value: 60, label: "1 hora", description: "Conveniencia" },
    ] as TokenTTLOption[],
    refresh: [
        { value: 10080, label: "7 dias", description: "Alta seguridad" },
        { value: 20160, label: "14 dias", description: "Balance" },
        { value: 43200, label: "30 dias", description: "Recomendado" },
        { value: 129600, label: "90 dias", description: "Larga duracion" },
    ] as TokenTTLOption[],
    id: [
        { value: 15, label: "15 min", description: "Alta seguridad" },
        { value: 60, label: "1 hora", description: "Recomendado" },
        { value: 480, label: "8 horas", description: "Sesion larga" },
        { value: 1440, label: "24 horas", description: "Maxima conveniencia" },
    ] as TokenTTLOption[],
}

// ----------------------------------------------------------------------------
// DEFAULT FORM STATE
// ----------------------------------------------------------------------------

export const DEFAULT_FORM: ClientFormState = {
    name: "",
    clientId: "",
    type: "public",
    subType: "spa",
    description: "",
    redirectUris: [],
    allowedOrigins: [],
    postLogoutUris: [],
    scopes: ["openid", "profile", "email"],
    providers: ["password"],
    grantTypes: ["authorization_code", "refresh_token"],
    accessTokenTTL: 15,
    refreshTokenTTL: 43200,
    idTokenTTL: 60,
    requireEmailVerification: false,
    resetPasswordUrl: "",
    verifyEmailUrl: "",
    frontChannelLogoutUrl: "",
    backChannelLogoutUrl: "",
}

// ----------------------------------------------------------------------------
// HELPER FUNCTIONS - Fase 0
// ----------------------------------------------------------------------------

/**
 * Retorna los pasos del wizard filtrados segun el subtipo de aplicacion.
 * M2M salta el Step 3 (URIs) porque no necesita redirect URIs.
 */
export function getStepsForSubType(subType: AppSubType): WizardStep[] {
    const config = APP_SUB_TYPES[subType]
    return WIZARD_STEPS.filter(step => config.wizardSteps.includes(step.id))
}

/**
 * Retorna los grant types relevantes para un subtipo de aplicacion.
 * Filtra GRANT_TYPES segun los relevantGrantTypes del config.
 */
export function getGrantTypesForSubType(subType: AppSubType): GrantTypeConfig[] {
    const config = APP_SUB_TYPES[subType]
    return GRANT_TYPES.filter(gt => config.relevantGrantTypes.includes(gt.id))
}

/**
 * Determina si un subtipo tiene usuarios interactivos.
 * Util para decidir que scopes mostrar y si mostrar providers.
 */
export function hasInteractiveUsers(subType: AppSubType): boolean {
    return APP_SUB_TYPES[subType].hasInteractiveUsers
}

/**
 * Determina si un subtipo requiere redirect URIs.
 * M2M no requiere redirect URIs.
 */
export function requiresRedirectUris(subType: AppSubType): boolean {
    return APP_SUB_TYPES[subType].requiresRedirectUris
}

/**
 * Retorna los scopes apropiados segun el tipo de aplicacion.
 * - user: scopes OIDC (openid, profile, email, etc.)
 * - api: scopes de recursos (read:users, write:data, etc.)
 */
export function getScopesForSubType(subType: AppSubType): typeof PREDEFINED_SCOPES | typeof API_SCOPES_EXAMPLES {
    const config = APP_SUB_TYPES[subType]
    return config.scopeType === "api" ? API_SCOPES_EXAMPLES : PREDEFINED_SCOPES
}

/**
 * Retorna el mensaje informativo contextual para un subtipo.
 */
export function getInfoMessage(subType: AppSubType): string {
    return APP_SUB_TYPES[subType].infoMessage
}
