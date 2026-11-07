package auth

import (
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/identity"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) error
}

type TokenIssuer interface {
	IssueAccess(user entity.User) (token string, expiresIn time.Duration, err error)
	ParseAccess(token string) (identity.Identity, error)
	IssueHost(user entity.User, quizID uuid.UUID) (token string, expiresIn time.Duration, err error)
}
