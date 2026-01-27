package social

import (
	"encoding/json"
	"net/http"

	httperrors "github.com/dropDatabas3/hellojohn/internal/http/errors"
	"github.com/dropDatabas3/hellojohn/internal/http/helpers"
	svc "github.com/dropDatabas3/hellojohn/internal/http/services/social"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// ProvidersController handles social providers list endpoints.
type ProvidersController struct {
	service svc.ProvidersService
}

// NewProvidersController creates a new ProvidersController.
func NewProvidersController(service svc.ProvidersService) *ProvidersController {
	return &ProvidersController{service: service}
}

// ProvidersResponse is the JSON response for GET /v2/auth/providers.
type ProvidersResponse struct {
	Providers []string `json:"providers"`
}

// GetProviders handles GET /v2/auth/providers and GET /v2/providers/status.
func (c *ProvidersController) GetProviders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ProvidersController.GetProviders"))

	// Validate HTTP method
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
		return
	}

	// Try to resolve tenant (optional - providers can work without tenant)
	tenantSlug := helpers.ResolveTenantSlug(r)

	// Call service
	providers, err := c.service.List(ctx, tenantSlug)
	if err != nil {
		log.Error("failed to list providers", logger.Err(err))
		httperrors.WriteError(w, httperrors.ErrInternalServerError)
		return
	}

	// Ensure non-nil slice for JSON
	if providers == nil {
		providers = []string{}
	}

	// Response
	resp := ProvidersResponse{
		Providers: providers,
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)

	log.Debug("providers list returned", logger.TenantID(tenantSlug))
}
