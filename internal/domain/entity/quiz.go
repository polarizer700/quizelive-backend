package entity

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
)

type QuizStatus string

const (
	QuizStatusDraft     QuizStatus = "draft"
	QuizStatusPublished QuizStatus = "published"
)

func (s QuizStatus) Valid() bool {
	return s == QuizStatusDraft || s == QuizStatusPublished
}

type Quiz struct {
	ID          uuid.UUID
	Title       string
	Description string
	OwnerID     uuid.UUID
	Status      QuizStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NormalizeQuizText(title, description string) (string, string, error) {
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)

	if n := utf8.RuneCountInString(title); n < 1 || n > 200 {
		return "", "", domain.FieldErrorf("title", "invalid", "title must be 1 to 200 characters")
	}

	if utf8.RuneCountInString(description) > 2000 {
		return "", "", domain.FieldErrorf("description", "invalid", "description must be at most 2000 characters")
	}

	return title, description, nil
}

func CanReadQuiz(role Role, userID, ownerID uuid.UUID, status QuizStatus) bool {
	if role == RoleAdmin {
		return true
	}

	if role == RoleEditor && userID == ownerID {
		return true
	}

	return role == RoleHost && status == QuizStatusPublished
}

func CanEditQuiz(role Role, userID, ownerID uuid.UUID) bool {
	if role == RoleAdmin {
		return true
	}

	return role == RoleEditor && userID == ownerID
}

func CanIssueHostToken(role Role, userID, ownerID uuid.UUID, status QuizStatus) bool {
	if status != QuizStatusPublished {
		return false
	}

	if role == RoleAdmin || role == RoleHost {
		return true
	}

	return role == RoleEditor && userID == ownerID
}
