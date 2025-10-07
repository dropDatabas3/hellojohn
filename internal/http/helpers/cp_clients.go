package helpers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/dropDatabas3/hellojohn/internal/app/cpctx"
	cp "github.com/dropDatabas3/hellojohn/internal/controlplane"
)

var (
	ErrClientNotFound   = errors.New("client not found")
	ErrRedirectMismatch = errors.New("redirect_uri mismatch")
	ErrUnauthorized     = errors.New("unauthorized client credentials")
)

func ResolveTenantSlug(r *http.Request) string {
	return cpctx.ResolveTenant(r)
}

func LookupClient(ctx context.Context, r *http.Request, clientID string) (*cp.OIDCClient, string, error) {
	slug := ResolveTenantSlug(r)
	if cpctx.Provider == nil {
		return nil, "", fmt.Errorf("control-plane provider not initialized")
	}
	c, err := cpctx.Provider.GetClient(ctx, slug, clientID)
	if err != nil {
		return nil, "", ErrClientNotFound
	}
	return c, slug, nil
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
