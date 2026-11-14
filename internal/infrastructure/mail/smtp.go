package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
)

const smtpTimeout = 12 * time.Second

// SMTPSender отправляет письмо через любой SMTP: Mailpit, Яндекс, Resend, Unisender.
type SMTPSender struct {
	host string
	port string
	from string
	auth smtp.Auth
}

func NewSMTPSender(host, port, username, password, from string) *SMTPSender {
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	return &SMTPSender{
		host: host,
		port: port,
		from: from,
		auth: auth,
	}
}

func (s *SMTPSender) Send(ctx context.Context, message auth.Message) error {
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

	deadline := time.Now().Add(smtpTimeout)
	if due, ok := ctx.Deadline(); ok && due.Before(deadline) {
		deadline = due
	}
	dialCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	var dialer net.Dialer
	conn, err := dialer.DialContext(dialCtx, "tcp", net.JoinHostPort(s.host, s.port))
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if err = conn.SetDeadline(deadline); err != nil {
		return fmt.Errorf("smtp deadline: %w", err)
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err = client.StartTLS(&tls.Config{ServerName: s.host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}

	if s.auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err = client.Auth(s.auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	if err = client.Mail(envelope); err != nil {
		return fmt.Errorf("smtp mail: %w", err)
	}
	if err = client.Rcpt(message.To); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err = writer.Write([]byte(body)); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("smtp write close: %w", err)
	}
	if err = client.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}

	return nil
}
