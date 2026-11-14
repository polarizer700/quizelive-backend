package repository

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
)

type Page struct {
	Limit  int32
	Offset int32
}

type UserRepository interface {
	Create(ctx context.Context, email, passwordHash string, role entity.Role, verifiedAt *time.Time) (entity.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (entity.User, error)
	GetByEmail(ctx context.Context, email string) (entity.User, error)
	List(ctx context.Context, page Page) ([]entity.User, int64, error)
	Update(ctx context.Context, user entity.User) (entity.User, error)
	MarkEmailVerified(ctx context.Context, id uuid.UUID, at time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type EmailVerificationRepository interface {
	// ReplacePending забывает старые неиспользованные ссылки и сохраняет новую.
	ReplacePending(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
	// Consume помечает ссылку использованной и подтверждает почту пользователя.
	Consume(ctx context.Context, tokenHash []byte, at time.Time) (uuid.UUID, error)
}

type PasswordResetRepository interface {
	ReplacePending(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error
	Consume(ctx context.Context, tokenHash []byte, at time.Time) (uuid.UUID, error)
}

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

type RefreshTokenRepository interface {
	Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (RefreshToken, error)
	FindByHash(ctx context.Context, tokenHash []byte) (RefreshToken, error)
	// RevokeIfActive отзывает токен, только если он ещё активен.
	// false значит, что токен уже отозван или не найден.
	RevokeIfActive(ctx context.Context, id uuid.UUID, at time.Time) (bool, error)
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error
}

type QuizFilter struct {
	OwnerID *uuid.UUID
	Status  *entity.QuizStatus
	Page    Page
}

type QuizRepository interface {
	Create(ctx context.Context, quiz entity.Quiz) (entity.Quiz, error)
	Get(ctx context.Context, id uuid.UUID) (entity.Quiz, error)
	List(ctx context.Context, filter QuizFilter) ([]entity.Quiz, int64, error)
	Update(ctx context.Context, quiz entity.Quiz) (entity.Quiz, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type GameResult struct {
	Nickname string
	Score    int
	Rank     int
}

type GameSessionRepository interface {
	Create(ctx context.Context, quizID, hostID uuid.UUID, code string) (uuid.UUID, error)
	Finish(ctx context.Context, sessionID uuid.UUID, finishedAt time.Time, results []GameResult) error
}

type QuestionRepository interface {
	Create(ctx context.Context, question entity.Question) (entity.Question, error)
	Get(ctx context.Context, quizID, questionID uuid.UUID) (entity.Question, error)
	ListByQuiz(ctx context.Context, quizID uuid.UUID) ([]entity.Question, error)
	Update(ctx context.Context, question entity.Question) (entity.Question, error)
	Delete(ctx context.Context, quizID, questionID uuid.UUID) error
	Reorder(ctx context.Context, quizID uuid.UUID, ids []uuid.UUID) error
	SetImage(ctx context.Context, quizID, questionID uuid.UUID, key, contentType string, size int64) (entity.Question, error)
	ClearImage(ctx context.Context, quizID, questionID uuid.UUID) (entity.Question, string, error)
}
