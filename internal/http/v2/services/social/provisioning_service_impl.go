package social

import (
	"context"
	"fmt"

	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProvisioningDeps contains dependencies for provisioning service.
type ProvisioningDeps struct {
	// TenantDBProvider is optional - if nil, uses control plane for tenant lookup
}

// provisioningService implements ProvisioningService.
type provisioningService struct{}

// NewProvisioningService creates a new ProvisioningService.
func NewProvisioningService(d ProvisioningDeps) ProvisioningService {
	return &provisioningService{}
}

// EnsureUserAndIdentity creates or updates a user from social login claims.
func (s *provisioningService) EnsureUserAndIdentity(ctx context.Context, tenantSlug, provider string, claims *OIDCClaims) (string, error) {
	log := logger.From(ctx).With(logger.Layer("service"), logger.Component("social.provisioning"))

	// Validate email
	if claims == nil || claims.Email == "" {
		return "", ErrProvisioningEmailMissing
	}

	// Get tenant from control plane
	if cpctx.Provider == nil {
		log.Error("control plane not initialized")
		return "", ErrProvisioningDBRequired
	}

	tenant, err := cpctx.Provider.GetTenantBySlug(ctx, tenantSlug)
	if err != nil {
		log.Error("tenant not found", logger.Err(err), logger.TenantID(tenantSlug))
		return "", fmt.Errorf("%w: tenant not found", ErrProvisioningDBRequired)
	}

	// Get tenant DB connection - DSN is in Settings.UserDB
	if tenant.Settings.UserDB == nil {
		log.Error("tenant has no UserDB configured", logger.TenantID(tenantSlug))
		return "", fmt.Errorf("%w: no UserDB for tenant", ErrProvisioningDBRequired)
	}
	dsn := tenant.Settings.UserDB.DSN
	if dsn == "" {
		log.Error("tenant has no DSN configured", logger.TenantID(tenantSlug))
		return "", fmt.Errorf("%w: no DSN for tenant", ErrProvisioningDBRequired)
	}

	// Connect to tenant DB
	// Connect to tenant DB via PoolManager
	pool, err := DefaultPoolManager.GetPool(ctx, dsn)
	if err != nil {
		log.Error("failed to get tenant DB pool", logger.Err(err), logger.TenantID(tenantSlug))
		return "", fmt.Errorf("%w: db connection failed", ErrProvisioningDBRequired)
	}
	// DO NOT close pool here, it is managed

	// Run provisioning
	userID, err := s.ensureUserAndIdentity(ctx, pool, provider, claims)
	if err != nil {
		log.Error("provisioning failed",
			logger.String("provider", provider),
			logger.TenantID(tenantSlug),
			logger.Err(err),
		)
		return "", fmt.Errorf("%w: %v", ErrProvisioningFailed, err)
	}

	log.Info("user provisioned",
		logger.String("provider", provider),
		logger.TenantID(tenantSlug),
		logger.String("user_id", userID),
		logger.String("email_masked", maskEmail(claims.Email)),
	)

	return userID, nil
}

// ensureUserAndIdentity: upsert app_user + identity(provider, sub)
func (s *provisioningService) ensureUserAndIdentity(ctx context.Context, pool *pgxpool.Pool, provider string, claims *OIDCClaims) (string, error) {
	log := logger.From(ctx).With(logger.Component("social.provisioning"))

	var userID string
	var emailVerified bool

	// 1) Try to find existing user by email
	qSelect := `SELECT id, email_verified FROM app_user WHERE email=$1 LIMIT 1`
	err := pool.QueryRow(ctx, qSelect, claims.Email).Scan(&userID, &emailVerified)

	if err != nil {
		if err != pgx.ErrNoRows {
			return "", fmt.Errorf("select user: %w", err)
		}

		// User doesn't exist - create them
		qInsert := `INSERT INTO app_user (email, email_verified, name, given_name, family_name, picture, locale, metadata)
			VALUES ($1,$2,$3,$4,$5,$6,$7,'{}'::jsonb) RETURNING id`

		err = pool.QueryRow(ctx, qInsert,
			claims.Email,
			claims.EmailVerified,
			claims.Name,
			claims.GivenName,
			claims.FamilyName,
			claims.Picture,
			claims.Locale,
		).Scan(&userID)

		if err != nil {
			return "", fmt.Errorf("insert user: %w", err)
		}

		log.Debug("user created",
			logger.String("user_id", userID),
			logger.String("email_masked", maskEmail(claims.Email)),
		)
	} else {
		// User exists - update verification if needed
		if claims.EmailVerified && !emailVerified {
			_, _ = pool.Exec(ctx, `UPDATE app_user SET email_verified=true WHERE id=$1`, userID)
			log.Debug("user email_verified updated", logger.String("user_id", userID))
		}
	}

	// 2) Ensure identity(provider, provider_user_id) exists
	var idExists bool
	err = pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM identity WHERE provider=$1 AND provider_user_id=$2 AND user_id=$3)
	`, provider, claims.Sub, userID).Scan(&idExists)

	if err != nil {
		return "", fmt.Errorf("check identity: %w", err)
	}

	if !idExists {
		_, err = pool.Exec(ctx, `
			INSERT INTO identity (user_id, provider, provider_user_id, email, email_verified)
			VALUES ($1,$2,$3,$4,$5)
		`, userID, provider, claims.Sub, claims.Email, claims.EmailVerified)

		if err != nil {
			return "", fmt.Errorf("insert identity: %w", err)
		}

		log.Debug("identity created",
			logger.String("user_id", userID),
			logger.String("provider", provider),
			logger.String("sub", claims.Sub),
		)
	}

	return userID, nil
}

// maskEmail masks an email for logging (shows first 2 chars + @domain)
func maskEmail(email string) string {
	if len(email) < 3 {
		return "***"
	}
	at := -1
	for i, c := range email {
		if c == '@' {
			at = i
			break
		}
	}
	if at < 0 || at < 2 {
		return email[:2] + "***"
	}
	return email[:2] + "***" + email[at:]
}
