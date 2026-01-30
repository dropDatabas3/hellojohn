package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/admin"
	httperrors "github.com/dropDatabas3/hellojohn/internal/http/errors"
	mw "github.com/dropDatabas3/hellojohn/internal/http/middlewares"
	svc "github.com/dropDatabas3/hellojohn/internal/http/services/admin"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	"github.com/google/uuid"
)

// RBACController maneja las rutas /v2/admin/rbac
type RBACController struct {
	service svc.RBACService
}

// NewRBACController crea un nuevo controller RBAC.
func NewRBACController(service svc.RBACService) *RBACController {
	return &RBACController{service: service}
}

// GetUserRoles maneja GET /v2/admin/rbac/users/{userID}/roles
func (c *RBACController) GetUserRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.GetUserRoles"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	userID := extractUserIDFromRBACPath(r.URL.Path)
	if userID == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("user_id requerido"))
		return
	}
	if _, err := uuid.Parse(userID); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("user_id debe ser UUID"))
		return
	}

	roles, err := c.service.GetUserRoles(ctx, tda, userID)
	if err != nil {
		log.Error("get roles failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	writeJSON(w, http.StatusOK, dto.RBACUserRolesResponse{
		UserID: userID,
		Roles:  roles,
	})
}

// UpdateUserRoles maneja POST /v2/admin/rbac/users/{userID}/roles
func (c *RBACController) UpdateUserRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.UpdateUserRoles"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	userID := extractUserIDFromRBACPath(r.URL.Path)
	if userID == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("user_id requerido"))
		return
	}
	if _, err := uuid.Parse(userID); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidFormat.WithDetail("user_id debe ser UUID"))
		return
	}

	var req dto.RBACRolesUpdateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	roles, err := c.service.UpdateUserRoles(ctx, tda, userID, req.Add, req.Remove)
	if err != nil {
		log.Error("update roles failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	log.Info("user roles updated", logger.UserID(userID))
	writeJSON(w, http.StatusOK, dto.RBACUserRolesResponse{
		UserID: userID,
		Roles:  roles,
	})
}

// GetRolePerms maneja GET /v2/admin/rbac/roles/{role}/perms
func (c *RBACController) GetRolePerms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.GetRolePerms"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	role := extractRoleFromRBACPath(r.URL.Path)
	if role == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("role requerido"))
		return
	}

	perms, err := c.service.GetRolePerms(ctx, tda, role)
	if err != nil {
		log.Error("get perms failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	writeJSON(w, http.StatusOK, dto.RBACRolePermsResponse{
		TenantID: tda.ID(),
		Role:     role,
		Perms:    perms,
	})
}

// UpdateRolePerms maneja POST /v2/admin/rbac/roles/{role}/perms
func (c *RBACController) UpdateRolePerms(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.UpdateRolePerms"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	role := extractRoleFromRBACPath(r.URL.Path)
	if role == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("role requerido"))
		return
	}

	var req dto.RBACPermsUpdateRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	perms, err := c.service.UpdateRolePerms(ctx, tda, role, req.Add, req.Remove)
	if err != nil {
		log.Error("update perms failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	log.Info("role perms updated", logger.String("role", role))
	writeJSON(w, http.StatusOK, dto.RBACRolePermsResponse{
		TenantID: tda.ID(),
		Role:     role,
		Perms:    perms,
	})
}

// ─── Helpers ───

// extractUserIDFromRBACPath extrae userID de /v2/admin/rbac/users/{userID}/roles
func extractUserIDFromRBACPath(path string) string {
	const prefix = "/v2/admin/rbac/users/"
	const suffix = "/roles"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return strings.Trim(mid, "/")
}

// extractRoleFromRBACPath extrae role de /v2/admin/rbac/roles/{role}/perms
func extractRoleFromRBACPath(path string) string {
	const prefix = "/v2/admin/rbac/roles/"
	const suffix = "/perms"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	mid := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return strings.Trim(mid, "/")
}

// extractRoleNameFromPath extrae role de /v2/admin/rbac/roles/{name}
func extractRoleNameFromPath(path string) string {
	const prefix = "/v2/admin/rbac/roles/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	name := strings.TrimPrefix(path, prefix)
	// Quitar posibles sufijos como /perms
	if idx := strings.Index(name, "/"); idx > 0 {
		name = name[:idx]
	}
	return strings.Trim(name, "/")
}

func mapRBACError(err error) *httperrors.AppError {
	if errors.Is(err, svc.ErrRBACNotSupported) {
		return httperrors.ErrNotImplemented.WithDetail("RBAC no soportado por el store")
	}
	errMsg := err.Error()
	switch {
	case strings.Contains(errMsg, "not found"):
		return httperrors.ErrNotFound.WithDetail(errMsg)
	case strings.Contains(errMsg, "no database"):
		return httperrors.ErrServiceUnavailable.WithDetail(errMsg)
	case strings.Contains(errMsg, "invalid role name"):
		return httperrors.ErrBadRequest.WithDetail(errMsg)
	case strings.Contains(errMsg, "cannot delete system role"):
		return httperrors.ErrForbidden.WithDetail(errMsg)
	case strings.Contains(errMsg, "cannot modify system role"):
		return httperrors.ErrForbidden.WithDetail(errMsg)
	default:
		return httperrors.ErrInternalServerError.WithCause(err)
	}
}

// ─── Role CRUD Handlers ───

// ListRoles maneja GET /v2/admin/rbac/roles
func (c *RBACController) ListRoles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.ListRoles"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	roles, err := c.service.ListRoles(ctx, tda)
	if err != nil {
		log.Error("list roles failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"roles": roles})
}

// GetRoleByName maneja GET /v2/admin/rbac/roles/{name}
func (c *RBACController) GetRoleByName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.GetRoleByName"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	name := extractRoleNameFromPath(r.URL.Path)
	if name == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("role name requerido"))
		return
	}

	role, err := c.service.GetRole(ctx, tda, name)
	if err != nil {
		log.Error("get role failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	writeJSON(w, http.StatusOK, role)
}

// CreateRole maneja POST /v2/admin/rbac/roles
func (c *RBACController) CreateRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.CreateRole"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	var req dto.CreateRoleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	if req.Name == "" {
		httperrors.WriteError(w, httperrors.ErrMissingFields.WithDetail("name requerido"))
		return
	}

	role, err := c.service.CreateRole(ctx, tda, req)
	if err != nil {
		log.Error("create role failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	log.Info("role created", logger.String("role", req.Name))
	writeJSON(w, http.StatusCreated, role)
}

// UpdateRoleByName maneja PUT /v2/admin/rbac/roles/{name}
func (c *RBACController) UpdateRoleByName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.UpdateRoleByName"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	name := extractRoleNameFromPath(r.URL.Path)
	if name == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("role name requerido"))
		return
	}

	var req dto.UpdateRoleRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req); err != nil {
		httperrors.WriteError(w, httperrors.ErrInvalidJSON)
		return
	}

	role, err := c.service.UpdateRole(ctx, tda, name, req)
	if err != nil {
		log.Error("update role failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	log.Info("role updated", logger.String("role", name))
	writeJSON(w, http.StatusOK, role)
}

// DeleteRoleByName maneja DELETE /v2/admin/rbac/roles/{name}
func (c *RBACController) DeleteRoleByName(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := logger.From(ctx).With(logger.Layer("controller"), logger.Op("RBACController.DeleteRoleByName"))

	tda := mw.GetTenant(ctx)
	if tda == nil {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail(tenantRequired))
		return
	}

	name := extractRoleNameFromPath(r.URL.Path)
	if name == "" {
		httperrors.WriteError(w, httperrors.ErrBadRequest.WithDetail("role name requerido"))
		return
	}

	if err := c.service.DeleteRole(ctx, tda, name); err != nil {
		log.Error("delete role failed", logger.Err(err))
		httperrors.WriteError(w, mapRBACError(err))
		return
	}

	log.Info("role deleted", logger.String("role", name))
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

// ListPermissions maneja GET /v2/admin/rbac/permissions
func (c *RBACController) ListPermissions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	permissions := c.service.ListPermissions(ctx)

	writeJSON(w, http.StatusOK, map[string]any{"permissions": permissions})
}
