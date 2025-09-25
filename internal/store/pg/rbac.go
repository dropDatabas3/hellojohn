package pg

import (
	"context"
	"errors"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
	"github.com/jackc/pgx/v5"
)

// ---------- LECTURAS ----------

// GetUserRoles: devuelve los roles asignados al usuario (por su user_id).
func (s *Store) GetUserRoles(ctx context.Context, userID string) ([]string, error) {
	const q = `
SELECT ur.role
FROM rbac_user_role ur
WHERE ur.user_id = $1
ORDER BY ur.role;`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var r string
		if err := rows.Scan(&r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetUserPermissions: permisos efectivos derivados de los roles del usuario.
func (s *Store) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	const q = `
SELECT DISTINCT rp.perm
FROM rbac_user_role ur
JOIN rbac_role_perm rp
  ON rp.tenant_id = ur.tenant_id AND rp.role = ur.role
WHERE ur.user_id = $1
ORDER BY rp.perm;`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetRolePerms: lista permisos vigentes para un rol de un tenant.
func (s *Store) GetRolePerms(ctx context.Context, tenantID, role string) ([]string, error) {
	const q = `
SELECT perm
FROM rbac_role_perm
WHERE tenant_id = $1 AND role = $2
ORDER BY perm;`
	rows, err := s.pool.Query(ctx, q, tenantID, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Compile-time check (opcional): confirma que *Store satisface la interfaz.
var _ core.RBACRepository = (*Store)(nil)

// ---------- ESCRITURAS ----------

func (s *Store) AssignUserRoles(ctx context.Context, userID string, add []string) error {
	// Obtener tenant_id del usuario (rbac_user_role tiene tenant_id)
	var tenantID string
	err := s.pool.QueryRow(ctx, `SELECT tenant_id FROM app_user WHERE id = $1`, userID).Scan(&tenantID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrNotFound
		}
		return err
	}

	// Normalizar + de-dup
	clean := make([]string, 0, len(add))
	seen := map[string]struct{}{}
	for _, r := range add {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		clean = append(clean, r)
	}
	if len(clean) == 0 {
		return nil
	}

	// Batch insert ON CONFLICT DO NOTHING
	b := &pgx.Batch{}
	for _, r := range clean {
		b.Queue(`INSERT INTO rbac_user_role (user_id, tenant_id, role) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, userID, tenantID, r)
	}
	br := s.pool.SendBatch(ctx, b)
	defer br.Close()
	for range clean {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RemoveUserRoles(ctx context.Context, userID string, remove []string) error {
	clean := make([]string, 0, len(remove))
	seen := map[string]struct{}{}
	for _, r := range remove {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		seen[r] = struct{}{}
		clean = append(clean, r)
	}
	if len(clean) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM rbac_user_role WHERE user_id = $1 AND role = ANY($2)`, userID, clean)
	return err
}

func (s *Store) AddRolePerms(ctx context.Context, tenantID, role string, add []string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		return core.ErrInvalid
	}
	clean := make([]string, 0, len(add))
	seen := map[string]struct{}{}
	for _, p := range add {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return nil
	}
	b := &pgx.Batch{}
	for _, p := range clean {
		b.Queue(`INSERT INTO rbac_role_perm (tenant_id, role, perm) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, tenantID, role, p)
	}
	br := s.pool.SendBatch(ctx, b)
	defer br.Close()
	for range clean {
		if _, err := br.Exec(); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RemoveRolePerms(ctx context.Context, tenantID, role string, remove []string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		return core.ErrInvalid
	}
	clean := make([]string, 0, len(remove))
	seen := map[string]struct{}{}
	for _, p := range remove {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return nil
	}
	_, err := s.pool.Exec(ctx, `DELETE FROM rbac_role_perm WHERE tenant_id = $1 AND role = $2 AND perm = ANY($3)`, tenantID, role, clean)
	return err
}
