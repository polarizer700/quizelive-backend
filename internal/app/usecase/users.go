package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/polarizer700/quizelive-backend/internal/domain"
	"github.com/polarizer700/quizelive-backend/internal/domain/auth"
	"github.com/polarizer700/quizelive-backend/internal/domain/entity"
	"github.com/polarizer700/quizelive-backend/internal/domain/identity"
	"github.com/polarizer700/quizelive-backend/internal/domain/repository"
)

type Users struct {
	users  repository.UserRepository
	hasher auth.PasswordHasher
}

func NewUsers(users repository.UserRepository, hasher auth.PasswordHasher) *Users {
	return &Users{users: users, hasher: hasher}
}

func (u *Users) CreateAdmin(ctx context.Context, email, password string) (entity.User, error) {
	return u.create(ctx, email, password, entity.RoleAdmin)
}

func (u *Users) Create(ctx context.Context, email, password string, role entity.Role) (entity.User, error) {
	actor, err := identity.Require(ctx)
	if err != nil {
		return entity.User{}, err
	}

	if !actor.Role.CanManageUsers() {
		return entity.User{}, domain.ErrForbidden
	}

	return u.create(ctx, email, password, role)
}

func (u *Users) List(ctx context.Context, limit, offset int) ([]entity.User, int64, error) {
	if _, err := u.requireAdmin(ctx); err != nil {
		return nil, 0, err
	}

	page, err := normalizePage(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	users, total, err := u.users.List(ctx, page)
	if err != nil {
		return nil, 0, err
	}

	for i := range users {
		users[i].PasswordHash = ""
	}

	return users, total, nil
}

func (u *Users) Get(ctx context.Context, id uuid.UUID) (entity.User, error) {
	if _, err := u.requireAdmin(ctx); err != nil {
		return entity.User{}, err
	}

	user, err := u.users.GetByID(ctx, id)
	if err != nil {
		return entity.User{}, err
	}

	user.PasswordHash = ""

	return user, nil
}

type UserUpdate struct {
	Email    *string
	Password *string
	Role     *entity.Role
}

func (u *Users) Update(ctx context.Context, id uuid.UUID, patch UserUpdate) (entity.User, error) {
	actor, err := u.requireAdmin(ctx)
	if err != nil {
		return entity.User{}, err
	}

	user, err := u.users.GetByID(ctx, id)
	if err != nil {
		return entity.User{}, err
	}

	if patch.Email != nil {
		email, emailErr := entity.NormalizeEmail(*patch.Email)
		if emailErr != nil {
			return entity.User{}, emailErr
		}

		user.Email = email
	}

	if patch.Password != nil {
		if passErr := entity.ValidatePassword(*patch.Password); passErr != nil {
			return entity.User{}, passErr
		}

		hash, hashErr := u.hasher.Hash(*patch.Password)
		if hashErr != nil {
			return entity.User{}, hashErr
		}

		user.PasswordHash = hash
	}

	if patch.Role != nil {
		if !patch.Role.Valid() {
			return entity.User{}, domain.FieldErrorf("role", "invalid", "role must be admin, editor or host")
		}

		if actor.UserID == user.ID && *patch.Role != actor.Role {
			return entity.User{}, domain.FieldErrorf("role", "invalid", "cannot change your own role")
		}

		user.Role = *patch.Role
	}

	updated, err := u.users.Update(ctx, user)
	if err != nil {
		return entity.User{}, err
	}

	updated.PasswordHash = ""

	return updated, nil
}

func (u *Users) Delete(ctx context.Context, id uuid.UUID) error {
	actor, err := u.requireAdmin(ctx)
	if err != nil {
		return err
	}

	if actor.UserID == id {
		return &domain.ConflictError{Message: "cannot delete yourself"}
	}

	if _, err = u.users.GetByID(ctx, id); err != nil {
		return err
	}

	return u.users.Delete(ctx, id)
}

func (u *Users) requireAdmin(ctx context.Context) (identity.Identity, error) {
	actor, err := identity.Require(ctx)
	if err != nil {
		return identity.Identity{}, err
	}

	if !actor.Role.CanManageUsers() {
		return identity.Identity{}, domain.ErrForbidden
	}

	return actor, nil
}

func (u *Users) create(ctx context.Context, emailRaw, password string, role entity.Role) (entity.User, error) {
	email, err := entity.NormalizeEmail(emailRaw)
	if err != nil {
		return entity.User{}, err
	}

	if err = entity.ValidatePassword(password); err != nil {
		return entity.User{}, err
	}

	if !role.Valid() {
		return entity.User{}, domain.FieldErrorf("role", "invalid", "role must be admin, editor or host")
	}

	hash, err := u.hasher.Hash(password)
	if err != nil {
		return entity.User{}, err
	}

	now := time.Now()
	user, err := u.users.Create(ctx, email, hash, role, &now)
	if err != nil {
		return entity.User{}, err
	}

	user.PasswordHash = ""

	return user, nil
}
