package repository

import (
	"context"
	"time"
)

// AdminType representa el tipo de administrador
type AdminType string

const (
	AdminTypeGlobal AdminType = "global" // Admin del sistema completo
	AdminTypeTenant AdminType = "tenant" // Admin con acceso limitado a tenants específicos
)

// Admin representa un administrador del sistema
type Admin struct {
	ID           string    `json:"id"`            // UUID único del admin
	Email        string    `json:"email"`         // Email del admin (único)
	PasswordHash string    `json:"password_hash"` // Hash bcrypt del password
	Name         string    `json:"name"`          // Nombre completo (opcional)
	Type         AdminType `json:"type"`          // Tipo de admin (global | tenant)

	// Para admins de tipo "tenant"
	AssignedTenants []string `json:"assigned_tenants,omitempty"` // IDs de tenants asignados (solo para AdminTypeTenant)

	// Metadata
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"` // Última vez que hizo login
	DisabledAt *time.Time `json:"disabled_at,omitempty"`  // Si está deshabilitado
	CreatedBy  *string    `json:"created_by,omitempty"`   // ID del admin que lo creó
}

// AdminRepository maneja la persistencia de administradores del sistema.
// Los admins se almacenan en el Control Plane (FileSystem).
//
// Ubicación: /data/admins/
//   - admins.yaml: Lista de todos los admins
//
// Estructura admins.yaml:
//   admins:
//     - id: uuid
//       email: admin@example.com
//       password_hash: bcrypt_hash
//       name: John Doe
//       type: global
//       created_at: 2026-01-22T10:00:00Z
//       updated_at: 2026-01-22T10:00:00Z
//     - id: uuid2
//       email: tenant-admin@example.com
//       type: tenant
//       assigned_tenants:
//         - tenant-uuid-1
//         - tenant-uuid-2
type AdminRepository interface {
	// ─── Read Operations ───

	// List retorna todos los admins del sistema.
	// Soporta filtrado por tipo (opcional).
	List(ctx context.Context, filter AdminFilter) ([]Admin, error)

	// GetByID busca un admin por su ID.
	// Retorna ErrNotFound si no existe.
	GetByID(ctx context.Context, id string) (*Admin, error)

	// GetByEmail busca un admin por su email.
	// Retorna ErrNotFound si no existe.
	GetByEmail(ctx context.Context, email string) (*Admin, error)

	// ─── Write Operations ───

	// Create crea un nuevo admin.
	// El email debe ser único.
	// Retorna ErrConflict si el email ya existe.
	Create(ctx context.Context, input CreateAdminInput) (*Admin, error)

	// Update actualiza un admin existente.
	// Solo se actualizan los campos no-nil en el input.
	Update(ctx context.Context, id string, input UpdateAdminInput) (*Admin, error)

	// Delete elimina un admin del sistema.
	// Retorna ErrNotFound si no existe.
	Delete(ctx context.Context, id string) error

	// ─── Auth Operations ───

	// CheckPassword verifica si un password es correcto para un admin.
	// Retorna true si coincide, false si no.
	CheckPassword(passwordHash, plainPassword string) bool

	// UpdateLastSeen actualiza el timestamp de último login.
	UpdateLastSeen(ctx context.Context, id string) error

	// ─── Tenant Assignment (solo para AdminTypeTenant) ───

	// AssignTenants asigna tenants a un admin de tipo tenant.
	// Reemplaza la lista completa de tenants asignados.
	AssignTenants(ctx context.Context, adminID string, tenantIDs []string) error

	// HasAccessToTenant verifica si un admin tiene acceso a un tenant específico.
	// Los admins globales siempre retornan true.
	// Los admins de tenant retornan true solo si el tenant está asignado.
	HasAccessToTenant(ctx context.Context, adminID, tenantID string) (bool, error)
}

// AdminFilter define filtros para listar admins
type AdminFilter struct {
	Type     *AdminType // Filtrar por tipo (nil = todos)
	Disabled *bool      // Filtrar por estado (nil = todos, true = solo disabled, false = solo activos)
	Limit    int        // Límite de resultados (0 = sin límite)
	Offset   int        // Offset para paginación
}

// CreateAdminInput define los datos para crear un admin
type CreateAdminInput struct {
	Email           string    // Requerido
	PasswordHash    string    // Requerido (ya hasheado)
	Name            string    // Opcional
	Type            AdminType // Requerido (global | tenant)
	AssignedTenants []string  // Opcional (solo para AdminTypeTenant)
	CreatedBy       *string   // Opcional (ID del admin que lo crea)
}

// UpdateAdminInput define los campos actualizables de un admin
type UpdateAdminInput struct {
	Email           *string    // Opcional
	PasswordHash    *string    // Opcional
	Name            *string    // Opcional
	AssignedTenants *[]string  // Opcional (solo para AdminTypeTenant)
	DisabledAt      *time.Time // Opcional (nil = habilitar, non-nil = deshabilitar)
}

// ═══════════════════════════════════════════════════════════════════════════════
// Admin Refresh Tokens
// ═══════════════════════════════════════════════════════════════════════════════

// AdminRefreshToken representa un refresh token de admin persistido.
type AdminRefreshToken struct {
	TokenHash string    `json:"token_hash"` // SHA-256 hash del token
	AdminID   string    `json:"admin_id"`   // ID del admin propietario
	ExpiresAt time.Time `json:"expires_at"` // Fecha de expiración
	CreatedAt time.Time `json:"created_at"` // Fecha de creación
}

// AdminRefreshTokenRepository maneja la persistencia de refresh tokens de admin.
// Los refresh tokens se almacenan en el Control Plane (FileSystem).
//
// Ubicación: /data/admins/refresh_tokens.yaml
//
// Estructura refresh_tokens.yaml:
//   refresh_tokens:
//     - token_hash: sha256_hash
//       admin_id: uuid
//       expires_at: 2026-02-22T10:00:00Z
//       created_at: 2026-01-22T10:00:00Z
type AdminRefreshTokenRepository interface {
	// ─── Read Operations ───

	// GetByTokenHash busca un refresh token por su hash.
	// Retorna ErrNotFound si no existe.
	GetByTokenHash(ctx context.Context, tokenHash string) (*AdminRefreshToken, error)

	// ListByAdminID retorna todos los refresh tokens de un admin.
	ListByAdminID(ctx context.Context, adminID string) ([]AdminRefreshToken, error)

	// ─── Write Operations ───

	// Create crea un nuevo refresh token.
	// Retorna ErrConflict si el hash ya existe.
	Create(ctx context.Context, input CreateAdminRefreshTokenInput) error

	// Delete elimina un refresh token por su hash.
	// Retorna ErrNotFound si no existe.
	Delete(ctx context.Context, tokenHash string) error

	// DeleteByAdminID elimina todos los refresh tokens de un admin.
	// Útil cuando se deshabilita o elimina un admin.
	DeleteByAdminID(ctx context.Context, adminID string) (int, error)

	// DeleteExpired elimina todos los refresh tokens expirados.
	// Retorna el número de tokens eliminados.
	DeleteExpired(ctx context.Context, now time.Time) (int, error)
}

// CreateAdminRefreshTokenInput define los datos para crear un refresh token.
type CreateAdminRefreshTokenInput struct {
	AdminID   string    // Requerido
	TokenHash string    // Requerido (SHA-256 del token opaco)
	ExpiresAt time.Time // Requerido
}

