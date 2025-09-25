package core

import (
	"context"
	"time"
)

type Tx interface {
	Commit(ctx context.Context) error
	Rollback(ctx context.Context) error
}

type Repository interface {
	Ping(ctx context.Context) error
	BeginTx(ctx context.Context) (Tx, error)

	// Auth (mínimo)
	GetUserByEmail(ctx context.Context, tenantID, email string) (*User, *Identity, error)
	CheckPassword(hash *string, pwd string) bool

	// Registry
	CreateTenant(ctx context.Context, t *Tenant) error
	CreateClient(ctx context.Context, c *Client) error
	CreateClientVersion(ctx context.Context, v *ClientVersion) error
	PromoteClientVersion(ctx context.Context, clientID, versionID string) error

	// Lecturas
	GetClientByClientID(ctx context.Context, clientID string) (*Client, *ClientVersion, error)

	//------- Registro por password -------
	CreateUser(ctx context.Context, u *User) error
	CreatePasswordIdentity(ctx context.Context, userID, email string, emailVerified bool, passwordHash string) error

	// Register (alta simple con password)
	CreateUserWithPassword(ctx context.Context, tenantID, email, passwordHash string) (*User, *Identity, error)

	// Refresh tokens
	CreateRefreshToken(ctx context.Context, userID, clientID, tokenHash string, expiresAt time.Time, rotatedFrom *string) (string, error)
	GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, id string) error
	// Revoke all refresh tokens for a user (optionally filtered by clientID when non-empty)
	RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error

	// Sprint 2: necesario para /userinfo
	GetUserByID(ctx context.Context, userID string) (*User, error)

	// --- MFA / TOTP & Trusted Devices ---
	UpsertMFATOTP(ctx context.Context, userID string, secretEnc string) error
	ConfirmMFATOTP(ctx context.Context, userID string, at time.Time) error
	GetMFATOTP(ctx context.Context, userID string) (*MFATOTP, error)
	UpdateMFAUsedAt(ctx context.Context, userID string, at time.Time) error
	DisableMFATOTP(ctx context.Context, userID string) error
	InsertRecoveryCodes(ctx context.Context, userID string, hashes []string) error
	DeleteRecoveryCodes(ctx context.Context, userID string) error
	UseRecoveryCode(ctx context.Context, userID string, hash string, at time.Time) (bool, error)
	AddTrustedDevice(ctx context.Context, userID string, deviceHash string, exp time.Time) error
	IsTrustedDevice(ctx context.Context, userID string, deviceHash string, now time.Time) (bool, error)

	// --- Admin Clients ---
	ListClients(ctx context.Context, tenantID, query string) ([]Client, error)
	GetClientByID(ctx context.Context, id string) (*Client, *ClientVersion, error)
	UpdateClient(ctx context.Context, c *Client) error
	DeleteClient(ctx context.Context, id string) error

	// Revocar todos los refresh tokens por client (todos los usuarios)
	RevokeAllRefreshTokensByClient(ctx context.Context, clientID string) error
}
