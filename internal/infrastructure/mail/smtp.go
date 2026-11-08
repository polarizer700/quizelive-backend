package mail

import (
	"context"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"

	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
)

// SMTPSender отправляет письмо через любой SMTP: Mailpit, Яндекс, Resend, Unisender.
type SMTPSender struct {
	addr string
	from string
	auth smtp.Auth
}

func NewSMTPSender(host, port, username, password, from string) *SMTPSender {
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	return &SMTPSender{
		addr: net.JoinHostPort(host, port),
		from: from,
		auth: auth,
	}
}

func (s *SMTPSender) Send(_ context.Context, message auth.Message) error {
	envelope := s.from
	if parsed, err := mail.ParseAddress(s.from); err == nil {
		envelope = parsed.Address
	}

	body := strings.Join([]string{
		"From: " + s.from,
		"To: " + message.To,
		"Subject: " + mime.QEncoding.Encode("utf-8", message.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		message.Text,
	}, "\r\n")

	if err := smtp.SendMail(s.addr, s.auth, envelope, []string{message.To}, []byte(body)); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}

	return nil
}
