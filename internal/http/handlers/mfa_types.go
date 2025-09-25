package handlers

import "time"

// mfaChallenge representa los datos almacenados en el cache para un challenge MFA pendiente.
// Es usado por todos los handlers que manejan flujos de MFA (login, challenge, social auth).
type mfaChallenge struct {
	UserID   string   `json:"uid"`
	TenantID string   `json:"tid"`
	ClientID string   `json:"cid"`
	AMRBase  []string `json:"amr"`
	Scope    []string `json:"scp"`
}

// consentChallenge se guarda en cache cuando faltan consentimientos para scopes pedidos.
// Es análogo al patrón mfa_required + token (one-shot + TTL corto).
type consentChallenge struct {
	UserID              string    `json:"uid"`
	TenantID            string    `json:"tid"`
	ClientID            string    `json:"cid"`
	RedirectURI         string    `json:"ru"`
	State               string    `json:"st"`
	Nonce               string    `json:"n"`
	CodeChallenge       string    `json:"cc"`
	CodeChallengeMethod string    `json:"ccm"`
	RequestedScopes     []string  `json:"scp"`
	AMR                 []string  `json:"amr"`
	ExpiresAt           time.Time `json:"exp"`
}
