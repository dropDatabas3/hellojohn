package core

import "time"

type KeyStatus string

const (
	KeyActive   KeyStatus = "active"
	KeyRetiring KeyStatus = "retiring"
	KeyRetired  KeyStatus = "retired"
)

type SigningKey struct {
	KID        string
	Alg        string // "EdDSA"
	PublicKey  []byte
	PrivateKey []byte // Puede ser nil en prod/KMS
	Status     KeyStatus
	NotBefore  time.Time
	CreatedAt  time.Time
	RotatedAt  *time.Time
}
