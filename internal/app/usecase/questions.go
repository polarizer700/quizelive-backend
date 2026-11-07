package usecase

import (
	"context"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

type Questions struct {
	quizzes   repository.QuizRepository
	questions repository.QuestionRepository
}

func NewQuestions(quizzes repository.QuizRepository, questions repository.QuestionRepository) *Questions {
	return &Questions{quizzes: quizzes, questions: questions}
}

func (s *Questions) List(ctx context.Context, quizID uuid.UUID) ([]entity.Question, error) {
	if _, _, err := s.ensureRead(ctx, quizID); err != nil {
		return nil, err
	}

	return s.questions.ListByQuiz(ctx, quizID)
}

func (s *Questions) Get(ctx context.Context, quizID, questionID uuid.UUID) (entity.Question, error) {
	if _, _, err := s.ensureRead(ctx, quizID); err != nil {
		return entity.Question{}, err
	}

	return s.questions.Get(ctx, quizID, questionID)
}

func (s *Questions) Create(ctx context.Context, quizID uuid.UUID, input entity.Question) (entity.Question, error) {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return entity.Question{}, err
	}

	prepared, err := entity.PrepareQuestion(input)
	if err != nil {
		return entity.Question{}, err
	}

	prepared.QuizID = quizID

	return s.questions.Create(ctx, prepared)
}

func (s *Questions) Update(ctx context.Context, quizID, questionID uuid.UUID, input entity.Question) (entity.Question, error) {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return entity.Question{}, err
	}

	current, err := s.questions.Get(ctx, quizID, questionID)
	if err != nil {
		return entity.Question{}, err
	}

	prepared, err := entity.PrepareQuestion(input)
	if err != nil {
		return entity.Question{}, err
	}

	prepared.ID = current.ID
	prepared.QuizID = current.QuizID
	prepared.Position = current.Position

	return s.questions.Update(ctx, prepared)
}

func (s *Questions) Delete(ctx context.Context, quizID, questionID uuid.UUID) error {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return err
	}

	return s.questions.Delete(ctx, quizID, questionID)
}

func (s *Questions) Reorder(ctx context.Context, quizID uuid.UUID, ids []uuid.UUID) ([]entity.Question, error) {
	if _, _, err := s.ensureEdit(ctx, quizID); err != nil {
		return nil, err
	}

	current, err := s.questions.ListByQuiz(ctx, quizID)
	if err != nil {
		return nil, err
	}

	if len(ids) != len(current) || len(ids) == 0 && len(current) != 0 {
		return nil, domain.FieldErrorf("question_ids", "invalid", "question_ids must contain every question exactly once")
	}

	existing := make(map[uuid.UUID]struct{}, len(current))
	for _, question := range current {
		existing[question.ID] = struct{}{}
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := existing[id]; !ok {
			return nil, domain.FieldErrorf("question_ids", "invalid", "question_ids must contain every question exactly once")
		}

		if _, ok := seen[id]; ok {
			return nil, domain.FieldErrorf("question_ids", "invalid", "question_ids must contain every question exactly once")
		}

		seen[id] = struct{}{}
	}

	if err = s.questions.Reorder(ctx, quizID, ids); err != nil {
		return nil, err
	}

	return s.questions.ListByQuiz(ctx, quizID)
}

func (s *Questions) ensureRead(ctx context.Context, quizID uuid.UUID) (entity.Quiz, bool, error) {
	_, quiz, err := readableQuiz(ctx, s.quizzes, quizID)
	if err != nil {
		return entity.Quiz{}, false, err
	}

	return quiz, true, nil
}

func (s *Questions) ensureEdit(ctx context.Context, quizID uuid.UUID) (entity.Quiz, bool, error) {
	actor, quiz, err := readableQuiz(ctx, s.quizzes, quizID)
	if err != nil {
		return entity.Quiz{}, false, err
	}

	if !entity.CanEditQuiz(actor.Role, actor.UserID, quiz.OwnerID) {
		return entity.Quiz{}, false, domain.ErrForbidden
	}

	return quiz, true, nil
}
