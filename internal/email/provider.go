package email

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/dropDatabas3/hellojohn/internal/jwt"
	"github.com/google/uuid"
)

// SenderProvider resolves an email Sender for a given tenant context.
type SenderProvider interface {
	GetSender(ctx context.Context, tenantID uuid.UUID) (Sender, error)
}

// TenantSenderProvider implements SenderProvider using the ControlPlane to fetch tenant settings.
type TenantSenderProvider struct {
	CP        controlplane.ControlPlane
	MasterKey string // Hex encoded master key for decrypting secrets
}

func NewTenantSenderProvider(cp controlplane.ControlPlane, masterKey string) *TenantSenderProvider {
	return &TenantSenderProvider{
		CP:        cp,
		MasterKey: masterKey,
	}
}

func (p *TenantSenderProvider) GetSender(ctx context.Context, tenantID uuid.UUID) (Sender, error) {
	// 1. Fetch tenant by ID
	var tenant *controlplane.Tenant
	var err error

	if fs, ok := controlplane.AsFSProvider(p.CP); ok {
		tenant, err = fs.GetTenantByID(ctx, tenantID.String())
	} else {
		return nil, fmt.Errorf("provider does not support GetTenantByID")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get tenant %s: %w", tenantID, err)
	}

	// 2. Check for SMTP settings
	settings := tenant.Settings
	if settings.SMTP == nil {
		return nil, fmt.Errorf("no SMTP settings for tenant %s", tenant.Slug)
	}

	// 3. Decrypt password if present
	smtpPass := settings.SMTP.Password
	if settings.SMTP.PasswordEnc != "" && p.MasterKey != "" {
		encryptedBytes, err := base64.RawURLEncoding.DecodeString(settings.SMTP.PasswordEnc)
		if err != nil {
			log.Printf("failed to decode smtp password for tenant %s: %v", tenant.Slug, err)
		} else {
			decrypted, err := jwt.DecryptPrivateKey(encryptedBytes, p.MasterKey)
			if err != nil {
				log.Printf("failed to decrypt smtp password for tenant %s: %v", tenant.Slug, err)
			} else {
				smtpPass = string(decrypted)
			}
		}
	}

	// 4. Build Sender
	sender := NewSMTPSender(
		settings.SMTP.Host,
		settings.SMTP.Port,
		settings.SMTP.FromEmail,
		settings.SMTP.Username,
		smtpPass,
	)

	// Set TLS mode if specified or default
	if settings.SMTP.UseTLS {
		sender.TLSMode = "ssl"
	} else {
		sender.TLSMode = "auto"
	}

	return sender, nil
}
