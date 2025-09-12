package handlers

// mfaChallenge representa los datos almacenados en el cache para un challenge MFA pendiente.
// Es usado por todos los handlers que manejan flujos de MFA (login, challenge, social auth).
type mfaChallenge struct {
	UserID   string   `json:"uid"`
	TenantID string   `json:"tid"`
	ClientID string   `json:"cid"`
	AMRBase  []string `json:"amr"`
	Scope    []string `json:"scp"`
}
