package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
)

type Store struct{}

func New(ctx context.Context, dsn string) (core.Repository, error) {
	return nil, fmt.Errorf("mysql: %w", core.ErrNotImplemented)
}

// Below are stubs to satisfy interfaces at compile time.
func (s *Store) Ping(ctx context.Context) error               { return core.ErrNotImplemented }
func (s *Store) BeginTx(ctx context.Context) (core.Tx, error) { return nil, core.ErrNotImplemented }
func (s *Store) GetUserByEmail(ctx context.Context, tenantID, email string) (*core.User, *core.Identity, error) {
	return nil, nil, core.ErrNotImplemented
}
func (s *Store) CheckPassword(hash *string, pwd string) bool { return false }
func (s *Store) CreateTenant(ctx context.Context, t *core.Tenant) error {
	return core.ErrNotImplemented
}
func (s *Store) CreateClient(ctx context.Context, c *core.Client) error {
	return core.ErrNotImplemented
}
func (s *Store) CreateClientVersion(ctx context.Context, v *core.ClientVersion) error {
	return core.ErrNotImplemented
}
func (s *Store) PromoteClientVersion(ctx context.Context, clientID, versionID string) error {
	return core.ErrNotImplemented
}
func (s *Store) GetClientByClientID(ctx context.Context, clientID string) (*core.Client, *core.ClientVersion, error) {
	return nil, nil, core.ErrNotImplemented
}
func (s *Store) CreateUser(ctx context.Context, u *core.User) error { return core.ErrNotImplemented }
func (s *Store) CreatePasswordIdentity(ctx context.Context, userID, email string, emailVerified bool, passwordHash string) error {
	return core.ErrNotImplemented
}
func (s *Store) CreateUserWithPassword(ctx context.Context, tenantID, email, passwordHash string) (*core.User, *core.Identity, error) {
	return nil, nil, core.ErrNotImplemented
}
func (s *Store) CreateRefreshToken(ctx context.Context, userID, clientID, tokenHash string, expiresAt time.Time, rotatedFrom *string) (string, error) {
	return "", core.ErrNotImplemented
}
func (s *Store) GetRefreshTokenByHash(ctx context.Context, tokenHash string) (*core.RefreshToken, error) {
	return nil, core.ErrNotImplemented
}
func (s *Store) RevokeRefreshToken(ctx context.Context, id string) error {
	return core.ErrNotImplemented
}
func (s *Store) GetUserByID(ctx context.Context, userID string) (*core.User, error) {
	return nil, core.ErrNotImplemented
}
func (s *Store) RevokeAllRefreshTokens(ctx context.Context, userID, clientID string) error {
	return core.ErrNotImplemented
}
func (s *Store) ListClients(ctx context.Context, tenantID, query string) ([]core.Client, error) {
	return nil, core.ErrNotImplemented
}
func (s *Store) GetClientByID(ctx context.Context, id string) (*core.Client, *core.ClientVersion, error) {
	return nil, nil, core.ErrNotImplemented
}
func (s *Store) UpdateClient(ctx context.Context, c *core.Client) error {
	return core.ErrNotImplemented
}
func (s *Store) DeleteClient(ctx context.Context, id string) error { return core.ErrNotImplemented }
func (s *Store) RevokeAllRefreshTokensByClient(ctx context.Context, clientID string) error {
	return core.ErrNotImplemented
}

// New admin methods
func (s *Store) DisableUser(ctx context.Context, userID, by, reason string) error {
	return core.ErrNotImplemented
}
func (s *Store) EnableUser(ctx context.Context, userID, by string) error {
	return core.ErrNotImplemented
}
func (s *Store) RevokeAllRefreshByUser(ctx context.Context, userID string) (int, error) {
	return 0, core.ErrNotImplemented
}
