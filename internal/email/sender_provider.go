package emailv2

import (
	"context"
	"fmt"

	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/observability/logger"
	sec "github.com/dropDatabas3/hellojohn/internal/security/secretbox"
	store "github.com/dropDatabas3/hellojohn/internal/store"
	"github.com/google/uuid"
)

// senderProvider implementa SenderProvider usando Store V2 DAL.
type senderProvider struct {
	dal       store.DataAccessLayer
	masterKey string // Hex encoded master key para descifrar secrets
}

// NewSenderProvider crea un SenderProvider que usa Store V2 DAL.
//
// masterKey es la clave maestra para descifrar passwords SMTP encriptadas.
// Debe ser hex-encoded de al menos 32 bytes.
func NewSenderProvider(dal store.DataAccessLayer, masterKey string) SenderProvider {
	return &senderProvider{
		dal:       dal,
		masterKey: masterKey,
	}
}

// GetSender obtiene un Sender configurado para el tenant especificado.
// tenantSlugOrID puede ser UUID o slug del tenant.
func (p *senderProvider) GetSender(ctx context.Context, tenantSlugOrID string) (Sender, error) {
	log := logger.From(ctx).With(
		logger.String("component", "SenderProvider"),
		logger.String("tenant", tenantSlugOrID),
	)

	// 1. Resolver tenant (por UUID o slug)
	tenant, err := p.resolveTenant(ctx, tenantSlugOrID)
	if err != nil {
		log.Error("failed to resolve tenant", logger.Err(err))
		return nil, fmt.Errorf("resolve tenant: %w", err)
	}

	// 2. Verificar settings SMTP
	if tenant.Settings.SMTP == nil {
		log.Warn("no SMTP settings for tenant")
		return nil, fmt.Errorf("no SMTP settings for tenant %s", tenant.Slug)
	}

	smtp := tenant.Settings.SMTP

	// 3. Descifrar password si está encriptada
	password := smtp.Password
	if smtp.PasswordEnc != "" && password == "" {
		decrypted, err := p.decryptPassword(smtp.PasswordEnc)
		if err != nil {
			log.Warn("failed to decrypt SMTP password", logger.Err(err))
			// No retornar error - intentar con password vacío
		} else {
			password = decrypted
		}
	}

	// 4. Construir Sender
	sender := NewSMTPSender(
		smtp.Host,
		smtp.Port,
		smtp.FromEmail,
		smtp.Username,
		password,
	)

	// Configurar TLS
	if smtp.UseTLS {
		sender.TLSMode = "ssl"
	} else {
		sender.TLSMode = "auto"
	}

	log.Debug("sender created",
		logger.String("host", smtp.Host),
		logger.Int("port", smtp.Port),
	)

	return sender, nil
}

// resolveTenant intenta resolver tenant por UUID primero, luego por slug.
func (p *senderProvider) resolveTenant(ctx context.Context, tenantSlugOrID string) (*repository.Tenant, error) {
	tenants := p.dal.ConfigAccess().Tenants()

	// Intentar parsear como UUID
	if id, err := uuid.Parse(tenantSlugOrID); err == nil {
		tenant, err := tenants.GetByID(ctx, id.String())
		if err == nil {
			return tenant, nil
		}
		// Si falla por UUID, intentar por slug como fallback
	}

	// Intentar por slug
	tenant, err := tenants.GetBySlug(ctx, tenantSlugOrID)
	if err != nil {
		return nil, fmt.Errorf("tenant not found: %s", tenantSlugOrID)
	}

	return tenant, nil
}

// decryptPassword descifra una password encriptada con secretbox.
// El formato esperado es base64(nonce)|base64(ciphertext).
func (p *senderProvider) decryptPassword(encrypted string) (string, error) {
	// Usar la masterKey inyectada explícitamente (evita depender de env vars ocultas)
	decrypted, err := sec.DecryptWithKey(p.masterKey, encrypted)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	return decrypted, nil
}
