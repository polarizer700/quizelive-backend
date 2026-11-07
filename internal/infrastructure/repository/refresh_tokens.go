package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository/sqlcdb"
)

type RefreshTokenRepository struct {
	q *sqlcdb.Queries
}

func NewRefreshTokenRepository(pool *pgxpool.Pool) *RefreshTokenRepository {
	return &RefreshTokenRepository{q: sqlcdb.New(pool)}
}

func (r *RefreshTokenRepository) Insert(ctx context.Context, userID uuid.UUID, tokenHash []byte, expiresAt time.Time) (repository.RefreshToken, error) {
	row, err := r.q.InsertRefreshToken(ctx, sqlcdb.InsertRefreshTokenParams{
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: pgTime(expiresAt),
	})
	if err != nil {
		return repository.RefreshToken{}, mapDB(err)
	}

	return mapRefresh(row), nil
}

func (r *RefreshTokenRepository) FindByHash(ctx context.Context, tokenHash []byte) (repository.RefreshToken, error) {
	row, err := r.q.GetRefreshTokenByHash(ctx, tokenHash)
	if err != nil {
		return repository.RefreshToken{}, mapDB(err)
	}

	return mapRefresh(row), nil
}

func (r *RefreshTokenRepository) RevokeIfActive(ctx context.Context, id uuid.UUID, at time.Time) (bool, error) {
	affected, err := r.q.RevokeRefreshTokenIfActive(ctx, sqlcdb.RevokeRefreshTokenIfActiveParams{
		ID:        id,
		RevokedAt: pgTime(at),
	})
	if err != nil {
		return false, mapDB(err)
	}

	return affected > 0, nil
}

func (r *RefreshTokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID, at time.Time) error {
	return mapDB(r.q.RevokeRefreshTokensByUser(ctx, sqlcdb.RevokeRefreshTokensByUserParams{
		UserID:    userID,
		RevokedAt: pgTime(at),
	}))
}

func mapRefresh(row sqlcdb.RefreshToken) repository.RefreshToken {
	return repository.RefreshToken{
		ID:        row.ID,
		UserID:    row.UserID,
		ExpiresAt: timeFrom(row.ExpiresAt),
		RevokedAt: timePtr(row.RevokedAt),
	}
}
