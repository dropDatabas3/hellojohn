package pg

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

func (s *Store) CreateRefreshTokenTC(ctx context.Context, tenantID, clientIDText, userID string, ttl time.Duration) (string, error) {
	// Generar token seguro
	token := generateSecureToken()
	tokenHash := hashToken(token)

	const q = `
		INSERT INTO refresh_token (tenant_id, client_id_text, user_id, token_hash, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW() + $5::interval)
		RETURNING id`

	var tokenID string
	err := s.pool.QueryRow(ctx, q, tenantID, clientIDText, userID, tokenHash, ttl.String()).Scan(&tokenID)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (s *Store) GetRefreshTokenTC(ctx context.Context, tenantID, clientIDText, token string) (*core.RefreshToken, error) {
	tokenHash := hashToken(token)
	return s.GetRefreshTokenByHashTC(ctx, tenantID, clientIDText, tokenHash)
}

func (s *Store) GetRefreshTokenByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (*core.RefreshToken, error) {
	const q = `
		SELECT id, user_id, client_id_text, token_hash, issued_at, expires_at, rotated_from, revoked_at, tenant_id
		FROM refresh_token
		WHERE tenant_id = $1 AND client_id_text = $2 AND token_hash = $3
		LIMIT 1`

	var rt core.RefreshToken
	err := s.pool.QueryRow(ctx, q, tenantID, clientIDText, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.ClientIDText, &rt.TokenHash, &rt.IssuedAt,
		&rt.ExpiresAt, &rt.RotatedFrom, &rt.RevokedAt, &rt.TenantID)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	return &rt, nil
}

func (s *Store) RevokeRefreshTokensByUserClientTC(ctx context.Context, tenantID, clientIDText, userID string) (int64, error) {
	const q = `
		UPDATE refresh_token 
		SET revoked_at = NOW() 
		WHERE tenant_id = $1 AND client_id_text = $2 AND user_id = $3 AND revoked_at IS NULL`

	ct, err := s.pool.Exec(ctx, q, tenantID, clientIDText, userID)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// RevokeRefreshByHashTC: revoca 1 refresh por tenant + client_id_text + token_hash
func (s *Store) RevokeRefreshByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (int64, error) {
	const q = `
		UPDATE refresh_token
		   SET revoked_at = NOW()
		 WHERE tenant_id = $1
		   AND client_id_text = $2
		   AND token_hash = $3
		   AND revoked_at IS NULL`
	tag, err := s.pool.Exec(ctx, q, tenantID, clientIDText, tokenHash)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// helper opcional si te mandan el refresh plano
func (s *Store) RevokeRefreshTC(ctx context.Context, tenantID, clientIDText, refreshPlain string) (int64, error) {
	return s.RevokeRefreshByHashTC(ctx, tenantID, clientIDText, hashToken(refreshPlain))
}

// generateSecureToken crea un token seguro de 32 bytes como hex
func generateSecureToken() string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic("failed to generate secure random token: " + err.Error())
	}
	return hex.EncodeToString(bytes)
}

// hashToken crea un hash SHA256 del token para almacenamiento seguro
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
