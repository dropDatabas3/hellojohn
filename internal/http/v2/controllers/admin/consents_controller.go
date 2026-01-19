package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/admin"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/admin"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	"github.com/google/uuid"
)

const tenantRequired = "tenant required"

// ConsentsController maneja las rutas /v2/admin/consents
type ConsentsController struct {
	service svc.ConsentService
}

// NewConsentsController crea un nuevo controller de consents.
func NewConsentsController(service svc.ConsentService) *ConsentsController {
	return &ConsentsController{service: service}
}

// Upsert maneja POST /v2/admin/consents/upsert
func (c *ConsentsController) Upsert(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ConsentsController.Upsert"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	var req dto.ConsentUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	// Validaciones
	if req.UserID == "" || req.ClientID == "" || len(req.Scopes) == 0 {
		httperrors.WriteError(w, httperrors.ErrMissingFields.WithDetail("user_id, client_id y scopes requeridos"))
		return
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("user_id debe ser UUID"))
		return
	}

	consent, err := c.service.Upsert(ctx, tda, req.UserID, req.ClientID, req.Scopes)
	if err != nil {
		log.Error("upsert failed", logger.Err(err))
		httperrors.WriteError(w, mapConsentError(err))
		return
	}

	writeJSON(w, http.StatusOK, toConsentResponse(*consent))
}

// ListByUser maneja GET /v2/admin/consents/by-user/{userID}
func (c *ConsentsController) ListByUser(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ConsentsController.ListByUser"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	// Extraer userID de query o path
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		userID = extractUserIDFromPath(r.URL.Path)
	}
	if userID == "" {
		httperrors.WriteError(w, httperrors.ErrMissingFields.WithDetail("user_id requerido"))
		return
	}
	if _, err := uuid.Parse(userID); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("user_id debe ser UUID"))
		return
	}

	activeOnly := strings.EqualFold(r.URL.Query().Get("active_only"), "true")

	consents, err := c.service.ListByUser(ctx, tda, userID, activeOnly)
	if err != nil {
		log.Error("list failed", logger.Err(err))
		httperrors.WriteError(w, mapConsentError(err))
		return
	}

	writeJSON(w, http.StatusOK, toConsentListResponse(consents))
}

// List maneja GET /v2/admin/consents (con filtros)
func (c *ConsentsController) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ConsentsController.List"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	clientID := strings.TrimSpace(r.URL.Query().Get("client_id"))
	activeOnly := strings.EqualFold(r.URL.Query().Get("active_only"), "true")

	// Si hay user + client: get específico
	if userID != "" && clientID != "" {
		if _, err := uuid.Parse(userID); err != nil {
			httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("user_id debe ser UUID"))
			return
		}

		consent, err := c.service.Get(ctx, tda, userID, clientID)
		if err != nil {
			if isNotFound(err) {
				writeJSON(w, http.StatusOK, []dto.ConsentResponse{})
				return
			}
			log.Error("get failed", logger.Err(err))
			httperrors.WriteError(w, mapConsentError(err))
			return
		}

		// Filtrar por activeOnly
		if activeOnly && consent.RevokedAt != nil {
			writeJSON(w, http.StatusOK, []dto.ConsentResponse{})
			return
		}

		writeJSON(w, http.StatusOK, []dto.ConsentResponse{toConsentResponse(*consent)})
		return
	}

	// Si solo user: list by user
	if userID != "" {
		if _, err := uuid.Parse(userID); err != nil {
			httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("user_id debe ser UUID"))
			return
		}

		consents, err := c.service.ListByUser(ctx, tda, userID, activeOnly)
		if err != nil {
			log.Error("list failed", logger.Err(err))
			httperrors.WriteError(w, mapConsentError(err))
			return
		}

		writeJSON(w, http.StatusOK, toConsentListResponse(consents))
		return
	}

	// Sin filtros suficientes
	httperrors.WriteError(w, httperrors.ErrMissingFields.WithDetail("requerido al menos user_id"))
}

// Revoke maneja POST /v2/admin/consents/revoke
func (c *ConsentsController) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ConsentsController.Revoke"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	var req dto.ConsentRevokeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	if req.UserID == "" || req.ClientID == "" {
		httperrors.WriteError(w, httperrors.ErrMissingFields.WithDetail("user_id y client_id requeridos"))
		return
	}
	if _, err := uuid.Parse(req.UserID); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("user_id debe ser UUID"))
		return
	}

	// Parsear timestamp opcional
	at := time.Now()
	if req.At != "" {
		t, err := time.Parse(time.RFC3339, req.At)
		if err != nil {
			httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("at debe ser RFC3339"))
			return
		}
		at = t
	}

	if err := c.service.Revoke(ctx, tda, req.UserID, req.ClientID, at); err != nil {
		log.Error("revoke failed", logger.Err(err))
		httperrors.WriteError(w, mapConsentError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Delete maneja DELETE /v2/admin/consents/{userID}/{clientID}
func (c *ConsentsController) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ConsentsController.Delete"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	// Extraer userID y clientID del path
	userID, clientID := extractConsentPathParams(r.URL.Path)
	if userID == "" || clientID == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("se espera /consents/{user_id}/{client_id}"))
		return
	}
	if _, err := uuid.Parse(userID); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("user_id debe ser UUID"))
		return
	}

	if err := c.service.Revoke(ctx, tda, userID, clientID, time.Now()); err != nil {
		log.Error("delete failed", logger.Err(err))
		httperrors.WriteError(w, mapConsentError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Helpers ───

func extractUserIDFromPath(path string) string {
	const prefix = "/v2/admin/consents/by-user/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimPrefix(path, prefix)
}

func extractConsentPathParams(path string) (userID, clientID string) {
	const prefix = "/v2/admin/consents/"
	if !strings.HasPrefix(path, prefix) {
		return "", ""
	}
	tail := strings.TrimPrefix(path, prefix)
	parts := strings.Split(strings.Trim(tail, "/"), "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func toConsentResponse(c repository.Consent) dto.ConsentResponse {
	return dto.ConsentResponse{
		ID:        c.ID,
		TenantID:  c.TenantID,
		UserID:    c.UserID,
		ClientID:  c.ClientID,
		Scopes:    c.Scopes,
		CreatedAt: c.GrantedAt, // Mapeado desde GrantedAt
		UpdatedAt: c.UpdatedAt,
		RevokedAt: c.RevokedAt,
	}
}

func toConsentListResponse(consents []repository.Consent) []dto.ConsentResponse {
	result := make([]dto.ConsentResponse, 0, len(consents))
	for _, c := range consents {
		result = append(result, toConsentResponse(c))
	}
	return result
}

func mapConsentError(err error) *httperrors.AppError {
	switch {
	case isNotFound(err):
		return httperrors.ErrNotFound.WithDetail(err.Error())
	case isServiceUnavailable(err):
		return httperrors.ErrServiceUnavailable.WithDetail(err.Error())
	case isBadInput(err):
		return httperrors.ErrBadRequest.WithDetail(err.Error())
	default:
		return httperrors.ErrInternalServerError.WithCause(err)
	}
}

func isServiceUnavailable(err error) bool {
	return strings.Contains(err.Error(), "no database") || strings.Contains(err.Error(), "not available")
}
