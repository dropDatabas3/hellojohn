package core

import "context"

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

	// ------- NUEVO: Registro por password -------
	CreateUser(ctx context.Context, u *User) error
	CreatePasswordIdentity(ctx context.Context, userID, email string, emailVerified bool, passwordHash string) error
}
