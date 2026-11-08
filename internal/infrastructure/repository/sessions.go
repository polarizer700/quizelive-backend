package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

type SessionRepository struct {
	pool *pgxpool.Pool
}

func NewSessionRepository(pool *pgxpool.Pool) *SessionRepository {
	return &SessionRepository{pool: pool}
}

func (r *SessionRepository) Create(ctx context.Context, quizID, hostID uuid.UUID, code string) (uuid.UUID, error) {
	var id uuid.UUID
	err := r.pool.QueryRow(ctx, `
		INSERT INTO game_sessions (quiz_id, host_id, code)
		VALUES ($1, $2, $3)
		RETURNING id
	`, quizID, hostID, code).Scan(&id)
	if err != nil {
		return uuid.Nil, mapDB(err)
	}

	return id, nil
}

func (r *SessionRepository) Finish(ctx context.Context, sessionID uuid.UUID, finishedAt time.Time, results []repository.GameResult) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() { _ = tx.Rollback(ctx) }()

	if _, err = tx.Exec(ctx, `
		UPDATE game_sessions
		SET finished_at = $2,
		    started_at = COALESCE(started_at, created_at)
		WHERE id = $1
	`, sessionID, finishedAt); err != nil {
		return err
	}

	for _, result := range results {
		if _, err = tx.Exec(ctx, `
			INSERT INTO game_results (session_id, nickname, score, rank)
			VALUES ($1, $2, $3, $4)
		`, sessionID, result.Nickname, result.Score, result.Rank); err != nil {
			return mapDB(err)
		}
	}

	return tx.Commit(ctx)
}
