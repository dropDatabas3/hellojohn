package handlers

import (
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
)

// PublicFormsHandler handles public requests for form configurations.
type PublicFormsHandler struct {
	CPProvider controlplane.ControlPlane
}

// ServeHTTP handles GET /v1/public/tenants/{slug}/forms/{type}
func (h *PublicFormsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Path: /v1/public/tenants/{slug}/forms/{type}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 6 {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_path", "Invalid path", 4001)
		return
	}

	slug := parts[3]
	formType := parts[5] // login or register

	if h.CPProvider == nil {
		httpx.WriteError(w, http.StatusInternalServerError, "cp_unavailable", "Control Plane unavailable", 5001)
		return
	}

	tenant, err := h.CPProvider.GetTenantBySlug(r.Context(), slug)
	if err != nil {
		httpx.WriteError(w, http.StatusNotFound, "tenant_not_found", "Tenant not found", 4041)
		return
	}

	if tenant.Settings.Forms == nil {
		// Return default empty config if not configured
		httpx.WriteJSON(w, http.StatusOK, getDefaultFormConfig(formType))
		return
	}

	var config *controlplane.FormConfig
	switch formType {
	case "login":
		config = tenant.Settings.Forms.Login
	case "register":
		config = tenant.Settings.Forms.Register
	default:
		httpx.WriteError(w, http.StatusBadRequest, "invalid_form_type", "Invalid form type. Use 'login' or 'register'", 4002)
		return
	}

	if config == nil {
		httpx.WriteJSON(w, http.StatusOK, getDefaultFormConfig(formType))
		return
	}

	httpx.WriteJSON(w, http.StatusOK, config)
}

func getDefaultFormConfig(formType string) controlplane.FormConfig {
	// Basic default configuration
	fields := []controlplane.FormField{
		{ID: "email", Type: "email", Label: "Email", Name: "email", Required: true, Placeholder: "name@example.com"},
		{ID: "password", Type: "password", Label: "Password", Name: "password", Required: true, Placeholder: "••••••••"},
	}

	if formType == "register" {
		fields = append([]controlplane.FormField{
			{ID: "name", Type: "text", Label: "Full Name", Name: "name", Required: true, Placeholder: "John Doe"},
		}, fields...)
	}

	return controlplane.FormConfig{
		Theme: controlplane.FormTheme{
			PrimaryColor:    "#0f172a", // Slate 900
			BackgroundColor: "#ffffff",
			TextColor:       "#334155", // Slate 700
			BorderRadius:    "0.5rem",
			InputStyle: controlplane.InputStyle{
				Variant: "outlined",
			},
			ButtonStyle: controlplane.ButtonStyle{
				Variant:   "solid",
				FullWidth: true,
			},
			Spacing:    "normal",
			ShowLabels: true,
		},
		Steps: []controlplane.FormStep{
			{
				ID:     "step-1",
				Title:  "Account Information",
				Fields: fields,
			},
		},
		SocialLayout: controlplane.SocialLayout{
			Position: "bottom",
			Style:    "grid",
		},
	}
}
