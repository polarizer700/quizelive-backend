package identity

import (
	"context"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
)

type Identity struct {
	UserID uuid.UUID
	Role   entity.Role
}

func (i Identity) Valid() bool {
	return i.UserID != uuid.Nil && i.Role.Valid()
}

type ctxKey struct{}

func With(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

func Current(ctx context.Context) (Identity, error) {
	id, ok := ctx.Value(ctxKey{}).(Identity)
	if !ok || !id.Valid() {
		return Identity{}, domain.ErrUnauthorized
	}

	return id, nil
}

func Require(ctx context.Context) (Identity, error) {
	id, err := Current(ctx)
	if err != nil {
		return Identity{}, err
	}

	if !id.Role.Valid() {
		return Identity{}, domain.ErrForbidden
	}

	return id, nil
}
