package core

import (
	"context"
	"time"
)

// TenantClientAware define métodos que soportan el nuevo esquema tenant+client
// sin Foreign Keys, para migración progresiva del data-plane.
type TenantClientAware interface {
	// Refresh tokens con tenant+client directo
	CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error)
	GetRefreshTokenByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (*RefreshToken, error)
	RevokeRefreshTokensByUserClientTC(ctx context.Context, tenantID, clientID, userID string) (int64, error)

	// User consents con tenant+client directo
	UpsertConsentTC(ctx context.Context, tenantID, clientID, userID string, scopes []string) error
	ListConsentsByUserTC(ctx context.Context, tenantID, userID string) ([]UserConsent, error)
	RevokeConsentTC(ctx context.Context, tenantID, clientID, userID string) error
}
