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
	slog.Info("mail", "to", message.To, "subject", message.Subject, "text", message.Text)
	return nil
}
