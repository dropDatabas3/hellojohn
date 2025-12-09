package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/controlplane"
	"github.com/dropDatabas3/hellojohn/internal/email"
	httpx "github.com/dropDatabas3/hellojohn/internal/http"
	jwtx "github.com/dropDatabas3/hellojohn/internal/jwt"
)

type TestEmailReq struct {
	To   string                     `json:"to"`
	SMTP *controlplane.SMTPSettings `json:"smtp,omitempty"` // Optional override
}

// SendTestEmail sends a test email using either provided SMTP settings or stored tenant settings.
func SendTestEmail(w http.ResponseWriter, r *http.Request, t *controlplane.Tenant) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method_not_allowed", "solo POST", 5090)
		return
	}

	var req TestEmailReq
	if !httpx.ReadJSON(w, r, &req) {
		return
	}

	req.To = strings.TrimSpace(req.To)
	if req.To == "" {
		httpx.WriteError(w, http.StatusBadRequest, "missing_fields", "campo 'to' requerido", 5091)
		return
	}

	// Determine effective SMTP settings
	var smtpCfg *controlplane.SMTPSettings

	// If provided in request, use it (testing form values)
	if req.SMTP != nil {
		smtpCfg = req.SMTP
	} else {
		// Fallback to stored settings
		if t.Settings.SMTP != nil {
			smtpCfg = t.Settings.SMTP
			// Decrypt password if needed
			if smtpCfg.PasswordEnc != "" && smtpCfg.Password == "" {
				masterKey := os.Getenv("SIGNING_MASTER_KEY")
				if masterKey != "" {
					if encBytes, err := jwtx.DecodeBase64URL(smtpCfg.PasswordEnc); err == nil {
						if dec, err := jwtx.DecryptPrivateKey(encBytes, masterKey); err == nil {
							smtpCfg.Password = string(dec)
						}
					}
				}
			}
		}
	}

	if smtpCfg == nil || smtpCfg.Host == "" {
		httpx.WriteError(w, http.StatusBadRequest, "smtp_missing", "no se encontraron credenciales SMTP para probar", 5092)
		return
	}

	// Validate minimal fields
	if smtpCfg.Port == 0 {
		smtpCfg.Port = 587 // Default standard submission port
	}

	sender := email.NewSMTPSender(
		smtpCfg.Host,
		smtpCfg.Port,
		smtpCfg.FromEmail,
		smtpCfg.Username,
		smtpCfg.Password,
	)
	if smtpCfg.UseTLS {
		sender.TLSMode = "starttls" // or logic to map boolean to mode if needed.
		// Our UI sends useTLS as boolean. SMTPSender uses string mode.
		// Let's assume boolean true means "secure" -> starttls or ssl?
		// Usually true = implicit SSL/wrappers? Or STARTTLS?
		// Let's interpret true as auto/starttls since 465 vs 587 differs.
		// Actually email package defaults to "auto".
		// If UI specifically requests UseTLS, maybe we enforce?
		// For now simple pass through.
	}

	start := time.Now()
	subject := fmt.Sprintf("Mailing Test - %s", t.Name)
	bodyHTML := fmt.Sprintf(`
		<h1>Mailing Configurado Correctamente</h1>
		<p>Hola,</p>
		<p>Este es un correo de prueba enviado desde <strong>%s</strong>.</p>
		<p>Configuración usada:</p>
		<ul>
			<li>Host: %s</li>
			<li>Port: %d</li>
			<li>User: %s</li>
		</ul>
		<p>Enviado en: %s</p>
	`, t.Name, smtpCfg.Host, smtpCfg.Port, smtpCfg.Username, start.Format(time.RFC1123))

	bodyText := fmt.Sprintf("Mailing Configurado Correctamente\n\nEste es un correo de prueba desde %s.\nConfiguración: Host=%s Port=%d User=%s\n",
		t.Name, smtpCfg.Host, smtpCfg.Port, smtpCfg.Username)

	if err := sender.Send(req.To, subject, bodyHTML, bodyText); err != nil {
		diag := email.DiagnoseSMTP(err)
		httpx.WriteError(w, http.StatusBadGateway, "smtp_error", fmt.Sprintf("Error enviando: %v (Code: %s)", err, diag.Code), 5093)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"sent_to": req.To,
	})
}
