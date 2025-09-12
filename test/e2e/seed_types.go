package e2e

// Definiciones del seed separadas de helpers.go para orden
type seedData struct {
	Tenant struct {
		ID   string `yaml:"id"`
		Slug string `yaml:"slug"`
	} `yaml:"tenant"`

	Clients struct {
		Web struct {
			Name      string   `yaml:"name"`
			ClientID  string   `yaml:"client_id"`
			Type      string   `yaml:"type"`
			Origins   []string `yaml:"origins"`
			Redirects []string `yaml:"redirects"`
		} `yaml:"web"`
		Backend *struct {
			Name     string `yaml:"name"`
			ClientID string `yaml:"client_id"`
			Type     string `yaml:"type"`
		} `yaml:"backend,omitempty"`
		// Compatibilidad con seeds antiguos:
		API *struct {
			Name     string `yaml:"name"`
			ClientID string `yaml:"client_id"`
			Type     string `yaml:"type"`
		} `yaml:"api,omitempty"`
	} `yaml:"clients"`

	Users struct {
		Admin struct {
			ID       string `yaml:"id"`
			Email    string `yaml:"email"`
			Password string `yaml:"password"`
		} `yaml:"admin"`
		MFA struct {
			ID                 string   `yaml:"id"`
			Email              string   `yaml:"email"`
			Password           string   `yaml:"password"`
			TOTPBase32         string   `yaml:"totp_secret_base32"`
			OTPAuthURL         string   `yaml:"otpauth_url"`
			Recovery           []string `yaml:"recovery_codes"`
			TrustedDeviceToken string   `yaml:"trusted_device_token"`
		} `yaml:"mfa"`
		Unverified struct {
			ID        string `yaml:"id"`
			Email     string `yaml:"email"`
			Password  string `yaml:"password"`
			VerifyURL string `yaml:"verify_url"`
		} `yaml:"unverified"`
	} `yaml:"users"`

	EmailFlows struct {
		Verify string `yaml:"verify_url"`
		Reset  string `yaml:"reset_url"`
	} `yaml:"email_flows"`
}
