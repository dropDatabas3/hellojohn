package social

import (
	"net/http"
	"strings"

	httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
	svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/social"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// CallbackController handles social login callback endpoint.
type CallbackController struct {
	service svc.CallbackService
}

// NewCallbackController creates a new CallbackController.
func NewCallbackController(service svc.CallbackService) *CallbackController {
	return &CallbackController{service: service}
}

// Callback handles GET /v2/auth/social/{provider}/callback
func (c *CallbackController) Callback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("CallbackController.Callback"))

	// Validate HTTP method
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
		return
	}

	// Extract provider from path (Go 1.22+ path params)
	provider := r.PathValue("provider")
	if provider == "" {
		// Fallback: parse from URL path manually
		// Path expected: /v2/auth/social/{provider}/callback
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v2/auth/social/"), "/")
		if len(parts) >= 1 {
			provider = parts[0]
		}
	}

	if provider == "" {
		log.Warn("missing provider in path")
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("missing provider"))
		return
	}

	// Read query parameters
	q := r.URL.Query()

	// Check for IDP error first
	if idpError := strings.TrimSpace(q.Get("error")); idpError != "" {
		idpDesc := strings.TrimSpace(q.Get("error_description"))
		log.Warn("IDP error",
			logger.String("provider", provider),
			logger.String("error", idpError),
			logger.String("description", idpDesc),
		)
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("idp_error: "+idpError+" "+idpDesc))
		return
	}

	state := strings.TrimSpace(q.Get("state"))
	code := strings.TrimSpace(q.Get("code"))

	if state == "" {
		log.Warn("missing state")
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("state required"))
		return
	}

	if code == "" {
		log.Warn("missing code")
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("code required"))
		return
	}

	// Build base URL from request
	scheme := r.URL.Scheme
	if scheme == "" {
		scheme = "https"
		if strings.HasPrefix(r.Host, "localhost") || strings.HasPrefix(r.Host, "127.0.0.1") {
			scheme = "http"
		}
		// Check X-Forwarded-Proto header
		if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
			scheme = proto
		}
	}
	baseURL := scheme + "://" + r.Host

	// Call service
	result, err := c.service.Callback(ctx, svc.CallbackRequest{
		Provider: provider,
		State:    state,
		Code:     code,
		BaseURL:  baseURL,
	})

	if err != nil {
		log.Error("callback failed",
			logger.String("provider", provider),
			logger.Err(err),
		)

		switch err {
		case svc.ErrCallbackMissingState:
			httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("state required"))
		case svc.ErrCallbackMissingCode:
			httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("code required"))
		case svc.ErrCallbackInvalidState:
			httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("invalid state"))
		case svc.ErrCallbackProviderMismatch:
			httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("provider mismatch"))
		case svc.ErrCallbackProviderUnknown, svc.ErrCallbackProviderDisabled:
			httperrors.WriteError(w, httperrors.ErrNotFound.WithDetail("provider not enabled"))
		case svc.ErrCallbackOIDCExchangeFailed:
			httperrors.WriteError(w, httperrors.ErrInternalServerError.WithDetail("code exchange failed"))
		case svc.ErrCallbackIDTokenInvalid:
			httperrors.WriteError(w, httperrors.ErrUnauthorized.WithDetail("id_token invalid"))
		case svc.ErrCallbackEmailMissing:
			httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("email missing"))
		default:
			httperrors.WriteError(w, httperrors.ErrInternalServerError)
		}
		return
	}

	// Set anti-cache headers
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	// Redirect or JSON response
	if result.RedirectURL != "" {
		http.Redirect(w, r, result.RedirectURL, http.StatusFound)
		log.Debug("redirecting to client",
			logger.String("provider", provider),
		)
		return
	}

	// JSON response
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.JSONResponse)

	log.Debug("callback completed",
		logger.String("provider", provider),
	)
}
