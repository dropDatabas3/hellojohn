package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
)

// RBACService define las operaciones RBAC para el admin API.
type RBACService interface {
	GetUserRoles(ctx context.Context, tda store.TenantDataAccess, userID string) ([]string, error)
	UpdateUserRoles(ctx context.Context, tda store.TenantDataAccess, userID string, add, remove []string) ([]string, error)
	GetRolePerms(ctx context.Context, tda store.TenantDataAccess, role string) ([]string, error)
	UpdateRolePerms(ctx context.Context, tda store.TenantDataAccess, role string, add, remove []string) ([]string, error)
}

// RBACReader es la interfaz para leer roles RBAC.
type RBACReader interface {
	GetUserRoles(ctx context.Context, userID string) ([]string, error)
}

// RBACWriter es la interfaz para escribir roles/permisos RBAC.
type RBACWriter interface {
	AssignUserRoles(ctx context.Context, userID string, add []string) error
	RemoveUserRoles(ctx context.Context, userID string, remove []string) error
	GetRolePerms(ctx context.Context, tenantID, role string) ([]string, error)
	AddRolePerms(ctx context.Context, tenantID, role string, add []string) error
	RemoveRolePerms(ctx context.Context, tenantID, role string, remove []string) error
}

// rbacService implementa RBACService.
type rbacService struct{}

// NewRBACService crea un nuevo servicio RBAC.
func NewRBACService() RBACService {
	return &rbacService{}
}

const componentRBAC = "admin.rbac"

var (
	ErrRBACNotSupported = fmt.Errorf("RBAC not supported by store")
)

func (s *rbacService) GetUserRoles(ctx context.Context, tda store.TenantDataAccess, userID string) ([]string, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentRBAC),
		logger.Op("GetUserRoles"),
		logger.UserID(userID),
	)

	if err := tda.RequireDB(); err != nil {
		return nil, err
	}

	// Intentar obtener RBACReader del TDA
	reader, ok := tda.(RBACReader)
	if !ok {
		log.Warn("store does not support RBAC read")
		return nil, ErrRBACNotSupported
	}

	roles, err := reader.GetUserRoles(ctx, userID)
	if err != nil {
		log.Error("failed to get user roles", logger.Err(err))
		return nil, err
	}

	return roles, nil
}

func (s *rbacService) UpdateUserRoles(ctx context.Context, tda store.TenantDataAccess, userID string, add, remove []string) ([]string, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentRBAC),
		logger.Op("UpdateUserRoles"),
		logger.UserID(userID),
	)

	if err := tda.RequireDB(); err != nil {
		return nil, err
	}

	// Intentar obtener RBACWriter del TDA
	writer, ok := tda.(RBACWriter)
	if !ok {
		log.Warn("store does not support RBAC write")
		return nil, ErrRBACNotSupported
	}

	// Normalizar y deduplicar
	add = dedupTrim(add)
	remove = dedupTrim(remove)

	if len(add) > 0 {
		if err := writer.AssignUserRoles(ctx, userID, add); err != nil {
			log.Error("failed to assign roles", logger.Err(err))
			return nil, err
		}
	}

	if len(remove) > 0 {
		if err := writer.RemoveUserRoles(ctx, userID, remove); err != nil {
			log.Error("failed to remove roles", logger.Err(err))
			return nil, err
		}
	}

	// Re-leer estado final
	reader, ok := tda.(RBACReader)
	if !ok {
		return nil, ErrRBACNotSupported
	}

	return reader.GetUserRoles(ctx, userID)
}

func (s *rbacService) GetRolePerms(ctx context.Context, tda store.TenantDataAccess, role string) ([]string, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentRBAC),
		logger.Op("GetRolePerms"),
		logger.String("role", role),
	)

	if err := tda.RequireDB(); err != nil {
		return nil, err
	}

	writer, ok := tda.(RBACWriter)
	if !ok {
		log.Warn("store does not support RBAC write")
		return nil, ErrRBACNotSupported
	}

	perms, err := writer.GetRolePerms(ctx, tda.ID(), role)
	if err != nil {
		log.Error("failed to get role perms", logger.Err(err))
		return nil, err
	}

	return perms, nil
}

func (s *rbacService) UpdateRolePerms(ctx context.Context, tda store.TenantDataAccess, role string, add, remove []string) ([]string, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentRBAC),
		logger.Op("UpdateRolePerms"),
		logger.String("role", role),
	)

	if err := tda.RequireDB(); err != nil {
		return nil, err
	}

	writer, ok := tda.(RBACWriter)
	if !ok {
		log.Warn("store does not support RBAC write")
		return nil, ErrRBACNotSupported
	}

	// Normalizar y deduplicar
	add = dedupTrim(add)
	remove = dedupTrim(remove)

	tenantID := tda.ID()

	if len(add) > 0 {
		if err := writer.AddRolePerms(ctx, tenantID, role, add); err != nil {
			log.Error("failed to add perms", logger.Err(err))
			return nil, err
		}
	}

	if len(remove) > 0 {
		if err := writer.RemoveRolePerms(ctx, tenantID, role, remove); err != nil {
			log.Error("failed to remove perms", logger.Err(err))
			return nil, err
		}
	}

	// Re-leer estado final
	return writer.GetRolePerms(ctx, tenantID, role)
}

// dedupTrim aplica trim, elimina vacíos y deduplica.
func dedupTrim(in []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(in))
	for _, v := range in {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}
