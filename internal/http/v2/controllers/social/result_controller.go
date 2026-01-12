package social

import (
	"net/http"
	"strings"

	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/social"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
	svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/social"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// ResultController handles GET /v2/auth/social/result.
type ResultController struct {
	service svc.ResultService
}

// NewResultController creates a new social result controller.
func NewResultController(service svc.ResultService) *ResultController {
	return &ResultController{service: service}
}

// GetResult handles the social login code result request.
// Returns the stored tokens for a login code.
// Supports JSON output only (HTML template removed for security and simplicity).
func (c *ResultController) GetResult(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ResultController.GetResult"))

	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
		return
	}

	// Parse query parameters
	q := r.URL.Query()
	code := strings.TrimSpace(q.Get("code"))
	peek := q.Get("peek") == "1"

	if code == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("code is required"))
		return
	}

	// Build request
	req := dto.ResultRequest{
		Code: code,
		Peek: peek,
	}

	// Call service
	result, err := c.service.GetResult(ctx, req)
	if err != nil {
		switch err {
		case svc.ErrResultCodeMissing:
			httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("code is required"))
		case svc.ErrResultCodeNotFound:
			httperrors.WriteError(w, httperrors.ErrNotFound.WithDetail("code not found or expired"))
		default:
			log.Error("get result error", logger.Err(err))
			httperrors.WriteError(w, httperrors.ErrInternalServerError)
		}
		return
	}

	// Set anti-cache headers
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Pragma", "no-cache")

	// Add peek debug header if in peek mode
	if result.Peek {
		w.Header().Set("X-Debug-Note", "peek=1 (code not consumed)")
	}

	// Return JSON (payload is the raw tokens JSON)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(result.Payload)

	log.Debug("social result returned")
}

// GetResultJSON is an alias that explicitly returns JSON.
// This can be used if an HTML endpoint is needed separately.
func (c *ResultController) GetResultJSON(w http.ResponseWriter, r *http.Request) {
	c.GetResult(w, r)
}

// ResultMetadata returns metadata about a code without the full payload.
// Useful for checking if a code exists without consuming it.
func (c *ResultController) ResultMetadata(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ResultController.ResultMetadata"))

	if r.Method != http.MethodHead {
		w.Header().Set("Allow", "HEAD")
		httperrors.WriteError(w, httperrors.ErrMethodNotAllowed)
		return
	}

	q := r.URL.Query()
	code := strings.TrimSpace(q.Get("code"))

	if code == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("code is required"))
		return
	}

	// Use peek mode to check without consuming
	req := dto.ResultRequest{
		Code: code,
		Peek: true,
	}

	_, err := c.service.GetResult(ctx, req)
	if err != nil {
		if err == svc.ErrResultCodeNotFound {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Error("metadata error", logger.Err(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
}
