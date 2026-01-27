// Package admin provee servicios para operaciones administrativas HTTP V2.
package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	dto "github.com/dropDatabas3/hellojohn/internal/http/dto/admin"
	"github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
)

// AuthService define las operaciones de autenticación para admins.
type AuthService interface {
	Login(ctx context.Context, req dto.AdminLoginRequest) (*dto.AdminLoginResult, error)
	Refresh(ctx context.Context, req dto.AdminRefreshRequest) (*dto.AdminLoginResult, error)
}

// authService implementa AuthService.
type authService struct {
	cp         controlplane.Service
	issuer     *jwt.Issuer
	refreshTTL time.Duration
}

// AuthServiceDeps contiene las dependencias para el servicio de autenticación de admins.
type AuthServiceDeps struct {
	ControlPlane controlplane.Service
	Issuer       *jwt.Issuer
	RefreshTTL   time.Duration
}

// NewAuthService crea un nuevo servicio de autenticación de admins.
func NewAuthService(deps AuthServiceDeps) AuthService {
	if deps.RefreshTTL == 0 {
		deps.RefreshTTL = 30 * 24 * time.Hour // 30 días por defecto
	}
	return &authService{
		cp:         deps.ControlPlane,
		issuer:     deps.Issuer,
		refreshTTL: deps.RefreshTTL,
	}
}

// Errores del servicio de autenticación
var (
	ErrInvalidAdminCredentials = fmt.Errorf("invalid admin credentials")
	ErrAdminDisabled           = fmt.Errorf("admin account disabled")
	ErrInvalidRefreshToken     = fmt.Errorf("invalid refresh token")
	ErrRefreshTokenExpired     = fmt.Errorf("refresh token expired")
)

// Login autentica un admin con email y password.
func (s *authService) Login(ctx context.Context, req dto.AdminLoginRequest) (*dto.AdminLoginResult, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("admin.auth"),
		logger.Op("Login"),
		logger.String("email", req.Email),
	)

	// 1. Buscar admin en Control Plane
	admin, err := s.cp.GetAdminByEmail(ctx, req.Email)
	if err != nil {
		if repository.IsNotFound(err) {
			log.Warn("admin not found")
			return nil, ErrInvalidAdminCredentials
		}
		log.Error("failed to get admin", logger.Err(err))
		return nil, err
	}

	// 2. Verificar password usando Argon2id
	if !s.cp.CheckAdminPassword(admin.PasswordHash, req.Password) {
		log.Warn("invalid password")
		return nil, ErrInvalidAdminCredentials
	}

	// 3. Verificar que no esté deshabilitado
	if admin.DisabledAt != nil {
		log.Warn("admin disabled")
		return nil, ErrAdminDisabled
	}

	// 4. Emitir access token
	accessToken, expiresIn, err := s.issuer.IssueAdminAccess(ctx, jwt.AdminAccessClaims{
		AdminID:   admin.ID,
		Email:     admin.Email,
		AdminType: string(admin.Type),
		Tenants:   admin.AssignedTenants,
	})
	if err != nil {
		log.Error("failed to issue access token", logger.Err(err))
		return nil, fmt.Errorf("failed to issue access token: %w", err)
	}

	// 5. Generar refresh token opaco
	refreshToken := generateOpaqueToken()

	// 6. Guardar refresh token en Control Plane
	err = s.cp.CreateAdminRefreshToken(ctx, controlplane.AdminRefreshTokenInput{
		AdminID:   admin.ID,
		TokenHash: hashToken(refreshToken),
		ExpiresAt: time.Now().Add(s.refreshTTL),
	})
	if err != nil {
		log.Error("failed to create refresh token", logger.Err(err))
		return nil, fmt.Errorf("failed to create refresh token: %w", err)
	}

	log.Debug("refresh token persisted")

	log.Info("admin logged in successfully")

	return &dto.AdminLoginResult{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		TokenType:    "Bearer",
		Admin: dto.AdminInfo{
			ID:      admin.ID,
			Email:   admin.Email,
			Type:    string(admin.Type),
			Tenants: admin.AssignedTenants,
		},
	}, nil
}

// Refresh renueva el access token usando un refresh token válido.
func (s *authService) Refresh(ctx context.Context, req dto.AdminRefreshRequest) (*dto.AdminLoginResult, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("admin.auth"),
		logger.Op("Refresh"),
	)

	// 1. Verificar refresh token en Control Plane
	tokenHash := hashToken(req.RefreshToken)
	adminRefresh, err := s.cp.GetAdminRefreshToken(ctx, tokenHash)
	if err != nil {
		if repository.IsNotFound(err) {
			log.Warn("refresh token not found")
			return nil, ErrInvalidRefreshToken
		}
		log.Error("failed to get refresh token", logger.Err(err))
		return nil, err
	}

	// 2. Verificar expiración
	if time.Now().After(adminRefresh.ExpiresAt) {
		log.Warn("refresh token expired")
		return nil, ErrRefreshTokenExpired
	}

	// 3. Obtener admin
	admin, err := s.cp.GetAdmin(ctx, adminRefresh.AdminID)
	if err != nil {
		log.Error("failed to get admin", logger.Err(err))
		return nil, err
	}

	// 4. Verificar que no esté deshabilitado
	if admin.DisabledAt != nil {
		log.Warn("admin disabled")
		return nil, ErrAdminDisabled
	}

	// 5. Emitir nuevo access token
	accessToken, expiresIn, err := s.issuer.IssueAdminAccess(ctx, jwt.AdminAccessClaims{
		AdminID:   admin.ID,
		Email:     admin.Email,
		AdminType: string(admin.Type),
		Tenants:   admin.AssignedTenants,
	})
	if err != nil {
		log.Error("failed to issue access token", logger.Err(err))
		return nil, fmt.Errorf("failed to issue access token: %w", err)
	}

	log.Info("admin token refreshed successfully")

	return &dto.AdminLoginResult{
		AccessToken:  accessToken,
		RefreshToken: req.RefreshToken, // Reutilizar el mismo refresh token
		ExpiresIn:    expiresIn,
		TokenType:    "Bearer",
		Admin: dto.AdminInfo{
			ID:      admin.ID,
			Email:   admin.Email,
			Type:    string(admin.Type),
			Tenants: admin.AssignedTenants,
		},
	}, nil
}

// generateOpaqueToken genera un token opaco aleatorio.
func generateOpaqueToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// Note: hashToken() ya existe en users_service.go (SHA-256) - reutilizamos esa función
