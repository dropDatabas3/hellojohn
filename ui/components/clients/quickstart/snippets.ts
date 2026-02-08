// ============================================================================
// QUICK START - SDK CODE SNIPPETS
// Templates de codigo para cada SDK con placeholders reemplazados
// ============================================================================

import type { AppSubType } from "@/components/clients/wizard"

export interface SnippetConfig {
  clientId: string
  tenantSlug: string
  domain: string
  type: "public" | "confidential"
  secret?: string
  subType?: AppSubType
}

export interface SdkTab {
  id: string
  label: string
  language: string
  installCmd: string
  filename: string
  /** If true, this tab is only shown for M2M clients */
  m2mOnly?: boolean
  /** If true, this tab is hidden for M2M clients */
  hideForM2M?: boolean
}

export const SDK_TABS: SdkTab[] = [
  { id: "javascript", label: "JavaScript", language: "javascript", installCmd: "npm install @hellojohn/js", filename: "auth.js", hideForM2M: true },
  { id: "react", label: "React", language: "tsx", installCmd: "npm install @hellojohn/react", filename: "App.tsx", hideForM2M: true },
  { id: "node", label: "Node.js", language: "typescript", installCmd: "npm install @hellojohn/node", filename: "auth.ts" },
  { id: "go", label: "Go", language: "go", installCmd: "go get github.com/dropDatabas3/hellojohn-go", filename: "main.go" },
]

// Map sub-type to default SDK tab
export const SUB_TYPE_DEFAULT_SDK: Record<string, string> = {
  spa: "react",
  mobile: "javascript",
  api_server: "node",
  m2m: "node",
}

/**
 * Get filtered SDK tabs based on client subType
 */
export function getFilteredSdkTabs(subType?: AppSubType): SdkTab[] {
  const isM2M = subType === "m2m"
  return SDK_TABS.filter(tab => {
    if (isM2M && tab.hideForM2M) return false
    if (!isM2M && tab.m2mOnly) return false
    return true
  })
}

export function getSnippet(sdkId: string, config: SnippetConfig): string {
  const secret = config.secret || "TU_CLIENT_SECRET"
  const isM2M = config.subType === "m2m"

  // M2M-specific snippets (client_credentials flow)
  if (isM2M) {
    return getM2MSnippet(sdkId, config, secret)
  }

  // Regular (interactive user) snippets
  switch (sdkId) {
    case "javascript":
      return `import { createHelloJohn } from '@hellojohn/js'

const auth = createHelloJohn({
  domain: '${config.domain}',
  clientID: '${config.clientId}',
  tenantID: '${config.tenantSlug}',
  redirectURI: window.location.origin + '/callback',
})

// Login con redirect (PKCE)
await auth.loginWithRedirect()

// Manejar callback
await auth.handleRedirectCallback()

// Obtener usuario autenticado
const user = await auth.getUser()
console.log('Usuario:', user)`

    case "react":
      return `import { HelloJohnProvider, useAuth } from '@hellojohn/react'

function App() {
  return (
    <HelloJohnProvider
      domain="${config.domain}"
      clientID="${config.clientId}"
      tenantID="${config.tenantSlug}"
      redirectURI={window.location.origin + '/callback'}
    >
      <MyApp />
    </HelloJohnProvider>
  )
}

function MyApp() {
  const { isAuthenticated, user, login, logout } = useAuth()

  if (!isAuthenticated) {
    return <button onClick={login}>Iniciar sesion</button>
  }

  return (
    <div>
      <p>Hola, {user.name}</p>
      <button onClick={logout}>Cerrar sesion</button>
    </div>
  )
}`

    case "node":
      return config.type === "confidential"
        ? `import { createServerClient } from '@hellojohn/node'

const auth = createServerClient({
  domain: '${config.domain}',
  clientID: '${config.clientId}',
  clientSecret: '${secret}',
  tenantID: '${config.tenantSlug}',
})

// Verificar token de un request entrante
const user = await auth.verifyToken(req.headers.authorization)

// Obtener informacion de un usuario
const userInfo = await auth.getUser(userId)`
        : `import { createServerClient } from '@hellojohn/node'

const auth = createServerClient({
  domain: '${config.domain}',
  clientID: '${config.clientId}',
  tenantID: '${config.tenantSlug}',
})

// Verificar token de un request entrante
const user = await auth.verifyToken(req.headers.authorization)

// Obtener informacion de un usuario
const userInfo = await auth.getUser(userId)`

    case "go":
      return config.type === "confidential"
        ? `package main

import "github.com/dropDatabas3/hellojohn-go"

func main() {
    client := hellojohn.NewClient(hellojohn.Config{
        Domain:       "${config.domain}",
        ClientID:     "${config.clientId}",
        ClientSecret: "${secret}",
        TenantID:     "${config.tenantSlug}",
    })

    // Verificar token
    claims, err := client.VerifyToken(tokenString)

    // Obtener usuario
    user, err := client.GetUser(ctx, userID)
}`
        : `package main

import "github.com/dropDatabas3/hellojohn-go"

func main() {
    client := hellojohn.NewClient(hellojohn.Config{
        Domain:   "${config.domain}",
        ClientID: "${config.clientId}",
        TenantID: "${config.tenantSlug}",
    })

    // Verificar token
    claims, err := client.VerifyToken(tokenString)

    // Obtener usuario
    user, err := client.GetUser(ctx, userID)
}`

    default:
      return "// SDK no disponible"
  }
}

/**
 * Get M2M-specific snippets using client_credentials flow
 */
function getM2MSnippet(sdkId: string, config: SnippetConfig, secret: string): string {
  switch (sdkId) {
    case "node":
      return `import { createM2MClient } from '@hellojohn/node'

// Cliente Machine-to-Machine (M2M)
// Usa client_credentials para obtener tokens sin interaccion de usuario
const auth = createM2MClient({
  domain: '${config.domain}',
  clientID: '${config.clientId}',
  clientSecret: '${secret}',
  tenantID: '${config.tenantSlug}',
})

// Obtener un access token para llamar a tu API
const { accessToken, expiresIn } = await auth.getAccessToken({
  scope: 'read:data write:data', // Scopes que necesitas
})

// Usar el token para llamar a tu API protegida
const response = await fetch('https://api.tuempresa.com/v1/resource', {
  headers: {
    'Authorization': \`Bearer \${accessToken}\`,
  },
})

// El token se cachea automaticamente y se renueva antes de expirar
// Puedes llamar getAccessToken() multiples veces sin preocuparte

console.log('Token expira en:', expiresIn, 'segundos')`

    case "go":
      return `package main

import (
    "context"
    "fmt"
    "net/http"
    
    "github.com/dropDatabas3/hellojohn-go"
)

func main() {
    // Cliente Machine-to-Machine (M2M)
    // Usa client_credentials para obtener tokens sin interaccion de usuario
    client := hellojohn.NewM2MClient(hellojohn.M2MConfig{
        Domain:       "${config.domain}",
        ClientID:     "${config.clientId}",
        ClientSecret: "${secret}",
        TenantID:     "${config.tenantSlug}",
    })

    ctx := context.Background()

    // Obtener un access token para llamar a tu API
    token, err := client.GetAccessToken(ctx, hellojohn.TokenRequest{
        Scope: "read:data write:data", // Scopes que necesitas
    })
    if err != nil {
        panic(err)
    }

    // Usar el token para llamar a tu API protegida
    req, _ := http.NewRequest("GET", "https://api.tuempresa.com/v1/resource", nil)
    req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.AccessToken))

    // El token se cachea automaticamente y se renueva antes de expirar

    fmt.Printf("Token expira en: %d segundos\\n", token.ExpiresIn)
}`

    default:
      return `// Para M2M, recomendamos usar Node.js o Go
// Estos SDKs soportan client_credentials de forma nativa

// Si usas otro lenguaje, puedes hacer la llamada HTTP directamente:
// POST ${config.domain}/oauth/token
// Content-Type: application/x-www-form-urlencoded
//
// grant_type=client_credentials
// &client_id=${config.clientId}
// &client_secret=${secret}
// &scope=read:data write:data`
  }
}

/**
 * Get next steps based on SDK and client subType
 */
export function getNextSteps(sdkId: string, subType?: AppSubType): string[] {
  const tab = SDK_TABS.find(t => t.id === sdkId)
  if (!tab) return []

  const isM2M = subType === "m2m"

  if (isM2M) {
    return [
      `Instala el SDK: ${tab.installCmd}`,
      "Copia el codigo de arriba en tu servidor/servicio",
      "⚠️ Guarda el client_secret de forma segura (variables de entorno, vault)",
      "Configura los scopes que tu servicio necesita en el panel",
      "Listo! Tu servicio puede obtener tokens para llamar a tus APIs",
    ]
  }

  return [
    `Instala el SDK: ${tab.installCmd}`,
    "Copia el codigo de arriba en tu aplicacion",
    "Asegurate de que la redirect URI coincida con tu configuracion",
    "Listo! Tus usuarios pueden autenticarse con HelloJohn",
  ]
}
