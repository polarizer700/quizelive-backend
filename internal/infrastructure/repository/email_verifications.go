package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository/sqlcdb"
)

type EmailVerificationRepository struct {
	pool *pgxpool.Pool
}

func NewEmailVerificationRepository(pool *pgxpool.Pool) *EmailVerificationRepository {
	return &EmailVerificationRepository{pool: pool}
}

func (r *EmailVerificationRepository) ReplacePending(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	q := sqlcdb.New(tx)
	if err = q.DeletePendingVerifications(ctx, userID); err != nil {
		return err
	}

	if err = q.InsertEmailVerification(ctx, sqlcdb.InsertEmailVerificationParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: pgTime(expiresAt),
	}); err != nil {
		return mapDB(err)
	}

	return tx.Commit(ctx)
}

func (r *EmailVerificationRepository) Consume(ctx context.Context, tokenHash []byte, at time.Time) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)

	q := sqlcdb.New(tx)
	userID, err := q.ConsumeEmailVerification(ctx, sqlcdb.ConsumeEmailVerificationParams{
		TokenHash: tokenHash,
		UsedAt:    pgTime(at),
	})
	if errors.Is(mapDB(err), domain.ErrNotFound) {
		row, lookupErr := q.GetEmailVerificationByHash(ctx, tokenHash)
		if lookupErr != nil {
			return uuid.Nil, mapDB(lookupErr)
		}

		user, lookupErr := q.GetUserByID(ctx, row.UserID)
		if lookupErr != nil {
			return uuid.Nil, mapDB(lookupErr)
		}

		if !row.UsedAt.Valid || !user.EmailVerifiedAt.Valid {
			return uuid.Nil, domain.ErrNotFound
		}

		if err = tx.Commit(ctx); err != nil {
			return uuid.Nil, err
		}

		return row.UserID, nil
	}
	if err != nil {
		return uuid.Nil, mapDB(err)
	}

	if err = q.MarkUserEmailVerified(ctx, sqlcdb.MarkUserEmailVerifiedParams{
		ID:              userID,
		EmailVerifiedAt: pgTime(at),
	}); err != nil {
		return uuid.Nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}

	return userID, nil
}
