package repository

import (
	"context"
	"time"
)

// RefreshToken representa un token de refresco.
type RefreshToken struct {
	ID          string
	UserID      string
	TenantID    string
	ClientID    string // client_id texto (no UUID)
	TokenHash   string
	IssuedAt    time.Time
	ExpiresAt   time.Time
	RotatedFrom *string
	RevokedAt   *time.Time
}

// CreateRefreshTokenInput contiene los datos para crear un refresh token.
type CreateRefreshTokenInput struct {
	TenantID   string
	ClientID   string
	UserID     string
	TokenHash  string
	TTLSeconds int
}

// TokenRepository define operaciones sobre refresh tokens.
type TokenRepository interface {
	// Create crea un nuevo refresh token.
	// Retorna el ID del token creado.
	Create(ctx context.Context, input CreateRefreshTokenInput) (string, error)

	// GetByHash busca un token por su hash.
	// Retorna ErrNotFound si no existe.
	GetByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)

	// Revoke revoca un token por su ID.
	Revoke(ctx context.Context, tokenID string) error

	// RevokeAllByUser revoca todos los tokens de un usuario.
	// Si clientID no está vacío, filtra solo por ese client.
	// Retorna el número de tokens revocados.
	RevokeAllByUser(ctx context.Context, userID, clientID string) (int, error)

	// RevokeAllByClient revoca todos los tokens de un client (todos los usuarios).
	RevokeAllByClient(ctx context.Context, clientID string) error
}
