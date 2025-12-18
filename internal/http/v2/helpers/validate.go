package helpers

import (
	"regexp"
	"strings"

	//controlplane "github.com/dropDatabas3/hellojohn/internal/controlplane/v2"
	"github.com/dropDatabas3/hellojohn/internal/validation"
)

var tenantSlugRe = regexp.MustCompile(`^[a-z0-9\-]{1,64}$`)

// ValidTenantSlug valida un slug de tenant con el patrón usado en v1/v2.
func ValidTenantSlug(slug string) bool {
	s := strings.TrimSpace(slug)
	return tenantSlugRe.MatchString(s)
}

/*
Deprecated: Revisar V2 control-plane para esta lógica.
// ValidRedirectURI aplica la regla estándar del control-plane:
// https obligatorio salvo localhost/127.0.0.1.
func ValidRedirectURI(uri string) bool {
	return controlplane.DefaultValidateRedirectURI(uri)
}
*/

// ValidScopeName reusa la regex permisiva del package validation.
func ValidScopeName(scope string) bool {
	return validation.ValidScopeName(strings.TrimSpace(scope))
}

func ValidScopes(scopes []string) bool {
	for _, s := range scopes {
		if !ValidScopeName(s) {
			return false
		}
	}
	return true
}
