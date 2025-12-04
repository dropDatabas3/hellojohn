package jwt

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/store/core"
)

// FileSigningKeyStore implementa signingKeyStore usando archivos en disco.
// Garantías:
// - Escritura atómica: write tmp → fsync → rename
// - Encriptación de claves privadas con SIGNING_MASTER_KEY
// - Archivos active.json y retiring.json
// - Rotación sin cambios de kid en restart
type FileSigningKeyStore struct {
	keysDir string
	mu      sync.RWMutex

	// Cache en memoria para evitar lecturas frecuentes
	activeKey   *core.SigningKey
	retiringKey *core.SigningKey
	lastCheck   time.Time
	checkTTL    time.Duration
}

// keyFileData representa la estructura del archivo JSON
type keyFileData struct {
	KID           string    `json:"kid"`
	Algorithm     string    `json:"algorithm"`
	PrivateKeyEnc string    `json:"private_key_enc"` // Encrypted with SIGNING_MASTER_KEY
	PublicKeyPEM  string    `json:"public_key_pem"`  // PEM encoded public key
	Status        string    `json:"status"`          // "active" or "retiring"
	NotBefore     time.Time `json:"not_before"`
	CreatedAt     time.Time `json:"created_at"`
	// Rotation metadata (only present for retiring.json)
	RetiredAtUnix int64 `json:"retired_at,omitempty"`
	GraceSeconds  int64 `json:"grace_seconds,omitempty"`
}

// NewFileSigningKeyStore crea un nuevo keystore basado en archivos
func NewFileSigningKeyStore(keysDir string) (*FileSigningKeyStore, error) {
	if err := os.MkdirAll(keysDir, 0755); err != nil {
		return nil, fmt.Errorf("create keys directory: %w", err)
	}

	return &FileSigningKeyStore{
		keysDir:  keysDir,
		checkTTL: 30 * time.Second, // Cache keys for 30 seconds
	}, nil
}

// dirForTenant returns the directory to read/write keys for a tenant.
// "global" (or empty) maps to the base keysDir for backward compatibility.
func (s *FileSigningKeyStore) dirForTenant(tenant string) string {
	base := filepath.Clean(s.keysDir)
	if tenant == "" || tenant == "global" {
		return base
	}
	return filepath.Join(base, tenant)
}

// loadKeyFromDirFile loads a key JSON from a specific directory (active.json or retiring.json).
func (s *FileSigningKeyStore) loadKeyFromDirFile(dir, filename string) (*core.SigningKey, error) {
	path := filepath.Join(dir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var keyData keyFileData
	if err := json.Unmarshal(data, &keyData); err != nil {
		return nil, fmt.Errorf("unmarshal key data: %w", err)
	}

	// Decrypt private if present (mirrors loadKeyFromFile)
	var privateKey []byte
	if keyData.PrivateKeyEnc != "" {
		masterKey := os.Getenv("SIGNING_MASTER_KEY")
		if masterKey == "" {
			return nil, fmt.Errorf("SIGNING_MASTER_KEY not set for encrypted key")
		}
		encrypted, err := base64.StdEncoding.DecodeString(keyData.PrivateKeyEnc)
		if err != nil {
			return nil, fmt.Errorf("decode base64 private key: %w", err)
		}
		decrypted, err := DecryptPrivateKey(encrypted, masterKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt private key: %w", err)
		}
		privateKey = decrypted
	}

	// Parse public PEM
	block, _ := pem.Decode([]byte(keyData.PublicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid public key PEM")
	}

	publicKey := block.Bytes
	status := core.KeyActive
	if filename == "retiring.json" {
		status = core.KeyRetiring
	}
	return &core.SigningKey{
		KID:        keyData.KID,
		Alg:        keyData.Algorithm,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Status:     status,
		NotBefore:  keyData.NotBefore,
		CreatedAt:  keyData.CreatedAt,
	}, nil
}

// GetActiveSigningKeyForTenant returns the tenant's active key; falls back to global if not found.
func (s *FileSigningKeyStore) GetActiveSigningKeyForTenant(ctx context.Context, tenant string) (*core.SigningKey, error) {
	dir := s.dirForTenant(tenant)
	if k, err := s.loadKeyFromDirFile(dir, "active.json"); err == nil {
		return k, nil
	} else if errors.Is(err, fs.ErrNotExist) {
		// Bootstrap: create tenant-specific key if missing
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create tenant keys dir: %w", err)
		}
		newKey, gerr := s.generateNewKey()
		if gerr != nil {
			return nil, fmt.Errorf("generate tenant key: %w", gerr)
		}
		if serr := s.saveKeyToDirFile(dir, "active.json", newKey); serr != nil {
			return nil, fmt.Errorf("save tenant active key: %w", serr)
		}
		// Return a copy to callers
		keyCopy := *newKey
		return &keyCopy, nil
	}
	// Fallback to global for other errors
	return s.loadKeyFromDirFile(s.dirForTenant("global"), "active.json")
}

// ListPublicSigningKeysForTenant lists public (active + retiring) keys for a tenant, with fallback to global if tenant dir missing.
func (s *FileSigningKeyStore) ListPublicSigningKeysForTenant(ctx context.Context, tenant string) ([]core.SigningKey, error) {
	dir := s.dirForTenant(tenant)
	var keys []core.SigningKey

	if k, err := s.loadKeyFromDirFile(dir, "active.json"); err == nil {
		pub := *k
		pub.PrivateKey = nil
		keys = append(keys, pub)
		// Handle retiring.json only if within grace window (if metadata is present)
		rpath := filepath.Join(dir, "retiring.json")
		if b, rerr := os.ReadFile(rpath); rerr == nil {
			var meta keyFileData
			if jerr := json.Unmarshal(b, &meta); jerr == nil {
				include := true
				if meta.GraceSeconds > 0 && meta.RetiredAtUnix > 0 {
					expireAt := time.Unix(meta.RetiredAtUnix, 0).Add(time.Duration(meta.GraceSeconds) * time.Second)
					if time.Now().After(expireAt) {
						include = false
						// Grace expired: clean up retiring.json
						_ = os.Remove(rpath)
					}
				}
				if include {
					if r, err := s.loadKeyFromDirFile(dir, "retiring.json"); err == nil {
						pubR := *r
						pubR.PrivateKey = nil
						keys = append(keys, pubR)
					}
				}
			}
		}
		return keys, nil
	} else if errors.Is(err, fs.ErrNotExist) {
		// Bootstrap: create tenant-specific key if missing (Lazy Generation)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create tenant keys dir: %w", err)
		}
		newKey, gerr := s.generateNewKey()
		if gerr != nil {
			return nil, fmt.Errorf("generate tenant key: %w", gerr)
		}
		if serr := s.saveKeyToDirFile(dir, "active.json", newKey); serr != nil {
			return nil, fmt.Errorf("save tenant active key: %w", serr)
		}
		// Return the new key as the only public key
		pub := *newKey
		pub.PrivateKey = nil
		keys = append(keys, pub)
		return keys, nil
	}
	// Fallback to global
	if k, err := s.loadKeyFromDirFile(s.dirForTenant("global"), "active.json"); err == nil {
		pub := *k
		pub.PrivateKey = nil
		keys = append(keys, pub)
		if r, err := s.loadKeyFromDirFile(s.dirForTenant("global"), "retiring.json"); err == nil {
			pubR := *r
			pubR.PrivateKey = nil
			keys = append(keys, pubR)
		}
		return keys, nil
	} else {
		return nil, err
	}
}

// ListAllTenantPublicSigningKeys recorre los subdirectorios de keysDir y retorna las claves públicas (active/retiring)
// de todos los tenants (excluye el directorio base/global).
func (s *FileSigningKeyStore) ListAllTenantPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	base := filepath.Clean(s.keysDir)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}
	var out []core.SigningKey
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// omit the base/global directory entries that are not tenant folders
		// tenant folders are subdirectories under base (any name allowed; tests use slug)
		dir := filepath.Join(base, name)
		// Try to read active/retiring in this dir
		if k, err := s.loadKeyFromDirFile(dir, "active.json"); err == nil {
			pub := *k
			pub.PrivateKey = nil
			out = append(out, pub)
		}
		if r, err := s.loadKeyFromDirFile(dir, "retiring.json"); err == nil {
			pubR := *r
			pubR.PrivateKey = nil
			out = append(out, pubR)
		}
	}
	return out, nil
}

// GetActiveSigningKey implementa la interfaz signingKeyStore
func (s *FileSigningKeyStore) GetActiveSigningKey(ctx context.Context) (*core.SigningKey, error) {
	s.mu.RLock()

	// Cache hit si aún es válido
	if s.activeKey != nil && time.Since(s.lastCheck) < s.checkTTL {
		key := *s.activeKey // copy
		s.mu.RUnlock()
		return &key, nil
	}
	s.mu.RUnlock()

	// Cache miss o expirado - necesitamos leer del disco
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check después del lock
	if s.activeKey != nil && time.Since(s.lastCheck) < s.checkTTL {
		key := *s.activeKey // copy
		return &key, nil
	}

	// Leer active.json
	activeKey, err := s.loadKeyFromFile("active.json")
	if errors.Is(err, fs.ErrNotExist) {
		// No hay clave activa - crear una nueva
		newKey, err := s.generateNewKey()
		if err != nil {
			return nil, fmt.Errorf("generate new key: %w", err)
		}

		if err := s.saveKeyToFile("active.json", newKey); err != nil {
			return nil, fmt.Errorf("save new active key: %w", err)
		}

		s.activeKey = newKey
		s.lastCheck = time.Now()
		key := *newKey // copy
		return &key, nil
	}

	if err != nil {
		return nil, fmt.Errorf("load active key: %w", err)
	}

	s.activeKey = activeKey
	s.lastCheck = time.Now()
	key := *activeKey // copy
	return &key, nil
}

// ListPublicSigningKeys implementa la interfaz signingKeyStore
func (s *FileSigningKeyStore) ListPublicSigningKeys(ctx context.Context) ([]core.SigningKey, error) {
	var keys []core.SigningKey

	// Clave activa
	activeKey, err := s.GetActiveSigningKey(ctx)
	if err != nil {
		return nil, fmt.Errorf("get active key: %w", err)
	}

	// Crear copia sin private key para exposición pública
	publicActive := *activeKey
	publicActive.PrivateKey = nil
	keys = append(keys, publicActive)

	// Clave retiring (si existe)
	retiringKey, err := s.loadKeyFromFile("retiring.json")
	if err == nil {
		publicRetiring := *retiringKey
		publicRetiring.PrivateKey = nil
		keys = append(keys, publicRetiring)
	}

	return keys, nil
}

// InsertSigningKey implementa la interfaz signingKeyStore
func (s *FileSigningKeyStore) InsertSigningKey(ctx context.Context, k *core.SigningKey) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Determinar el archivo basado en el status
	filename := "active.json"
	if k.Status == core.KeyRetiring {
		filename = "retiring.json"
	}

	if err := s.saveKeyToFile(filename, k); err != nil {
		return fmt.Errorf("save key to file %s: %w", filename, err)
	}

	// Actualizar cache
	if k.Status == core.KeyActive {
		s.activeKey = k
		s.lastCheck = time.Now()
	} else if k.Status == core.KeyRetiring {
		s.retiringKey = k
	}

	return nil
}

// generateNewKey crea una nueva clave Ed25519
func (s *FileSigningKeyStore) generateNewKey() (*core.SigningKey, error) {
	pub, priv, err := GenerateEd25519()
	if err != nil {
		return nil, fmt.Errorf("generate ed25519: %w", err)
	}

	now := time.Now().UTC()
	return &core.SigningKey{
		KID:        "fs-" + now.Format("20060102T150405Z"),
		Alg:        "EdDSA",
		PublicKey:  pub,
		PrivateKey: priv,
		Status:     core.KeyActive,
		NotBefore:  now,
		CreatedAt:  now,
	}, nil
}

// loadKeyFromFile carga una clave desde un archivo
func (s *FileSigningKeyStore) loadKeyFromFile(filename string) (*core.SigningKey, error) {
	path := filepath.Join(s.keysDir, filename)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var keyData keyFileData
	if err := json.Unmarshal(data, &keyData); err != nil {
		return nil, fmt.Errorf("unmarshal key data: %w", err)
	}

	// Desencriptar clave privada si está encriptada
	var privateKey []byte
	if keyData.PrivateKeyEnc != "" {
		masterKey := os.Getenv("SIGNING_MASTER_KEY")
		if masterKey == "" {
			return nil, fmt.Errorf("SIGNING_MASTER_KEY not set for encrypted key")
		}

		// Decodificar desde base64
		encrypted, err := base64.StdEncoding.DecodeString(keyData.PrivateKeyEnc)
		if err != nil {
			return nil, fmt.Errorf("decode base64 private key: %w", err)
		}

		decrypted, err := DecryptPrivateKey(encrypted, masterKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt private key: %w", err)
		}
		privateKey = decrypted
	}

	// Parsear public key desde PEM
	block, _ := pem.Decode([]byte(keyData.PublicKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid public key PEM")
	}

	publicKey := block.Bytes

	// Determinar status del filename
	status := core.KeyActive
	if filename == "retiring.json" {
		status = core.KeyRetiring
	}

	return &core.SigningKey{
		KID:        keyData.KID,
		Alg:        keyData.Algorithm,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		Status:     status,
		NotBefore:  keyData.NotBefore,
		CreatedAt:  keyData.CreatedAt,
	}, nil
}

// saveKeyToFile guarda una clave en un archivo con escritura atómica
func (s *FileSigningKeyStore) saveKeyToFile(filename string, key *core.SigningKey) error {
	return s.saveKeyToDirFile(s.keysDir, filename, key)
}

// saveKeyToDirFile guarda una clave en un archivo dentro de un directorio específico con escritura atómica
func (s *FileSigningKeyStore) saveKeyToDirFile(dir, filename string, key *core.SigningKey) error {
	// Encriptar clave privada si hay master key
	var privateKeyEnc string
	if len(key.PrivateKey) > 0 {
		masterKey := os.Getenv("SIGNING_MASTER_KEY")
		if masterKey != "" {
			encrypted, err := EncryptPrivateKey(key.PrivateKey, masterKey)
			if err != nil {
				return fmt.Errorf("encrypt private key: %w", err)
			}
			// Codificar en base64 para almacenamiento seguro
			privateKeyEnc = base64.StdEncoding.EncodeToString(encrypted)
		}
	}

	// Codificar public key como PEM
	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: key.PublicKey,
	})

	keyData := keyFileData{
		KID:           key.KID,
		Algorithm:     key.Alg,
		PrivateKeyEnc: privateKeyEnc,
		PublicKeyPEM:  string(publicKeyPEM),
		Status:        string(key.Status),
		NotBefore:     key.NotBefore,
		CreatedAt:     key.CreatedAt,
	}

	data, err := json.MarshalIndent(keyData, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal key data: %w", err)
	}

	// Escritura atómica: tmp → fsync → rename
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	finalPath := filepath.Join(dir, filename)
	tmpPath := finalPath + ".tmp"

	// Escribir a archivo temporal
	tmpFile, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create tmp file: %w", err)
	}

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write tmp file: %w", err)
	}

	// Sync para asegurar que esté en disco
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("sync tmp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close tmp file: %w", err)
	}

	// Rename atómico
	if err := os.Rename(tmpPath, finalPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
}

// saveRetiringToDirFile writes retiring.json with grace metadata (retired_at, grace_seconds)
func (s *FileSigningKeyStore) saveRetiringToDirFile(dir string, key *core.SigningKey, retiredAt time.Time, graceSeconds int64) error {
	// Encrypt private if present
	var privateKeyEnc string
	if len(key.PrivateKey) > 0 {
		if masterKey := os.Getenv("SIGNING_MASTER_KEY"); masterKey != "" {
			enc, err := EncryptPrivateKey(key.PrivateKey, masterKey)
			if err != nil {
				return fmt.Errorf("encrypt private key: %w", err)
			}
			privateKeyEnc = base64.StdEncoding.EncodeToString(enc)
		}
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: key.PublicKey})

	kd := keyFileData{
		KID:           key.KID,
		Algorithm:     key.Alg,
		PrivateKeyEnc: privateKeyEnc,
		PublicKeyPEM:  string(publicKeyPEM),
		Status:        string(core.KeyRetiring),
		NotBefore:     key.NotBefore,
		CreatedAt:     key.CreatedAt,
		RetiredAtUnix: retiredAt.Unix(),
		GraceSeconds:  graceSeconds,
	}

	data, err := json.MarshalIndent(kd, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal retiring data: %w", err)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	finalPath := filepath.Join(dir, "retiring.json")
	tmpPath := finalPath + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("create tmp: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := f.Sync(); err != nil {
		f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("sync tmp: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close tmp: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename retiring: %w", err)
	}
	return nil
}

// RotateFor moves tenant's active.json to retiring.json with grace seconds and creates a new active key.
// Returns the newly created active key.
func (s *FileSigningKeyStore) RotateFor(tenant string, graceSeconds int64) (*core.SigningKey, error) {
	if tenant == "" || tenant == "global" {
		return nil, fmt.Errorf("rotateFor: tenant required (not global)")
	}
	dir := s.dirForTenant(tenant)

	// If retiring exists and grace expired, clean it
	if b, err := os.ReadFile(filepath.Join(dir, "retiring.json")); err == nil {
		var meta keyFileData
		if json.Unmarshal(b, &meta) == nil && meta.GraceSeconds > 0 && meta.RetiredAtUnix > 0 {
			expireAt := time.Unix(meta.RetiredAtUnix, 0).Add(time.Duration(meta.GraceSeconds) * time.Second)
			if time.Now().After(expireAt) {
				_ = os.Remove(filepath.Join(dir, "retiring.json"))
			}
		}
	}

	// Read current active; if missing, bootstrap a new active and return
	cur, err := s.loadKeyFromDirFile(dir, "active.json")
	if errors.Is(err, fs.ErrNotExist) {
		// Bootstrap new active
		newKey, gerr := s.generateNewKey()
		if gerr != nil {
			return nil, fmt.Errorf("generate tenant key: %w", gerr)
		}
		if err := s.saveKeyToDirFile(dir, "active.json", newKey); err != nil {
			return nil, fmt.Errorf("save active: %w", err)
		}
		keyCopy := *newKey
		return &keyCopy, nil
	} else if err != nil {
		return nil, fmt.Errorf("read active: %w", err)
	}

	// Write retiring with grace metadata
	if err := s.saveRetiringToDirFile(dir, cur, time.Now().UTC(), graceSeconds); err != nil {
		return nil, fmt.Errorf("write retiring: %w", err)
	}

	// Create new active
	newKey, gerr := s.generateNewKey()
	if gerr != nil {
		return nil, fmt.Errorf("generate new active: %w", gerr)
	}
	if err := s.saveKeyToDirFile(dir, "active.json", newKey); err != nil {
		return nil, fmt.Errorf("save new active: %w", err)
	}
	keyCopy := *newKey
	return &keyCopy, nil
}
