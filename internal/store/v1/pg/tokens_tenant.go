package pg

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
)

func (s *Store) CreateRefreshTokenTC(ctx context.Context, tenantID, clientIDText, userID string, ttl time.Duration) (string, error) {
	// Generate secure token
	token := generateSecureToken()
	tokenHash := hashToken(token)

	// Use ONLY tenant schema (no tenant_id column, client_id is TEXT)
	const q = `
		INSERT INTO refresh_token (client_id, user_id, token_hash, issued_at, expires_at, metadata)
		VALUES ($1, $2, $3, NOW(), NOW() + $4::interval, '{}')
		RETURNING id`

	var tokenID string
	err := s.pool.QueryRow(ctx, q, clientIDText, userID, tokenHash, ttl.String()).Scan(&tokenID)
	if err != nil {
		return "", fmt.Errorf("tenant_insert: %v", err)
	}

	return token, nil
}

func containsSQLState(msg, code string) bool {
	// pgx drivers usually return error objects, checking text for MVP
	return strings.Contains(msg, code) || strings.Contains(msg, "undefined column")
}

func (s *Store) GetRefreshTokenTC(ctx context.Context, tenantID, clientIDText, token string) (*core.RefreshToken, error) {
	tokenHash := hashToken(token)
	return s.GetRefreshTokenByHashTC(ctx, tenantID, clientIDText, tokenHash)
}

func (s *Store) GetRefreshTokenByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (*core.RefreshToken, error) {
	// Intentar query Global
	const qGlobal = `
		SELECT id, user_id, client_id_text, token_hash, issued_at, expires_at, rotated_from, revoked_at, tenant_id
		FROM refresh_token
		WHERE tenant_id = $1 AND client_id_text = $2 AND token_hash = $3
		LIMIT 1`

	// Intentar query Tenant
	const qTenant = `
		SELECT id, user_id, client_id, token_hash, issued_at, expires_at, rotated_from, revoked_at
		FROM refresh_token
		WHERE client_id = $1 AND token_hash = $2
		LIMIT 1`

	var rt core.RefreshToken
	err := s.pool.QueryRow(ctx, qGlobal, tenantID, clientIDText, tokenHash).Scan(
		&rt.ID, &rt.UserID, &rt.ClientIDText, &rt.TokenHash, &rt.IssuedAt,
		&rt.ExpiresAt, &rt.RotatedFrom, &rt.RevokedAt, &rt.TenantID)

	if err != nil {
		errMsg := err.Error()
		if containsSQLState(errMsg, "42703") || containsSQLState(errMsg, "42P01") { // Undefined table or column
			// Fallback Tenant
			errT := s.pool.QueryRow(ctx, qTenant, clientIDText, tokenHash).Scan(
				&rt.ID, &rt.UserID, &rt.ClientIDText, &rt.TokenHash, &rt.IssuedAt,
				&rt.ExpiresAt, &rt.RotatedFrom, &rt.RevokedAt)

			if errT != nil {
				if errT.Error() == "no rows in result set" {
					return nil, core.ErrNotFound
				}
				return nil, errT
			}
			// Tenant query success
			rt.TenantID = tenantID // Inject tenantID implicitly known from context
			return &rt, nil
		}
		if err.Error() == "no rows in result set" {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	return &rt, nil
}

func (s *Store) RevokeRefreshTokensByUserClientTC(ctx context.Context, tenantID, clientIDText, userID string) (int64, error) {
	// Try Global
	const qGlobal = `
		UPDATE refresh_token 
		SET revoked_at = NOW() 
		WHERE tenant_id = $1 AND client_id_text = $2 AND user_id = $3 AND revoked_at IS NULL`

	// Try Tenant
	const qTenant = `
		UPDATE refresh_token 
		SET revoked_at = NOW() 
		WHERE client_id = $1 AND user_id = $2 AND revoked_at IS NULL`

	ct, err := s.pool.Exec(ctx, qGlobal, tenantID, clientIDText, userID)
	if err != nil {
		if containsSQLState(err.Error(), "42703") {
			ctT, errT := s.pool.Exec(ctx, qTenant, clientIDText, userID)
			if errT != nil {
				return 0, errT
			}
			return ctT.RowsAffected(), nil
		}
		return 0, err
	}
	return ct.RowsAffected(), nil
}

// RevokeRefreshByHashTC: revoca 1 refresh por tenant + client_id_text + token_hash
func (s *Store) RevokeRefreshByHashTC(ctx context.Context, tenantID, clientIDText, tokenHash string) (int64, error) {
	const qGlobal = `
		UPDATE refresh_token
		   SET revoked_at = NOW()
		 WHERE tenant_id = $1
		   AND client_id_text = $2
		   AND token_hash = $3
		   AND revoked_at IS NULL`

	const qTenant = `
		UPDATE refresh_token
		   SET revoked_at = NOW()
		 WHERE client_id = $1
		   AND token_hash = $2
		   AND revoked_at IS NULL`

	tag, err := s.pool.Exec(ctx, qGlobal, tenantID, clientIDText, tokenHash)
	if err != nil {
		if containsSQLState(err.Error(), "42703") {
			tagT, errT := s.pool.Exec(ctx, qTenant, clientIDText, tokenHash)
			if errT != nil {
				return 0, errT
			}
			return tagT.RowsAffected(), nil
		}
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
