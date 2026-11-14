package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository/sqlcdb"
)

type PasswordResetRepository struct {
	pool *pgxpool.Pool
}

func NewPasswordResetRepository(pool *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{pool: pool}
}

func (r *PasswordResetRepository) ReplacePending(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := sqlcdb.New(tx)
	if err = q.DeletePendingPasswordResets(ctx, userID); err != nil {
		return err
	}

	if err = q.InsertPasswordReset(ctx, sqlcdb.InsertPasswordResetParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: pgTime(expiresAt),
	}); err != nil {
		return mapDB(err)
	}

	return tx.Commit(ctx)
}

func (r *PasswordResetRepository) Consume(ctx context.Context, tokenHash []byte, at time.Time) (uuid.UUID, error) {
	q := sqlcdb.New(r.pool)
	userID, err := q.ConsumePasswordReset(ctx, sqlcdb.ConsumePasswordResetParams{
		TokenHash: tokenHash,
		UsedAt:    pgTime(at),
	})
	if err != nil {
		return uuid.Nil, mapDB(err)
	}

	return userID, nil
}
