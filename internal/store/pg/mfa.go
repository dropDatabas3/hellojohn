package pg

import (
	"context"
	"errors"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Helper interno para parsear userID string -> uuid
func parseUUID(id string) (uuid.UUID, error) { return uuid.Parse(id) }

func (s *Store) UpsertMFATOTP(ctx context.Context, userID string, secretEnc string) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO user_mfa_totp (user_id, secret_encrypted)
		VALUES ($1,$2)
		ON CONFLICT (user_id)
		DO UPDATE SET secret_encrypted = EXCLUDED.secret_encrypted,
					  confirmed_at = NULL,
					  last_used_at = NULL
	`, uid, secretEnc)
	return err
}

func (s *Store) ConfirmMFATOTP(ctx context.Context, userID string, at time.Time) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE user_mfa_totp SET confirmed_at = $2 WHERE user_id = $1`, uid, at)
	return err
}

func (s *Store) GetMFATOTP(ctx context.Context, userID string) (*core.MFATOTP, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return nil, err
	}
	row := s.pool.QueryRow(ctx, `
		SELECT user_id, secret_encrypted, confirmed_at, last_used_at, created_at, updated_at
		FROM user_mfa_totp WHERE user_id = $1
	`, uid)
	var m core.MFATOTP
	if err := row.Scan(&m.UserID, &m.SecretEncrypted, &m.ConfirmedAt, &m.LastUsedAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	m.UserID = uid.String()
	return &m, nil
}

func (s *Store) UpdateMFAUsedAt(ctx context.Context, userID string, at time.Time) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE user_mfa_totp SET last_used_at = $2 WHERE user_id = $1`, uid, at)
	return err
}

func (s *Store) DisableMFATOTP(ctx context.Context, userID string) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	if _, err := s.pool.Exec(ctx, `DELETE FROM user_mfa_totp WHERE user_id = $1`, uid); err != nil {
		return err
	}
	_, _ = s.pool.Exec(ctx, `DELETE FROM mfa_recovery_code WHERE user_id = $1`, uid)
	_, _ = s.pool.Exec(ctx, `DELETE FROM trusted_device WHERE user_id = $1`, uid)
	return nil
}

// ====================== Recovery codes ======================

func (s *Store) InsertRecoveryCodes(ctx context.Context, userID string, hashes []string) error {
	if len(hashes) == 0 {
		return nil
	}
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	var b pgx.Batch
	for _, h := range hashes {
		b.Queue(`INSERT INTO mfa_recovery_code (user_id, code_hash) VALUES ($1,$2)`, uid, h)
	}
	br := s.pool.SendBatch(ctx, &b)
	for range hashes {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return err
		}
	}
	return br.Close()
}

// DeleteRecoveryCodes elimina todos los recovery codes del usuario (para rotación segura)
func (s *Store) DeleteRecoveryCodes(ctx context.Context, userID string) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `DELETE FROM mfa_recovery_code WHERE user_id = $1`, uid)
	return err
}

func (s *Store) UseRecoveryCode(ctx context.Context, userID string, hash string, at time.Time) (bool, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return false, err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE mfa_recovery_code
		SET used_at = $3
		WHERE user_id = $1 AND code_hash = $2 AND used_at IS NULL
	`, uid, hash, at)
	return tag.RowsAffected() == 1, err
}

// ====================== Trusted devices ======================

func (s *Store) AddTrustedDevice(ctx context.Context, userID string, deviceHash string, exp time.Time) error {
	uid, err := parseUUID(userID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO trusted_device (user_id, device_hash, expires_at)
		VALUES ($1,$2,$3)
		ON CONFLICT (user_id, device_hash)
		DO UPDATE SET expires_at = EXCLUDED.expires_at
	`, uid, deviceHash, exp)
	return err
}

func (s *Store) IsTrustedDevice(ctx context.Context, userID string, deviceHash string, now time.Time) (bool, error) {
	uid, err := parseUUID(userID)
	if err != nil {
		return false, err
	}
	var count int
	if err := s.pool.QueryRow(ctx, `
		SELECT COUNT(1)
		FROM trusted_device
		WHERE user_id = $1 AND device_hash = $2 AND expires_at > $3
	`, uid, deviceHash, now).Scan(&count); err != nil {
		return false, err
	}
	return count == 1, nil
}
