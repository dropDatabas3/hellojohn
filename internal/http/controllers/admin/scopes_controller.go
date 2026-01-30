package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/admin"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/errors"
	mw "github.com/dropDatabas3/hellojohn/internal/http/middlewares"
	svc "github.com/dropDatabas3/hellojohn/internal/http/services/admin"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// ScopesController maneja las rutas /v2/admin/scopes
type ScopesController struct {
	service svc.ScopeService
}

// NewScopesController crea un nuevo controller de scopes.
func NewScopesController(service svc.ScopeService) *ScopesController {
	return &ScopesController{service: service}
}

// ListScopes maneja GET /v2/admin/scopes
func (c *ScopesController) ListScopes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ScopesController.ListScopes"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	scopes, err := c.service.List(ctx, tda.Slug())
	if err != nil {
		log.Error("list failed", logger.Err(err))
		httperrors.WriteError(w, mapScopeError(err))
		return
	}

	resp := make([]dto.ScopeResponse, 0, len(scopes))
	for _, s := range scopes {
		resp = append(resp, toScopeResponse(s))
	}

	writeJSON(w, http.StatusOK, resp)
}

// UpsertScope maneja POST/PUT /v2/admin/scopes
func (c *ScopesController) UpsertScope(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ScopesController.UpsertScope"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	var req dto.ScopeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		httperrors.WriteError(w, httperrors.ErrMissingFields.WithDetail("name requerido"))
		return
	}

	input := toScopeInput(req)
	scope, err := c.service.Upsert(ctx, tda.Slug(), input)
	if err != nil {
		log.Error("upsert failed", logger.Err(err))
		httperrors.WriteError(w, mapScopeError(err))
		return
	}

	log.Info("scope upserted", logger.String("scope_name", scope.Name))
	writeJSON(w, http.StatusOK, toScopeResponse(*scope))
}

// DeleteScope maneja DELETE /v2/admin/scopes/{name}
func (c *ScopesController) DeleteScope(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("ScopesController.DeleteScope"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	// Extraer name del path
	name := extractScopeName(r.URL.Path)
	if name == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("missing scope name"))
		return
	}

	if err := c.service.Delete(ctx, tda.Slug(), name); err != nil {
		log.Error("delete failed", logger.Err(err))
		httperrors.WriteError(w, mapScopeError(err))
		return
	}

	log.Info("scope deleted", logger.String("scope_name", name))
	writeJSON(w, http.StatusOK, dto.StatusResponse{Status: "ok"})
}

// ─── Helpers ───

func extractScopeName(path string) string {
	const prefix = "/v2/admin/scopes/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	return strings.TrimPrefix(path, prefix)
}

func toScopeResponse(s repository.Scope) dto.ScopeResponse {
	resp := dto.ScopeResponse{
		Name:        s.Name,
		Description: s.Description,
		DisplayName: s.DisplayName,
		Claims:      s.Claims,
		DependsOn:   s.DependsOn,
		System:      s.System,
	}
	if !s.CreatedAt.IsZero() {
		resp.CreatedAt = s.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	if s.UpdatedAt != nil && !s.UpdatedAt.IsZero() {
		resp.UpdatedAt = s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
	}
	return resp
}

func toScopeInput(req dto.ScopeRequest) repository.ScopeInput {
	return repository.ScopeInput{
		Name:        req.Name,
		Description: req.Description,
		DisplayName: req.DisplayName,
		Claims:      req.Claims,
		DependsOn:   req.DependsOn,
		System:      req.System,
	}
}

func mapScopeError(err error) *httperrors.AppError {
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found"):
		return httperrors.ErrNotFound.WithDetail(errMsg)
	case strings.Contains(errMsg, "in use"):
		return httperrors.ErrConflict.WithDetail(errMsg)
	case strings.Contains(errMsg, "required"):
		return httperrors.ErrBadRequest.WithDetail(errMsg)
	default:
		return httperrors.ErrInternalServerError.WithCause(err)
	}
}
