package email

import (
	"crypto/tls"
	"fmt"
	"log"

	mail "github.com/go-mail/mail"
)

type Sender interface {
	Send(to string, subject string, htmlBody string, textBody string) error
}

type SMTPSender struct {
	Host               string
	Port               int
	From               string
	User               string
	Pass               string
	TLSMode            string // "auto" | "starttls" | "ssl" | "none"
	InsecureSkipVerify bool
}

func NewSMTPSender(host string, port int, from, user, pass string) *SMTPSender {
	return &SMTPSender{
		Host:    host,
		Port:    port,
		From:    from,
		User:    user,
		Pass:    pass,
		TLSMode: "auto",
	}
}

func (s *SMTPSender) Send(to, subject, htmlBody, textBody string) error {
	log.Printf(`{"level":"info","msg":"smtp_send_try","host":"%s","port":%d,"from":"%s","to":"%s","subject":"%s","tls_mode":"%s"}`, s.Host, s.Port, s.From, to, subject, s.TLSMode)

	m := mail.NewMessage()
	m.SetHeader("From", s.From)
	m.SetHeader("To", to)
	m.SetHeader("Subject", subject)

	// Preferimos multipart/alternative (txt + html)
	if textBody != "" {
		m.SetBody("text/plain", textBody)
	}
	if htmlBody != "" {
		if textBody == "" {
			m.SetBody("text/html", htmlBody)
		} else {
			m.AddAlternative("text/html", htmlBody)
		}
	}

	d := mail.NewDialer(s.Host, s.Port, s.User, s.Pass)
	d.TLSConfig = &tls.Config{
		ServerName:         s.Host,
		InsecureSkipVerify: s.InsecureSkipVerify, // sólo dev
	}

	switch s.TLSMode {
	case "ssl":
		d.SSL = true
	case "none":
		d.TLSConfig = &tls.Config{InsecureSkipVerify: s.InsecureSkipVerify}
	default:
		// "auto"/"starttls": go-mail negocia STARTTLS si corresponde
	}

	if err := d.DialAndSend(m); err != nil {
		log.Printf(`{"level":"error","msg":"smtp_send_err","to":"%s","err":"%v"}`, to, err)
		return fmt.Errorf("smtp send: %w", err)
	}
	log.Printf(`{"level":"info","msg":"smtp_send_ok","to":"%s"}`, to)
	return nil
}
