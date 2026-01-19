// Package admin contiene controllers para endpoints administrativos.
package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/controlplane/v2"
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/admin"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/v2/errors"
	mw "github.com/dropDatabas3/hellojohn/internal/http/v2/middlewares"
	svc "github.com/dropDatabas3/hellojohn/internal/http/v2/services/admin"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// ClientsController maneja las rutas /v2/admin/clients
type ClientsController struct {
	service svc.ClientService
}

// NewClientsController crea un nuevo controller de clients.
func NewClientsController(service svc.ClientService) *ClientsController {
	return &ClientsController{service: service}
}

// ListClients maneja GET /v2/admin/clients
func (c *ClientsController) ListClients(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(
		logger.Layer("controller"),
		logger.Op("ClientsController.ListClients"),
	)

	// El tenant viene del middleware
	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("tenant required"))
		return
	}

	log = log.With(logger.TenantSlug(tda.Slug()))

	clients, err := c.service.List(ctx, tda.Slug())
	if err != nil {
		log.Error("list failed", logger.Err(err))
		httperrors.WriteError(w, mapError(err))
		return
	}

	resp := make([]dto.ClientResponse, 0, len(clients))
	for _, cl := range clients {
		resp = append(resp, toClientResponse(cl))
	}

	writeJSON(w, http.StatusOK, resp)
}

// CreateClient maneja POST /v2/admin/clients
func (c *ClientsController) CreateClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(
		logger.Layer("controller"),
		logger.Op("ClientsController.CreateClient"),
	)

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("tenant required"))
		return
	}

	log = log.With(logger.TenantSlug(tda.Slug()))

	var req dto.ClientRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	input := toClientInput(req)

	client, err := c.service.Create(ctx, tda.Slug(), input)
	if err != nil {
		log.Error("create failed", logger.Err(err))
		httperrors.WriteError(w, mapError(err))
		return
	}

	log.Info("client created", logger.ClientID(client.ClientID))
	writeJSON(w, http.StatusCreated, toClientResponse(*client))
}

// UpdateClient maneja PUT/PATCH /v2/admin/clients/{clientId}
func (c *ClientsController) UpdateClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(
		logger.Layer("controller"),
		logger.Op("ClientsController.UpdateClient"),
	)

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("tenant required"))
		return
	}

	// Extraer clientId del path
	clientID := extractClientID(r.URL.Path)
	if clientID == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("missing client_id"))
		return
	}

	log = log.With(logger.TenantSlug(tda.Slug()), logger.ClientID(clientID))

	var req dto.ClientRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	// Forzar el clientId del path (comportamiento V1)
	req.ClientID = clientID
	input := toClientInput(req)

	client, err := c.service.Update(ctx, tda.Slug(), input)
	if err != nil {
		log.Error("update failed", logger.Err(err))
		httperrors.WriteError(w, mapError(err))
		return
	}

	log.Info("client updated")
	writeJSON(w, http.StatusOK, toClientResponse(*client))
}

// DeleteClient maneja DELETE /v2/admin/clients/{clientId}
func (c *ClientsController) DeleteClient(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(
		logger.Layer("controller"),
		logger.Op("ClientsController.DeleteClient"),
	)

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("tenant required"))
		return
	}

	clientID := extractClientID(r.URL.Path)
	if clientID == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("missing client_id"))
		return
	}

	log = log.With(logger.TenantSlug(tda.Slug()), logger.ClientID(clientID))

	if err := c.service.Delete(ctx, tda.Slug(), clientID); err != nil {
		log.Error("delete failed", logger.Err(err))
		httperrors.WriteError(w, mapError(err))
		return
	}

	log.Info("client deleted")
	writeJSON(w, http.StatusOK, dto.StatusResponse{Status: "ok"})
}

// ─── Helpers ───

func extractClientID(path string) string {
	// path = /v2/admin/clients/{clientId}
	const base = "/v2/admin/clients/"
	if !strings.HasPrefix(path, base) {
		return ""
	}
	return strings.Trim(strings.TrimPrefix(path, base), "/")
}

func toClientInput(req dto.ClientRequest) controlplane.ClientInput {
	return controlplane.ClientInput{
		Name:                     req.Name,
		ClientID:                 req.ClientID,
		Type:                     req.Type,
		RedirectURIs:             req.RedirectURIs,
		AllowedOrigins:           req.AllowedOrigins,
		Providers:                req.Providers,
		Scopes:                   req.Scopes,
		Secret:                   req.Secret,
		RequireEmailVerification: req.RequireEmailVerification,
		ResetPasswordURL:         req.ResetPasswordURL,
		VerifyEmailURL:           req.VerifyEmailURL,
	}
}

func toClientResponse(cl repository.Client) dto.ClientResponse {
	return dto.ClientResponse{
		ID:                       cl.ID,
		Name:                     cl.Name,
		ClientID:                 cl.ClientID,
		Type:                     cl.Type,
		RedirectURIs:             cl.RedirectURIs,
		AllowedOrigins:           cl.AllowedOrigins,
		Providers:                cl.Providers,
		Scopes:                   cl.Scopes,
		SecretHash:               cl.SecretEnc, // Enc field maps to hash in response
		RequireEmailVerification: cl.RequireEmailVerification,
		ResetPasswordURL:         cl.ResetPasswordURL,
		VerifyEmailURL:           cl.VerifyEmailURL,
		// CreatedAt/UpdatedAt no existen en repository.Client, se omiten
	}
}

func mapError(err error) *httperrors.AppError {
	switch {
	case isNotFound(err):
		return httperrors.ErrNotFound.WithDetail(err.Error())
	case isConflict(err):
		return httperrors.ErrConflict.WithDetail(err.Error())
	case isBadInput(err):
		return httperrors.ErrBadRequest.WithDetail(err.Error())
	default:
		return httperrors.ErrInternalServerError.WithCause(err)
	}
}

func isNotFound(err error) bool {
	return strings.Contains(err.Error(), "not found")
}

func isConflict(err error) bool {
	return strings.Contains(err.Error(), "conflict") || strings.Contains(err.Error(), "already exists")
}

func isBadInput(err error) bool {
	return strings.Contains(err.Error(), "bad input") || strings.Contains(err.Error(), "required")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
