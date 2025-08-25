package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

type Params struct {
	Memory      uint32 // KiB
	Time        uint32
	Parallelism uint8
	KeyLen      uint32
}

var Default = Params{Memory: 64 * 1024, Time: 3, Parallelism: 1, KeyLen: 32}

// Hash devuelve un PHC string: $argon2id$v=19$m=...,t=...,p=...$<saltB64>$<dkB64>
func Hash(p Params, plain string) (string, error) {
	if plain == "" {
		return "", fmt.Errorf("empty password")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	dk := argon2.IDKey([]byte(plain), salt, p.Time, p.Memory, p.Parallelism, p.KeyLen)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		p.Memory, p.Time, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(dk),
	), nil
}

func Verify(plain, phc string) bool {
	var v int
	var m, t, p int
	var saltB64, dkB64 string
	n, _ := fmt.Sscanf(phc, "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", &v, &m, &t, &p, &saltB64, &dkB64)
	if n != 6 || v != 19 {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(saltB64)
	if err != nil {
		return false
	}
	dkStored, err := base64.RawStdEncoding.DecodeString(dkB64)
	if err != nil {
		return false
	}
	key := argon2.IDKey([]byte(plain), salt, uint32(t), uint32(m), uint8(p), uint32(len(dkStored)))
	return subtle.ConstantTimeCompare(key, dkStored) == 1
}
