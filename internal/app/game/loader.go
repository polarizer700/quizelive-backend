package gameapp

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/config"
	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/game"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

type Catalog struct {
	QuizID    uuid.UUID
	HostID    uuid.UUID
	SessionID uuid.UUID
	Questions []game.Question
}

type Loader struct {
	parser         auth.HostTokenParser
	quizzes        repository.QuizRepository
	questions      repository.QuestionRepository
	sessions       repository.GameSessionRepository
	mediaPublicURL string
}

func NewLoader(
	parser auth.HostTokenParser,
	quizzes repository.QuizRepository,
	questions repository.QuestionRepository,
	sessions repository.GameSessionRepository,
	cfg *config.Config,
) *Loader {
	return &Loader{
		parser:         parser,
		quizzes:        quizzes,
		questions:      questions,
		sessions:       sessions,
		mediaPublicURL: strings.TrimRight(cfg.MediaPublicURL, "/"),
	}
}

func (l *Loader) Identify(ctx context.Context, hostToken string) (uuid.UUID, uuid.UUID, error) {
	hostID, quizID, err := l.parser.ParseHost(hostToken)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	quiz, err := l.quizzes.Get(ctx, quizID)
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}

	if quiz.Status != entity.QuizStatusPublished {
		return uuid.Nil, uuid.Nil, &domain.ConflictError{Message: "викторина не опубликована"}
	}

	return hostID, quiz.ID, nil
}

func (l *Loader) Open(ctx context.Context, hostToken, code string) (Catalog, error) {
	hostID, quizID, err := l.parser.ParseHost(hostToken)
	if err != nil {
		return Catalog{}, err
	}

	quiz, err := l.quizzes.Get(ctx, quizID)
	if err != nil {
		return Catalog{}, err
	}

	if quiz.Status != entity.QuizStatusPublished {
		return Catalog{}, &domain.ConflictError{Message: "викторина не опубликована"}
	}

	rows, err := l.questions.ListByQuiz(ctx, quiz.ID)
	if err != nil {
		return Catalog{}, err
	}

	if len(rows) == 0 {
		return Catalog{}, &domain.ConflictError{Message: "в викторине нет вопросов"}
	}

	sessionID, err := l.sessions.Create(ctx, quiz.ID, hostID, code)
	if err != nil {
		return Catalog{}, err
	}

	return Catalog{
		QuizID:    quiz.ID,
		HostID:    hostID,
		SessionID: sessionID,
		Questions: l.snapshot(rows),
	}, nil
}

func (l *Loader) snapshot(rows []entity.Question) []game.Question {
	out := make([]game.Question, 0, len(rows))
	for _, row := range rows {
		options := make([]game.Option, 0, len(row.Options))
		for _, option := range row.Options {
			options = append(options, game.Option{
				ID:        option.ID.String(),
				Text:      option.Text,
				IsCorrect: option.IsCorrect,
			})
		}

		out = append(out, game.Question{
			ID:       row.ID.String(),
			Type:     row.Type,
			Text:     row.Text,
			Limit:    time.Duration(row.TimeLimitSec) * time.Second,
			Points:   row.Points,
			Options:  options,
			Answers:  append([]string(nil), row.TextAnswers...),
			ImageURL: mediaURL(l.mediaPublicURL, row.ImageKey, row.ImageURL),
		})
	}

	return out
}

func mediaURL(base, key, existing string) string {
	if existing != "" {
		return existing
	}
	if base == "" || key == "" {
		return ""
	}

	return base + "/" + strings.TrimPrefix(key, "/")
}
