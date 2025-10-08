package pg

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

func (s *Store) CreateRefreshTokenTC(ctx context.Context, tenantID, clientID, userID string, ttl time.Duration) (string, error) {
	const q = `
		INSERT INTO refresh_tokens (token, user_id, tenant_id, client_id, expires_at, created_at)
		VALUES ($1, $2, $3, $4, NOW() + $5::interval, NOW())
		RETURNING token`
	tok := generateSecureToken()
	var out string
	err := s.pool.QueryRow(ctx, q, tok, userID, tenantID, clientID, ttl.String()).Scan(&out)
	if err != nil {
		return "", err
	}
	return out, nil
}

func (s *Store) RevokeRefreshTokensByUserClientTC(ctx context.Context, tenantID, clientID, userID string) (int64, error) {
	const q = `DELETE FROM refresh_tokens WHERE user_id=$1 AND tenant_id=$2 AND client_id=$3`
	ct, err := s.pool.Exec(ctx, q, userID, tenantID, clientID)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// generateSecureToken crea un token seguro de 32 bytes como hex
func generateSecureToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("failed to generate secure random token: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}
