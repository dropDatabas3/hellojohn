package pg

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dropDatabas3/hellojohn/internal/store/v1/core"
	"github.com/dropDatabas3/hellojohn/internal/validation"
)

// PgExecQuerier es la mínima interfaz que cumplen *pgxpool.Pool y pgx.Tx.
// Usamos pgconn.CommandTag directamente (lo que retorna Exec en pgx).
type PgExecQuerier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// ScopesConsentsPG implementa core.ScopesConsentsRepository sobre Postgres.
type ScopesConsentsPG struct {
	db PgExecQuerier
}

func NewScopesConsentsPG(db PgExecQuerier) *ScopesConsentsPG {
	return &ScopesConsentsPG{db: db}
}

// ─────────────────────────────
// Scopes
// ─────────────────────────────

func (r *ScopesConsentsPG) CreateScope(ctx context.Context, tenantID, name, description string) (core.Scope, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return core.Scope{}, errors.New("scope name vacío")
	}
	// doble cinturón: validación también a nivel repo
	if !validation.ValidScopeName(name) {
		return core.Scope{}, core.ErrInvalid
	}
	const q = `
INSERT INTO scope (tenant_id, name, description)
VALUES ($1, $2, $3)
RETURNING id, tenant_id, name, description, created_at;
`
	var s core.Scope
	err := r.db.QueryRow(ctx, q, tenantID, name, description).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.CreatedAt)
	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" { // unique_violation
			return core.Scope{}, core.ErrConflict
		}
		return core.Scope{}, err
	}
	return s, nil
}

func (r *ScopesConsentsPG) GetScopeByName(ctx context.Context, tenantID, name string) (core.Scope, error) {
	const q = `
SELECT id, tenant_id, name, description, created_at
FROM scope
WHERE tenant_id = $1 AND name = $2;
`
	var s core.Scope
	err := r.db.QueryRow(ctx, q, tenantID, strings.ToLower(strings.TrimSpace(name))).
		Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.CreatedAt)
	return s, err
}

func (r *ScopesConsentsPG) ListScopes(ctx context.Context, tenantID string) ([]core.Scope, error) {
	const q = `
SELECT id, tenant_id, name, description, created_at
FROM scope
WHERE tenant_id = $1
ORDER BY name ASC;
`
	rows, err := r.db.Query(ctx, q, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]core.Scope, 0, 16)
	for rows.Next() {
		var s core.Scope
		if err := rows.Scan(&s.ID, &s.TenantID, &s.Name, &s.Description, &s.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (r *ScopesConsentsPG) DeleteScope(ctx context.Context, tenantID, scopeID string) error {
	// Obtener nombre (los consents almacenan nombres en granted_scopes)
	var name string
	if err := r.db.QueryRow(ctx, `SELECT name FROM scope WHERE tenant_id=$1 AND id=$2`, tenantID, scopeID).Scan(&name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrNotFound
		}
		return err
	}
	// Verificar si está en uso por algún consentimiento ACTIVO (revoked_at IS NULL) del mismo tenant
	var inUse bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			  FROM user_consent uc
			  JOIN client c ON c.id = uc.client_id
			 WHERE c.tenant_id = $1
			   AND uc.revoked_at IS NULL
			   AND $2 = ANY(uc.granted_scopes)
			 LIMIT 1
		)
	`, tenantID, name).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return core.ErrConflict
	}
	_, err := r.db.Exec(ctx, `DELETE FROM scope WHERE tenant_id=$1 AND id=$2`, tenantID, scopeID)
	return err
}

// DeleteScopeByID elimina un scope sólo por ID y verifica si está en uso.
func (r *ScopesConsentsPG) DeleteScopeByID(ctx context.Context, id string) error {
	// Obtener tenant_id y nombre para evaluar si está en uso por consents activos.
	var tenantID, name string
	if err := r.db.QueryRow(ctx, `SELECT tenant_id, name FROM scope WHERE id=$1`, id).Scan(&tenantID, &name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.ErrNotFound
		}
		return err
	}
	// Chequeo de uso: algún consentimiento ACTIVO del mismo tenant que contenga este nombre.
	var inUse bool
	if err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			  FROM user_consent uc
			  JOIN client c ON c.id = uc.client_id
			 WHERE c.tenant_id = $1
			   AND uc.revoked_at IS NULL
			   AND $2 = ANY(uc.granted_scopes)
			 LIMIT 1
		)
	`, tenantID, name).Scan(&inUse); err != nil {
		return err
	}
	if inUse {
		return core.ErrConflict
	}
	// Borrar por id.
	if _, err := r.db.Exec(ctx, `DELETE FROM scope WHERE id=$1`, id); err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23503" { // fallback si hubiera futuras FKs
			return core.ErrConflict
		}
		return err
	}
	return nil
}

// UpdateScopeDescription actualiza solamente la descripción del scope.
// No permite cambiar el nombre para no invalidar consentimientos existentes.
func (r *ScopesConsentsPG) UpdateScopeDescription(ctx context.Context, tenantID, scopeID, description string) error {
	tag, err := r.db.Exec(ctx, `UPDATE scope SET description=$3 WHERE tenant_id=$1 AND id=$2`, tenantID, scopeID, strings.TrimSpace(description))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return core.ErrNotFound
	}
	return nil
}

// UpdateScopeDescriptionByID actualiza la descripción usando solo el ID (sin tenant) para usos administrativos.
func (r *ScopesConsentsPG) UpdateScopeDescriptionByID(ctx context.Context, id, description string) error {
	const q = `UPDATE scope SET description=$2 WHERE id=$1`
	ct, err := r.db.Exec(ctx, q, id, strings.TrimSpace(description))
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return core.ErrNotFound
	}
	return nil
}

// ─────────────────────────────
// Consents
// ─────────────────────────────

func (r *ScopesConsentsPG) UpsertConsent(ctx context.Context, userID, clientID string, scopes []string) (core.UserConsent, error) {
	// Normalizar lista (trim + de-dup, lowercase)
	clean := make([]string, 0, len(scopes))
	seen := map[string]struct{}{}
	for _, s := range scopes {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		clean = append(clean, s)
	}
	if len(clean) == 0 {
		return core.UserConsent{}, errors.New("granted_scopes vacío")
	}

	// Unión con los existentes al upsert (DISTINCT)
	const q = `
INSERT INTO user_consent (user_id, client_id_text, granted_scopes)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, client_id_text)
DO UPDATE SET
	granted_scopes = (
		SELECT ARRAY(
			SELECT DISTINCT x
				FROM UNNEST(user_consent.granted_scopes || EXCLUDED.granted_scopes) AS t(x)
		)
	),
	revoked_at = NULL,
	granted_at = COALESCE(user_consent.granted_at, now())
RETURNING id, user_id, client_id_text, granted_scopes, granted_at, revoked_at;
`
	var uc core.UserConsent
	err := r.db.QueryRow(ctx, q, userID, clientID, clean).
		Scan(&uc.ID, &uc.UserID, &uc.ClientIDText, &uc.GrantedScopes, &uc.GrantedAt, &uc.RevokedAt)
	return uc, err
}

func (r *ScopesConsentsPG) GetConsent(ctx context.Context, userID, clientID string) (core.UserConsent, error) {
	const q = `
SELECT id, user_id, client_id_text, granted_scopes, granted_at, revoked_at
FROM user_consent
WHERE user_id = $1 AND client_id_text = $2;
`
	var uc core.UserConsent
	err := r.db.QueryRow(ctx, q, userID, clientID).
		Scan(&uc.ID, &uc.UserID, &uc.ClientIDText, &uc.GrantedScopes, &uc.GrantedAt, &uc.RevokedAt)
	return uc, err
}

func (r *ScopesConsentsPG) ListConsentsByUser(ctx context.Context, userID string, activeOnly bool) ([]core.UserConsent, error) {
	q := `
SELECT id, user_id, client_id_text, granted_scopes, granted_at, revoked_at
FROM user_consent
WHERE user_id = $1`
	if activeOnly {
		q += " AND revoked_at IS NULL"
	}
	q += " ORDER BY granted_at DESC;"
	rows, err := r.db.Query(ctx, q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.UserConsent
	for rows.Next() {
		var uc core.UserConsent
		if err := rows.Scan(&uc.ID, &uc.UserID, &uc.ClientIDText, &uc.GrantedScopes, &uc.GrantedAt, &uc.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, uc)
	}
	return out, rows.Err()
}

func (r *ScopesConsentsPG) RevokeConsent(ctx context.Context, userID, clientID string, at time.Time) error {
	const q = `
UPDATE user_consent
SET revoked_at = $3
WHERE user_id = $1 AND client_id_text = $2;
`
	if _, err := r.db.Exec(ctx, q, userID, clientID, at); err != nil {
		return err
	}
	// Revocar refresh del par (user,client) para que no se pueda refreshear luego del revoke
	const q2 = `
UPDATE refresh_token
   SET revoked_at = $3
 WHERE user_id = $1 AND client_id_text = $2 AND revoked_at IS NULL;`
	_, err := r.db.Exec(ctx, q2, userID, clientID, at)
	return err
}

var _ core.ScopesConsentsRepository = (*ScopesConsentsPG)(nil)
