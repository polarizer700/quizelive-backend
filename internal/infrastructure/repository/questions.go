package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository/sqlcdb"
)

type QuestionRepository struct {
	pool *pgxpool.Pool
	q    *sqlcdb.Queries
}

func NewQuestionRepository(pool *pgxpool.Pool) *QuestionRepository {
	return &QuestionRepository{pool: pool, q: sqlcdb.New(pool)}
}

func (r *QuestionRepository) Create(ctx context.Context, question entity.Question) (entity.Question, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return entity.Question{}, err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)

	position, err := q.NextQuestionPosition(ctx, question.QuizID)
	if err != nil {
		return entity.Question{}, mapDB(err)
	}

	row, err := q.CreateQuestion(ctx, sqlcdb.CreateQuestionParams{
		QuizID:       question.QuizID,
		Position:     position,
		Type:         string(question.Type),
		Text:         question.Text,
		TimeLimitSec: int32(question.TimeLimitSec),
		Points:       int32(question.Points),
	})
	if err != nil {
		return entity.Question{}, mapDB(err)
	}

	options, err := replaceChildren(ctx, q, row.ID, question)
	if err != nil {
		return entity.Question{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return entity.Question{}, err
	}

	return assemble(row, options, question.TextAnswers), nil
}

func (r *QuestionRepository) Get(ctx context.Context, quizID, questionID uuid.UUID) (entity.Question, error) {
	questions, err := r.ListByQuiz(ctx, quizID)
	if err != nil {
		return entity.Question{}, err
	}

	for _, question := range questions {
		if question.ID == questionID {
			return question, nil
		}
	}

	return entity.Question{}, domain.ErrNotFound
}

func (r *QuestionRepository) ListByQuiz(ctx context.Context, quizID uuid.UUID) ([]entity.Question, error) {
	rows, err := r.q.ListQuestionsByQuiz(ctx, quizID)
	if err != nil {
		return nil, mapDB(err)
	}

	options, err := r.q.ListOptionsByQuiz(ctx, quizID)
	if err != nil {
		return nil, mapDB(err)
	}

	answers, err := r.q.ListTextAnswersByQuiz(ctx, quizID)
	if err != nil {
		return nil, mapDB(err)
	}

	optionsByQuestion := map[uuid.UUID][]entity.AnswerOption{}
	for _, option := range options {
		optionsByQuestion[option.QuestionID] = append(optionsByQuestion[option.QuestionID], entity.AnswerOption{
			ID:        option.ID,
			Position:  int(option.Position),
			Text:      option.Text,
			IsCorrect: option.IsCorrect,
		})
	}

	answersByQuestion := map[uuid.UUID][]string{}
	for _, answer := range answers {
		answersByQuestion[answer.QuestionID] = append(answersByQuestion[answer.QuestionID], answer.NormalizedValue)
	}

	out := make([]entity.Question, 0, len(rows))
	for _, row := range rows {
		out = append(out, assemble(row, optionsByQuestion[row.ID], answersByQuestion[row.ID]))
	}

	return out, nil
}

func (r *QuestionRepository) Update(ctx context.Context, question entity.Question) (entity.Question, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return entity.Question{}, err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)

	row, err := q.UpdateQuestion(ctx, sqlcdb.UpdateQuestionParams{
		ID:           question.ID,
		Type:         string(question.Type),
		Text:         question.Text,
		TimeLimitSec: int32(question.TimeLimitSec),
		Points:       int32(question.Points),
	})
	if err != nil {
		return entity.Question{}, mapDB(err)
	}

	options, err := replaceChildren(ctx, q, row.ID, question)
	if err != nil {
		return entity.Question{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return entity.Question{}, err
	}

	return assemble(row, options, question.TextAnswers), nil
}

func (r *QuestionRepository) Delete(ctx context.Context, quizID, questionID uuid.UUID) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)

	current, err := q.ListQuestionsByQuiz(ctx, quizID)
	if err != nil {
		return mapDB(err)
	}

	found := false
	for _, question := range current {
		if question.ID == questionID {
			found = true
			break
		}
	}

	if !found {
		return domain.ErrNotFound
	}

	if err = q.DeleteQuestion(ctx, sqlcdb.DeleteQuestionParams{ID: questionID, QuizID: quizID}); err != nil {
		return mapDB(err)
	}

	if err = q.NegateQuestionPositions(ctx, quizID); err != nil {
		return mapDB(err)
	}

	position := int32(1)
	for _, question := range current {
		if question.ID == questionID {
			continue
		}

		if err = q.SetQuestionPosition(ctx, sqlcdb.SetQuestionPositionParams{
			QuizID:   quizID,
			ID:       question.ID,
			Position: position,
		}); err != nil {
			return mapDB(err)
		}

		position++
	}

	return tx.Commit(ctx)
}

func (r *QuestionRepository) Reorder(ctx context.Context, quizID uuid.UUID, ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	q := r.q.WithTx(tx)
	if err = q.NegateQuestionPositions(ctx, quizID); err != nil {
		return mapDB(err)
	}

	for i, id := range ids {
		if err = q.SetQuestionPosition(ctx, sqlcdb.SetQuestionPositionParams{
			QuizID:   quizID,
			ID:       id,
			Position: int32(i + 1),
		}); err != nil {
			return mapDB(err)
		}
	}

	return tx.Commit(ctx)
}

func replaceChildren(ctx context.Context, q *sqlcdb.Queries, questionID uuid.UUID, question entity.Question) ([]entity.AnswerOption, error) {
	if err := q.DeleteOptionsByQuestion(ctx, questionID); err != nil {
		return nil, mapDB(err)
	}

	if err := q.DeleteTextAnswersByQuestion(ctx, questionID); err != nil {
		return nil, mapDB(err)
	}

	options := make([]entity.AnswerOption, 0, len(question.Options))
	for _, option := range question.Options {
		row, err := q.CreateOption(ctx, sqlcdb.CreateOptionParams{
			QuestionID: questionID,
			Position:   int32(option.Position),
			Text:       option.Text,
			IsCorrect:  option.IsCorrect,
		})
		if err != nil {
			return nil, mapDB(err)
		}

		options = append(options, entity.AnswerOption{
			ID:        row.ID,
			Position:  int(row.Position),
			Text:      row.Text,
			IsCorrect: row.IsCorrect,
		})
	}

	for _, answer := range question.TextAnswers {
		if err := q.CreateTextAnswer(ctx, sqlcdb.CreateTextAnswerParams{
			QuestionID:      questionID,
			NormalizedValue: answer,
		}); err != nil {
			return nil, mapDB(err)
		}
	}

	return options, nil
}

func assemble(row sqlcdb.Question, options []entity.AnswerOption, answers []string) entity.Question {
	if options == nil {
		options = []entity.AnswerOption{}
	}

	if answers == nil {
		answers = []string{}
	}

	return entity.Question{
		ID:           row.ID,
		QuizID:       row.QuizID,
		Position:     int(row.Position),
		Type:         entity.QuestionType(row.Type),
		Text:         row.Text,
		TimeLimitSec: int(row.TimeLimitSec),
		Points:       int(row.Points),
		Options:      options,
		TextAnswers:  answers,
	}
}
