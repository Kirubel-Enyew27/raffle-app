package email

import (
	"context"
	"fmt"
	"net/smtp"
)

// SMTPSender sends email via a plain SMTP relay using net/smtp (stdlib).
type SMTPSender struct {
	host string // e.g. "smtp.example.com"
	port string // e.g. "587"
	from string
	auth smtp.Auth // nil for unauthenticated relays
}

func NewSMTPSender(host, port, from, user, password string) *SMTPSender {
	var auth smtp.Auth
	if user != "" {
		auth = smtp.PlainAuth("", user, password, host)
	}
	return &SMTPSender{host: host, port: port, from: from, auth: auth}
}

func (s *SMTPSender) Send(_ context.Context, to, subject, body string) error {
	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s",
		s.from, to, subject, body,
	))
	addr := s.host + ":" + s.port
	return smtp.SendMail(addr, s.auth, s.from, []string{to}, msg)
}
