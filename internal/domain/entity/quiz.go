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

type QuizGameType string

const (
	GameTypeQuiz     QuizGameType = "quiz"
	GameTypePubQuiz  QuizGameType = "pub_quiz"
	GameTypeJeopardy QuizGameType = "jeopardy"
)

func (t QuizGameType) Valid() bool {
	switch t {
	case GameTypeQuiz, GameTypePubQuiz, GameTypeJeopardy:
		return true
	default:
		return false
	}
}

const MaxBreakAfterRound = 10

type Quiz struct {
	ID              uuid.UUID
	Title           string
	Description     string
	OwnerID         uuid.UUID
	Status          QuizStatus
	GameType        QuizGameType
	BreakAfterRound int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NormalizeGameConfig проверяет тип игры и раунд перерыва. Перерыв имеет смысл только для pub_quiz.
// PublicGame — опубликованная игра из общего каталога.
type PublicGame struct {
	ID             uuid.UUID
	Title          string
	Description    string
	OwnerID        uuid.UUID
	GameType       QuizGameType
	QuestionsCount int
	CreatedAt      time.Time
}

func NormalizeGameConfig(gameType QuizGameType, breakAfterRound int) error {
	if !gameType.Valid() {
		return domain.FieldErrorf("game_type", "invalid", "game_type must be quiz, pub_quiz or jeopardy")
	}

	if breakAfterRound < 0 || breakAfterRound > MaxBreakAfterRound {
		return domain.FieldErrorf("break_after_round", "invalid", "break_after_round must be between 0 and 10")
	}

	if breakAfterRound > 0 && gameType != GameTypePubQuiz {
		return domain.FieldErrorf("break_after_round", "invalid", "перерыв доступен только в квизе")
	}

	return nil
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

	// Опубликованные игры — общий каталог: читать может любая роль.
	if status == QuizStatusPublished {
		return true
	}

	return role == RoleEditor && userID == ownerID
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
