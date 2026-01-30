// Package fs implementa el adapter FileSystem para store/v2.
// Lee archivos YAML directamente, sin dependencias de controlplane/fs.
package fs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	store "github.com/dropDatabas3/hellojohn/internal/store"
)

func init() {
	store.RegisterAdapter(&fsAdapter{})
}

// fsAdapter implementa store.Adapter para FileSystem.
type fsAdapter struct{}

func (a *fsAdapter) Name() string { return "fs" }

func (a *fsAdapter) Connect(ctx context.Context, cfg store.AdapterConfig) (store.AdapterConnection, error) {
	root := cfg.FSRoot
	if root == "" {
		root = "data"
	}

	// Verificar que existe, si no existe lo creamos automáticamente
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			// Crear el directorio raíz automáticamente
			if mkErr := os.MkdirAll(root, 0755); mkErr != nil {
				return nil, fmt.Errorf("fs: failed to create root path %s: %w", root, mkErr)
			}
		} else {
			return nil, fmt.Errorf("fs: root path error: %w", err)
		}
	} else if !info.IsDir() {
		return nil, fmt.Errorf("fs: root path is not a directory: %s", root)
	}

	return &fsConnection{
		root:             root,
		signingMasterKey: cfg.SigningMasterKey,
	}, nil
}

// fsConnection representa una conexión activa al FileSystem.
type fsConnection struct {
	root             string
	mu               sync.RWMutex
	signingMasterKey string // hex, 64 chars - inyectado al iniciar
}

func (c *fsConnection) Name() string { return "fs" }

func (c *fsConnection) Ping(ctx context.Context) error {
	_, err := os.Stat(c.root)
	return err
}

func (c *fsConnection) Close() error { return nil }

// ─── Repositorios soportados ───

func (c *fsConnection) Tenants() repository.TenantRepository { return &tenantRepo{conn: c} }
func (c *fsConnection) Clients() repository.ClientRepository { return &clientRepo{conn: c} }
func (c *fsConnection) Scopes() repository.ScopeRepository   { return &scopeRepo{conn: c} }
func (c *fsConnection) Claims() repository.ClaimRepository   { return &claimRepo{conn: c} }
func (c *fsConnection) Keys() repository.KeyRepository {
	return newKeyRepo(filepath.Join(c.root, "keys"), c.signingMasterKey)
}
func (c *fsConnection) Admins() repository.AdminRepository { return newAdminRepo(c.root) }
func (c *fsConnection) AdminRefreshTokens() repository.AdminRefreshTokenRepository {
	return newAdminRefreshTokenRepo(c.root)
}

// Data plane (NO soportado por FS)
func (c *fsConnection) Users() repository.UserRepository             { return nil }
func (c *fsConnection) Tokens() repository.TokenRepository           { return nil }
func (c *fsConnection) MFA() repository.MFARepository                { return nil }
func (c *fsConnection) Consents() repository.ConsentRepository       { return nil }
func (c *fsConnection) RBAC() repository.RBACRepository              { return nil }
func (c *fsConnection) Schema() repository.SchemaRepository          { return nil }
func (c *fsConnection) EmailTokens() repository.EmailTokenRepository { return nil }
func (c *fsConnection) Identities() repository.IdentityRepository    { return nil }
func (c *fsConnection) Sessions() repository.SessionRepository       { return nil }

// ─── Helpers ───

func (c *fsConnection) tenantPath(slug string) string {
	return filepath.Join(c.root, "tenants", slug)
}

func (c *fsConnection) tenantFile(slug string) string {
	return filepath.Join(c.tenantPath(slug), "tenant.yaml")
}

func (c *fsConnection) clientsFile(slug string) string {
	return filepath.Join(c.tenantPath(slug), "clients.yaml")
}

func (c *fsConnection) scopesFile(slug string) string {
	return filepath.Join(c.tenantPath(slug), "scopes.yaml")
}

// ─── TenantRepository ───

type tenantRepo struct{ conn *fsConnection }

func (r *tenantRepo) List(ctx context.Context) ([]repository.Tenant, error) {
	tenantsDir := filepath.Join(r.conn.root, "tenants")
	entries, err := os.ReadDir(tenantsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []repository.Tenant{}, nil
		}
		return nil, fmt.Errorf("fs: read tenants dir: %w", err)
	}

	var tenants []repository.Tenant
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		slug := entry.Name()
		if strings.HasPrefix(slug, ".") {
			continue // Ignorar ocultos
		}
		tenant, err := r.GetBySlug(ctx, slug)
		if err != nil {
			continue // Ignorar tenants inválidos
		}
		tenants = append(tenants, *tenant)
	}
	return tenants, nil
}

func (r *tenantRepo) GetBySlug(ctx context.Context, slug string) (*repository.Tenant, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	data, err := os.ReadFile(r.conn.tenantFile(slug))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, repository.ErrNotFound
		}
		return nil, fmt.Errorf("fs: read tenant file: %w", err)
	}

	var raw tenantYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("fs: parse tenant yaml: %w", err)
	}

	return raw.toRepository(slug), nil
}

func (r *tenantRepo) GetByID(ctx context.Context, id string) (*repository.Tenant, error) {
	// Buscar en todos los tenants
	tenants, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tenants {
		if t.ID == id {
			return &t, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *tenantRepo) Create(ctx context.Context, tenant *repository.Tenant) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	// Verificar que no existe
	tenantPath := r.conn.tenantPath(tenant.Slug)
	if _, err := os.Stat(tenantPath); err == nil {
		return repository.ErrConflict
	}

	// Crear directorio
	if err := os.MkdirAll(tenantPath, 0755); err != nil {
		return fmt.Errorf("fs: create tenant dir: %w", err)
	}

	// Escribir tenant.yaml
	raw := toTenantYAML(tenant)
	data, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("fs: marshal tenant: %w", err)
	}

	return os.WriteFile(r.conn.tenantFile(tenant.Slug), data, 0644)
}

func (r *tenantRepo) Update(ctx context.Context, tenant *repository.Tenant) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	// Verificar que existe
	if _, err := os.Stat(r.conn.tenantFile(tenant.Slug)); os.IsNotExist(err) {
		return repository.ErrNotFound
	}

	raw := toTenantYAML(tenant)
	data, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("fs: marshal tenant: %w", err)
	}

	return os.WriteFile(r.conn.tenantFile(tenant.Slug), data, 0644)
}

func (r *tenantRepo) Delete(ctx context.Context, slug string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	tenantPath := r.conn.tenantPath(slug)
	if _, err := os.Stat(tenantPath); os.IsNotExist(err) {
		return repository.ErrNotFound
	}

	return os.RemoveAll(tenantPath)
}

func (r *tenantRepo) UpdateSettings(ctx context.Context, slug string, settings *repository.TenantSettings) error {
	tenant, err := r.GetBySlug(ctx, slug)
	if err != nil {
		return err
	}
	tenant.Settings = *settings
	return r.Update(ctx, tenant)
}

// ─── ClientRepository ───

type clientRepo struct{ conn *fsConnection }

func (r *clientRepo) Get(ctx context.Context, tenantID, clientID string) (*repository.Client, error) {
	clients, err := r.List(ctx, tenantID, "")
	if err != nil {
		return nil, err
	}
	for _, c := range clients {
		if c.ClientID == clientID {
			return &c, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *clientRepo) GetByUUID(ctx context.Context, uuid string) (*repository.Client, *repository.ClientVersion, error) {
	return nil, nil, repository.ErrNotImplemented
}

func (r *clientRepo) List(ctx context.Context, tenantID string, query string) ([]repository.Client, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	data, err := os.ReadFile(r.conn.clientsFile(tenantID))
	if err != nil {
		if os.IsNotExist(err) {
			return []repository.Client{}, nil
		}
		return nil, fmt.Errorf("fs: read clients file: %w", err)
	}

	var raw clientsYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("fs: parse clients yaml: %w", err)
	}

	var clients []repository.Client
	for _, c := range raw.Clients {
		client := c.toRepository(tenantID)
		if query == "" || strings.Contains(strings.ToLower(client.Name), strings.ToLower(query)) {
			clients = append(clients, *client)
		}
	}
	return clients, nil
}

func (r *clientRepo) Create(ctx context.Context, tenantID string, input repository.ClientInput) (*repository.Client, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	// Leer existentes
	clients, err := r.listRaw(tenantID)
	if err != nil {
		return nil, err
	}

	// Verificar que no existe
	for _, c := range clients {
		if c.ClientID == input.ClientID {
			return nil, repository.ErrConflict
		}
	}

	// Agregar nuevo
	newClient := clientYAML{
		ClientID:                 input.ClientID,
		Name:                     input.Name,
		Type:                     input.Type,
		RedirectURIs:             input.RedirectURIs,
		AllowedOrigins:           input.AllowedOrigins,
		Providers:                input.Providers,
		Scopes:                   input.Scopes,
		RequireEmailVerification: input.RequireEmailVerification,
	}
	clients = append(clients, newClient)

	// Escribir
	if err := r.writeClients(tenantID, clients); err != nil {
		return nil, err
	}

	return newClient.toRepository(tenantID), nil
}

func (r *clientRepo) Update(ctx context.Context, tenantID string, input repository.ClientInput) (*repository.Client, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	clients, err := r.listRaw(tenantID)
	if err != nil {
		return nil, err
	}

	found := false
	for i, c := range clients {
		if c.ClientID == input.ClientID {
			clients[i] = clientYAML{
				ClientID:                 input.ClientID,
				Name:                     input.Name,
				Type:                     input.Type,
				RedirectURIs:             input.RedirectURIs,
				AllowedOrigins:           input.AllowedOrigins,
				Providers:                input.Providers,
				Scopes:                   input.Scopes,
				RequireEmailVerification: input.RequireEmailVerification,
			}
			found = true
			break
		}
	}

	if !found {
		return nil, repository.ErrNotFound
	}

	if err := r.writeClients(tenantID, clients); err != nil {
		return nil, err
	}

	return r.Get(ctx, tenantID, input.ClientID)
}

func (r *clientRepo) Delete(ctx context.Context, tenantID, clientID string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	clients, err := r.listRaw(tenantID)
	if err != nil {
		return err
	}

	var filtered []clientYAML
	found := false
	for _, c := range clients {
		if c.ClientID == clientID {
			found = true
			continue
		}
		filtered = append(filtered, c)
	}

	if !found {
		return repository.ErrNotFound
	}

	return r.writeClients(tenantID, filtered)
}

func (r *clientRepo) DecryptSecret(ctx context.Context, tenantID, clientID string) (string, error) {
	// TODO: Implementar descifrado con secretbox
	return "", repository.ErrNotImplemented
}

func (r *clientRepo) ValidateClientID(id string) bool {
	return len(id) >= 3 && len(id) <= 64
}

func (r *clientRepo) ValidateRedirectURI(uri string) bool {
	uri = strings.ToLower(strings.TrimSpace(uri))
	if strings.HasPrefix(uri, "https://") {
		return true
	}
	if strings.HasPrefix(uri, "http://localhost") || strings.HasPrefix(uri, "http://127.0.0.1") {
		return true
	}
	return false
}

func (r *clientRepo) IsScopeAllowed(client *repository.Client, scope string) bool {
	for _, s := range client.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func (r *clientRepo) listRaw(tenantID string) ([]clientYAML, error) {
	data, err := os.ReadFile(r.conn.clientsFile(tenantID))
	if err != nil {
		if os.IsNotExist(err) {
			return []clientYAML{}, nil
		}
		return nil, err
	}
	var raw clientsYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw.Clients, nil
}

func (r *clientRepo) writeClients(tenantID string, clients []clientYAML) error {
	raw := clientsYAML{Clients: clients}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(r.conn.clientsFile(tenantID), data, 0644)
}

// ─── ScopeRepository ───

type scopeRepo struct{ conn *fsConnection }

func (r *scopeRepo) Create(ctx context.Context, tenantID string, input repository.ScopeInput) (*repository.Scope, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	scopes, err := r.listRaw(tenantID)
	if err != nil {
		return nil, err
	}

	for _, s := range scopes {
		if s.Name == input.Name {
			return nil, repository.ErrConflict
		}
	}

	now := time.Now()
	newScope := scopeYAML{
		Name:        input.Name,
		Description: input.Description,
		DisplayName: input.DisplayName,
		Claims:      input.Claims,
		DependsOn:   input.DependsOn,
		System:      input.System,
		CreatedAt:   now.Format(time.RFC3339),
		UpdatedAt:   now.Format(time.RFC3339),
	}
	scopes = append(scopes, newScope)

	if err := r.writeScopes(tenantID, scopes); err != nil {
		return nil, err
	}

	return &repository.Scope{
		TenantID:    tenantID,
		Name:        input.Name,
		Description: input.Description,
		DisplayName: input.DisplayName,
		Claims:      input.Claims,
		DependsOn:   input.DependsOn,
		System:      input.System,
		CreatedAt:   now,
		UpdatedAt:   &now,
	}, nil
}

func (r *scopeRepo) GetByName(ctx context.Context, tenantID, name string) (*repository.Scope, error) {
	scopes, err := r.List(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	for _, s := range scopes {
		if s.Name == name {
			return &s, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *scopeRepo) List(ctx context.Context, tenantID string) ([]repository.Scope, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	scopes, err := r.listRaw(tenantID)
	if err != nil {
		return nil, err
	}

	var result []repository.Scope
	for _, s := range scopes {
		scope := repository.Scope{
			TenantID:    tenantID,
			Name:        s.Name,
			Description: s.Description,
			DisplayName: s.DisplayName,
			Claims:      s.Claims,
			DependsOn:   s.DependsOn,
			System:      s.System,
		}
		if s.CreatedAt != "" {
			if t, err := time.Parse(time.RFC3339, s.CreatedAt); err == nil {
				scope.CreatedAt = t
			}
		}
		if s.UpdatedAt != "" {
			if t, err := time.Parse(time.RFC3339, s.UpdatedAt); err == nil {
				scope.UpdatedAt = &t
			}
		}
		result = append(result, scope)
	}
	return result, nil
}

func (r *scopeRepo) Update(ctx context.Context, tenantID string, input repository.ScopeInput) (*repository.Scope, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	scopes, err := r.listRaw(tenantID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	found := false
	var updatedScope repository.Scope
	for i, s := range scopes {
		if s.Name == input.Name {
			scopes[i].Description = input.Description
			scopes[i].DisplayName = input.DisplayName
			scopes[i].Claims = input.Claims
			scopes[i].DependsOn = input.DependsOn
			scopes[i].System = input.System
			scopes[i].UpdatedAt = now.Format(time.RFC3339)

			updatedScope = repository.Scope{
				TenantID:    tenantID,
				Name:        input.Name,
				Description: input.Description,
				DisplayName: input.DisplayName,
				Claims:      input.Claims,
				DependsOn:   input.DependsOn,
				System:      input.System,
				UpdatedAt:   &now,
			}
			if scopes[i].CreatedAt != "" {
				if t, err := time.Parse(time.RFC3339, scopes[i].CreatedAt); err == nil {
					updatedScope.CreatedAt = t
				}
			}
			found = true
			break
		}
	}

	if !found {
		return nil, repository.ErrNotFound
	}

	if err := r.writeScopes(tenantID, scopes); err != nil {
		return nil, err
	}
	return &updatedScope, nil
}

func (r *scopeRepo) Delete(ctx context.Context, tenantID, scopeID string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	scopes, err := r.listRaw(tenantID)
	if err != nil {
		return err
	}

	var filtered []scopeYAML
	found := false
	for _, s := range scopes {
		if s.Name == scopeID {
			found = true
			continue
		}
		filtered = append(filtered, s)
	}

	if !found {
		return repository.ErrNotFound
	}

	return r.writeScopes(tenantID, filtered)
}

func (r *scopeRepo) listRaw(tenantID string) ([]scopeYAML, error) {
	data, err := os.ReadFile(r.conn.scopesFile(tenantID))
	if err != nil {
		if os.IsNotExist(err) {
			return []scopeYAML{}, nil
		}
		return nil, err
	}
	var raw scopesYAML
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw.Scopes, nil
}

func (r *scopeRepo) Upsert(ctx context.Context, tenantID string, input repository.ScopeInput) (*repository.Scope, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	scopes, err := r.listRaw(tenantID)
	if err != nil {
		return nil, err
	}

	now := time.Now()

	// Buscar si existe
	for i, s := range scopes {
		if s.Name == input.Name {
			// Existe: actualizar todos los campos
			scopes[i].Description = input.Description
			scopes[i].DisplayName = input.DisplayName
			scopes[i].Claims = input.Claims
			scopes[i].DependsOn = input.DependsOn
			scopes[i].System = input.System
			scopes[i].UpdatedAt = now.Format(time.RFC3339)

			if err := r.writeScopes(tenantID, scopes); err != nil {
				return nil, err
			}

			scope := &repository.Scope{
				TenantID:    tenantID,
				Name:        input.Name,
				Description: input.Description,
				DisplayName: input.DisplayName,
				Claims:      input.Claims,
				DependsOn:   input.DependsOn,
				System:      input.System,
				UpdatedAt:   &now,
			}
			if scopes[i].CreatedAt != "" {
				if t, err := time.Parse(time.RFC3339, scopes[i].CreatedAt); err == nil {
					scope.CreatedAt = t
				}
			}
			return scope, nil
		}
	}

	// No existe: crear
	newScope := scopeYAML{
		Name:        input.Name,
		Description: input.Description,
		DisplayName: input.DisplayName,
		Claims:      input.Claims,
		DependsOn:   input.DependsOn,
		System:      input.System,
		CreatedAt:   now.Format(time.RFC3339),
		UpdatedAt:   now.Format(time.RFC3339),
	}
	scopes = append(scopes, newScope)

	if err := r.writeScopes(tenantID, scopes); err != nil {
		return nil, err
	}

	return &repository.Scope{
		TenantID:    tenantID,
		Name:        input.Name,
		Description: input.Description,
		DisplayName: input.DisplayName,
		Claims:      input.Claims,
		DependsOn:   input.DependsOn,
		System:      input.System,
		CreatedAt:   now,
		UpdatedAt:   &now,
	}, nil
}

func (r *scopeRepo) writeScopes(tenantID string, scopes []scopeYAML) error {
	raw := scopesYAML{Scopes: scopes}
	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}
	return os.WriteFile(r.conn.scopesFile(tenantID), data, 0644)
}

// ─── ClaimRepository ───

type claimRepo struct{ conn *fsConnection }

func (r *claimRepo) claimsFile(tenantID string) string {
	return filepath.Join(r.conn.tenantPath(tenantID), "claims.yaml")
}

func (r *claimRepo) Create(ctx context.Context, tenantID string, input repository.ClaimInput) (*repository.ClaimDefinition, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	// Check for duplicate name
	for _, c := range data.CustomClaims {
		if c.Name == input.Name {
			return nil, repository.ErrConflict
		}
	}

	now := time.Now()
	newClaim := customClaimYAML{
		ID:            uuid.New().String(),
		Name:          input.Name,
		Description:   input.Description,
		Source:        input.Source,
		Value:         input.Value,
		AlwaysInclude: input.AlwaysInclude,
		Scopes:        input.Scopes,
		Enabled:       input.Enabled,
		System:        false,
		CreatedAt:     now.Format(time.RFC3339),
		UpdatedAt:     now.Format(time.RFC3339),
	}
	data.CustomClaims = append(data.CustomClaims, newClaim)

	if err := r.saveClaimsFile(tenantID, data); err != nil {
		return nil, err
	}

	return &repository.ClaimDefinition{
		ID:            newClaim.ID,
		TenantID:      tenantID,
		Name:          input.Name,
		Description:   input.Description,
		Source:        input.Source,
		Value:         input.Value,
		AlwaysInclude: input.AlwaysInclude,
		Scopes:        input.Scopes,
		Enabled:       input.Enabled,
		System:        false,
		CreatedAt:     now,
		UpdatedAt:     now,
	}, nil
}

func (r *claimRepo) Get(ctx context.Context, tenantID, claimID string) (*repository.ClaimDefinition, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	for _, c := range data.CustomClaims {
		if c.ID == claimID {
			return r.yamlToDefinition(tenantID, c), nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *claimRepo) GetByName(ctx context.Context, tenantID, name string) (*repository.ClaimDefinition, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	for _, c := range data.CustomClaims {
		if c.Name == name {
			return r.yamlToDefinition(tenantID, c), nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *claimRepo) List(ctx context.Context, tenantID string) ([]repository.ClaimDefinition, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	result := make([]repository.ClaimDefinition, 0, len(data.CustomClaims))
	for _, c := range data.CustomClaims {
		result = append(result, *r.yamlToDefinition(tenantID, c))
	}
	return result, nil
}

func (r *claimRepo) Update(ctx context.Context, tenantID, claimID string, input repository.ClaimInput) (*repository.ClaimDefinition, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	for i, c := range data.CustomClaims {
		if c.ID == claimID {
			data.CustomClaims[i].Description = input.Description
			data.CustomClaims[i].Source = input.Source
			data.CustomClaims[i].Value = input.Value
			data.CustomClaims[i].AlwaysInclude = input.AlwaysInclude
			data.CustomClaims[i].Scopes = input.Scopes
			data.CustomClaims[i].Enabled = input.Enabled
			data.CustomClaims[i].UpdatedAt = now.Format(time.RFC3339)

			if err := r.saveClaimsFile(tenantID, data); err != nil {
				return nil, err
			}
			return r.yamlToDefinition(tenantID, data.CustomClaims[i]), nil
		}
	}
	return nil, repository.ErrNotFound
}

func (r *claimRepo) Delete(ctx context.Context, tenantID, claimID string) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return err
	}

	found := false
	filtered := make([]customClaimYAML, 0, len(data.CustomClaims))
	for _, c := range data.CustomClaims {
		if c.ID == claimID {
			found = true
			continue
		}
		filtered = append(filtered, c)
	}

	if !found {
		return repository.ErrNotFound
	}

	data.CustomClaims = filtered
	return r.saveClaimsFile(tenantID, data)
}

func (r *claimRepo) GetStandardClaimsConfig(ctx context.Context, tenantID string) ([]repository.StandardClaimConfig, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	result := make([]repository.StandardClaimConfig, 0, len(data.StandardClaims))
	for _, sc := range data.StandardClaims {
		result = append(result, repository.StandardClaimConfig{
			ClaimName:   sc.Name,
			Description: sc.Description,
			Enabled:     sc.Enabled,
			Scope:       sc.Scope,
		})
	}
	return result, nil
}

func (r *claimRepo) SetStandardClaimEnabled(ctx context.Context, tenantID, claimName string, enabled bool) error {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return err
	}

	for i, sc := range data.StandardClaims {
		if sc.Name == claimName {
			data.StandardClaims[i].Enabled = enabled
			return r.saveClaimsFile(tenantID, data)
		}
	}
	return repository.ErrNotFound
}

func (r *claimRepo) GetSettings(ctx context.Context, tenantID string) (*repository.ClaimsSettings, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	return &repository.ClaimsSettings{
		TenantID:             tenantID,
		IncludeInAccessToken: data.Settings.IncludeInAccessToken,
		UseNamespacedClaims:  data.Settings.UseNamespacedClaims,
		NamespacePrefix:      data.Settings.NamespacePrefix,
	}, nil
}

func (r *claimRepo) UpdateSettings(ctx context.Context, tenantID string, input repository.ClaimsSettingsInput) (*repository.ClaimsSettings, error) {
	r.conn.mu.Lock()
	defer r.conn.mu.Unlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	if input.IncludeInAccessToken != nil {
		data.Settings.IncludeInAccessToken = *input.IncludeInAccessToken
	}
	if input.UseNamespacedClaims != nil {
		data.Settings.UseNamespacedClaims = *input.UseNamespacedClaims
	}
	if input.NamespacePrefix != nil {
		data.Settings.NamespacePrefix = input.NamespacePrefix
	}

	if err := r.saveClaimsFile(tenantID, data); err != nil {
		return nil, err
	}

	return &repository.ClaimsSettings{
		TenantID:             tenantID,
		IncludeInAccessToken: data.Settings.IncludeInAccessToken,
		UseNamespacedClaims:  data.Settings.UseNamespacedClaims,
		NamespacePrefix:      data.Settings.NamespacePrefix,
	}, nil
}

func (r *claimRepo) GetEnabledClaimsForScopes(ctx context.Context, tenantID string, scopes []string) ([]repository.ClaimDefinition, error) {
	r.conn.mu.RLock()
	defer r.conn.mu.RUnlock()

	data, err := r.loadClaimsFile(tenantID)
	if err != nil {
		return nil, err
	}

	scopeSet := make(map[string]bool)
	for _, s := range scopes {
		scopeSet[s] = true
	}

	var result []repository.ClaimDefinition
	for _, c := range data.CustomClaims {
		if !c.Enabled {
			continue
		}
		if c.AlwaysInclude {
			result = append(result, *r.yamlToDefinition(tenantID, c))
			continue
		}
		for _, cs := range c.Scopes {
			if scopeSet[cs] {
				result = append(result, *r.yamlToDefinition(tenantID, c))
				break
			}
		}
	}
	return result, nil
}

func (r *claimRepo) loadClaimsFile(tenantID string) (*claimsFileYAML, error) {
	path := r.claimsFile(tenantID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return r.defaultClaimsConfig(), nil
		}
		return nil, err
	}
	var cfg claimsFileYAML
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (r *claimRepo) saveClaimsFile(tenantID string, data *claimsFileYAML) error {
	raw, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	return os.WriteFile(r.claimsFile(tenantID), raw, 0644)
}

func (r *claimRepo) yamlToDefinition(tenantID string, c customClaimYAML) *repository.ClaimDefinition {
	def := &repository.ClaimDefinition{
		ID:            c.ID,
		TenantID:      tenantID,
		Name:          c.Name,
		Description:   c.Description,
		Source:        c.Source,
		Value:         c.Value,
		AlwaysInclude: c.AlwaysInclude,
		Scopes:        c.Scopes,
		Enabled:       c.Enabled,
		System:        c.System,
	}
	if c.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, c.CreatedAt); err == nil {
			def.CreatedAt = t
		}
	}
	if c.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, c.UpdatedAt); err == nil {
			def.UpdatedAt = t
		}
	}
	return def
}

func (r *claimRepo) defaultClaimsConfig() *claimsFileYAML {
	return &claimsFileYAML{
		StandardClaims: []standardClaimYAML{
			{Name: "sub", Description: "Subject identifier", Enabled: true, Scope: "openid"},
			{Name: "email", Description: "User email address", Enabled: true, Scope: "email"},
			{Name: "email_verified", Description: "Email verified flag", Enabled: true, Scope: "email"},
			{Name: "name", Description: "Full name", Enabled: true, Scope: "profile"},
			{Name: "given_name", Description: "First name", Enabled: true, Scope: "profile"},
			{Name: "family_name", Description: "Last name", Enabled: true, Scope: "profile"},
			{Name: "picture", Description: "Profile picture URL", Enabled: true, Scope: "profile"},
			{Name: "locale", Description: "User locale", Enabled: true, Scope: "profile"},
			{Name: "zoneinfo", Description: "Timezone", Enabled: true, Scope: "profile"},
			{Name: "updated_at", Description: "Last update timestamp", Enabled: true, Scope: "profile"},
			{Name: "address", Description: "Physical address", Enabled: true, Scope: "address"},
			{Name: "phone_number", Description: "Phone number", Enabled: true, Scope: "phone"},
			{Name: "phone_number_verified", Description: "Phone verified flag", Enabled: true, Scope: "phone"},
		},
		CustomClaims: []customClaimYAML{},
		Settings: claimsSettingsYAML{
			IncludeInAccessToken: true,
			UseNamespacedClaims:  false,
			NamespacePrefix:      nil,
		},
	}
}

// ─── Claims YAML Types ───

type claimsFileYAML struct {
	StandardClaims []standardClaimYAML `yaml:"standard_claims"`
	CustomClaims   []customClaimYAML   `yaml:"custom_claims"`
	Settings       claimsSettingsYAML  `yaml:"settings"`
}

type standardClaimYAML struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description,omitempty"`
	Enabled     bool   `yaml:"enabled"`
	Scope       string `yaml:"scope"`
}

type customClaimYAML struct {
	ID            string   `yaml:"id"`
	Name          string   `yaml:"name"`
	Description   string   `yaml:"description,omitempty"`
	Source        string   `yaml:"source"`
	Value         string   `yaml:"value"`
	AlwaysInclude bool     `yaml:"always_include"`
	Scopes        []string `yaml:"scopes,omitempty"`
	Enabled       bool     `yaml:"enabled"`
	System        bool     `yaml:"system,omitempty"`
	CreatedAt     string   `yaml:"created_at,omitempty"`
	UpdatedAt     string   `yaml:"updated_at,omitempty"`
}

type claimsSettingsYAML struct {
	IncludeInAccessToken bool    `yaml:"include_in_access_token"`
	UseNamespacedClaims  bool    `yaml:"use_namespaced_claims"`
	NamespacePrefix      *string `yaml:"namespace_prefix,omitempty"`
}

// ─── YAML Types ───

type tenantYAML struct {
	ID          string             `yaml:"id"`
	Name        string             `yaml:"name"`
	DisplayName string             `yaml:"display_name,omitempty"`
	CreatedAt   time.Time          `yaml:"createdAt,omitempty"`
	UpdatedAt   time.Time          `yaml:"updatedAt,omitempty"`
	Settings    tenantSettingsYAML `yaml:"settings,omitempty"`
}

type tenantSettingsYAML struct {
	LogoURL                     string `yaml:"logoUrl,omitempty"`
	BrandColor                  string `yaml:"brandColor,omitempty"`
	SessionLifetimeSeconds      int    `yaml:"sessionLifetimeSeconds,omitempty"`
	RefreshTokenLifetimeSeconds int    `yaml:"refreshTokenLifetimeSeconds,omitempty"`
	MFAEnabled                  bool   `yaml:"mfaEnabled,omitempty"`
	SocialLoginEnabled          bool   `yaml:"social_login_enabled,omitempty"`
	IssuerMode                  string `yaml:"issuerMode,omitempty"`
	IssuerOverride              string `yaml:"issuerOverride,omitempty"`

	SMTP *struct {
		Host        string `yaml:"host,omitempty"`
		Port        int    `yaml:"port,omitempty"`
		Username    string `yaml:"username,omitempty"`
		PasswordEnc string `yaml:"passwordEnc,omitempty"`
		FromEmail   string `yaml:"fromEmail,omitempty"`
		UseTLS      bool   `yaml:"useTLS,omitempty"`
	} `yaml:"smtp,omitempty"`

	UserDB *struct {
		Driver     string `yaml:"driver,omitempty"`
		DSNEnc     string `yaml:"dsnEnc,omitempty"`
		DSN        string `yaml:"dsn,omitempty"`
		Schema     string `yaml:"schema,omitempty"`
		ManualMode bool   `yaml:"manualMode,omitempty"`
	} `yaml:"userDb,omitempty"`

	Cache *struct {
		Enabled  bool   `yaml:"enabled"`
		Driver   string `yaml:"driver,omitempty"`
		Host     string `yaml:"host,omitempty"`
		Port     int    `yaml:"port,omitempty"`
		Password string `yaml:"password,omitempty"`
		PassEnc  string `yaml:"passEnc,omitempty"`
		DB       int    `yaml:"db,omitempty"`
		Prefix   string `yaml:"prefix,omitempty"`
	} `yaml:"cache,omitempty"`

	SocialProviders *struct {
		GoogleEnabled   bool   `yaml:"googleEnabled,omitempty"`
		GoogleClient    string `yaml:"googleClient,omitempty"`
		GoogleSecretEnc string `yaml:"googleSecretEnc,omitempty"`
	} `yaml:"socialProviders,omitempty"`

	UserFields []userFieldYAML `yaml:"userFields,omitempty"`
}

// userFieldYAML representa un campo custom de usuario para serialización YAML.
type userFieldYAML struct {
	Name        string `yaml:"name"`
	Type        string `yaml:"type"`
	Required    bool   `yaml:"required,omitempty"`
	Unique      bool   `yaml:"unique,omitempty"`
	Indexed     bool   `yaml:"indexed,omitempty"`
	Description string `yaml:"description,omitempty"`
}

func (t *tenantYAML) toRepository(slug string) *repository.Tenant {
	tenant := &repository.Tenant{
		ID:          t.ID,
		Slug:        slug,
		Name:        t.Name,
		DisplayName: t.DisplayName,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
		Settings: repository.TenantSettings{
			LogoURL:                     t.Settings.LogoURL,
			BrandColor:                  t.Settings.BrandColor,
			SessionLifetimeSeconds:      t.Settings.SessionLifetimeSeconds,
			RefreshTokenLifetimeSeconds: t.Settings.RefreshTokenLifetimeSeconds,
			MFAEnabled:                  t.Settings.MFAEnabled,
			SocialLoginEnabled:          t.Settings.SocialLoginEnabled,
			IssuerMode:                  t.Settings.IssuerMode,
			IssuerOverride:              t.Settings.IssuerOverride,
		},
	}

	if t.Settings.SMTP != nil {
		tenant.Settings.SMTP = &repository.SMTPSettings{
			Host:        t.Settings.SMTP.Host,
			Port:        t.Settings.SMTP.Port,
			Username:    t.Settings.SMTP.Username,
			PasswordEnc: t.Settings.SMTP.PasswordEnc,
			FromEmail:   t.Settings.SMTP.FromEmail,
			UseTLS:      t.Settings.SMTP.UseTLS,
		}
	}

	if t.Settings.UserDB != nil {
		tenant.Settings.UserDB = &repository.UserDBSettings{
			Driver:     t.Settings.UserDB.Driver,
			DSNEnc:     t.Settings.UserDB.DSNEnc,
			DSN:        t.Settings.UserDB.DSN,
			Schema:     t.Settings.UserDB.Schema,
			ManualMode: t.Settings.UserDB.ManualMode,
		}
	}

	if t.Settings.Cache != nil {
		tenant.Settings.Cache = &repository.CacheSettings{
			Enabled:  t.Settings.Cache.Enabled,
			Driver:   t.Settings.Cache.Driver,
			Host:     t.Settings.Cache.Host,
			Port:     t.Settings.Cache.Port,
			Password: t.Settings.Cache.Password,
			PassEnc:  t.Settings.Cache.PassEnc,
			DB:       t.Settings.Cache.DB,
			Prefix:   t.Settings.Cache.Prefix,
		}
	}

	if t.Settings.SocialProviders != nil {
		tenant.Settings.SocialProviders = &repository.SocialConfig{
			GoogleEnabled:   t.Settings.SocialProviders.GoogleEnabled,
			GoogleClient:    t.Settings.SocialProviders.GoogleClient,
			GoogleSecretEnc: t.Settings.SocialProviders.GoogleSecretEnc,
		}
	}

	// UserFields
	if len(t.Settings.UserFields) > 0 {
		tenant.Settings.UserFields = make([]repository.UserFieldDefinition, len(t.Settings.UserFields))
		for i, uf := range t.Settings.UserFields {
			tenant.Settings.UserFields[i] = repository.UserFieldDefinition{
				Name:        uf.Name,
				Type:        uf.Type,
				Required:    uf.Required,
				Unique:      uf.Unique,
				Indexed:     uf.Indexed,
				Description: uf.Description,
			}
		}
	}

	return tenant
}

func toTenantYAML(t *repository.Tenant) *tenantYAML {
	y := &tenantYAML{
		ID:          t.ID,
		Name:        t.Name,
		DisplayName: t.DisplayName,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   time.Now(),
		Settings: tenantSettingsYAML{
			LogoURL:                     t.Settings.LogoURL,
			BrandColor:                  t.Settings.BrandColor,
			SessionLifetimeSeconds:      t.Settings.SessionLifetimeSeconds,
			RefreshTokenLifetimeSeconds: t.Settings.RefreshTokenLifetimeSeconds,
			MFAEnabled:                  t.Settings.MFAEnabled,
			SocialLoginEnabled:          t.Settings.SocialLoginEnabled,
			IssuerMode:                  t.Settings.IssuerMode,
			IssuerOverride:              t.Settings.IssuerOverride,
		},
	}

	// SMTP
	if t.Settings.SMTP != nil {
		y.Settings.SMTP = &struct {
			Host        string `yaml:"host,omitempty"`
			Port        int    `yaml:"port,omitempty"`
			Username    string `yaml:"username,omitempty"`
			PasswordEnc string `yaml:"passwordEnc,omitempty"`
			FromEmail   string `yaml:"fromEmail,omitempty"`
			UseTLS      bool   `yaml:"useTLS,omitempty"`
		}{
			Host:        t.Settings.SMTP.Host,
			Port:        t.Settings.SMTP.Port,
			Username:    t.Settings.SMTP.Username,
			PasswordEnc: t.Settings.SMTP.PasswordEnc,
			FromEmail:   t.Settings.SMTP.FromEmail,
			UseTLS:      t.Settings.SMTP.UseTLS,
		}
	}

	// UserDB
	if t.Settings.UserDB != nil {
		y.Settings.UserDB = &struct {
			Driver     string `yaml:"driver,omitempty"`
			DSNEnc     string `yaml:"dsnEnc,omitempty"`
			DSN        string `yaml:"dsn,omitempty"`
			Schema     string `yaml:"schema,omitempty"`
			ManualMode bool   `yaml:"manualMode,omitempty"`
		}{
			Driver:     t.Settings.UserDB.Driver,
			DSNEnc:     t.Settings.UserDB.DSNEnc,
			DSN:        t.Settings.UserDB.DSN,
			Schema:     t.Settings.UserDB.Schema,
			ManualMode: t.Settings.UserDB.ManualMode,
		}
	}

	// Cache
	if t.Settings.Cache != nil {
		y.Settings.Cache = &struct {
			Enabled  bool   `yaml:"enabled"`
			Driver   string `yaml:"driver,omitempty"`
			Host     string `yaml:"host,omitempty"`
			Port     int    `yaml:"port,omitempty"`
			Password string `yaml:"password,omitempty"`
			PassEnc  string `yaml:"passEnc,omitempty"`
			DB       int    `yaml:"db,omitempty"`
			Prefix   string `yaml:"prefix,omitempty"`
		}{
			Enabled:  t.Settings.Cache.Enabled,
			Driver:   t.Settings.Cache.Driver,
			Host:     t.Settings.Cache.Host,
			Port:     t.Settings.Cache.Port,
			Password: t.Settings.Cache.Password,
			PassEnc:  t.Settings.Cache.PassEnc,
			DB:       t.Settings.Cache.DB,
			Prefix:   t.Settings.Cache.Prefix,
		}
	}

	// SocialProviders
	if t.Settings.SocialProviders != nil {
		y.Settings.SocialProviders = &struct {
			GoogleEnabled   bool   `yaml:"googleEnabled,omitempty"`
			GoogleClient    string `yaml:"googleClient,omitempty"`
			GoogleSecretEnc string `yaml:"googleSecretEnc,omitempty"`
		}{
			GoogleEnabled:   t.Settings.SocialProviders.GoogleEnabled,
			GoogleClient:    t.Settings.SocialProviders.GoogleClient,
			GoogleSecretEnc: t.Settings.SocialProviders.GoogleSecretEnc,
		}
	}

	// UserFields
	if len(t.Settings.UserFields) > 0 {
		y.Settings.UserFields = make([]userFieldYAML, len(t.Settings.UserFields))
		for i, uf := range t.Settings.UserFields {
			y.Settings.UserFields[i] = userFieldYAML{
				Name:        uf.Name,
				Type:        uf.Type,
				Required:    uf.Required,
				Unique:      uf.Unique,
				Indexed:     uf.Indexed,
				Description: uf.Description,
			}
		}
	}

	return y
}

type clientsYAML struct {
	Clients []clientYAML `yaml:"clients"`
}

type clientYAML struct {
	ClientID                 string   `yaml:"clientId"`
	Name                     string   `yaml:"name"`
	Type                     string   `yaml:"type"`
	RedirectURIs             []string `yaml:"redirectUris"`
	AllowedOrigins           []string `yaml:"allowedOrigins,omitempty"`
	Providers                []string `yaml:"providers,omitempty"`
	Scopes                   []string `yaml:"scopes,omitempty"`
	SecretEnc                string   `yaml:"secretEnc,omitempty"`
	RequireEmailVerification bool     `yaml:"requireEmailVerification,omitempty"`
}

func (c *clientYAML) toRepository(tenantID string) *repository.Client {
	return &repository.Client{
		TenantID:                 tenantID,
		ClientID:                 c.ClientID,
		Name:                     c.Name,
		Type:                     c.Type,
		RedirectURIs:             c.RedirectURIs,
		AllowedOrigins:           c.AllowedOrigins,
		Providers:                c.Providers,
		Scopes:                   c.Scopes,
		SecretEnc:                c.SecretEnc,
		RequireEmailVerification: c.RequireEmailVerification,
	}
}

type scopesYAML struct {
	Scopes []scopeYAML `yaml:"scopes"`
}

type scopeYAML struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	DisplayName string   `yaml:"display_name,omitempty"`
	Claims      []string `yaml:"claims,omitempty"`
	DependsOn   string   `yaml:"depends_on,omitempty"`
	System      bool     `yaml:"system,omitempty"`
	CreatedAt   string   `yaml:"created_at,omitempty"`
	UpdatedAt   string   `yaml:"updated_at,omitempty"`
}
