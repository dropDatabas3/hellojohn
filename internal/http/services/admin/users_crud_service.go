package admin

import (
	"context"
	"fmt"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/admin"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

// UserCRUDService maneja las operaciones CRUD de usuarios.
type UserCRUDService interface {
	Create(ctx context.Context, tenantID string, req dto.CreateUserRequest) (*dto.UserResponse, error)
	List(ctx context.Context, tenantID string, page, pageSize int, search string) (*dto.ListUsersResponse, error)
	Get(ctx context.Context, tenantID, userID string) (*dto.UserResponse, error)
	Update(ctx context.Context, tenantID, userID string, req dto.UpdateUserRequest) error
	Delete(ctx context.Context, tenantID, userID string) error
}

// UserCRUDDeps contiene las dependencias del service.
type UserCRUDDeps struct {
	DAL store.DataAccessLayer
}

type userCRUDService struct {
	deps UserCRUDDeps
}

// NewUserCRUDService crea una nueva instancia del servicio.
func NewUserCRUDService(deps UserCRUDDeps) UserCRUDService {
	return &userCRUDService{deps: deps}
}

// Errores del servicio
var (
	ErrUserInvalidInput   = fmt.Errorf("invalid user input")
	ErrUserNotFound       = fmt.Errorf("user not found")
	ErrUserEmailDuplicate = fmt.Errorf("email already exists")
	ErrUserTenantNotFound = fmt.Errorf("tenant not found")
	ErrUserTenantNoDB     = fmt.Errorf("tenant has no database configured")
)

// Create crea un nuevo usuario en el tenant.
func (s *userCRUDService) Create(ctx context.Context, tenantID string, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	log := logger.From(ctx)

	// 1. Validación básica
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	if req.Email == "" {
		return nil, fmt.Errorf("%w: email is required", ErrUserInvalidInput)
	}
	if req.Password == "" {
		return nil, fmt.Errorf("%w: password is required", ErrUserInvalidInput)
	}

	// 2. Validar política de contraseña
	policy := password.Policy{MinLength: 8}
	if ok, reasons := policy.Validate(req.Password); !ok {
		return nil, fmt.Errorf("%w: password policy violation: %v", ErrUserInvalidInput, reasons)
	}

	// 3. Hash de la contraseña usando Argon2id
	passwordHash, err := password.Hash(password.Default, req.Password)
	if err != nil {
		log.Error("failed to hash password", logger.Err(err))
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 4. Obtener acceso al tenant
	tda, err := s.deps.DAL.ForTenant(ctx, tenantID)
	if err != nil {
		if store.IsTenantNotFound(err) {
			return nil, ErrUserTenantNotFound
		}
		return nil, err
	}

	// 5. Verificar que tenant tenga DB (Data Plane)
	if err := tda.RequireDB(); err != nil {
		log.Warn("tenant has no database", logger.TenantID(tenantID))
		return nil, ErrUserTenantNoDB
	}

	// 6. Crear usuario
	user, _, err := tda.Users().Create(ctx, repository.CreateUserInput{
		TenantID:       tda.ID(),
		Email:          req.Email,
		PasswordHash:   passwordHash,
		Name:           req.Name,
		GivenName:      req.GivenName,
		FamilyName:     req.FamilyName,
		Picture:        req.Picture,
		Locale:         req.Locale,
		CustomFields:   req.CustomFields,
		SourceClientID: req.SourceClientID,
	})
	if err != nil {
		if repository.IsConflict(err) {
			return nil, ErrUserEmailDuplicate
		}
		log.Error("failed to create user", logger.Err(err), logger.Email(req.Email))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	log.Info("user created", logger.UserID(user.ID), logger.Email(user.Email))

	return mapUserToResponse(user), nil
}

// List lista los usuarios del tenant con paginación.
func (s *userCRUDService) List(ctx context.Context, tenantID string, page, pageSize int, search string) (*dto.ListUsersResponse, error) {
	log := logger.From(ctx)

	// 1. Validación de paginación
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 200 {
		pageSize = 200
	}

	// 2. Obtener acceso al tenant
	tda, err := s.deps.DAL.ForTenant(ctx, tenantID)
	if err != nil {
		if store.IsTenantNotFound(err) {
			return nil, ErrUserTenantNotFound
		}
		return nil, err
	}

	// 3. Verificar que tenant tenga DB
	if err := tda.RequireDB(); err != nil {
		log.Warn("tenant has no database", logger.TenantID(tenantID))
		return nil, ErrUserTenantNoDB
	}

	// 4. Listar usuarios
	offset := (page - 1) * pageSize
	users, err := tda.Users().List(ctx, tda.ID(), repository.ListUsersFilter{
		Limit:  pageSize,
		Offset: offset,
		Search: strings.TrimSpace(search),
	})
	if err != nil {
		log.Error("failed to list users", logger.Err(err))
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	// 5. Mapear a DTOs
	userResponses := make([]dto.UserResponse, len(users))
	for i, user := range users {
		userResponses[i] = *mapUserToResponse(&user)
	}

	log.Info("users listed", logger.Count(len(users)), logger.Int("page", page), logger.Int("page_size", pageSize))

	return &dto.ListUsersResponse{
		Users:      userResponses,
		TotalCount: len(users), // TODO: En el futuro, obtener count total de la DB
		Page:       page,
		PageSize:   pageSize,
	}, nil
}

// Get obtiene un usuario específico del tenant.
func (s *userCRUDService) Get(ctx context.Context, tenantID, userID string) (*dto.UserResponse, error) {
	log := logger.From(ctx)

	// 1. Validación
	if userID == "" {
		return nil, fmt.Errorf("%w: user_id is required", ErrUserInvalidInput)
	}

	// 2. Obtener acceso al tenant
	tda, err := s.deps.DAL.ForTenant(ctx, tenantID)
	if err != nil {
		if store.IsTenantNotFound(err) {
			return nil, ErrUserTenantNotFound
		}
		return nil, err
	}

	// 3. Verificar que tenant tenga DB
	if err := tda.RequireDB(); err != nil {
		log.Warn("tenant has no database", logger.TenantID(tenantID))
		return nil, ErrUserTenantNoDB
	}

	// 4. Obtener usuario
	user, err := tda.Users().GetByID(ctx, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, ErrUserNotFound
		}
		log.Error("failed to get user", logger.Err(err), logger.UserID(userID))
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	log.Info("user retrieved", logger.UserID(user.ID))

	return mapUserToResponse(user), nil
}

// Update actualiza los datos de un usuario.
func (s *userCRUDService) Update(ctx context.Context, tenantID, userID string, req dto.UpdateUserRequest) error {
	log := logger.From(ctx)

	// 1. Validación
	if userID == "" {
		return fmt.Errorf("%w: user_id is required", ErrUserInvalidInput)
	}

	// 2. Obtener acceso al tenant
	tda, err := s.deps.DAL.ForTenant(ctx, tenantID)
	if err != nil {
		if store.IsTenantNotFound(err) {
			return ErrUserTenantNotFound
		}
		return err
	}

	// 3. Verificar que tenant tenga DB
	if err := tda.RequireDB(); err != nil {
		log.Warn("tenant has no database", logger.TenantID(tenantID))
		return ErrUserTenantNoDB
	}

	// 4. Actualizar usuario
	var customFields map[string]any
	if req.CustomFields != nil {
		customFields = *req.CustomFields
	}

	err = tda.Users().Update(ctx, userID, repository.UpdateUserInput{
		Name:           req.Name,
		GivenName:      req.GivenName,
		FamilyName:     req.FamilyName,
		Picture:        req.Picture,
		Locale:         req.Locale,
		SourceClientID: req.SourceClientID,
		CustomFields:   customFields,
	})
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		log.Error("failed to update user", logger.Err(err), logger.UserID(userID))
		return fmt.Errorf("failed to update user: %w", err)
	}

	log.Info("user updated", logger.UserID(userID))

	return nil
}

// Delete elimina un usuario del tenant.
func (s *userCRUDService) Delete(ctx context.Context, tenantID, userID string) error {
	log := logger.From(ctx)

	// 1. Validación
	if userID == "" {
		return fmt.Errorf("%w: user_id is required", ErrUserInvalidInput)
	}

	// 2. Obtener acceso al tenant
	tda, err := s.deps.DAL.ForTenant(ctx, tenantID)
	if err != nil {
		if store.IsTenantNotFound(err) {
			return ErrUserTenantNotFound
		}
		return err
	}

	// 3. Verificar que tenant tenga DB
	if err := tda.RequireDB(); err != nil {
		log.Warn("tenant has no database", logger.TenantID(tenantID))
		return ErrUserTenantNoDB
	}

	// 4. Eliminar usuario
	err = tda.Users().Delete(ctx, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return ErrUserNotFound
		}
		log.Error("failed to delete user", logger.Err(err), logger.UserID(userID))
		return fmt.Errorf("failed to delete user: %w", err)
	}

	log.Info("user deleted", logger.UserID(userID))

	return nil
}

// mapUserToResponse convierte un repository.User a dto.UserResponse.
func mapUserToResponse(user *repository.User) *dto.UserResponse {
	return &dto.UserResponse{
		ID:             user.ID,
		TenantID:       user.TenantID,
		Email:          user.Email,
		Name:           user.Name,
		GivenName:      user.GivenName,
		FamilyName:     user.FamilyName,
		Picture:        user.Picture,
		Locale:         user.Locale,
		EmailVerified:  user.EmailVerified,
		SourceClientID: user.SourceClientID,
		CreatedAt:      user.CreatedAt,
		DisabledAt:     user.DisabledAt,
		CustomFields:   user.CustomFields,
	}
}
