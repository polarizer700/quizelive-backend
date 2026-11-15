package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository/sqlcdb"
)

type QuizRepository struct {
	q *sqlcdb.Queries
}

func NewQuizRepository(pool *pgxpool.Pool) *QuizRepository {
	return &QuizRepository{q: sqlcdb.New(pool)}
}

func (r *QuizRepository) Create(ctx context.Context, quiz entity.Quiz) (entity.Quiz, error) {
	row, err := r.q.CreateQuiz(ctx, sqlcdb.CreateQuizParams{
		Title:           quiz.Title,
		Description:     quiz.Description,
		OwnerID:         quiz.OwnerID,
		Status:          string(quiz.Status),
		GameType:        string(quiz.GameType),
		BreakAfterRound: int32(quiz.BreakAfterRound), // #nosec G115 -- entity.NormalizeGameConfig гарантирует диапазон 0..10
	})
	if err != nil {
		return entity.Quiz{}, mapDB(err)
	}

	return mapQuiz(quizFromCreate(row)), nil
}

func (r *QuizRepository) Get(ctx context.Context, id uuid.UUID) (entity.Quiz, error) {
	row, err := r.q.GetQuiz(ctx, id)
	if err != nil {
		return entity.Quiz{}, mapDB(err)
	}

	return mapQuiz(quizFromGet(row)), nil
}

func (r *QuizRepository) List(ctx context.Context, filter repository.QuizFilter) ([]entity.Quiz, int64, error) {
	params := quizFilter(filter)

	rows, err := r.q.ListQuizzes(ctx, sqlcdb.ListQuizzesParams{
		OwnerID:   params.OwnerID,
		Status:    params.Status,
		GameType:  params.GameType,
		Search:    params.Search,
		RowLimit:  filter.Page.Limit,
		RowOffset: filter.Page.Offset,
	})
	if err != nil {
		return nil, 0, mapDB(err)
	}

	total, err := r.q.CountQuizzes(ctx, params)
	if err != nil {
		return nil, 0, mapDB(err)
	}

	quizzes := make([]entity.Quiz, 0, len(rows))
	for _, row := range rows {
		quizzes = append(quizzes, mapQuiz(quizFromList(row)))
	}

	return quizzes, total, nil
}

func (r *QuizRepository) Update(ctx context.Context, quiz entity.Quiz) (entity.Quiz, error) {
	row, err := r.q.UpdateQuiz(ctx, sqlcdb.UpdateQuizParams{
		ID:              quiz.ID,
		Title:           quiz.Title,
		Description:     quiz.Description,
		Status:          string(quiz.Status),
		GameType:        string(quiz.GameType),
		BreakAfterRound: int32(quiz.BreakAfterRound), // #nosec G115 -- entity.NormalizeGameConfig гарантирует диапазон 0..10
	})
	if err != nil {
		return entity.Quiz{}, mapDB(err)
	}

	return mapQuiz(quizFromUpdate(row)), nil
}

func (r *QuizRepository) ListPublic(ctx context.Context, filter repository.PublicGameFilter) ([]entity.PublicGame, int64, error) {
	params := publicGameFilter(filter)

	rows, err := r.q.ListPublicGames(ctx, sqlcdb.ListPublicGamesParams{
		GameType:  params.GameType,
		Search:    params.Search,
		RowLimit:  filter.Page.Limit,
		RowOffset: filter.Page.Offset,
	})
	if err != nil {
		return nil, 0, mapDB(err)
	}

	total, err := r.q.CountPublicGames(ctx, params)
	if err != nil {
		return nil, 0, mapDB(err)
	}

	games := make([]entity.PublicGame, 0, len(rows))
	for _, row := range rows {
		games = append(games, entity.PublicGame{
			ID:             row.ID,
			Title:          row.Title,
			Description:    row.Description,
			OwnerID:        row.OwnerID,
			GameType:       entity.QuizGameType(row.GameType),
			QuestionsCount: int(row.QuestionsCount),
			CreatedAt:      timeFrom(row.CreatedAt),
		})
	}

	return games, total, nil
}

func (r *QuizRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return mapDB(r.q.DeleteQuiz(ctx, id))
}

func quizFilter(filter repository.QuizFilter) sqlcdb.CountQuizzesParams {
	params := sqlcdb.CountQuizzesParams{}
	if filter.OwnerID != nil {
		params.OwnerID = pgtype.UUID{Bytes: *filter.OwnerID, Valid: true}
	}

	if filter.Status != nil {
		status := string(*filter.Status)
		params.Status = &status
	}

	if filter.GameType != nil {
		gameType := string(*filter.GameType)
		params.GameType = &gameType
	}

	if search := strings.TrimSpace(filter.Search); search != "" {
		params.Search = &search
	}

	return params
}

func publicGameFilter(filter repository.PublicGameFilter) sqlcdb.CountPublicGamesParams {
	params := sqlcdb.CountPublicGamesParams{}
	if filter.GameType != nil {
		gameType := string(*filter.GameType)
		params.GameType = &gameType
	}

	if search := strings.TrimSpace(filter.Search); search != "" {
		params.Search = &search
	}

	return params
}

func mapQuiz(row sqlcdb.Quiz) entity.Quiz {
	return entity.Quiz{
		ID:              row.ID,
		Title:           row.Title,
		Description:     row.Description,
		OwnerID:         row.OwnerID,
		Status:          entity.QuizStatus(row.Status),
		GameType:        entity.QuizGameType(row.GameType),
		BreakAfterRound: int(row.BreakAfterRound),
		CreatedAt:       timeFrom(row.CreatedAt),
		UpdatedAt:       timeFrom(row.UpdatedAt),
	}
}

func quizFromCreate(row sqlcdb.CreateQuizRow) sqlcdb.Quiz {
	return sqlcdb.Quiz{
		ID:              row.ID,
		Title:           row.Title,
		Description:     row.Description,
		OwnerID:         row.OwnerID,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		GameType:        row.GameType,
		BreakAfterRound: row.BreakAfterRound,
	}
}

func quizFromGet(row sqlcdb.GetQuizRow) sqlcdb.Quiz {
	return sqlcdb.Quiz{
		ID:              row.ID,
		Title:           row.Title,
		Description:     row.Description,
		OwnerID:         row.OwnerID,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		GameType:        row.GameType,
		BreakAfterRound: row.BreakAfterRound,
	}
}

func quizFromList(row sqlcdb.ListQuizzesRow) sqlcdb.Quiz {
	return sqlcdb.Quiz{
		ID:              row.ID,
		Title:           row.Title,
		Description:     row.Description,
		OwnerID:         row.OwnerID,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		GameType:        row.GameType,
		BreakAfterRound: row.BreakAfterRound,
	}
}

func quizFromUpdate(row sqlcdb.UpdateQuizRow) sqlcdb.Quiz {
	return sqlcdb.Quiz{
		ID:              row.ID,
		Title:           row.Title,
		Description:     row.Description,
		OwnerID:         row.OwnerID,
		Status:          row.Status,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		GameType:        row.GameType,
		BreakAfterRound: row.BreakAfterRound,
	}
}
