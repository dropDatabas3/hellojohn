package admin

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	emailv2 "github.com/dropDatabas3/hellojohn/internal/email"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	"github.com/dropDatabas3/hellojohn/internal/security/password"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

// UserActionService define operaciones administrativas sobre usuarios.
type UserActionService interface {
	Disable(ctx context.Context, tda store.TenantDataAccess, userID, reason string, duration time.Duration, actor string) error
	Enable(ctx context.Context, tda store.TenantDataAccess, userID, actor string) error
	ResendVerification(ctx context.Context, tda store.TenantDataAccess, userID, actor string) error
	SetEmailVerified(ctx context.Context, tda store.TenantDataAccess, userID string, verified bool, actor string) error
	SetPassword(ctx context.Context, tda store.TenantDataAccess, userID, newPassword, actor string) error
}

// userActionService implementa UserActionService.
type userActionService struct {
	emailSvc emailv2.Service
}

// NewUserActionService crea un nuevo service de acciones de usuarios.
func NewUserActionService(emailSvc emailv2.Service) UserActionService {
	return &userActionService{emailSvc: emailSvc}
}

const (
	componentUserAction = "admin.users"
	errUsersRepoNil     = "users repository not available"
)

func (s *userActionService) Disable(ctx context.Context, tda store.TenantDataAccess, userID, reason string, duration time.Duration, actor string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentUserAction),
		logger.Op("Disable"),
		logger.UserID(userID),
	)

	if err := tda.RequireDB(); err != nil {
		return err
	}

	users := tda.Users()
	if users == nil {
		return fmt.Errorf(errUsersRepoNil)
	}

	// Calcular until si hay duración
	var until *time.Time
	if duration > 0 {
		t := time.Now().Add(duration)
		until = &t
	}

	// Disable user
	if err := users.Disable(ctx, userID, actor, reason, until); err != nil {
		log.Error("disable failed", logger.Err(err))
		return err
	}

	// Revocar tokens (best-effort)
	if tokens := tda.Tokens(); tokens != nil {
		if _, err := tokens.RevokeAllByUser(ctx, userID, ""); err != nil {
			log.Warn("best-effort token revocation failed", logger.Err(err))
		}
	}

	// Enviar email notificación (best-effort)
	go s.sendBlockNotification(ctx, tda, userID, reason, until)

	log.Info("user disabled")
	return nil
}

func (s *userActionService) Enable(ctx context.Context, tda store.TenantDataAccess, userID, actor string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentUserAction),
		logger.Op("Enable"),
		logger.UserID(userID),
	)

	if err := tda.RequireDB(); err != nil {
		return err
	}

	users := tda.Users()
	if users == nil {
		return fmt.Errorf(errUsersRepoNil)
	}

	if err := users.Enable(ctx, userID, actor); err != nil {
		log.Error("enable failed", logger.Err(err))
		return err
	}

	// Enviar email notificación (best-effort)
	go s.sendUnblockNotification(ctx, tda, userID)

	log.Info("user enabled")
	return nil
}

func (s *userActionService) ResendVerification(ctx context.Context, tda store.TenantDataAccess, userID, actor string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentUserAction),
		logger.Op("ResendVerification"),
		logger.UserID(userID),
	)

	if err := tda.RequireDB(); err != nil {
		return err
	}

	users := tda.Users()
	if users == nil {
		return fmt.Errorf(errUsersRepoNil)
	}

	// Obtener usuario
	user, err := users.GetByID(ctx, userID)
	if err != nil {
		log.Error("user not found", logger.Err(err))
		return err
	}

	if user.EmailVerified {
		return fmt.Errorf("email already verified")
	}

	// Crear token de verificación
	emailTokens := tda.EmailTokens()
	if emailTokens == nil {
		return fmt.Errorf("email tokens not available")
	}

	// Generar token random
	rawToken := generateSecureToken()
	tokenHash := hashToken(rawToken)

	verifyTTL := 48 * time.Hour
	input := repository.CreateEmailTokenInput{
		TenantID:   tda.ID(),
		UserID:     userID,
		Email:      user.Email,
		Type:       repository.EmailTokenVerification,
		TokenHash:  tokenHash,
		TTLSeconds: int(verifyTTL.Seconds()),
	}

	if _, err := emailTokens.Create(ctx, input); err != nil {
		log.Error("token creation failed", logger.Err(err))
		return err
	}

	// Construir link con el token plain (no hash)
	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	link := baseURL + "/v2/auth/verify-email?token=" + rawToken + "&tenant_id=" + tda.ID()
	if user.SourceClientID != nil && *user.SourceClientID != "" {
		link += "&client_id=" + *user.SourceClientID
	}

	// Enviar email usando emailv2.Service
	if s.emailSvc != nil {
		req := emailv2.SendVerificationRequest{
			TenantSlugOrID: tda.ID(),
			UserID:         userID,
			Email:          user.Email,
			Token:          rawToken,
			TTL:            verifyTTL,
		}
		if err := s.emailSvc.SendVerificationEmail(ctx, req); err != nil {
			log.Error("email send failed", logger.Err(err))
			return fmt.Errorf("email_error: %w", err)
		}
	}

	log.Info("verification email sent")
	return nil
}

func (s *userActionService) SetEmailVerified(ctx context.Context, tda store.TenantDataAccess, userID string, verified bool, actor string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentUserAction),
		logger.Op("SetEmailVerified"),
		logger.UserID(userID),
	)

	if err := tda.RequireDB(); err != nil {
		return err
	}

	users := tda.Users()
	if users == nil {
		return fmt.Errorf(errUsersRepoNil)
	}

	if err := users.SetEmailVerified(ctx, userID, verified); err != nil {
		log.Error("set email verified failed", logger.Err(err))
		return err
	}

	log.Info("email verified status changed", logger.Bool("verified", verified))
	return nil
}

func (s *userActionService) SetPassword(ctx context.Context, tda store.TenantDataAccess, userID, newPassword, actor string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentUserAction),
		logger.Op("SetPassword"),
		logger.UserID(userID),
	)

	if err := tda.RequireDB(); err != nil {
		return err
	}

	users := tda.Users()
	if users == nil {
		return fmt.Errorf(errUsersRepoNil)
	}

	// Validar longitud mínima
	if len(newPassword) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	// Obtener usuario para verificar que existe
	user, err := users.GetByID(ctx, userID)
	if err != nil {
		log.Error("user not found", logger.Err(err))
		return err
	}

	// Hash de la contraseña usando argon2id (mismo formato que en registro)
	hash, err := hashPasswordArgon2id(newPassword)
	if err != nil {
		log.Error("password hash failed", logger.Err(err))
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := users.UpdatePasswordHash(ctx, userID, hash); err != nil {
		log.Error("update password hash failed", logger.Err(err))
		return err
	}

	// Revocar todos los tokens activos del usuario
	if tokens := tda.Tokens(); tokens != nil {
		if _, err := tokens.RevokeAllByUser(ctx, userID, ""); err != nil {
			log.Warn("best-effort token revocation failed", logger.Err(err))
		}
	}

	// Enviar notificación por email (best-effort)
	go s.sendPasswordChangedNotification(ctx, tda, user.Email)

	log.Info("password changed by admin")
	return nil
}

func (s *userActionService) sendPasswordChangedNotification(ctx context.Context, tda store.TenantDataAccess, email string) {
	if s.emailSvc == nil || email == "" {
		return
	}

	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentUserAction),
		logger.Op("sendPasswordChangedNotification"),
	)

	req := emailv2.SendNotificationRequest{
		TenantSlugOrID: tda.ID(),
		Email:          email,
		TemplateID:     "password_changed",
	}
	if err := s.emailSvc.SendNotificationEmail(ctx, req); err != nil {
		log.Warn("notification email failed", logger.Err(err))
	}
}

// hashPasswordArgon2id genera un hash Argon2id para la contraseña.
func hashPasswordArgon2id(newPassword string) (string, error) {
	return password.Hash(password.Default, newPassword)
}

func (s *userActionService) sendBlockNotification(ctx context.Context, tda store.TenantDataAccess, userID, reason string, until *time.Time) {
	if s.emailSvc == nil {
		return
	}

	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentUserAction),
		logger.Op("sendBlockNotification"),
	)

	users := tda.Users()
	if users == nil {
		return
	}

	user, err := users.GetByID(ctx, userID)
	if err != nil || user.Email == "" {
		return
	}

	var untilStr string
	if until != nil {
		untilStr = until.Format("2006-01-02 15:04")
	}

	req := emailv2.SendNotificationRequest{
		TenantSlugOrID: tda.ID(),
		Email:          user.Email,
		TemplateID:     "user_blocked",
		TemplateVars:   map[string]any{"Reason": reason, "Until": untilStr},
	}
	if err := s.emailSvc.SendNotificationEmail(ctx, req); err != nil {
		log.Warn("notification email failed", logger.Err(err))
	}
}

func (s *userActionService) sendUnblockNotification(ctx context.Context, tda store.TenantDataAccess, userID string) {
	if s.emailSvc == nil {
		return
	}

	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component(componentUserAction),
		logger.Op("sendUnblockNotification"),
	)

	users := tda.Users()
	if users == nil {
		return
	}

	user, err := users.GetByID(ctx, userID)
	if err != nil || user.Email == "" {
		return
	}

	req := emailv2.SendNotificationRequest{
		TenantSlugOrID: tda.ID(),
		Email:          user.Email,
		TemplateID:     "user_unblocked",
	}
	if err := s.emailSvc.SendNotificationEmail(ctx, req); err != nil {
		log.Warn("notification email failed", logger.Err(err))
	}
}

// ─── Helpers ───

func generateSecureToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}
