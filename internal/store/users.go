package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type UserStore struct{ DB DBOps }

func (s *UserStore) SetEmailVerified(ctx context.Context, userID uuid.UUID) error {
	_, err := s.DB.Exec(ctx, `UPDATE app_user SET email_verified = TRUE WHERE id = $1`, userID)
	return err
}

func (s *UserStore) LookupUserIDByEmail(ctx context.Context, tenantID uuid.UUID, email string) (uuid.UUID, bool, error) {
	var id uuid.UUID
	err := s.DB.QueryRow(ctx, `SELECT id FROM app_user WHERE tenant_id=$1 AND email=$2`, tenantID, email).Scan(&id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return uuid.Nil, false, nil
		}
		return uuid.Nil, false, err
	}
	return id, true, nil
}

func (s *UserStore) UpdatePasswordHash(ctx context.Context, userID uuid.UUID, newPHC string) error {
	_, err := s.DB.Exec(ctx, `
        UPDATE identity
           SET password_hash = $1
         WHERE user_id = $2 AND provider = 'password'`,
		newPHC, userID,
	)
	return err
}

func (s *UserStore) RevokeAllRefreshTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := s.DB.Exec(ctx, `UPDATE refresh_token SET revoked_at = now() WHERE user_id=$1 AND revoked_at IS NULL`, userID)
	return err
}

// Devuelve el email (lowercased) del usuario, o error si no existe
func (s *UserStore) GetEmailByID(ctx context.Context, userID uuid.UUID) (string, error) {
	var email string
	if err := s.DB.QueryRow(ctx, `SELECT email FROM app_user WHERE id = $1`, userID).Scan(&email); err != nil {
		return "", err
	}
	return email, nil
}
