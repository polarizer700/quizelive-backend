package adminhttp

import (
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/transport/http/openapi"
)

func errInvalidBody() error {
	return domain.FieldErrorf("body", "invalid", "request body is invalid")
}

func errUnauthorized() error {
	return domain.ErrUnauthorized
}

func toUser(user entity.User) openapi.User {
	return openapi.User{
		Id:        openapi_types.UUID(user.ID),
		Email:     openapi_types.Email(user.Email),
		Role:      openapi.UserRole(user.Role),
		CreatedAt: user.CreatedAt,
	}
}

func toUsers(users []entity.User) []openapi.User {
	out := make([]openapi.User, 0, len(users))
	for _, user := range users {
		out = append(out, toUser(user))
	}

	return out
}

func toQuiz(quiz entity.Quiz) openapi.Quiz {
	return openapi.Quiz{
		Id:              openapi_types.UUID(quiz.ID),
		Title:           quiz.Title,
		Description:     quiz.Description,
		OwnerId:         openapi_types.UUID(quiz.OwnerID),
		Status:          openapi.QuizStatus(quiz.Status),
		GameType:        openapi.QuizGameType(quiz.GameType),
		BreakAfterRound: quiz.BreakAfterRound,
		CreatedAt:       quiz.CreatedAt,
		UpdatedAt:       quiz.UpdatedAt,
	}
}

func toPublicGame(game entity.PublicGame) openapi.PublicGame {
	return openapi.PublicGame{
		Id:             openapi_types.UUID(game.ID),
		Title:          game.Title,
		Description:    game.Description,
		OwnerId:        openapi_types.UUID(game.OwnerID),
		GameType:       openapi.QuizGameType(game.GameType),
		QuestionsCount: game.QuestionsCount,
		CreatedAt:      game.CreatedAt,
	}
}

func toQuizzes(quizzes []entity.Quiz) []openapi.Quiz {
	out := make([]openapi.Quiz, 0, len(quizzes))
	for _, quiz := range quizzes {
		out = append(out, toQuiz(quiz))
	}

	return out
}

func toQuestion(question entity.Question) openapi.Question {
	options := make([]openapi.AnswerOption, 0, len(question.Options))
	for _, option := range question.Options {
		options = append(options, openapi.AnswerOption{
			Id:        openapi_types.UUID(option.ID),
			Position:  option.Position,
			Text:      option.Text,
			IsCorrect: option.IsCorrect,
		})
	}

	answers := question.TextAnswers
	if answers == nil {
		answers = []string{}
	}

	out := openapi.Question{
		Id:           openapi_types.UUID(question.ID),
		QuizId:       openapi_types.UUID(question.QuizID),
		Position:     question.Position,
		Type:         openapi.QuestionType(question.Type),
		Text:         question.Text,
		TimeLimitSec: question.TimeLimitSec,
		Points:       question.Points,
		Category:     question.Category,
		Options:      options,
		TextAnswers:  answers,
	}
	if question.ImageURL != "" {
		url := question.ImageURL
		out.ImageUrl = &url
	}

	return out
}

func toQuestions(questions []entity.Question) []openapi.Question {
	out := make([]openapi.Question, 0, len(questions))
	for _, question := range questions {
		out = append(out, toQuestion(question))
	}

	return out
}

func fromQuestionInput(input openapi.QuestionInput) entity.Question {
	question := entity.Question{
		Type:         entity.QuestionType(input.Type),
		Text:         input.Text,
		TimeLimitSec: input.TimeLimitSec,
		Points:       input.Points,
	}

	if input.Options != nil {
		for _, option := range *input.Options {
			question.Options = append(question.Options, entity.AnswerOption{
				Text:      option.Text,
				IsCorrect: option.IsCorrect,
			})
		}
	}

	if input.TextAnswers != nil {
		question.TextAnswers = append(question.TextAnswers, (*input.TextAnswers)...)
	}

	if input.Category != nil {
		question.Category = *input.Category
	}

	return question
}

func uuidFrom(id openapi_types.UUID) uuid.UUID {
	return uuid.UUID(id)
}

func pageArgs(limit, offset *int) (int, int) {
	pageLimit := 0
	pageOffset := 0

	if limit != nil {
		pageLimit = *limit
	}

	if offset != nil {
		pageOffset = *offset
	}

	return pageLimit, pageOffset
}
