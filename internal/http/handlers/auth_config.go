package handlers

import (
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app"
	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
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
		if (err != nil || cl == nil) && cpctx.Provider != nil {
			log.Printf("DEBUG: auth_config client SQL lookup failed for %s, trying FS scan...", clientID)
			tenants, errList := cpctx.Provider.ListTenants(ctx)
			if errList == nil {
				for _, t := range tenants {
					if cFS, errGet := cpctx.Provider.GetClient(ctx, t.Slug, clientID); errGet == nil && cFS != nil {
						// Found in FS! Make a fake Store Client struct from it
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
			TenantName: t.Name,
			TenantSlug: t.Slug,
			ClientName: cl.Name,
			// LogoURL: t.Settings.Branding.LogoURL,
			SocialProviders: cl.Providers,
			PasswordEnabled: true, // Simplified check
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
			"smtp_enabled":         t.Settings.SMTP != nil,
			"social_login_enabled": t.Settings.SocialLoginEnabled,
			"mfa_enabled":          t.Settings.MFAEnabled,
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
