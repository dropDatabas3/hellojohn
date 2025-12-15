package helpers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/v1/cpctx"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
)

var (
	ErrClientNotFound   = errors.New("client not found")
	ErrRedirectMismatch = errors.New("redirect_uri mismatch")
	ErrUnauthorized     = errors.New("unauthorized client credentials")
)

// FSClient is a lightweight runtime view of a client resolved from the FS control-plane.
// It includes the tenant slug/UUID and selected client fields needed by auth flows.
type FSClient struct {
	TenantSlug     string
	TenantUUID     string
	ClientID       string
	ClientType     cp.ClientType // "public" | "confidential"
	RedirectURIs   []string
	AllowedOrigins []string
	Providers      []string
	Scopes         []string
	SecretEnc      string // present only for confidential clients
	// Social providers config is available in the CP model; add if needed later.
}

func (c FSClient) IsConfidential() bool { return c.ClientType == cp.ClientTypeConfidential }

func ResolveTenantSlug(r *http.Request) string {
	return cpctx.ResolveTenant(r)
}

// ResolveClientFSBySlug loads a client from the FS control-plane for the given tenant slug and clientID.
// It returns a normalized FSClient shape for runtime usage. Secret is not decrypted here.
func ResolveClientFSBySlug(ctx context.Context, tenantSlug, clientID string) (FSClient, error) {
	if cpctx.Provider == nil {
		return FSClient{}, fmt.Errorf("control-plane provider not initialized")
	}
	// Normalize identifier (accept slug or UUID)
	slug := tenantSlug
	// Try as-is
	ten, err := cpctx.Provider.GetTenantBySlug(ctx, slug)
	if err != nil {
		// Fallback: search by ID and map to slug
		if tenants, lerr := cpctx.Provider.ListTenants(ctx); lerr == nil {
			for _, t := range tenants {
				if t.ID == tenantSlug {
					slug = t.Slug
					ten = &t
					err = nil
					break
				}
			}
		}
		if err != nil || ten == nil {
			return FSClient{}, err
		}
	}
	// 2) Get client
	cl, err := cpctx.Provider.GetClient(ctx, slug, clientID)
	if err != nil {
		return FSClient{}, ErrClientNotFound
	}
	// 3) Map to FSClient
	out := FSClient{
		TenantSlug:     slug,
		TenantUUID:     ten.ID,
		ClientID:       cl.ClientID,
		ClientType:     cl.Type,
		RedirectURIs:   append([]string{}, cl.RedirectURIs...),
		AllowedOrigins: append([]string{}, cl.AllowedOrigins...),
		Providers:      append([]string{}, cl.Providers...),
		Scopes:         append([]string{}, cl.Scopes...),
		SecretEnc:      cl.SecretEnc,
	}
	return out, nil
}

// ResolveClientFSByTenantID loads a client from the FS control-plane using tenant UUID instead of slug.
// It iterates through tenants to find the matching UUID and then loads the client.
func ResolveClientFSByTenantID(ctx context.Context, tenantID, clientID string) (FSClient, error) {
	if cpctx.Provider == nil {
		return FSClient{}, fmt.Errorf("control-plane provider not initialized")
	}

	// Find tenant by UUID
	tenants, err := cpctx.Provider.ListTenants(ctx)
	if err != nil {
		return FSClient{}, err
	}

	var tenant *cp.Tenant
	for _, t := range tenants {
		if t.ID == tenantID {
			tenant = &t
			break
		}
	}

	if tenant == nil {
		return FSClient{}, fmt.Errorf("tenant not found for ID: %s", tenantID)
	}

	// Load client by slug
	cl, err := cpctx.Provider.GetClient(ctx, tenant.Slug, clientID)
	if err != nil {
		return FSClient{}, ErrClientNotFound
	}

	return FSClient{
		TenantSlug:     tenant.Slug,
		TenantUUID:     tenant.ID,
		ClientID:       cl.ClientID,
		ClientType:     cl.Type,
		RedirectURIs:   append([]string{}, cl.RedirectURIs...),
		AllowedOrigins: append([]string{}, cl.AllowedOrigins...),
		Providers:      append([]string{}, cl.Providers...),
		Scopes:         append([]string{}, cl.Scopes...),
		SecretEnc:      cl.SecretEnc,
	}, nil
}

// ResolveClientFS is a convenience wrapper that infers the tenant slug from the request.
func ResolveClientFS(ctx context.Context, r *http.Request, clientID string) (FSClient, error) {
	slug := ResolveTenantSlug(r)
	return ResolveClientFSBySlug(ctx, slug, clientID)
}

func LookupClient(ctx context.Context, r *http.Request, clientID string) (*cp.OIDCClient, string, error) {
	slug := ResolveTenantSlug(r)
	if cpctx.Provider == nil {
		return nil, "", fmt.Errorf("control-plane provider not initialized")
	}
	// Try the resolved tenant first
	c, err := cpctx.Provider.GetClient(ctx, slug, clientID)
	if err == nil {
		return c, slug, nil
	}
	// If not found and using default "local", search across all tenants
	// This supports OAuth flows where tenant is not explicitly provided
	tenants, terr := cpctx.Provider.ListTenants(ctx)
	if terr != nil {
		return nil, "", ErrClientNotFound
	}
	for _, t := range tenants {
		if t.Slug == slug {
			continue // already tried
		}
		if c, err := cpctx.Provider.GetClient(ctx, t.Slug, clientID); err == nil {
			return c, t.Slug, nil
		}
	}
	return nil, "", ErrClientNotFound
}

func ValidateRedirectURI(c *cp.OIDCClient, redirect string) error {
	if strings.TrimSpace(redirect) == "" {
		return fmt.Errorf("missing redirect_uri")
	}
	ok := false
	for _, u := range c.RedirectURIs {
		if u == redirect {
			ok = true
			break
		}
	}
	if !ok {
		return ErrRedirectMismatch
	}
	// Reglas base (https or localhost)
	if !cp.DefaultValidateRedirectURI(redirect) {
		return ErrRedirectMismatch
	}
	return nil
}

func ValidateClientSecret(ctx context.Context, r *http.Request, slug string, c *cp.OIDCClient, providedSecret string) error {
	if c.Type != cp.ClientTypeConfidential {
		return nil // public no requieren secreto
	}
	dec, err := cpctx.Provider.DecryptClientSecret(ctx, slug, c.ClientID)
	if err != nil {
		return fmt.Errorf("decrypt client secret: %w", err)
	}
	if strings.TrimSpace(dec) == "" {
		return ErrUnauthorized
	}
	if subtleEq(dec, providedSecret) {
		return nil
	}
	return ErrUnauthorized
}

func subtleEq(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}
