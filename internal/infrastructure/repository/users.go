package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
	"github.com/polarizer700/quizelive-backend/internal/infrastructure/repository/sqlcdb"
)

type UserRepository struct {
	q *sqlcdb.Queries
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{q: sqlcdb.New(pool)}
}

func (r *UserRepository) Create(ctx context.Context, email, passwordHash string, role entity.Role, verifiedAt *time.Time) (entity.User, error) {
	row, err := r.q.CreateUser(ctx, sqlcdb.CreateUserParams{
		Email:           email,
		PasswordHash:    passwordHash,
		Role:            string(role),
		EmailVerifiedAt: pgTimePtr(verifiedAt),
	})
	if err != nil {
		return entity.User{}, mapDB(err)
	}

	return userFrom(row.ID, row.Email, row.PasswordHash, row.Role, row.EmailVerifiedAt, row.CreatedAt), nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (entity.User, error) {
	row, err := r.q.GetUserByID(ctx, id)
	if err != nil {
		return entity.User{}, mapDB(err)
	}

	return userFrom(row.ID, row.Email, row.PasswordHash, row.Role, row.EmailVerifiedAt, row.CreatedAt), nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (entity.User, error) {
	row, err := r.q.GetUserByEmail(ctx, email)
	if err != nil {
		return entity.User{}, mapDB(err)
	}

	return userFrom(row.ID, row.Email, row.PasswordHash, row.Role, row.EmailVerifiedAt, row.CreatedAt), nil
}

func (r *UserRepository) List(ctx context.Context, page repository.Page) ([]entity.User, int64, error) {
	rows, err := r.q.ListUsers(ctx, sqlcdb.ListUsersParams{Limit: page.Limit, Offset: page.Offset})
	if err != nil {
		return nil, 0, mapDB(err)
	}

	total, err := r.q.CountUsers(ctx)
	if err != nil {
		return nil, 0, mapDB(err)
	}

	users := make([]entity.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, userFrom(row.ID, row.Email, row.PasswordHash, row.Role, row.EmailVerifiedAt, row.CreatedAt))
	}

	return users, total, nil
}

func (r *UserRepository) Update(ctx context.Context, user entity.User) (entity.User, error) {
	row, err := r.q.UpdateUser(ctx, sqlcdb.UpdateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		Role:         string(user.Role),
	})
	if err != nil {
		return entity.User{}, mapDB(err)
	}

	return userFrom(row.ID, row.Email, row.PasswordHash, row.Role, row.EmailVerifiedAt, row.CreatedAt), nil
}

func (r *UserRepository) MarkEmailVerified(ctx context.Context, id uuid.UUID, at time.Time) error {
	return mapDB(r.q.MarkUserEmailVerified(ctx, sqlcdb.MarkUserEmailVerifiedParams{
		ID:              id,
		EmailVerifiedAt: pgTime(at),
	}))
}

func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return mapDB(r.q.DeleteUser(ctx, id))
}

func userFrom(id uuid.UUID, email, passwordHash, role string, verifiedAt, createdAt pgtype.Timestamptz) entity.User {
	return entity.User{
		ID:              id,
		Email:           email,
		PasswordHash:    passwordHash,
		Role:            entity.Role(role),
		EmailVerifiedAt: timePtr(verifiedAt),
		CreatedAt:       timeFrom(createdAt),
	}
}
