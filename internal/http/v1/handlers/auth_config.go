/*
auth_config.go — Endpoint “config” para el frontend de auth (branding + providers + features + custom fields)

Qué hace este handler
---------------------
Implementa:
  GET /v1/auth/config?client_id=...

Su objetivo es devolverle al frontend (login/register UI, SDK, etc.) la “config pública” necesaria para:
- Branding del tenant (nombre, slug, logo, color)
- Datos del client (nombre, providers habilitados)
- Flags de features (smtp/social/mfa/require_email_verification)
- Definición de custom fields (derivada de tenant.Settings.UserFields)
- URLs de flujos email (reset password / verify email) si vienen configuradas en el client (FS)

Si NO viene client_id:
- devuelve un config genérico “HelloJohn Admin” con password_enabled=true (modo fallback).

Flujo real (paso a paso)
------------------------
1) Lee query param `client_id`.
   - Si viene vacío => responde “Admin config” genérico y chau.

2) Busca el client en SQL (Store)
   - c.Store.GetClientByClientID(ctx, clientID)
   - Si falla o nil, hace un fallback pesado a FS:
       - cpctx.Provider.ListTenants()
       - por cada tenant: cpctx.Provider.GetClient(ctx, tenantSlug, clientID)
       - si lo encuentra, “fabrica” un core.Client (medio trucho) con:
           - ID = cFS.ClientID (ojo)
           - TenantID = t.ID (o si vacío, usa t.Slug)
           - Name + Providers del FS
       - además guarda `clientFS` para extraer config extra (RequireEmailVerification, ResetPasswordURL, VerifyEmailURL)

3) Si no encontró client => 404 client_not_found

4) Busca el tenant para branding
   - exige cpctx.Provider != nil
   - intenta cpctx.Provider.GetTenantByID(ctx, cl.TenantID)
   - si falla => vuelve a listar tenants y matchea por ID o por Slug (otro O(N))

5) Construye la respuesta
   - TenantName / TenantSlug / ClientName / SocialProviders / PasswordEnabled
   - si `clientFS` existe:
       - setea RequireEmailVerification + ResetPasswordURL + VerifyEmailURL
   - logo:
       - si t.Settings.LogoURL existe, lo usa
       - si no existe o no empieza con http => intenta leer “logo.png” del FS (DATA_ROOT/tenants/{slug}/logo.png)
         y lo embebe como data URL base64 (data:image/png;base64,...)
   - color:
       - PrimaryColor = t.Settings.BrandColor (si está)
   - passwordEnabled:
       - por default true
       - si cl.Providers tiene items, revisa si incluye "password" (case-insensitive)
   - Features map:
       smtp_enabled, social_login_enabled, mfa_enabled, require_email_verification
   - CustomFields:
       recorre t.Settings.UserFields y los transforma a CustomFieldSchema (Label = Name)

Cuellos de botella / cosas “viejas y rotas” probables
-----------------------------------------------------
A) O(N) en caliente por request (dos veces)
   - Fallback de client: ListTenants + GetClient por tenant (potencialmente carísimo)
   - Fallback de tenant: ListTenants y match ID/slug
   Esto escala horrible con muchos tenants.

B) Mezcla de fuentes (SQL vs FS) sin una abstracción clara
   - El handler hace “dual read” y arma structs fake.
   - cl.ID = clientID cuando no hay UUID real => te va a romper invariantes en otros lugares.

C) Logo embebido como base64 en el JSON
   - Puede inflar respuestas y cache/cdn se vuelve un quilombo.
   - Además lee disco por request (otra vez, caro).

D) Logging “DEBUG” con log.Printf en handler
   - Ruido en prod y puede filtrar info operativa.
   - Si mañana metés secrets en settings, te podés mandar una cagada.

E) Responsabilidades mezcladas
   - Controller hace: lookup, fallback FS, resolver tenant, leer logo del FS, construir response.
   - Difícil de testear, y más difícil de evolucionar.

F) Esquema de custom fields: hoy Label=Name porque “no hay label”
   - Si mañana agregás label, este handler debería estar listo para mapearlo.
   - Y Type “text/number/boolean”: hoy se confía en uf.Type sin validación.

Cómo lo refactorizaría en V2 (bien prolijo)
-------------------------------------------
Meta: controller finito + service + repos + caches. Y que “de dónde sale la data” esté encapsulado.

1) Introducir un “ConfigService” (Service Layer)
   - Patrón GoF: Facade (fachada) hacia varias fuentes y caches.
   - Firma:
       ConfigService.GetAuthConfig(ctx, clientID string) (AuthConfigResponse, error)

2) Introducir un “ClientResolver” (Strategy + Chain of Responsibility)
   - GoF: Strategy para “resolver client” según backend activo.
   - GoF: Chain of Responsibility para fallback ordenado:
       a) SQLClientRepo (rápido)
       b) FSClientRepo (si está habilitado)
   - Esto evita que el handler tenga loops de tenants.

3) Introducir “TenantResolver” (cacheado)
   - ResolveTenantByIDOrSlug(ctx, idOrSlug) -> Tenant
   - Cache TTL/LRU (por ej 1-5 min) y/o invalidación cuando cambia el controlplane.

4) Introducir “LogoProvider” (Strategy)
   - GoF: Strategy para resolver logo:
       - URLLogoProvider (si LogoURL es http(s))
       - FSLogoProvider (lee del FS)
   - Pero importante: NO embebas base64 en este endpoint si podés evitarlo.
     Mejor:
       - devolver siempre una URL de assets (ej /v1/assets/tenants/{slug}/logo.png)
       - y que ese endpoint sirva el archivo con cache headers.

5) DTOs + contratos estables
   - Este endpoint es “público” => mantenelo versionado.
   - CamelCase consistente si tu v2 lo pide.
   - Validaciones: providers permitidos, tipos de custom fields, etc.

6) Cache multi-nivel (mejora de infra sin goroutines falopa)
   - Cachear:
       - client config por client_id (TTL corto)
       - tenant branding por tenant_slug/id (TTL corto)
       - logo bytes o “logo URL” (TTL más largo)
   - Esto te baja muchísimo CPU/IO.

Dónde meter concurrencia (si querés aprovechar Go)
--------------------------------------------------
Acá sí tiene sentido un poco, pero con criterio:
- Una vez que resolviste el tenant y el client:
  podés construir features + custom fields en paralelo? meh, no vale.
- Donde sí: si tu LogoProvider implica IO (leer archivo, o pedir a storage),
  y también necesitás otra cosa, podés hacer:
    - goroutine para resolver logo
    - goroutine para resolver tenant settings
  y esperar con errgroup + context.
Pero si cacheás bien, no lo necesitás.

Plan concreto de refactor por capas
-----------------------------------
A) Controller: controllers/auth_config_controller.go
   - parsea client_id
   - llama configSvc.GetAuthConfig(ctx, clientID)
   - WriteJSON

B) Service: services/auth_config_service.go
   - si clientID vacío => config admin default
   - cl := clientResolver.ResolveByClientID(ctx, clientID)
   - tenant := tenantResolver.Resolve(ctx, cl.TenantRef) (id o slug)
   - logo := logoProvider.Resolve(ctx, tenant)
   - arma response (features + providers + custom fields)
   - devuelve

C) Repos:
   - repos/client_repo_sql.go
   - repos/client_repo_fs.go
   - repos/tenant_repo_cp.go (o tenant resolver)

D) Infra:
   - infra/cache (TTL/LRU)
   - infra/assets (servir logo estático con cache-control)

Checklist de mejoras inmediatas sin romper todo
-----------------------------------------------
- Sacar loops de ListTenants() del handler (ponerlo en resolver con cache).
- Dejar de embebir logo base64 en config (devolver URL).
- No “fabrices” core.Client con ID=clientID (creá un DTO propio de config).
- Limitar logs y mover a logger estructurado.
- Test unitarios: casos SQL ok, FS fallback ok, tenant mismatch, client not found.

Resumen del veredicto
---------------------
Este handler es útil, pero hoy es un “mezcladito” de SQL+FS con búsquedas O(N) y lectura de FS por request.
En V2 hay que convertirlo en un Facade (ConfigService) apoyado en Resolvers (Strategy/Chain) + caches,
y separar “branding/assets” del JSON para que sea eficiente y mantenible.
*/

package handlers

import (
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1"
	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

type CustomFieldSchema struct {
	Name     string `json:"name"`
	Type     string `json:"type"` // text, number, boolean
	Label    string `json:"label"`
	Required bool   `json:"required"`
}

type AuthConfigResponse struct {
	TenantName      string              `json:"tenant_name"`
	TenantSlug      string              `json:"tenant_slug"`
	ClientName      string              `json:"client_name"`
	LogoURL         string              `json:"logo_url,omitempty"`
	PrimaryColor    string              `json:"primary_color,omitempty"`
	SocialProviders []string            `json:"social_providers"`
	PasswordEnabled bool                `json:"password_enabled"`
	Features        map[string]bool     `json:"features,omitempty"`
	CustomFields    []CustomFieldSchema `json:"custom_fields,omitempty"`

	// Email verification & password reset
	RequireEmailVerification bool   `json:"require_email_verification,omitempty"`
	ResetPasswordURL         string `json:"reset_password_url,omitempty"`
	VerifyEmailURL           string `json:"verify_email_url,omitempty"`
}

func NewAuthConfigHandler(c *app.Container) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		clientID := r.URL.Query().Get("client_id")
		if clientID == "" {
			// Return generic/admin config if no client
			httpx.WriteJSON(w, http.StatusOK, AuthConfigResponse{
				TenantName:      "HelloJohn Admin",
				PasswordEnabled: true,
			})
			return
		}

		ctx := r.Context()

		// 1. Lookup Client
		cl, _, err := c.Store.GetClientByClientID(ctx, clientID)

		// Fallback to FS if not found in SQL (e.g. YAML-only client)
		var clientFS *controlplane.OIDCClient // Keep reference to FS client for extra fields
		if (err != nil || cl == nil) && cpctx.Provider != nil {
			log.Printf("DEBUG: auth_config client SQL lookup failed for %s, trying FS scan...", clientID)
			tenants, errList := cpctx.Provider.ListTenants(ctx)
			if errList == nil {
				for _, t := range tenants {
					if cFS, errGet := cpctx.Provider.GetClient(ctx, t.Slug, clientID); errGet == nil && cFS != nil {
						// Found in FS! Make a fake Store Client struct from it
						clientFS = cFS // Save reference for extra fields
						cl = &core.Client{
							ID:        cFS.ClientID, // Use ClientID as ID for now or UUID if available
							ClientID:  cFS.ClientID,
							TenantID:  t.ID, // Prefer UUID, fallback to Slug if empty handled later
							Name:      cFS.Name,
							Providers: cFS.Providers,
						}
						// Ensure TenantID is robust
						if cl.TenantID == "" {
							cl.TenantID = t.Slug
						}
						err = nil // Cleared
						log.Printf("DEBUG: auth_config resolved client %s from FS tenant %s", clientID, t.Slug)
						break
					}
				}
			}
		}

		if err != nil || cl == nil {
			httpx.WriteError(w, http.StatusNotFound, "client_not_found", "client no encontrado", 1004)
			return
		}

		// 2. Lookup Tenant to get branding
		// Use cpctx.Provider because Store (SQL) might not have valid GetTenantByID if using FS mode,
		// and GetTenantByID is part of the ControlPlane interface.
		if cpctx.Provider == nil {
			httpx.WriteError(w, http.StatusInternalServerError, "provider_not_initialized", "cp provider nil", 1005)
			return
		}

		t, err := cpctx.Provider.GetTenantByID(ctx, cl.TenantID)
		// Fallback: If ID lookup fails, try slug if we have it in cl.TenantID (from FS fallback)
		// Or if cl.TenantID was ID but provider expects Slug.
		if err != nil {
			// Quick re-scan to match ID or slug if direct lookup failed
			// Reuse scan logic from session_login?
			// Simplest: Iterate tenants again to find match by ID or Slug
			tenants, _ := cpctx.Provider.ListTenants(ctx)
			for _, ten := range tenants {
				if ten.ID == cl.TenantID || ten.Slug == cl.TenantID {
					t = &ten
					err = nil
					break
				}
			}
		}

		if err != nil {
			httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "tenant no encontrado", 1004)
			return
		}

		// 3. Construct Response
		resp := AuthConfigResponse{
			TenantName:      t.Name,
			TenantSlug:      t.Slug,
			ClientName:      cl.Name,
			SocialProviders: cl.Providers,
			PasswordEnabled: true, // Simplified check
		}

		// Populate email verification config from FS client if available
		if clientFS != nil {
			resp.RequireEmailVerification = clientFS.RequireEmailVerification
			resp.ResetPasswordURL = clientFS.ResetPasswordURL
			resp.VerifyEmailURL = clientFS.VerifyEmailURL
		}

		if t.Settings.LogoURL != "" {
			resp.LogoURL = t.Settings.LogoURL
		}
		// Try to load logo from FS if LogoURL is empty or points to local file
		if resp.LogoURL == "" || !strings.HasPrefix(resp.LogoURL, "http") {
			// Try to read logo.png from tenant FS folder
			dataRoot := os.Getenv("DATA_ROOT")
			if dataRoot == "" {
				dataRoot = "./data/hellojohn"
			}
			logoPath := filepath.Join(dataRoot, "tenants", t.Slug, "logo.png")
			if data, err := os.ReadFile(logoPath); err == nil {
				resp.LogoURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(data)
				log.Printf("DEBUG: auth_config loaded logo from FS for tenant %s", t.Slug)
			}
		}
		// If using brandColor from settings if available
		if t.Settings.BrandColor != "" {
			resp.PrimaryColor = t.Settings.BrandColor
		}

		// Check if password is in providers
		hasPwd := false
		for _, p := range cl.Providers {
			if strings.EqualFold(p, "password") {
				hasPwd = true
			}
		}
		if len(cl.Providers) > 0 {
			resp.PasswordEnabled = hasPwd
		}

		resp.Features = map[string]bool{
			"smtp_enabled":               t.Settings.SMTP != nil,
			"social_login_enabled":       t.Settings.SocialLoginEnabled,
			"mfa_enabled":                t.Settings.MFAEnabled,
			"require_email_verification": resp.RequireEmailVerification,
		}

		// Extract Custom Fields from UserFields definition
		for _, uf := range t.Settings.UserFields {
			resp.CustomFields = append(resp.CustomFields, CustomFieldSchema{
				Name:     uf.Name,
				Type:     uf.Type,
				Required: uf.Required,
				Label:    uf.Name, // Fallback label to name if not provided (UserFieldDefinition has no label)
			})
		}

		httpx.WriteJSON(w, http.StatusOK, resp)
	}
}
