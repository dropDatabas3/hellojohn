package pg

import (
	"context"
	"errors"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
	"github.com/jackc/pgx/v5"
)

// GetActiveSigningKey: clave activa más reciente y válida (now >= not_before)
func (s *Store) GetActiveSigningKey(ctx context.Context) (*core.SigningKey, error) {
	const q = `
SELECT kid, alg, public_key, private_key, status, not_before, created_at, rotated_at
FROM signing_keys
WHERE status = 'active' AND now() >= not_before
ORDER BY not_before DESC
LIMIT 1`
	row := s.pool.QueryRow(ctx, q)

	var k core.SigningKey
	var rotatedAt *time.Time
	if err := row.Scan(&k.KID, &k.Alg, &k.PublicKey, &k.PrivateKey, &k.Status, &k.NotBefore, &k.CreatedAt, &rotatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	k.RotatedAt = rotatedAt
	return &k, nil
}

// ListPublicSigningKeys: claves publicables (active + retiring)
func (s *Store) ListPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	const q = `
SELECT kid, alg, public_key, NULL::bytea as private_key, status, not_before, created_at, rotated_at
FROM signing_keys
WHERE status IN ('active','retiring')
ORDER BY status DESC, not_before DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.SigningKey
	for rows.Next() {
		var k core.SigningKey
		var rotatedAt *time.Time
		if err := rows.Scan(&k.KID, &k.Alg, &k.PublicKey, &k.PrivateKey, &k.Status, &k.NotBefore, &k.CreatedAt, &rotatedAt); err != nil {
			return nil, err
		}
		k.RotatedAt = rotatedAt
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Store) InsertSigningKey(ctx context.Context, k *core.SigningKey) error {
	const q = `
INSERT INTO signing_keys (kid, alg, public_key, private_key, status, not_before, created_at)
VALUES ($1, $2, $3, $4, $5, COALESCE($6, now()), now())`
	_, err := s.pool.Exec(ctx, q, k.KID, k.Alg, k.PublicKey, k.PrivateKey, k.Status, k.NotBefore)
	return err
}

func (s *Store) UpdateSigningKeyStatus(ctx context.Context, kid string, newStatus core.KeyStatus) error {
	const q = `UPDATE signing_keys SET status = $2, rotated_at = CASE WHEN $2 <> 'active' THEN now() ELSE rotated_at END WHERE kid = $1`
	_, err := s.pool.Exec(ctx, q, kid, newStatus)
	return err
}

// RotateSigningKeyTx: crea nueva ACTIVE y pasa la anterior a RETIRING en una tx.
func (s *Store) RotateSigningKeyTx(ctx context.Context, newKey core.SigningKey) (*core.SigningKey, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var prev *core.SigningKey
	{
		const q = `
SELECT kid, alg, public_key, private_key, status, not_before, created_at, rotated_at
FROM signing_keys
WHERE status = 'active' AND now() >= not_before
ORDER BY not_before DESC
LIMIT 1`
		row := tx.QueryRow(ctx, q)
		var k core.SigningKey
		var rotatedAt *time.Time
		if err := row.Scan(&k.KID, &k.Alg, &k.PublicKey, &k.PrivateKey, &k.Status, &k.NotBefore, &k.CreatedAt, &rotatedAt); err == nil {
			k.RotatedAt = rotatedAt
			prev = &k
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
	}

	// Marcar la anterior como retiring PRIMERO (para evitar constraint violation)
	if prev != nil {
		const q = `UPDATE signing_keys SET status='retiring', rotated_at=now() WHERE kid=$1 AND status='active'`
		if _, err := tx.Exec(ctx, q, prev.KID); err != nil {
			return nil, err
		}
	}

	// Luego insertar la nueva como active
	{
		const q = `
INSERT INTO signing_keys (kid, alg, public_key, private_key, status, not_before, created_at)
VALUES ($1,$2,$3,$4,'active',COALESCE($5, now()), now())`
		if _, err := tx.Exec(ctx, q, newKey.KID, newKey.Alg, newKey.PublicKey, newKey.PrivateKey, newKey.NotBefore); err != nil {
			return nil, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return prev, nil
}

// ListAllSigningKeys: todas las claves (active, retiring, retired)
func (s *Store) ListAllSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	const q = `
SELECT kid, alg, public_key, NULL::bytea as private_key, status, not_before, created_at, rotated_at
FROM signing_keys
ORDER BY status DESC, not_before DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.SigningKey
	for rows.Next() {
		var k core.SigningKey
		var rotatedAt *time.Time
		if err := rows.Scan(&k.KID, &k.Alg, &k.PublicKey, &k.PrivateKey, &k.Status, &k.NotBefore, &k.CreatedAt, &rotatedAt); err != nil {
			return nil, err
		}
		k.RotatedAt = rotatedAt
		out = append(out, k)
	}
	return out, rows.Err()
}

// RetireOldKeys: marca claves 'retiring' anteriores al cutoff como 'retired'
func (s *Store) RetireOldKeys(ctx context.Context, cutoff time.Time) (int, error) {
	const q = `
UPDATE signing_keys
SET status = 'retired'
WHERE status = 'retiring'
  AND rotated_at IS NOT NULL
  AND rotated_at < $1`
	result, err := s.pool.Exec(ctx, q, cutoff)
	if err != nil {
		return 0, err
	}
	return int(result.RowsAffected()), nil
}
