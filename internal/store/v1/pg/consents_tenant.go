package pg

import (
	"context"

	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
)

func (s *Store) UpsertConsentTC(ctx context.Context, tenantID, clientIDText, userID string, scopes []string) error {
	const q = `
		INSERT INTO user_consent (user_id, tenant_id, client_id_text, granted_scopes, granted_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id, tenant_id, client_id_text)
		DO UPDATE SET granted_scopes = EXCLUDED.granted_scopes, granted_at = NOW(), revoked_at = NULL`

	_, err := s.pool.Exec(ctx, q, userID, tenantID, clientIDText, scopes)
	return err
}

func (s *Store) ListConsentsByUserTC(ctx context.Context, tenantID, userID string) ([]core.UserConsent, error) {
	const q = `
		SELECT id, user_id, client_id_text, granted_scopes, granted_at, revoked_at, tenant_id 
		FROM user_consent 
		WHERE user_id = $1 AND tenant_id = $2 AND revoked_at IS NULL`

	rows, err := s.pool.Query(ctx, q, userID, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.UserConsent
	for rows.Next() {
		var c core.UserConsent
		if err := rows.Scan(&c.ID, &c.UserID, &c.ClientIDText, &c.GrantedScopes, &c.GrantedAt, &c.RevokedAt, &c.TenantID); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *Store) RevokeConsentTC(ctx context.Context, tenantID, clientIDText, userID string) error {
	const q = `
		UPDATE user_consent 
		SET revoked_at = NOW() 
		WHERE user_id = $1 AND tenant_id = $2 AND client_id_text = $3 AND revoked_at IS NULL`

	_, err := s.pool.Exec(ctx, q, userID, tenantID, clientIDText)
	return err
}
