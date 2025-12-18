/*
admin_mailing.go — Admin “Send Test Email” (SMTP config resolution + decryption + send)

Qué hace esta pieza
-------------------
Este archivo implementa un endpoint administrativo para enviar un “email de prueba” (test email)
para validar que la configuración SMTP de un tenant funciona.

El flujo principal:
  - Recibe un request POST con JSON { to, smtp? }.
  - Determina la configuración SMTP efectiva:
      A) si el request trae smtp (override), usa esa (ideal para probar valores de un formulario).
      B) si no trae smtp, usa la configuración SMTP guardada en el tenant (t.Settings.SMTP).
  - Si la config guardada trae PasswordEnc y Password está vacío, intenta descifrar PasswordEnc
    usando una master key desde environment.
  - Construye un SMTP sender (email.NewSMTPSender) y envía un mail con subject/body de prueba.
  - Devuelve 200 con {status:"ok", sent_to:"..."} o un error con httpx.WriteError.

No maneja persistencia (no guarda SMTP). Solo “prueba envío”.

Endpoints / contrato esperado
-----------------------------
Se asume que esta función se usa en una ruta administrativa (enrutada desde otro handler/router).
Solo acepta:
  - POST (si no, 405)

Request JSON:
  - to: string requerido (destinatario)
  - smtp: objeto opcional controlplane.SMTPSettings (override para probar credenciales)

Response JSON:
  - 200 OK: {"status":"ok","sent_to":"..."}
Errores:
  - 405 si no es POST
  - 400 si falta 'to' o no hay SMTP disponible
  - 502 si falla el envío SMTP (BadGateway) con diagnóstico

DTOs actuales (acoplamiento)
----------------------------
- TestEmailReq: DTO HTTP de entrada {To, SMTP?}.
- SMTPSettings: viene de controlplane (modelo interno), usado directamente como DTO de entrada.
  Esto acopla el contrato HTTP al modelo interno de controlplane, y además permite que el request
  inyecte campos que quizá no querés exponer (ej: PasswordEnc, etc.). En V2 conviene DTO propio.

Cómo determina la configuración SMTP efectiva
---------------------------------------------
1) Valida request:
   - requiere r.Method POST
   - parsea JSON con httpx.ReadJSON
   - req.To no puede estar vacío

2) Selección de configuración:
   - Si req.SMTP != nil => smtpCfg = req.SMTP (override)
   - Sino:
       - Si t.Settings.SMTP != nil => smtpCfg = t.Settings.SMTP

3) Desencriptado de password (solo en fallback stored settings):
   - Si smtpCfg.PasswordEnc != "" y smtpCfg.Password == "":
       - lee masterKey desde env var "SIGNING_MASTER_KEY"
       - intenta:
          DecodeBase64URL(PasswordEnc) -> bytes
          DecryptPrivateKey(bytes, masterKey) -> plaintext
          smtpCfg.Password = string(plaintext)

   Nota: el nombre "SIGNING_MASTER_KEY" es confuso: se usa para descifrar password SMTP, no para firmar JWT.
   También muta smtpCfg.Password en memoria (puede impactar si smtpCfg apunta a un struct compartido).

4) Validaciones mínimas:
   - Si smtpCfg nil o Host vacío => 400 smtp_missing
   - Si Port=0 => default 587

5) Construye sender y envía:
   - sender := email.NewSMTPSender(host, port, fromEmail, username, password)
   - Si UseTLS true => ajusta sender.TLSMode="starttls" (hay comentario de ambigüedad STARTTLS vs SSL)
   - Genera subject/body HTML/Text con info del tenant y config usada.
   - sender.Send(to, subject, html, text)
   - Si error => email.DiagnoseSMTP(err) y devuelve 502 con detalle

Puntos de mejora (deuda técnica / refactor hacia V2)
----------------------------------------------------
1) Separación de capas (Controller/Service/Client):
   Actualmente:
     - valida método + parsea JSON (controller)
     - resuelve config efectiva y descifra secret (service)
     - instancia SMTP sender y envía (client)
   Está todo mezclado en una función.

   En V2:
     - Controller: valida POST, parsea dto.TestEmailRequest, obtiene tenant desde TenantContext,
       llama a service.SendTestEmail(...)
     - Service: decide config efectiva, valida campos, (opcional) descifra password,
       arma contenido, llama a EmailClient.
     - Client: interfaz EmailSender con implementación SMTP.

2) Evitar mutar settings compartidas:
   Cuando usa t.Settings.SMTP, smtpCfg apunta al mismo struct del tenant. Al descifrar, se asigna smtpCfg.Password,
   lo cual puede quedar “cacheado” en memoria del tenant y se presta a leaks (por logs o dumps).
   Mejor:
     - clonar smtpCfg antes de completar Password
     - y mantener el password en una variable local solo para el envío.

3) Gestión segura de secretos:
   - No debería depender de una env var con nombre ambiguo (“SIGNING_MASTER_KEY”) para descifrar SMTP.
   - Si hay un “master key” del sistema, debería vivir en un componente SecretManager (TenantResources),
     y/o usar una env var específica (ej: SMTP_MASTER_KEY o SECRETBOX_MASTER_KEY) consistente.
   - No devolver nunca el password en responses, ni loguearlo.

4) TLS mode ambiguo:
   El código fuerza TLSMode="starttls" si UseTLS es true, pero el comentario indica ambigüedad.
   En V2: definir contrato claro:
     - tlsMode: "auto" | "starttls" | "ssl"
     - y mapearlo sin suposiciones. O bien: UseTLS + Port 465 => ssl, Port 587 => starttls.

5) Timeouts y experiencia de usuario:
   Send SMTP puede tardar o colgar si no hay timeout (depende de email package).
   En V2 conviene:
     - usar context con timeout (ej 10s) si el cliente SMTP lo soporta
     - devolver error claro si se excede el timeout

6) Concurrencia (Golang) — dónde aplica
   Un “send test email” normalmente es una acción manual, así que:
     - se puede hacer sincrónico (responder cuando termina).
   Pero para no bloquear el server ante SMTP lento, hay 2 enfoques:
     A) Sincrónico con timeout (recomendado para “test”)
     B) Asincrónico: encolar un job en un worker pool y responder 202 Accepted con un “job id”.
        Esto es útil si querés UI que muestre estado. Es más complejo, pero prolijo.

   Si adoptás worker pool:
     - tené un pool “emails” con concurrencia limitada (semáforo/pool) para no saturar salida
     - y siempre con context/cancelación en shutdown.

Mapa a arquitectura V2 (qué sería qué)
--------------------------------------
- DTOs:
  - dto.AdminTestEmailRequest { to, smtpOverride? }
  - dto.SMTPOverride { host, port, fromEmail, username, password, tlsMode }
  (evitar usar controlplane.SMTPSettings directamente como DTO público)

- Controller:
  - AdminMailingController.SendTestEmail(ctx, tenantCtx, req)

- Service:
  - MailingService.SendTestEmail(ctx, tenantID, to, smtpOverride?)
    - resuelve config: override vs tenant settings
    - valida
    - obtiene credenciales (descifra via SecretManager)
    - llama email client
    - traduce errores a códigos (smtp auth fail, timeout, dns fail, etc.)

- Client:
  - EmailClient interface:
      Send(ctx, to, subject, html, text) error
    Implementación SMTP (wrapper de email.NewSMTPSender).

Decisiones de compatibilidad (para no romper)
---------------------------------------------
- Solo POST.
- Si no se envía smtp override, usa SMTP del tenant.
- Si falta SMTP o Host => 400 smtp_missing.
- Port default 587 si viene 0.
- Ante error SMTP: 502 smtp_error con diagnóstico.

Concurrencia recomendada acá (para tu V2)

Para “test email” yo haría sincrónico con timeout (simple y UX clara).
---------------------------------------------------------------------
- Si igual querés aprovechar Go:
  + ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
  +  y tu cliente SMTP respeta ese contexto (si tu lib no lo respeta, le agregás timeouts al dial).
- Si querés “pro” con UI:
  +  Worker pool emails + queue size chico (backpressure)
  +  POST /admin/v2/mailing/test devuelve 202 Accepted + jobId
  +  GET /admin/v2/jobs/{jobId} para ver estado (esto ya es otro módulo).
*/

package handlers

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dropDatabas3/hellojohn/internal/controlplane/v1"
	"github.com/dropDatabas3/hellojohn/internal/email"
	httpx "github.com/dropDatabas3/hellojohn/internal/http/v1"
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
