package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/identity"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

// Mailer отправляет письмо. Локально это лог, на сервере — SMTP любого провайдера.
type Mailer interface {
	Send(ctx context.Context, message Message) error
}

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenIssuer interface {
	IssueAccess(user entity.User) (token string, expiresIn time.Duration, err error)
	ParseAccess(token string) (identity.Identity, error)
	IssueHost(user entity.User, quizID uuid.UUID) (token string, expiresIn time.Duration, err error)
}

// HostTokenParser проверяет короткий токен ведущего, которым открывается комната.
type HostTokenParser interface {
	ParseHost(token string) (userID, quizID uuid.UUID, err error)
}
