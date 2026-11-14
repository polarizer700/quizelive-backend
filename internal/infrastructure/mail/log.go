package mail

import (
	"context"
	"log/slog"

	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
)

// LogSender пишет письмо в лог. Так локально видно ссылку подтверждения без SMTP.
type LogSender struct{}

func NewLogSender() *LogSender {
	return &LogSender{}
}

func (s *LogSender) Send(_ context.Context, message auth.Message) error {
	// Не пишем тело письма: там могут быть токены подтверждения.
	slog.Info("mail queued locally", "category", "mail", "to", message.To, "subject", message.Subject)
	return nil
}
