package admin

import (
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/security/secretbox"
)

// EncryptTenantSecrets encrypts sensitive fields in the settings using secretbox.
// It modifies the settings in place.
// It clears the plain text password fields after encryption.
// Note: masterKeyHex parameter is kept for signature compatibility but not used - secretbox uses global key.
func encryptTenantSecrets(s *repository.TenantSettings, masterKeyHex string) error {
	if s == nil {
		return nil
	}

	// SMTP
	if s.SMTP != nil && s.SMTP.Password != "" {
		enc, err := secretbox.Encrypt(s.SMTP.Password)
		if err != nil {
			return err
		}
		s.SMTP.PasswordEnc = enc
		s.SMTP.Password = "" // Clear plain
	}

	// UserDB
	if s.UserDB != nil && s.UserDB.DSN != "" {
		enc, err := secretbox.Encrypt(s.UserDB.DSN)
		if err != nil {
			return err
		}
		s.UserDB.DSNEnc = enc
		s.UserDB.DSN = ""
	}

	// Cache
	if s.Cache != nil && s.Cache.Password != "" {
		enc, err := secretbox.Encrypt(s.Cache.Password)
		if err != nil {
			return err
		}
		s.Cache.PassEnc = enc
		s.Cache.Password = ""
	}

	// Social Providers
	if s.SocialProviders != nil && s.SocialProviders.GoogleSecret != "" {
		enc, err := secretbox.Encrypt(s.SocialProviders.GoogleSecret)
		if err != nil {
			return err
		}
		s.SocialProviders.GoogleSecretEnc = enc
		s.SocialProviders.GoogleSecret = ""
	}

	return nil
}
