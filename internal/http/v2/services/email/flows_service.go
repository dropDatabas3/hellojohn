package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	dto "github.com/dropDatabas3/hellojohn/internal/http/v2/dto/email"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	store "github.com/dropDatabas3/hellojohn/internal/store/v2"
	"go.uber.org/zap"
)

// FlowsService defines operations for email verification and password reset flows.
type FlowsService interface {
	// VerifyEmailStart initiates email verification flow.
	VerifyEmailStart(ctx context.Context, tda store.TenantDataAccess, req dto.VerifyEmailStartRequest, userID *string) error

	// VerifyEmailConfirm confirms email verification by consuming the token.
	VerifyEmailConfirm(ctx context.Context, tda store.TenantDataAccess, req dto.VerifyEmailConfirmRequest) (*dto.VerifyEmailResult, error)

	// ForgotPassword initiates password reset flow.
	ForgotPassword(ctx context.Context, tda store.TenantDataAccess, req dto.ForgotPasswordRequest) error

	// ResetPassword resets the password using the reset token.
	ResetPassword(ctx context.Context, tda store.TenantDataAccess, req dto.ResetPasswordRequest) (*dto.ResetPasswordResult, error)
}

// FlowsDeps contains dependencies for the email flows service.
type FlowsDeps struct {
	Sender SenderProvider
	Config FlowsConfig
}

// SenderProvider provides email sender by tenant.
type SenderProvider interface {
	GetSender(ctx context.Context, tenantID uuid.UUID) (Sender, error)
}

// Sender sends emails.
type Sender interface {
	Send(to, subject, htmlBody, textBody string) error
}

// FlowsConfig contains configuration for email flows.
type FlowsConfig struct {
	BaseURL         string
	VerifyEmailPath string // Default: /v2/auth/verify-email
	ResetPath       string // Default: /v2/auth/reset
}

type flowsService struct {
	sender SenderProvider
	config FlowsConfig
}

// NewFlowsService creates a new FlowsService.
func NewFlowsService(deps FlowsDeps) FlowsService {
	cfg := deps.Config
	if cfg.VerifyEmailPath == "" {
		cfg.VerifyEmailPath = "/v2/auth/verify-email"
	}
	if cfg.ResetPath == "" {
		cfg.ResetPath = "/v2/auth/reset"
	}
	return &flowsService{
		sender: deps.Sender,
		config: cfg,
	}
}

// Service errors
var (
	ErrFlowsMissingTenant   = fmt.Errorf("tenant_id is required")
	ErrFlowsMissingClient   = fmt.Errorf("client_id is required")
	ErrFlowsMissingEmail    = fmt.Errorf("email is required")
	ErrFlowsMissingToken    = fmt.Errorf("token is required")
	ErrFlowsMissingPassword = fmt.Errorf("new_password is required")
	ErrFlowsInvalidToken    = fmt.Errorf("token invalid or expired")
	ErrFlowsUserNotFound    = fmt.Errorf("user not found")
	ErrFlowsNoDatabase      = fmt.Errorf("database not available")
	ErrFlowsWeakPassword    = fmt.Errorf("password does not meet policy")
	ErrFlowsSendFailed      = fmt.Errorf("failed to send email")
)

// VerifyEmailStart initiates email verification flow.
// If userID is provided (authenticated), uses that. Otherwise looks up by email.
func (s *flowsService) VerifyEmailStart(ctx context.Context, tda store.TenantDataAccess, req dto.VerifyEmailStartRequest, userID *string) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("email.flows"),
		logger.Op("VerifyEmailStart"),
	)

	// Validate input
	if req.TenantID == "" && req.ClientID == "" {
		return ErrFlowsMissingTenant
	}
	if req.ClientID == "" {
		return ErrFlowsMissingClient
	}

	// Require database
	if err := tda.RequireDB(); err != nil {
		log.Debug("database not available", logger.Err(err))
		return ErrFlowsNoDatabase
	}

	var email string
	var uid uuid.UUID

	if userID != nil && *userID != "" {
		// Authenticated flow - get email from user
		uid, err := uuid.Parse(*userID)
		if err != nil {
			log.Debug("invalid user ID format", logger.Err(err))
			return ErrFlowsUserNotFound
		}
		usersRepo := tda.Users()
		user, err := usersRepo.GetByID(ctx, uid.String())
		if err != nil {
			log.Debug("user lookup failed", logger.Err(err))
			return ErrFlowsUserNotFound
		}
		email = user.Email
		_ = uid // uid available if needed for token creation
	} else {
		// Unauthenticated flow - lookup by email
		if req.Email == "" {
			return ErrFlowsMissingEmail
		}
		email = strings.TrimSpace(strings.ToLower(req.Email))

		tenantID := req.TenantID
		if tenantID == "" {
			tenantID = req.ClientID
		}

		usersRepo := tda.Users()
		user, _, err := usersRepo.GetByEmail(ctx, tenantID, email)
		if err != nil {
			// Anti-enumeration: don't reveal if user exists
			log.Debug("user lookup failed (anti-enum)", logger.Err(err))
			return nil // Silent success
		}
		uidParsed, _ := uuid.Parse(user.ID)
		uid = uidParsed
	}

	// TODO: Create verification token via Tokens repo
	// TODO: Build verification link
	// TODO: Send email via sender provider

	log.Debug("verify email start",
		zap.String("email", email),
		zap.String("user_id", uid.String()),
	)

	// For now, return success (token creation and email sending to be implemented)
	return nil
}

// VerifyEmailConfirm confirms email verification by consuming the token.
func (s *flowsService) VerifyEmailConfirm(ctx context.Context, tda store.TenantDataAccess, req dto.VerifyEmailConfirmRequest) (*dto.VerifyEmailResult, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("email.flows"),
		logger.Op("VerifyEmailConfirm"),
	)

	if req.Token == "" {
		return nil, ErrFlowsMissingToken
	}

	// Require database
	if err := tda.RequireDB(); err != nil {
		log.Debug("database not available", logger.Err(err))
		return nil, ErrFlowsNoDatabase
	}

	// TODO: Consume verification token via Tokens repo
	// TODO: Mark user email as verified
	// TODO: Resolve redirect URI

	log.Debug("verify email confirm",
		zap.String("token_prefix", req.Token[:min(8, len(req.Token))]),
	)

	// Placeholder result
	return &dto.VerifyEmailResult{
		Verified: true,
		Redirect: req.RedirectURI,
	}, nil
}

// ForgotPassword initiates password reset flow.
func (s *flowsService) ForgotPassword(ctx context.Context, tda store.TenantDataAccess, req dto.ForgotPasswordRequest) error {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("email.flows"),
		logger.Op("ForgotPassword"),
	)

	// Validate input
	if req.TenantID == "" {
		return ErrFlowsMissingTenant
	}
	if req.ClientID == "" {
		return ErrFlowsMissingClient
	}
	if req.Email == "" {
		return ErrFlowsMissingEmail
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))

	// Require database
	if err := tda.RequireDB(); err != nil {
		log.Debug("database not available", logger.Err(err))
		return ErrFlowsNoDatabase
	}

	// Lookup user (anti-enumeration: always return success)
	usersRepo := tda.Users()
	_, _, err := usersRepo.GetByEmail(ctx, req.TenantID, email)
	if err != nil {
		log.Debug("user lookup failed (anti-enum)")
		return nil // Silent success
	}

	// TODO: Create password reset token
	// TODO: Build reset link
	// TODO: Send email via sender provider

	log.Debug("forgot password",
		zap.String("email", email),
	)

	return nil
}

// ResetPassword resets the password using the reset token.
func (s *flowsService) ResetPassword(ctx context.Context, tda store.TenantDataAccess, req dto.ResetPasswordRequest) (*dto.ResetPasswordResult, error) {
	log := logger.From(ctx).With(
		logger.Layer("service"),
		logger.Component("email.flows"),
		logger.Op("ResetPassword"),
	)

	// Validate input
	if req.TenantID == "" {
		return nil, ErrFlowsMissingTenant
	}
	if req.ClientID == "" {
		return nil, ErrFlowsMissingClient
	}
	if req.Token == "" {
		return nil, ErrFlowsMissingToken
	}
	if req.NewPassword == "" {
		return nil, ErrFlowsMissingPassword
	}

	// Require database
	if err := tda.RequireDB(); err != nil {
		log.Debug("database not available", logger.Err(err))
		return nil, ErrFlowsNoDatabase
	}

	// TODO: Validate password policy
	// TODO: Consume password reset token
	// TODO: Update password hash
	// TODO: Revoke all refresh tokens
	// TODO: Optional auto-login

	log.Debug("reset password",
		zap.String("token_prefix", req.Token[:min(8, len(req.Token))]),
	)

	// Placeholder result
	return &dto.ResetPasswordResult{
		AutoLogin: false,
	}, nil
}
