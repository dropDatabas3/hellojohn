package admin

import (
	"github.com/dropDatabas3/hellojohn/internal/domain/repository"
	"github.com/dropDatabas3/hellojohn/internal/security/keycrypto"
)

// EncryptTenantSecrets encrypts sensitive fields in the settings using the provided master key.
// It modifies the settings in place.
// It clears the plain text password fields after encryption.
func encryptTenantSecrets(s *repository.TenantSettings, masterKeyHex string) error {
	if s == nil {
		return nil
	}

	// SMTP
	if s.SMTP != nil && s.SMTP.Password != "" {
		enc, err := keycrypto.EncryptPrivateKey([]byte(s.SMTP.Password), masterKeyHex)
		if err != nil {
			return err
		}
		s.SMTP.PasswordEnc = string(enc)
		s.SMTP.Password = "" // Clear plain
	}

	// UserDB
	if s.UserDB != nil && s.UserDB.DSN != "" {
		enc, err := keycrypto.EncryptPrivateKey([]byte(s.UserDB.DSN), masterKeyHex)
		if err != nil {
			return err
		}
		s.UserDB.DSNEnc = string(enc)
		s.UserDB.DSN = ""
	}

	// Cache
	if s.Cache != nil && s.Cache.Password != "" {
		enc, err := keycrypto.EncryptPrivateKey([]byte(s.Cache.Password), masterKeyHex)
		if err != nil {
			return err
		}
		s.Cache.PassEnc = string(enc)
		s.Cache.Password = ""
	}

	return nil
}
