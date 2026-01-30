package admin

// ─── Import/Export DTOs ───

// TenantImportRequest representa los datos para importar configuración de un tenant.
type TenantImportRequest struct {
	Version    string                  `json:"version"`              // "1.0"
	ExportedAt string                  `json:"exportedAt,omitempty"` // ISO8601 timestamp
	Mode       string                  `json:"mode,omitempty"`       // "merge" | "replace" (default: merge)
	Tenant     *TenantImportInfo       `json:"tenant,omitempty"`
	Settings   *TenantSettingsResponse `json:"settings,omitempty"`
	Clients    []ClientImportData      `json:"clients,omitempty"`
	Scopes     []ScopeImportData       `json:"scopes,omitempty"`
	Users      []UserImportData        `json:"users,omitempty"`
	Roles      []RoleImportData        `json:"roles,omitempty"`
}

// TenantImportInfo información básica del tenant a importar.
type TenantImportInfo struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	DisplayName string `json:"display_name,omitempty"`
	Language    string `json:"language,omitempty"`
}

// ClientImportData datos de cliente para import.
type ClientImportData struct {
	ClientID      string   `json:"client_id"`
	Name          string   `json:"name"`
	Description   string   `json:"description,omitempty"`
	ClientType    string   `json:"client_type"` // "public" | "confidential"
	RedirectURIs  []string `json:"redirect_uris,omitempty"`
	AllowedScopes []string `json:"allowed_scopes,omitempty"`
	TokenTTL      int      `json:"token_ttl,omitempty"`
	RefreshTTL    int      `json:"refresh_ttl,omitempty"`
}

// ScopeImportData datos de scope para import.
type ScopeImportData struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Claims      []string `json:"claims,omitempty"`
	System      bool     `json:"system,omitempty"`
}

// UserImportData datos de usuario para import.
// NOTA: No se importan passwords encriptados por seguridad.
type UserImportData struct {
	Email         string                 `json:"email"`
	Username      string                 `json:"username,omitempty"`
	EmailVerified bool                   `json:"email_verified,omitempty"`
	Disabled      bool                   `json:"disabled,omitempty"`
	Roles         []string               `json:"roles,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	// SetPasswordOnImport: si true, se genera password temporal
	SetPasswordOnImport bool `json:"set_password_on_import,omitempty"`
}

// RoleImportData datos de rol para import.
type RoleImportData struct {
	Name         string   `json:"name"`
	Description  string   `json:"description,omitempty"`
	InheritsFrom string   `json:"inherits_from,omitempty"`
	Permissions  []string `json:"permissions,omitempty"`
}

// ─── Import Validation Response ───

// ImportValidationResult resultado de validación de import (dry-run).
type ImportValidationResult struct {
	Valid     bool           `json:"valid"`
	Errors    []string       `json:"errors,omitempty"`
	Warnings  []string       `json:"warnings,omitempty"`
	Conflicts []ConflictInfo `json:"conflicts,omitempty"`
	Summary   ImportSummary  `json:"summary"`
}

// ConflictInfo información sobre un conflicto detectado.
type ConflictInfo struct {
	Type       string `json:"type"`       // "client", "scope", "user", "role"
	Identifier string `json:"identifier"` // ID o nombre del recurso en conflicto
	Existing   string `json:"existing"`   // Descripción del existente
	Incoming   string `json:"incoming"`   // Descripción del entrante
	Action     string `json:"action"`     // "skip" | "overwrite" | "merge"
}

// ImportSummary resumen de qué se importará.
type ImportSummary struct {
	TenantName       string `json:"tenant_name"`
	SettingsIncluded bool   `json:"settings_included"`
	ClientsCount     int    `json:"clients_count"`
	ScopesCount      int    `json:"scopes_count"`
	UsersCount       int    `json:"users_count"`
	RolesCount       int    `json:"roles_count"`
}

// ─── Import Result Response ───

// ImportResultResponse resultado de una operación de import.
type ImportResultResponse struct {
	Success         bool          `json:"success"`
	Message         string        `json:"message,omitempty"`
	TenantID        string        `json:"tenant_id,omitempty"`
	TenantSlug      string        `json:"tenant_slug,omitempty"`
	ItemsImported   ImportCounts  `json:"items_imported"`
	ItemsSkipped    ImportCounts  `json:"items_skipped"`
	Errors          []ImportError `json:"errors,omitempty"`
	UsersNeedingPwd []string      `json:"users_needing_password,omitempty"` // Emails de usuarios que necesitan resetear password
}

// ImportCounts conteo de items procesados.
type ImportCounts struct {
	Settings int `json:"settings"`
	Clients  int `json:"clients"`
	Scopes   int `json:"scopes"`
	Users    int `json:"users"`
	Roles    int `json:"roles"`
}

// ImportError error durante importación de un item específico.
type ImportError struct {
	Type       string `json:"type"`       // "client", "scope", "user", "role", "settings"
	Identifier string `json:"identifier"` // ID o nombre del recurso
	Error      string `json:"error"`      // Mensaje de error
}

// ─── Export Options ───

// ExportOptionsRequest opciones para exportar configuración.
type ExportOptionsRequest struct {
	IncludeSettings bool `json:"include_settings"`
	IncludeClients  bool `json:"include_clients"`
	IncludeScopes   bool `json:"include_scopes"`
	IncludeUsers    bool `json:"include_users"`
	IncludeRoles    bool `json:"include_roles"`
}

// TenantExportResponse respuesta de export completo.
type TenantExportResponse struct {
	Version    string                  `json:"version"`
	ExportedAt string                  `json:"exportedAt"`
	Tenant     *TenantImportInfo       `json:"tenant"`
	Settings   *TenantSettingsResponse `json:"settings,omitempty"`
	Clients    []ClientImportData      `json:"clients,omitempty"`
	Scopes     []ScopeImportData       `json:"scopes,omitempty"`
	Users      []UserImportData        `json:"users,omitempty"`
	Roles      []RoleImportData        `json:"roles,omitempty"`
}
