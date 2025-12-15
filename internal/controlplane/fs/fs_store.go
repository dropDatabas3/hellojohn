package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
	sec "github.com/dropDatabas3/hellojohn/internal/security/secretbox"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

var (
	ErrTenantNotFound = errors.New("tenant not found")
	ErrClientNotFound = errors.New("client not found")
	ErrScopeInUse     = errors.New("scope in use by a client")
	ErrBadInput       = errors.New("bad input")
)

// FSProvider implementa ControlPlane usando YAML en disco.
type FSProvider struct {
	root string

	mu sync.RWMutex // protege caches a futuro (si agregamos)
	// opcional: agregar caches simples por slug
}

func New(root string) *FSProvider { return &FSProvider{root: filepath.Clean(root)} }

// FSRoot returns the configured FS root directory
func (p *FSProvider) FSRoot() string { return p.root }

func (p *FSProvider) tenantsDir() string           { return filepath.Join(p.root, "tenants") }
func (p *FSProvider) tenantDir(slug string) string { return filepath.Join(p.tenantsDir(), slug) }
func (p *FSProvider) tenantFile(slug string) string {
	return filepath.Join(p.tenantDir(slug), "tenant.yaml")
}
func (p *FSProvider) clientsFile(slug string) string {
	return filepath.Join(p.tenantDir(slug), "clients.yaml")
}
func (p *FSProvider) scopesFile(slug string) string {
	return filepath.Join(p.tenantDir(slug), "scopes.yaml")
}

// ===== helpers FS =====

func readYAML[T any](path string, out *T) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, out); err != nil {
		return err
	}
	return nil
}

func writeYAML(path string, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	if err := atomicWriteFile(path, b, 0o600); err != nil {
		if cpctx.MarkFSDegraded != nil {
			cpctx.MarkFSDegraded(fmt.Sprintf("writeYAML failed: %v", err))
		}
		return err
	}
	if cpctx.ClearFSDegraded != nil {
		cpctx.ClearFSDegraded()
	}
	return nil
}

func ensureTenantLayout(root string, slug string) error {
	dir := filepath.Join(root, "tenants", slug)
	return os.MkdirAll(dir, 0o755)
}

// ensureTenantID assigns a UUID to the tenant if it's missing. Returns true if it changed.
func ensureTenantID(t *cp.Tenant) bool {
	if t == nil {
		return false
	}
	id := strings.TrimSpace(t.ID)
	if id == "" {
		t.ID = uuid.NewString()
		return true
	}
	// If the ID isn't a valid UUID (e.g., legacy files with id=slug), replace it
	if _, err := uuid.Parse(id); err != nil {
		t.ID = uuid.NewString()
		return true
	}
	return false
}

// ===== ControlPlane impl =====

func (p *FSProvider) ListTenants(ctx context.Context) ([]cp.Tenant, error) {
	entries, err := os.ReadDir(p.tenantsDir())
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var out []cp.Tenant
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		t, err := p.GetTenantBySlug(ctx, e.Name())
		if err == nil && t != nil {
			out = append(out, *t)
		}
	}
	// orden estable por slug
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

func (p *FSProvider) GetTenantBySlug(ctx context.Context, slug string) (*cp.Tenant, error) {
	tf := p.tenantFile(slug)
	var t cp.Tenant
	if err := readYAML(tf, &t); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrTenantNotFound
		}
		return nil, err
	}
	// cargar clients/scopes si existen
	var clients []cp.OIDCClient
	if err := readYAML(p.clientsFile(slug), &clients); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	var scopes []cp.Scope
	if err := readYAML(p.scopesFile(slug), &scopes); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	t.Clients = clients
	t.Scopes = scopes
	return &t, nil
}

func (p *FSProvider) GetTenantByID(ctx context.Context, id string) (*cp.Tenant, error) {
	// Scan all tenants to find match by ID
	// This is O(N) but acceptable for FS provider scale
	tenants, err := p.ListTenants(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tenants {
		if t.ID == id {
			// Re-fetch to ensure full details (though ListTenants usually fetches full)
			// ListTenants calls GetTenantBySlug internally so 't' is already fully populated
			return &t, nil
		}
	}
	return nil, ErrTenantNotFound
}

func (p *FSProvider) UpsertTenant(ctx context.Context, t *cp.Tenant) error {
	if strings.TrimSpace(t.Slug) == "" || strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: tenant name/slug required", ErrBadInput)
	}
	// Validar issuer mode (permite vacío como "global")
	if err := validateIssuerMode(t.Settings.IssuerMode); err != nil {
		return err
	}
	if err := ensureTenantLayout(p.root, t.Slug); err != nil {
		return err
	}
	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	// Backfill/ensure UUID ID
	_ = ensureTenantID(t)
	t.UpdatedAt = now
	return writeYAML(p.tenantFile(t.Slug), t)
}

func (p *FSProvider) DeleteTenant(ctx context.Context, slug string) error {
	// Seguridad: no borramos el directorio entero para evitar accidentes en MVP.
	// Sólo renombramos el archivo principal (soft-delete se puede hacer luego).
	tf := p.tenantFile(slug)
	if _, err := os.Stat(tf); os.IsNotExist(err) {
		return ErrTenantNotFound
	}
	// soft delete: tenant.yaml -> tenant.deleted.yaml con timestamp
	newName := filepath.Join(p.tenantDir(slug), fmt.Sprintf("tenant.deleted.%d.yaml", time.Now().Unix()))
	return os.Rename(tf, newName)
}

// ===== Scopes =====

func (p *FSProvider) ListScopes(ctx context.Context, slug string) ([]cp.Scope, error) {
	var scopes []cp.Scope
	if err := readYAML(p.scopesFile(slug), &scopes); err != nil {
		if os.IsNotExist(err) {
			return []cp.Scope{}, nil
		}
		return nil, err
	}
	// ordenar
	sort.Slice(scopes, func(i, j int) bool { return scopes[i].Name < scopes[j].Name })
	return scopes, nil
}

func (p *FSProvider) UpsertScope(ctx context.Context, slug string, s cp.Scope) error {
	if strings.TrimSpace(s.Name) == "" {
		return fmt.Errorf("%w: scope name required", ErrBadInput)
	}
	if err := ensureTenantLayout(p.root, slug); err != nil {
		return err
	}
	scopes, _ := p.ListScopes(ctx, slug)
	// prohibir borrar/editar system=true en otras fases; aquí sólo upsert libre (respetar flag)
	found := false
	for i := range scopes {
		if scopes[i].Name == s.Name {
			// si ya es system, preservamos ese flag
			if scopes[i].System {
				s.System = true
			}
			scopes[i] = s
			found = true
			break
		}
	}
	if !found {
		scopes = append(scopes, s)
	}
	return writeYAML(p.scopesFile(slug), scopes)
}

func (p *FSProvider) DeleteScope(ctx context.Context, slug, name string) error {
	scopes, _ := p.ListScopes(ctx, slug)
	// check system
	for _, sc := range scopes {
		if sc.Name == name && sc.System {
			return fmt.Errorf("cannot delete system scope: %s", name)
		}
	}
	// check in use by any client
	clients, _ := p.ListClients(ctx, slug)
	for _, c := range clients {
		for _, s := range c.Scopes {
			if s == name {
				return ErrScopeInUse
			}
		}
	}
	// filter out
	var out []cp.Scope
	for _, sc := range scopes {
		if sc.Name != name {
			out = append(out, sc)
		}
	}
	return writeYAML(p.scopesFile(slug), out)
}

// ===== Clients =====

func (p *FSProvider) ListClients(ctx context.Context, slug string) ([]cp.OIDCClient, error) {
	var clients []cp.OIDCClient
	if err := readYAML(p.clientsFile(slug), &clients); err != nil {
		if os.IsNotExist(err) {
			return []cp.OIDCClient{}, nil
		}
		return nil, err
	}
	sort.Slice(clients, func(i, j int) bool { return clients[i].ClientID < clients[j].ClientID })
	return clients, nil
}

func (p *FSProvider) GetClient(ctx context.Context, slug, clientID string) (*cp.OIDCClient, error) {
	clients, err := p.ListClients(ctx, slug)
	if err != nil {
		return nil, err
	}
	for i := range clients {
		if clients[i].ClientID == clientID {
			return &clients[i], nil
		}
	}
	return nil, ErrClientNotFound
}

func (p *FSProvider) UpsertClient(ctx context.Context, slug string, in cp.ClientInput) (*cp.OIDCClient, error) {
	// Validaciones mínimas
	if !p.ValidateClientID(strings.TrimSpace(in.ClientID)) {
		return nil, fmt.Errorf("%w: invalid clientId", ErrBadInput)
	}
	if strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("%w: name required", ErrBadInput)
	}
	if in.Type != cp.ClientTypePublic && in.Type != cp.ClientTypeConfidential {
		return nil, fmt.Errorf("%w: invalid client type", ErrBadInput)
	}
	if len(in.RedirectURIs) == 0 {
		return nil, fmt.Errorf("%w: redirectUris required", ErrBadInput)
	}
	for _, u := range in.RedirectURIs {
		if !p.ValidateRedirectURI(u) {
			return nil, fmt.Errorf("%w: invalid redirect uri: %s", ErrBadInput, u)
		}
	}
	if err := ensureTenantLayout(p.root, slug); err != nil {
		return nil, err
	}

	clients, err := p.ListClients(ctx, slug)
	if err != nil {
		return nil, err
	}
	var found *cp.OIDCClient
	for i := range clients {
		if clients[i].ClientID == in.ClientID {
			found = &clients[i]
			break
		}
	}
	if found == nil {
		clients = append(clients, cp.OIDCClient{
			ClientID: in.ClientID,
		})
		found = &clients[len(clients)-1]
	}
	// actualizar campos editables
	found.Name = in.Name
	found.Type = in.Type
	found.RedirectURIs = uniqueStrings(in.RedirectURIs)
	found.AllowedOrigins = uniqueStrings(in.AllowedOrigins)
	found.Providers = uniqueStrings(in.Providers)
	found.Scopes = uniqueStrings(in.Scopes)

	// Email verification & password reset config
	found.RequireEmailVerification = in.RequireEmailVerification
	found.ResetPasswordURL = in.ResetPasswordURL
	found.VerifyEmailURL = in.VerifyEmailURL

	// claims opcionales (MVP): persistir tal cual (validación mínima UI-side)
	if in.ClaimSchema != nil {
		found.ClaimSchema = in.ClaimSchema
	}
	if in.ClaimMapping != nil {
		found.ClaimMapping = in.ClaimMapping
	}

	// secreto: si vino plain en input, cifrar y guardar
	if in.Type == cp.ClientTypeConfidential {
		if strings.TrimSpace(in.Secret) != "" {
			enc, err := sec.Encrypt(in.Secret)
			if err != nil {
				return nil, fmt.Errorf("encrypt client secret: %w", err)
			}
			found.SecretEnc = enc
		} else if strings.TrimSpace(found.SecretEnc) == "" {
			// si es confidencial y no hay secreto aún ⇒ error (creación exige secreto)
			return nil, fmt.Errorf("%w: secret required for confidential client", ErrBadInput)
		}
	} else {
		// public client: no guarda secret
		found.SecretEnc = ""
	}

	// write
	if err := writeYAML(p.clientsFile(slug), clients); err != nil {
		return nil, err
	}
	return found, nil
}

func (p *FSProvider) DeleteClient(ctx context.Context, slug, clientID string) error {
	clients, err := p.ListClients(ctx, slug)
	if err != nil {
		return err
	}
	var out []cp.OIDCClient
	found := false
	for _, c := range clients {
		if c.ClientID == clientID {
			found = true
			continue
		}
		out = append(out, c)
	}
	if !found {
		return ErrClientNotFound
	}
	return writeYAML(p.clientsFile(slug), out)
}

func (p *FSProvider) DecryptClientSecret(ctx context.Context, slug, clientID string) (string, error) {
	c, err := p.GetClient(ctx, slug, clientID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(c.SecretEnc) == "" {
		return "", nil
	}
	return sec.Decrypt(c.SecretEnc)
}

// ===== validations =====

func (p *FSProvider) ValidateClientID(id string) bool     { return cp.DefaultValidateClientID(id) }
func (p *FSProvider) ValidateRedirectURI(uri string) bool { return cp.DefaultValidateRedirectURI(uri) }
func (p *FSProvider) IsScopeAllowed(c *cp.OIDCClient, s string) bool {
	return cp.DefaultIsScopeAllowed(c, s)
}

// ===== Raw helpers para Admin API con versionado =====

// GetTenantRaw retorna el tenant y el YAML raw para versionado
func (p *FSProvider) GetTenantRaw(ctx context.Context, slug string) (*cp.Tenant, []byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.getTenantRawUnsafe(ctx, slug)
}

// getTenantRawUnsafe versión sin lock para uso interno
func (p *FSProvider) getTenantRawUnsafe(ctx context.Context, slug string) (*cp.Tenant, []byte, error) {
	tf := p.tenantFile(slug)
	rawData, err := os.ReadFile(tf)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, ErrTenantNotFound
		}
		return nil, nil, err
	}

	var t cp.Tenant
	if err := yaml.Unmarshal(rawData, &t); err != nil {
		return nil, nil, err
	}

	// cargar clients/scopes si existen
	var clients []cp.OIDCClient
	if err := readYAML(p.clientsFile(slug), &clients); err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	var scopes []cp.Scope
	if err := readYAML(p.scopesFile(slug), &scopes); err != nil && !os.IsNotExist(err) {
		return nil, nil, err
	}
	t.Clients = clients
	t.Scopes = scopes

	return &t, rawData, nil
}

// GetTenantSettingsRaw retorna solo los settings y el YAML raw del tenant completo
func (p *FSProvider) GetTenantSettingsRaw(ctx context.Context, slug string) (*cp.TenantSettings, []byte, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.getTenantSettingsRawUnsafe(ctx, slug)
}

// getTenantSettingsRawUnsafe versión sin lock para uso interno
func (p *FSProvider) getTenantSettingsRawUnsafe(ctx context.Context, slug string) (*cp.TenantSettings, []byte, error) {
	tenant, rawData, err := p.getTenantRawUnsafe(ctx, slug)
	if err != nil {
		return nil, nil, err
	}
	return &tenant.Settings, rawData, nil
}

// UpdateTenantSettings actualiza solo los settings con cifrado de campos sensibles
func (p *FSProvider) UpdateTenantSettings(ctx context.Context, slug string, settings *cp.TenantSettings) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if err := ensureTenantLayout(p.root, slug); err != nil {
		return err
	}

	// Leer el tenant actual para preservar otros campos (sin lock adicional)
	tenant, _, err := p.getTenantRawUnsafe(ctx, slug)
	if err != nil && err != ErrTenantNotFound {
		return err
	}

	if tenant == nil {
		// Si no existe el tenant, creamos uno básico
		now := time.Now().UTC()
		tenant = &cp.Tenant{
			ID:        uuid.NewString(),
			Name:      slug,
			Slug:      slug,
			CreatedAt: now,
			UpdatedAt: now,
		}
	}

	// Backfill/ensure UUID ID when missing or invalid
	_ = ensureTenantID(tenant)

	// Cifrar campos sensibles en settings
	if settings != nil {
		// Validar issuer mode
		if err := validateIssuerMode(settings.IssuerMode); err != nil {
			return err
		}
		encryptedSettings := *settings

		// Cifrar SMTP password si viene en plain
		if encryptedSettings.SMTP != nil {
			if plainPwd := strings.TrimSpace(encryptedSettings.SMTP.Password); plainPwd != "" {
				if encrypted, err := sec.Encrypt(plainPwd); err == nil {
					encryptedSettings.SMTP.PasswordEnc = encrypted
					encryptedSettings.SMTP.Password = "" // Limpiar campo plain
				} else {
					return fmt.Errorf("failed to encrypt SMTP password: %w", err)
				}
			}
		}

		// Cifrar UserDB DSN si viene en plain
		if encryptedSettings.UserDB != nil {
			if plainDSN := strings.TrimSpace(encryptedSettings.UserDB.DSN); plainDSN != "" {
				if encrypted, err := sec.Encrypt(plainDSN); err == nil {
					encryptedSettings.UserDB.DSNEnc = encrypted
					encryptedSettings.UserDB.DSN = "" // Limpiar campo plain
				} else {
					return fmt.Errorf("failed to encrypt UserDB DSN: %w", err)
				}
			}
		}

		tenant.Settings = encryptedSettings
	}

	// Actualizar timestamp
	tenant.UpdatedAt = time.Now().UTC()

	// Escritura atómica con el mismo patrón que keystore
	return p.writeYAMLAtomic(p.tenantFile(slug), tenant)
}

// writeYAMLAtomic implementa el mismo patrón de escritura segura que el keystore
func (p *FSProvider) writeYAMLAtomic(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal yaml: %w", err)
	}
	if err := atomicWriteFile(path, data, 0600); err != nil {
		if cpctx.MarkFSDegraded != nil {
			cpctx.MarkFSDegraded(fmt.Sprintf("writeYAMLAtomic failed: %v", err))
		}
		return err
	}
	if cpctx.ClearFSDegraded != nil {
		cpctx.ClearFSDegraded()
	}
	return nil
}

// ===== utils =====

func uniqueStrings(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}

// ----- validations (local) -----
func validateIssuerMode(m cp.IssuerMode) error {
	switch m {
	case "", cp.IssuerModeGlobal, cp.IssuerModePath, cp.IssuerModeDomain:
		return nil
	default:
		return fmt.Errorf("invalid issuerMode: %q", m)
	}
}
