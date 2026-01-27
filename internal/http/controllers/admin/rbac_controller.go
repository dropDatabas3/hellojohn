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
	default:
		return httperrors.ErrInternalServerError.WithCause(err)
	}
}
